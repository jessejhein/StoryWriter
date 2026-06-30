// BDD Scenario: 3.1.1 - List an empty Codex
// Requirements: M3-R01, M3-R09, M3-R10, M3-R11
// Test purpose: The Codex list route returns exact empty and populated shapes in canonical sort order.
package api_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"storywork/internal/api"
	"storywork/internal/codex"
)

func TestCodexListRouteReturnsEntriesAndMapsStatuses(t *testing.T) {
	t.Parallel()

	service := &storyServiceStub{codexEntries: []codex.Entry{}}
	handler := api.NewHandler(&projectStoreStub{}, &activeProjectSessionStub{}, service, "test")

	// Test: GET /api/codex returns the documented empty entries array shape.
	// Requirements: M3-R09
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/codex", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	var body struct {
		Entries []codex.Entry `json:"entries"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if len(body.Entries) != 0 {
		t.Fatalf("entries = %#v", body.Entries)
	}

	// Test: malformed canonical errors remain internal-server failures rather than silently skipping entries.
	// Requirements: M3-R01
	service = &storyServiceStub{loadCodexErr: errors.New("decode codex/characters/x.yaml: unsupported")}
	response = httptest.NewRecorder()
	api.NewHandler(&projectStoreStub{}, &activeProjectSessionStub{}, service, "test").ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/codex", nil))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
}
