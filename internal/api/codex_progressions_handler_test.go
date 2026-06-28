package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"storywork/internal/api"
	"storywork/internal/codex"
	"storywork/internal/story"
)

// BDD Scenario: 3.2.1 - Save ordered progressions
// Requirements: M3-R05, M3-R06, M3-R09
// Test purpose: Plain-English description of the progression load/save routes for strict JSON decoding, nullable expected revisions, and HTTP status mapping.
func TestCodexProgressionRoutesValidateJSONAndMapStatuses(t *testing.T) {
	t.Parallel()

	revision := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	document := codex.ProgressionDocument{
		EntryID:      "char_0123456789abcdef0123",
		Progressions: []codex.Progression{},
		Revision:     &revision,
	}
	service := &storyServiceStub{progressionDocument: document}
	handler := api.NewHandler(&projectStoreStub{}, &activeProjectSessionStub{}, service, "test")

	// Test: GET and PUT progressions use the route entry ID and forward nullable expected revisions.
	// Requirements: M3-R09
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/codex/char_0123456789abcdef0123/progressions", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d", response.Code, http.StatusOK)
	}
	body := `{"progressions":[],"expected_revision":null}`
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/api/codex/char_0123456789abcdef0123/progressions", strings.NewReader(body)))
	if response.Code != http.StatusOK {
		t.Fatalf("put status = %d, want %d", response.Code, http.StatusOK)
	}
	if service.progressionEntryID != "char_0123456789abcdef0123" {
		t.Fatalf("entry ID = %q", service.progressionEntryID)
	}

	// Test: invalid progression payloads and stale revisions map to the documented statuses.
	// Requirements: M3-R06
	cases := []struct {
		name   string
		err    error
		status int
	}{
		{name: "invalid payload", err: codex.ErrInvalidProgression, status: http.StatusBadRequest},
		{name: "stale revision", err: story.ErrStaleRevision, status: http.StatusConflict},
	}
	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			response := httptest.NewRecorder()
			api.NewHandler(&projectStoreStub{}, &activeProjectSessionStub{}, &storyServiceStub{saveProgressionsErr: testCase.err}, "test").ServeHTTP(
				response,
				httptest.NewRequest(http.MethodPut, "/api/codex/char_0123456789abcdef0123/progressions", strings.NewReader(body)),
			)
			if response.Code != testCase.status {
				t.Fatalf("status = %d, want %d", response.Code, testCase.status)
			}
		})
	}
}

// BDD Scenario: 3.3.1 - Resolve before and after an anchor
// Requirements: M3-R07, M3-R09, M3-R10
// Test purpose: Plain-English description of the active-state route for required query parameters and stable entry-scene read requests.
func TestCodexActiveRouteRequiresSceneAndMapsStatuses(t *testing.T) {
	t.Parallel()

	service := &storyServiceStub{
		activeState: codex.ActiveState{
			SceneID: "scn_0123456789abcdef0123",
			Entry: codex.Entry{
				ID:          "char_0123456789abcdef0123",
				Type:        codex.TypeCharacter,
				Name:        "Ben",
				Aliases:     []string{},
				Tags:        []string{},
				Description: "Guide.",
				Metadata:    map[string]string{},
			},
			AppliedProgressionIDs: []string{"prog_0123456789abcdef0123"},
		},
	}
	handler := api.NewHandler(&projectStoreStub{}, &activeProjectSessionStub{}, service, "test")

	// Test: the active-state route requires scene_id and forwards stable entry and scene IDs to the service.
	// Requirements: M3-R09
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/codex/char_0123456789abcdef0123/active?scene_id=scn_0123456789abcdef0123", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if service.activeEntryID != "char_0123456789abcdef0123" || service.activeSceneID != "scn_0123456789abcdef0123" {
		t.Fatalf("active request = %q %q", service.activeEntryID, service.activeSceneID)
	}

	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/codex/char_0123456789abcdef0123/active", nil))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("missing scene status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}
