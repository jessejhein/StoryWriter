package action

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"storywork/internal/agent"
	"storywork/internal/contextpack"
	"storywork/internal/project"
	"storywork/internal/provider"
	"storywork/internal/story"
)

// Test: serialized action runs contain provider identity but cannot disclose a credential.
// Requirements: M5-R04, M5-R11, M5-R12.
func TestRunJSONExcludesProviderCredentialValues(t *testing.T) {
	t.Parallel()

	run := Run{
		RunID: "run_00000000000000000001", Status: RunPending,
		Provider: agent.ProviderIdentity{ProfileID: "hosted", Type: provider.TypeOpenAICompatible, Model: "model"},
	}
	encoded, err := json.Marshal(run)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if strings.Contains(string(encoded), "sentinel-action-run-credential") {
		t.Fatalf("serialized run leaked credential: %s", encoded)
	}
	if !strings.Contains(string(encoded), `"profile_id":"hosted"`) {
		t.Fatalf("serialized run omitted provider identity: %s", encoded)
	}
}

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
	mu           sync.Mutex
	scene        story.SceneDocument
	err          error
	request      story.AcceptScenePatchRequest
	calls        int
	started      chan struct{}
	release      chan struct{}
	beforeReturn func(story.AcceptScenePatchRequest)
}

func (a *fakeAcceptor) AcceptScenePatch(_ context.Context, request story.AcceptScenePatchRequest) (story.SceneDocument, error) {
	a.mu.Lock()
	a.calls++
	a.request = request
	started := a.started
	release := a.release
	beforeReturn := a.beforeReturn
	scene := a.scene
	err := a.err
	a.mu.Unlock()
	if started != nil {
		started <- struct{}{}
	}
	if release != nil {
		<-release
	}
	if beforeReturn != nil {
		beforeReturn(request)
	}
	return scene, err
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
	next  string
	err   error
	calls int
}

type fakeProfileResolver struct {
	resolved  provider.ResolvedProfile
	found     bool
	err       error
	byID      map[string]provider.ResolvedProfile
	foundByID map[string]bool
}

func (r *fakeProfileResolver) Resolve(_ context.Context, profileID string) (provider.ResolvedProfile, bool, error) {
	if r.err != nil {
		return provider.ResolvedProfile{}, false, r.err
	}
	if r.byID != nil {
		resolved, ok := r.byID[profileID]
		if !ok {
			return provider.ResolvedProfile{}, r.foundByID[profileID], nil
		}
		found := true
		if r.foundByID != nil {
			found = r.foundByID[profileID]
		}
		return resolved, found, nil
	}
	return r.resolved, r.found, r.err
}

