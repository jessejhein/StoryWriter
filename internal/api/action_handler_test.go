package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"storywork/internal/action"
	"storywork/internal/agent"
	"storywork/internal/api"
	"storywork/internal/story"
)

// BDD trace:
//   - Requirements: M4-R01, M4-R02, M4-R04, M4-R05, M4-R11, M4-R16.
//   - Scenario: 4.1.1, 4.2.1, 4.3.2, 4.4.1, 4.4.2.
//   - Test purpose: verify the exact JSON shapes for registry, availability,
//     run, accept, and reject routes plus query parsing and strict body mapping.
func TestActionRoutesReturnExactJSONShapes(t *testing.T) {
	t.Parallel()

	stub := &storyServiceStub{
		agents: []agent.Agent{{
			ID:          "line_polish",
			Name:        "Line Polish",
			Description: "Rewrite selected prose.",
			AppliesWhen: agent.ApplicabilityRule{
				Surfaces:    []agent.Surface{agent.SurfaceEditor},
				InputScopes: []agent.InputScope{agent.InputScopeSelection},
				MinWords:    20,
				MaxWords:    1500,
			},
			ContextPolicy: agent.ContextPolicy{
				Required:  []agent.ContextPack{agent.ContextSelectedText, agent.ContextStyleSheet},
				Optional:  []agent.ContextPack{agent.ContextSurrounding},
				Forbidden: []agent.ContextPack{agent.ContextGlobalCodexRAG, agent.ContextRawImportNotes},
			},
			RAGPolicy: agent.RAGPolicy{Mode: agent.RAGModeNone},
			Control:   agent.Control{OutputMode: agent.OutputModePatch, RequiresAcceptance: true},
		}},
		styles: []agent.Style{{
			ID:                "precise_editor",
			Name:              "Precise Editor",
			ProviderProfileID: "mock_default",
			Model:             "mock",
			Temperature:       0.2,
			SystemPrompt:      "You are a careful prose editor.",
		}},
		availableActions: []action.AvailableAction{{
			AgentID:            "line_polish",
			Name:               "Line Polish",
			Description:        "Rewrite selected prose.",
			OutputMode:         "patch",
			RequiresAcceptance: true,
			StyleIDs:           []string{"precise_editor"},
		}},
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
		actionAcceptRun: action.Run{
			RunID:  "run_0123456789abcdef0123",
			Status: action.RunAccepted,
		},
		actionAcceptScene: story.SceneDocument{
			ID:        "scn_0123456789abcdef0123",
			ChapterID: "ch_0123456789abcdef0123",
			Title:     "The Duel",
			FrontMatter: story.SceneFrontMatter{
				POV:           "Luke",
				Status:        "draft",
				ExcludeFromAI: false,
			},
			Markdown: "Updated prose...",
			Revision: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		},
		actionRejectRun: action.Run{
			RunID:  "run_0123456789abcdef0123",
			Status: action.RunRejected,
		},
	}
	handler := api.NewHandler(&projectStoreStub{}, &activeProjectSessionStub{}, stub, "test")

	for _, testCase := range []struct {
		name         string
		method       string
		path         string
		body         string
		status       int
		expectedJSON string
	}{
		{
			name:         "list agents",
			method:       http.MethodGet,
			path:         "/api/agents",
			status:       http.StatusOK,
			expectedJSON: `{"agents":[{"id":"line_polish","name":"Line Polish","description":"Rewrite selected prose.","surfaces":["editor"],"input_scopes":["selection"],"min_words":20,"max_words":1500,"required_context":["selected_text","style_sheet"],"optional_context":["surrounding_paragraphs"],"forbidden_context":["global_codex_rag","raw_import_notes"],"rag_mode":"none","output_mode":"patch","requires_acceptance":true}]}`,
		},
		{
			name:         "list styles",
			method:       http.MethodGet,
			path:         "/api/styles",
			status:       http.StatusOK,
			expectedJSON: `{"styles":[{"id":"precise_editor","name":"Precise Editor","provider_profile_id":"mock_default","model":"mock","temperature":0.2,"system_prompt":"You are a careful prose editor."}]}`,
		},
		{
			name:         "available actions",
			method:       http.MethodGet,
			path:         "/api/actions/available?surface=editor&input_scope=selection&scene_id=scn_0123456789abcdef0123&selection_words=200",
			status:       http.StatusOK,
			expectedJSON: `{"actions":[{"agent_id":"line_polish","name":"Line Polish","description":"Rewrite selected prose.","output_mode":"patch","requires_acceptance":true,"style_ids":["precise_editor"]}]}`,
		},
		{
			name:         "run action",
			method:       http.MethodPost,
			path:         "/api/actions/run",
			body:         `{"agent_id":"line_polish","style_id":"precise_editor","surface":"editor","input_scope":"selection","scene_id":"scn_0123456789abcdef0123","scene_revision":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","selection":{"start_byte":120,"end_byte":640,"text":"Selected prose..."}}`,
			status:       http.StatusCreated,
			expectedJSON: `{"run_id":"run_0123456789abcdef0123","status":"pending","agent_id":"line_polish","style_id":"precise_editor","scene_id":"scn_0123456789abcdef0123","scene_revision":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","selection":{"start_byte":120,"end_byte":640},"output_mode":"patch","patch":{"original":"Selected prose...","replacement":"Mock polished: Selected prose..."},"context_summary":{"packs_used":["selected_text","style_sheet"],"rag_mode":"none"}}`,
		},
		{
			name:         "accept action",
			method:       http.MethodPost,
			path:         "/api/actions/run_0123456789abcdef0123/accept",
			body:         `{"expected_revision":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`,
			status:       http.StatusOK,
			expectedJSON: `{"run_id":"run_0123456789abcdef0123","status":"accepted","scene":{"id":"scn_0123456789abcdef0123","chapter_id":"ch_0123456789abcdef0123","title":"The Duel","frontmatter":{"pov":"Luke","status":"draft","exclude_from_ai":false},"markdown":"Updated prose...","revision":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}}`,
		},
		{
			name:         "reject action",
			method:       http.MethodPost,
			path:         "/api/actions/run_0123456789abcdef0123/reject",
			status:       http.StatusOK,
			expectedJSON: `{"run_id":"run_0123456789abcdef0123","status":"rejected"}`,
		},
	} {
		request := httptest.NewRequest(testCase.method, testCase.path, strings.NewReader(testCase.body))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != testCase.status {
			t.Fatalf("%s status = %d, want %d", testCase.name, response.Code, testCase.status)
		}
		assertJSONShape(t, response.Body.Bytes(), testCase.expectedJSON)
	}

	runRequest := stub.actionRunRequest
	if runRequest.AgentID != "line_polish" || runRequest.Selection.StartByte != 120 || runRequest.Selection.EndByte != 640 {
		t.Fatalf("run request = %#v", runRequest)
	}
	if stub.actionAcceptRunID != "run_0123456789abcdef0123" || stub.actionAcceptRevision == "" {
		t.Fatalf("accept call = %q %q", stub.actionAcceptRunID, stub.actionAcceptRevision)
	}
	if stub.actionRejectRunID != "run_0123456789abcdef0123" {
		t.Fatalf("reject run id = %q", stub.actionRejectRunID)
	}
}

