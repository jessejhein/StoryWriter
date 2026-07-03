package agent

// BDD Scenario: 8.3.1 - Run only after explicit authorization
// Requirements: M8-R11
// Test purpose: Prove agent dispatch still consumes the neutral modelchat completer contract.

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"storywork/internal/modelchat"
	"storywork/internal/provider"
)

// Test: agent CompleteChat remains a thin compatibility wrapper over modelchat.
// Requirements: M8-R11.
func TestM8AgentCompleteChatDelegatesToModelchat(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"delegated"}}]}`)),
		}, nil
	})}
	response, err := CompleteChat(context.Background(), client, ChatRequest{
		Profile: provider.ResolvedProfile{
			Profile: provider.Profile{ID: "hosted", Type: provider.TypeOpenAICompatible, BaseURL: "https://api.example.test/v1"},
		},
		Model:    "model",
		Messages: []ChatMessage{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("CompleteChat() error = %v", err)
	}
	if response.Content != "delegated" {
		t.Fatalf("content = %q", response.Content)
	}
}

// Test: HTTP generator dispatch maps modelchat provider identity without transport duplication.
// Requirements: M8-R11.
func TestM8AgentDispatchConsumesModelchatIdentity(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"Rewritten"}}]}`)),
		}, nil
	})}
	dispatcher := NewDispatcher(resolvedProfileStub{
		profile: provider.ResolvedProfile{
			Profile: provider.Profile{
				ID: "hosted", Type: provider.TypeOpenAICompatible, BaseURL: "https://api.example.test/v1",
				Capabilities: provider.Capabilities{Chat: true, MaxContextTokens: 8192},
				Readiness:    provider.ReadinessReady,
			},
		},
		found: true,
	}, client)
	response, err := dispatcher.Generate(context.Background(), GenerateRequest{
		Agent: Agent{
			Version: 2, ID: "line_polish", Name: "Line Polish", Description: "Rewrite.",
			AppliesWhen:       ApplicabilityRule{Surfaces: []Surface{SurfaceEditor}, InputScopes: []InputScope{InputScopeSelection}, MinWords: 1, MaxWords: 100},
			ModelRequirements: ModelRequirements{MinContextTokens: 1},
			ContextPolicy:     ContextPolicy{Required: []ContextPack{ContextSelectedText, ContextStyleSheet}},
			RAGPolicy:         RAGPolicy{Mode: RAGModeNone},
			Control:           Control{OutputMode: OutputModePatch, RequiresAcceptance: true},
			Output:            Output{Type: OutputTypeReplacementText, RequiresDiffPreview: true},
		},
		Style:   Style{Version: 2, ID: "style", Name: "Style", ProviderProfileID: "hosted", Model: "model", Temperature: 0.2, SystemPrompt: "Prompt"},
		Packet:  ContextPacket{SelectedText: "Original", Style: Style{ProviderProfileID: "hosted", Model: "model"}},
		Summary: ContextSummary{PacksUsed: []ContextPack{ContextSelectedText, ContextStyleSheet}, RAGMode: RAGModeNone},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if response.Provider.ProfileID != "hosted" || response.Provider.Type != provider.TypeOpenAICompatible || response.Provider.Model != "model" {
		t.Fatalf("provider identity = %#v", response.Provider)
	}
	var _ modelchat.ProviderIdentity = response.Provider
}
