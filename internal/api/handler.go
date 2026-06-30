package api

// handler.go defines the Storywork HTTP routes and JSON error-mapping policy.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"storywork/internal/action"
	"storywork/internal/agent"
	"storywork/internal/codex"
	"storywork/internal/project"
	"storywork/internal/story"
)

// ProjectStore is the project application boundary used by HTTP handlers.
type ProjectStore interface {
	// Create provisions a new portable project folder and returns its summary.
	Create(ctx context.Context, request project.CreateRequest) (project.Project, error)
	// Open validates an existing project folder and returns its summary.
	Open(ctx context.Context, path string) (project.Project, error)
}

// ActiveProjectSession stores the current project for later outline routes.
type ActiveProjectSession interface {
	// Set replaces the current active project for later requests.
	Set(project.Project)
}

type codexEntryResponse struct {
	ID          string            `json:"id"`
	Type        codex.EntryType   `json:"type"`
	Name        string            `json:"name"`
	Aliases     []string          `json:"aliases"`
	Tags        []string          `json:"tags"`
	Description string            `json:"description"`
	Metadata    map[string]string `json:"metadata"`
	Revision    string            `json:"revision"`
}

// codexActiveEntryResponse is the resolved entry shape for active-state inspection.
// It omits revision because the resolved projection is not a canonical document.
type codexActiveEntryResponse struct {
	ID          string            `json:"id"`
	Type        codex.EntryType   `json:"type"`
	Name        string            `json:"name"`
	Aliases     []string          `json:"aliases"`
	Tags        []string          `json:"tags"`
	Description string            `json:"description"`
	Metadata    map[string]string `json:"metadata"`
}

type codexActiveStateResponse struct {
	SceneID               string                   `json:"scene_id"`
	Entry                 codexActiveEntryResponse `json:"entry"`
	AppliedProgressionIDs []string                 `json:"applied_progression_ids"`
}

func newCodexEntryResponse(entry codex.Entry) codexEntryResponse {
	return codexEntryResponse{
		ID:          entry.ID,
		Type:        entry.Type,
		Name:        entry.Name,
		Aliases:     entry.Aliases,
		Tags:        entry.Tags,
		Description: entry.Description,
		Metadata:    entry.Metadata,
		Revision:    entry.Revision,
	}
}

func newCodexActiveEntryResponse(entry codex.Entry) codexActiveEntryResponse {
	return codexActiveEntryResponse{
		ID:          entry.ID,
		Type:        entry.Type,
		Name:        entry.Name,
		Aliases:     entry.Aliases,
		Tags:        entry.Tags,
		Description: entry.Description,
		Metadata:    entry.Metadata,
	}
}

// methodNotAllowed returns a handler that responds with 405 in the project JSON
// error shape and sets the Allow header, for known routes with an unsupported
// method. The spec requires JSON errors and an Allow header on 405.
func methodNotAllowed(allow string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Allow", allow)
		writeError(writer, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed; allowed: %s", request.Method, allow))
	}
}

// writeBodyLimitError maps a JSON body read error to the documented status. An
// oversized Codex/progression mutation body is a 400 Bad Request per the
// Milestone 3 status table (which lists no 413).
func writeBodyLimitError(writer http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		writeError(writer, status, fmt.Errorf("request body exceeds the 1 MiB limit"))
		return
	}
	writeError(writer, status, err)
}

