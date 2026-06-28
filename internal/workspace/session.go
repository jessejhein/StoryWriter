// Package workspace stores the active project for this local backend process.
package workspace

import (
	"sync"

	"storywork/internal/project"
)

// Session keeps the active project in memory for outline routes.
type Session struct {
	mu      sync.RWMutex
	current project.Project
	has     bool
}

// NewSession creates an empty active-project session.
func NewSession() *Session {
	return &Session{}
}

// Set replaces the current active project.
func (s *Session) Set(current project.Project) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.current = current
	s.has = true
}

// Current returns the active project when one has been set.
func (s *Session) Current() (project.Project, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.has {
		return project.Project{}, false
	}
	return s.current, true
}
