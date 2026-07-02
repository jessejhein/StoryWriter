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
	"os"
	"strconv"
	"strings"

	"storywork/internal/action"
	"storywork/internal/agent"
	"storywork/internal/codex"
	"storywork/internal/extract"
	"storywork/internal/importer"
	"storywork/internal/project"
	"storywork/internal/provider"
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

func newImportCandidateResponse(candidate importer.Candidate) importCandidateResponse {
	return importCandidateResponse{
		ID:                     candidate.ID,
		Kind:                   candidate.Kind,
		ProposalVersion:        candidate.ProposalVersion,
		Status:                 candidate.Status,
		Revision:               candidate.Revision,
		Provenance:             candidate.Provenance,
		Proposal:               candidateProposalResponse(candidate),
		ReplacementCandidateID: candidate.Decision.ReplacementCandidateID,
		CanonicalRefs:          candidate.Decision.CanonicalRefs,
	}
}

func candidateProposalResponse(candidate importer.Candidate) any {
	switch candidate.Kind {
	case importer.CandidateKindCodex:
		return candidate.Proposal.Codex
	case importer.CandidateKindArc:
		return candidate.Proposal.Arc
	case importer.CandidateKindChapter:
		return candidate.Proposal.Chapter
	case importer.CandidateKindScene:
		return candidate.Proposal.Scene
	default:
		return struct{}{}
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

func writeImportBodyError(writer http.ResponseWriter, err error) {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		writeError(writer, http.StatusRequestEntityTooLarge, errors.New("request body exceeds the 1 MiB limit"))
		return
	}
	writeError(writer, http.StatusBadRequest, err)
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
}

// ActionStore serves project-local agent registry and transient action runs.
type ActionStore interface {
	// Agents returns the strictly loaded project-local agent registry.
	Agents(ctx context.Context) ([]agent.Agent, error)
	// Styles returns the strictly loaded project-local style registry.
	Styles(ctx context.Context) ([]agent.Style, error)
	// AvailableActions returns the applicable actions for the current state.
	AvailableActions(ctx context.Context, input agent.AvailabilityInput) ([]action.AvailableAction, error)
	// Run executes one transient action against canonical scene bytes.
	Run(ctx context.Context, request action.RunRequest) (action.Run, error)
	// Accept applies one pending run to canonical scene markdown.
	Accept(ctx context.Context, runID, expectedRevision string) (action.Run, story.SceneDocument, error)
	// Reject discards one pending run without mutating canon.
	Reject(ctx context.Context, runID string) (action.Run, error)
}

// ProviderStore serves application-level provider profile configuration.
type ProviderStore interface {
	// ProviderProfiles returns the application-level provider profiles and revision.
	ProviderProfiles(ctx context.Context) ([]provider.Profile, *string, error)
	// SaveProviderProfiles replaces the application-level provider profiles.
	SaveProviderProfiles(ctx context.Context, profiles []provider.Profile, expectedRevision *string) ([]provider.Profile, *string, error)
}

// ImportStore serves Markdown import snapshots and review candidates.
type ImportStore interface {
	// ImportDirectory snapshots a source directory into the active project.
	ImportDirectory(ctx context.Context, sourceDirectory string) (importer.ImportResponse, error)
	// ListImports returns imported snapshot summaries.
	ListImports(ctx context.Context) ([]importer.ImportSummary, error)
	// LoadImport returns one imported snapshot and its file manifest.
	LoadImport(ctx context.Context, importID string) (importer.ImportResponse, error)
	// ListImportChunks returns one import's deterministic chunks.
	ListImportChunks(ctx context.Context, importID string) ([]importer.Chunk, error)
	// ExtractImport runs provider-neutral extraction against selected chunks.
	ExtractImport(ctx context.Context, request importer.ExtractRequest) (importer.ExtractResponse, error)
	// ListImportCandidates lists durable review candidates with optional filters.
	ListImportCandidates(ctx context.Context, status *importer.CandidateStatus, kind *importer.CandidateKind) ([]importer.Candidate, error)
	// LoadImportCandidate loads one durable review candidate.
	LoadImportCandidate(ctx context.Context, candidateID string) (importer.Candidate, error)
	// UpdateImportCandidate edits one pending candidate.
	UpdateImportCandidate(ctx context.Context, candidateID, expectedRevision string, proposal importer.CandidateProposal) (importer.Candidate, error)
	// MergeImportCandidates merges two compatible pending candidates.
	MergeImportCandidates(ctx context.Context, candidateID string, request importer.MergeRequest) (importer.Candidate, []string, error)
	// DiscardImportCandidate marks one pending candidate as discarded.
	DiscardImportCandidate(ctx context.Context, candidateID, expectedRevision string) (importer.Candidate, error)
	// AcceptImportCandidate writes one candidate into canon atomically.
	AcceptImportCandidate(ctx context.Context, candidateID, expectedRevision string) (importer.Candidate, []importer.CanonicalRef, error)
}

// HandlerDependencies groups the cohesive HTTP handler boundaries.
type HandlerDependencies struct {
	Projects  ProjectStore
	Session   ActiveProjectSession
	Stories   StoryStore
	Actions   ActionStore
	Providers ProviderStore
	Imports   ImportStore
	Version   string
}

type importCandidateResponse struct {
	ID                     string                   `json:"id"`
	Kind                   importer.CandidateKind   `json:"kind"`
	ProposalVersion        int                      `json:"proposal_version"`
	Status                 importer.CandidateStatus `json:"status"`
	Revision               string                   `json:"revision"`
	Provenance             importer.Provenance      `json:"provenance"`
	Proposal               any                      `json:"proposal"`
	ReplacementCandidateID *string                  `json:"replacement_candidate_id"`
	CanonicalRefs          []importer.CanonicalRef  `json:"canonical_refs"`
}

type requiredJSONField struct {
	name      string
	allowNull bool
}

type providerProfileRequest struct {
	ID           *string                      `json:"id"`
	Name         *string                      `json:"name"`
	Type         *provider.Type               `json:"type"`
	BaseURL      *string                      `json:"base_url"`
	Auth         *providerAuthRequest         `json:"auth"`
	Capabilities *providerCapabilitiesRequest `json:"capabilities"`
}

type providerAuthRequest struct {
	Type          *provider.AuthType `json:"type"`
	CredentialEnv *string            `json:"credential_env"`
}

type providerCapabilitiesRequest struct {
	Chat             *bool `json:"chat"`
	Streaming        *bool `json:"streaming"`
	StructuredOutput *bool `json:"structured_output"`
	MaxContextTokens *int  `json:"max_context_tokens"`
}

// NewHandler creates the full local Storywork HTTP router for the current milestone set.
func NewHandler(deps HandlerDependencies) http.Handler {
	projects := deps.Projects
	session := deps.Session
	stories := deps.Stories
	actionStore := deps.Actions
	providers := deps.Providers
	importStore := deps.Imports
	version := deps.Version

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
	mux.HandleFunc("GET /api/provider-profiles", func(writer http.ResponseWriter, request *http.Request) {
		profiles, revision, err := providers.ProviderProfiles(request.Context())
		if err != nil {
			writeProviderError(writer, err)
			return
		}
		if profiles == nil {
			profiles = []provider.Profile{}
		}
		writeJSON(writer, http.StatusOK, map[string]any{
			"profiles": profiles,
			"revision": revision,
		})
	})
	mux.HandleFunc("PUT /api/provider-profiles", func(writer http.ResponseWriter, request *http.Request) {
		var putRequest struct {
			Profiles         *[]providerProfileRequest `json:"profiles"`
			ExpectedRevision *string                   `json:"expected_revision"`
		}
		if err := decodeJSONWithRequiredFields(writer, request, &putRequest, 1<<20,
			requiredJSONField{name: "profiles"},
			requiredJSONField{name: "expected_revision", allowNull: true},
		); err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				writeError(writer, http.StatusRequestEntityTooLarge, errors.New("request body exceeds the 1 MiB limit"))
				return
			}
			writeError(writer, http.StatusBadRequest, err)
			return
		}
		if putRequest.Profiles == nil {
			writeError(writer, http.StatusBadRequest, errors.New("profiles is required"))
			return
		}
		if putRequest.ExpectedRevision != nil {
			if err := provider.ValidateRevision(*putRequest.ExpectedRevision); err != nil {
				writeError(writer, http.StatusBadRequest, err)
				return
			}
		}
		domainProfiles, err := decodeProviderProfiles(*putRequest.Profiles)
		if err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}
		profiles, revision, err := providers.SaveProviderProfiles(request.Context(), domainProfiles, putRequest.ExpectedRevision)
		if err != nil {
			writeProviderError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, map[string]any{
			"profiles": profiles,
			"revision": revision,
		})
	})
	mux.HandleFunc("POST /api/imports", func(writer http.ResponseWriter, request *http.Request) {
		var importRequest struct {
			SourceDirectory string `json:"source_directory"`
		}
		if err := decodeJSONWithRequiredFields(writer, request, &importRequest, 1<<20, requiredJSONField{name: "source_directory"}); err != nil {
			writeImportBodyError(writer, err)
			return
		}
		response, err := importStore.ImportDirectory(request.Context(), importRequest.SourceDirectory)
		if err != nil {
			writeImportError(writer, err)
			return
		}
		writeJSON(writer, http.StatusCreated, response)
	})
	mux.HandleFunc("GET /api/imports", func(writer http.ResponseWriter, request *http.Request) {
		if err := validateExactQuery(request); err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}
		importSummaries, err := importStore.ListImports(request.Context())
		if err != nil {
			writeImportError(writer, err)
			return
		}
		if importSummaries == nil {
			importSummaries = []importer.ImportSummary{}
		}
		writeJSON(writer, http.StatusOK, map[string]any{"imports": importSummaries})
	})
	mux.HandleFunc("GET /api/imports/{import_id}", func(writer http.ResponseWriter, request *http.Request) {
		if err := importer.ValidateImportID(request.PathValue("import_id")); err != nil {
			writeImportError(writer, err)
			return
		}
		if err := validateExactQuery(request); err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}
		response, err := importStore.LoadImport(request.Context(), request.PathValue("import_id"))
		if err != nil {
			writeImportError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, response)
	})
	mux.HandleFunc("GET /api/imports/{import_id}/chunks", func(writer http.ResponseWriter, request *http.Request) {
		if err := importer.ValidateImportID(request.PathValue("import_id")); err != nil {
			writeImportError(writer, err)
			return
		}
		if err := validateExactQuery(request); err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}
		chunks, err := importStore.ListImportChunks(request.Context(), request.PathValue("import_id"))
		if err != nil {
			writeImportError(writer, err)
			return
		}
		if chunks == nil {
			chunks = []importer.Chunk{}
		}
		writeJSON(writer, http.StatusOK, map[string]any{"chunks": chunks})
	})
	mux.HandleFunc("POST /api/imports/{import_id}/extractions", func(writer http.ResponseWriter, request *http.Request) {
		if err := importer.ValidateImportID(request.PathValue("import_id")); err != nil {
			writeImportError(writer, err)
			return
		}
		var extractionRequest struct {
			ChunkIDs  []string     `json:"chunk_ids"`
			Mode      extract.Mode `json:"mode"`
			ProfileID string       `json:"profile_id"`
			Model     string       `json:"model"`
		}
		if err := decodeJSONWithRequiredFields(writer, request, &extractionRequest, 1<<20,
			requiredJSONField{name: "chunk_ids"},
			requiredJSONField{name: "mode"},
			requiredJSONField{name: "profile_id"},
			requiredJSONField{name: "model"},
		); err != nil {
			writeImportBodyError(writer, err)
			return
		}
		extraction, err := importStore.ExtractImport(request.Context(), importer.ExtractRequest{
			ImportID:  request.PathValue("import_id"),
			ChunkIDs:  extractionRequest.ChunkIDs,
			Mode:      extractionRequest.Mode,
			ProfileID: extractionRequest.ProfileID,
			Model:     extractionRequest.Model,
		})
		if err != nil {
			writeImportError(writer, err)
			return
		}
		response := make([]importCandidateResponse, 0, len(extraction.Candidates))
		for _, candidate := range extraction.Candidates {
			response = append(response, newImportCandidateResponse(candidate))
		}
		writeJSON(writer, http.StatusCreated, map[string]any{"candidates": response, "provider": extraction.Provider})
	})
	mux.HandleFunc("GET /api/import-candidates", func(writer http.ResponseWriter, request *http.Request) {
		status, kind, err := parseCandidateFilters(request.URL.Query())
		if err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}
		candidates, err := importStore.ListImportCandidates(request.Context(), status, kind)
		if err != nil {
			writeImportError(writer, err)
			return
		}
		response := make([]importCandidateResponse, 0, len(candidates))
		for _, candidate := range candidates {
			response = append(response, newImportCandidateResponse(candidate))
		}
		writeJSON(writer, http.StatusOK, map[string]any{"candidates": response})
	})
	mux.HandleFunc("GET /api/import-candidates/{candidate_id}", func(writer http.ResponseWriter, request *http.Request) {
		if err := importer.ValidateCandidateID(request.PathValue("candidate_id")); err != nil {
			writeImportError(writer, err)
			return
		}
		candidate, err := importStore.LoadImportCandidate(request.Context(), request.PathValue("candidate_id"))
		if err != nil {
			writeImportError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, newImportCandidateResponse(candidate))
	})
	mux.HandleFunc("PUT /api/import-candidates/{candidate_id}", func(writer http.ResponseWriter, request *http.Request) {
		if err := importer.ValidateCandidateID(request.PathValue("candidate_id")); err != nil {
			writeImportError(writer, err)
			return
		}
		body, err := readJSONBodyWithLimit(writer, request, 1<<20)
		if err != nil {
			writeImportBodyError(writer, err)
			return
		}
		var editEnvelope struct {
			Proposal         json.RawMessage `json:"proposal"`
			ExpectedRevision string          `json:"expected_revision"`
		}
		if err := requireJSONFields(body, requiredJSONField{name: "proposal"}, requiredJSONField{name: "expected_revision"}); err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}
		if err := decodeJSONBytes(body, &editEnvelope); err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}
		if err := importer.ValidateCandidateRevision(editEnvelope.ExpectedRevision); err != nil {
			writeImportError(writer, err)
			return
		}
		current, err := importStore.LoadImportCandidate(request.Context(), request.PathValue("candidate_id"))
		if err != nil {
			writeImportError(writer, err)
			return
		}
		proposal, err := decodeCandidateProposalJSON(current.Kind, editEnvelope.Proposal)
		if err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}
		candidate, err := importStore.UpdateImportCandidate(request.Context(), current.ID, editEnvelope.ExpectedRevision, proposal)
		if err != nil {
			writeImportError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, newImportCandidateResponse(candidate))
	})
	mux.HandleFunc("POST /api/import-candidates/{candidate_id}/merge", func(writer http.ResponseWriter, request *http.Request) {
		if err := importer.ValidateCandidateID(request.PathValue("candidate_id")); err != nil {
			writeImportError(writer, err)
			return
		}
		body, err := readJSONBodyWithLimit(writer, request, 1<<20)
		if err != nil {
			writeImportBodyError(writer, err)
			return
		}
		var mergeEnvelope struct {
			OtherCandidateID      string          `json:"other_candidate_id"`
			ExpectedRevision      string          `json:"expected_revision"`
			OtherExpectedRevision string          `json:"other_expected_revision"`
			Proposal              json.RawMessage `json:"proposal"`
		}
		if err := requireJSONFields(body,
			requiredJSONField{name: "other_candidate_id"},
			requiredJSONField{name: "expected_revision"},
			requiredJSONField{name: "other_expected_revision"},
			requiredJSONField{name: "proposal"},
		); err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}
		if err := decodeJSONBytes(body, &mergeEnvelope); err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}
		if err := importer.ValidateCandidateID(mergeEnvelope.OtherCandidateID); err != nil {
			writeImportError(writer, err)
			return
		}
		if err := importer.ValidateCandidateRevision(mergeEnvelope.ExpectedRevision); err != nil {
			writeImportError(writer, err)
			return
		}
		if err := importer.ValidateCandidateRevision(mergeEnvelope.OtherExpectedRevision); err != nil {
			writeImportError(writer, err)
			return
		}
		current, err := importStore.LoadImportCandidate(request.Context(), request.PathValue("candidate_id"))
		if err != nil {
			writeImportError(writer, err)
			return
		}
		proposal, err := decodeCandidateProposalJSON(current.Kind, mergeEnvelope.Proposal)
		if err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}
		candidate, mergedIDs, err := importStore.MergeImportCandidates(request.Context(), current.ID, importer.MergeRequest{
			OtherCandidateID:      mergeEnvelope.OtherCandidateID,
			ExpectedRevision:      mergeEnvelope.ExpectedRevision,
			OtherExpectedRevision: mergeEnvelope.OtherExpectedRevision,
			Proposal:              proposal,
		})
		if err != nil {
			writeImportError(writer, err)
			return
		}
		writeJSON(writer, http.StatusCreated, map[string]any{"candidate": newImportCandidateResponse(candidate), "merged_candidate_ids": mergedIDs})
	})
	mux.HandleFunc("POST /api/import-candidates/{candidate_id}/discard", func(writer http.ResponseWriter, request *http.Request) {
		if err := importer.ValidateCandidateID(request.PathValue("candidate_id")); err != nil {
			writeImportError(writer, err)
			return
		}
		var discardRequest struct {
			ExpectedRevision string `json:"expected_revision"`
		}
		if err := decodeJSONWithRequiredFields(writer, request, &discardRequest, 1<<20, requiredJSONField{name: "expected_revision"}); err != nil {
			writeImportBodyError(writer, err)
			return
		}
		if err := importer.ValidateCandidateRevision(discardRequest.ExpectedRevision); err != nil {
			writeImportError(writer, err)
			return
		}
		candidate, err := importStore.DiscardImportCandidate(request.Context(), request.PathValue("candidate_id"), discardRequest.ExpectedRevision)
		if err != nil {
			writeImportError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, newImportCandidateResponse(candidate))
	})
	mux.HandleFunc("POST /api/import-candidates/{candidate_id}/accept", func(writer http.ResponseWriter, request *http.Request) {
		if err := importer.ValidateCandidateID(request.PathValue("candidate_id")); err != nil {
			writeImportError(writer, err)
			return
		}
		var acceptRequest struct {
			ExpectedRevision string `json:"expected_revision"`
		}
		if err := decodeJSONWithRequiredFields(writer, request, &acceptRequest, 1<<20, requiredJSONField{name: "expected_revision"}); err != nil {
			writeImportBodyError(writer, err)
			return
		}
		if err := importer.ValidateCandidateRevision(acceptRequest.ExpectedRevision); err != nil {
			writeImportError(writer, err)
			return
		}
		candidate, canonicalRefs, err := importStore.AcceptImportCandidate(request.Context(), request.PathValue("candidate_id"), acceptRequest.ExpectedRevision)
		if err != nil {
			writeImportError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, map[string]any{"candidate": newImportCandidateResponse(candidate), "canonical_refs": canonicalRefs})
	})
	mux.HandleFunc("/api/imports", methodNotAllowed("GET, POST"))
	mux.HandleFunc("/api/imports/{import_id}", methodNotAllowed("GET"))
	mux.HandleFunc("/api/imports/{import_id}/chunks", methodNotAllowed("GET"))
	mux.HandleFunc("/api/imports/{import_id}/extractions", methodNotAllowed("POST"))
	mux.HandleFunc("/api/import-candidates", methodNotAllowed("GET"))
	mux.HandleFunc("/api/import-candidates/{candidate_id}", methodNotAllowed("GET, PUT"))
	mux.HandleFunc("/api/import-candidates/{candidate_id}/merge", methodNotAllowed("POST"))
	mux.HandleFunc("/api/import-candidates/{candidate_id}/discard", methodNotAllowed("POST"))
	mux.HandleFunc("/api/import-candidates/{candidate_id}/accept", methodNotAllowed("POST"))
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
		agents, err := actionStore.Agents(request.Context())
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
		styles, err := actionStore.Styles(request.Context())
		if err != nil {
			writeStoryError(writer, err)
			return
		}
		profiles, _, err := providers.ProviderProfiles(request.Context())
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
		availableActions, err := actionStore.AvailableActions(request.Context(), input)
		if err != nil {
			writeStoryError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, map[string]any{"actions": availableActions})
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
		run, err := actionStore.Run(request.Context(), domainRequest)
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
			"provider":        run.Provider,
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
		run, sceneDocument, err := actionStore.Accept(request.Context(), request.PathValue("run_id"), acceptRequest.ExpectedRevision)
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
		run, err := actionStore.Reject(request.Context(), request.PathValue("run_id"))
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
	mux.HandleFunc("/api/provider-profiles", methodNotAllowed("GET, PUT"))
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