// StoryStore serves and mutates the active project's outline, scenes, and Codex state.
type StoryStore interface {
	// Outline returns the active project's hierarchical outline.
	Outline(ctx context.Context) (story.Outline, error)
	// CreateArc appends one new top-level arc.
	CreateArc(ctx context.Context, title string) (story.MutationResult, error)
	// CreateChapter appends one new chapter under an existing arc.
	CreateChapter(ctx context.Context, arcID, title string) (story.MutationResult, error)
	// CreateScene appends one new scene under an existing chapter.
	CreateScene(ctx context.Context, chapterID, title string) (story.MutationResult, error)
	// Reorder reorders chapters or scenes using stable IDs.
	Reorder(ctx context.Context, request story.ReorderRequest) (story.MutationResult, error)
	// LoadScene returns one canonical scene for editor use.
	LoadScene(ctx context.Context, sceneID string) (story.SceneDocument, error)
	// SaveScene validates and persists one explicit scene edit.
	SaveScene(ctx context.Context, sceneID string, request story.SaveSceneRequest) (story.SceneDocument, error)
	// CodexEntries returns the active project's validated Codex list.
	CodexEntries(ctx context.Context) ([]codex.Entry, error)
	// LoadCodexEntry returns one validated canonical Codex entry.
	LoadCodexEntry(ctx context.Context, entryID string) (codex.Entry, error)
	// CreateCodexEntry creates one new canonical Codex entry.
	CreateCodexEntry(ctx context.Context, request codex.SaveEntryRequest) (codex.Entry, error)
	// UpdateCodexEntry edits one existing canonical Codex entry.
	UpdateCodexEntry(ctx context.Context, entryID string, request codex.SaveEntryRequest) (codex.Entry, error)
	// LoadProgressions returns one entry's ordered progression document.
	LoadProgressions(ctx context.Context, entryID string) (codex.ProgressionDocument, error)
	// SaveProgressions replaces one entry's ordered progression document.
	SaveProgressions(ctx context.Context, entryID string, request codex.SaveProgressionsRequest) (codex.ProgressionDocument, error)
	// ResolveActiveCodexState resolves one entry as of a target scene.
	ResolveActiveCodexState(ctx context.Context, entryID, sceneID string) (codex.ActiveState, error)
	// Agents returns the strictly loaded project-local agent registry.
	Agents(ctx context.Context) ([]agent.Agent, error)
	// Styles returns the strictly loaded project-local style registry.
	Styles(ctx context.Context) ([]agent.Style, error)
	// AvailableActions returns the applicable actions for the current state.
	AvailableActions(ctx context.Context, input agent.AvailabilityInput) ([]action.AvailableAction, error)
	// Run executes one transient mock action against canonical scene bytes.
	Run(ctx context.Context, request action.RunRequest) (action.Run, error)
	// Accept applies one pending run to canonical scene markdown.
	Accept(ctx context.Context, runID, expectedRevision string) (action.Run, story.SceneDocument, error)
	// Reject discards one pending run without mutating canon.
	Reject(ctx context.Context, runID string) (action.Run, error)
}

type requiredJSONField struct {
	name      string
	allowNull bool
}

