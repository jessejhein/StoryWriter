package agent

import (
	"errors"
	"testing"
)

// BDD trace:
//   - Requirements: M4-R01, M4-R02, M4-R03, M4-R04, M4-R05, M4-R06.
//   - Scenario: 4.2.1, 4.2.2, 4.3.1.
//   - Test purpose: verify strict registry validation, deterministic applicability,
//     Unicode word counts, and minimal context assembly before adapters exist.
func TestRegistryValidationApplicabilityAndContext(t *testing.T) {
	t.Parallel()

	linePolish := Agent{
		Version:     1,
		ID:          "line_polish",
		Name:        "Line Polish",
		Description: "Rewrite selected prose.",
		AppliesWhen: ApplicabilityRule{
			Surfaces:    []Surface{SurfaceEditor},
			InputScopes: []InputScope{InputScopeSelection},
			MinWords:    20,
			MaxWords:    1500,
		},
		ContextPolicy: ContextPolicy{
			Required:  []ContextPack{ContextSelectedText, ContextStyleSheet},
			Optional:  []ContextPack{ContextSurrounding},
			Forbidden: []ContextPack{ContextGlobalCodexRAG, ContextRawImportNotes},
		},
		RAGPolicy: RAGPolicy{Mode: RAGModeNone},
		Control: Control{
			OutputMode:         OutputModePatch,
			RequiresAcceptance: true,
			CanModifyCanon:     false,
		},
		Output: Output{
			Type:                OutputTypeReplacementText,
			RequiresDiffPreview: true,
		},
	}
	chapterRefiner := Agent{
		Version:     1,
		ID:          "chapter_refiner",
		Name:        "Chapter Refiner",
		Description: "Refine a chapter.",
		AppliesWhen: ApplicabilityRule{
			Surfaces:    []Surface{SurfaceChapterView},
			InputScopes: []InputScope{InputScopeChapter},
			MinWords:    1000,
			MaxWords:    12000,
		},
		ContextPolicy: ContextPolicy{
			Required:  []ContextPack{ContextCurrentChapter, ContextChapterSummary, ContextStyleSheet},
			Optional:  []ContextPack{ContextArcSummary},
			Forbidden: []ContextPack{ContextRawImportNotes},
		},
		RAGPolicy: RAGPolicy{Mode: RAGModeNone},
		Control: Control{
			OutputMode:         OutputModePatch,
			RequiresAcceptance: true,
			CanModifyCanon:     false,
		},
		Output: Output{
			Type:                OutputTypeRevisedText,
			RequiresDiffPreview: true,
		},
	}
	style := Style{
		Version:           1,
		ID:                "precise_editor",
		Name:              "Precise Editor",
		ProviderProfileID: "mock_default",
		Model:             "mock",
		Temperature:       0.2,
		SystemPrompt:      "You are a careful prose editor.",
	}
	if _, err := ValidateAgent(linePolish); err != nil {
		t.Fatalf("ValidateAgent(linePolish) error = %v", err)
	}
	if _, err := ValidateAgent(chapterRefiner); err != nil {
		t.Fatalf("ValidateAgent(chapterRefiner) error = %v", err)
	}
	if _, err := ValidateStyle(style); err != nil {
		t.Fatalf("ValidateStyle() error = %v", err)
	}

	decisions := ApplicableAgents([]Agent{chapterRefiner, linePolish}, AvailabilityInput{
		Surface:        SurfaceEditor,
		InputScope:     InputScopeSelection,
		SceneID:        "scn_0123456789abcdef0123",
		SelectionWords: 200,
	})
	if !decisions[1].Applicable || decisions[1].Agent.ID != "line_polish" {
		t.Fatalf("line polish decision = %#v", decisions)
	}
	if decisions[0].Applicable || decisions[0].ExcludedReason == "" {
		t.Fatalf("chapter refiner decision = %#v", decisions[0])
	}

	shortSelection := ApplicableAgents([]Agent{linePolish}, AvailabilityInput{
		Surface:        SurfaceEditor,
		InputScope:     InputScopeSelection,
		SceneID:        "scn_0123456789abcdef0123",
		SelectionWords: 19,
	})
	if shortSelection[0].Applicable || shortSelection[0].ExcludedReason != "selection is too short" {
		t.Fatalf("short selection decision = %#v", shortSelection[0])
	}

	if got := WordCount("  one\tTwo\nthree  "); got != 3 {
		t.Fatalf("WordCount() = %d, want 3", got)
	}
	if got := WordCount("Luz ágil\nvuela"); got != 3 {
		t.Fatalf("WordCount(multibyte) = %d, want 3", got)
	}

	packet, summary, err := BuildContext(BuildContextRequest{
		Agent:        linePolish,
		Style:        style,
		SelectedText: "Selected prose",
	})
	if err != nil {
		t.Fatalf("BuildContext() error = %v", err)
	}
	if packet.SelectedText != "Selected prose" || packet.Style.ID != style.ID {
		t.Fatalf("context packet = %#v", packet)
	}
	if len(summary.PacksUsed) != 2 || summary.PacksUsed[0] != ContextSelectedText || summary.PacksUsed[1] != ContextStyleSheet {
		t.Fatalf("context summary = %#v", summary)
	}
	if _, _, err := BuildContext(BuildContextRequest{
		Agent:                  linePolish,
		Style:                  style,
		SelectedText:           "Selected prose",
		RequestedOptionalPacks: []ContextPack{ContextGlobalCodexRAG},
	}); err == nil {
		t.Fatal("BuildContext(forbidden optional) error = nil, want failure")
	}

	invalid := linePolish
	invalid.ContextPolicy.Optional = []ContextPack{ContextStyleSheet}
	if _, err := ValidateAgent(invalid); err == nil || !errors.Is(err, ErrInvalidAgent) {
		t.Fatalf("ValidateAgent(disjoint context) error = %v, want ErrInvalidAgent", err)
	}
}
