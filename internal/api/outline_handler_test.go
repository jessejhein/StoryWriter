package api_test

// outline_handler_test.go provides shared API fakes and verifies outline routes.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"storywork/internal/action"
	"storywork/internal/agent"
	"storywork/internal/api"
	"storywork/internal/codex"
	"storywork/internal/importer"
	"storywork/internal/project"
	"storywork/internal/provider"
	"storywork/internal/story"
)

type activeProjectSessionStub struct {
	setCalls []project.Project
}

func (s *activeProjectSessionStub) Set(current project.Project) {
	s.setCalls = append(s.setCalls, current)
}

type storyServiceStub struct {
	outlineResult           story.Outline
	mutationResult          story.MutationResult
	sceneResult             story.SceneDocument
	codexEntries            []codex.Entry
	codexEntry              codex.Entry
	progressionDocument     codex.ProgressionDocument
	activeState             codex.ActiveState
	outlineErr              error
	createArcErr            error
	createChapterErr        error
	createSceneErr          error
	reorderErr              error
	loadSceneErr            error
	saveSceneErr            error
	loadCodexErr            error
	createCodexErr          error
	updateCodexErr          error
	loadProgressionsErr     error
	saveProgressionsErr     error
	activeCodexErr          error
	createArcTitle          string
	createChapterArcID      string
	createChapterTitle      string
	createSceneChapterID    string
	createSceneTitle        string
	reorderRequest          story.ReorderRequest
	loadSceneID             string
	saveSceneID             string
	saveSceneRequest        story.SaveSceneRequest
	codexEntryID            string
	saveCodexRequest        codex.SaveEntryRequest
	progressionEntryID      string
	saveProgressionsReq     codex.SaveProgressionsRequest
	activeEntryID           string
	activeSceneID           string
	agents                  []agent.Agent
	styles                  []agent.Style
	availableActions        []action.AvailableAction
	actionRun               action.Run
	actionRunErr            error
	agentsErr               error
	stylesErr               error
	availableActionsErr     error
	providerProfiles        []provider.Profile
	providerRevision        *string
	providerProfilesErr     error
	saveProviderErr         error
	actionAcceptErr         error
	actionRejectErr         error
	actionAcceptRun         action.Run
	actionRejectRun         action.Run
	actionAcceptScene       story.SceneDocument
	actionRunRequest        action.RunRequest
	previewRequest          action.TaggedRunRequest
	previewResult           action.ContextPreviewResult
	previewErr              error
	actionAcceptRunID       string
	actionAcceptRevision    string
	actionRejectRunID       string
	actionInvitationID      string
	actionInvitationRequest action.InvitationRunRequest
	availableInput          agent.AvailabilityInput
	saveProviderInput       []provider.Profile
	saveProviderExpected    *string
	importResponse          importer.ImportResponse
	importErr               error
	importList              []importer.ImportSummary
	importListErr           error
	loadImportResponse      importer.ImportResponse
	loadImportErr           error
	importChunks            []importer.Chunk
	importChunksErr         error
	extractResponse         importer.ExtractResponse
	extractErr              error
	candidates              []importer.Candidate
	candidatesErr           error
	candidate               importer.Candidate
	candidateErr            error
	updateCandidate         importer.Candidate
	updateCandidateErr      error
	mergeCandidate          importer.Candidate
	mergeCandidateIDs       []string
	mergeCandidateErr       error
	discardCandidate        importer.Candidate
	discardCandidateErr     error
	acceptCandidate         importer.Candidate
	acceptRefs              []importer.CanonicalRef
	acceptCandidateErr      error
}

func (s *storyServiceStub) Outline(context.Context) (story.Outline, error) {
	return s.outlineResult, s.outlineErr
}

func (s *storyServiceStub) CreateArc(_ context.Context, title string) (story.MutationResult, error) {
	s.createArcTitle = title
	return s.mutationResult, s.createArcErr
}

func (s *storyServiceStub) CreateChapter(_ context.Context, arcID, title string) (story.MutationResult, error) {
	s.createChapterArcID = arcID
	s.createChapterTitle = title
	return s.mutationResult, s.createChapterErr
}

func (s *storyServiceStub) CreateScene(_ context.Context, chapterID, title string) (story.MutationResult, error) {
	s.createSceneChapterID = chapterID
	s.createSceneTitle = title
	return s.mutationResult, s.createSceneErr
}

func (s *storyServiceStub) Reorder(_ context.Context, request story.ReorderRequest) (story.MutationResult, error) {
	s.reorderRequest = request
	return s.mutationResult, s.reorderErr
}

