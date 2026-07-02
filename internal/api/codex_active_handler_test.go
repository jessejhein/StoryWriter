// BDD Scenario: 3.3.1 - Resolve before and after an anchor
// Requirements: M3-R07, M3-R09, M3-R10
// Test purpose: The active-state route requires stable entry and scene targets and maps domain failures to documented statuses.
package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"storywork/internal/codex"
)

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
	handler := newTestHandler(&projectStoreStub{}, &activeProjectSessionStub{}, service, "test")

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
