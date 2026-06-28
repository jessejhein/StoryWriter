package api_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"storywork/internal/api"
	"storywork/internal/codex"
	"storywork/internal/story"
)

// BDD Scenario: 3.1.1 - List an empty Codex
// Requirements: M3-R01, M3-R09, M3-R10, M3-R11
// Test purpose: Plain-English description of the Codex list route for empty and populated JSON responses plus documented status mapping.
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

// BDD Scenario: 3.1.2 - Create an entry
// Requirements: M3-R02, M3-R04, M3-R09
// Test purpose: Plain-English description of the create and update Codex routes for strict JSON decoding, route-ID authority, and status mapping.
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
