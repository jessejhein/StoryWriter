package api_test

// BDD Scenario: characterization guard for segregated HTTP dependencies
// Requirements: M7-R19
// Test purpose: Prove the HTTP handler accepts cohesive store boundaries before
// Milestone 7 adds new action routes.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Test: HandlerDependencies accepts separate story, action, provider, and import stores.
// Requirements: M7-R19.
func TestHandlerDependenciesAcceptCohesiveStores(t *testing.T) {
	t.Parallel()

	stub := &storyServiceStub{}
	handler := newTestHandler(&projectStoreStub{}, &activeProjectSessionStub{}, stub, "deps-test")

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("health status = %d, want 200 body=%s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"version":"deps-test"`) {
		t.Fatalf("health body = %s", response.Body.String())
	}
}
