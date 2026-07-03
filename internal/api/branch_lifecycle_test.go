// BDD Scenario: 8.1.1 - Create from current canon
// Requirements: M8-R01, M8-R02
// Test purpose: Branch lifecycle routes return strict JSON and status codes.

package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"storywork/internal/api"
)

func newBranchHandler() http.Handler {
	return api.NewHandler(api.HandlerDependencies{
		Projects:  &projectStoreStub{},
		Session:   &activeProjectSessionStub{},
		Stories:   &storyServiceStub{},
		Actions:   &storyServiceStub{},
		Providers: &storyServiceStub{},
		Imports:   &storyServiceStub{},
		Branches:  branchServiceStub{},
		Version:   "branch-test",
	})
}

// Test: status and list routes return 200 JSON.
// Requirements: M8-R01.
func TestBranchStatusAndListRoutes(t *testing.T) {
	t.Parallel()
	handler := newBranchHandler()
	for _, path := range []string{"/api/branches/status", "/api/branches"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("%s status = %d body=%s", path, response.Code, response.Body.String())
		}
		var payload map[string]any
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if path == "/api/branches/status" {
			for _, key := range []string{"active_branch", "active_kind", "main_head", "experiment_head", "active_experiment_id", "worktree_clean"} {
				if _, ok := payload[key]; !ok {
					t.Fatalf("status response missing %q: %s", key, response.Body.String())
				}
			}
		}
	}
}

// Test: create route validates JSON and returns 201.
// Requirements: M8-R02.
func TestBranchCreateRoute(t *testing.T) {
	t.Parallel()
	handler := newBranchHandler()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/branches", strings.NewReader(`{"name":"Test Exp"}`)))
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

// Test: unsupported methods return Allow header.
// Requirements: M8-R01.
func TestBranchRoutesMethodNotAllowed(t *testing.T) {
	t.Parallel()
	handler := newBranchHandler()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/api/branches", nil))
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", response.Code)
	}
	if got := response.Header().Get("Allow"); got == "" {
		t.Fatal("Allow header missing")
	}
}
