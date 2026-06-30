// BDD Scenario: 3.1.2 - Create an entry
// Requirements: M3-R02, M3-R04, M3-R09
// Test purpose: Codex create and update routes enforce strict JSON, route-ID authority, and documented status mapping.
package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"storywork/internal/api"
	"storywork/internal/codex"
	"storywork/internal/story"
)

func TestCodexCreateAndUpdateRoutesValidateJSONAndMapStatuses(t *testing.T) {
	t.Parallel()

	entry := codex.Entry{
		ID:          "char_0123456789abcdef0123",
		Type:        codex.TypeCharacter,
		Name:        "Obi-Wan Kenobi",
		Aliases:     []string{"Ben"},
		Tags:        []string{"mentor"},
		Description: "Guide.",
		Metadata:    map[string]string{"status": "alive"},
		Revision:    "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	service := &storyServiceStub{codexEntry: entry}
	handler := api.NewHandler(&projectStoreStub{}, &activeProjectSessionStub{}, service, "test")

	// Test: POST /api/codex forwards the canonical create payload and returns 201.
	// Requirements: M3-R02
	createBody := `{"type":"character","name":"Obi-Wan Kenobi","aliases":["Ben"],"tags":["mentor"],"description":"Guide.","metadata":{"status":"alive"}}`
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/codex", strings.NewReader(createBody)))
	if response.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", response.Code, http.StatusCreated)
	}
	var created map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatalf("create response JSON error = %v", err)
	}
	if _, ok := created["version"]; ok {
		t.Fatalf("create response leaked version: %s", response.Body.String())
	}
	if service.saveCodexRequest.Type != codex.TypeCharacter || service.saveCodexRequest.Name != "Obi-Wan Kenobi" {
		t.Fatalf("create request = %#v", service.saveCodexRequest)
	}

	// Test: PUT /api/codex/{id} uses the route ID and expected revision from strict JSON.
	// Requirements: M3-R09
	updateBody := `{"name":"Ben Kenobi","aliases":["Ben"],"tags":["mentor"],"description":"Guide.","metadata":{"status":"alive"},"expected_revision":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/api/codex/char_0123456789abcdef0123", strings.NewReader(updateBody)))
	if response.Code != http.StatusOK {
		t.Fatalf("update status = %d, want %d", response.Code, http.StatusOK)
	}
	var updated map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &updated); err != nil {
		t.Fatalf("update response JSON error = %v", err)
	}
	if _, ok := updated["version"]; ok {
		t.Fatalf("update response leaked version: %s", response.Body.String())
	}
	if service.codexEntryID != "char_0123456789abcdef0123" || service.saveCodexRequest.ExpectedRevision == "" {
		t.Fatalf("update request = %#v %q", service.saveCodexRequest, service.codexEntryID)
	}

	cases := []struct {
		name   string
		method string
		path   string
		body   string
		err    error
		status int
	}{
		{name: "unknown field", method: http.MethodPost, path: "/api/codex", body: `{"type":"character","name":"Ben","aliases":[],"tags":[],"description":"","metadata":{},"extra":true}`, status: http.StatusBadRequest},
		{name: "missing create aliases", method: http.MethodPost, path: "/api/codex", body: `{"type":"character","name":"Ben","tags":[],"description":"","metadata":{}}`, status: http.StatusBadRequest},
		{name: "missing create metadata", method: http.MethodPost, path: "/api/codex", body: `{"type":"character","name":"Ben","aliases":[],"tags":[],"description":""}`, status: http.StatusBadRequest},
		{name: "missing update aliases", method: http.MethodPut, path: "/api/codex/char_0123456789abcdef0123", body: `{"name":"Ben Kenobi","tags":["mentor"],"description":"Guide.","metadata":{"status":"alive"},"expected_revision":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`, status: http.StatusBadRequest},
		{name: "missing update expected revision", method: http.MethodPut, path: "/api/codex/char_0123456789abcdef0123", body: `{"name":"Ben Kenobi","aliases":["Ben"],"tags":["mentor"],"description":"Guide.","metadata":{"status":"alive"}}`, status: http.StatusBadRequest},
		{name: "invalid update revision shape", method: http.MethodPut, path: "/api/codex/char_0123456789abcdef0123", body: `{"name":"Ben Kenobi","aliases":["Ben"],"tags":["mentor"],"description":"Guide.","metadata":{"status":"alive"},"expected_revision":"stale"}`, err: codex.ErrInvalidRevision, status: http.StatusBadRequest},
		{name: "bad type", method: http.MethodPost, path: "/api/codex", body: createBody, err: codex.ErrInvalidType, status: http.StatusBadRequest},
		{name: "stale revision", method: http.MethodPut, path: "/api/codex/char_0123456789abcdef0123", body: updateBody, err: story.ErrStaleRevision, status: http.StatusConflict},
		{name: "missing entry", method: http.MethodGet, path: "/api/codex/char_0123456789abcdef0123", err: codex.ErrEntryNotFound, status: http.StatusNotFound},
	}

	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			service := &storyServiceStub{
				createCodexErr: testCase.err,
				updateCodexErr: testCase.err,
				loadCodexErr:   testCase.err,
			}
			response := httptest.NewRecorder()
			api.NewHandler(&projectStoreStub{}, &activeProjectSessionStub{}, service, "test").ServeHTTP(
				response,
				httptest.NewRequest(testCase.method, testCase.path, strings.NewReader(testCase.body)),
			)
			if response.Code != testCase.status {
				t.Fatalf("status = %d, want %d", response.Code, testCase.status)
			}
		})
	}
}
