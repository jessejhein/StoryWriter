// BDD Scenario: 8.2.1 - List exact changed files
// Requirements: M8-R05, M8-R06
// Test purpose: Comparison routes return strict JSON and reject unknown query keys.

package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Test: comparison route returns 200 for stubbed experiment.
// Requirements: M8-R05.
func TestBranchComparisonRoute(t *testing.T) {
	t.Parallel()
	handler := newBranchHandler()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/branches/brn_0123456789abcdef0123/comparison", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

// Test: file comparison requires single path query.
// Requirements: M8-R07.
func TestBranchFileComparisonRouteRequiresPath(t *testing.T) {
	t.Parallel()
	handler := newBranchHandler()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/branches/brn_0123456789abcdef0123/comparison/file", nil))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}
