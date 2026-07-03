package api_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"storywork/internal/story"
)

// BDD trace:
//   - Requirement: M2-R01, M2-R02, M2-R08.
//   - Scenario: 2.1.3 — Invalid or unknown scene.
//   - Test purpose: verify the scene load route forwards the stable ID and maps
//     malformed IDs, unknown scenes, missing active projects, and malformed
//     canonical storage to the documented HTTP statuses.
func TestSceneLoadRouteMapsStatuses(t *testing.T) {
	t.Parallel()

	scene := story.SceneDocument{
		ID:        "scn_00000000000000000001",
		ChapterID: "ch_00000000000000000001",
		Title:     "The Duel",
		FrontMatter: story.SceneFrontMatter{
			POV:           "Luke",
			Status:        "draft",
			ExcludeFromAI: false,
		},
		Markdown: "Scene prose.\n",
		Revision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	service := &storyServiceStub{sceneResult: scene}
	handler := newTestHandler(&projectStoreStub{}, &activeProjectSessionStub{}, service, "test")

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/scenes/scn_00000000000000000001", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if service.loadSceneID != "scn_00000000000000000001" {
		t.Fatalf("load scene ID = %q", service.loadSceneID)
	}
	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if body["revision"] != scene.Revision {
		t.Fatalf("body = %#v", body)
	}

	cases := []struct {
		name   string
		err    error
		status int
	}{
		{name: "invalid id", err: story.ErrInvalidID, status: http.StatusBadRequest},
		{name: "not found", err: story.ErrSceneNotFound, status: http.StatusNotFound},
		{name: "no active project", err: story.ErrNoActiveProject, status: http.StatusConflict},
		{name: "malformed canonical", err: errors.New("decode scenes/foo.md: unsupported"), status: http.StatusInternalServerError},
	}
	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			service := &storyServiceStub{loadSceneErr: testCase.err}
			response := httptest.NewRecorder()
			newTestHandler(&projectStoreStub{}, &activeProjectSessionStub{}, service, "test").ServeHTTP(
				response,
				httptest.NewRequest(http.MethodGet, "/api/scenes/scn_00000000000000000001", nil),
			)
			if response.Code != testCase.status {
				t.Fatalf("status = %d, want %d", response.Code, testCase.status)
			}
		})
	}
}

// BDD trace:
//   - Requirement: M2-R04, M2-R05, M2-R06, M2-R08.
//   - Scenario: 2.2.1 — Save valid edits.
//   - Test purpose: verify the scene save route strictly decodes JSON, forwards
//     the stable ID and request payload, and maps documented save failures.
func TestSceneSaveRouteValidatesJSONAndMapsStatuses(t *testing.T) {
	t.Parallel()

	scene := story.SceneDocument{
		ID:        "scn_00000000000000000001",
		ChapterID: "ch_00000000000000000001",
		Title:     "The Duel",
		FrontMatter: story.SceneFrontMatter{
			POV:           "Luke",
			Status:        "revised",
			ExcludeFromAI: false,
		},
		Markdown: "Revised.\n",
		Revision: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}
	service := &storyServiceStub{sceneResult: scene}
	handler := newTestHandler(&projectStoreStub{}, &activeProjectSessionStub{}, service, "test")

	requestBody := `{"title":"The Duel","frontmatter":{"pov":"Luke","status":"revised","exclude_from_ai":false},"markdown":"Revised.\n","expected_revision":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/api/scenes/scn_00000000000000000001", strings.NewReader(requestBody)))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if service.saveSceneID != "scn_00000000000000000001" || service.saveSceneRequest.ExpectedRevision == "" {
		t.Fatalf("save scene request = %#v", service.saveSceneRequest)
	}

	cases := []struct {
		name   string
		body   string
		err    error
		status int
	}{
		{name: "unknown field", body: `{"title":"The Duel","frontmatter":{"pov":"Luke","status":"draft","exclude_from_ai":false},"markdown":"","expected_revision":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","extra":true}`, status: http.StatusBadRequest},
		{name: "malformed JSON", body: `{"title":`, status: http.StatusBadRequest},
		{name: "no changes", body: requestBody, err: story.ErrNoSceneChanges, status: http.StatusBadRequest},
		{name: "stale revision", body: requestBody, err: story.ErrStaleRevision, status: http.StatusConflict},
		{name: "dirty project", body: requestBody, err: story.ErrDirtyWorktree, status: http.StatusConflict},
		{name: "scene missing", body: requestBody, err: story.ErrSceneNotFound, status: http.StatusNotFound},
		{name: "bad metadata", body: requestBody, err: story.ErrInvalidStatus, status: http.StatusBadRequest},
	}
	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			service := &storyServiceStub{saveSceneErr: testCase.err}
			response := httptest.NewRecorder()
			newTestHandler(&projectStoreStub{}, &activeProjectSessionStub{}, service, "test").ServeHTTP(
				response,
				httptest.NewRequest(http.MethodPut, "/api/scenes/scn_00000000000000000001", strings.NewReader(testCase.body)),
			)
			if response.Code != testCase.status {
				t.Fatalf("status = %d, want %d: %s", response.Code, testCase.status, response.Body.String())
			}
		})
	}
}
