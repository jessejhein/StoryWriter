package agent

import (
	"context"
	"errors"
	"testing"
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
