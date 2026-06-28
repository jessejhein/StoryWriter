// Package app composes the backend dependencies.
package app

import (
	"net/http"
	"time"

	"storywork/internal/api"
	"storywork/internal/gitstore"
	"storywork/internal/index"
	"storywork/internal/project"
)

// NewHandler creates the production HTTP application.
func NewHandler(version string) http.Handler {
	projects := project.NewService(gitstore.New("git"), index.New(), time.Now)
	return api.NewHandler(projects, version)
}
