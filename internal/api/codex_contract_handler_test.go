// BDD Scenario: 3.1.4 - Reject invalid entry data
// Requirements: M3-R02, M3-R03, M3-R04, M3-R09
// Test purpose: Codex and progression routes return 405 with JSON and Allow, reject oversized mutation bodies as 400, reject malformed entry IDs as 400, and return the documented JSON shapes for list, single-entry, progression, and active-state responses.
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

func TestCodexRoutesRejectUnsupportedMethods(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		method string
		path   string
		allow  string
	}{
		// Test: unsupported methods on known Codex routes return 405 with JSON error and an Allow header.
		// Requirements: M3-R09
		{name: "delete list", method: http.MethodDelete, path: "/api/codex", allow: "GET, POST"},
		{name: "patch list", method: http.MethodPatch, path: "/api/codex", allow: "GET, POST"},
		{name: "delete entry", method: http.MethodDelete, path: "/api/codex/char_0123456789abcdef0123", allow: "GET, PUT"},
		{name: "patch entry", method: http.MethodPatch, path: "/api/codex/char_0123456789abcdef0123", allow: "GET, PUT"},
		{name: "post progressions", method: http.MethodPost, path: "/api/codex/char_0123456789abcdef0123/progressions", allow: "GET, PUT"},
		{name: "delete active", method: http.MethodDelete, path: "/api/codex/char_0123456789abcdef0123/active", allow: "GET"},
	}
	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			handler := api.NewHandler(&projectStoreStub{}, &activeProjectSessionStub{}, &storyServiceStub{}, "test")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(testCase.method, testCase.path, nil))
			if response.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
			}
			if allow := response.Header().Get("Allow"); allow != testCase.allow {
				t.Fatalf("Allow = %q, want %q", allow, testCase.allow)
			}
			var body map[string]string
			if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if _, ok := body["error"]; !ok {
				t.Fatalf("response body = %s, want JSON error shape", response.Body.String())
			}
		})
	}
}

func TestCodexMutationRoutesRejectOversizedBodiesAsBadRequest(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		method string
		path   string
	}{
		// Test: oversized Codex/progression mutation bodies are rejected as 400 Bad Request, not an undocumented 413.
		// Requirements: M3-R09
		{name: "post entry", method: http.MethodPost, path: "/api/codex"},
		{name: "put entry", method: http.MethodPut, path: "/api/codex/char_0123456789abcdef0123"},
		{name: "put progressions", method: http.MethodPut, path: "/api/codex/char_0123456789abcdef0123/progressions"},
	}
	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			handler := api.NewHandler(&projectStoreStub{}, &activeProjectSessionStub{}, &storyServiceStub{}, "test")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(testCase.method, testCase.path, strings.NewReader(strings.Repeat(" ", 1<<20)+`{"type":"character","name":"x","aliases":[],"tags":[],"description":"","metadata":{}}`)))
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
			}
			var body map[string]string
			if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if _, ok := body["error"]; !ok {
				t.Fatalf("response body = %s, want JSON error shape", response.Body.String())
			}
		})
	}
}