// BDD trace:
//   - Requirements: M4-R03, M4-R09, M4-R15, M4-R16.
//   - Scenario: 4.1.2, 4.3.3, 4.4.1, 4.4.3.
//   - Test purpose: verify malformed query/input, conflict, not-found, and
//     service-unavailable conditions map to the documented HTTP statuses.
func TestActionRouteStatusMapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		method string
		path   string
		body   string
		stub   storyServiceStub
		status int
	}{
		{name: "invalid availability selection words", method: http.MethodGet, path: "/api/actions/available?surface=editor&input_scope=selection&scene_id=scn_1&selection_words=oops", status: http.StatusBadRequest},
		{name: "invalid availability negative selection words", method: http.MethodGet, path: "/api/actions/available?surface=editor&input_scope=selection&scene_id=scn_1&selection_words=-1", status: http.StatusBadRequest},
		{name: "invalid availability surface", method: http.MethodGet, path: "/api/actions/available?surface=codex&input_scope=selection&scene_id=scn_1&selection_words=200", status: http.StatusBadRequest},
		{name: "invalid availability input scope", method: http.MethodGet, path: "/api/actions/available?surface=editor&input_scope=scene&scene_id=scn_1&selection_words=200", status: http.StatusBadRequest},
		{name: "missing availability scene id", method: http.MethodGet, path: "/api/actions/available?surface=editor&input_scope=selection&selection_words=200", status: http.StatusBadRequest},
		{name: "registry failure on list", method: http.MethodGet, path: "/api/agents", stub: storyServiceStub{agentsErr: agent.ErrRegistryLoad}, status: http.StatusInternalServerError},
		{name: "agent missing on run", method: http.MethodPost, path: "/api/actions/run", body: `{"agent_id":"line_polish","style_id":"precise_editor","surface":"editor","input_scope":"selection","scene_id":"scn_0123456789abcdef0123","scene_revision":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","selection":{"start_byte":0,"end_byte":5,"text":"Alpha"}}`, stub: storyServiceStub{actionRunErr: action.ErrAgentNotFound}, status: http.StatusNotFound},
		{name: "selection mismatch on run", method: http.MethodPost, path: "/api/actions/run", body: `{"agent_id":"line_polish","style_id":"precise_editor","surface":"editor","input_scope":"selection","scene_id":"scn_0123456789abcdef0123","scene_revision":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","selection":{"start_byte":0,"end_byte":5,"text":"Alpha"}}`, stub: storyServiceStub{actionRunErr: story.ErrInvalidSelection}, status: http.StatusBadRequest},
		{name: "capacity on run", method: http.MethodPost, path: "/api/actions/run", body: `{"agent_id":"line_polish","style_id":"precise_editor","surface":"editor","input_scope":"selection","scene_id":"scn_0123456789abcdef0123","scene_revision":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","selection":{"start_byte":0,"end_byte":5,"text":"Alpha"}}`, stub: storyServiceStub{actionRunErr: action.ErrRunCapacity}, status: http.StatusServiceUnavailable},
		{name: "accept conflict", method: http.MethodPost, path: "/api/actions/run_0123456789abcdef0123/accept", body: `{"expected_revision":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`, stub: storyServiceStub{actionAcceptErr: action.ErrRunConflict}, status: http.StatusConflict},
		{name: "reject missing", method: http.MethodPost, path: "/api/actions/run_0123456789abcdef0123/reject", stub: storyServiceStub{actionRejectErr: action.ErrRunNotFound}, status: http.StatusNotFound},
	}

	for _, testCase := range tests {
		handler := api.NewHandler(&projectStoreStub{}, &activeProjectSessionStub{}, &testCase.stub, "test")
		request := httptest.NewRequest(testCase.method, testCase.path, strings.NewReader(testCase.body))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != testCase.status {
			t.Fatalf("%s status = %d, want %d (body=%s)", testCase.name, response.Code, testCase.status, response.Body.String())
		}
	}

	handler := api.NewHandler(&projectStoreStub{}, &activeProjectSessionStub{}, &storyServiceStub{}, "test")
	for _, testCase := range []struct {
		path  string
		allow string
	}{
		{path: "/api/actions/available", allow: "GET"},
		{path: "/api/actions/run", allow: "POST"},
		{path: "/api/actions/run_0123456789abcdef0123/accept", allow: "POST"},
		{path: "/api/actions/run_0123456789abcdef0123/reject", allow: "POST"},
	} {
		request := httptest.NewRequest(http.MethodPut, testCase.path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusMethodNotAllowed {
			t.Fatalf("method not allowed status = %d, want 405", response.Code)
		}
		if allow := response.Header().Get("Allow"); allow != testCase.allow {
			t.Fatalf("Allow = %q, want %q", allow, testCase.allow)
		}
	}
}

func assertJSONShape(t *testing.T, body []byte, expected string) {
	t.Helper()

	var got any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("json.Unmarshal(got) error = %v; body=%s", err, string(body))
	}
	var want any
	if err := json.Unmarshal([]byte(expected), &want); err != nil {
		t.Fatalf("json.Unmarshal(want) error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("JSON body = %s, want %s", string(body), expected)
	}
}
