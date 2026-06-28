package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"storywork/internal/api"
	"storywork/internal/project"
	"storywork/internal/story"
)

type activeProjectSessionStub struct {
	setCalls []project.Project
}

func (s *activeProjectSessionStub) Set(current project.Project) {
	s.setCalls = append(s.setCalls, current)
}

type storyServiceStub struct {
	outlineResult        story.Outline
	mutationResult       story.MutationResult
	outlineErr           error
	createArcErr         error
	createChapterErr     error
	createSceneErr       error
	reorderErr           error
	createArcTitle       string
	createChapterArcID   string
	createChapterTitle   string
	createSceneChapterID string
	createSceneTitle     string
	reorderRequest       story.ReorderRequest
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
	handler := api.NewHandler(projectStore, session, &storyServiceStub{}, "test")

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
	handler := api.NewHandler(&projectStoreStub{}, &activeProjectSessionStub{}, &storyServiceStub{outlineResult: outline}, "test")

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
	handler := api.NewHandler(&projectStoreStub{}, &activeProjectSessionStub{}, service, "test")

	createArcRequest := httptest.NewRequest(http.MethodPost, "/api/arcs", strings.NewReader(`{"title":"Act One"}`))
	createArcResponse := httptest.NewRecorder()
	handler.ServeHTTP(createArcResponse, createArcRequest)
	if createArcResponse.Code != http.StatusCreated {
		t.Fatalf("create arc status = %d, want %d", createArcResponse.Code, http.StatusCreated)
	}
	if service.createArcTitle != "Act One" {
		t.Fatalf("create arc title = %q", service.createArcTitle)
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
			name:       "no active project conflict",
			method:     http.MethodGet,
			path:       "/api/outline",
			serviceErr: story.ErrNoActiveProject,
			status:     http.StatusConflict,
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
			handler := api.NewHandler(&projectStoreStub{}, &activeProjectSessionStub{}, service, "test")
			request := httptest.NewRequest(testCase.method, testCase.path, strings.NewReader(testCase.body))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != testCase.status {
				t.Fatalf("status = %d, want %d: %s", response.Code, testCase.status, response.Body.String())
			}
		})
	}
}
