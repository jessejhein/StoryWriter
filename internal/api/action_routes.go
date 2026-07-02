package api

// action_routes.go registers Milestone 7 action HTTP routes and response helpers.

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"storywork/internal/action"
	"storywork/internal/agent"
	"storywork/internal/contextpack"
	"storywork/internal/provider"
	"storywork/internal/story"
)

type actionRouteDeps struct {
	actions   ActionStore
	providers ProviderStore
}

func registerActionRoutes(mux *http.ServeMux, deps actionRouteDeps) {
	mux.HandleFunc("GET /api/agents", func(writer http.ResponseWriter, request *http.Request) {
		agents, err := deps.actions.Agents(request.Context())
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
		styles, err := deps.actions.Styles(request.Context())
		if err != nil {
			writeStoryError(writer, err)
			return
		}
		profiles, _, err := deps.providers.ProviderProfiles(request.Context())
		if err != nil {
			writeProviderError(writer, err)
			return
		}
		readinessByID := make(map[string]provider.ProfileReadiness, len(profiles))
		for _, item := range profiles {
			if item.Readiness == provider.ReadinessMissingCredential {
				readinessByID[item.ID] = provider.ProfileReadinessMissingCredential
				continue
			}
			readinessByID[item.ID] = provider.ProfileReadinessReady
		}
		type styleResponse struct {
			ID                string                    `json:"id"`
			Version           int                       `json:"version"`
			Name              string                    `json:"name"`
			ProviderProfileID string                    `json:"provider_profile_id"`
			Model             string                    `json:"model"`
			Temperature       float64                   `json:"temperature"`
			SystemPrompt      string                    `json:"system_prompt"`
			ProviderReadiness provider.ProfileReadiness `json:"provider_readiness"`
		}
		response := make([]styleResponse, 0, len(styles))
		for _, item := range styles {
			readiness := provider.ProfileReadinessReady
			if item.Version == 2 {
				var ok bool
				readiness, ok = readinessByID[item.ProviderProfileID]
				if !ok {
					readiness = provider.ProfileReadinessMissingProfile
				}
			}
			response = append(response, styleResponse{
				ID:                item.ID,
				Version:           item.Version,
				Name:              item.Name,
				ProviderProfileID: item.ProviderProfileID,
				Model:             item.Model,
				Temperature:       item.Temperature,
				SystemPrompt:      item.SystemPrompt,
				ProviderReadiness: readiness,
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
		availableActions, err := deps.actions.AvailableActions(request.Context(), input)
		if err != nil {
			writeStoryError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, map[string]any{"actions": availableActions})
	})
	mux.HandleFunc("POST /api/actions/context-preview", func(writer http.ResponseWriter, request *http.Request) {
		previewRequest, _, err := decodeTaggedRunRequest(writer, request)
		if err != nil {
			return
		}
		preview, err := deps.actions.PreviewContext(request.Context(), previewRequest)
		if err != nil {
			writeStoryError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, map[string]any{
			"manifest":        preview.Manifest,
			"target_revision": preview.TargetRevision,
		})
	})
	mux.HandleFunc("POST /api/actions/run", func(writer http.ResponseWriter, request *http.Request) {
		taggedRequest, tagged, err := decodeTaggedRunRequest(writer, request)
		if err != nil {
			return
		}
		var run action.Run
		if tagged {
			run, err = deps.actions.RunTagged(request.Context(), taggedRequest)
		} else {
			selection := taggedRequest.Target.Selection
			if selection == nil {
				writeError(writer, http.StatusBadRequest, errors.New("selection target is required"))
				return
			}
			legacyRequest := action.RunRequest{
				AgentID: taggedRequest.AgentID, StyleID: taggedRequest.StyleID,
				Surface: agent.SurfaceEditor, InputScope: agent.InputScopeSelection,
				SceneID: selection.SceneID, SceneRevision: selection.SceneRevision,
				Selection: action.Selection{
					StartByte: selection.StartByte, EndByte: selection.EndByte, Text: selection.SelectedText,
				},
			}
			if err := action.ValidateRunRequest(legacyRequest); err != nil {
				writeStoryError(writer, err)
				return
			}
			run, err = deps.actions.Run(request.Context(), legacyRequest)
		}
		if err != nil {
			writeStoryError(writer, err)
			return
		}
		writeActionRunResponse(writer, run)
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
		result, err := deps.actions.AcceptRun(request.Context(), request.PathValue("run_id"), acceptRequest.ExpectedRevision)
		if err != nil {
			writeStoryError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, map[string]any{
			"run_id":                result.Run.RunID,
			"status":                result.Run.Status,
			"scene":                 result.Scene,
			"follow_up_invitations": result.FollowUpInvitations,
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
		run, err := deps.actions.Reject(request.Context(), request.PathValue("run_id"))
		if err != nil {
			writeStoryError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, map[string]any{
			"run_id": run.RunID,
			"status": run.Status,
		})
	})
	mux.HandleFunc("POST /api/action-invitations/{invitation_id}/run", func(writer http.ResponseWriter, request *http.Request) {
		invitationID := request.PathValue("invitation_id")
		if err := action.ValidateInvitationID(invitationID); err != nil {
			writeStoryError(writer, err)
			return
		}
		var runRequest struct {
			StyleID                string `json:"style_id"`
			ExpectedTargetRevision string `json:"expected_target_revision"`
		}
		if err := decodeJSONWithRequiredFields(writer, request, &runRequest, 1<<20,
			requiredJSONField{name: "style_id"},
			requiredJSONField{name: "expected_target_revision"},
		); err != nil {
			writeBodyLimitError(writer, err)
			return
		}
		run, err := deps.actions.RunInvitation(request.Context(), invitationID, action.InvitationRunRequest{
			StyleID: runRequest.StyleID, ExpectedTargetRevision: runRequest.ExpectedTargetRevision,
		})
		if err != nil {
			writeStoryError(writer, err)
			return
		}
		writeActionRunResponse(writer, run)
	})
	mux.HandleFunc("/api/actions/available", methodNotAllowed("GET"))
	mux.HandleFunc("/api/actions/context-preview", methodNotAllowed("POST"))
	mux.HandleFunc("/api/actions/run", methodNotAllowed("POST"))
	mux.HandleFunc("/api/actions/{run_id}/accept", methodNotAllowed("POST"))
	mux.HandleFunc("/api/actions/{run_id}/reject", methodNotAllowed("POST"))
	mux.HandleFunc("/api/action-invitations/{invitation_id}/run", methodNotAllowed("POST"))
}

func validateAvailabilityInput(input agent.AvailabilityInput) error {
	switch input.Surface {
	case agent.SurfaceEditor, agent.SurfaceChapterView:
	default:
		return fmt.Errorf("surface must be one of %q or %q", agent.SurfaceEditor, agent.SurfaceChapterView)
	}
	switch input.InputScope {
	case agent.InputScopeSelection, agent.InputScopeScene, agent.InputScopeChapter, agent.InputScopeChapterReview:
	default:
		return fmt.Errorf("input_scope must be one of %q, %q, %q, or %q", agent.InputScopeSelection, agent.InputScopeScene, agent.InputScopeChapter, agent.InputScopeChapterReview)
	}
	if input.Surface == agent.SurfaceEditor && (input.InputScope == agent.InputScopeSelection || input.InputScope == agent.InputScopeScene) && strings.TrimSpace(input.SceneID) == "" {
		return fmt.Errorf("scene_id is required for editor availability")
	}
	return nil
}

func writeActionRunResponse(writer http.ResponseWriter, run action.Run) {
	response := map[string]any{
		"run_id":   run.RunID,
		"status":   run.Status,
		"agent_id": run.AgentID,
		"style_id": run.StyleID,
		"provider": run.Provider,
	}
	if run.Scope != "" {
		response["scope"] = run.Scope
		response["parent_run_id"] = nullableString(run.ParentRunID)
		if run.RootRunID != "" {
			response["root_run_id"] = run.RootRunID
		}
		if run.ChainDepth > 0 {
			response["chain_depth"] = run.ChainDepth
		}
		if len(run.Manifest.PacksUsed) > 0 || run.Manifest.Scope != "" {
			response["manifest"] = run.Manifest
		}
	}
	if run.SceneID != "" {
		response["scene_id"] = run.SceneID
		response["scene_revision"] = run.SceneRevision
	}
	if run.ChapterID != "" {
		response["chapter_id"] = run.ChapterID
		response["chapter_fingerprint"] = run.ChapterFingerprint
	}
	if run.Scope == contextpack.ScopeSelection || run.Scope == "" {
		response["output_mode"] = "patch"
		response["selection"] = map[string]int{
			"start_byte": run.Selection.StartByte,
			"end_byte":   run.Selection.EndByte,
		}
		response["patch"] = map[string]string{"original": run.OriginalText, "replacement": run.Replacement}
		response["context_summary"] = run.ContextSummary
	} else if run.Scope == contextpack.ScopeScene {
		response["output_mode"] = "patch"
		response["patch"] = map[string]string{"original": run.OriginalText, "replacement": run.Replacement}
		response["context_summary"] = run.ContextSummary
	} else if run.Status == action.RunCompleted || len(run.Findings) > 0 {
		response["output_mode"] = "suggestion"
		response["findings"] = run.Findings
	}
	if len(run.FollowUpInvitations) > 0 {
		response["follow_up_invitations"] = run.FollowUpInvitations
	}
	writeJSON(writer, http.StatusCreated, response)
}

func decodeTaggedRunRequest(writer http.ResponseWriter, request *http.Request) (action.TaggedRunRequest, bool, error) {
	var payload struct {
		AgentID string `json:"agent_id"`
		StyleID string `json:"style_id"`
		Scope   string `json:"scope"`
		Target  *struct {
			SceneID       *string `json:"scene_id"`
			SceneRevision *string `json:"scene_revision"`
			ChapterID     *string `json:"chapter_id"`
			Fingerprint   *string `json:"fingerprint"`
			StartByte     *int    `json:"start_byte"`
			EndByte       *int    `json:"end_byte"`
			Text          *string `json:"text"`
		} `json:"target"`
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
	if err := decodeJSONWithRequiredFields(writer, request, &payload, 1<<20,
		requiredJSONField{name: "agent_id"},
		requiredJSONField{name: "style_id"},
	); err != nil {
		writeBodyLimitError(writer, err)
		return action.TaggedRunRequest{}, false, err
	}
	if payload.Scope != "" {
		if payload.Target == nil {
			writeError(writer, http.StatusBadRequest, errors.New("target is required"))
			return action.TaggedRunRequest{}, true, errors.New("target is required")
		}
		target := action.TaggedTarget{Scope: contextpack.Scope(payload.Scope)}
		switch target.Scope {
		case contextpack.ScopeSelection:
			if payload.Target.SceneID == nil || payload.Target.SceneRevision == nil || payload.Target.StartByte == nil || payload.Target.EndByte == nil || payload.Target.Text == nil {
				writeError(writer, http.StatusBadRequest, errors.New("target scene_id, scene_revision, start_byte, end_byte, and text are required"))
				return action.TaggedRunRequest{}, true, errors.New("invalid selection target")
			}
			target.Selection = &action.SelectionTarget{
				SceneID: *payload.Target.SceneID, SceneRevision: *payload.Target.SceneRevision,
				StartByte: *payload.Target.StartByte, EndByte: *payload.Target.EndByte, SelectedText: *payload.Target.Text,
			}
		case contextpack.ScopeScene:
			if payload.Target.SceneID == nil || payload.Target.SceneRevision == nil {
				writeError(writer, http.StatusBadRequest, errors.New("target scene_id and scene_revision are required"))
				return action.TaggedRunRequest{}, true, errors.New("invalid scene target")
			}
			target.Scene = &action.SceneTarget{SceneID: *payload.Target.SceneID, SceneRevision: *payload.Target.SceneRevision}
		case contextpack.ScopeChapterReview:
			if payload.Target.ChapterID == nil || payload.Target.Fingerprint == nil {
				writeError(writer, http.StatusBadRequest, errors.New("target chapter_id and fingerprint are required"))
				return action.TaggedRunRequest{}, true, errors.New("invalid chapter target")
			}
			target.Chapter = &action.ChapterReviewTarget{ChapterID: *payload.Target.ChapterID, Fingerprint: *payload.Target.Fingerprint}
		default:
			writeError(writer, http.StatusBadRequest, fmt.Errorf("scope %q is unsupported", payload.Scope))
			return action.TaggedRunRequest{}, true, action.ErrInvalidRunRequest
		}
		request := action.TaggedRunRequest{AgentID: payload.AgentID, StyleID: payload.StyleID, Target: target}
		if err := action.ValidateTaggedRunRequest(request); err != nil {
			writeStoryError(writer, err)
			return action.TaggedRunRequest{}, true, err
		}
		return request, true, nil
	}
	if payload.Surface == "" || payload.InputScope == "" || payload.SceneID == "" || payload.SceneRevision == "" || payload.Selection == nil || payload.Selection.StartByte == nil || payload.Selection.EndByte == nil || payload.Selection.Text == nil {
		writeError(writer, http.StatusBadRequest, errors.New("scope or legacy selection fields are required"))
		return action.TaggedRunRequest{}, false, action.ErrInvalidRunRequest
	}
	legacy, err := action.NormalizeLegacyRunRequest(action.RunRequest{
		AgentID: payload.AgentID, StyleID: payload.StyleID,
		Surface: agent.Surface(payload.Surface), InputScope: agent.InputScope(payload.InputScope),
		SceneID: payload.SceneID, SceneRevision: payload.SceneRevision,
		Selection: action.Selection{StartByte: *payload.Selection.StartByte, EndByte: *payload.Selection.EndByte, Text: *payload.Selection.Text},
	})
	if err != nil {
		writeStoryError(writer, err)
		return action.TaggedRunRequest{}, false, err
	}
	return action.TaggedRunRequest{AgentID: payload.AgentID, StyleID: payload.StyleID, Target: legacy}, false, nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