func (g *fakeRunIDGenerator) Next() (string, error) {
	g.calls++
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
	service := newActionTestService(scene, provider, acceptor, NewRunStore(), &fakeRunIDGenerator{next: "run_0123456789abcdef0123"})

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
	if provider.calls != 1 {
		t.Fatalf("provider calls = %d, want 1", provider.calls)
	}
	selectionPacket, ok := provider.request.TypedPacket.(contextpack.SelectionPacket)
	if !ok || selectionPacket.SelectedText != "Alpha beta" {
		t.Fatalf("provider typed packet = %#v", provider.request.TypedPacket)
	}
	if !manifestHasPacks(run.Manifest.PacksUsed, contextpack.PackSelectedText, contextpack.PackStyleSheet) {
		t.Fatalf("run manifest packs = %#v", run.Manifest.PacksUsed)
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

	secondRunStore := NewRunStore()
	service = newActionTestService(scene, provider, acceptor, secondRunStore, &fakeRunIDGenerator{next: "run_ffffffffffffffffffff"})
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
	acceptResult, err := service.Accept(context.Background(), run.RunID, scene.Revision)
	if err != nil {
		t.Fatalf("Accept() error = %v", err)
	}
	accepted := acceptResult.Run
	savedScene := acceptResult.Scene
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
//   - Requirements: M5-R10, M5-R11.
//   - Scenario: 5.5.2.
//   - Test purpose: verify invalid provider output is rejected before any
//     transient run is inserted or project mutation path is reached.
func TestServiceRejectsInvalidProviderOutputBeforeRunInsertion(t *testing.T) {
	t.Parallel()

	linePolish := agent.Agent{
		Version:     2,
		ID:          "line_polish",
		Name:        "Line Polish",
		Description: "Rewrite selected prose.",
		AppliesWhen: agent.ApplicabilityRule{Surfaces: []agent.Surface{agent.SurfaceEditor}, InputScopes: []agent.InputScope{agent.InputScopeSelection}, MinWords: 2, MaxWords: 1500},
		ModelRequirements: agent.ModelRequirements{
			MinContextTokens: 2048,
		},
		ContextPolicy: agent.ContextPolicy{
			Required:  []agent.ContextPack{agent.ContextSelectedText, agent.ContextStyleSheet},
			Optional:  []agent.ContextPack{agent.ContextSurrounding},
			Forbidden: []agent.ContextPack{agent.ContextGlobalCodexRAG, agent.ContextRawImportNotes},
		},
		RAGPolicy: agent.RAGPolicy{Mode: agent.RAGModeNone},
		Control:   agent.Control{OutputMode: agent.OutputModePatch, RequiresAcceptance: true},
		Output:    agent.Output{Type: agent.OutputTypeReplacementText, RequiresDiffPreview: true},
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
	service := NewService(
		&fakeSession{project: project.Project{Path: "/tmp/test-project"}, ok: true},
		&fakeLoader{registry: agent.Registry{Agents: []agent.Agent{linePolish}, Styles: []agent.Style{testPreciseEditorStyle()}}},
		&fakeSceneLoader{scene: scene},
		&fakeAcceptor{},
		&fakeProvider{response: agent.GenerateResponse{Replacement: " \t"}},
		nil,
		NewRunStore(),
		&fakeRunIDGenerator{next: "run_0123456789abcdef0123"},
	).WithMaterialSource(selectionMaterialSource(scene, "Alpha beta")).WithContextBuilder(contextpack.NewBuilder())

	_, err := service.Run(context.Background(), RunRequest{
		AgentID:       "line_polish",
		StyleID:       "precise_editor",
		Surface:       agent.SurfaceEditor,
		InputScope:    agent.InputScopeSelection,
		SceneID:       scene.ID,
		SceneRevision: scene.Revision,
		Selection:     Selection{StartByte: 0, EndByte: len([]byte("Alpha beta")), Text: "Alpha beta"},
	})
	if !errors.Is(err, ErrProviderRejected) {
		t.Fatalf("Run(invalid output) error = %v, want ErrProviderRejected", err)
	}
}

// BDD trace:
//   - Requirements: M5-R08, M5-R09.
//   - Scenarios: 5.2.1, 5.2.2, 5.2.3.
//   - Test purpose: verify available actions expose only compatible real styles
//     and the run path rechecks credential readiness immediately before
//     provider execution.
func TestServiceFiltersRealProviderStylesAndRechecksRunTimeReadiness(t *testing.T) {
	t.Parallel()

	linePolish := agent.Agent{
		Version:     2,
		ID:          "line_polish",
		Name:        "Line Polish",
		Description: "Rewrite selected prose.",
		AppliesWhen: agent.ApplicabilityRule{Surfaces: []agent.Surface{agent.SurfaceEditor}, InputScopes: []agent.InputScope{agent.InputScopeSelection}, MinWords: 2, MaxWords: 1500},
		ModelRequirements: agent.ModelRequirements{
			MinContextTokens: 2048,
		},
		ContextPolicy: agent.ContextPolicy{
			Required:  []agent.ContextPack{agent.ContextSelectedText, agent.ContextStyleSheet},
			Optional:  []agent.ContextPack{agent.ContextSurrounding},
			Forbidden: []agent.ContextPack{agent.ContextGlobalCodexRAG, agent.ContextRawImportNotes},
		},
		RAGPolicy: agent.RAGPolicy{Mode: agent.RAGModeNone},
		Control:   agent.Control{OutputMode: agent.OutputModePatch, RequiresAcceptance: true},
		Output:    agent.Output{Type: agent.OutputTypeReplacementText, RequiresDiffPreview: true},
	}
	mockStyle := agent.Style{
		Version:           1,
		ID:                "precise_editor",
		Name:              "Precise Editor",
		ProviderProfileID: "mock_default",
		Model:             "mock",
		Temperature:       0.2,
		SystemPrompt:      "Prompt",
	}
	realStyle := agent.Style{
		Version:           2,
		ID:                "local_precise_editor",
		Name:              "Local Precise Editor",
		ProviderProfileID: "local_openai",
		Model:             "local-model-name",
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
	resolver := &fakeProfileResolver{
		resolved: provider.ResolvedProfile{
			Profile: provider.Profile{
				ID:      "local_openai",
				Type:    provider.TypeOpenAICompatible,
				BaseURL: "http://127.0.0.1:1234/v1",
				Auth: provider.AuthConfig{
					Type: provider.AuthTypeNone,
				},
				Capabilities: provider.Capabilities{
					Chat:             true,
					MaxContextTokens: 8192,
				},
			},
		},
		found: true,
	}
	service := NewService(
		&fakeSession{project: project.Project{Path: "/tmp/test-project"}, ok: true},
		&fakeLoader{registry: agent.Registry{Agents: []agent.Agent{linePolish}, Styles: []agent.Style{realStyle, mockStyle}}},
		&fakeSceneLoader{scene: scene},
		&fakeAcceptor{},
		&fakeProvider{response: agent.GenerateResponse{Replacement: "Refined text"}},
		resolver,
		NewRunStore(),
		&fakeRunIDGenerator{next: "run_0123456789abcdef0123"},
	).WithMaterialSource(selectionMaterialSource(scene, "Alpha beta")).WithContextBuilder(contextpack.NewBuilder())

	actions, err := service.AvailableActions(context.Background(), agent.AvailabilityInput{
		Surface:        agent.SurfaceEditor,
		InputScope:     agent.InputScopeSelection,
		SceneID:        scene.ID,
		SelectionWords: 2,
	})
	if err != nil {
		t.Fatalf("AvailableActions() error = %v", err)
	}
	if len(actions) != 1 || !slices.Equal(actions[0].StyleIDs, []string{"local_precise_editor", "precise_editor"}) {
		t.Fatalf("available actions = %#v", actions)
	}

	resolver.resolved.Profile.Auth.Type = provider.AuthTypeBearerEnv
	resolver.resolved.Readiness = provider.ReadinessMissingCredential
	_, err = service.Run(context.Background(), RunRequest{
		AgentID:       "line_polish",
		StyleID:       "local_precise_editor",
		Surface:       agent.SurfaceEditor,
		InputScope:    agent.InputScopeSelection,
		SceneID:       scene.ID,
		SceneRevision: scene.Revision,
		Selection:     Selection{StartByte: 0, EndByte: len([]byte("Alpha beta")), Text: "Alpha beta"},
	})
	if !errors.Is(err, ErrProviderInvalid) {
		t.Fatalf("Run(missing credential) error = %v, want ErrProviderInvalid", err)
	}
}

// BDD trace:
//   - Requirements: M5-R08, M5-R09.
//   - Scenario: 5.2.1, 5.2.2.
//   - Test purpose: verify compatible styles are listed in style-name then
//     style-ID order regardless of registry order, while incompatible styles are
//     excluded and action ordering remains agent-name then ID.
func TestAvailableActionsSortsCompatibleStylesByNameThenID(t *testing.T) {
	t.Parallel()

	linePolish := agent.Agent{
		Version:           2,
		ID:                "line_polish",
		Name:              "Line Polish",
		Description:       "Rewrite selected prose.",
		AppliesWhen:       agent.ApplicabilityRule{Surfaces: []agent.Surface{agent.SurfaceEditor}, InputScopes: []agent.InputScope{agent.InputScopeSelection}, MinWords: 2, MaxWords: 1500},
		ModelRequirements: agent.ModelRequirements{MinContextTokens: 2048},
		ContextPolicy:     agent.ContextPolicy{Required: []agent.ContextPack{agent.ContextSelectedText, agent.ContextStyleSheet}},
		RAGPolicy:         agent.RAGPolicy{Mode: agent.RAGModeNone},
		Control:           agent.Control{OutputMode: agent.OutputModePatch, RequiresAcceptance: true},
		Output:            agent.Output{Type: agent.OutputTypeReplacementText, RequiresDiffPreview: true},
	}
	alphaAgent := linePolish
	alphaAgent.ID = "alpha_polish"
	alphaAgent.Name = "Alpha Polish"

	styles := []agent.Style{
		{Version: 2, ID: "zeta_id", Name: "Zeta Voice", ProviderProfileID: "ready_profile", Model: "model", Temperature: 0.2, SystemPrompt: "Prompt"},
		{Version: 2, ID: "alpha_b", Name: "Alpha Voice", ProviderProfileID: "ready_profile", Model: "model", Temperature: 0.2, SystemPrompt: "Prompt"},
		{Version: 2, ID: "alpha_a", Name: "Alpha Voice", ProviderProfileID: "ready_profile", Model: "model", Temperature: 0.2, SystemPrompt: "Prompt"},
		{Version: 2, ID: "first_but_incompatible", Name: "Aardvark Voice", ProviderProfileID: "small_context", Model: "model", Temperature: 0.2, SystemPrompt: "Prompt"},
	}
	scene := testActionScene()
	resolver := &fakeProfileResolver{
		byID: map[string]provider.ResolvedProfile{
			"ready_profile": {
				Profile: provider.Profile{
					ID:      "ready_profile",
					Type:    provider.TypeOpenAICompatible,
					BaseURL: "http://127.0.0.1:1234/v1",
					Auth:    provider.AuthConfig{Type: provider.AuthTypeNone},
					Capabilities: provider.Capabilities{
						Chat:             true,
						MaxContextTokens: 8192,
					},
					Readiness: provider.ReadinessReady,
				},
			},
			"small_context": {
				Profile: provider.Profile{
					ID:      "small_context",
					Type:    provider.TypeOpenAICompatible,
					BaseURL: "http://127.0.0.1:1234/v1",
					Auth:    provider.AuthConfig{Type: provider.AuthTypeNone},
					Capabilities: provider.Capabilities{
						Chat:             true,
						MaxContextTokens: 32,
					},
					Readiness: provider.ReadinessReady,
				},
			},
		},
		foundByID: map[string]bool{
			"ready_profile": true,
			"small_context": true,
		},
	}
	service := NewService(
		&fakeSession{project: project.Project{Path: "/tmp/test-project"}, ok: true},
		&fakeLoader{registry: agent.Registry{Agents: []agent.Agent{linePolish, alphaAgent}, Styles: styles}},
		&fakeSceneLoader{scene: scene},
		&fakeAcceptor{},
		&fakeProvider{response: agent.GenerateResponse{Replacement: "Refined text"}},
		resolver,
		NewRunStore(),
		&fakeRunIDGenerator{next: "run_0123456789abcdef0123"},
	).WithMaterialSource(selectionMaterialSource(scene, "Alpha beta")).WithContextBuilder(contextpack.NewBuilder())

	actions, err := service.AvailableActions(context.Background(), agent.AvailabilityInput{
		Surface:        agent.SurfaceEditor,
		InputScope:     agent.InputScopeSelection,
		SceneID:        scene.ID,
		SelectionWords: 2,
	})
	if err != nil {
		t.Fatalf("AvailableActions() error = %v", err)
	}
	if got := []string{actions[0].AgentID, actions[1].AgentID}; !slices.Equal(got, []string{"alpha_polish", "line_polish"}) {
		t.Fatalf("action ordering = %v, want [alpha_polish line_polish]", got)
	}
	for _, actionDefinition := range actions {
		if !slices.Equal(actionDefinition.StyleIDs, []string{"alpha_a", "alpha_b", "zeta_id"}) {
			t.Fatalf("style ordering = %v, want [alpha_a alpha_b zeta_id]", actionDefinition.StyleIDs)
		}
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

	linePolish := testLinePolishAgent()
	linePolish.AppliesWhen.MinWords = 1
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
		&fakeLoader{registry: agent.Registry{Agents: []agent.Agent{linePolish}, Styles: []agent.Style{testPreciseEditorStyle()}}},
		&fakeSceneLoader{scene: scene},
		acceptor,
		provider,
		nil,
		NewRunStore(),
		&fakeRunIDGenerator{next: "run_0123456789abcdef0123"},
	).WithMaterialSource(selectionMaterialSource(scene, "Alpha")).WithContextBuilder(contextpack.NewBuilder())

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
	if _, err := service.Accept(context.Background(), run.RunID, scene.Revision); !errors.Is(err, story.ErrDirtyWorktree) {
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

	store := NewRunStore()
	for i := 0; i < 1000; i++ {
		run := Run{RunID: formatRunID(i), Status: RunRejected}
		if err := store.Insert(run); err != nil {
			t.Fatalf("Insert(terminal %d) error = %v", i, err)
		}
	}
	if err := store.Insert(Run{RunID: "run_ffffffffffffffffffff", Status: RunPending}); err != nil {
		t.Fatalf("Insert(after evicting terminal) error = %v", err)
	}

	liveStore := NewRunStore()
	for i := 0; i < 1000; i++ {
		if err := liveStore.Insert(Run{RunID: formatRunID(i), Status: RunPending}); err != nil {
			t.Fatalf("Insert(live %d) error = %v", i, err)
		}
	}
	if err := liveStore.Insert(Run{RunID: "run_ffffffffffffffffffff", Status: RunPending}); !errors.Is(err, ErrRunCapacity) {
		t.Fatalf("Insert(live capacity) error = %v, want ErrRunCapacity", err)
	}
}

// BDD trace:
//   - Requirements: M4-R10, M4-R15.
//   - Scenario: 4.4.1, 4.4.2.
//   - Test purpose: verify a colliding generated ID cannot replace an existing
//     pending review run in transient memory.
func TestRunStoreRejectsDuplicateRunIDs(t *testing.T) {
	t.Parallel()

	store := NewRunStore()
	runID := "run_0123456789abcdef0123"
	if err := store.Insert(Run{RunID: runID, Status: RunPending, OriginalText: "first"}); err != nil {
		t.Fatalf("Insert(first) error = %v", err)
	}
	if err := store.Insert(Run{RunID: runID, Status: RunPending, OriginalText: "replacement"}); !errors.Is(err, ErrDuplicateRunID) {
		t.Fatalf("Insert(duplicate) error = %v, want ErrDuplicateRunID", err)
	}
	claimed, err := store.ClaimAccepting(runID)
	if err != nil {
		t.Fatalf("ClaimAccepting() error = %v", err)
	}
	if claimed.OriginalText != "first" {
		t.Fatalf("stored original = %q, want first", claimed.OriginalText)
	}
}

// BDD trace:
//   - Requirement: M4-R10 transient run storage remains deterministic.
//   - Scenario: removing an evicted run ID preserves insertion order without
//     mutating the caller's order snapshot.
//   - Test purpose: prevent subtle aliasing from in-place slice filtering.
func TestRemoveIDPreservesOrderAndInput(t *testing.T) {
	ids := []string{"run_first", "run_remove", "run_last"}
	original := append([]string(nil), ids...)

	got := removeID(ids, "run_remove")

	if want := []string{"run_first", "run_last"}; !slices.Equal(got, want) {
		t.Fatalf("removeID() = %v, want %v", got, want)
	}
	if !slices.Equal(ids, original) {
		t.Fatalf("removeID() mutated input to %v, want %v", ids, original)
	}
}

// BDD trace:
//   - Requirements: M4-R10, M4-R15.
//   - Scenario: 4.3.2, 4.4.2.
//   - Test purpose: verify concurrent accepts cannot both claim the same
//     transient run or invoke canonical mutation twice.
func TestConcurrentAcceptsAllowExactlyOneRunClaim(t *testing.T) {
	t.Parallel()

	service, scene, acceptor, _ := newConcurrentActionTestService(t, "Mock polished: Alpha beta", "run_0123456789abcdef0123")
	run, err := service.Run(context.Background(), selectionRunRequest(scene))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	firstResult := make(chan acceptResult, 1)
	secondResult := make(chan acceptResult, 1)
	go func() {
		result, err := service.Accept(context.Background(), run.RunID, scene.Revision)
		firstResult <- acceptResult{run: result.Run, scene: result.Scene, err: err}
	}()

	waitForAcceptorStart(t, acceptor.started)

	go func() {
		result, err := service.Accept(context.Background(), run.RunID, scene.Revision)
		secondResult <- acceptResult{run: result.Run, scene: result.Scene, err: err}
	}()

	second := <-secondResult
	if !errors.Is(second.err, ErrRunConflict) {
		t.Fatalf("second Accept() error = %v, want ErrRunConflict", second.err)
	}
	if calls := acceptorCalls(acceptor); calls != 1 {
		t.Fatalf("acceptor calls = %d, want 1", calls)
	}

	close(acceptor.release)
	first := <-firstResult
	if first.err != nil {
		t.Fatalf("first Accept() error = %v", first.err)
	}
	if first.run.Status != RunAccepted {
		t.Fatalf("accepted run status = %q, want %q", first.run.Status, RunAccepted)
	}
	if first.scene.Revision != acceptor.scene.Revision {
		t.Fatalf("accepted scene revision = %q, want %q", first.scene.Revision, acceptor.scene.Revision)
	}
	if calls := acceptorCalls(acceptor); calls != 1 {
		t.Fatalf("final acceptor calls = %d, want 1", calls)
	}
}

// BDD trace:
//   - Requirements: M4-R10, M4-R12, M4-R15.
//   - Scenario: 4.4.1, 4.4.2, 4.4.3.
//   - Test purpose: verify accept/reject races permit exactly one terminal
//     decision and invoke canonical mutation only when accept wins.
func TestConcurrentAcceptAndRejectAllowExactlyOneTerminalDecision(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name              string
		acceptClaimsFirst bool
		wantAcceptorCalls int
		wantWinnerStatus  RunStatus
		wantLoserErr      error
	}{
		{
			name:              "accept claims first",
			acceptClaimsFirst: true,
			wantAcceptorCalls: 1,
			wantWinnerStatus:  RunAccepted,
			wantLoserErr:      ErrRunConflict,
		},
		{
			name:              "reject transitions first",
			acceptClaimsFirst: false,
			wantAcceptorCalls: 0,
			wantWinnerStatus:  RunRejected,
			wantLoserErr:      ErrRunConflict,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			service, scene, acceptor, _ := newConcurrentActionTestService(t, "Mock polished: Alpha beta", "run_0123456789abcdef0123")
			run, err := service.Run(context.Background(), selectionRunRequest(scene))
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}

			acceptResultCh := make(chan acceptResult, 1)
			rejectResultCh := make(chan rejectResult, 1)

			if tc.acceptClaimsFirst {
				go func() {
					result, err := service.Accept(context.Background(), run.RunID, scene.Revision)
					acceptResultCh <- acceptResult{run: result.Run, scene: result.Scene, err: err}
				}()
				waitForAcceptorStart(t, acceptor.started)
				go func() {
					run, err := service.Reject(context.Background(), run.RunID)
					rejectResultCh <- rejectResult{run: run, err: err}
				}()

				reject := <-rejectResultCh
				if !errors.Is(reject.err, tc.wantLoserErr) {
					t.Fatalf("Reject() error = %v, want %v", reject.err, tc.wantLoserErr)
				}
				close(acceptor.release)

				accept := <-acceptResultCh
				if accept.err != nil {
					t.Fatalf("Accept() error = %v", accept.err)
				}
				if accept.run.Status != tc.wantWinnerStatus {
					t.Fatalf("accepted run status = %q, want %q", accept.run.Status, tc.wantWinnerStatus)
				}
			} else {
				startAccept := make(chan struct{})
				go func() {
					<-startAccept
					result, err := service.Accept(context.Background(), run.RunID, scene.Revision)
					acceptResultCh <- acceptResult{run: result.Run, scene: result.Scene, err: err}
				}()
				go func() {
					run, err := service.Reject(context.Background(), run.RunID)
					rejectResultCh <- rejectResult{run: run, err: err}
				}()

				reject := <-rejectResultCh
				if reject.err != nil {
					t.Fatalf("Reject() error = %v", reject.err)
				}
				if reject.run.Status != tc.wantWinnerStatus {
					t.Fatalf("rejected run status = %q, want %q", reject.run.Status, tc.wantWinnerStatus)
				}
				close(startAccept)

				accept := <-acceptResultCh
				if !errors.Is(accept.err, tc.wantLoserErr) {
					t.Fatalf("Accept() error = %v, want %v", accept.err, tc.wantLoserErr)
				}
			}

			if calls := acceptorCalls(acceptor); calls != tc.wantAcceptorCalls {
				t.Fatalf("acceptor calls = %d, want %d", calls, tc.wantAcceptorCalls)
			}
		})
	}
}

// BDD trace:
//   - Requirements: M4-R08, M4-R10, M4-R11.
//   - Scenario: 4.3.2.
//   - Test purpose: verify byte-identical provider output is rejected before a
//     transient run ID is allocated or any accept/reject boundary can observe it.
func TestRunRejectsByteIdenticalProviderOutputWithoutStoringRun(t *testing.T) {
	t.Parallel()

	scene := testActionScene()
	provider := &fakeProvider{response: agent.GenerateResponse{Replacement: "Alpha beta"}}
	acceptor := &fakeAcceptor{}
	runStore := NewRunStore()
	ids := &fakeRunIDGenerator{next: "run_0123456789abcdef0123"}
	service := newActionTestService(scene, provider, acceptor, runStore, ids)

	_, err := service.Run(context.Background(), selectionRunRequest(scene))
	if !errors.Is(err, story.ErrNoSceneChanges) {
		t.Fatalf("Run() error = %v, want ErrNoSceneChanges", err)
	}
	if ids.calls != 0 {
		t.Fatalf("run ID generator calls = %d, want 0", ids.calls)
	}
	if provider.calls != 1 {
		t.Fatalf("provider calls = %d, want 1", provider.calls)
	}
	if acceptorCalls(acceptor) != 0 {
		t.Fatalf("acceptor calls = %d, want 0", acceptorCalls(acceptor))
	}
	if _, err := service.Reject(context.Background(), ids.next); !errors.Is(err, ErrRunNotFound) {
		t.Fatalf("Reject() error = %v, want ErrRunNotFound", err)
	}
	if _, err := service.Accept(context.Background(), ids.next, scene.Revision); !errors.Is(err, ErrRunNotFound) {
		t.Fatalf("Accept() error = %v, want ErrRunNotFound", err)
	}
}

type acceptResult struct {
	run   Run
	scene story.SceneDocument
	err   error
}

type rejectResult struct {
	run Run
	err error
}

func newConcurrentActionTestService(t *testing.T, replacement, runID string) (*Service, story.SceneDocument, *fakeAcceptor, *fakeProvider) {
	t.Helper()

	scene := testActionScene()
	acceptor := &fakeAcceptor{
		scene:   acceptedActionScene(scene, replacement),
		started: make(chan struct{}, 1),
		release: make(chan struct{}),
	}
	provider := &fakeProvider{response: agent.GenerateResponse{Replacement: replacement}}
	service := newActionTestService(
		scene,
		provider,
		acceptor,
		NewRunStore(),
		&fakeRunIDGenerator{next: runID},
	)
	return service, scene, acceptor, provider
}

func manifestHasPacks(packs []contextpack.Pack, required ...contextpack.Pack) bool {
	if len(packs) != len(required) {
		return false
	}
	seen := make(map[contextpack.Pack]struct{}, len(packs))
	for _, pack := range packs {
		seen[pack] = struct{}{}
	}
	for _, pack := range required {
		if _, ok := seen[pack]; !ok {
			return false
		}
	}
	return true
}

func selectionMaterialSource(scene story.SceneDocument, selectedText string) ContextMaterialSource {
	return &fakeMaterialSource{result: story.ContextMaterialResult{
		Material: contextpack.Material{
			Scope:         contextpack.ScopeSelection,
			SelectionText: selectedText,
		},
		TargetRevision: scene.Revision,
	}}
}

func newActionTestService(scene story.SceneDocument, provider *fakeProvider, acceptor *fakeAcceptor, runs *RunStore, ids *fakeRunIDGenerator) *Service {
	linePolish := testLinePolishAgent()
	style := testPreciseEditorStyle()
	return NewService(
		&fakeSession{project: project.Project{Path: "/tmp/test-project"}, ok: true},
		&fakeLoader{registry: agent.Registry{Agents: []agent.Agent{linePolish}, Styles: []agent.Style{style}}},
		&fakeSceneLoader{scene: scene},
		acceptor,
		provider,
		nil,
		runs,
		ids,
	).WithMaterialSource(selectionMaterialSource(scene, "Alpha beta")).WithContextBuilder(contextpack.NewBuilder())
}

func testLinePolishAgent() agent.Agent {
	return agent.Agent{
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
}

func testPreciseEditorStyle() agent.Style {
	return agent.Style{
		Version:           1,
		ID:                "precise_editor",
		Name:              "Precise Editor",
		ProviderProfileID: "mock_default",
		Model:             "mock",
		Temperature:       0.2,
		SystemPrompt:      "Prompt",
	}
}

func testActionScene() story.SceneDocument {
	return story.SceneDocument{
		ID:          "scn_0123456789abcdef0123",
		ChapterID:   "ch_0123456789abcdef0123",
		Title:       "The Duel",
		Markdown:    "Alpha beta gamma delta.\n",
		Revision:    "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Canonical:   []byte("scene"),
		FrontMatter: story.SceneFrontMatter{Status: "draft"},
	}
}

func acceptedActionScene(scene story.SceneDocument, replacement string) story.SceneDocument {
	scene.Markdown = replacement + " gamma delta.\n"
	scene.Revision = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	return scene
}

func selectionRunRequest(scene story.SceneDocument) RunRequest {
	return RunRequest{
		AgentID:       "line_polish",
		StyleID:       "precise_editor",
		Surface:       agent.SurfaceEditor,
		InputScope:    agent.InputScopeSelection,
		SceneID:       scene.ID,
		SceneRevision: scene.Revision,
		Selection: Selection{
			StartByte: 0,
			EndByte:   len([]byte("Alpha beta")),
			Text:      "Alpha beta",
		},
	}
}

func waitForAcceptorStart(t *testing.T, started <-chan struct{}) {
	t.Helper()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for acceptor start")
	}
}

func acceptorCalls(acceptor *fakeAcceptor) int {
	acceptor.mu.Lock()
	defer acceptor.mu.Unlock()
	return acceptor.calls
}

func formatRunID(i int) string {
	return fmt.Sprintf("run_%020x", i)
}
