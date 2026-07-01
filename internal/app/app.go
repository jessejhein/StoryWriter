package app

// app.go wires the production adapters into the Storywork HTTP application.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"storywork/internal/action"
	"storywork/internal/agent"
	"storywork/internal/api"
	"storywork/internal/codex"
	"storywork/internal/gitstore"
	"storywork/internal/index"
	"storywork/internal/project"
	"storywork/internal/provider"
	"storywork/internal/story"
	"storywork/internal/storyfile"
	"storywork/internal/workspace"
)

type compositeStore struct {
	stories   *story.Service
	actions   *action.Service
	providers *providerDependencies
}

var errInvalidProviderConfigPath = errors.New("invalid provider config path")

type providerDependencies struct {
	service   *provider.Service
	configErr error
}

func newProviderDependencies(override string, userConfigDir func() (string, error)) *providerDependencies {
	configDir, err := provider.ResolveConfigDir(override, userConfigDir)
	if err != nil {
		return &providerDependencies{configErr: fmt.Errorf("%w: %w", errInvalidProviderConfigPath, err)}
	}
	return &providerDependencies{service: provider.NewService(
		provider.NewStore(filepath.Join(configDir, "providers.yaml")),
		provider.EnvironmentBroker{LookupEnv: os.LookupEnv},
	)}
}

func (p *providerDependencies) List(ctx context.Context) ([]provider.Profile, *string, error) {
	if p.configErr != nil {
		return nil, nil, p.configErr
	}
	return p.service.List(ctx)
}

func (p *providerDependencies) Save(ctx context.Context, profiles []provider.Profile, expectedRevision *string) ([]provider.Profile, *string, error) {
	if p.configErr != nil {
		return nil, nil, p.configErr
	}
	return p.service.Save(ctx, profiles, expectedRevision)
}

func (p *providerDependencies) Resolve(ctx context.Context, profileID string) (provider.ResolvedProfile, bool, error) {
	if p.configErr != nil {
		return provider.ResolvedProfile{}, false, p.configErr
	}
	return p.service.Resolve(ctx, profileID)
}

func (s *compositeStore) Outline(ctx context.Context) (story.Outline, error) {
	return s.stories.Outline(ctx)
}
func (s *compositeStore) CreateArc(ctx context.Context, title string) (story.MutationResult, error) {
	return s.stories.CreateArc(ctx, title)
}
func (s *compositeStore) CreateChapter(ctx context.Context, arcID, title string) (story.MutationResult, error) {
	return s.stories.CreateChapter(ctx, arcID, title)
}
func (s *compositeStore) CreateScene(ctx context.Context, chapterID, title string) (story.MutationResult, error) {
	return s.stories.CreateScene(ctx, chapterID, title)
}
func (s *compositeStore) Reorder(ctx context.Context, request story.ReorderRequest) (story.MutationResult, error) {
	return s.stories.Reorder(ctx, request)
}
func (s *compositeStore) LoadScene(ctx context.Context, sceneID string) (story.SceneDocument, error) {
	return s.stories.LoadScene(ctx, sceneID)
}
func (s *compositeStore) SaveScene(ctx context.Context, sceneID string, request story.SaveSceneRequest) (story.SceneDocument, error) {
	return s.stories.SaveScene(ctx, sceneID, request)
}
func (s *compositeStore) CodexEntries(ctx context.Context) ([]codex.Entry, error) {
	return s.stories.CodexEntries(ctx)
}
func (s *compositeStore) LoadCodexEntry(ctx context.Context, entryID string) (codex.Entry, error) {
	return s.stories.LoadCodexEntry(ctx, entryID)
}
func (s *compositeStore) CreateCodexEntry(ctx context.Context, request codex.SaveEntryRequest) (codex.Entry, error) {
	return s.stories.CreateCodexEntry(ctx, request)
}
func (s *compositeStore) UpdateCodexEntry(ctx context.Context, entryID string, request codex.SaveEntryRequest) (codex.Entry, error) {
	return s.stories.UpdateCodexEntry(ctx, entryID, request)
}
func (s *compositeStore) LoadProgressions(ctx context.Context, entryID string) (codex.ProgressionDocument, error) {
	return s.stories.LoadProgressions(ctx, entryID)
}
func (s *compositeStore) SaveProgressions(ctx context.Context, entryID string, request codex.SaveProgressionsRequest) (codex.ProgressionDocument, error) {
	return s.stories.SaveProgressions(ctx, entryID, request)
}
func (s *compositeStore) ResolveActiveCodexState(ctx context.Context, entryID, sceneID string) (codex.ActiveState, error) {
	return s.stories.ResolveActiveCodexState(ctx, entryID, sceneID)
}
func (s *compositeStore) Agents(ctx context.Context) ([]agent.Agent, error) {
	return s.actions.Agents(ctx)
}
func (s *compositeStore) Styles(ctx context.Context) ([]agent.Style, error) {
	return s.actions.Styles(ctx)
}
func (s *compositeStore) AvailableActions(ctx context.Context, input agent.AvailabilityInput) ([]action.AvailableAction, error) {
	return s.actions.AvailableActions(ctx, input)
}
func (s *compositeStore) Run(ctx context.Context, request action.RunRequest) (action.Run, error) {
	return s.actions.Run(ctx, request)
}
func (s *compositeStore) Accept(ctx context.Context, runID, expectedRevision string) (action.Run, story.SceneDocument, error) {
	return s.actions.Accept(ctx, runID, expectedRevision)
}
func (s *compositeStore) Reject(ctx context.Context, runID string) (action.Run, error) {
	return s.actions.Reject(ctx, runID)
}
func (s *compositeStore) ProviderProfiles(ctx context.Context) ([]provider.Profile, *string, error) {
	return s.providers.List(ctx)
}
func (s *compositeStore) SaveProviderProfiles(ctx context.Context, profiles []provider.Profile, expectedRevision *string) ([]provider.Profile, *string, error) {
	return s.providers.Save(ctx, profiles, expectedRevision)
}

// NewHandler creates the production HTTP application for the supplied version string.
func NewHandler(version string) http.Handler {
	git := gitstore.New("git")
	disposableIndex := index.New()
	session := workspace.NewSession()
	projects := project.NewService(git, disposableIndex, time.Now)
	files := storyfile.New()
	stories := story.NewService(session, files, git, disposableIndex, story.NewRandomIDGenerator())
	providerService := newProviderDependencies(os.Getenv("STORYWORK_CONFIG_DIR"), os.UserConfigDir)
	actions := action.NewService(
		session,
		agent.NewLoader(),
		stories,
		stories,
		agent.NewDispatcher(providerService, nil),
		providerService,
		action.NewRunStore(),
		action.NewRandomIDGenerator(),
	)
	return api.NewHandler(projects, session, &compositeStore{
		stories:   stories,
		actions:   actions,
		providers: providerService,
	}, version)
}