// NewHandler creates the full local Storywork HTTP router for the current milestone set.
func NewHandler(projects ProjectStore, session ActiveProjectSession, stories StoryStore, version string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, http.StatusOK, map[string]string{"status": "ok", "version": version})
	})
	mux.HandleFunc("POST /api/projects", func(writer http.ResponseWriter, request *http.Request) {
		var createRequest project.CreateRequest
		if err := decodeJSON(request, &createRequest); err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}
		created, err := projects.Create(request.Context(), createRequest)
		if err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}
		session.Set(created)
		writeJSON(writer, http.StatusCreated, created)
	})
	mux.HandleFunc("POST /api/projects/open", func(writer http.ResponseWriter, request *http.Request) {
		var openRequest struct {
			Path string `json:"path"`
		}
		if err := decodeJSON(request, &openRequest); err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}
		opened, err := projects.Open(request.Context(), openRequest.Path)
		if err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}
		session.Set(opened)
		writeJSON(writer, http.StatusOK, opened)
	})
	mux.HandleFunc("GET /api/outline", func(writer http.ResponseWriter, request *http.Request) {
		outline, err := stories.Outline(request.Context())
		if err != nil {
			writeStoryError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, outline)
	})
	mux.HandleFunc("POST /api/arcs", func(writer http.ResponseWriter, request *http.Request) {
		var createRequest struct {
			Title string `json:"title"`
		}
		if err := decodeJSON(request, &createRequest); err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}
		result, err := stories.CreateArc(request.Context(), createRequest.Title)
		if err != nil {
			writeStoryError(writer, err)
			return
		}
		writeJSON(writer, http.StatusCreated, result)
	})
	mux.HandleFunc("POST /api/chapters", func(writer http.ResponseWriter, request *http.Request) {
		var createRequest struct {
			ArcID string `json:"arc_id"`
			Title string `json:"title"`
		}
		if err := decodeJSON(request, &createRequest); err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}
		result, err := stories.CreateChapter(request.Context(), createRequest.ArcID, createRequest.Title)
		if err != nil {
			writeStoryError(writer, err)
			return
		}
		writeJSON(writer, http.StatusCreated, result)
	})
	mux.HandleFunc("POST /api/scenes", func(writer http.ResponseWriter, request *http.Request) {
		var createRequest struct {
			ChapterID string `json:"chapter_id"`
			Title     string `json:"title"`
		}
		if err := decodeJSON(request, &createRequest); err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}
		result, err := stories.CreateScene(request.Context(), createRequest.ChapterID, createRequest.Title)
		if err != nil {
			writeStoryError(writer, err)
			return
		}
		writeJSON(writer, http.StatusCreated, result)
	})
	mux.HandleFunc("POST /api/outline/reorder", func(writer http.ResponseWriter, request *http.Request) {
		var reorderRequest story.ReorderRequest
		if err := decodeJSON(request, &reorderRequest); err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}
		result, err := stories.Reorder(request.Context(), reorderRequest)
		if err != nil {
			writeStoryError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, result)
	})
	mux.HandleFunc("GET /api/scenes/{scene_id}", func(writer http.ResponseWriter, request *http.Request) {
		sceneDocument, err := stories.LoadScene(request.Context(), request.PathValue("scene_id"))
		if err != nil {
			writeStoryError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, sceneDocument)
	})
	mux.HandleFunc("PUT /api/scenes/{scene_id}", func(writer http.ResponseWriter, request *http.Request) {
		var saveRequest struct {
			Title            string                  `json:"title"`
			FrontMatter      *story.SceneFrontMatter `json:"frontmatter"`
			Markdown         string                  `json:"markdown"`
			ExpectedRevision string                  `json:"expected_revision"`
		}
		if err := decodeJSONWithLimit(writer, request, &saveRequest, 6<<20); err != nil {
			status := http.StatusBadRequest
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				status = http.StatusRequestEntityTooLarge
			}
			writeError(writer, status, err)
			return
		}
		if saveRequest.FrontMatter == nil {
			writeError(writer, http.StatusBadRequest, errors.New("frontmatter is required"))
			return
		}
		sceneDocument, err := stories.SaveScene(request.Context(), request.PathValue("scene_id"), story.SaveSceneRequest{
			Title:            saveRequest.Title,
			FrontMatter:      *saveRequest.FrontMatter,
			Markdown:         saveRequest.Markdown,
			ExpectedRevision: saveRequest.ExpectedRevision,
		})
		if err != nil {
			writeStoryError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, sceneDocument)
	})
	mux.HandleFunc("GET /api/codex", func(writer http.ResponseWriter, request *http.Request) {
		entries, err := stories.CodexEntries(request.Context())
		if err != nil {
			writeStoryError(writer, err)
			return
		}
		response := make([]codexEntryResponse, 0, len(entries))
		for _, entry := range entries {
			response = append(response, newCodexEntryResponse(entry))
		}
		writeJSON(writer, http.StatusOK, map[string][]codexEntryResponse{"entries": response})
	})
	mux.HandleFunc("/api/codex", methodNotAllowed("GET, POST"))
	mux.HandleFunc("POST /api/codex", func(writer http.ResponseWriter, request *http.Request) {
		var createRequest struct {
			Type        codex.EntryType   `json:"type"`
			Name        string            `json:"name"`
			Aliases     []string          `json:"aliases"`
			Tags        []string          `json:"tags"`
			Description string            `json:"description"`
			Metadata    map[string]string `json:"metadata"`
		}
		if err := decodeJSONWithRequiredFields(writer, request, &createRequest, 1<<20,
			requiredJSONField{name: "type"},
			requiredJSONField{name: "name"},
			requiredJSONField{name: "aliases"},
			requiredJSONField{name: "tags"},
			requiredJSONField{name: "description"},
			requiredJSONField{name: "metadata"},
		); err != nil {
			writeBodyLimitError(writer, err)
			return
		}
		entry, err := stories.CreateCodexEntry(request.Context(), codex.SaveEntryRequest{
			Type:        createRequest.Type,
			Name:        createRequest.Name,
			Aliases:     createRequest.Aliases,
			Tags:        createRequest.Tags,
			Description: createRequest.Description,
			Metadata:    createRequest.Metadata,
		})
		if err != nil {
			writeStoryError(writer, err)
			return
		}
		writeJSON(writer, http.StatusCreated, newCodexEntryResponse(entry))
	})
	mux.HandleFunc("GET /api/codex/{entry_id}", func(writer http.ResponseWriter, request *http.Request) {
		entry, err := stories.LoadCodexEntry(request.Context(), request.PathValue("entry_id"))
		if err != nil {
			writeStoryError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, newCodexEntryResponse(entry))
	})
	mux.HandleFunc("/api/codex/{entry_id}", methodNotAllowed("GET, PUT"))
	mux.HandleFunc("PUT /api/codex/{entry_id}", func(writer http.ResponseWriter, request *http.Request) {
		var updateRequest struct {
			Name             string            `json:"name"`
			Aliases          []string          `json:"aliases"`
			Tags             []string          `json:"tags"`
			Description      string            `json:"description"`
			Metadata         map[string]string `json:"metadata"`
			ExpectedRevision string            `json:"expected_revision"`
		}
		if err := decodeJSONWithRequiredFields(writer, request, &updateRequest, 1<<20,
			requiredJSONField{name: "name"},
			requiredJSONField{name: "aliases"},
			requiredJSONField{name: "tags"},
			requiredJSONField{name: "description"},
			requiredJSONField{name: "metadata"},
			requiredJSONField{name: "expected_revision"},
		); err != nil {
			writeBodyLimitError(writer, err)
			return
		}
		entry, err := stories.UpdateCodexEntry(request.Context(), request.PathValue("entry_id"), codex.SaveEntryRequest{
			Name:             updateRequest.Name,
			Aliases:          updateRequest.Aliases,
			Tags:             updateRequest.Tags,
			Description:      updateRequest.Description,
			Metadata:         updateRequest.Metadata,
			ExpectedRevision: updateRequest.ExpectedRevision,
		})
		if err != nil {
			writeStoryError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, newCodexEntryResponse(entry))
	})
	mux.HandleFunc("GET /api/codex/{entry_id}/progressions", func(writer http.ResponseWriter, request *http.Request) {
		document, err := stories.LoadProgressions(request.Context(), request.PathValue("entry_id"))
		if err != nil {
			writeStoryError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, document)
	})
	mux.HandleFunc("/api/codex/{entry_id}/progressions", methodNotAllowed("GET, PUT"))
	mux.HandleFunc("PUT /api/codex/{entry_id}/progressions", func(writer http.ResponseWriter, request *http.Request) {
		var updateRequest struct {
			Progressions     []codex.Progression `json:"progressions"`
			ExpectedRevision *string             `json:"expected_revision"`
		}
		body, err := readJSONBodyWithLimit(writer, request, 1<<20)
		if err == nil {
			err = requireJSONFields(body,
				requiredJSONField{name: "progressions"},
				requiredJSONField{name: "expected_revision", allowNull: true},
			)
		}
		if err == nil {
			err = validateProgressionUpdateJSON(body)
		}
		if err == nil {
			err = decodeJSONBytes(body, &updateRequest)
		}
		if err != nil {
			writeBodyLimitError(writer, err)
			return
		}
		document, err := stories.SaveProgressions(request.Context(), request.PathValue("entry_id"), codex.SaveProgressionsRequest{
			Progressions:     updateRequest.Progressions,
			ExpectedRevision: updateRequest.ExpectedRevision,
		})
		if err != nil {
			writeStoryError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, document)
	})
	mux.HandleFunc("GET /api/codex/{entry_id}/active", func(writer http.ResponseWriter, request *http.Request) {
		sceneID := request.URL.Query().Get("scene_id")
		if sceneID == "" {
			writeError(writer, http.StatusBadRequest, errors.New("scene_id is required"))
			return
		}
		activeState, err := stories.ResolveActiveCodexState(request.Context(), request.PathValue("entry_id"), sceneID)
		if err != nil {
			writeStoryError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, codexActiveStateResponse{
			SceneID:               activeState.SceneID,
			Entry:                 newCodexActiveEntryResponse(activeState.Entry),
			AppliedProgressionIDs: activeState.AppliedProgressionIDs,
		})
	})
	mux.HandleFunc("/api/codex/{entry_id}/active", methodNotAllowed("GET"))
	mux.HandleFunc("GET /api/agents", func(writer http.ResponseWriter, request *http.Request) {
		agents, err := stories.Agents(request.Context())
		if err != nil {
			writeStoryError(writer, err)
			return
		}
		type agentResponse struct {
			ID                 string              `json:"id"`
			Name               string              `json:"name"`
			Description        string              `json:"description"`
			Surfaces           []agent.Surface     `json:"surfaces"`
			InputScopes        []agent.InputScope  `json:"input_scopes"`
			MinWords           int                 `json:"min_words"`
			MaxWords           int                 `json:"max_words"`
			RequiredContext    []agent.ContextPack `json:"required_context"`
			OptionalContext    []agent.ContextPack `json:"optional_context"`
			ForbiddenContext   []agent.ContextPack `json:"forbidden_context"`
			RAGMode            agent.RAGMode       `json:"rag_mode"`
			OutputMode         agent.OutputMode    `json:"output_mode"`
			RequiresAcceptance bool                `json:"requires_acceptance"`
		}
		response := make([]agentResponse, 0, len(agents))
		for _, item := range agents {
			response = append(response, agentResponse{
				ID:                 item.ID,
				Name:               item.Name,
				Description:        item.Description,
				Surfaces:           append([]agent.Surface(nil), item.AppliesWhen.Surfaces...),
				InputScopes:        append([]agent.InputScope(nil), item.AppliesWhen.InputScopes...),
				MinWords:           item.AppliesWhen.MinWords,
				MaxWords:           item.AppliesWhen.MaxWords,
				RequiredContext:    append([]agent.ContextPack(nil), item.ContextPolicy.Required...),
				OptionalContext:    append([]agent.ContextPack(nil), item.ContextPolicy.Optional...),
				ForbiddenContext:   append([]agent.ContextPack(nil), item.ContextPolicy.Forbidden...),
				RAGMode:            item.RAGPolicy.Mode,
				OutputMode:         item.Control.OutputMode,
				RequiresAcceptance: item.Control.RequiresAcceptance,
			})
		}
		writeJSON(writer, http.StatusOK, map[string]any{"agents": response})
	})
	mux.HandleFunc("GET /api/styles", func(writer http.ResponseWriter, request *http.Request) {
		styles, err := stories.Styles(request.Context())
		if err != nil {
			writeStoryError(writer, err)
			return
		}
		type styleResponse struct {
			ID                string  `json:"id"`
			Name              string  `json:"name"`
			ProviderProfileID string  `json:"provider_profile_id"`
			Model             string  `json:"model"`
			Temperature       float64 `json:"temperature"`
			SystemPrompt      string  `json:"system_prompt"`
		}
		response := make([]styleResponse, 0, len(styles))
		for _, item := range styles {
			response = append(response, styleResponse{
				ID:                item.ID,
				Name:              item.Name,
				ProviderProfileID: item.ProviderProfileID,
				Model:             item.Model,
				Temperature:       item.Temperature,
				SystemPrompt:      item.SystemPrompt,
			})
		}
		writeJSON(writer, http.StatusOK, map[string]any{"styles": response})
	})
	mux.HandleFunc("GET /api/actions/available", func(writer http.ResponseWriter, request *http.Request) {
		if err := validateExactQuery(request, "surface", "input_scope", "scene_id", "selection_words"); err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}
		if !request.URL.Query().Has("selection_words") {
			writeError(writer, http.StatusBadRequest, errors.New("selection_words is required"))
			return
		}
		selectionWords := 0
		if raw := request.URL.Query().Get("selection_words"); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil {
				writeError(writer, http.StatusBadRequest, fmt.Errorf("selection_words must be an integer"))
				return
			}
			if value < 0 {
				writeError(writer, http.StatusBadRequest, fmt.Errorf("selection_words must be greater than or equal to zero"))
				return
			}
			selectionWords = value
		}
		input := agent.AvailabilityInput{
			Surface:        agent.Surface(request.URL.Query().Get("surface")),
			InputScope:     agent.InputScope(request.URL.Query().Get("input_scope")),
			SceneID:        request.URL.Query().Get("scene_id"),
			SelectionWords: selectionWords,
		}
		if err := validateAvailabilityInput(input); err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}
		actions, err := stories.AvailableActions(request.Context(), input)
		if err != nil {
			writeStoryError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, map[string]any{"actions": actions})
	})
	mux.HandleFunc("POST /api/actions/run", func(writer http.ResponseWriter, request *http.Request) {
		var runRequest struct {
			AgentID       string `json:"agent_id"`
			StyleID       string `json:"style_id"`
			Surface       string `json:"surface"`
			InputScope    string `json:"input_scope"`
			SceneID       string `json:"scene_id"`
			SceneRevision string `json:"scene_revision"`
			Selection     *struct {
				StartByte *int    `json:"start_byte"`
				EndByte   *int    `json:"end_byte"`
				Text      *string `json:"text"`
			} `json:"selection"`
		}
		if err := decodeJSONWithRequiredFields(writer, request, &runRequest, 1<<20,
			requiredJSONField{name: "agent_id"},
			requiredJSONField{name: "style_id"},
			requiredJSONField{name: "surface"},
			requiredJSONField{name: "input_scope"},
			requiredJSONField{name: "scene_id"},
			requiredJSONField{name: "scene_revision"},
			requiredJSONField{name: "selection"},
		); err != nil {
			writeBodyLimitError(writer, err)
			return
		}
		if runRequest.Selection == nil || runRequest.Selection.StartByte == nil || runRequest.Selection.EndByte == nil || runRequest.Selection.Text == nil {
			writeError(writer, http.StatusBadRequest, errors.New("selection.start_byte, selection.end_byte, and selection.text are required"))
			return
		}
		domainRequest := action.RunRequest{
			AgentID: runRequest.AgentID, StyleID: runRequest.StyleID,
			Surface: agent.Surface(runRequest.Surface), InputScope: agent.InputScope(runRequest.InputScope),
			SceneID: runRequest.SceneID, SceneRevision: runRequest.SceneRevision,
			Selection: action.Selection{StartByte: *runRequest.Selection.StartByte, EndByte: *runRequest.Selection.EndByte, Text: *runRequest.Selection.Text},
		}
		if err := action.ValidateRunRequest(domainRequest); err != nil {
			writeStoryError(writer, err)
			return
		}
		run, err := stories.Run(request.Context(), domainRequest)
		if err != nil {
			writeStoryError(writer, err)
			return
		}
		writeJSON(writer, http.StatusCreated, map[string]any{
			"run_id":         run.RunID,
			"status":         run.Status,
			"agent_id":       run.AgentID,
			"style_id":       run.StyleID,
			"scene_id":       run.SceneID,
			"scene_revision": run.SceneRevision,
			"selection": map[string]int{
				"start_byte": run.Selection.StartByte,
				"end_byte":   run.Selection.EndByte,
			},
			"output_mode": "patch",
			"patch": map[string]string{
				"original":    run.OriginalText,
				"replacement": run.Replacement,
			},
			"context_summary": run.ContextSummary,
		})
	})
	mux.HandleFunc("POST /api/actions/{run_id}/accept", func(writer http.ResponseWriter, request *http.Request) {
		if err := action.ValidateRunID(request.PathValue("run_id")); err != nil {
			writeStoryError(writer, err)
			return
		}
		var acceptRequest struct {
			ExpectedRevision string `json:"expected_revision"`
		}
		if err := decodeJSONWithRequiredFields(writer, request, &acceptRequest, 1<<20, requiredJSONField{name: "expected_revision"}); err != nil {
			writeBodyLimitError(writer, err)
			return
		}
		if err := story.ValidateRevision(acceptRequest.ExpectedRevision); err != nil {
			writeStoryError(writer, err)
			return
		}
		run, sceneDocument, err := stories.Accept(request.Context(), request.PathValue("run_id"), acceptRequest.ExpectedRevision)
		if err != nil {
			writeStoryError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, map[string]any{
			"run_id": run.RunID,
			"status": run.Status,
			"scene":  sceneDocument,
		})
	})
	mux.HandleFunc("POST /api/actions/{run_id}/reject", func(writer http.ResponseWriter, request *http.Request) {
		if err := action.ValidateRunID(request.PathValue("run_id")); err != nil {
			writeStoryError(writer, err)
			return
		}
		if err := requireEmptyBody(request); err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}
		run, err := stories.Reject(request.Context(), request.PathValue("run_id"))
		if err != nil {
			writeStoryError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, map[string]any{
			"run_id": run.RunID,
			"status": run.Status,
		})
	})
	mux.HandleFunc("/api/actions/available", methodNotAllowed("GET"))
	mux.HandleFunc("/api/actions/run", methodNotAllowed("POST"))
	mux.HandleFunc("/api/actions/{run_id}/accept", methodNotAllowed("POST"))
	mux.HandleFunc("/api/actions/{run_id}/reject", methodNotAllowed("POST"))
	return mux
}

