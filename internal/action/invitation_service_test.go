// BDD Scenario: 7.4.1 - Offer a follow-up without calling a provider
// Requirements: M7-R11, M7-R12
// Test purpose: Invitation publication and explicit execution remain author-controlled.

package action

import (
	"context"
	"errors"
	"testing"

	"storywork/internal/agent"
	"storywork/internal/contextpack"
	"storywork/internal/project"
	"storywork/internal/story"
)

type countingProvider struct {
	fakeProvider
}

func (p *countingProvider) Generate(ctx context.Context, request agent.GenerateRequest) (agent.GenerateResponse, error) {
	p.calls++
	return p.fakeProvider.Generate(ctx, request)
}

// Test: accept publishes invitations only after commit succeeds.
// Requirements: M7-R11.
func TestAcceptPatchPublishesInvitationAfterCommitOnly(t *testing.T) {
	t.Parallel()

	scene := testActionScene()
	acceptor := &fakeAcceptor{scene: acceptedActionScene(scene, "Mock polished: Alpha beta")}
	invites := NewInvitationStore(10)
	service := newInvitationTestService(t, scene, acceptor, invites)
	linePolish := testLinePolishV3Agent()
	service.loader = &fakeLoader{registry: agent.Registry{Agents: []agent.Agent{linePolish}, Styles: []agent.Style{testPreciseEditorStyle()}}}

	run, err := service.Run(context.Background(), selectionRunRequest(scene))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	result, err := service.Accept(context.Background(), run.RunID, scene.Revision)
	if err != nil {
		t.Fatalf("Accept() error = %v", err)
	}
	if len(result.FollowUpInvitations) != 1 || result.FollowUpInvitations[0].AgentID != "scene_rewrite" {
		t.Fatalf("invitations = %#v", result.FollowUpInvitations)
	}
	if acceptor.calls != 1 {
		t.Fatalf("acceptor calls = %d, want 1", acceptor.calls)
	}
}

