package agent

// BDD Scenario: characterization guard for provider message scope
// Requirements: M7-R02, M7-R19
// Test purpose: Lock selection-scoped provider messages and adapter dispatch
// before Milestone 7 introduces scope-specific message builders.

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"storywork/internal/provider"
)

// Test: HTTP provider messages remain selection-scoped with no wider story context.
// Requirements: M7-R02.
func TestM7ExistingProviderMessagesRemainSelectionScoped(t *testing.T) {
	t.Parallel()

	style := Style{
		Version:           2,
		ID:                "precise_editor",
		Name:              "Precise Editor",
		ProviderProfileID: "local_openai",
		Model:             "local-model",
		Temperature:       0.2,
		SystemPrompt:      "Careful editor system prompt",
	}
	agentDefinition := Agent{
		Version:     2,
		ID:          "line_polish",
		Name:        "Line Polish",
		Description: "Rewrite selected prose.",
		AppliesWhen: ApplicabilityRule{Surfaces: []Surface{SurfaceEditor}, InputScopes: []InputScope{InputScopeSelection}, MinWords: 1, MaxWords: 1500},
		ModelRequirements: ModelRequirements{MinContextTokens: 2048},
		ContextPolicy: ContextPolicy{
			Required:  []ContextPack{ContextSelectedText, ContextStyleSheet},
			Forbidden: []ContextPack{ContextGlobalCodexRAG, ContextRawImportNotes},
		},
		RAGPolicy: RAGPolicy{Mode: RAGModeNone},
		Control:   Control{OutputMode: OutputModePatch, RequiresAcceptance: true},
		Output:    Output{Type: OutputTypeReplacementText, RequiresDiffPreview: true},
	}
	request := GenerateRequest{
		Agent: agentDefinition,
		Style: style,
		Packet: ContextPacket{
			SelectedText: "Only this paragraph.",
			Style:        style,
		},
		Summary: ContextSummary{
			PacksUsed: []ContextPack{ContextSelectedText, ContextStyleSheet},
			RAGMode:   RAGModeNone,
		},
	}

	for _, generator := range []*HTTPGenerator{
		newOpenAICompatibleGenerator(nil),
		newOllamaGenerator(nil),
	} {
		messages, temperature := generator.messages(request)
		if temperature == nil || *temperature != 0.2 {
			t.Fatalf("temperature = %#v, want 0.2 pointer", temperature)
		}
		if len(messages) != 2 || messages[0].Role != "system" || messages[1].Role != "user" {
			t.Fatalf("messages = %#v, want system and user roles only", messages)
		}
		if messages[0].Content != "Careful editor system prompt" {
			t.Fatalf("system prompt = %q", messages[0].Content)
		}
		user := messages[1].Content
		if !strings.Contains(user, "Rewrite only the selected text") {
			t.Fatalf("user prompt missing selection instruction: %q", user)
		}
		if !strings.Contains(user, "Only this paragraph.") {
			t.Fatalf("user prompt missing selected text: %q", user)
		}
		for _, forbidden := range []string{
			"current_scene",
			"active_codex",
			"outline_neighborhood",
			"chapter_summary",
			"global_codex",
			"raw_import",
		} {
			if strings.Contains(strings.ToLower(user), forbidden) {
				t.Fatalf("user prompt included wider-context marker %q: %s", forbidden, user)
			}
		}
	}
}

