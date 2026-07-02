// BDD Scenario: 7.3.2 - Return suggestions without canon mutation
// Requirements: M7-R04, M7-R10, M7-R16
// Test purpose: Chapter review runs complete with strict findings and never mutate canon.

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

type chapterMaterialSource struct {
	result story.ContextMaterialResult
}

func (s *chapterMaterialSource) LoadSelectionMaterial(context.Context, story.SelectionMaterialRequest) (story.ContextMaterialResult, error) {
	return story.ContextMaterialResult{}, nil
}

func (s *chapterMaterialSource) LoadSceneMaterial(context.Context, string, string) (story.ContextMaterialResult, error) {
	return story.ContextMaterialResult{}, nil
}

func (s *chapterMaterialSource) LoadChapterMaterial(context.Context, string) (story.ContextMaterialResult, error) {
	return s.result, nil
}

// Test: chapter review run revalidates fingerprint and rebuilds context.
// Requirements: M7-R04.
func TestChapterReviewRunRevalidatesFingerprintAndContext(t *testing.T) {
	t.Parallel()

	source := &chapterMaterialSource{result: story.ContextMaterialResult{
		Material: contextpack.Material{
			Scope:         contextpack.ScopeChapterReview,
			ChapterScenes: []contextpack.ChapterSceneText{{SceneID: "scn_0123456789abcdef0123", Markdown: "Scene.\n"}},
		},
		TargetRevision: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}}
	service := newChapterReviewTestService(t, source, &fakeProvider{})

	_, err := service.RunTagged(context.Background(), TaggedRunRequest{
		AgentID: "chapter_review", StyleID: "precise_editor",
		Target: TaggedTarget{
			Scope: contextpack.ScopeChapterReview,
			Chapter: &ChapterReviewTarget{
				ChapterID:   "ch_0123456789abcdef0123",
				Fingerprint: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
		},
	})
	if !errors.Is(err, story.ErrStaleRevision) {
		t.Fatalf("RunTagged() error = %v, want ErrStaleRevision", err)
	}
}

// Test: chapter review run completes with strict findings.
// Requirements: M7-R04, M7-R10.
func TestChapterReviewRunCompletesWithStrictFindings(t *testing.T) {
	t.Parallel()

	source := &chapterMaterialSource{result: story.ContextMaterialResult{
		Material: contextpack.Material{
			Scope:         contextpack.ScopeChapterReview,
			ChapterScenes: []contextpack.ChapterSceneText{{SceneID: "scn_0123456789abcdef0123", Markdown: "Scene.\n"}},
		},
		TargetRevision: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}}
	provider := &fakeProvider{response: agent.GenerateResponse{Replacement: `{"findings":[{"title":"Issue","explanation":"Detail","scene_ids":["scn_0123456789abcdef0123"],"follow_up_agent_ids":[]}]}`}}
	service := newChapterReviewTestService(t, source, provider)

	run, err := service.RunTagged(context.Background(), TaggedRunRequest{
		AgentID: "chapter_review", StyleID: "precise_editor",
		Target: TaggedTarget{
			Scope: contextpack.ScopeChapterReview,
			Chapter: &ChapterReviewTarget{
				ChapterID:   "ch_0123456789abcdef0123",
				Fingerprint: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			},
		},
	})
	if err != nil {
		t.Fatalf("RunTagged() error = %v", err)
	}
	if run.Status != RunCompleted || len(run.Findings) != 1 {
		t.Fatalf("run = %#v", run)
	}
}

// Test: chapter review run creates no canonical mutation path.
// Requirements: M7-R16.
func TestChapterReviewRunCreatesNoCanonicalIndexOrGitMutation(t *testing.T) {
	t.Parallel()

	acceptor := &fakeAcceptor{}
	source := &chapterMaterialSource{result: story.ContextMaterialResult{
		Material: contextpack.Material{
			Scope:         contextpack.ScopeChapterReview,
			ChapterScenes: []contextpack.ChapterSceneText{{SceneID: "scn_0123456789abcdef0123", Markdown: "Scene.\n"}},
		},
		TargetRevision: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}}
	provider := &fakeProvider{response: agent.GenerateResponse{Replacement: `{"findings":[]}`}}
	service := newChapterReviewTestService(t, source, provider)
	service.acceptor = acceptor

	run, err := service.RunTagged(context.Background(), TaggedRunRequest{
		AgentID: "chapter_review", StyleID: "precise_editor",
		Target: TaggedTarget{
			Scope: contextpack.ScopeChapterReview,
			Chapter: &ChapterReviewTarget{
				ChapterID:   "ch_0123456789abcdef0123",
				Fingerprint: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			},
		},
	})
	if err != nil {
		t.Fatalf("RunTagged() error = %v", err)
	}
	if acceptorCalls(acceptor) != 0 {
		t.Fatalf("acceptor calls = %d, want 0", acceptorCalls(acceptor))
	}
	if _, err := service.Accept(context.Background(), run.RunID, "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"); !errors.Is(err, ErrRunConflict) {
		t.Fatalf("Accept() error = %v, want ErrRunConflict", err)
	}
}

// Test: invalid provider output is rejected without inserting a run.
// Requirements: M7-R16.
func TestChapterReviewRunRejectsInvalidProviderOutputWithoutRun(t *testing.T) {
	t.Parallel()

	source := &chapterMaterialSource{result: story.ContextMaterialResult{
		Material: contextpack.Material{
			Scope:         contextpack.ScopeChapterReview,
			ChapterScenes: []contextpack.ChapterSceneText{{SceneID: "scn_0123456789abcdef0123", Markdown: "Scene.\n"}},
		},
		TargetRevision: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}}
	provider := &fakeProvider{response: agent.GenerateResponse{Replacement: "not json"}}
	service := newChapterReviewTestService(t, source, provider)

	_, err := service.RunTagged(context.Background(), TaggedRunRequest{
		AgentID: "chapter_review", StyleID: "precise_editor",
		Target: TaggedTarget{
			Scope: contextpack.ScopeChapterReview,
			Chapter: &ChapterReviewTarget{
				ChapterID:   "ch_0123456789abcdef0123",
				Fingerprint: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			},
		},
	})
	if !errors.Is(err, ErrProviderInvalid) {
		t.Fatalf("RunTagged() error = %v, want ErrProviderInvalid", err)
	}
}

// Test: suggestion runs have no accept operation.
// Requirements: M7-R16.
func TestSuggestionRunHasNoAcceptOperation(t *testing.T) {
	t.Parallel()

	run := Run{RunID: "run_0123456789abcdef0123", Status: RunCompleted, Scope: contextpack.ScopeChapterReview}
	store := NewRunStore()
	if err := store.Insert(run); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}
	service := NewService(
		&fakeSession{project: project.Project{Path: "/tmp/test"}, ok: true},
		&fakeLoader{registry: agent.Registry{}},
		&fakeSceneLoader{},
		&fakeAcceptor{},
		&fakeProvider{},
		nil,
		store,
		&fakeRunIDGenerator{next: "run_ffffffffffffffffffff"},
	)
	if _, err := service.Accept(context.Background(), run.RunID, "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); !errors.Is(err, ErrRunConflict) {
		t.Fatalf("Accept() error = %v, want ErrRunConflict", err)
	}
}

func newChapterReviewTestService(t *testing.T, source *chapterMaterialSource, provider *fakeProvider) *Service {
	t.Helper()
	return NewService(
		&fakeSession{project: project.Project{Path: "/tmp/test"}, ok: true},
		&fakeLoader{registry: agent.Registry{Agents: []agent.Agent{testChapterReviewAgent()}, Styles: []agent.Style{testPreciseEditorStyle()}}},
		&fakeSceneLoader{},
		&fakeAcceptor{},
		provider,
		nil,
		NewRunStore(),
		&fakeRunIDGenerator{next: "run_0123456789abcdef0123"},
	).WithMaterialSource(source).WithContextBuilder(contextpack.NewBuilder()).
		WithInvitationStore(NewInvitationStore(100)).
		WithInvitationIDGenerator(&fakeInvitationIDGenerator{next: "invite_0123456789abcdef0123"})
}

type fakeInvitationIDGenerator struct {
	next string
}

func (g *fakeInvitationIDGenerator) Next() (string, error) {
	return g.next, nil
}