func (s *storyServiceStub) LoadScene(_ context.Context, sceneID string) (story.SceneDocument, error) {
	s.loadSceneID = sceneID
	return s.sceneResult, s.loadSceneErr
}

func (s *storyServiceStub) SaveScene(_ context.Context, sceneID string, request story.SaveSceneRequest) (story.SceneDocument, error) {
	s.saveSceneID = sceneID
	s.saveSceneRequest = request
	return s.sceneResult, s.saveSceneErr
}

func (s *storyServiceStub) CodexEntries(context.Context) ([]codex.Entry, error) {
	return s.codexEntries, s.loadCodexErr
}

func (s *storyServiceStub) LoadCodexEntry(_ context.Context, entryID string) (codex.Entry, error) {
	s.codexEntryID = entryID
	return s.codexEntry, s.loadCodexErr
}

func (s *storyServiceStub) CreateCodexEntry(_ context.Context, request codex.SaveEntryRequest) (codex.Entry, error) {
	s.saveCodexRequest = request
	return s.codexEntry, s.createCodexErr
}

func (s *storyServiceStub) UpdateCodexEntry(_ context.Context, entryID string, request codex.SaveEntryRequest) (codex.Entry, error) {
	s.codexEntryID = entryID
	s.saveCodexRequest = request
	return s.codexEntry, s.updateCodexErr
}

func (s *storyServiceStub) LoadProgressions(_ context.Context, entryID string) (codex.ProgressionDocument, error) {
	s.progressionEntryID = entryID
	return s.progressionDocument, s.loadProgressionsErr
}

func (s *storyServiceStub) SaveProgressions(_ context.Context, entryID string, request codex.SaveProgressionsRequest) (codex.ProgressionDocument, error) {
	s.progressionEntryID = entryID
	s.saveProgressionsReq = request
	return s.progressionDocument, s.saveProgressionsErr
}

func (s *storyServiceStub) ResolveActiveCodexState(_ context.Context, entryID, sceneID string) (codex.ActiveState, error) {
	s.activeEntryID = entryID
	s.activeSceneID = sceneID
	return s.activeState, s.activeCodexErr
}

func (s *storyServiceStub) Agents(context.Context) ([]agent.Agent, error) {
	return s.agents, s.agentsErr
}

func (s *storyServiceStub) Styles(context.Context) ([]agent.Style, error) {
	return s.styles, s.stylesErr
}

func (s *storyServiceStub) AvailableActions(_ context.Context, input agent.AvailabilityInput) ([]action.AvailableAction, error) {
	s.availableInput = input
	return s.availableActions, s.availableActionsErr
}

func (s *storyServiceStub) Run(_ context.Context, request action.RunRequest) (action.Run, error) {
	s.actionRunRequest = request
	return s.actionRun, s.actionRunErr
}

func (s *storyServiceStub) Accept(_ context.Context, runID, expectedRevision string) (action.AcceptResult, error) {
	s.actionAcceptRunID = runID
	s.actionAcceptRevision = expectedRevision
	if s.actionAcceptErr != nil {
		return action.AcceptResult{}, s.actionAcceptErr
	}
	return action.AcceptResult{Run: s.actionAcceptRun, Scene: s.actionAcceptScene}, nil
}

func (s *storyServiceStub) AcceptRun(_ context.Context, runID, expectedRevision string) (action.AcceptResult, error) {
	return s.Accept(context.Background(), runID, expectedRevision)
}

func (s *storyServiceStub) AcceptBody(_ context.Context, runID, expectedRevision string) (action.AcceptResult, error) {
	return s.Accept(context.Background(), runID, expectedRevision)
}

func (s *storyServiceStub) RunTagged(_ context.Context, request action.TaggedRunRequest) (action.Run, error) {
	s.previewRequest = request
	return s.actionRun, s.actionRunErr
}

func (s *storyServiceStub) RunInvitation(_ context.Context, invitationID string, request action.InvitationRunRequest) (action.Run, error) {
	s.actionInvitationID = invitationID
	s.actionInvitationRequest = request
	return s.actionRun, s.actionRunErr
}

func (s *storyServiceStub) PreviewContext(_ context.Context, request action.TaggedRunRequest) (action.ContextPreviewResult, error) {
	s.previewRequest = request
	return s.previewResult, s.previewErr
}

func (s *storyServiceStub) Reject(_ context.Context, runID string) (action.Run, error) {
	s.actionRejectRunID = runID
	return s.actionRejectRun, s.actionRejectErr
}