// Test: mock, OpenAI-compatible, and Ollama dispatch remain compatible with selection runs.
// Requirements: M7-R19.
func TestM7MockOpenAIAndOllamaDispatchRemainCompatible(t *testing.T) {
	t.Parallel()

	linePolish := Agent{
		Version:     2,
		ID:          "line_polish",
		Name:        "Line Polish",
		Description: "Rewrite selected prose.",
		AppliesWhen: ApplicabilityRule{Surfaces: []Surface{SurfaceEditor}, InputScopes: []InputScope{InputScopeSelection}, MinWords: 1, MaxWords: 1500},
		ModelRequirements: ModelRequirements{MinContextTokens: 2048},
		ContextPolicy: ContextPolicy{
			Required:  []ContextPack{ContextSelectedText, ContextStyleSheet},
			Forbidden: []ContextPack{ContextGlobalCodexRAG, ContextRawImportNotes},
		},
		RAGPolicy: RAGPolicy{Mode: RAGModeNone},
		Control:   Control{OutputMode: OutputModePatch, RequiresAcceptance: true},
		Output:    Output{Type: OutputTypeReplacementText, RequiresDiffPreview: true},
	}
	mockStyle := Style{
		Version: 1, ID: "precise_editor", Name: "Precise Editor",
		ProviderProfileID: "mock_default", Model: "mock", Temperature: 0.2, SystemPrompt: "Prompt",
	}
	packet := ContextPacket{SelectedText: "Selected prose", Style: mockStyle}
	summary := ContextSummary{PacksUsed: []ContextPack{ContextSelectedText, ContextStyleSheet}, RAGMode: RAGModeNone}

	mockResponse, err := NewMockProvider().Generate(context.Background(), GenerateRequest{
		Agent: linePolish, Style: mockStyle, Packet: packet, Summary: summary,
	})
	if err != nil {
		t.Fatalf("Mock Generate() error = %v", err)
	}
	if mockResponse.Replacement != "Mock polished: Selected prose" {
		t.Fatalf("mock replacement = %q", mockResponse.Replacement)
	}

	openAIStyle := Style{
		Version: 2, ID: "openai_style", Name: "OpenAI", ProviderProfileID: "openai",
		Model: "gpt-test", Temperature: 0.2, SystemPrompt: "Prompt",
	}
	ollamaStyle := Style{
		Version: 2, ID: "ollama_style", Name: "Ollama", ProviderProfileID: "ollama",
		Model: "ollama-test", Temperature: 0.4, SystemPrompt: "Prompt",
	}

	openAIClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(request.Body)
		if !strings.Contains(string(body), "Rewrite only the selected text") {
			t.Fatalf("OpenAI body missing selection instruction: %s", string(body))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"Rewritten"}}]}`)),
		}, nil
	})}
	openAIDispatcher := NewDispatcher(resolvedProfileStub{
		profile: provider.ResolvedProfile{
			Profile: provider.Profile{
				ID: "openai", Type: provider.TypeOpenAICompatible, BaseURL: "https://api.example.test/v1",
				Auth: provider.AuthConfig{Type: provider.AuthTypeBearerEnv, CredentialEnv: "KEY"},
				Capabilities: provider.Capabilities{Chat: true, MaxContextTokens: 8192},
				Readiness: provider.ReadinessReady,
			},
			Credential: provider.Credential{Value: "test-key"},
		},
		found: true,
	}, openAIClient)
	if _, err := openAIDispatcher.Generate(context.Background(), GenerateRequest{
		Agent: linePolish, Style: openAIStyle, Packet: packet, Summary: summary,
	}); err != nil {
		t.Fatalf("OpenAI Generate() error = %v", err)
	}

	ollamaClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(request.Body)
		if !strings.Contains(string(body), "Rewrite only the selected text") {
			t.Fatalf("Ollama body missing selection instruction: %s", string(body))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"message":{"content":"Ollama rewrite"}}`)),
		}, nil
	})}
	ollamaDispatcher := NewDispatcher(resolvedProfileStub{
		profile: provider.ResolvedProfile{
			Profile: provider.Profile{
				ID: "ollama", Type: provider.TypeOllama, BaseURL: "http://127.0.0.1:11434",
				Auth: provider.AuthConfig{Type: provider.AuthTypeNone},
				Capabilities: provider.Capabilities{Chat: true, MaxContextTokens: 8192},
				Readiness: provider.ReadinessReady,
			},
		},
		found: true,
	}, ollamaClient)
	if _, err := ollamaDispatcher.Generate(context.Background(), GenerateRequest{
		Agent: linePolish, Style: ollamaStyle, Packet: packet, Summary: summary,
	}); err != nil {
		t.Fatalf("Ollama Generate() error = %v", err)
	}
}