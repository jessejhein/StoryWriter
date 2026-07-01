package agent

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"storywork/internal/provider"
)

// BDD trace:
//   - Requirements: M4-R07, M4-R08.
//   - Scenario: 4.3.2.
//   - Test purpose: verify the provider boundary is provider-neutral, supports
//     cancellation, and produces deterministic mock output that must differ from
//     the original selection.
func TestMockProviderProducesDeterministicReplacementAndHonorsCancellation(t *testing.T) {
	t.Parallel()

	agentDefinition := Agent{
		Version:     1,
		ID:          "line_polish",
		Name:        "Line Polish",
		Description: "Rewrite selected prose.",
		AppliesWhen: ApplicabilityRule{Surfaces: []Surface{SurfaceEditor}, InputScopes: []InputScope{InputScopeSelection}, MinWords: 20, MaxWords: 1500},
		ContextPolicy: ContextPolicy{
			Required:  []ContextPack{ContextSelectedText, ContextStyleSheet},
			Optional:  []ContextPack{ContextSurrounding},
			Forbidden: []ContextPack{ContextGlobalCodexRAG, ContextRawImportNotes},
		},
		RAGPolicy: RAGPolicy{Mode: RAGModeNone},
		Control:   Control{OutputMode: OutputModePatch, RequiresAcceptance: true},
		Output:    Output{Type: OutputTypeReplacementText, RequiresDiffPreview: true},
	}
	style := Style{
		Version:           1,
		ID:                "precise_editor",
		Name:              "Precise Editor",
		ProviderProfileID: "mock_default",
		Model:             "mock",
		Temperature:       0.2,
		SystemPrompt:      "Prompt",
	}
	provider := NewMockProvider()
	response, err := provider.Generate(context.Background(), GenerateRequest{
		Agent:   agentDefinition,
		Style:   style,
		Packet:  ContextPacket{SelectedText: "  Selected prose  ", Style: style},
		Summary: ContextSummary{PacksUsed: []ContextPack{ContextSelectedText, ContextStyleSheet}, RAGMode: RAGModeNone},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if got := response.Replacement; got != "Mock polished: Selected prose" {
		t.Fatalf("replacement = %q, want deterministic mock output", got)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := provider.Generate(ctx, GenerateRequest{
		Agent:   agentDefinition,
		Style:   style,
		Packet:  ContextPacket{SelectedText: "Selected prose", Style: style},
		Summary: ContextSummary{PacksUsed: []ContextPack{ContextSelectedText, ContextStyleSheet}, RAGMode: RAGModeNone},
	}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Generate(canceled) error = %v, want context.Canceled", err)
	}
}

type resolvedProfileStub struct {
	profile provider.ResolvedProfile
	found   bool
	err     error
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func (s resolvedProfileStub) Resolve(context.Context, string) (provider.ResolvedProfile, bool, error) {
	return s.profile, s.found, s.err
}

type closingReadCloser struct {
	reader io.Reader
	closed bool
}

func (c *closingReadCloser) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}

func (c *closingReadCloser) Close() error {
	c.closed = true
	return nil
}

// BDD trace:
//   - Requirements: M5-R05, M5-R06, M5-R07, M5-R10.
//   - Scenarios: 5.3.1, 5.3.2, 5.4.1.
//   - Test purpose: verify the dispatcher maps exact OpenAI-compatible and
//     Ollama requests, preserves the selected text verbatim, and returns stable
//     provider identity with safe output normalization.
func TestDispatcherRoutesOpenAICompatibleAndOllamaRequests(t *testing.T) {
	t.Parallel()

	linePolish := Agent{
		Version:     2,
		ID:          "line_polish",
		Name:        "Line Polish",
		Description: "Rewrite selected prose.",
		AppliesWhen: ApplicabilityRule{Surfaces: []Surface{SurfaceEditor}, InputScopes: []InputScope{InputScopeSelection}, MinWords: 20, MaxWords: 1500},
		ModelRequirements: ModelRequirements{
			MinContextTokens: 2048,
		},
		ContextPolicy: ContextPolicy{
			Required:  []ContextPack{ContextSelectedText, ContextStyleSheet},
			Optional:  []ContextPack{ContextSurrounding},
			Forbidden: []ContextPack{ContextGlobalCodexRAG, ContextRawImportNotes},
		},
		RAGPolicy: RAGPolicy{Mode: RAGModeNone},
		Control:   Control{OutputMode: OutputModePatch, RequiresAcceptance: true},
		Output:    Output{Type: OutputTypeReplacementText, RequiresDiffPreview: true},
	}
	openAIStyle := Style{
		Version:           2,
		ID:                "local_precise_editor",
		Name:              "Local Precise Editor",
		ProviderProfileID: "local_openai",
		Model:             "local-model-name",
		Temperature:       0.2,
		SystemPrompt:      "Style system prompt",
	}
	ollamaStyle := Style{
		Version:           2,
		ID:                "ollama_precise_editor",
		Name:              "Ollama Precise Editor",
		ProviderProfileID: "local_ollama",
		Model:             "ollama-model",
		Temperature:       0.4,
		SystemPrompt:      "Ollama prompt",
	}
	packet := ContextPacket{SelectedText: " Selected prose\r\nverbatim ", Style: openAIStyle}
	summary := ContextSummary{PacksUsed: []ContextPack{ContextSelectedText, ContextStyleSheet}, RAGMode: RAGModeNone}

	openAIClient := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.String() != "https://api.example.test/v1/chat/completions" {
				t.Fatalf("OpenAI URL = %q", request.URL.String())
			}
			if auth := request.Header.Get("Authorization"); auth != "Bearer test-key" {
				t.Fatalf("OpenAI Authorization = %q", auth)
			}
			if got := request.Header.Get("Content-Type"); got != "application/json" {
				t.Fatalf("OpenAI Content-Type = %q", got)
			}
			body, err := io.ReadAll(request.Body)
			if err != nil {
				t.Fatalf("ReadAll(OpenAI body) error = %v", err)
			}
			if !strings.Contains(string(body), `"stream":false`) || !strings.Contains(string(body), `"temperature":0.2`) {
				t.Fatalf("OpenAI body = %s", string(body))
			}
			if !strings.Contains(string(body), `Selected text:\n Selected prose\r\nverbatim `) {
				t.Fatalf("OpenAI selected text missing verbatim body = %s", string(body))
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"Rewritten\r\ntext"}}]}`)),
			}, nil
		})}

	dispatcher := NewDispatcher(resolvedProfileStub{
		profile: provider.ResolvedProfile{
			Profile: provider.Profile{
				ID:      "local_openai",
				Type:    provider.TypeOpenAICompatible,
				BaseURL: "https://api.example.test/v1",
				Auth:    provider.AuthConfig{Type: provider.AuthTypeBearerEnv, CredentialEnv: "STORYWORK_HOSTED_API_KEY"},
				Capabilities: provider.Capabilities{
					Chat:             true,
					MaxContextTokens: 8192,
				},
				Readiness: provider.ReadinessReady,
			},
			Credential: provider.Credential{Value: "test-key"},
		},
		found: true,
	}, openAIClient)
	response, err := dispatcher.Generate(context.Background(), GenerateRequest{
		Agent:   linePolish,
		Style:   openAIStyle,
		Packet:  packet,
		Summary: summary,
	})
	if err != nil {
		t.Fatalf("Generate(OpenAI) error = %v", err)
	}
	if response.Replacement != "Rewritten\ntext" {
		t.Fatalf("OpenAI replacement = %q", response.Replacement)
	}
	if response.Provider.ProfileID != "local_openai" || response.Provider.Type != provider.TypeOpenAICompatible || response.Provider.Model != "local-model-name" {
		t.Fatalf("OpenAI provider identity = %#v", response.Provider)
	}

	ollamaClient := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.String() != "http://127.0.0.1:11434/api/chat" {
				t.Fatalf("Ollama URL = %q", request.URL.String())
			}
			if auth := request.Header.Get("Authorization"); auth != "" {
				t.Fatalf("Ollama Authorization = %q", auth)
			}
			body, err := io.ReadAll(request.Body)
			if err != nil {
				t.Fatalf("ReadAll(Ollama body) error = %v", err)
			}
			if !strings.Contains(string(body), `"stream":false`) || !strings.Contains(string(body), `"temperature":0.4`) {
				t.Fatalf("Ollama body = %s", string(body))
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"message":{"content":"Ollama rewrite"}}`)),
			}, nil
		})}

	dispatcher = NewDispatcher(resolvedProfileStub{
		profile: provider.ResolvedProfile{
			Profile: provider.Profile{
				ID:      "local_ollama",
				Type:    provider.TypeOllama,
				BaseURL: "http://127.0.0.1:11434",
				Auth:    provider.AuthConfig{Type: provider.AuthTypeNone},
				Capabilities: provider.Capabilities{
					Chat:             true,
					MaxContextTokens: 8192,
				},
				Readiness: provider.ReadinessReady,
			},
		},
		found: true,
	}, ollamaClient)
	response, err = dispatcher.Generate(context.Background(), GenerateRequest{
		Agent:   linePolish,
		Style:   ollamaStyle,
		Packet:  ContextPacket{SelectedText: "Selected prose", Style: ollamaStyle},
		Summary: summary,
	})
	if err != nil {
		t.Fatalf("Generate(Ollama) error = %v", err)
	}
	if response.Replacement != "Ollama rewrite" || response.Provider.ProfileID != "local_ollama" {
		t.Fatalf("Ollama response = %#v", response)
	}
}