func validateExactQuery(request *http.Request, allowed ...string) error {
	allowedKeys := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		allowedKeys[key] = struct{}{}
	}
	for key, values := range request.URL.Query() {
		if _, ok := allowedKeys[key]; !ok {
			return fmt.Errorf("unknown query parameter %q", key)
		}
		if len(values) != 1 {
			return fmt.Errorf("query parameter %q must be provided once", key)
		}
	}
	return nil
}

func requireEmptyBody(request *http.Request) error {
	body, err := io.ReadAll(io.LimitReader(request.Body, 2))
	if err != nil {
		return fmt.Errorf("read request body: %w", err)
	}
	if len(body) != 0 {
		return errors.New("request body must be empty")
	}
	return nil
}

func decodeJSON(request *http.Request, target any) error {
	return decodeJSONWithLimit(nil, request, target, 1<<20)
}

func decodeJSONWithRequiredFields(writer http.ResponseWriter, request *http.Request, target any, limit int64, requiredFields ...requiredJSONField) error {
	body, err := readJSONBodyWithLimit(writer, request, limit)
	if err != nil {
		return err
	}
	if err := requireJSONFields(body, requiredFields...); err != nil {
		return err
	}
	return decodeJSONBytes(body, target)
}

func decodeJSONWithLimit(writer http.ResponseWriter, request *http.Request, target any, limit int64) error {
	body, err := readJSONBodyWithLimit(writer, request, limit)
	if err != nil {
		return err
	}
	return decodeJSONBytes(body, target)
}

