package app

// app.go wires the production adapters into the Storywork HTTP application.

import (
	"net/http"
	"time"

	"storywork/internal/api"
	"storywork/internal/gitstore"
	"storywork/internal/index"
	"storywork/internal/project"
	"storywork/internal/story"
	"storywork/internal/storyfile"
	"storywork/internal/workspace"
)

// NewHandler creates the production HTTP application for the supplied version string.
func NewHandler(version string) http.Handler {
	git := gitstore.New("git")
	disposableIndex := index.New()
	session := workspace.NewSession()
	projects := project.NewService(git, disposableIndex, time.Now)
	stories := story.NewService(session, storyfile.New(), git, disposableIndex, story.NewRandomIDGenerator())
	return api.NewHandler(projects, session, stories, version)
}
