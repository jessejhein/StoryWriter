// BDD Scenario: 7.2.3 - Review and accept one scene replacement
// Requirements: M7-R01, M7-R03, M7-R04
// Test purpose: Version-3 agent definitions accept only supported scopes, outputs, budgets, and follow-up transitions.

package agent

import (
	"errors"
	"testing"
)

func validSceneRewriteV3() Agent {
	return Agent{
		Version:     3,
		ID:          "scene_rewrite",
		Name:        "Scene Rewrite",
		Description: "Rewrite one scene while preserving established facts and intent.",
		AppliesWhen: ApplicabilityRule{
			Surfaces:    []Surface{SurfaceEditor},
			InputScopes: []InputScope{InputScopeScene},
			MinWords:    1,
			MaxWords:    12000,
		},
		ModelRequirements: ModelRequirements{
			MinContextTokens:         4096,
			SupportsStreaming:        false,
			SupportsStructuredOutput: false,
		},
		ContextPolicy: ContextPolicy{
			Required:  []ContextPack{ContextCurrentScene, ContextStyleSheet, ContextActiveCodex},
			Optional:  []ContextPack{ContextOutlineNeighbor},
			Forbidden: []ContextPack{ContextGlobalCodexRAG, ContextRawImportNotes, ContextPriorChat},
		},
		ContextBudget: ContextBudget{
			MaxInputEstimatedTokens:       12000,
			ReservedOutputEstimatedTokens: 4000,
		},
		RAGPolicy: RAGPolicy{Mode: RAGModeTimelineAware},
		FollowUps: FollowUpPolicy{
			OnAccept: []FollowUpRule{
				{AgentID: "chapter_review", Scope: FollowUpScopeChapterReview, Relationship: FollowUpRelationshipTriggered},
			},
		},
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
}

func validChapterReviewV3() Agent {
	return Agent{
		Version:     3,
		ID:          "chapter_review",
		Name:        "Chapter Review",
		Description: "Return editorial findings for one chapter without rewriting canon.",
		AppliesWhen: ApplicabilityRule{
			Surfaces:    []Surface{SurfaceChapterView},
			InputScopes: []InputScope{InputScopeChapterReview},
			MinWords:    1,
			MaxWords:    12000,
		},
		ModelRequirements: ModelRequirements{
			MinContextTokens:         4096,
			SupportsStreaming:        false,
			SupportsStructuredOutput: true,
		},
		ContextPolicy: ContextPolicy{
			Required:  []ContextPack{ContextCurrentChapter, ContextStyleSheet, ContextActiveCodex},
			Optional:  []ContextPack{ContextOutlineNeighbor},
			Forbidden: []ContextPack{ContextGlobalCodexRAG, ContextRawImportNotes, ContextPriorChat},
		},
		ContextBudget: ContextBudget{
			MaxInputEstimatedTokens:       16000,
			ReservedOutputEstimatedTokens: 2000,
		},
		RAGPolicy: RAGPolicy{Mode: RAGModeTimelineAware},
		Control: Control{
			OutputMode:         OutputModeSuggestion,
			RequiresAcceptance: false,
			CanModifyCanon:     false,
		},
		Output: Output{
			Type:                OutputTypeEditorialFindings,
			RequiresDiffPreview: false,
		},
	}
}

func validLinePolishV3() Agent {
	return Agent{
		Version:     3,
		ID:          "line_polish",
		Name:        "Line Polish",
		Description: "Rewrite selected prose for clarity, cadence, and flow while preserving meaning.",
		AppliesWhen: ApplicabilityRule{
			Surfaces:    []Surface{SurfaceEditor},
			InputScopes: []InputScope{InputScopeSelection},
			MinWords:    20,
			MaxWords:    1500,
		},
		ModelRequirements: ModelRequirements{
			MinContextTokens:         2048,
			SupportsStreaming:        false,
			SupportsStructuredOutput: false,
		},
		ContextPolicy: ContextPolicy{
			Required:  []ContextPack{ContextSelectedText, ContextStyleSheet},
			Optional:  []ContextPack{ContextSurrounding},
			Forbidden: []ContextPack{ContextGlobalCodexRAG, ContextRawImportNotes, ContextPriorChat},
		},
		ContextBudget: ContextBudget{
			MaxInputEstimatedTokens:       4096,
			ReservedOutputEstimatedTokens: 1024,
		},
		RAGPolicy: RAGPolicy{Mode: RAGModeNone},
		FollowUps: FollowUpPolicy{
			OnAccept: []FollowUpRule{
				{AgentID: "scene_rewrite", Scope: FollowUpScopeScene, Relationship: FollowUpRelationshipTriggered},
			},
		},
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
}

// Test: version 3 accepts supported scopes and output combinations.
// Requirements: M7-R01, M7-R03, M7-R04.
func TestValidateAgentV3AcceptsSupportedScopesAndOutputs(t *testing.T) {
	t.Parallel()

	for _, agent := range []Agent{validLinePolishV3(), validSceneRewriteV3(), validChapterReviewV3()} {
		if _, err := ValidateAgentV3(agent); err != nil {
			t.Fatalf("ValidateAgentV3(%q) error = %v", agent.ID, err)
		}
	}
}

// Test: version 3 accepts none and timeline-aware RAG modes only.
// Requirements: M7-R05, M7-R06.
func TestValidateAgentV3AcceptsNoneAndTimelineAwareRAG(t *testing.T) {
	t.Parallel()

	agent := validSceneRewriteV3()
	if _, err := ValidateAgentV3(agent); err != nil {
		t.Fatalf("timeline-aware agent error = %v", err)
	}
	agent.RAGPolicy.Mode = RAGModeNone
	agent.ContextPolicy.Required = []ContextPack{ContextCurrentScene, ContextStyleSheet}
	agent.ContextPolicy.Optional = nil
	if _, err := ValidateAgentV3(agent); err != nil {
		t.Fatalf("none RAG agent error = %v", err)
	}
}

// Test: version 3 validates context budgets against provider requirements.
// Requirements: M7-R08.
func TestValidateAgentV3ValidatesContextBudget(t *testing.T) {
	t.Parallel()

	agent := validSceneRewriteV3()
	agent.ContextBudget.MaxInputEstimatedTokens = 1000
	if _, err := ValidateAgentV3(agent); err == nil {
		t.Fatal("ValidateAgentV3() error = nil, want budget failure")
	}
	agent = validSceneRewriteV3()
	agent.ContextBudget.ReservedOutputEstimatedTokens = agent.ContextBudget.MaxInputEstimatedTokens
	if _, err := ValidateAgentV3(agent); err == nil {
		t.Fatal("ValidateAgentV3() error = nil, want reserved-output failure")
	}
	agent = validChapterReviewV3()
	agent.ModelRequirements.MinContextTokens = agent.ContextBudget.MaxInputEstimatedTokens + 1
	if _, err := ValidateAgentV3(agent); err == nil {
		t.Fatal("ValidateAgentV3() error = nil, want provider budget failure")
	}
}

// Test: version 3 validates configured follow-up transitions.
// Requirements: M7-R11, M7-R12.
func TestValidateAgentV3ValidatesFollowUpTransitions(t *testing.T) {
	t.Parallel()

	agent := validLinePolishV3()
	agent.FollowUps.OnAccept[0].Scope = FollowUpScopeChapterReview
	if _, err := ValidateAgentV3(agent); err == nil {
		t.Fatal("ValidateAgentV3() error = nil, want broader-scope transition failure")
	}
	agent = validSceneRewriteV3()
	agent.FollowUps.OnAccept[0].Relationship = FollowUpRelationshipDependsOn
	if _, err := ValidateAgentV3(agent); err == nil {
		t.Fatal("ValidateAgentV3() error = nil, want unsupported relationship failure")
	}
}

// Test: version 3 rejects cycles, duplicates, and unknown values.
// Requirements: M7-R07, M7-R11.
func TestValidateAgentV3RejectsCyclesDuplicatesAndUnknownValues(t *testing.T) {
	t.Parallel()

	agent := validLinePolishV3()
	agent.FollowUps.OnAccept = append(agent.FollowUps.OnAccept, agent.FollowUps.OnAccept[0])
	if _, err := ValidateAgentV3(agent); err == nil {
		t.Fatal("ValidateAgentV3() error = nil, want duplicate follow-up failure")
	}
	agent = validSceneRewriteV3()
	agent.FollowUps.OnAccept[0].AgentID = "line_polish"
	if _, err := ValidateAgentV3(agent); err == nil {
		t.Fatal("ValidateAgentV3() error = nil, want invalid transition failure")
	}
	agent = validSceneRewriteV3()
	agent.ContextPolicy.Required = append(agent.ContextPolicy.Required, ContextPack("unknown_pack"))
	if _, err := ValidateAgentV3(agent); !errors.Is(err, ErrInvalidAgent) {
		t.Fatalf("ValidateAgentV3() error = %v, want %v", err, ErrInvalidAgent)
	}
}

// Test: version 1 and version 2 validation remain unchanged.
// Requirements: M7-R19.
func TestValidateAgentV1AndV2RemainUnchanged(t *testing.T) {
	t.Parallel()

	v1 := Agent{
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
	if _, err := ValidateAgent(v1); err != nil {
		t.Fatalf("ValidateAgent(v1) error = %v", err)
	}
	if _, err := ValidateAgentV3(v1); err == nil {
		t.Fatal("ValidateAgentV3(v1) error = nil, want version failure")
	}

	v2 := v1
	v2.Version = 2
	v2.ModelRequirements = ModelRequirements{MinContextTokens: 2048}
	if _, err := ValidateAgent(v2); err != nil {
		t.Fatalf("ValidateAgent(v2) error = %v", err)
	}
	v2.ContextBudget = ContextBudget{MaxInputEstimatedTokens: 4096, ReservedOutputEstimatedTokens: 1024}
	if _, err := ValidateAgent(v2); err == nil {
		t.Fatal("ValidateAgent(v2 with v3 fields) error = nil, want unsupported version")
	}
}
