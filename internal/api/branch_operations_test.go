// BDD Scenario: 8.3.1 - Run only after explicit authorization
// Requirements: M8-R09, M8-R12, M8-R17
// Test purpose: Ramification, promotion, and discard routes enforce strict JSON.

package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Test: ramification route requires strict body fields.
// Requirements: M8-R09.
func TestBranchRamificationRouteRequiresStrictBody(t *testing.T) {
	t.Parallel()
	handler := newBranchHandler()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/branches/brn_0123456789abcdef0123/ramifications", strings.NewReader(`{"goal":"test"}`)))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

// Test: promotion route accepts strict body.
// Requirements: M8-R12.
func TestBranchPromotionRoute(t *testing.T) {
	t.Parallel()
	handler := newBranchHandler()
	body := `{"paths":["outline.yaml"],"expected_main_head":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","expected_experiment_head":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","comparison_fingerprint":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}`
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/branches/brn_0123456789abcdef0123/promote", strings.NewReader(body)))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if _, ok := payload["promoted_paths"]; !ok {
		t.Fatalf("promotion response missing promoted_paths: %s", response.Body.String())
	}
	if payload["experiment_id"] != "brn_0123456789abcdef0123" {
		t.Fatalf("experiment_id = %#v", payload["experiment_id"])
	}
}

// Test: discard route accepts strict body.
// Requirements: M8-R17.
func TestBranchDiscardRoute(t *testing.T) {
	t.Parallel()
	handler := newBranchHandler()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/branches/brn_0123456789abcdef0123/discard", strings.NewReader(`{"expected_experiment_head":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}`)))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}
