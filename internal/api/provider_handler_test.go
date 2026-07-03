package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"storywork/internal/provider"
)

// BDD trace:
//   - Requirements: M5-R12, M5-R13.
//   - Scenarios: 5.1.1, 5.1.2, 5.1.4.
//   - Test purpose: verify exact public provider-profile GET/PUT JSON shapes,
//     null revision handling, readiness exposure, and that these routes work
//     without an active story project.
func TestProviderProfileRoutesReturnExactJSONShapes(t *testing.T) {
	t.Parallel()

	revision := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	stub := &storyServiceStub{
		providerProfiles: []provider.Profile{
			{
				ID:      "hosted_api",
				Name:    "Hosted API",
				Type:    provider.TypeOpenAICompatible,
				BaseURL: "https://api.example.test/v1",
				Auth: provider.AuthConfig{
					Type:          provider.AuthTypeBearerEnv,
					CredentialEnv: "STORYWORK_HOSTED_API_KEY",
				},
				Capabilities: provider.Capabilities{
					Chat:             true,
					Streaming:        false,
					StructuredOutput: false,
					MaxContextTokens: 32768,
				},
				Readiness: provider.ReadinessReady,
			},
		},
		providerRevision: &revision,
	}
	handler := newTestHandler(&projectStoreStub{}, &activeProjectSessionStub{}, stub, "test")

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/provider-profiles", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GET status = %d", response.Code)
	}
	assertJSONShape(t, response.Body.Bytes(), `{"profiles":[{"id":"hosted_api","name":"Hosted API","type":"openai_compatible","base_url":"https://api.example.test/v1","auth":{"type":"bearer_env","credential_env":"STORYWORK_HOSTED_API_KEY"},"capabilities":{"chat":true,"streaming":false,"structured_output":false,"max_context_tokens":32768},"readiness":"ready"}],"revision":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`)

	emptyHandler := newTestHandler(&projectStoreStub{}, &activeProjectSessionStub{}, &storyServiceStub{}, "test")
	response = httptest.NewRecorder()
	emptyHandler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/provider-profiles", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GET empty status = %d", response.Code)
	}
	assertJSONShape(t, response.Body.Bytes(), `{"profiles":[],"revision":null}`)

	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/api/provider-profiles", strings.NewReader(`{"profiles":[{"id":"local_ollama","name":"Local Ollama","type":"ollama","base_url":"http://127.0.0.1:11434","auth":{"type":"none","credential_env":""},"capabilities":{"chat":true,"streaming":false,"structured_output":false,"max_context_tokens":8192}}],"expected_revision":null}`)))
	if response.Code != http.StatusOK {
		t.Fatalf("PUT status = %d body=%s", response.Code, response.Body.String())
	}
	if stub.saveProviderExpected != nil {
		t.Fatalf("SaveProviderProfiles expected revision = %v, want nil", stub.saveProviderExpected)
	}
	if len(stub.saveProviderInput) != 1 || stub.saveProviderInput[0].ID != "local_ollama" {
		t.Fatalf("SaveProviderProfiles input = %#v", stub.saveProviderInput)
	}
}

