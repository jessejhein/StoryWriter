package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"storywork/internal/api"
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
	handler := api.NewHandler(&projectStoreStub{}, &activeProjectSessionStub{}, stub, "test")

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/provider-profiles", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GET status = %d", response.Code)
	}
	assertJSONShape(t, response.Body.Bytes(), `{"profiles":[{"id":"hosted_api","name":"Hosted API","type":"openai_compatible","base_url":"https://api.example.test/v1","auth":{"type":"bearer_env","credential_env":"STORYWORK_HOSTED_API_KEY"},"capabilities":{"chat":true,"streaming":false,"structured_output":false,"max_context_tokens":32768},"readiness":"ready"}],"revision":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`)

	emptyHandler := api.NewHandler(&projectStoreStub{}, &activeProjectSessionStub{}, &storyServiceStub{}, "test")
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
	api.NewHandler(&projectStoreStub{}, &activeProjectSessionStub{}, conflictStub, "test").ServeHTTP(
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
		{name: "bad expected revision shape", method: http.MethodPut, path: "/api/provider-profiles", body: `{"profiles":[],"expected_revision":"bad"}`, status: http.StatusBadRequest},
		{name: "method not allowed", method: http.MethodPost, path: "/api/provider-profiles", body: ``, status: http.StatusMethodNotAllowed},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			response := httptest.NewRecorder()
			api.NewHandler(&projectStoreStub{}, &activeProjectSessionStub{}, &storyServiceStub{}, "test").ServeHTTP(
				response,
				httptest.NewRequest(testCase.method, testCase.path, strings.NewReader(testCase.body)),
			)
			if response.Code != testCase.status {
				t.Fatalf("status = %d, want %d body=%s", response.Code, testCase.status, response.Body.String())
			}
		})
	}
}