func decodeProviderProfiles(requests []providerProfileRequest) ([]provider.Profile, error) {
	profiles := make([]provider.Profile, 0, len(requests))
	for index, item := range requests {
		if item.ID == nil || item.Name == nil || item.Type == nil || item.BaseURL == nil || item.Auth == nil || item.Capabilities == nil {
			return nil, fmt.Errorf("profile %d requires id, name, type, base_url, auth, and capabilities", index+1)
		}
		if item.Auth.Type == nil || item.Auth.CredentialEnv == nil {
			return nil, fmt.Errorf("profile %d auth requires type and credential_env", index+1)
		}
		capabilities := item.Capabilities
		if capabilities.Chat == nil || capabilities.Streaming == nil || capabilities.StructuredOutput == nil || capabilities.MaxContextTokens == nil {
			return nil, fmt.Errorf("profile %d capabilities requires chat, streaming, structured_output, and max_context_tokens", index+1)
		}
		profiles = append(profiles, provider.Profile{
			ID:      *item.ID,
			Name:    *item.Name,
			Type:    *item.Type,
			BaseURL: *item.BaseURL,
			Auth: provider.AuthConfig{
				Type:          *item.Auth.Type,
				CredentialEnv: *item.Auth.CredentialEnv,
			},
			Capabilities: provider.Capabilities{
				Chat:             *capabilities.Chat,
				Streaming:        *capabilities.Streaming,
				StructuredOutput: *capabilities.StructuredOutput,
				MaxContextTokens: *capabilities.MaxContextTokens,
			},
		})
	}
	return profiles, nil
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

func parseCandidateFilters(values map[string][]string) (*importer.CandidateStatus, *importer.CandidateKind, error) {
	var status *importer.CandidateStatus
	if raw, ok := values["status"]; ok {
		if len(raw) != 1 || strings.TrimSpace(raw[0]) == "" {
			return nil, nil, errors.New("invalid candidate status filter")
		}
		parsed, err := importer.ValidateCandidateStatus(raw[0])
		if err != nil {
			return nil, nil, errors.New("invalid candidate status filter")
		}
		status = &parsed
	}
	var kind *importer.CandidateKind
	if raw, ok := values["kind"]; ok {
		if len(raw) != 1 || strings.TrimSpace(raw[0]) == "" {
			return nil, nil, errors.New("invalid candidate kind filter")
		}
		parsed, err := importer.ValidateCandidateKind(raw[0])
		if err != nil {
			return nil, nil, errors.New("invalid candidate kind filter")
		}
		kind = &parsed
	}
	for key := range values {
		if key != "status" && key != "kind" {
			return nil, nil, errors.New("invalid candidate filter")
		}
	}
	return status, kind, nil
}

func decodeCandidateProposalJSON(kind importer.CandidateKind, raw json.RawMessage) (importer.CandidateProposal, error) {
	switch kind {
	case importer.CandidateKindCodex:
		if err := requireJSONFields(raw,
			requiredJSONField{name: "type"},
			requiredJSONField{name: "name"},
			requiredJSONField{name: "aliases"},
			requiredJSONField{name: "tags"},
			requiredJSONField{name: "description"},
		); err != nil {
			return importer.CandidateProposal{}, err
		}
		var proposal importer.CodexProposal
		if err := decodeJSONBytes(raw, &proposal); err != nil {
			return importer.CandidateProposal{}, err
		}
		return importer.CandidateProposal{Codex: &proposal}, nil
	case importer.CandidateKindArc:
		if err := requireJSONFields(raw, requiredJSONField{name: "title"}); err != nil {
			return importer.CandidateProposal{}, err
		}
		var proposal importer.ArcProposal
		if err := decodeJSONBytes(raw, &proposal); err != nil {
			return importer.CandidateProposal{}, err
		}
		return importer.CandidateProposal{Arc: &proposal}, nil
	case importer.CandidateKindChapter:
		if err := requireJSONFields(raw, requiredJSONField{name: "title"}, requiredJSONField{name: "parent_candidate_id"}); err != nil {
			return importer.CandidateProposal{}, err
		}
		var proposal importer.ChapterProposal
		if err := decodeJSONBytes(raw, &proposal); err != nil {
			return importer.CandidateProposal{}, err
		}
		return importer.CandidateProposal{Chapter: &proposal}, nil
	case importer.CandidateKindScene:
		if err := requireJSONFields(raw, requiredJSONField{name: "title"}, requiredJSONField{name: "parent_candidate_id"}); err != nil {
			return importer.CandidateProposal{}, err
		}
		var proposal importer.SceneProposal
		if err := decodeJSONBytes(raw, &proposal); err != nil {
			return importer.CandidateProposal{}, err
		}
		return importer.CandidateProposal{Scene: &proposal}, nil
	default:
		return importer.CandidateProposal{}, errors.New("unsupported candidate kind")
	}
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

func writeImportError(writer http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, story.ErrNoActiveProject), errors.Is(err, story.ErrDirtyWorktree), errors.Is(err, importer.ErrCandidateConflict), errors.Is(err, importer.ErrCandidateTerminal), errors.Is(err, importer.ErrParentNotAccepted):
		writeError(writer, http.StatusConflict, errors.New("import operation conflicts with current project state"))
	case errors.Is(err, importer.ErrInvalidSourceDirectory), errors.Is(err, importer.ErrNoEligibleFiles), errors.Is(err, importer.ErrSymlinkRefused), errors.Is(err, importer.ErrSourceChanged), errors.Is(err, importer.ErrInvalidContent), errors.Is(err, importer.ErrInvalidCandidate), errors.Is(err, importer.ErrInvalidID), errors.Is(err, importer.ErrInvalidPath), errors.Is(err, importer.ErrNoCandidateChanges), errors.Is(err, extract.ErrInvalidRequest):
		writeError(writer, http.StatusBadRequest, errors.New("invalid import request"))
	case errors.Is(err, extract.ErrInvalidResponse), errors.Is(err, agent.ErrProviderRejected):
		writeError(writer, http.StatusBadGateway, errors.New("extraction provider returned an invalid response"))
	case errors.Is(err, agent.ErrProviderOffline), errors.Is(err, agent.ErrProviderInvalid):
		writeError(writer, http.StatusServiceUnavailable, errors.New("extraction provider is unavailable"))
	case errors.Is(err, os.ErrNotExist):
		writeError(writer, http.StatusNotFound, errors.New("import resource not found"))
	default:
		writeError(writer, http.StatusInternalServerError, errors.New("import operation failed"))
	}
}

func writeProviderError(writer http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, provider.ErrInvalidProfile), errors.Is(err, provider.ErrNoProfileChanges):
		writeError(writer, http.StatusBadRequest, err)
	case errors.Is(err, provider.ErrProfileRevisionConflict):
		writeError(writer, http.StatusConflict, err)
	default:
		writeError(writer, http.StatusInternalServerError, err)
	}
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
	case errors.Is(err, story.ErrInvalidSelection), errors.Is(err, agent.ErrInvalidAgent), errors.Is(err, agent.ErrInvalidStyle), errors.Is(err, agent.ErrInapplicable), errors.Is(err, action.ErrInvalidRunRequest), errors.Is(err, action.ErrProviderInvalid):
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
	case errors.Is(err, action.ErrProviderRejected):
		return http.StatusBadGateway
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