// BDD trace:
//   - Requirements: M5-R12.
//   - Scenario: 5.1.3.
//   - Test purpose: verify provider-profile JSON validation, 413 body limits,
//     stale revision mapping, method handling, and safe error statuses.
func TestProviderProfileRouteValidationAndStatusMapping(t *testing.T) {
	t.Parallel()

	conflictStub := &storyServiceStub{saveProviderErr: provider.ErrProfileRevisionConflict}
	response := httptest.NewRecorder()
	newTestHandler(&projectStoreStub{}, &activeProjectSessionStub{}, conflictStub, "test").ServeHTTP(
		response,
		httptest.NewRequest(http.MethodPut, "/api/provider-profiles", strings.NewReader(`{"profiles":[],"expected_revision":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`)),
	)
	if response.Code != http.StatusConflict {
		t.Fatalf("PUT conflict status = %d", response.Code)
	}

	tests := []struct {
		name   string
		method string
		path   string
		body   string
		status int
	}{
		{name: "bad json", method: http.MethodPut, path: "/api/provider-profiles", body: `{`, status: http.StatusBadRequest},
		{name: "missing profiles", method: http.MethodPut, path: "/api/provider-profiles", body: `{"expected_revision":null}`, status: http.StatusBadRequest},
		{name: "unknown field", method: http.MethodPut, path: "/api/provider-profiles", body: `{"profiles":[],"expected_revision":null,"extra":true}`, status: http.StatusBadRequest},
		{name: "unknown field in profile", method: http.MethodPut, path: "/api/provider-profiles", body: `{"profiles":[{"id":"local","name":"Local","type":"ollama","base_url":"http://127.0.0.1:11434","extra":true,"auth":{"type":"none","credential_env":""},"capabilities":{"chat":true,"streaming":false,"structured_output":false,"max_context_tokens":8192}}],"expected_revision":null}`, status: http.StatusBadRequest},
		{name: "unknown field in auth", method: http.MethodPut, path: "/api/provider-profiles", body: `{"profiles":[{"id":"local","name":"Local","type":"ollama","base_url":"http://127.0.0.1:11434","auth":{"type":"none","credential_env":"","extra":true},"capabilities":{"chat":true,"streaming":false,"structured_output":false,"max_context_tokens":8192}}],"expected_revision":null}`, status: http.StatusBadRequest},
		{name: "unknown field in capabilities", method: http.MethodPut, path: "/api/provider-profiles", body: `{"profiles":[{"id":"local","name":"Local","type":"ollama","base_url":"http://127.0.0.1:11434","auth":{"type":"none","credential_env":""},"capabilities":{"chat":true,"streaming":false,"structured_output":false,"max_context_tokens":8192,"extra":true}}],"expected_revision":null}`, status: http.StatusBadRequest},
		{name: "missing nested credential env", method: http.MethodPut, path: "/api/provider-profiles", body: `{"profiles":[{"id":"local","name":"Local","type":"ollama","base_url":"http://127.0.0.1:11434","auth":{"type":"none"},"capabilities":{"chat":true,"streaming":false,"structured_output":false,"max_context_tokens":8192}}],"expected_revision":null}`, status: http.StatusBadRequest},
		{name: "missing nested capability", method: http.MethodPut, path: "/api/provider-profiles", body: `{"profiles":[{"id":"local","name":"Local","type":"ollama","base_url":"http://127.0.0.1:11434","auth":{"type":"none","credential_env":""},"capabilities":{"chat":true,"structured_output":false,"max_context_tokens":8192}}],"expected_revision":null}`, status: http.StatusBadRequest},
		{name: "null profile entry", method: http.MethodPut, path: "/api/provider-profiles", body: `{"profiles":[null],"expected_revision":null}`, status: http.StatusBadRequest},
		{name: "null auth", method: http.MethodPut, path: "/api/provider-profiles", body: `{"profiles":[{"id":"local","name":"Local","type":"ollama","base_url":"http://127.0.0.1:11434","auth":null,"capabilities":{"chat":true,"streaming":false,"structured_output":false,"max_context_tokens":8192}}],"expected_revision":null}`, status: http.StatusBadRequest},
		{name: "null credential env", method: http.MethodPut, path: "/api/provider-profiles", body: `{"profiles":[{"id":"local","name":"Local","type":"ollama","base_url":"http://127.0.0.1:11434","auth":{"type":"none","credential_env":null},"capabilities":{"chat":true,"streaming":false,"structured_output":false,"max_context_tokens":8192}}],"expected_revision":null}`, status: http.StatusBadRequest},
		{name: "wrong nested type", method: http.MethodPut, path: "/api/provider-profiles", body: `{"profiles":[{"id":"local","name":"Local","type":"ollama","base_url":"http://127.0.0.1:11434","auth":[],"capabilities":{"chat":true,"streaming":false,"structured_output":false,"max_context_tokens":8192}}],"expected_revision":null}`, status: http.StatusBadRequest},
		{name: "trailing JSON document", method: http.MethodPut, path: "/api/provider-profiles", body: `{"profiles":[],"expected_revision":null} {}`, status: http.StatusBadRequest},
		{name: "bad expected revision shape", method: http.MethodPut, path: "/api/provider-profiles", body: `{"profiles":[],"expected_revision":"bad"}`, status: http.StatusBadRequest},
		{name: "method not allowed", method: http.MethodPost, path: "/api/provider-profiles", body: ``, status: http.StatusMethodNotAllowed},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			stub := &storyServiceStub{}
			response := httptest.NewRecorder()
			newTestHandler(&projectStoreStub{}, &activeProjectSessionStub{}, stub, "test").ServeHTTP(
				response,
				httptest.NewRequest(testCase.method, testCase.path, strings.NewReader(testCase.body)),
			)
			if response.Code != testCase.status {
				t.Fatalf("status = %d, want %d body=%s", response.Code, testCase.status, response.Body.String())
			}
			if testCase.method == http.MethodPut && len(stub.saveProviderInput) != 0 {
				t.Fatalf("SaveProviderProfiles input = %#v, want no save attempt", stub.saveProviderInput)
			}
		})
	}
}

// Test: provider-profile PUT respects the documented 1 MiB body limit while
// still allowing a request whose encoded JSON is exactly at the boundary.
// Requirements: M5-R12.
func TestProviderProfileRouteEnforcesBodyLimitBoundary(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(&projectStoreStub{}, &activeProjectSessionStub{}, &storyServiceStub{}, "test")

	allowedBody := exactSizedProviderRequestBody(t, 1<<20)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/api/provider-profiles", strings.NewReader(allowedBody)))
	if response.Code != http.StatusBadRequest && response.Code != http.StatusOK {
		t.Fatalf("exact-limit status = %d body=%s", response.Code, response.Body.String())
	}
	if response.Code == http.StatusRequestEntityTooLarge {
		t.Fatalf("exact-limit status = %d, want request accepted by size gate", response.Code)
	}

	overLimitBody := allowedBody + " "
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/api/provider-profiles", strings.NewReader(overLimitBody)))
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("over-limit status = %d, want %d body=%s", response.Code, http.StatusRequestEntityTooLarge, response.Body.String())
	}
}

func exactSizedProviderRequestBody(t *testing.T, size int) string {
	t.Helper()

	prefix := `{"profiles":[{"id":"local","name":"`
	suffix := `","type":"ollama","base_url":"http://127.0.0.1:11434","auth":{"type":"none","credential_env":""},"capabilities":{"chat":true,"streaming":false,"structured_output":false,"max_context_tokens":8192}}],"expected_revision":null}`
	if len(prefix)+len(suffix) > size {
		t.Fatalf("fixed request frame %d exceeds target size %d", len(prefix)+len(suffix), size)
	}
	return prefix + strings.Repeat("a", size-len(prefix)-len(suffix)) + suffix
}
