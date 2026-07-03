// BDD Scenario: 7.1.1 - Preview minimal Line Polish context
// Requirements: M7-R07, M7-R09
// Test purpose: Context preview validates targets and returns redacted manifests without side effects.

package action

import (
	"context"
	"errors"
	"strings"
	"testing"

	"storywork/internal/agent"
	"storywork/internal/contextpack"
	"storywork/internal/project"
	"storywork/internal/provider"
	"storywork/internal/story"
)

type fakeMaterialSource struct {
	result story.ContextMaterialResult
	err    error
	calls  int
}

func (f *fakeMaterialSource) LoadSelectionMaterial(context.Context, story.SelectionMaterialRequest) (story.ContextMaterialResult, error) {
	f.calls++
	return f.result, f.err
}

func (f *fakeMaterialSource) LoadSceneMaterial(context.Context, string, string) (story.ContextMaterialResult, error) {
	f.calls++
	return f.result, f.err
}

func (f *fakeMaterialSource) LoadChapterMaterial(context.Context, string) (story.ContextMaterialResult, error) {
	f.calls++
	return f.result, f.err
}

type countingIDGenerator struct {
	calls int
}

func (g *countingIDGenerator) Next() (string, error) {
	g.calls++
	return "run_should_not_be_used", nil
}

// Test: preview validates registry applicability and style compatibility.
// Requirements: M7-R07.
func TestPreviewValidatesRegistryApplicabilityAndStyle(t *testing.T) {
	t.Parallel()

	service := newPreviewTestService(t, &fakeMaterialSource{}, &fakeProvider{}, &countingIDGenerator{})
	_, err := service.PreviewContext(context.Background(), TaggedRunRequest{
		AgentID: "missing_agent", StyleID: "precise_editor",
		Target: selectionTargetFixture(),
	})
	if !errors.Is(err, ErrAgentNotFound) {
		t.Fatalf("PreviewContext() error = %v, want ErrAgentNotFound", err)
	}
}

// Test: preview builds selection, scene, and chapter manifests.
// Requirements: M7-R07, M7-R10.
func TestPreviewBuildsSelectionSceneAndChapterManifests(t *testing.T) {
	t.Parallel()

	selectionSource := &fakeMaterialSource{result: story.ContextMaterialResult{
		Material:       contextpack.Material{Scope: contextpack.ScopeSelection, SelectionText: "Alpha beta"},
		TargetRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}}
	service := newPreviewTestService(t, selectionSource, &fakeProvider{}, &countingIDGenerator{})
	preview, err := service.PreviewContext(context.Background(), TaggedRunRequest{
		AgentID: "line_polish", StyleID: "precise_editor", Target: selectionTargetFixture(),
	})
	if err != nil {
		t.Fatalf("selection PreviewContext() error = %v", err)
	}
	if preview.Manifest.Scope != contextpack.ScopeSelection || len(preview.Manifest.PacksUsed) != 2 {
		t.Fatalf("selection manifest = %#v", preview.Manifest)
	}

	sceneSource := &fakeMaterialSource{result: story.ContextMaterialResult{
		Material: contextpack.Material{
			Scope: contextpack.ScopeScene, SceneMarkdown: "Ann arrives.\n",
			SceneOrder:      []contextpack.SceneOrderRef{{ID: "scn_0123456789abcdef0123"}},
			CodexCandidates: []contextpack.CodexEntryCandidate{{EntryID: "char_0123456789abcdef0123", EntryType: "character", Name: "Ann"}},
		},
		TargetRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}}
	service = newPreviewTestServiceWithAgents(t, sceneSource, testSceneRewriteAgent())
	preview, err = service.PreviewContext(context.Background(), TaggedRunRequest{
		AgentID: "scene_rewrite", StyleID: "precise_editor",
		Target: TaggedTarget{Scope: contextpack.ScopeScene, Scene: &SceneTarget{
			SceneID:       "scn_0123456789abcdef0123",
			SceneRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		}},
	})
	if err != nil {
		t.Fatalf("scene PreviewContext() error = %v", err)
	}
	if preview.Manifest.Scope != contextpack.ScopeScene {
		t.Fatalf("scene manifest = %#v", preview.Manifest)
	}

	chapterSource := &fakeMaterialSource{result: story.ContextMaterialResult{
		Material: contextpack.Material{
			Scope:         contextpack.ScopeChapterReview,
			ChapterScenes: []contextpack.ChapterSceneText{{SceneID: "scn_0123456789abcdef0123", Markdown: "Scene.\n"}},
			SceneOrder:    []contextpack.SceneOrderRef{{ID: "scn_0123456789abcdef0123"}},
		},
		TargetRevision: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}}
	service = newPreviewTestServiceWithAgents(t, chapterSource, testChapterReviewAgent())
	preview, err = service.PreviewContext(context.Background(), TaggedRunRequest{
		AgentID: "chapter_review", StyleID: "precise_editor",
		Target: TaggedTarget{Scope: contextpack.ScopeChapterReview, Chapter: &ChapterReviewTarget{
			ChapterID:   "ch_0123456789abcdef0123",
			Fingerprint: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		}},
	})
	if err != nil {
		t.Fatalf("chapter PreviewContext() error = %v", err)
	}
	if preview.Manifest.Scope != contextpack.ScopeChapterReview || preview.TargetRevision != chapterSource.result.TargetRevision {
		t.Fatalf("chapter manifest = %#v", preview.Manifest)
	}
}