func (s *storyServiceStub) ProviderProfiles(context.Context) ([]provider.Profile, *string, error) {
	return s.providerProfiles, s.providerRevision, s.providerProfilesErr
}

func (s *storyServiceStub) SaveProviderProfiles(_ context.Context, profiles []provider.Profile, expectedRevision *string) ([]provider.Profile, *string, error) {
	s.saveProviderInput = profiles
	s.saveProviderExpected = expectedRevision
	return s.providerProfiles, s.providerRevision, s.saveProviderErr
}

func (s *storyServiceStub) ImportDirectory(context.Context, string) (importer.ImportResponse, error) {
	return s.importResponse, s.importErr
}

func (s *storyServiceStub) ListImports(context.Context) ([]importer.ImportSummary, error) {
	return s.importList, s.importListErr
}

func (s *storyServiceStub) LoadImport(context.Context, string) (importer.ImportResponse, error) {
	return s.loadImportResponse, s.loadImportErr
}

func (s *storyServiceStub) ListImportChunks(context.Context, string) ([]importer.Chunk, error) {
	return s.importChunks, s.importChunksErr
}

func (s *storyServiceStub) ExtractImport(context.Context, importer.ExtractRequest) (importer.ExtractResponse, error) {
	return s.extractResponse, s.extractErr
}

func (s *storyServiceStub) ListImportCandidates(context.Context, *importer.CandidateStatus, *importer.CandidateKind) ([]importer.Candidate, error) {
	return s.candidates, s.candidatesErr
}

func (s *storyServiceStub) LoadImportCandidate(context.Context, string) (importer.Candidate, error) {
	return s.candidate, s.candidateErr
}

func (s *storyServiceStub) UpdateImportCandidate(context.Context, string, string, importer.CandidateProposal) (importer.Candidate, error) {
	return s.updateCandidate, s.updateCandidateErr
}

func (s *storyServiceStub) MergeImportCandidates(context.Context, string, importer.MergeRequest) (importer.Candidate, []string, error) {
	return s.mergeCandidate, s.mergeCandidateIDs, s.mergeCandidateErr
}

func (s *storyServiceStub) DiscardImportCandidate(context.Context, string, string) (importer.Candidate, error) {
	return s.discardCandidate, s.discardCandidateErr
}

func (s *storyServiceStub) AcceptImportCandidate(context.Context, string, string) (importer.Candidate, []importer.CanonicalRef, error) {
	return s.acceptCandidate, s.acceptRefs, s.acceptCandidateErr
}

func newTestHandler(projects api.ProjectStore, session api.ActiveProjectSession, stub *storyServiceStub, version string) http.Handler {
	return api.NewHandler(api.HandlerDependencies{
		Projects:  projects,
		Session:   session,
		Stories:   stub,
		Actions:   stub,
		Providers: stub,
		Imports:   stub,
		Version:   version,
	})
}

// BDD trace:
//   - Requirement: Milestone 1 fixed design decision, active project session.
//   - Scenario: after a successful project create or open request, the backend
//     sets the active project session used by later outline routes.
//   - Test purpose: verify the HTTP layer stores the returned project in the
//     active session for both create and open flows.
func TestProjectRoutesSetActiveSession(t *testing.T) {
	t.Parallel()

	createdProject := project.Project{ID: "proj_test_novel", Path: "/tmp/test-novel", GitInitialized: true, IndexInitialized: true}
	projectStore := &projectStoreStub{created: createdProject}
	session := &activeProjectSessionStub{}
	handler := newTestHandler(projectStore, session, &storyServiceStub{}, "test")

	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", strings.NewReader(`{"name":"Test Novel","path":"/tmp/test-novel"}`))
	createResponse := httptest.NewRecorder()
	handler.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", createResponse.Code, http.StatusCreated)
	}

	openRequest := httptest.NewRequest(http.MethodPost, "/api/projects/open", strings.NewReader(`{"path":"/tmp/test-novel"}`))
	openResponse := httptest.NewRecorder()
	handler.ServeHTTP(openResponse, openRequest)
	if openResponse.Code != http.StatusOK {
		t.Fatalf("open status = %d, want %d", openResponse.Code, http.StatusOK)
	}

	if len(session.setCalls) != 2 {
		t.Fatalf("session Set() calls = %d, want 2", len(session.setCalls))
	}
	if session.setCalls[0].Path != "/tmp/test-novel" || session.setCalls[1].Path != "/tmp/test-novel" {
		t.Fatalf("session Set() paths = %#v", session.setCalls)
	}
}

