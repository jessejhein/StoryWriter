// BDD Scenario: 8.1.3 - Continue normal work in the experiment
// Requirements: M8-R18, M8-R19, M8-R20
// Test purpose: action runs bind provider output to the branch/head that built
// their context and reject in-flight branch changes before transient storage.

package action

import (
	"context"
	"errors"
	"sync"
	"testing"

	"storywork/internal/agent"
	"storywork/internal/contextpack"
	"storywork/internal/project"
	"storywork/internal/story"
)

const (
	branchA = "branch/race-0123456789abcdef0123"
	branchB = "main"
	headA   = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	headB   = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

type sequenceBranchSnapshotter struct {
	mu        sync.Mutex
	snapshots []BranchSnapshot
	current   BranchSnapshot
	calls     int
}

func newSequenceBranchSnapshotter(snapshots ...BranchSnapshot) *sequenceBranchSnapshotter {
	current := BranchSnapshot{Branch: branchA, Head: headA}
	if len(snapshots) > 0 {
		current = snapshots[len(snapshots)-1]
	}
	return &sequenceBranchSnapshotter{snapshots: snapshots, current: current}
}

func (s *sequenceBranchSnapshotter) Snapshot(context.Context) (BranchSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if len(s.snapshots) == 0 {
		return s.current, nil
	}
	next := s.snapshots[0]
	s.snapshots = s.snapshots[1:]
	s.current = next
	return next, nil
}

func (s *sequenceBranchSnapshotter) Set(snapshot BranchSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots = nil
	s.current = snapshot
}

type blockingProvider struct {
	mu       sync.Mutex
	response agent.GenerateResponse
	started  chan struct{}
	release  chan struct{}
	calls    int
}

func newBlockingProvider(response agent.GenerateResponse) *blockingProvider {
	return &blockingProvider{
		response: response,
		started:  make(chan struct{}, 1),
		release:  make(chan struct{}),
	}
}

func (p *blockingProvider) Generate(_ context.Context, request agent.GenerateRequest) (agent.GenerateResponse, error) {
	p.mu.Lock()
	p.calls++
	p.mu.Unlock()
	p.started <- struct{}{}
	<-p.release
	return p.response, nil
}

func (p *blockingProvider) Calls() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

// Test: a branch/head change between context construction and provider
// execution rejects before model work and stores no transient run.
// Requirements: M8-R18, M8-R19.
func TestMilestone8BranchSnapshotChangeBeforeProviderRejectsRun(t *testing.T) {
	t.Parallel()

	runs := NewRunStore()
	provider := &fakeProvider{response: agent.GenerateResponse{Replacement: "Mock polished: Alpha beta"}}
	service := newMilestone8SelectionBranchService(t, runs, provider)
	service.WithBranchSnapshotter(newSequenceBranchSnapshotter(
		BranchSnapshot{Branch: branchA, Head: headA},
		BranchSnapshot{Branch: branchB, Head: headB},
	))

	_, err := service.RunTagged(context.Background(), TaggedRunRequest{
		AgentID: "line_polish", StyleID: "precise_editor", Target: selectionTargetFixture(),
	})
	if !errors.Is(err, ErrRunConflict) {
		t.Fatalf("RunTagged() error = %v, want ErrRunConflict", err)
	}
	if provider.calls != 0 {
		t.Fatalf("provider calls = %d, want 0", provider.calls)
	}
	if _, ok := runs.Get("run_0123456789abcdef0123"); ok {
		t.Fatal("run was inserted after branch snapshot conflict")
	}
}

// Test: a branch/head change while provider work is in flight rejects after
// provider return and stores no output under the new branch.
// Requirements: M8-R18, M8-R19.
func TestMilestone8BranchSnapshotChangeDuringProviderRejectsWithoutStoredRun(t *testing.T) {
	t.Parallel()

	runs := NewRunStore()
	provider := newBlockingProvider(agent.GenerateResponse{Replacement: "Mock polished: Alpha beta"})
	service := newMilestone8SelectionBranchService(t, runs, provider)
	snapshots := newSequenceBranchSnapshotter(BranchSnapshot{Branch: branchA, Head: headA})
	service.WithBranchSnapshotter(snapshots)

	errCh := make(chan error, 1)
	go func() {
		_, err := service.RunTagged(context.Background(), TaggedRunRequest{
			AgentID: "line_polish", StyleID: "precise_editor", Target: selectionTargetFixture(),
		})
		errCh <- err
	}()

	<-provider.started
	snapshots.Set(BranchSnapshot{Branch: branchB, Head: headB})
	close(provider.release)

	if err := <-errCh; !errors.Is(err, ErrRunConflict) {
		t.Fatalf("RunTagged() error = %v, want ErrRunConflict", err)
	}
	if provider.Calls() != 1 {
		t.Fatalf("provider calls = %d, want 1", provider.Calls())
	}
	if _, ok := runs.Get("run_0123456789abcdef0123"); ok {
		t.Fatal("run was inserted after in-flight branch snapshot conflict")
	}
}

// Test: unchanged branch/head snapshots still allow the run and store the
// exact branch/head that guarded context construction.
// Requirements: M8-R19.
func TestMilestone8StableBranchSnapshotStoresRunSnapshot(t *testing.T) {
	t.Parallel()

	runs := NewRunStore()
	service := newMilestone8SelectionBranchService(t, runs, &fakeProvider{
		response: agent.GenerateResponse{Replacement: "Mock polished: Alpha beta"},
	})
	service.WithBranchSnapshotter(newSequenceBranchSnapshotter(
		BranchSnapshot{Branch: branchA, Head: headA},
		BranchSnapshot{Branch: branchA, Head: headA},
		BranchSnapshot{Branch: branchA, Head: headA},
	))

	run, err := service.RunTagged(context.Background(), TaggedRunRequest{
		AgentID: "line_polish", StyleID: "precise_editor", Target: selectionTargetFixture(),
	})
	if err != nil {
		t.Fatalf("RunTagged() error = %v", err)
	}
	if run.Branch != branchA || run.BranchHead != headA {
		t.Fatalf("run snapshot = %q/%q, want %q/%q", run.Branch, run.BranchHead, branchA, headA)
	}
}

// Test: invitation execution inherits the in-flight branch guard and releases
// the invitation claim when a provider-time branch change invalidates the run.
// Requirements: M8-R18, M8-R19.
func TestRunInvitationBranchSnapshotChangeDuringProviderReleasesClaim(t *testing.T) {
	t.Parallel()

	parentRun := Run{
		RunID: "run_aaaaaaaaaaaaaaaaaaaa", Status: RunAccepted, AgentID: "line_polish",
		Scope: contextpack.ScopeSelection, SceneID: "scn_0123456789abcdef0123", ChainDepth: 1,
	}
	runs := NewRunStore()
	if err := runs.Insert(parentRun); err != nil {
		t.Fatalf("Insert(parent) error = %v", err)
	}
	invites := NewInvitationStore(10)
	id := "invite_0123456789abcdef0123"
	if err := invites.Publish(Invitation{
		ID: id, ParentRunID: parentRun.RunID, RootRunID: parentRun.RunID, ChainDepth: 2,
		AgentID: "scene_rewrite", Scope: contextpack.ScopeScene, SceneID: parentRun.SceneID, Relationship: InvitationTriggered,
		Branch: branchA, BranchHead: headA,
	}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	provider := newBlockingProvider(agent.GenerateResponse{Replacement: "Mock rewritten: Scene.\n"})
	service := newSceneInvitationTestService(t, runs, invites, &sceneRunMaterialSource{result: story.ContextMaterialResult{
		Material:       contextpack.Material{Scope: contextpack.ScopeScene, SceneMarkdown: "Scene.\n", TargetSceneID: parentRun.SceneID, SceneOrder: []contextpack.SceneOrderRef{{ID: parentRun.SceneID}}},
		TargetRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}})
	service.provider = provider
	service.loader = &fakeLoader{registry: agent.Registry{Agents: []agent.Agent{testLinePolishV3Agent(), testSceneRewriteAgent()}, Styles: []agent.Style{testPreciseEditorStyle()}}}
	snapshots := newSequenceBranchSnapshotter(BranchSnapshot{Branch: branchA, Head: headA})
	service.WithBranchSnapshotter(snapshots)

	errCh := make(chan error, 1)
	go func() {
		_, err := service.RunInvitation(context.Background(), id, InvitationRunRequest{
			StyleID: "precise_editor", ExpectedTargetRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		})
		errCh <- err
	}()

	<-provider.started
	snapshots.Set(BranchSnapshot{Branch: branchB, Head: headB})
	close(provider.release)

	if err := <-errCh; !errors.Is(err, ErrRunConflict) {
		t.Fatalf("RunInvitation() error = %v, want ErrRunConflict", err)
	}
	if _, ok := runs.Get("run_bbbbbbbbbbbbbbbbbbbb"); ok {
		t.Fatal("child run was inserted after in-flight branch snapshot conflict")
	}
	invitation, ok := invites.Get(id)
	if !ok || invitation.Status != "offered" {
		t.Fatalf("invitation after failed run = %#v, exists %v; want offered", invitation, ok)
	}
}

// Test: completed suggestion runs publish follow-up invitations against the
// run's validated branch snapshot instead of re-sampling a later branch state.
// Requirements: M8-R18, M8-R19.
func TestMilestone8CompletedRunInvitationsUseValidatedBranchSnapshot(t *testing.T) {
	t.Parallel()

	invites := NewInvitationStore(10)
	service := NewService(
		&fakeSession{project: project.Project{Path: "/tmp/test-project"}, ok: true},
		&fakeLoader{registry: agent.Registry{Agents: []agent.Agent{testChapterReviewAgent(), testSceneRewriteAgent()}, Styles: []agent.Style{testPreciseEditorStyle()}}},
		&fakeSceneLoader{},
		&fakeAcceptor{},
		&fakeProvider{},
		nil,
		NewRunStore(),
		&fakeRunIDGenerator{next: "run_0123456789abcdef0123"},
	).WithInvitationStore(invites).
		WithInvitationIDGenerator(&fakeInvitationIDGenerator{next: "invite_0123456789abcdef0123"}).
		WithBranchSnapshotter(newSequenceBranchSnapshotter(BranchSnapshot{Branch: branchB, Head: headB}))

	run := Run{
		RunID: "run_0123456789abcdef0123", Status: RunCompleted, AgentID: "chapter_review",
		Scope: contextpack.ScopeChapterReview, ChapterID: "ch_0123456789abcdef0123",
		Branch: branchA, BranchHead: headA,
	}
	published, err := service.publishPreparedInvitations(run, []preparedInvitation{{
		AgentID: "scene_rewrite", Scope: contextpack.ScopeScene, SceneID: "scn_0123456789abcdef0123",
		Relationship: InvitationTriggered,
	}})
	if err != nil {
		t.Fatalf("publishPreparedInvitations() error = %v", err)
	}
	if len(published) != 1 {
		t.Fatalf("published invitations = %#v", published)
	}
	invitation, ok := invites.Get(published[0].InvitationID)
	if !ok {
		t.Fatal("published invitation missing from store")
	}
	if invitation.Branch != branchA || invitation.BranchHead != headA {
		t.Fatalf("invitation snapshot = %q/%q, want %q/%q", invitation.Branch, invitation.BranchHead, branchA, headA)
	}
}

// Test: accepted patch runs refresh invitation snapshots after the commit so
// follow-ups target the current branch head rather than the pre-accept run head.
// Requirements: M8-R19.
func TestMilestone8AcceptedRunInvitationsRefreshCommittedBranchSnapshot(t *testing.T) {
	t.Parallel()

	invites := NewInvitationStore(10)
	service := NewService(
		&fakeSession{project: project.Project{Path: "/tmp/test-project"}, ok: true},
		&fakeLoader{registry: agent.Registry{Agents: []agent.Agent{testLinePolishV3Agent(), testSceneRewriteAgent()}, Styles: []agent.Style{testPreciseEditorStyle()}}},
		&fakeSceneLoader{scene: testActionScene()},
		&fakeAcceptor{},
		&fakeProvider{},
		nil,
		NewRunStore(),
		&fakeRunIDGenerator{next: "run_0123456789abcdef0123"},
	).WithInvitationStore(invites).
		WithInvitationIDGenerator(&fakeInvitationIDGenerator{next: "invite_0123456789abcdef0123"}).
		WithBranchSnapshotter(newSequenceBranchSnapshotter(BranchSnapshot{Branch: branchA, Head: headB}))

	run := Run{
		RunID: "run_0123456789abcdef0123", Status: RunAccepted, AgentID: "line_polish",
		Scope: contextpack.ScopeSelection, SceneID: "scn_0123456789abcdef0123",
		Branch: branchA, BranchHead: headA,
	}
	published, err := service.publishPreparedInvitations(run, []preparedInvitation{{
		AgentID: "scene_rewrite", Scope: contextpack.ScopeScene, SceneID: "scn_0123456789abcdef0123",
		Relationship: InvitationTriggered,
	}})
	if err != nil {
		t.Fatalf("publishPreparedInvitations() error = %v", err)
	}
	invitation, ok := invites.Get(published[0].InvitationID)
	if !ok {
		t.Fatal("published invitation missing from store")
	}
	if invitation.Branch != branchA || invitation.BranchHead != headB {
		t.Fatalf("invitation snapshot = %q/%q, want %q/%q", invitation.Branch, invitation.BranchHead, branchA, headB)
	}
}

func newMilestone8SelectionBranchService(t *testing.T, runs *RunStore, provider agent.TextGenerator) *Service {
	t.Helper()
	scene := testActionScene()
	linePolish := testLinePolishAgent()
	return NewService(
		&fakeSession{project: project.Project{Path: "/tmp/test-project"}, ok: true},
		&fakeLoader{registry: agent.Registry{Agents: []agent.Agent{linePolish}, Styles: []agent.Style{testPreciseEditorStyle()}}},
		&fakeSceneLoader{scene: scene},
		&fakeAcceptor{},
		provider,
		nil,
		runs,
		&fakeRunIDGenerator{next: "run_0123456789abcdef0123"},
	).WithMaterialSource(selectionMaterialSource(scene, "Alpha beta")).WithContextBuilder(contextpack.NewBuilder())
}
