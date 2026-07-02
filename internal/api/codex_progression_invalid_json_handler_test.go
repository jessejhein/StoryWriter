// BDD Scenario: 3.2.2 - Reject invalid progressions
// Requirements: M3-R06, M3-R09
// Test purpose: The progression API rejects null and omitted nested fields before invoking the story service.
package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

)

// Test: malformed nested progression JSON returns a JSON 400 response without invoking the story service.
// Requirements: M3-R06, M3-R09
func TestCodexProgressionRouteRejectsNullAndMissingNestedFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
	}{
		{name: "null progression", body: `{"progressions":[null],"expected_revision":null}`},
		{name: "null progression ID", body: `{"progressions":[{"id":null,"anchor":{"type":"scene","id":"scn_0123456789abcdef0123","timing":"after"},"changes":{"description":"Gone."}}],"expected_revision":null}`},
		{name: "empty progression ID", body: `{"progressions":[{"id":"","anchor":{"type":"scene","id":"scn_0123456789abcdef0123","timing":"after"},"changes":{"description":"Gone."}}],"expected_revision":null}`},
		{name: "numeric progression ID", body: `{"progressions":[{"id":42,"anchor":{"type":"scene","id":"scn_0123456789abcdef0123","timing":"after"},"changes":{"description":"Gone."}}],"expected_revision":null}`},
		{name: "missing anchor", body: `{"progressions":[{"changes":{"description":"Gone."}}],"expected_revision":null}`},
		{name: "null anchor", body: `{"progressions":[{"anchor":null,"changes":{"description":"Gone."}}],"expected_revision":null}`},
		{name: "array anchor", body: `{"progressions":[{"anchor":[],"changes":{"description":"Gone."}}],"expected_revision":null}`},
		{name: "missing anchor type", body: `{"progressions":[{"anchor":{"id":"scn_0123456789abcdef0123","timing":"after"},"changes":{"description":"Gone."}}],"expected_revision":null}`},
		{name: "null anchor type", body: `{"progressions":[{"anchor":{"type":null,"id":"scn_0123456789abcdef0123","timing":"after"},"changes":{"description":"Gone."}}],"expected_revision":null}`},
		{name: "numeric anchor type", body: `{"progressions":[{"anchor":{"type":42,"id":"scn_0123456789abcdef0123","timing":"after"},"changes":{"description":"Gone."}}],"expected_revision":null}`},
		{name: "missing anchor ID", body: `{"progressions":[{"anchor":{"type":"scene","timing":"after"},"changes":{"description":"Gone."}}],"expected_revision":null}`},
		{name: "missing anchor timing", body: `{"progressions":[{"anchor":{"type":"scene","id":"scn_0123456789abcdef0123"},"changes":{"description":"Gone."}}],"expected_revision":null}`},
		{name: "null anchor timing", body: `{"progressions":[{"anchor":{"type":"scene","id":"scn_0123456789abcdef0123","timing":null},"changes":{"description":"Gone."}}],"expected_revision":null}`},
		{name: "missing changes", body: `{"progressions":[{"anchor":{"type":"scene","id":"scn_0123456789abcdef0123","timing":"after"}}],"expected_revision":null}`},
		{name: "null changes", body: `{"progressions":[{"anchor":{"type":"scene","id":"scn_0123456789abcdef0123","timing":"after"},"changes":null}],"expected_revision":null}`},
		{name: "array changes", body: `{"progressions":[{"anchor":{"type":"scene","id":"scn_0123456789abcdef0123","timing":"after"},"changes":[]}],"expected_revision":null}`},
		{name: "null description", body: `{"progressions":[{"anchor":{"type":"scene","id":"scn_0123456789abcdef0123","timing":"after"},"changes":{"description":null,"metadata":{"status":"gone"}}}],"expected_revision":null}`},
		{name: "numeric description", body: `{"progressions":[{"anchor":{"type":"scene","id":"scn_0123456789abcdef0123","timing":"after"},"changes":{"description":42}}],"expected_revision":null}`},
		{name: "null metadata", body: `{"progressions":[{"anchor":{"type":"scene","id":"scn_0123456789abcdef0123","timing":"after"},"changes":{"description":"Gone.","metadata":null}}],"expected_revision":null}`},
		{name: "array metadata", body: `{"progressions":[{"anchor":{"type":"scene","id":"scn_0123456789abcdef0123","timing":"after"},"changes":{"metadata":[]}}],"expected_revision":null}`},
		{name: "null metadata value", body: `{"progressions":[{"anchor":{"type":"scene","id":"scn_0123456789abcdef0123","timing":"after"},"changes":{"metadata":{"status":null}}}],"expected_revision":null}`},
		{name: "numeric metadata value", body: `{"progressions":[{"anchor":{"type":"scene","id":"scn_0123456789abcdef0123","timing":"after"},"changes":{"metadata":{"status":42}}}],"expected_revision":null}`},
	}

	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			service := &storyServiceStub{}
			response := httptest.NewRecorder()
			newTestHandler(&projectStoreStub{}, &activeProjectSessionStub{}, service, "test").ServeHTTP(
				response,
				httptest.NewRequest(http.MethodPut, "/api/codex/char_0123456789abcdef0123/progressions", strings.NewReader(testCase.body)),
			)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusBadRequest, response.Body.String())
			}
			if contentType := response.Header().Get("Content-Type"); contentType != "application/json" {
				t.Fatalf("Content-Type = %q, want application/json", contentType)
			}
			var body struct {
				Error string `json:"error"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body.Error == "" {
				t.Fatalf("error response = %s, decode error = %v", response.Body.String(), err)
			}
			if service.progressionEntryID != "" {
				t.Fatalf("SaveProgressions() called with entry ID %q", service.progressionEntryID)
			}
		})
	}
}
