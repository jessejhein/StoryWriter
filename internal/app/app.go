package app

// app.go wires the production adapters into the Storywork HTTP application.

import (
	"context"
	"net/http"
	"time"

	"storywork/internal/action"
	"storywork/internal/agent"
	"storywork/internal/api"
	"storywork/internal/codex"
	"storywork/internal/gitstore"
	"storywork/internal/index"
	"storywork/internal/project"
	"storywork/internal/story"
	"storywork/internal/storyfile"
	"storywork/internal/workspace"
)

type compositeStore struct {
	stories *story.Service
	actions *action.Service
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

// NewHandler creates the production HTTP application for the supplied version string.
func NewHandler(version string) http.Handler {
	git := gitstore.New("git")
	disposableIndex := index.New()
	session := workspace.NewSession()
	projects := project.NewService(git, disposableIndex, time.Now)
	files := storyfile.New()
	stories := story.NewService(session, files, git, disposableIndex, story.NewRandomIDGenerator())
	actions := action.NewService(
		session,
		agent.NewLoader(),
		stories,
		stories,
		agent.NewMockProvider(),
		action.NewRunStore(time.Now),
		action.NewRandomIDGenerator(),
	)
	return api.NewHandler(projects, session, &compositeStore{stories: stories, actions: actions}, version)
}
