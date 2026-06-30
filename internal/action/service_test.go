package action

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"storywork/internal/agent"
	"storywork/internal/project"
	"storywork/internal/story"
)

type fakeSession struct {
	project project.Project
	ok      bool
}

func (s *fakeSession) Current() (project.Project, bool) {
	return s.project, s.ok
}

type fakeLoader struct {
	registry agent.Registry
	err      error
}

func (l *fakeLoader) Load(string) (agent.Registry, error) {
	return l.registry, l.err
}

type fakeSceneLoader struct {
	scene story.SceneDocument
	err   error
	calls int
}

func (l *fakeSceneLoader) LoadScene(context.Context, string) (story.SceneDocument, error) {
	l.calls++
	return l.scene, l.err
}

type fakeAcceptor struct {
	scene   story.SceneDocument
	err     error
	request story.AcceptScenePatchRequest
	calls   int
}

func (a *fakeAcceptor) AcceptScenePatch(_ context.Context, request story.AcceptScenePatchRequest) (story.SceneDocument, error) {
	a.calls++
	a.request = request
	return a.scene, a.err
}

type fakeProvider struct {
	response agent.GenerateResponse
	err      error
	request  agent.GenerateRequest
	calls    int
}

func (p *fakeProvider) Generate(_ context.Context, request agent.GenerateRequest) (agent.GenerateResponse, error) {
	p.calls++
	p.request = request
	return p.response, p.err
}

type fakeRunIDGenerator struct {
	next string
	err  error
}

func (g *fakeRunIDGenerator) Next() (string, error) {
	return g.next, g.err
}

