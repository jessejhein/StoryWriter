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