// BDD trace:
//   - Requirements: M5-R07.
//   - Scenarios: 5.3.3, 5.4.2.
//   - Test purpose: verify redirects, non-success statuses, malformed success
//     envelopes, and empty outputs fail with safe typed provider errors.
func TestDispatcherClassifiesUnsafeProviderFailures(t *testing.T) {
	t.Parallel()

	linePolish := Agent{
		Version:           2,
		ID:                "line_polish",
		Name:              "Line Polish",
		Description:       "Rewrite selected prose.",
		AppliesWhen:       ApplicabilityRule{Surfaces: []Surface{SurfaceEditor}, InputScopes: []InputScope{InputScopeSelection}, MinWords: 20, MaxWords: 1500},
		ModelRequirements: ModelRequirements{MinContextTokens: 2048},
		ContextPolicy: ContextPolicy{
			Required: []ContextPack{ContextSelectedText, ContextStyleSheet},
		},
		RAGPolicy: RAGPolicy{Mode: RAGModeNone},
		Control:   Control{OutputMode: OutputModePatch, RequiresAcceptance: true},
		Output:    Output{Type: OutputTypeReplacementText, RequiresDiffPreview: true},
	}
	style := Style{
		Version:           2,
		ID:                "local_precise_editor",
		Name:              "Local Precise Editor",
		ProviderProfileID: "local_openai",
		Model:             "local-model-name",
		Temperature:       0.2,
		SystemPrompt:      "Prompt",
	}

	makeDispatcher := func(statusCode int, contentType string, body string, roundTripErr error, header http.Header) *Dispatcher {
		client := &http.Client{
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
			Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				if roundTripErr != nil {
					return nil, roundTripErr
				}
				if header == nil {
					header = http.Header{}
				}
				if contentType != "" {
					header.Set("Content-Type", contentType)
				}
				return &http.Response{
					StatusCode: statusCode,
					Header:     header,
					Body:       io.NopCloser(bytes.NewBufferString(body)),
				}, nil
			})}
		return NewDispatcher(resolvedProfileStub{
			profile: provider.ResolvedProfile{
				Profile: provider.Profile{
					ID:      "local_openai",
					Type:    provider.TypeOpenAICompatible,
					BaseURL: "https://api.example.test",
					Auth:    provider.AuthConfig{Type: provider.AuthTypeNone},
					Capabilities: provider.Capabilities{
						Chat:             true,
						MaxContextTokens: 8192,
					},
					Readiness: provider.ReadinessReady,
				},
			},
			found: true,
		}, client)
	}

	tests := []struct {
		name         string
		statusCode   int
		contentType  string
		body         string
		roundTripErr error
		header       http.Header
		want         error
	}{
		{
			name:       "redirect rejected",
			statusCode: http.StatusFound,
			header:     http.Header{"Location": []string{"/elsewhere"}},
			want:       ErrProviderRejected,
		},
		{
			name:       "429 unavailable",
			statusCode: http.StatusTooManyRequests,
			want:       ErrProviderOffline,
		},
		{
			name:        "malformed JSON rejected",
			statusCode:  http.StatusOK,
			contentType: "application/json",
			body:        `{"choices":[`,
			want:        ErrProviderRejected,
		},
		{
			name:        "empty content rejected",
			statusCode:  http.StatusOK,
			contentType: "application/json",
			body:        `{"choices":[{"message":{"content":" \t"}}]}`,
			want:        ErrProviderRejected,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dispatcher := makeDispatcher(tc.statusCode, tc.contentType, tc.body, tc.roundTripErr, tc.header)
			_, err := dispatcher.Generate(context.Background(), GenerateRequest{
				Agent:   linePolish,
				Style:   style,
				Packet:  ContextPacket{SelectedText: "Selected prose", Style: style},
				Summary: ContextSummary{PacksUsed: []ContextPack{ContextSelectedText, ContextStyleSheet}, RAGMode: RAGModeNone},
			})
			if !errors.Is(err, tc.want) {
				t.Fatalf("Generate() error = %v, want %v", err, tc.want)
			}
		})
	}
}