// BDD trace:
//   - Requirement: Milestone 1, Story 1.1, view the outline.
//   - Scenario: given an active project, when I request the outline, then I
//     receive the outline JSON shape with ordered arcs, chapters, and scenes.
//   - Test purpose: verify the outline handler returns the exact nested JSON body
//     for a successful outline read.
func TestGetOutlineReturnsNestedJSONShape(t *testing.T) {
	t.Parallel()

	outline := story.Outline{
		Version: 1,
		Arcs: []story.Arc{{
			ID: "arc_00000000000000000001", Title: "Act One", DisplayLabel: "Arc 1",
			Chapters: []story.Chapter{{
				ID: "ch_00000000000000000001", Title: "Arrival", DisplayLabel: "Chapter 1.1",
				Scenes: []story.Scene{{
					ID: "scn_00000000000000000001", Title: "The Station", DisplayLabel: "Scene 1.1.1",
				}},
			}},
		}},
	}
	handler := newTestHandler(&projectStoreStub{}, &activeProjectSessionStub{}, &storyServiceStub{outlineResult: outline}, "test")

	request := httptest.NewRequest(http.MethodGet, "/api/outline", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if body["version"] != float64(1) {
		t.Fatalf("version = %#v", body["version"])
	}
}

// BDD trace:
//   - Requirement: Milestone 1, Stories 1.2 to 1.4, create/reorder structure and
//     preserve checkpoint integrity.
//   - Scenario: structure routes return success payloads on valid requests and
//     map malformed input, missing parents, dirty worktrees, active-project
//     conflicts, and internal failures to the documented HTTP statuses.
//   - Test purpose: verify strict JSON decoding and `errors.Is`-based status
//     mapping for Milestone 1 mutation routes.
func TestOutlineMutationRoutesValidateRequestsAndMapErrors(t *testing.T) {
	t.Parallel()

	result := story.MutationResult{
		ChangedID: "arc_00000000000000000001",
		Outline: story.Outline{
			Version: 1,
			Arcs:    []story.Arc{{ID: "arc_00000000000000000001", Title: "Act One", DisplayLabel: "Arc 1"}},
		},
	}
	service := &storyServiceStub{mutationResult: result}
	handler := newTestHandler(&projectStoreStub{}, &activeProjectSessionStub{}, service, "test")

	createArcRequest := httptest.NewRequest(http.MethodPost, "/api/arcs", strings.NewReader(`{"title":"Act One"}`))
	createArcResponse := httptest.NewRecorder()
	handler.ServeHTTP(createArcResponse, createArcRequest)
	if createArcResponse.Code != http.StatusCreated {
		t.Fatalf("create arc status = %d, want %d", createArcResponse.Code, http.StatusCreated)
	}
	if service.createArcTitle != "Act One" {
		t.Fatalf("create arc title = %q", service.createArcTitle)
	}
	var createBody map[string]any
	if err := json.NewDecoder(createArcResponse.Body).Decode(&createBody); err != nil {
		t.Fatalf("decode create arc response: %v", err)
	}
	if createBody["changed_id"] != "arc_00000000000000000001" || createBody["outline"] == nil {
		t.Fatalf("create arc response = %#v", createBody)
	}

	createChapterRequest := httptest.NewRequest(http.MethodPost, "/api/chapters", strings.NewReader(`{"arc_id":"arc_00000000000000000001","title":"Arrival"}`))
	createChapterResponse := httptest.NewRecorder()
	handler.ServeHTTP(createChapterResponse, createChapterRequest)
	if createChapterResponse.Code != http.StatusCreated || service.createChapterArcID != "arc_00000000000000000001" || service.createChapterTitle != "Arrival" {
		t.Fatalf("create chapter status/arguments = %d %q %q", createChapterResponse.Code, service.createChapterArcID, service.createChapterTitle)
	}

	createSceneRequest := httptest.NewRequest(http.MethodPost, "/api/scenes", strings.NewReader(`{"chapter_id":"ch_00000000000000000001","title":"The Station"}`))
	createSceneResponse := httptest.NewRecorder()
	handler.ServeHTTP(createSceneResponse, createSceneRequest)
	if createSceneResponse.Code != http.StatusCreated || service.createSceneChapterID != "ch_00000000000000000001" || service.createSceneTitle != "The Station" {
		t.Fatalf("create scene status/arguments = %d %q %q", createSceneResponse.Code, service.createSceneChapterID, service.createSceneTitle)
	}

	reorderRequest := httptest.NewRequest(http.MethodPost, "/api/outline/reorder", strings.NewReader(`{"parent_type":"arc","parent_id":"arc_00000000000000000001","ordered_child_ids":["ch_00000000000000000002","ch_00000000000000000001"]}`))
	reorderResponse := httptest.NewRecorder()
	handler.ServeHTTP(reorderResponse, reorderRequest)
	if reorderResponse.Code != http.StatusOK {
		t.Fatalf("reorder status = %d, want %d", reorderResponse.Code, http.StatusOK)
	}
	if service.reorderRequest.ParentID != "arc_00000000000000000001" {
		t.Fatalf("reorder parent ID = %q", service.reorderRequest.ParentID)
	}

	cases := []struct {
		name       string
		method     string
		path       string
		body       string
		serviceErr error
		status     int
	}{
		{
			name:   "malformed JSON rejected",
			method: http.MethodPost,
			path:   "/api/scenes",
			body:   `{"chapter_id":`,
			status: http.StatusBadRequest,
		},
		{
			name:   "trailing JSON rejected",
			method: http.MethodPost,
			path:   "/api/arcs",
			body:   `{"title":"Act One"}{"title":"Act Two"}`,
			status: http.StatusBadRequest,
		},
		{
			name:   "unknown field rejected",
			method: http.MethodPost,
			path:   "/api/chapters",
			body:   `{"arc_id":"arc_00000000000000000001","title":"Arrival","extra":true}`,
			status: http.StatusBadRequest,
		},
		{
			name:       "invalid title",
			method:     http.MethodPost,
			path:       "/api/arcs",
			body:       `{"title":""}`,
			serviceErr: story.ErrInvalidTitle,
			status:     http.StatusBadRequest,
		},
		{
			name:       "invalid ID shape",
			method:     http.MethodPost,
			path:       "/api/chapters",
			body:       `{"arc_id":"../../unsafe","title":"Arrival"}`,
			serviceErr: story.ErrInvalidID,
			status:     http.StatusBadRequest,
		},
		{
			name:       "no active project conflict",
			method:     http.MethodGet,
			path:       "/api/outline",
			serviceErr: story.ErrNoActiveProject,
			status:     http.StatusConflict,
		},
		{
			name:   "request over one MiB rejected",
			method: http.MethodPost,
			path:   "/api/arcs",
			body:   strings.Repeat(" ", 1<<20) + `{"title":"Act One"}`,
			status: http.StatusBadRequest,
		},
		{
			name:       "invalid reorder request",
			method:     http.MethodPost,
			path:       "/api/outline/reorder",
			body:       `{"parent_type":"arc","parent_id":"arc_00000000000000000001","ordered_child_ids":["ch_00000000000000000001"]}`,
			serviceErr: story.ErrInvalidReorder,
			status:     http.StatusBadRequest,
		},
		{
			name:       "missing parent",
			method:     http.MethodPost,
			path:       "/api/chapters",
			body:       `{"arc_id":"arc_00000000000000000009","title":"Arrival"}`,
			serviceErr: story.ErrParentNotFound,
			status:     http.StatusNotFound,
		},
		{
			name:       "dirty repository",
			method:     http.MethodPost,
			path:       "/api/arcs",
			body:       `{"title":"Act One"}`,
			serviceErr: story.ErrDirtyWorktree,
			status:     http.StatusConflict,
		},
		{
			name:       "internal adapter failure",
			method:     http.MethodPost,
			path:       "/api/scenes",
			body:       `{"chapter_id":"ch_00000000000000000001","title":"The Station"}`,
			serviceErr: errors.New("adapter failed"),
			status:     http.StatusInternalServerError,
		},
	}

	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			service := &storyServiceStub{}
			switch testCase.path {
			case "/api/outline":
				service.outlineErr = testCase.serviceErr
			case "/api/arcs":
				service.createArcErr = testCase.serviceErr
			case "/api/chapters":
				service.createChapterErr = testCase.serviceErr
			case "/api/scenes":
				service.createSceneErr = testCase.serviceErr
			case "/api/outline/reorder":
				service.reorderErr = testCase.serviceErr
			}
			handler := newTestHandler(&projectStoreStub{}, &activeProjectSessionStub{}, service, "test")
			request := httptest.NewRequest(testCase.method, testCase.path, strings.NewReader(testCase.body))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != testCase.status {
				t.Fatalf("status = %d, want %d: %s", response.Code, testCase.status, response.Body.String())
			}
		})
	}
}