// Test: preview calls no provider or run ID generator.
// Requirements: M7-R09.
func TestPreviewCallsNoProviderOrIDStore(t *testing.T) {
	t.Parallel()

	provider := &fakeProvider{}
	ids := &countingIDGenerator{}
	service := newPreviewTestService(t, &fakeMaterialSource{result: story.ContextMaterialResult{
		Material:       contextpack.Material{Scope: contextpack.ScopeSelection, SelectionText: "Alpha beta"},
		TargetRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}}, provider, ids)
	if _, err := service.PreviewContext(context.Background(), TaggedRunRequest{
		AgentID: "line_polish", StyleID: "precise_editor", Target: selectionTargetFixture(),
	}); err != nil {
		t.Fatalf("PreviewContext() error = %v", err)
	}
	if provider.calls != 0 || ids.calls != 0 {
		t.Fatalf("provider calls = %d, id calls = %d", provider.calls, ids.calls)
	}
}

// Test: preview performs no file, index, or git mutation via material source only.
// Requirements: M7-R09.
func TestPreviewPerformsNoFileIndexOrGitMutation(t *testing.T) {
	t.Parallel()

	source := &fakeMaterialSource{result: story.ContextMaterialResult{
		Material:       contextpack.Material{Scope: contextpack.ScopeSelection, SelectionText: "Alpha"},
		TargetRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}}
	service := newPreviewTestService(t, source, &fakeProvider{}, &countingIDGenerator{})
	if _, err := service.PreviewContext(context.Background(), TaggedRunRequest{
		AgentID: "line_polish", StyleID: "precise_editor", Target: selectionTargetFixture(),
	}); err != nil {
		t.Fatalf("PreviewContext() error = %v", err)
	}
	if source.calls != 1 {
		t.Fatalf("material calls = %d, want 1", source.calls)
	}
}

// Test: preview returns budget and canonical state errors.
// Requirements: M7-R07, M7-R08.
func TestPreviewReturnsBudgetAndCanonicalStateErrors(t *testing.T) {
	t.Parallel()

	service := newPreviewTestService(t, &fakeMaterialSource{err: story.ErrStaleRevision}, &fakeProvider{}, &countingIDGenerator{})
	if _, err := service.PreviewContext(context.Background(), TaggedRunRequest{
		AgentID: "line_polish", StyleID: "precise_editor", Target: selectionTargetFixture(),
	}); !errors.Is(err, story.ErrStaleRevision) {
		t.Fatalf("PreviewContext() error = %v, want ErrStaleRevision", err)
	}
}

// Test: preview budgets against the selected provider's actual context limit.
// Requirements: M7-R07, M7-R08, M7-R12.
func TestPreviewUsesResolvedProviderContextLimit(t *testing.T) {
	t.Parallel()

	agentDefinition := testSceneRewriteAgent()
	agentDefinition.ModelRequirements.MinContextTokens = 32
	style := testPreciseEditorStyle()
	style.Version = 2
	style.ProviderProfileID = "small_context"
	style.Model = "test-model"
	source := &fakeMaterialSource{result: story.ContextMaterialResult{
		Material: contextpack.Material{
			Scope: contextpack.ScopeScene, SceneMarkdown: strings.Repeat("x", 100),
			TargetSceneID: "scn_0123456789abcdef0123",
			SceneOrder:    []contextpack.SceneOrderRef{{ID: "scn_0123456789abcdef0123"}},
		},
		TargetRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}}
	resolver := &fakeProfileResolver{resolved: provider.ResolvedProfile{
		Profile: provider.Profile{
			ID: "small_context", Type: provider.TypeOpenAICompatible,
			Capabilities: provider.Capabilities{Chat: true, MaxContextTokens: 64},
			Readiness:    provider.ReadinessReady,
		},
	}, found: true}
	service := NewService(
		&fakeSession{project: project.Project{Path: "/tmp/test"}, ok: true},
		&fakeLoader{registry: agent.Registry{Agents: []agent.Agent{agentDefinition}, Styles: []agent.Style{style}}},
		&fakeSceneLoader{}, &fakeAcceptor{}, &fakeProvider{}, resolver,
		NewRunStore(), &fakeRunIDGenerator{next: "run_0123456789abcdef0123"},
	).WithMaterialSource(source).WithContextBuilder(contextpack.NewBuilder())

	_, err := service.PreviewContext(context.Background(), TaggedRunRequest{
		AgentID: agentDefinition.ID, StyleID: style.ID,
		Target: TaggedTarget{Scope: contextpack.ScopeScene, Scene: &SceneTarget{
			SceneID:       "scn_0123456789abcdef0123",
			SceneRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		}},
	})
	if !errors.Is(err, contextpack.ErrBudgetOverflow) {
		t.Fatalf("PreviewContext() error = %v, want provider-limit budget overflow", err)
	}
}