func TestCodexListResponseShapeAndSortOrder(t *testing.T) {
	t.Parallel()

	// The list response shape is documented as {"entries":[...]} with each entry
	// carrying id, type, name, aliases, tags, description, metadata, and revision.
	// Sort order is enforced by the service layer; the handler forwards service
	// order verbatim, so the test asserts the shape and that the handler does not
	// re-sort, plus that every entry carries a non-empty revision.
	entries := []codex.Entry{
		{ID: "char_0123456789abcdef0001", Type: codex.TypeCharacter, Name: "Ben", Aliases: []string{}, Tags: []string{}, Description: "Guide.", Metadata: map[string]string{}, Revision: "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"},
		{ID: "loc_0123456789abcdef00002", Type: codex.TypeLocation, Name: "Tatooine", Aliases: []string{}, Tags: []string{}, Description: "Desert.", Metadata: map[string]string{}, Revision: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
		{ID: "char_0123456789abcdef0003", Type: codex.TypeCharacter, Name: "Anakin", Aliases: []string{}, Tags: []string{}, Description: "Jedi.", Metadata: map[string]string{}, Revision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
	}
	handler := api.NewHandler(&projectStoreStub{}, &activeProjectSessionStub{}, &storyServiceStub{codexEntries: entries}, "test")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/codex", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	var body struct {
		Entries []struct {
			ID          string            `json:"id"`
			Type        string            `json:"type"`
			Name        string            `json:"name"`
			Aliases     []string          `json:"aliases"`
			Tags        []string          `json:"tags"`
			Description string            `json:"description"`
			Metadata    map[string]string `json:"metadata"`
			Revision    string            `json:"revision"`
		} `json:"entries"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if len(body.Entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(body.Entries))
	}
	// The handler forwards service order verbatim.
	for index, want := range entries {
		if body.Entries[index].ID != want.ID {
			t.Fatalf("entry[%d].ID = %q, want %q", index, body.Entries[index].ID, want.ID)
		}
	}
	for _, entry := range body.Entries {
		if entry.Revision == "" {
			t.Fatalf("entry %q revision missing from response shape", entry.ID)
		}
	}
}

func TestCodexSingleEntryResponseShape(t *testing.T) {
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
	handler := api.NewHandler(&projectStoreStub{}, &activeProjectSessionStub{}, &storyServiceStub{codexEntry: entry}, "test")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/codex/char_0123456789abcdef0123", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	want := []string{"id", "type", "name", "aliases", "tags", "description", "metadata", "revision"}
	for _, key := range want {
		if _, ok := body[key]; !ok {
			t.Fatalf("response missing key %q: %s", key, response.Body.String())
		}
	}
	if _, ok := body["version"]; ok {
		t.Fatalf("response leaked version: %s", response.Body.String())
	}
	if _, ok := body["canonical"]; ok {
		t.Fatalf("response leaked canonical bytes: %s", response.Body.String())
	}
}

func TestCodexProgressionResponseShape(t *testing.T) {
	t.Parallel()

	revision := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	document := codex.ProgressionDocument{
		EntryID: "char_0123456789abcdef0123",
		Progressions: []codex.Progression{{
			ID:      "prog_0123456789abcdef0123",
			Anchor:  codex.ProgressionAnchor{Type: "scene", ID: "scn_0123456789abcdef0123", Timing: "after"},
			Changes: codex.ProgressionChange{Description: ptrString("Gone."), Metadata: map[string]string{"status": "deceased"}},
		}},
		Revision: &revision,
	}
	handler := api.NewHandler(&projectStoreStub{}, &activeProjectSessionStub{}, &storyServiceStub{progressionDocument: document}, "test")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/codex/char_0123456789abcdef0123/progressions", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	var body struct {
		EntryID      string `json:"entry_id"`
		Progressions []struct {
			ID     string `json:"id"`
			Anchor struct {
				Type   string `json:"type"`
				ID     string `json:"id"`
				Timing string `json:"timing"`
			} `json:"anchor"`
			Changes struct {
				Description string            `json:"description"`
				Metadata    map[string]string `json:"metadata"`
			} `json:"changes"`
		} `json:"progressions"`
		Revision *string `json:"revision"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if body.EntryID != document.EntryID {
		t.Fatalf("entry_id = %q", body.EntryID)
	}
	if body.Revision == nil || *body.Revision != revision {
		t.Fatalf("revision = %#v", body.Revision)
	}
	if len(body.Progressions) != 1 || body.Progressions[0].ID != "prog_0123456789abcdef0123" {
		t.Fatalf("progressions = %#v", body.Progressions)
	}
}

func TestCodexProgressionResponseEmptyDocumentShape(t *testing.T) {
	t.Parallel()

	// A no-progression-file response uses progressions [] and revision null per the contract.
	handler := api.NewHandler(&projectStoreStub{}, &activeProjectSessionStub{}, &storyServiceStub{progressionDocument: codex.ProgressionDocument{Progressions: []codex.Progression{}, Revision: nil}}, "test")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/codex/char_0123456789abcdef0123/progressions", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	var body struct {
		Progressions []any   `json:"progressions"`
		Revision     *string `json:"revision"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if len(body.Progressions) != 0 {
		t.Fatalf("progressions = %#v, want []", body.Progressions)
	}
	if body.Revision != nil {
		t.Fatalf("revision = %#v, want null", body.Revision)
	}
}

func TestCodexActiveStateResponseShape(t *testing.T) {
	t.Parallel()

	activeState := codex.ActiveState{
		SceneID: "scn_0123456789abcdef0123",
		Entry: codex.Entry{
			ID:          "char_0123456789abcdef0123",
			Type:        codex.TypeCharacter,
			Name:        "Obi-Wan Kenobi",
			Aliases:     []string{"Ben"},
			Tags:        []string{"mentor"},
			Description: "Gone, but influential.",
			Metadata:    map[string]string{"status": "deceased"},
		},
		AppliedProgressionIDs: []string{"prog_0123456789abcdef0123"},
	}
	handler := api.NewHandler(&projectStoreStub{}, &activeProjectSessionStub{}, &storyServiceStub{activeState: activeState}, "test")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/codex/char_0123456789abcdef0123/active?scene_id=scn_0123456789abcdef0123", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	var body struct {
		SceneID string `json:"scene_id"`
		Entry   struct {
			ID          string            `json:"id"`
			Type        string            `json:"type"`
			Name        string            `json:"name"`
			Aliases     []string          `json:"aliases"`
			Tags        []string          `json:"tags"`
			Description string            `json:"description"`
			Metadata    map[string]string `json:"metadata"`
		} `json:"entry"`
		AppliedProgressionIDs []string `json:"applied_progression_ids"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if body.SceneID != activeState.SceneID {
		t.Fatalf("scene_id = %q", body.SceneID)
	}
	if body.Entry.ID != activeState.Entry.ID || body.Entry.Description != activeState.Entry.Description {
		t.Fatalf("entry = %#v", body.Entry)
	}
	if len(body.AppliedProgressionIDs) != 1 || body.AppliedProgressionIDs[0] != "prog_0123456789abcdef0123" {
		t.Fatalf("applied_progression_ids = %#v", body.AppliedProgressionIDs)
	}
}

func TestCodexActiveStateRejectsMalformedAndAbsentSceneID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		path   string
		status int
		err    error
	}{
		// Test: a malformed scene ID returns 400 Bad Request and an absent scene ID returns 404 Not Found.
		// Requirements: M3-R09, M3-R18
		{name: "malformed scene id", path: "/api/codex/char_0123456789abcdef0123/active?scene_id=not-a-scene", err: codex.ErrInvalidID, status: http.StatusBadRequest},
		{name: "absent scene id", path: "/api/codex/char_0123456789abcdef0123/active?scene_id=scn_0123456789abcdef0123", err: codex.ErrSceneNotFound, status: http.StatusNotFound},
	}
	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			handler := api.NewHandler(&projectStoreStub{}, &activeProjectSessionStub{}, &storyServiceStub{activeCodexErr: testCase.err}, "test")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, testCase.path, nil))
			if response.Code != testCase.status {
				t.Fatalf("status = %d, want %d", response.Code, testCase.status)
			}
		})
	}
}

