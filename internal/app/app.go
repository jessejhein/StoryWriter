// Package app composes the backend dependencies.
package app

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

// NewHandler creates the production HTTP application.
func NewHandler(version string) http.Handler {
	git := gitstore.New("git")
	disposableIndex := index.New()
	session := workspace.NewSession()
	projects := project.NewService(git, disposableIndex, time.Now)
	stories := story.NewService(session, storyfile.New(), git, disposableIndex, story.NewRandomIDGenerator())
	return api.NewHandler(projects, session, stories, version)
}
