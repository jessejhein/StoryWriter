// BDD Scenario: 7.1.1 - Preview minimal Line Polish context
// Requirements: M7-R09, M7-R17
// Test purpose: Context preview HTTP route returns redacted manifests without side effects.

package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"storywork/internal/action"
	"storywork/internal/contextpack"
)

// Test: context preview route returns exact manifest JSON.
// Requirements: M7-R17.
func TestContextPreviewRouteReturnsExactManifestJSON(t *testing.T) {
	t.Parallel()

	stub := &storyServiceStub{previewResult: action.ContextPreviewResult{
		Manifest: contextpack.Manifest{
			Scope: contextpack.ScopeSelection,
			PacksUsed: []contextpack.Pack{contextpack.PackSelectedText, contextpack.PackStyleSheet},
			PacksOmitted: []contextpack.PackOmission{},
			EstimatedInputTokens: 42,
			MaxInputEstimatedTokens: 8000,
			RAGMode: contextpack.RAGModeNone,
		},
		TargetRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}}
	handler := newTestHandler(&projectStoreStub{}, &activeProjectSessionStub{}, stub, "test")
	body := `{"agent_id":"line_polish","style_id":"precise_editor","surface":"editor","input_scope":"selection","scene_id":"scn_0123456789abcdef0123","scene_revision":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","selection":{"start_byte":0,"end_byte":10,"text":"Alpha beta"}}`
	request := httptest.NewRequest(http.MethodPost, "/api/actions/context-preview", strings.NewReader(body))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	expected := `{"manifest":{"scope":"selection","packs_used":["selected_text","style_sheet"],"packs_omitted":[],"estimated_input_tokens":42,"max_input_estimated_tokens":8000,"rag_mode":"none"},"target_revision":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`
	assertJSONShape(t, response.Body.Bytes(), expected)
}

// Test: context preview route accepts every tagged scope.
// Requirements: M7-R17.
func TestContextPreviewRouteAcceptsEveryTaggedScope(t *testing.T) {
	t.Parallel()

	stub := &storyServiceStub{previewResult: action.ContextPreviewResult{
		TargetRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}}
	handler := newTestHandler(&projectStoreStub{}, &activeProjectSessionStub{}, stub, "test")
	for _, body := range []string{
		`{"agent_id":"line_polish","style_id":"precise_editor","scope":"selection","target":{"scene_id":"scn_0123456789abcdef0123","scene_revision":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","start_byte":0,"end_byte":10,"text":"Alpha beta"}}`,
		`{"agent_id":"scene_rewrite","style_id":"precise_editor","scope":"scene","target":{"scene_id":"scn_0123456789abcdef0123","scene_revision":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}`,
		`{"agent_id":"chapter_review","style_id":"precise_editor","scope":"chapter_review","target":{"chapter_id":"ch_0123456789abcdef0123","fingerprint":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}}`,
	} {
		request := httptest.NewRequest(http.MethodPost, "/api/actions/context-preview", strings.NewReader(body))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("body %s status = %d, want 200", body, response.Code)
		}
	}
}

// Test: context preview route preserves legacy selection body.
// Requirements: M7-R19.
func TestContextPreviewRoutePreservesLegacySelectionBody(t *testing.T) {
	t.Parallel()

	stub := &storyServiceStub{previewResult: action.ContextPreviewResult{TargetRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}
	handler := newTestHandler(&projectStoreStub{}, &activeProjectSessionStub{}, stub, "test")
	body := `{"agent_id":"line_polish","style_id":"precise_editor","surface":"editor","input_scope":"selection","scene_id":"scn_0123456789abcdef0123","scene_revision":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","selection":{"start_byte":0,"end_byte":10,"text":"Alpha beta"}}`
	request := httptest.NewRequest(http.MethodPost, "/api/actions/context-preview", strings.NewReader(body))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	if stub.previewRequest.Target.Scope != contextpack.ScopeSelection || stub.previewRequest.Target.Selection == nil {
		t.Fatalf("preview request = %#v", stub.previewRequest)
	}
}

// Test: context preview route rejects strict JSON violations.
// Requirements: M7-R17.
func TestContextPreviewRouteRejectsStrictJSONViolations(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(&projectStoreStub{}, &activeProjectSessionStub{}, &storyServiceStub{}, "test")
	request := httptest.NewRequest(http.MethodPost, "/api/actions/context-preview", strings.NewReader(`{"agent_id":"line_polish","style_id":"precise_editor","extra":true}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
}

// Test: context preview route maps documented statuses and Allow header.
// Requirements: M7-R17.
func TestContextPreviewRouteMapsAllDocumentedStatusesAndAllow(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(&projectStoreStub{}, &activeProjectSessionStub{}, &storyServiceStub{previewErr: action.ErrAgentNotFound}, "test")
	request := httptest.NewRequest(http.MethodGet, "/api/actions/context-preview", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != "POST" {
		t.Fatalf("method status = %d allow=%q", response.Code, response.Header().Get("Allow"))
	}
	request = httptest.NewRequest(http.MethodPost, "/api/actions/context-preview", strings.NewReader(`{"agent_id":"missing","style_id":"precise_editor","scope":"selection","target":{"scene_id":"scn_0123456789abcdef0123","scene_revision":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","start_byte":0,"end_byte":10,"text":"Alpha beta"}}`))
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("missing agent status = %d", response.Code)
	}
}

// Test: context preview response does not leak packet content.
// Requirements: M7-R17.
func TestContextPreviewResponseDoesNotLeakPacketContent(t *testing.T) {
	t.Parallel()

	stub := &storyServiceStub{previewResult: action.ContextPreviewResult{
		Manifest: contextpack.Manifest{
			Scope: contextpack.ScopeSelection,
			PacksUsed: []contextpack.Pack{contextpack.PackSelectedText, contextpack.PackStyleSheet},
			RAGMode: contextpack.RAGModeNone,
		},
		TargetRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}}
	handler := newTestHandler(&projectStoreStub{}, &activeProjectSessionStub{}, stub, "test")
	body := `{"agent_id":"line_polish","style_id":"precise_editor","scope":"selection","target":{"scene_id":"scn_0123456789abcdef0123","scene_revision":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","start_byte":0,"end_byte":10,"text":"Alpha beta"}}`
	request := httptest.NewRequest(http.MethodPost, "/api/actions/context-preview", strings.NewReader(body))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	payload := response.Body.String()
	for _, forbidden := range []string{"Alpha beta", "system_prompt", "selected_text_content", "packet"} {
		if strings.Contains(payload, forbidden) {
			t.Fatalf("response leaked %q: %s", forbidden, payload)
		}
	}
}