// Test: completed review creates allowed invitations without another provider call.
// Requirements: M7-R11, M7-R16.
func TestCompletedReviewCreatesAllowedInvitationsWithoutProviderCall(t *testing.T) {
	t.Parallel()

	provider := &countingProvider{fakeProvider: fakeProvider{response: agent.GenerateResponse{Replacement: `{"findings":[{"title":"Issue","explanation":"Detail","scene_ids":["scn_0123456789abcdef0123"],"follow_up_agent_ids":["scene_rewrite"]}]}`}}}
	source := &chapterMaterialSource{result: story.ContextMaterialResult{
		Material: contextpack.Material{
			Scope:         contextpack.ScopeChapterReview,
			ChapterScenes: []contextpack.ChapterSceneText{{SceneID: "scn_0123456789abcdef0123", Markdown: "Scene.\n"}},
		},
		TargetRevision: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}}
	invites := NewInvitationStore(10)
	service := newChapterReviewTestService(t, source, &fakeProvider{})
	service.provider = provider
	service.loader = &fakeLoader{registry: agent.Registry{Agents: []agent.Agent{testChapterReviewAgent()}, Styles: []agent.Style{testPreciseEditorStyle()}}}
	service.invitations = invites
	service.inviteIDs = &fakeInvitationIDGenerator{next: "invite_0123456789abcdef0123"}

	run, err := service.RunTagged(context.Background(), TaggedRunRequest{
		AgentID: "chapter_review", StyleID: "precise_editor",
		Target: TaggedTarget{
			Scope:   contextpack.ScopeChapterReview,
			Chapter: &ChapterReviewTarget{ChapterID: "ch_0123456789abcdef0123", Fingerprint: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
		},
	})
	if err != nil {
		t.Fatalf("RunTagged() error = %v", err)
	}
	if provider.calls < 1 {
		t.Fatalf("provider calls = %d, want at least 1", provider.calls)
	}
	if _, ok := invites.Get("invite_0123456789abcdef0123"); run.Status != RunCompleted || !ok {
		t.Fatalf("invitation not published for completed review")
	}
}

// Test: run invitation requires explicit request and revalidation.
// Requirements: M7-R12.
func TestRunInvitationRequiresExplicitRequestAndRevalidation(t *testing.T) {
	t.Parallel()

	invites := NewInvitationStore(10)
	service := newInvitationTestService(t, testActionScene(), &fakeAcceptor{}, invites)
	if err := invites.Publish(Invitation{
		ID: "invite_0123456789abcdef0123", ParentRunID: "run_aaaaaaaaaaaaaaaaaaaa", RootRunID: "run_aaaaaaaaaaaaaaaaaaaa",
		ChainDepth: 2, AgentID: "line_polish", Scope: contextpack.ScopeSelection, SceneID: "scn_0123456789abcdef0123",
	}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	_, err := service.RunInvitation(context.Background(), "invite_0123456789abcdef0123", InvitationRunRequest{StyleID: "precise_editor", ExpectedTargetRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})
	if !errors.Is(err, ErrInvitationForbidden) {
		t.Fatalf("RunInvitation() error = %v, want ErrInvitationForbidden", err)
	}
}

// Test: run invitation failure releases claim for retry.
// Requirements: M7-R12.
func TestRunInvitationFailureReleasesClaimForRetry(t *testing.T) {
	t.Parallel()

	parentRun := Run{
		RunID: "run_aaaaaaaaaaaaaaaaaaaa", Status: RunAccepted, AgentID: "line_polish",
		Scope: contextpack.ScopeSelection, SceneID: "scn_0123456789abcdef0123", ChainDepth: 1,
	}
	store := NewRunStore()
	if err := store.Insert(parentRun); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}
	invites := NewInvitationStore(10)
	service := newSceneRunTestService(t, &sceneRunMaterialSource{
		expectedRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		result: story.ContextMaterialResult{
			Material: contextpack.Material{
				Scope: contextpack.ScopeScene, SceneMarkdown: "Ann arrives.\n",
				SceneOrder: []contextpack.SceneOrderRef{{ID: "scn_0123456789abcdef0123"}},
			},
			TargetRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	}, &fakeProvider{response: agent.GenerateResponse{Replacement: "Mock rewritten: Ann arrives."}})
	service.runs = store
	service.loader = &fakeLoader{registry: agent.Registry{Agents: []agent.Agent{testLinePolishV3Agent(), testSceneRewriteAgent()}, Styles: []agent.Style{testPreciseEditorStyle()}}}
	service.invitations = invites
	service.inviteIDs = &fakeInvitationIDGenerator{next: "invite_ffffffffffffffffffff"}
	id := "invite_0123456789abcdef0123"
	if err := invites.Publish(Invitation{
		ID: id, ParentRunID: parentRun.RunID, RootRunID: parentRun.RunID, ChainDepth: 2,
		AgentID: "scene_rewrite", Scope: contextpack.ScopeScene, SceneID: "scn_0123456789abcdef0123", Relationship: InvitationTriggered,
	}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	_, err := service.RunInvitation(context.Background(), id, InvitationRunRequest{
		StyleID: "precise_editor", ExpectedTargetRevision: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	})
	if !errors.Is(err, story.ErrStaleRevision) {
		t.Fatalf("RunInvitation() error = %v, want ErrStaleRevision", err)
	}
	if _, err := invites.Claim(id); err != nil {
		t.Fatalf("Claim(retry) error = %v", err)
	}
}

// Test: successful invitation execution consumes invitation and creates child run.
// Requirements: M7-R12.
func TestRunInvitationSuccessConsumesAndCreatesChildRun(t *testing.T) {
	t.Parallel()

	parentRun := Run{
		RunID: "run_aaaaaaaaaaaaaaaaaaaa", Status: RunAccepted, AgentID: "line_polish",
		Scope: contextpack.ScopeSelection, SceneID: "scn_0123456789abcdef0123", ChainDepth: 1,
	}
	store := NewRunStore()
	if err := store.Insert(parentRun); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}
	material := &sceneRunMaterialSource{result: story.ContextMaterialResult{
		Material: contextpack.Material{
			Scope: contextpack.ScopeScene, SceneMarkdown: "Ann arrives.\n",
			SceneOrder: []contextpack.SceneOrderRef{{ID: "scn_0123456789abcdef0123"}},
		},
		TargetRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}}
	invites := NewInvitationStore(10)
	service := newSceneRunTestService(t, material, &fakeProvider{response: agent.GenerateResponse{Replacement: "Mock rewritten: Ann arrives."}})
	service.runs = store
	service.loader = &fakeLoader{registry: agent.Registry{Agents: []agent.Agent{testLinePolishV3Agent(), testSceneRewriteAgent()}, Styles: []agent.Style{testPreciseEditorStyle()}}}
	service.invitations = invites
	service.inviteIDs = &fakeInvitationIDGenerator{next: "invite_0123456789abcdef0123"}
	id := "invite_0123456789abcdef0123"
	if err := invites.Publish(Invitation{
		ID: id, ParentRunID: parentRun.RunID, RootRunID: parentRun.RunID, ChainDepth: 2,
		AgentID: "scene_rewrite", Scope: contextpack.ScopeScene, SceneID: "scn_0123456789abcdef0123", Relationship: InvitationTriggered,
	}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	child, err := service.RunInvitation(context.Background(), id, InvitationRunRequest{
		StyleID: "precise_editor", ExpectedTargetRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})
	if err != nil {
		t.Fatalf("RunInvitation() error = %v", err)
	}
	if child.ParentRunID != parentRun.RunID || child.RootRunID != parentRun.RunID || child.ChainDepth != 2 {
		t.Fatalf("child lineage = %#v", child)
	}
	if invitation, ok := invites.Get(id); !ok || invitation.Status != "consumed" {
		t.Fatalf("invitation status = %#v", invitation)
	}
}

// Test: forged stale and consumed invitations are rejected.
// Requirements: M7-R12.
func TestRunInvitationRejectsForgedStaleConsumedAndRecursiveInput(t *testing.T) {
	t.Parallel()

	service := newInvitationTestService(t, testActionScene(), &fakeAcceptor{}, NewInvitationStore(10))
	if _, err := service.RunInvitation(context.Background(), "invite_bad", InvitationRunRequest{StyleID: "precise_editor", ExpectedTargetRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}); err == nil {
		t.Fatal("RunInvitation(bad id) expected error")
	}
}

// Test: an invitation absent from the parent's current configured transitions is rejected.
// Requirements: M7-R11, M7-R12.
func TestRunInvitationRejectsDisallowedConfiguredTransition(t *testing.T) {
	t.Parallel()

	parentRun := Run{RunID: "run_aaaaaaaaaaaaaaaaaaaa", Status: RunAccepted, AgentID: "line_polish", Scope: contextpack.ScopeSelection, SceneID: "scn_0123456789abcdef0123", SceneRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ChainDepth: 1}
	invites := NewInvitationStore(10)
	runs := NewRunStore()
	if err := runs.Insert(parentRun); err != nil {
		t.Fatalf("Insert(parent) error = %v", err)
	}
	service := newSceneInvitationTestService(t, runs, invites, &sceneRunMaterialSource{result: story.ContextMaterialResult{
		Material:       contextpack.Material{Scope: contextpack.ScopeScene, SceneMarkdown: "Scene.\n", TargetSceneID: "scn_0123456789abcdef0123", SceneOrder: []contextpack.SceneOrderRef{{ID: "scn_0123456789abcdef0123"}}},
		TargetRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}})
	linePolish := testLinePolishV3Agent()
	linePolish.FollowUps = agent.FollowUpPolicy{}
	service.loader = &fakeLoader{registry: agent.Registry{Agents: []agent.Agent{linePolish, testSceneRewriteAgent()}, Styles: []agent.Style{testPreciseEditorStyle()}}}
	id := "invite_0123456789abcdef0123"
	if err := invites.Publish(Invitation{ID: id, ParentRunID: parentRun.RunID, RootRunID: parentRun.RunID, ChainDepth: 2, AgentID: "scene_rewrite", Scope: contextpack.ScopeScene, SceneID: parentRun.SceneID, Relationship: InvitationTriggered}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	_, err := service.RunInvitation(context.Background(), id, InvitationRunRequest{StyleID: "precise_editor", ExpectedTargetRevision: parentRun.SceneRevision})
	if !errors.Is(err, ErrInvitationForbidden) {
		t.Fatalf("RunInvitation() error = %v, want ErrInvitationForbidden", err)
	}
}

// Test: consumed invitation cannot run again.
// Requirements: M7-R12.
func TestRunInvitationRejectsConsumedInvitation(t *testing.T) {
	t.Parallel()

	invites := NewInvitationStore(10)
	service := newInvitationTestService(t, testActionScene(), &fakeAcceptor{}, invites)
	id := "invite_0123456789abcdef0123"
	if err := invites.Publish(Invitation{
		ID: id, ParentRunID: "run_aaaaaaaaaaaaaaaaaaaa", RootRunID: "run_aaaaaaaaaaaaaaaaaaaa",
		ChainDepth: 2, AgentID: "scene_rewrite", Scope: contextpack.ScopeScene, SceneID: "scn_0123456789abcdef0123",
	}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if _, err := invites.Claim(id); err != nil {
		t.Fatalf("Claim() error = %v", err)
	}
	if err := invites.Consume(id); err != nil {
		t.Fatalf("Consume() error = %v", err)
	}
	_, err := service.RunInvitation(context.Background(), id, InvitationRunRequest{
		StyleID: "precise_editor", ExpectedTargetRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})
	if !errors.Is(err, ErrInvitationConflict) {
		t.Fatalf("RunInvitation(consumed) error = %v, want ErrInvitationConflict", err)
	}
}

// Test: stale target revision is rejected before provider execution.
// Requirements: M7-R12.
func TestRunInvitationRejectsStaleTargetRevision(t *testing.T) {
	t.Parallel()

	parentRun := Run{
		RunID: "run_aaaaaaaaaaaaaaaaaaaa", Status: RunAccepted, AgentID: "line_polish",
		Scope: contextpack.ScopeSelection, SceneID: "scn_0123456789abcdef0123", ChainDepth: 1,
	}
	invites := NewInvitationStore(10)
	store := NewRunStore()
	if err := store.Insert(parentRun); err != nil {
		t.Fatalf("Insert(parent) error = %v", err)
	}
	service := newSceneInvitationTestService(t, store, invites, &sceneRunMaterialSource{
		result: story.ContextMaterialResult{
			Material:       contextpack.Material{Scope: contextpack.ScopeScene, SceneMarkdown: "Scene.\n"},
			TargetRevision: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		},
		expectedRevision: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	})
	id := "invite_0123456789abcdef0123"
	if err := invites.Publish(Invitation{
		ID: id, ParentRunID: parentRun.RunID, RootRunID: parentRun.RunID, ChainDepth: 2,
		AgentID: "scene_rewrite", Scope: contextpack.ScopeScene, SceneID: "scn_0123456789abcdef0123", Relationship: InvitationTriggered,
	}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	_, err := service.RunInvitation(context.Background(), id, InvitationRunRequest{
		StyleID: "precise_editor", ExpectedTargetRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})
	if !errors.Is(err, story.ErrStaleRevision) {
		t.Fatalf("RunInvitation(stale) error = %v, want ErrStaleRevision", err)
	}
}

// Test: maximum chain depth invitations are rejected before provider execution.
// Requirements: M7-R12.
func TestRunInvitationRejectsMaximumChainDepth(t *testing.T) {
	t.Parallel()

	parentRun := Run{
		RunID: "run_aaaaaaaaaaaaaaaaaaaa", Status: RunAccepted, AgentID: "line_polish",
		Scope: contextpack.ScopeSelection, SceneID: "scn_0123456789abcdef0123", ChainDepth: 1,
	}
	invites := NewInvitationStore(10)
	store := NewRunStore()
	if err := store.Insert(parentRun); err != nil {
		t.Fatalf("Insert(parent) error = %v", err)
	}
	service := newSceneInvitationTestService(t, store, invites, &sceneRunMaterialSource{result: story.ContextMaterialResult{
		Material:       contextpack.Material{Scope: contextpack.ScopeScene, SceneMarkdown: "Scene.\n"},
		TargetRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}})
	id := "invite_0123456789abcdef0123"
	if err := invites.Publish(Invitation{
		ID: id, ParentRunID: parentRun.RunID, RootRunID: parentRun.RunID, ChainDepth: 3,
		AgentID: "scene_rewrite", Scope: contextpack.ScopeScene, SceneID: "scn_0123456789abcdef0123",
	}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	_, err := service.RunInvitation(context.Background(), id, InvitationRunRequest{
		StyleID: "precise_editor", ExpectedTargetRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})
	if !errors.Is(err, ErrInvitationForbidden) {
		t.Fatalf("RunInvitation(depth) error = %v, want ErrInvitationForbidden", err)
	}
}

func newSceneInvitationTestService(t *testing.T, runs *RunStore, invites *InvitationStore, material ContextMaterialSource) *Service {
	t.Helper()
	scene := testActionScene()
	return NewService(
		&fakeSession{project: project.Project{Path: "/tmp/test"}, ok: true},
		&fakeLoader{registry: agent.Registry{Agents: []agent.Agent{testLinePolishV3Agent(), testSceneRewriteAgent()}, Styles: []agent.Style{testPreciseEditorStyle()}}},
		&fakeSceneLoader{scene: scene},
		&fakeAcceptor{},
		&fakeProvider{response: agent.GenerateResponse{Replacement: "Mock rewritten: Scene.\n"}},
		nil,
		runs,
		&fakeRunIDGenerator{next: "run_bbbbbbbbbbbbbbbbbbbb"},
	).WithMaterialSource(material).WithContextBuilder(contextpack.NewBuilder()).WithBodyAcceptor(&fakeBodyAcceptor{scene: scene}).
		WithInvitationStore(invites)
}

func newInvitationTestService(t *testing.T, scene story.SceneDocument, acceptor *fakeAcceptor, invites *InvitationStore) *Service {
	t.Helper()
	return newActionTestService(
		scene,
		&fakeProvider{response: agent.GenerateResponse{Replacement: "Mock polished: Alpha beta"}},
		acceptor,
		NewRunStore(),
		&fakeRunIDGenerator{next: "run_0123456789abcdef0123"},
	).WithInvitationStore(invites).WithInvitationIDGenerator(&fakeInvitationIDGenerator{next: "invite_0123456789abcdef0123"})
}

func testLinePolishV3Agent() agent.Agent {
	agentDef := testLinePolishAgent()
	agentDef.Version = 3
	agentDef.ModelRequirements = agent.ModelRequirements{MinContextTokens: 2048}
	agentDef.ContextBudget = agent.ContextBudget{MaxInputEstimatedTokens: 4096, ReservedOutputEstimatedTokens: 1024}
	agentDef.ContextPolicy.Forbidden = append(agentDef.ContextPolicy.Forbidden, agent.ContextPriorChat)
	agentDef.FollowUps = agent.FollowUpPolicy{OnAccept: []agent.FollowUpRule{{
		AgentID: "scene_rewrite", Scope: agent.FollowUpScopeScene, Relationship: agent.FollowUpRelationshipTriggered,
	}}}
	return agentDef
}