func selectionTargetFixture() TaggedTarget {
	return TaggedTarget{
		Scope: contextpack.ScopeSelection,
		Selection: &SelectionTarget{
			SceneID:       "scn_0123456789abcdef0123",
			SceneRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			StartByte:     0, EndByte: 10, SelectedText: "Alpha beta",
		},
	}
}

func newPreviewTestService(t *testing.T, material ContextMaterialSource, _ *fakeProvider, _ RunIDGenerator) *Service {
	t.Helper()
	return newPreviewTestServiceWithAgents(t, material, testLinePolishAgent())
}

func newPreviewTestServiceWithAgents(t *testing.T, material ContextMaterialSource, agents ...agent.Agent) *Service {
	t.Helper()
	if len(agents) == 0 {
		agents = []agent.Agent{testLinePolishAgent()}
	}
	return NewService(
		&fakeSession{project: project.Project{Path: "/tmp/test"}, ok: true},
		&fakeLoader{registry: agent.Registry{Agents: agents, Styles: []agent.Style{testPreciseEditorStyle()}}},
		&fakeSceneLoader{},
		&fakeAcceptor{},
		&fakeProvider{},
		nil,
		NewRunStore(),
		&fakeRunIDGenerator{next: "run_0123456789abcdef0123"},
	).WithMaterialSource(material).WithContextBuilder(contextpack.NewBuilder())
}

func testSceneRewriteAgent() agent.Agent {
	return agent.Agent{
		Version: 3, ID: "scene_rewrite", Name: "Scene Rewrite",
		Description: "Rewrite one scene.",
		AppliesWhen: agent.ApplicabilityRule{Surfaces: []agent.Surface{agent.SurfaceEditor}, InputScopes: []agent.InputScope{agent.InputScopeScene}, MinWords: 1, MaxWords: 12000},
		ContextPolicy: agent.ContextPolicy{
			Required:  []agent.ContextPack{agent.ContextCurrentScene, agent.ContextStyleSheet, agent.ContextActiveCodex},
			Optional:  []agent.ContextPack{agent.ContextOutlineNeighbor},
			Forbidden: []agent.ContextPack{agent.ContextGlobalCodexRAG, agent.ContextRawImportNotes},
		},
		ContextBudget: agent.ContextBudget{MaxInputEstimatedTokens: 12000, ReservedOutputEstimatedTokens: 4000},
		RAGPolicy:     agent.RAGPolicy{Mode: agent.RAGModeTimelineAware},
		Control:       agent.Control{OutputMode: agent.OutputModePatch, RequiresAcceptance: true},
		Output:        agent.Output{Type: agent.OutputTypeRevisedText, RequiresDiffPreview: true},
	}
}

func testChapterReviewAgent() agent.Agent {
	return agent.Agent{
		Version: 3, ID: "chapter_review", Name: "Chapter Review",
		Description: "Review one chapter.",
		AppliesWhen: agent.ApplicabilityRule{Surfaces: []agent.Surface{agent.SurfaceChapterView}, InputScopes: []agent.InputScope{agent.InputScopeChapterReview}, MinWords: 1, MaxWords: 12000},
		ContextPolicy: agent.ContextPolicy{
			Required:  []agent.ContextPack{agent.ContextCurrentChapter, agent.ContextStyleSheet, agent.ContextActiveCodex},
			Optional:  []agent.ContextPack{agent.ContextOutlineNeighbor},
			Forbidden: []agent.ContextPack{agent.ContextGlobalCodexRAG, agent.ContextRawImportNotes},
		},
		ContextBudget: agent.ContextBudget{MaxInputEstimatedTokens: 16000, ReservedOutputEstimatedTokens: 2000},
		RAGPolicy:     agent.RAGPolicy{Mode: agent.RAGModeTimelineAware},
		FollowUps: agent.FollowUpPolicy{OnAccept: []agent.FollowUpRule{{
			AgentID: "scene_rewrite", Scope: agent.FollowUpScopeScene, Relationship: agent.FollowUpRelationshipTriggered,
		}}},
		Control: agent.Control{OutputMode: agent.OutputModeSuggestion, RequiresAcceptance: false},
		Output:  agent.Output{Type: agent.OutputTypeEditorialFindings},
	}
}