// BDD trace:
//   - Requirements: M4-R04, M4-R05, M4-R06, M4-R07, M4-R08, M4-R09, M4-R10, M4-R11, M4-R12, M4-R15.
//   - Scenario: 4.2.1, 4.3.2, 4.3.3, 4.4.1.
//   - Test purpose: verify run and reject use the canonical scene selection,
//     minimal provider-neutral context, transient state only, and no mutation on
//     stale or conflicting requests.
func TestServiceRunRejectAndAcceptFlow(t *testing.T) {
	t.Parallel()

	linePolish := agent.Agent{
		Version:     1,
		ID:          "line_polish",
		Name:        "Line Polish",
		Description: "Rewrite selected prose.",
		AppliesWhen: agent.ApplicabilityRule{Surfaces: []agent.Surface{agent.SurfaceEditor}, InputScopes: []agent.InputScope{agent.InputScopeSelection}, MinWords: 2, MaxWords: 1500},
		ContextPolicy: agent.ContextPolicy{
			Required:  []agent.ContextPack{agent.ContextSelectedText, agent.ContextStyleSheet},
			Optional:  []agent.ContextPack{agent.ContextSurrounding},
			Forbidden: []agent.ContextPack{agent.ContextGlobalCodexRAG, agent.ContextRawImportNotes},
		},
		RAGPolicy: agent.RAGPolicy{Mode: agent.RAGModeNone},
		Control:   agent.Control{OutputMode: agent.OutputModePatch, RequiresAcceptance: true},
		Output:    agent.Output{Type: agent.OutputTypeReplacementText, RequiresDiffPreview: true},
	}
	style := agent.Style{
		Version:           1,
		ID:                "precise_editor",
		Name:              "Precise Editor",
		ProviderProfileID: "mock_default",
		Model:             "mock",
		Temperature:       0.2,
		SystemPrompt:      "Prompt",
	}
	scene := story.SceneDocument{
		ID:          "scn_0123456789abcdef0123",
		ChapterID:   "ch_0123456789abcdef0123",
		Title:       "The Duel",
		Markdown:    "Alpha beta gamma delta.\n",
		Revision:    "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Canonical:   []byte("scene"),
		FrontMatter: story.SceneFrontMatter{Status: "draft"},
	}
	startByte := 0
	endByte := len([]byte("Alpha beta"))
	provider := &fakeProvider{response: agent.GenerateResponse{Replacement: "Mock polished: Alpha beta"}}
	acceptor := &fakeAcceptor{scene: story.SceneDocument{
		ID:          scene.ID,
		ChapterID:   scene.ChapterID,
		Title:       scene.Title,
		FrontMatter: scene.FrontMatter,
		Markdown:    "Mock polished: Alpha beta gamma delta.\n",
		Revision:    "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}}
	service := NewService(
		&fakeSession{project: project.Project{Path: "/tmp/test-project"}, ok: true},
		&fakeLoader{registry: agent.Registry{Agents: []agent.Agent{linePolish}, Styles: []agent.Style{style}}},
		&fakeSceneLoader{scene: scene},
		acceptor,
		provider,
		NewRunStore(func() time.Time { return time.Date(2026, time.June, 29, 12, 0, 0, 0, time.UTC) }),
		&fakeRunIDGenerator{next: "run_0123456789abcdef0123"},
	)

	actions, err := service.AvailableActions(context.Background(), agent.AvailabilityInput{
		Surface:        agent.SurfaceEditor,
		InputScope:     agent.InputScopeSelection,
		SceneID:        scene.ID,
		SelectionWords: 2,
	})
	if err != nil {
		t.Fatalf("AvailableActions() error = %v", err)
	}
	if len(actions) != 1 || actions[0].AgentID != "line_polish" || len(actions[0].StyleIDs) != 1 || actions[0].StyleIDs[0] != "precise_editor" {
		t.Fatalf("available actions = %#v", actions)
	}

	run, err := service.Run(context.Background(), RunRequest{
		AgentID:       "line_polish",
		StyleID:       "precise_editor",
		Surface:       agent.SurfaceEditor,
		InputScope:    agent.InputScopeSelection,
		SceneID:       scene.ID,
		SceneRevision: scene.Revision,
		Selection: Selection{
			StartByte: startByte,
			EndByte:   endByte,
			Text:      "Alpha beta",
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if run.Status != RunPending || run.Replacement != "Mock polished: Alpha beta" || run.OriginalText != "Alpha beta" {
		t.Fatalf("run = %#v", run)
	}
	if provider.calls != 1 || provider.request.Packet.SelectedText != "Alpha beta" {
		t.Fatalf("provider request = %#v", provider.request)
	}
	if got := provider.request.Summary.PacksUsed; len(got) != 2 || got[0] != agent.ContextSelectedText || got[1] != agent.ContextStyleSheet {
		t.Fatalf("provider summary = %#v", provider.request.Summary)
	}

	rejected, err := service.Reject(context.Background(), run.RunID)
	if err != nil {
		t.Fatalf("Reject() error = %v", err)
	}
	if rejected.Status != RunRejected {
		t.Fatalf("rejected status = %q, want rejected", rejected.Status)
	}
	if _, err := service.Reject(context.Background(), run.RunID); !errors.Is(err, ErrRunConflict) {
		t.Fatalf("Reject(again) error = %v, want ErrRunConflict", err)
	}

	secondRunStore := NewRunStore(func() time.Time { return time.Date(2026, time.June, 29, 12, 0, 1, 0, time.UTC) })
	service = NewService(
		&fakeSession{project: project.Project{Path: "/tmp/test-project"}, ok: true},
		&fakeLoader{registry: agent.Registry{Agents: []agent.Agent{linePolish}, Styles: []agent.Style{style}}},
		&fakeSceneLoader{scene: scene},
		acceptor,
		provider,
		secondRunStore,
		&fakeRunIDGenerator{next: "run_ffffffffffffffffffff"},
	)
	run, err = service.Run(context.Background(), RunRequest{
		AgentID:       "line_polish",
		StyleID:       "precise_editor",
		Surface:       agent.SurfaceEditor,
		InputScope:    agent.InputScopeSelection,
		SceneID:       scene.ID,
		SceneRevision: scene.Revision,
		Selection:     Selection{StartByte: startByte, EndByte: endByte, Text: "Alpha beta"},
	})
	if err != nil {
		t.Fatalf("Run(second) error = %v", err)
	}
	accepted, savedScene, err := service.Accept(context.Background(), run.RunID, scene.Revision)
	if err != nil {
		t.Fatalf("Accept() error = %v", err)
	}
	if accepted.Status != RunAccepted {
		t.Fatalf("accepted status = %q, want accepted", accepted.Status)
	}
	if savedScene.Revision != acceptor.scene.Revision {
		t.Fatalf("saved scene = %#v", savedScene)
	}
	if acceptor.calls != 1 || acceptor.request.OriginalText != "Alpha beta" || acceptor.request.ReplacementText != "Mock polished: Alpha beta" {
		t.Fatalf("acceptor request = %#v", acceptor.request)
	}
}

// BDD trace:
//   - Requirements: M4-R09, M4-R10, M4-R11, M4-R15.
//   - Scenario: 4.3.3, 4.4.3.
//   - Test purpose: verify stale revisions, selection mismatches, and failed
//     accept attempts do not create persistent mutation and release the claimed
//     run back to pending for retry.
func TestServiceRejectsStaleSelectionsAndReleasesFailedAcceptClaims(t *testing.T) {
	t.Parallel()

	linePolish := agent.Agent{
		Version:     1,
		ID:          "line_polish",
		Name:        "Line Polish",
		Description: "Rewrite selected prose.",
		AppliesWhen: agent.ApplicabilityRule{Surfaces: []agent.Surface{agent.SurfaceEditor}, InputScopes: []agent.InputScope{agent.InputScopeSelection}, MinWords: 1, MaxWords: 1500},
		ContextPolicy: agent.ContextPolicy{
			Required:  []agent.ContextPack{agent.ContextSelectedText, agent.ContextStyleSheet},
			Optional:  []agent.ContextPack{agent.ContextSurrounding},
			Forbidden: []agent.ContextPack{agent.ContextGlobalCodexRAG, agent.ContextRawImportNotes},
		},
		RAGPolicy: agent.RAGPolicy{Mode: agent.RAGModeNone},
		Control:   agent.Control{OutputMode: agent.OutputModePatch, RequiresAcceptance: true},
		Output:    agent.Output{Type: agent.OutputTypeReplacementText, RequiresDiffPreview: true},
	}
	style := agent.Style{
		Version:           1,
		ID:                "precise_editor",
		Name:              "Precise Editor",
		ProviderProfileID: "mock_default",
		Model:             "mock",
		Temperature:       0.2,
		SystemPrompt:      "Prompt",
	}
	scene := story.SceneDocument{
		ID:          "scn_0123456789abcdef0123",
		ChapterID:   "ch_0123456789abcdef0123",
		Title:       "The Duel",
		Markdown:    "Alpha beta gamma.\n",
		Revision:    "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Canonical:   []byte("scene"),
		FrontMatter: story.SceneFrontMatter{Status: "draft"},
	}
	provider := &fakeProvider{response: agent.GenerateResponse{Replacement: "Mock polished: Alpha"}}
	acceptor := &fakeAcceptor{err: story.ErrDirtyWorktree}
	service := NewService(
		&fakeSession{project: project.Project{Path: "/tmp/test-project"}, ok: true},
		&fakeLoader{registry: agent.Registry{Agents: []agent.Agent{linePolish}, Styles: []agent.Style{style}}},
		&fakeSceneLoader{scene: scene},
		acceptor,
		provider,
		NewRunStore(func() time.Time { return time.Now() }),
		&fakeRunIDGenerator{next: "run_0123456789abcdef0123"},
	)

	if _, err := service.Run(context.Background(), RunRequest{
		AgentID:       "line_polish",
		StyleID:       "precise_editor",
		Surface:       agent.SurfaceEditor,
		InputScope:    agent.InputScopeSelection,
		SceneID:       scene.ID,
		SceneRevision: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Selection:     Selection{StartByte: 0, EndByte: len([]byte("Alpha")), Text: "Alpha"},
	}); !errors.Is(err, story.ErrStaleRevision) {
		t.Fatalf("Run(stale revision) error = %v, want ErrStaleRevision", err)
	}

	run, err := service.Run(context.Background(), RunRequest{
		AgentID:       "line_polish",
		StyleID:       "precise_editor",
		Surface:       agent.SurfaceEditor,
		InputScope:    agent.InputScopeSelection,
		SceneID:       scene.ID,
		SceneRevision: scene.Revision,
		Selection:     Selection{StartByte: 0, EndByte: len([]byte("Alpha")), Text: "Wrong"},
	})
	if err == nil || !errors.Is(err, story.ErrInvalidSelection) {
		t.Fatalf("Run(selection mismatch) error = %v, want ErrInvalidSelection", err)
	}

	run, err = service.Run(context.Background(), RunRequest{
		AgentID:       "line_polish",
		StyleID:       "precise_editor",
		Surface:       agent.SurfaceEditor,
		InputScope:    agent.InputScopeSelection,
		SceneID:       scene.ID,
		SceneRevision: scene.Revision,
		Selection:     Selection{StartByte: 0, EndByte: len([]byte("Alpha")), Text: "Alpha"},
	})
	if err != nil {
		t.Fatalf("Run(valid) error = %v", err)
	}
	if _, _, err := service.Accept(context.Background(), run.RunID, scene.Revision); !errors.Is(err, story.ErrDirtyWorktree) {
		t.Fatalf("Accept(dirty) error = %v, want ErrDirtyWorktree", err)
	}
	if rejected, err := service.Reject(context.Background(), run.RunID); err != nil || rejected.Status != RunRejected {
		t.Fatalf("Reject(after failed accept) = %#v, %v", rejected, err)
	}
}

// BDD trace:
//   - Requirements: M4-R10, M4-R15.
//   - Scenario: 4.4.1.
//   - Test purpose: verify the transient run store evicts only terminal runs and
//     rejects insertion when capacity is saturated exclusively by live runs.
func TestRunStoreEvictsTerminalRunsAndRejectsLiveCapacity(t *testing.T) {
	t.Parallel()

	store := NewRunStore(func() time.Time { return time.Now() })
	for i := 0; i < 1000; i++ {
		run := Run{RunID: formatRunID(i), Status: RunRejected}
		if err := store.Insert(run); err != nil {
			t.Fatalf("Insert(terminal %d) error = %v", i, err)
		}
	}
	if err := store.Insert(Run{RunID: "run_ffffffffffffffffffff", Status: RunPending}); err != nil {
		t.Fatalf("Insert(after evicting terminal) error = %v", err)
	}

	liveStore := NewRunStore(func() time.Time { return time.Now() })
	for i := 0; i < 1000; i++ {
		if err := liveStore.Insert(Run{RunID: formatRunID(i), Status: RunPending}); err != nil {
			t.Fatalf("Insert(live %d) error = %v", i, err)
		}
	}
	if err := liveStore.Insert(Run{RunID: "run_ffffffffffffffffffff", Status: RunPending}); !errors.Is(err, ErrRunCapacity) {
		t.Fatalf("Insert(live capacity) error = %v, want ErrRunCapacity", err)
	}
}

func formatRunID(i int) string {
	return fmt.Sprintf("run_%020x", i)
}