// Test: successful provider envelopes may contain provider-specific metadata,
// but trailing JSON must still be rejected.
// Requirements: M5-R05, M5-R06, M5-R07.
func TestDispatcherIgnoresProviderMetadataButRejectsTrailingJSON(t *testing.T) {
	t.Parallel()

	agentDefinition := Agent{
		Version:           2,
		ID:                "line_polish",
		Name:              "Line Polish",
		Description:       "Rewrite selected prose.",
		AppliesWhen:       ApplicabilityRule{Surfaces: []Surface{SurfaceEditor}, InputScopes: []InputScope{InputScopeSelection}, MinWords: 1, MaxWords: 100},
		ModelRequirements: ModelRequirements{MinContextTokens: 1},
		ContextPolicy:     ContextPolicy{Required: []ContextPack{ContextSelectedText, ContextStyleSheet}},
		RAGPolicy:         RAGPolicy{Mode: RAGModeNone},
		Control:           Control{OutputMode: OutputModePatch, RequiresAcceptance: true},
		Output:            Output{Type: OutputTypeReplacementText, RequiresDiffPreview: true},
	}
	style := Style{Version: 2, ID: "real_style", Name: "Real Style", ProviderProfileID: "real_profile", Model: "model", Temperature: 0.2, SystemPrompt: "Prompt"}
	resolved := resolvedProfileStub{profile: provider.ResolvedProfile{Profile: provider.Profile{
		ID: "real_profile", Type: provider.TypeOpenAICompatible, BaseURL: "https://api.example.test",
		Auth:         provider.AuthConfig{Type: provider.AuthTypeNone},
		Capabilities: provider.Capabilities{Chat: true, MaxContextTokens: 8192},
		Readiness:    provider.ReadinessReady,
	}}, found: true}

	for _, testCase := range []struct {
		name    string
		body    string
		wantErr bool
	}{
		{name: "metadata is ignored", body: `{"id":"chatcmpl-1","choices":[{"index":0,"message":{"role":"assistant","content":"Rewritten"},"finish_reason":"stop"}],"usage":{"total_tokens":12}}`},
		{name: "trailing document is rejected", body: `{"choices":[{"message":{"content":"Rewritten"}}]} {}`, wantErr: true},
	} {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(testCase.body))}, nil
			})}
			_, err := NewDispatcher(resolved, client).Generate(context.Background(), GenerateRequest{
				Agent: agentDefinition, Style: style,
				Packet:  ContextPacket{SelectedText: "Original", Style: style},
				Summary: ContextSummary{PacksUsed: []ContextPack{ContextSelectedText, ContextStyleSheet}, RAGMode: RAGModeNone},
			})
			if testCase.wantErr && !errors.Is(err, ErrProviderRejected) {
				t.Fatalf("Generate() error = %v, want provider rejection", err)
			}
			if !testCase.wantErr && err != nil {
				t.Fatalf("Generate() error = %v", err)
			}
		})
	}
}

