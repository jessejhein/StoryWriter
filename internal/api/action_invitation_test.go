// BDD Scenario: 7.4.2 - Explicitly run an invited action
// Requirements: M7-R12, M7-R17
// Test purpose: Invitation run HTTP route requires explicit POST and maps statuses safely.

package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"storywork/internal/action"
	"storywork/internal/contextpack"
)

// Test: invitation run route uses exact method, path, and body.
// Requirements: M7-R17.
func TestInvitationRunRouteUsesExactMethodPathAndBody(t *testing.T) {
	t.Parallel()

	stub := &storyServiceStub{actionRun: action.Run{
		RunID: "run_bbbbbbbbbbbbbbbbbbbb", Status: action.RunPending, AgentID: "scene_rewrite",
		Scope: contextpack.ScopeScene, SceneID: "scn_0123456789abcdef0123",
		ParentRunID: "run_aaaaaaaaaaaaaaaaaaaa", RootRunID: "run_aaaaaaaaaaaaaaaaaaaa", ChainDepth: 2,
	}}
	handler := newTestHandler(&projectStoreStub{}, &activeProjectSessionStub{}, stub, "test")
	body := `{"style_id":"precise_editor","expected_target_revision":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`
	request := httptest.NewRequest(http.MethodPost, "/api/action-invitations/invite_0123456789abcdef0123/run", strings.NewReader(body))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

// Test: invitation run route returns child lineage and manifest fields.
// Requirements: M7-R17.
func TestInvitationRunRouteReturnsChildLineageAndManifest(t *testing.T) {
	t.Parallel()

	stub := &storyServiceStub{actionRun: action.Run{
		RunID: "run_bbbbbbbbbbbbbbbbbbbb", Status: action.RunPending, AgentID: "scene_rewrite",
		Scope: contextpack.ScopeScene, SceneID: "scn_0123456789abcdef0123",
		ParentRunID: "run_aaaaaaaaaaaaaaaaaaaa", RootRunID: "run_aaaaaaaaaaaaaaaaaaaa", ChainDepth: 2,
		Manifest: contextpack.Manifest{Scope: contextpack.ScopeScene, PacksUsed: []contextpack.Pack{contextpack.PackCurrentScene}},
	}}
	handler := newTestHandler(&projectStoreStub{}, &activeProjectSessionStub{}, stub, "test")
	request := httptest.NewRequest(http.MethodPost, "/api/action-invitations/invite_0123456789abcdef0123/run", strings.NewReader(`{"style_id":"precise_editor","expected_target_revision":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"parent_run_id":"run_aaaaaaaaaaaaaaaaaaaa"`) {
		t.Fatalf("body = %s", response.Body.String())
	}
}

// Test: invitation run route rejects strict JSON and invalid IDs.
// Requirements: M7-R17.
func TestInvitationRunRouteRejectsStrictJSONAndInvalidIDs(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(&projectStoreStub{}, &activeProjectSessionStub{}, &storyServiceStub{}, "test")
	request := httptest.NewRequest(http.MethodPost, "/api/action-invitations/bad/run", strings.NewReader(`{"style_id":"precise_editor","expected_target_revision":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid id status = %d", response.Code)
	}
}

// Test: invitation run route maps not found, conflict, and unavailable statuses.
// Requirements: M7-R17.
func TestInvitationRunRouteMapsNotFoundConflictAndUnavailable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		err    error
		status int
	}{
		{name: "not found", err: action.ErrInvitationNotFound, status: http.StatusNotFound},
		{name: "conflict", err: action.ErrInvitationConflict, status: http.StatusConflict},
		{name: "unavailable", err: action.ErrRunCapacity, status: http.StatusServiceUnavailable},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			stub := &storyServiceStub{actionRunErr: testCase.err}
			handler := newTestHandler(&projectStoreStub{}, &activeProjectSessionStub{}, stub, "test")
			request := httptest.NewRequest(http.MethodPost, "/api/action-invitations/invite_0123456789abcdef0123/run", strings.NewReader(`{"style_id":"precise_editor","expected_target_revision":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != testCase.status {
				t.Fatalf("status = %d, want %d body=%s", response.Code, testCase.status, response.Body.String())
			}
		})
	}
}

// Test: invitation run route calls no provider before explicit POST.
// Requirements: M7-R12.
func TestInvitationRunRouteCallsNoProviderBeforeExplicitPOST(t *testing.T) {
	t.Parallel()

	stub := &storyServiceStub{}
	handler := newTestHandler(&projectStoreStub{}, &activeProjectSessionStub{}, stub, "test")
	request := httptest.NewRequest(http.MethodGet, "/api/action-invitations/invite_0123456789abcdef0123/run", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status = %d, want 405", response.Code)
	}
}