func readJSONBodyWithLimit(writer http.ResponseWriter, request *http.Request, limit int64) ([]byte, error) {
	defer request.Body.Close()
	reader := io.Reader(request.Body)
	if writer != nil {
		reader = http.MaxBytesReader(writer, request.Body, limit)
	} else {
		reader = io.LimitReader(request.Body, limit)
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("invalid JSON request: %w", err)
	}
	return body, nil
}

func decodeJSONBytes(body []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid JSON request: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		if err == nil {
			return errors.New("invalid JSON request: unexpected trailing JSON value")
		}
		return fmt.Errorf("invalid JSON request: %w", err)
	}
	return nil
}

func requireJSONFields(body []byte, requiredFields ...requiredJSONField) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return fmt.Errorf("invalid JSON request: %w", err)
	}
	for _, field := range requiredFields {
		value, ok := fields[field.name]
		if !ok {
			return fmt.Errorf("invalid JSON request: %s is required", field.name)
		}
		if !field.allowNull && bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("invalid JSON request: %s must not be null", field.name)
		}
	}
	return nil
}

// validateProgressionUpdateJSON enforces nested request presence and nullability
// before JSON is converted into domain structs, where null and omission would
// otherwise collapse to the same Go zero value.
func validateProgressionUpdateJSON(body []byte) error {
	var root struct {
		Progressions []json.RawMessage `json:"progressions"`
	}
	if err := json.Unmarshal(body, &root); err != nil {
		return fmt.Errorf("invalid JSON request: %w", err)
	}
	for index, rawProgression := range root.Progressions {
		context := fmt.Sprintf("progressions[%d]", index)
		fields, err := decodeRawJSONObject(rawProgression, context)
		if err != nil {
			return err
		}
		if rawID, ok := fields["id"]; ok {
			if err := requireNonEmptyJSONString(rawID, context+".id"); err != nil {
				return err
			}
		}
		rawAnchor, err := requireRawJSONField(fields, "anchor", context)
		if err != nil {
			return err
		}
		anchor, err := decodeRawJSONObject(rawAnchor, context+".anchor")
		if err != nil {
			return err
		}
		for _, field := range []string{"type", "id", "timing"} {
			rawValue, err := requireRawJSONField(anchor, field, context+".anchor")
			if err != nil {
				return err
			}
			if err := requireJSONString(rawValue, context+".anchor."+field); err != nil {
				return err
			}
		}
		rawChanges, err := requireRawJSONField(fields, "changes", context)
		if err != nil {
			return err
		}
		changes, err := decodeRawJSONObject(rawChanges, context+".changes")
		if err != nil {
			return err
		}
		description, hasDescription := changes["description"]
		metadata, hasMetadata := changes["metadata"]
		if !hasDescription && !hasMetadata {
			return fmt.Errorf("invalid JSON request: %s.changes must include description or metadata", context)
		}
		if hasDescription {
			if err := requireJSONString(description, context+".changes.description"); err != nil {
				return err
			}
		}
		if hasMetadata {
			metadataFields, err := decodeRawJSONObject(metadata, context+".changes.metadata")
			if err != nil {
				return err
			}
			for key, rawValue := range metadataFields {
				if err := requireJSONString(rawValue, context+".changes.metadata."+key); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// decodeRawJSONObject preserves field presence while validating a nested JSON object.
func decodeRawJSONObject(raw json.RawMessage, context string) (map[string]json.RawMessage, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, fmt.Errorf("invalid JSON request: %s must not be null", context)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil, fmt.Errorf("invalid JSON request: %s must be an object", context)
	}
	return fields, nil
}

// requireRawJSONField distinguishes a missing field from an explicitly null field.
func requireRawJSONField(fields map[string]json.RawMessage, field, context string) (json.RawMessage, error) {
	raw, ok := fields[field]
	if !ok {
		return nil, fmt.Errorf("invalid JSON request: %s.%s is required", context, field)
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, fmt.Errorf("invalid JSON request: %s.%s must not be null", context, field)
	}
	return raw, nil
}

// requireJSONString rejects null and non-string JSON scalars at transport boundaries.
func requireJSONString(raw json.RawMessage, context string) error {
	_, err := decodeJSONString(raw, context)
	return err
}

// decodeJSONString returns a validated string while retaining field context in errors.
func decodeJSONString(raw json.RawMessage, context string) (string, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", fmt.Errorf("invalid JSON request: %s must not be null", context)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("invalid JSON request: %s must be a string", context)
	}
	return value, nil
}

// requireNonEmptyJSONString validates fields whose omission has meaning but whose supplied value must be usable.
func requireNonEmptyJSONString(raw json.RawMessage, context string) error {
	value, err := decodeJSONString(raw, context)
	if err != nil {
		return err
	}
	if value == "" {
		return fmt.Errorf("invalid JSON request: %s must not be empty", context)
	}
	return nil
}

func writeStoryError(writer http.ResponseWriter, err error) {
	writeError(writer, statusForStoryError(err), err)
}

func validateAvailabilityInput(input agent.AvailabilityInput) error {
	switch input.Surface {
	case agent.SurfaceEditor, agent.SurfaceChapterView:
	default:
		return fmt.Errorf("surface must be one of %q or %q", agent.SurfaceEditor, agent.SurfaceChapterView)
	}
	switch input.InputScope {
	case agent.InputScopeSelection, agent.InputScopeChapter:
	default:
		return fmt.Errorf("input_scope must be one of %q or %q", agent.InputScopeSelection, agent.InputScopeChapter)
	}
	if input.Surface == agent.SurfaceEditor && input.InputScope == agent.InputScopeSelection && strings.TrimSpace(input.SceneID) == "" {
		return fmt.Errorf("scene_id is required for editor selection availability")
	}
	return nil
}

func statusForStoryError(err error) int {
	switch {
	case errors.Is(err, agent.ErrRegistryLoad):
		return http.StatusInternalServerError
	case errors.Is(err, story.ErrNoActiveProject), errors.Is(err, story.ErrDirtyWorktree), errors.Is(err, story.ErrStaleRevision):
		return http.StatusConflict
	case errors.Is(err, story.ErrInvalidTitle), errors.Is(err, story.ErrInvalidID), errors.Is(err, story.ErrInvalidReorder), errors.Is(err, story.ErrInvalidPOV), errors.Is(err, story.ErrInvalidStatus), errors.Is(err, story.ErrInvalidMarkdown), errors.Is(err, story.ErrInvalidRevision), errors.Is(err, story.ErrNoSceneChanges):
		return http.StatusBadRequest
	case errors.Is(err, story.ErrInvalidSelection), errors.Is(err, agent.ErrInvalidAgent), errors.Is(err, agent.ErrInvalidStyle), errors.Is(err, agent.ErrInapplicable), errors.Is(err, action.ErrInvalidRunRequest):
		return http.StatusBadRequest
	case errors.Is(err, codex.ErrInvalidType), errors.Is(err, codex.ErrInvalidID), errors.Is(err, codex.ErrInvalidName), errors.Is(err, codex.ErrInvalidAlias), errors.Is(err, codex.ErrInvalidTag), errors.Is(err, codex.ErrInvalidDescription), errors.Is(err, codex.ErrInvalidMetadata), errors.Is(err, codex.ErrInvalidRevision), errors.Is(err, codex.ErrInvalidProgression), errors.Is(err, codex.ErrNoChanges):
		return http.StatusBadRequest
	case errors.Is(err, action.ErrRunConflict):
		return http.StatusConflict
	case errors.Is(err, story.ErrParentNotFound), errors.Is(err, story.ErrSceneNotFound), errors.Is(err, codex.ErrEntryNotFound), errors.Is(err, codex.ErrSceneNotFound):
		return http.StatusNotFound
	case errors.Is(err, action.ErrAgentNotFound), errors.Is(err, action.ErrStyleNotFound), errors.Is(err, action.ErrRunNotFound):
		return http.StatusNotFound
	case errors.Is(err, action.ErrRunCapacity), errors.Is(err, action.ErrProviderUnavailable):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

func writeError(writer http.ResponseWriter, status int, err error) {
	writeJSON(writer, status, map[string]string{"error": err.Error()})
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	if err := json.NewEncoder(writer).Encode(value); err != nil && !errors.Is(err, http.ErrHandlerTimeout) {
		_, _ = writer.Write([]byte(strings.TrimSpace(err.Error())))
	}
}
