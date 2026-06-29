// BDD Scenario: 3.2.1 - Save ordered progressions
// Requirements: M3-R05, M3-R06, M3-R09
// Test purpose: Plain-English description of the progression load/save routes for strict JSON decoding, nullable expected revisions, and HTTP status mapping.
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