// Test: dependency injection cannot disable the redirect-refusal policy or
// forward a bearer credential to a redirected host.
// Requirements: M5-R04, M5-R07.
func TestDispatcherRefusesRedirectsWithInjectedClient(t *testing.T) {
	t.Parallel()

	requestCount := 0
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requestCount++
		if requestCount > 1 {
			if request.Header.Get("Authorization") != "" {
				t.Fatal("authorization header was forwarded to redirect target")
			}
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"unsafe"}}]}`))}, nil
		}
		return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": []string{"https://redirected.example/chat/completions"}}, Body: io.NopCloser(strings.NewReader("redirect body"))}, nil
	})}
	definition := Agent{
		Version: 2, ID: "line_polish", Name: "Line Polish", Description: "Rewrite.",
		AppliesWhen:       ApplicabilityRule{Surfaces: []Surface{SurfaceEditor}, InputScopes: []InputScope{InputScopeSelection}, MinWords: 1, MaxWords: 10},
		ModelRequirements: ModelRequirements{MinContextTokens: 1},
		ContextPolicy:     ContextPolicy{Required: []ContextPack{ContextSelectedText, ContextStyleSheet}},
		RAGPolicy:         RAGPolicy{Mode: RAGModeNone}, Control: Control{OutputMode: OutputModePatch, RequiresAcceptance: true},
		Output: Output{Type: OutputTypeReplacementText, RequiresDiffPreview: true},
	}
	style := Style{Version: 2, ID: "style", Name: "Style", ProviderProfileID: "hosted", Model: "model", Temperature: 0.2, SystemPrompt: "Prompt"}
	resolver := resolvedProfileStub{profile: provider.ResolvedProfile{Profile: provider.Profile{
		ID: "hosted", Type: provider.TypeOpenAICompatible, BaseURL: "https://origin.example/v1",
		Auth:         provider.AuthConfig{Type: provider.AuthTypeBearerEnv, CredentialEnv: "STORYWORK_KEY"},
		Capabilities: provider.Capabilities{Chat: true, MaxContextTokens: 8192}, Readiness: provider.ReadinessReady,
	}, Credential: provider.Credential{Value: "redirect-secret"}}, found: true}

	_, err := NewDispatcher(resolver, client).Generate(context.Background(), GenerateRequest{
		Agent: definition, Style: style, Packet: ContextPacket{SelectedText: "Original", Style: style},
		Summary: ContextSummary{PacksUsed: []ContextPack{ContextSelectedText, ContextStyleSheet}, RAGMode: RAGModeNone},
	})
	if !errors.Is(err, ErrProviderRejected) {
		t.Fatalf("Generate() error = %v, want provider rejection", err)
	}
	if requestCount != 1 {
		t.Fatalf("outbound request count = %d, want 1", requestCount)
	}
}

// Test: real-provider execution must fail closed before outbound I/O when the
// profile is missing or incompatible with the agent's declared requirements.
// Requirements: M5-R08, M5-R09, M5-R10.
func TestDispatcherRejectsMissingProfilesAndIncompatibleRequirements(t *testing.T) {
	t.Parallel()

	baseAgent := Agent{
		Version:           2,
		ID:                "line_polish",
		Name:              "Line Polish",
		Description:       "Rewrite selected prose.",
		AppliesWhen:       ApplicabilityRule{Surfaces: []Surface{SurfaceEditor}, InputScopes: []InputScope{InputScopeSelection}, MinWords: 1, MaxWords: 100},
		ModelRequirements: ModelRequirements{MinContextTokens: 2048},
		ContextPolicy:     ContextPolicy{Required: []ContextPack{ContextSelectedText, ContextStyleSheet}},
		RAGPolicy:         RAGPolicy{Mode: RAGModeNone},
		Control:           Control{OutputMode: OutputModePatch, RequiresAcceptance: true},
		Output:            Output{Type: OutputTypeReplacementText, RequiresDiffPreview: true},
	}
	baseStyle := Style{Version: 2, ID: "real_style", Name: "Real Style", ProviderProfileID: "real_profile", Model: "model", Temperature: 0.2, SystemPrompt: "Prompt"}
	panicClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("unexpected outbound request")
		return nil, nil
	})}

	readyProfile := provider.ResolvedProfile{
		Profile: provider.Profile{
			ID:      "real_profile",
			Type:    provider.TypeOpenAICompatible,
			BaseURL: "https://api.example.test/v1",
			Auth:    provider.AuthConfig{Type: provider.AuthTypeNone},
			Capabilities: provider.Capabilities{
				Chat:             true,
				MaxContextTokens: 8192,
			},
			Readiness: provider.ReadinessReady,
		},
	}

	tests := []struct {
		name     string
		resolver profileResolver
		agent    Agent
		want     error
	}{
		{name: "resolver missing", resolver: nil, agent: baseAgent, want: ErrProviderInvalid},
		{name: "profile not found", resolver: resolvedProfileStub{found: false}, agent: baseAgent, want: ErrProviderInvalid},
		{name: "missing credential", resolver: resolvedProfileStub{profile: func() provider.ResolvedProfile {
			profile := readyProfile
			profile.Auth = provider.AuthConfig{Type: provider.AuthTypeBearerEnv, CredentialEnv: "STORYWORK_KEY"}
			profile.Readiness = provider.ReadinessMissingCredential
			return profile
		}(), found: true}, agent: baseAgent, want: ErrProviderInvalid},
		{name: "chat unsupported", resolver: resolvedProfileStub{profile: func() provider.ResolvedProfile {
			profile := readyProfile
			profile.Capabilities.Chat = false
			return profile
		}(), found: true}, agent: baseAgent, want: ErrProviderInvalid},
		{name: "context too small", resolver: resolvedProfileStub{profile: func() provider.ResolvedProfile {
			profile := readyProfile
			profile.Capabilities.MaxContextTokens = 16
			return profile
		}(), found: true}, agent: baseAgent, want: ErrProviderInvalid},
		{name: "streaming required", resolver: resolvedProfileStub{profile: readyProfile, found: true}, agent: func() Agent {
			agentDefinition := baseAgent
			agentDefinition.ModelRequirements.SupportsStreaming = true
			return agentDefinition
		}(), want: ErrProviderInvalid},
		{name: "structured output required", resolver: resolvedProfileStub{profile: readyProfile, found: true}, agent: func() Agent {
			agentDefinition := baseAgent
			agentDefinition.ModelRequirements.SupportsStructuredOutput = true
			return agentDefinition
		}(), want: ErrProviderInvalid},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewDispatcher(tc.resolver, panicClient).Generate(context.Background(), GenerateRequest{
				Agent:   tc.agent,
				Style:   baseStyle,
				Packet:  ContextPacket{SelectedText: "Original", Style: baseStyle},
				Summary: ContextSummary{PacksUsed: []ContextPack{ContextSelectedText, ContextStyleSheet}, RAGMode: RAGModeNone},
			})
			if !errors.Is(err, tc.want) {
				t.Fatalf("Generate() error = %v, want %v", err, tc.want)
			}
		})
	}
}

// Test: the HTTP adapter must close response bodies on success and failure and
// reject oversize request/response payloads without leaking provider secrets.
// Requirements: M5-R04, M5-R07.
func TestHTTPGeneratorClosesBodiesAndEnforcesSizeLimits(t *testing.T) {
	t.Parallel()

	agentDefinition := Agent{
		Version:           2,
		ID:                "line_polish",
		Name:              "Line Polish",
		Description:       "Rewrite selected prose.",
		AppliesWhen:       ApplicabilityRule{Surfaces: []Surface{SurfaceEditor}, InputScopes: []InputScope{InputScopeSelection}, MinWords: 1, MaxWords: 100},
		ModelRequirements: ModelRequirements{MinContextTokens: 1},
		ContextPolicy:     ContextPolicy{Required: []ContextPack{ContextSelectedText, ContextStyleSheet}},
		RAGPolicy:         RAGPolicy{Mode: RAGModeNone},
		Control:           Control{OutputMode: OutputModePatch, RequiresAcceptance: true},
		Output:            Output{Type: OutputTypeReplacementText, RequiresDiffPreview: true},
	}
	style := Style{Version: 2, ID: "style", Name: "Style", ProviderProfileID: "hosted", Model: "model", Temperature: 0.2, SystemPrompt: "Prompt"}
	resolver := resolvedProfileStub{profile: provider.ResolvedProfile{Profile: provider.Profile{
		ID: "hosted", Type: provider.TypeOpenAICompatible, BaseURL: "https://origin.example/v1",
		Auth:         provider.AuthConfig{Type: provider.AuthTypeBearerEnv, CredentialEnv: "STORYWORK_KEY"},
		Capabilities: provider.Capabilities{Chat: true, MaxContextTokens: 8192}, Readiness: provider.ReadinessReady,
	}, Credential: provider.Credential{Value: "sentinel-secret"}}, found: true}

	t.Run("success and malformed responses close the body", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			body string
			want error
		}{
			{name: "success", body: `{"choices":[{"message":{"content":"Rewritten"}}]}`},
			{name: "malformed", body: `{"choices":[`, want: ErrProviderRejected},
		}
		for _, tc := range tests {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				responseBody := &closingReadCloser{reader: strings.NewReader(tc.body)}
				client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body:       responseBody,
					}, nil
				})}
				_, err := NewDispatcher(resolver, client).Generate(context.Background(), GenerateRequest{
					Agent:   agentDefinition,
					Style:   style,
					Packet:  ContextPacket{SelectedText: "Original", Style: style},
					Summary: ContextSummary{PacksUsed: []ContextPack{ContextSelectedText, ContextStyleSheet}, RAGMode: RAGModeNone},
				})
				if tc.want == nil && err != nil {
					t.Fatalf("Generate() error = %v", err)
				}
				if tc.want != nil && !errors.Is(err, tc.want) {
					t.Fatalf("Generate() error = %v, want %v", err, tc.want)
				}
				if !responseBody.closed {
					t.Fatal("response body was not closed")
				}
			})
		}
	})

	t.Run("oversize request is rejected before network", func(t *testing.T) {
		t.Parallel()

		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Fatal("unexpected outbound request")
			return nil, nil
		})}
		hugeStyle := style
		hugeStyle.SystemPrompt = strings.Repeat("x", 7<<20)
		_, err := NewDispatcher(resolver, client).Generate(context.Background(), GenerateRequest{
			Agent:   agentDefinition,
			Style:   hugeStyle,
			Packet:  ContextPacket{SelectedText: "Original", Style: hugeStyle},
			Summary: ContextSummary{PacksUsed: []ContextPack{ContextSelectedText, ContextStyleSheet}, RAGMode: RAGModeNone},
		})
		if !errors.Is(err, ErrProviderRejected) {
			t.Fatalf("Generate() error = %v, want ErrProviderRejected", err)
		}
	})

	t.Run("oversize response and redirect errors stay secret-safe", func(t *testing.T) {
		t.Parallel()

		oversizedJSON := `{"choices":[{"message":{"content":"` + strings.Repeat("a", 2<<20) + `"}}]}`
		for _, testCase := range []struct {
			name       string
			statusCode int
			header     http.Header
			body       string
		}{
			{name: "oversize response", statusCode: http.StatusOK, header: http.Header{"Content-Type": []string{"application/json"}}, body: oversizedJSON},
			{name: "redirect", statusCode: http.StatusFound, header: http.Header{"Location": []string{"https://redirected.example/sentinel-secret"}}, body: "sentinel-secret"},
		} {
			testCase := testCase
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()
				client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: testCase.statusCode,
						Header:     testCase.header,
						Body:       io.NopCloser(strings.NewReader(testCase.body)),
					}, nil
				})}
				_, err := NewDispatcher(resolver, client).Generate(context.Background(), GenerateRequest{
					Agent:   agentDefinition,
					Style:   style,
					Packet:  ContextPacket{SelectedText: "Original", Style: style},
					Summary: ContextSummary{PacksUsed: []ContextPack{ContextSelectedText, ContextStyleSheet}, RAGMode: RAGModeNone},
				})
				if !errors.Is(err, ErrProviderRejected) {
					t.Fatalf("Generate() error = %v, want ErrProviderRejected", err)
				}
				if strings.Contains(err.Error(), "sentinel-secret") {
					t.Fatalf("Generate() error leaked sentinel: %v", err)
				}
			})
		}
	})
}