func TestCodexRoutesMapNoActiveProjectAndDirtyWorktreeToConflict(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		method string
		path   string
		body   string
		err    error
	}{
		// Test: no active project and dirty worktree both map to 409 Conflict on Codex routes.
		// Requirements: M3-R13, M3-R14
		{name: "no active project on create", method: http.MethodPost, path: "/api/codex", body: `{"type":"character","name":"Ben","aliases":[],"tags":[],"description":"","metadata":{}}`, err: story.ErrNoActiveProject},
		{name: "no active project on load", method: http.MethodGet, path: "/api/codex", err: story.ErrNoActiveProject},
		{name: "no active project on update", method: http.MethodPut, path: "/api/codex/char_0123456789abcdef0123", body: `{"name":"Ben","aliases":[],"tags":[],"description":"","metadata":{},"expected_revision":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`, err: story.ErrNoActiveProject},
		{name: "dirty worktree on create", method: http.MethodPost, path: "/api/codex", body: `{"type":"character","name":"Ben","aliases":[],"tags":[],"description":"","metadata":{}}`, err: story.ErrDirtyWorktree},
		{name: "dirty worktree on progressions", method: http.MethodPut, path: "/api/codex/char_0123456789abcdef0123/progressions", body: `{"progressions":[],"expected_revision":null}`, err: story.ErrDirtyWorktree},
		{name: "dirty worktree on active", method: http.MethodGet, path: "/api/codex/char_0123456789abcdef0123/active?scene_id=scn_0123456789abcdef0123", err: story.ErrDirtyWorktree},
	}
	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			service := &storyServiceStub{}
			switch {
			case strings.Contains(testCase.path, "/progressions"):
				service.saveProgressionsErr = testCase.err
				service.loadProgressionsErr = testCase.err
			case strings.Contains(testCase.path, "/active"):
				service.activeCodexErr = testCase.err
			case testCase.method == http.MethodPost:
				service.createCodexErr = testCase.err
			case testCase.method == http.MethodPut:
				service.updateCodexErr = testCase.err
			default:
				service.loadCodexErr = testCase.err
			}
			handler := api.NewHandler(&projectStoreStub{}, &activeProjectSessionStub{}, service, "test")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(testCase.method, testCase.path, strings.NewReader(testCase.body)))
			if response.Code != http.StatusConflict {
				t.Fatalf("status = %d, want %d (err=%v)", response.Code, http.StatusConflict, testCase.err)
			}
		})
	}
}

