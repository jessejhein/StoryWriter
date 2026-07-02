package importer

import (
	"context"
	"sync"
	"testing"
	"time"

	"storywork/internal/mutation"
	"storywork/internal/project"
)

// TestReviewMutationsShareTheApplicationMutationCoordinator proves review
// transactions for different candidates cannot overlap their Git checkpoints.
func TestReviewMutationsShareTheApplicationMutationCoordinator(t *testing.T) {
	t.Parallel()

	projectPath := t.TempDir()
	store := NewCandidateStore()
	first := mustCreateArcCandidate(t, store, projectPath, "cand_0123456789abcdef0001", "First")
	second := mustCreateArcCandidate(t, store, projectPath, "cand_0123456789abcdef0002", "Second")
	git := newBlockingImporterGit()
	coordinator := mutation.NewCoordinator()
	service := NewService(
		fakeImporterSession{current: project.Project{Path: projectPath}, ok: true},
		git,
		&importerIndexStub{},
		NewSourceStore(),
		&fakeImporterIDs{},
		time.Now,
	).WithMutationCoordinator(coordinator)

	errorsByCandidate := make(chan error, 2)
	go func() {
		_, err := service.DiscardCandidate(context.Background(), first.ID, first.Revision)
		errorsByCandidate <- err
	}()
	<-git.firstCommitEntered

	go func() {
		_, err := service.DiscardCandidate(context.Background(), second.ID, second.Revision)
		errorsByCandidate <- err
	}()

	select {
	case <-git.secondCleanCheck:
		t.Fatal("second review transaction passed the clean check before the first checkpoint completed")
	case <-time.After(100 * time.Millisecond):
	}

	close(git.releaseFirstCommit)
	for range 2 {
		if err := <-errorsByCandidate; err != nil {
			t.Fatalf("DiscardCandidate() error = %v", err)
		}
	}
	if git.commitCount() != 2 {
		t.Fatalf("commit count = %d, want 2 isolated checkpoints", git.commitCount())
	}
}

func mustCreateArcCandidate(t *testing.T, store *CandidateStore, projectPath, id, title string) Candidate {
	t.Helper()
	candidate, err := store.Create(projectPath, Candidate{
		Version: CandidateVersion, ID: id, Kind: CandidateKindArc,
		ProposalVersion: 1, Status: CandidateStatusPending,
		Provenance: Provenance{ChunkIDs: []string{"chk_0123456789abcdef0123"}},
		Proposal:   CandidateProposal{Arc: &ArcProposal{Title: title}},
		Decision:   CandidateDecision{CanonicalRefs: []CanonicalRef{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return candidate
}

type blockingImporterGit struct {
	mu                 sync.Mutex
	cleanChecks        int
	commits            int
	firstCommitEntered chan struct{}
	releaseFirstCommit chan struct{}
	secondCleanCheck   chan struct{}
}

func newBlockingImporterGit() *blockingImporterGit {
	return &blockingImporterGit{
		firstCommitEntered: make(chan struct{}),
		releaseFirstCommit: make(chan struct{}),
		secondCleanCheck:   make(chan struct{}),
	}
}

func (g *blockingImporterGit) IsClean(context.Context, string) (bool, error) {
	g.mu.Lock()
	g.cleanChecks++
	check := g.cleanChecks
	g.mu.Unlock()
	if check == 2 {
		close(g.secondCleanCheck)
	}
	return true, nil
}

func (g *blockingImporterGit) CommitAll(context.Context, string, string) error {
	g.mu.Lock()
	g.commits++
	commit := g.commits
	g.mu.Unlock()
	if commit == 1 {
		close(g.firstCommitEntered)
		<-g.releaseFirstCommit
	}
	return nil
}

func (g *blockingImporterGit) UnstageAll(context.Context, string) error { return nil }

func (g *blockingImporterGit) commitCount() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.commits
}
