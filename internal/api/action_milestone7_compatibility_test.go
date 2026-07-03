package api_test

// BDD Scenario: characterization guard for legacy action run JSON
// Requirements: M7-R19
// Test purpose: Ensure Milestone 4-6 selection run bodies remain accepted while
// Milestone 7 adds tagged scope targets.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"storywork/internal/action"
	"storywork/internal/agent"
)

// Test: legacy Milestone 4-6 action run JSON remains accepted without scope fields.
// Requirements: M7-R19.
func TestM7LegacyActionRunJSONRemainsAccepted(t *testing.T) {
	t.Parallel()

	const legacyBody = `{"agent_id":"line_polish","style_id":"precise_editor","surface":"editor","input_scope":"selection","scene_id":"scn_0123456789abcdef0123","scene_revision":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","selection":{"start_byte":120,"end_byte":640,"text":"Selected prose..."}}`

	stub := &storyServiceStub{
		actionRun: action.Run{
			RunID:         "run_0123456789abcdef0123",
			Status:        action.RunPending,
			AgentID:       "line_polish",
			StyleID:       "precise_editor",
			SceneID:       "scn_0123456789abcdef0123",
			SceneRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Selection:     action.Selection{StartByte: 120, EndByte: 640},
			OriginalText:  "Selected prose...",
			Replacement:   "Mock polished: Selected prose...",
			ContextSummary: agent.ContextSummary{
				PacksUsed: []agent.ContextPack{agent.ContextSelectedText, agent.ContextStyleSheet},
				RAGMode:   agent.RAGModeNone,
			},
		},
	}
	handler := newTestHandler(&projectStoreStub{}, &activeProjectSessionStub{}, stub, "test")

	request := httptest.NewRequest(http.MethodPost, "/api/actions/run", strings.NewReader(legacyBody))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d body=%s", response.Code, http.StatusCreated, response.Body.String())
	}
	expectedJSON := `{"run_id":"run_0123456789abcdef0123","status":"pending","agent_id":"line_polish","style_id":"precise_editor","scene_id":"scn_0123456789abcdef0123","scene_revision":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","selection":{"start_byte":120,"end_byte":640},"output_mode":"patch","patch":{"original":"Selected prose...","replacement":"Mock polished: Selected prose..."},"context_summary":{"packs_used":["selected_text","style_sheet"],"rag_mode":"none"},"provider":{"profile_id":"","type":"","model":""}}`
	assertJSONShape(t, response.Body.Bytes(), expectedJSON)

	runRequest := stub.actionRunRequest
	if runRequest.AgentID != "line_polish" || runRequest.StyleID != "precise_editor" {
		t.Fatalf("run request agent/style = %#v", runRequest)
	}
	if runRequest.Surface != agent.SurfaceEditor || runRequest.InputScope != agent.InputScopeSelection {
		t.Fatalf("run request surface/scope = %#v", runRequest)
	}
	if runRequest.SceneID != "scn_0123456789abcdef0123" || runRequest.SceneRevision == "" {
		t.Fatalf("run request scene = %#v", runRequest)
	}
	if runRequest.Selection.StartByte != 120 || runRequest.Selection.EndByte != 640 || runRequest.Selection.Text != "Selected prose..." {
		t.Fatalf("run request selection = %#v", runRequest.Selection)
	}
	if strings.Contains(legacyBody, `"scope"`) || strings.Contains(response.Body.String(), `"scope"`) {
		t.Fatalf("legacy request/response unexpectedly required scope field")
	}
}