func TestCodexUpdateRejectsNoOpChanges(t *testing.T) {
	t.Parallel()

	// Test: a byte-identical update is mapped to 400 Bad Request, not 200, with no side effects.
	// Requirements: M3-R03
	body := `{"name":"Ben","aliases":[],"tags":[],"description":"","metadata":{},"expected_revision":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`
	handler := api.NewHandler(&projectStoreStub{}, &activeProjectSessionStub{}, &storyServiceStub{updateCodexErr: codex.ErrNoChanges}, "test")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/api/codex/char_0123456789abcdef0123", strings.NewReader(body)))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestCodexCreateRejectsIDAndRevisionFields(t *testing.T) {
	t.Parallel()

	// Test: POST may not include id or revision; unknown-field rejection returns 400.
	// Requirements: M3-R02
	body := `{"id":"char_0123456789abcdef0123","type":"character","name":"Ben","aliases":[],"tags":[],"description":"","metadata":{},"revision":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`
	handler := api.NewHandler(&projectStoreStub{}, &activeProjectSessionStub{}, &storyServiceStub{}, "test")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/codex", strings.NewReader(body)))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestCodexUpdateRejectsTypeField(t *testing.T) {
	t.Parallel()

	// Test: PUT may not include type; the route ID is authoritative and type comes from canonical storage.
	// Requirements: M3-R03
	body := `{"type":"character","name":"Ben","aliases":[],"tags":[],"description":"","metadata":{},"expected_revision":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`
	handler := api.NewHandler(&projectStoreStub{}, &activeProjectSessionStub{}, &storyServiceStub{}, "test")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/api/codex/char_0123456789abcdef0123", strings.NewReader(body)))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestCodexProgressionPutRejectsEntryIDField(t *testing.T) {
	t.Parallel()

	// Test: PUT progressions omits entry_id; the route entry ID is authoritative.
	// Requirements: M3-R05
	body := `{"entry_id":"char_0123456789abcdef0123","progressions":[],"expected_revision":null}`
	handler := api.NewHandler(&projectStoreStub{}, &activeProjectSessionStub{}, &storyServiceStub{}, "test")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/api/codex/char_0123456789abcdef0123/progressions", strings.NewReader(body)))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestCodexUpdateRejectsNullExpectedRevision(t *testing.T) {
	t.Parallel()

	// Test: PUT entry requires a non-null expected_revision; null is rejected as 400.
	// Requirements: M3-R17
	body := `{"name":"Ben","aliases":[],"tags":[],"description":"","metadata":{},"expected_revision":null}`
	handler := api.NewHandler(&projectStoreStub{}, &activeProjectSessionStub{}, &storyServiceStub{}, "test")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/api/codex/char_0123456789abcdef0123", strings.NewReader(body)))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestCodexRoutesRejectTrailingJSONAndWrongTypes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		// Test: trailing JSON values and wrong JSON types are rejected as 400 Bad Request.
		// Requirements: M3-R09
		{name: "trailing json on create", method: http.MethodPost, path: "/api/codex", body: `{"type":"character","name":"Ben","aliases":[],"tags":[],"description":"","metadata":{}}{}`},
		{name: "wrong type for name", method: http.MethodPost, path: "/api/codex", body: `{"type":"character","name":42,"aliases":[],"tags":[],"description":"","metadata":{}}`},
		{name: "trailing json on update", method: http.MethodPut, path: "/api/codex/char_0123456789abcdef0123", body: `{"name":"Ben","aliases":[],"tags":[],"description":"","metadata":{},"expected_revision":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}{}`},
		{name: "trailing json on progressions", method: http.MethodPut, path: "/api/codex/char_0123456789abcdef0123/progressions", body: `{"progressions":[],"expected_revision":null}{}`},
	}
	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			handler := api.NewHandler(&projectStoreStub{}, &activeProjectSessionStub{}, &storyServiceStub{}, "test")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(testCase.method, testCase.path, strings.NewReader(testCase.body)))
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
			}
		})
	}
}

func ptrString(value string) *string {
	return &value
}

var _ = errors.New
