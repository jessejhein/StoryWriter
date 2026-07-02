package action

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sync"

	"storywork/internal/agent"
	"storywork/internal/project"
	"storywork/internal/provider"
	"storywork/internal/story"
)

var (
	ErrInvalidRunRequest   = errors.New("invalid action run request")
	ErrRunNotFound         = errors.New("action run not found")
	ErrRunConflict         = errors.New("action run state conflict")
	ErrRunCapacity         = errors.New("action run capacity exhausted")
	ErrAgentNotFound       = errors.New("agent not found")
	ErrStyleNotFound       = errors.New("style not found")
	ErrProviderUnavailable = errors.New("provider unavailable")
	ErrProviderInvalid     = errors.New("provider invalid")
	ErrProviderRejected    = errors.New("provider rejected")
	ErrDuplicateRunID      = errors.New("duplicate action run ID")
)

var runIDPattern = regexp.MustCompile(`^run_[0-9a-f]{20}$`)

type Session interface {
	Current() (project.Project, bool)
}

type RegistryLoader interface {
	Load(projectPath string) (agent.Registry, error)
}

type SceneLoader interface {
	LoadScene(ctx context.Context, sceneID string) (story.SceneDocument, error)
}

type PatchAcceptor interface {
	AcceptScenePatch(ctx context.Context, request story.AcceptScenePatchRequest) (story.SceneDocument, error)
}

type RunIDGenerator interface {
	Next() (string, error)
}

type ProfileResolver interface {
	Resolve(ctx context.Context, profileID string) (provider.ResolvedProfile, bool, error)
}

type Selection struct {
	StartByte int    `json:"start_byte"`
	EndByte   int    `json:"end_byte"`
	Text      string `json:"text,omitempty"`
}

type RunRequest struct {
	AgentID       string
	StyleID       string
	Surface       agent.Surface
	InputScope    agent.InputScope
	SceneID       string
	SceneRevision string
	Selection     Selection
}

type AvailableAction struct {
	AgentID            string   `json:"agent_id"`
	Name               string   `json:"name"`
	Description        string   `json:"description"`
	OutputMode         string   `json:"output_mode"`
	RequiresAcceptance bool     `json:"requires_acceptance"`
	StyleIDs           []string `json:"style_ids"`
}

type RunStatus string

const (
	RunPending   RunStatus = "pending"
	RunAccepting RunStatus = "accepting"
	RunAccepted  RunStatus = "accepted"
	RunRejected  RunStatus = "rejected"
)

type Run struct {
	RunID          string                 `json:"run_id"`
	Status         RunStatus              `json:"status"`
	AgentID        string                 `json:"agent_id"`
	StyleID        string                 `json:"style_id"`
	SceneID        string                 `json:"scene_id"`
	SceneRevision  string                 `json:"scene_revision"`
	Selection      Selection              `json:"selection"`
	OriginalText   string                 `json:"-"`
	Replacement    string                 `json:"-"`
	ContextSummary agent.ContextSummary   `json:"context_summary"`
	Provider       agent.ProviderIdentity `json:"provider"`
}

type RunStore struct {
	mu    sync.Mutex
	runs  map[string]Run
	order []string
}

func NewRunStore() *RunStore {
	return &RunStore{
		runs: make(map[string]Run),
	}
}

func (s *RunStore) Insert(run Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.runs[run.RunID]; exists {
		return ErrDuplicateRunID
	}
	if len(s.runs) >= 1000 {
		evicted := false
		for _, runID := range append([]string(nil), s.order...) {
			current := s.runs[runID]
			if current.Status == RunAccepted || current.Status == RunRejected {
				delete(s.runs, runID)
				s.order = removeID(s.order, runID)
				evicted = true
				break
			}
		}
		if !evicted {
			return ErrRunCapacity
		}
	}
	s.runs[run.RunID] = run
	s.order = append(s.order, run.RunID)
	return nil
}

func (s *RunStore) ClaimAccepting(runID string) (Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	run, ok := s.runs[runID]
	if !ok {
		return Run{}, ErrRunNotFound
	}
	if run.Status != RunPending {
		return Run{}, ErrRunConflict
	}
	run.Status = RunAccepting
	s.runs[runID] = run
	return run, nil
}

func (s *RunStore) ReleasePending(runID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	run, ok := s.runs[runID]
	if !ok {
		return ErrRunNotFound
	}
	if run.Status != RunAccepting {
		return ErrRunConflict
	}
	run.Status = RunPending
	s.runs[runID] = run
	return nil
}

func (s *RunStore) MarkAccepted(runID string) (Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	run, ok := s.runs[runID]
	if !ok {
		return Run{}, ErrRunNotFound
	}
	if run.Status != RunAccepting {
		return Run{}, ErrRunConflict
	}
	run.Status = RunAccepted
	s.runs[runID] = run
	return run, nil
}

func (s *RunStore) MarkRejected(runID string) (Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	run, ok := s.runs[runID]
	if !ok {
		return Run{}, ErrRunNotFound
	}
	if run.Status != RunPending {
		return Run{}, ErrRunConflict
	}
	run.Status = RunRejected
	s.runs[runID] = run
	return run, nil
}

func removeID(ids []string, target string) []string {
	result := make([]string, 0, len(ids))
	for _, item := range ids {
		if item != target {
			result = append(result, item)
		}
	}
	return result
}

// ValidateRunID validates the opaque action-run identifier syntax.
func ValidateRunID(runID string) error {
	if !runIDPattern.MatchString(runID) {
		return fmt.Errorf("run_id %q is invalid: %w", runID, ErrInvalidRunRequest)
	}
	return nil
}

// ValidateRunRequest validates syntax before registry lookup or scene access.
func ValidateRunRequest(request RunRequest) error {
	if !registryRequestIDPattern.MatchString(request.AgentID) {
		return fmt.Errorf("agent_id %q is invalid: %w", request.AgentID, ErrInvalidRunRequest)
	}
	if !registryRequestIDPattern.MatchString(request.StyleID) {
		return fmt.Errorf("style_id %q is invalid: %w", request.StyleID, ErrInvalidRunRequest)
	}
	if request.Surface != agent.SurfaceEditor && request.Surface != agent.SurfaceChapterView {
		return fmt.Errorf("surface %q is invalid: %w", request.Surface, ErrInvalidRunRequest)
	}
	if request.InputScope != agent.InputScopeSelection && request.InputScope != agent.InputScopeChapter {
		return fmt.Errorf("input_scope %q is invalid: %w", request.InputScope, ErrInvalidRunRequest)
	}
	if err := story.ValidateSceneID(request.SceneID); err != nil {
		return err
	}
	if err := story.ValidateRevision(request.SceneRevision); err != nil {
		return err
	}
	return nil
}

var registryRequestIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
