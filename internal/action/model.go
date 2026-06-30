package action

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"storywork/internal/agent"
	"storywork/internal/project"
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
)

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
	RunID          string               `json:"run_id"`
	Status         RunStatus            `json:"status"`
	AgentID        string               `json:"agent_id"`
	StyleID        string               `json:"style_id"`
	SceneID        string               `json:"scene_id"`
	SceneRevision  string               `json:"scene_revision"`
	Selection      Selection            `json:"selection"`
	OriginalText   string               `json:"-"`
	Replacement    string               `json:"-"`
	ContextSummary agent.ContextSummary `json:"context_summary"`
	createdAt      time.Time
}

type RunStore struct {
	mu    sync.Mutex
	runs  map[string]Run
	order []string
	now   func() time.Time
}

func NewRunStore(now func() time.Time) *RunStore {
	return &RunStore{
		runs: make(map[string]Run),
		now:  now,
	}
}

func (s *RunStore) Insert(run Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()

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
	run.createdAt = s.now()
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
	result := ids[:0]
	for _, item := range ids {
		if item != target {
			result = append(result, item)
		}
	}
	return result
}

type Service struct {
	session  Session
	loader   RegistryLoader
	scenes   SceneLoader
	acceptor PatchAcceptor
	provider agent.TextGenerator
	runs     *RunStore
	ids      RunIDGenerator
}

func NewService(session Session, loader RegistryLoader, scenes SceneLoader, acceptor PatchAcceptor, provider agent.TextGenerator, runs *RunStore, ids RunIDGenerator) *Service {
	return &Service{
		session:  session,
		loader:   loader,
		scenes:   scenes,
		acceptor: acceptor,
		provider: provider,
		runs:     runs,
		ids:      ids,
	}
}

func (s *Service) Agents(ctx context.Context) ([]agent.Agent, error) {
	registry, err := s.registry()
	if err != nil {
		return nil, err
	}
	return append([]agent.Agent(nil), registry.Agents...), nil
}

func (s *Service) Styles(ctx context.Context) ([]agent.Style, error) {
	registry, err := s.registry()
	if err != nil {
		return nil, err
	}
	return append([]agent.Style(nil), registry.Styles...), nil
}

func (s *Service) AvailableActions(ctx context.Context, input agent.AvailabilityInput) ([]AvailableAction, error) {
	registry, err := s.registry()
	if err != nil {
		return nil, err
	}
	decisions := agent.ApplicableAgents(registry.Agents, input)
	styleIDs := make([]string, 0, len(registry.Styles))
	for _, style := range registry.Styles {
		styleIDs = append(styleIDs, style.ID)
	}
	result := make([]AvailableAction, 0, len(decisions))
	for _, decision := range decisions {
		if !decision.Applicable {
			continue
		}
		result = append(result, AvailableAction{
			AgentID:            decision.Agent.ID,
			Name:               decision.Agent.Name,
			Description:        decision.Agent.Description,
			OutputMode:         string(decision.Agent.Control.OutputMode),
			RequiresAcceptance: decision.Agent.Control.RequiresAcceptance,
			StyleIDs:           append([]string(nil), styleIDs...),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Name != result[j].Name {
			return result[i].Name < result[j].Name
		}
		return result[i].AgentID < result[j].AgentID
	})
	return result, nil
}

func (s *Service) Run(ctx context.Context, request RunRequest) (Run, error) {
	registry, err := s.registry()
	if err != nil {
		return Run{}, err
	}
	agentDefinition, err := findAgent(registry.Agents, request.AgentID)
	if err != nil {
		return Run{}, err
	}
	if err := agent.ValidateExecutableSelectionAgent(agentDefinition); err != nil {
		return Run{}, err
	}
	styleDefinition, err := findStyle(registry.Styles, request.StyleID)
	if err != nil {
		return Run{}, err
	}
	scene, err := s.scenes.LoadScene(ctx, request.SceneID)
	if err != nil {
		return Run{}, err
	}
	if scene.Revision != request.SceneRevision {
		return Run{}, fmt.Errorf("scene %q revision changed: %w", request.SceneID, story.ErrStaleRevision)
	}
	selected, err := story.ValidateMarkdownSelection(scene.Markdown, request.Selection.StartByte, request.Selection.EndByte, request.Selection.Text)
	if err != nil {
		return Run{}, err
	}
	selectionWords := agent.WordCount(selected)
	decisions := agent.ApplicableAgents([]agent.Agent{agentDefinition}, agent.AvailabilityInput{
		Surface:        request.Surface,
		InputScope:     request.InputScope,
		SceneID:        request.SceneID,
		SelectionWords: selectionWords,
	})
	if len(decisions) != 1 || !decisions[0].Applicable {
		return Run{}, fmt.Errorf("%s: %w", decisions[0].ExcludedReason, agent.ErrInapplicable)
	}
	packet, summary, err := agent.BuildContext(agent.BuildContextRequest{
		Agent:        agentDefinition,
		Style:        styleDefinition,
		SelectedText: selected,
	})
	if err != nil {
		return Run{}, err
	}
	generated, err := s.provider.Generate(ctx, agent.GenerateRequest{
		Agent:   agentDefinition,
		Style:   styleDefinition,
		Packet:  packet,
		Summary: summary,
	})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return Run{}, fmt.Errorf("%w: %w", ErrProviderUnavailable, err)
		}
		return Run{}, err
	}
	if generated.Replacement == selected {
		return Run{}, story.ErrNoSceneChanges
	}
	runID, err := s.ids.Next()
	if err != nil {
		return Run{}, err
	}
	run := Run{
		RunID:         runID,
		Status:        RunPending,
		AgentID:       agentDefinition.ID,
		StyleID:       styleDefinition.ID,
		SceneID:       request.SceneID,
		SceneRevision: request.SceneRevision,
		Selection: Selection{
			StartByte: request.Selection.StartByte,
			EndByte:   request.Selection.EndByte,
		},
		OriginalText:   selected,
		Replacement:    generated.Replacement,
		ContextSummary: summary,
	}
	if err := s.runs.Insert(run); err != nil {
		return Run{}, err
	}
	return run, nil
}

func (s *Service) Reject(ctx context.Context, runID string) (Run, error) {
	_ = ctx
	return s.runs.MarkRejected(runID)
}

func (s *Service) Accept(ctx context.Context, runID, expectedRevision string) (Run, story.SceneDocument, error) {
	run, err := s.runs.ClaimAccepting(runID)
	if err != nil {
		return Run{}, story.SceneDocument{}, err
	}
	scene, err := s.acceptor.AcceptScenePatch(ctx, story.AcceptScenePatchRequest{
		RunID:            run.RunID,
		SceneID:          run.SceneID,
		RunSceneRevision: run.SceneRevision,
		ExpectedRevision: expectedRevision,
		StartByte:        run.Selection.StartByte,
		EndByte:          run.Selection.EndByte,
		OriginalText:     run.OriginalText,
		ReplacementText:  run.Replacement,
	})
	if err != nil {
		_ = s.runs.ReleasePending(runID)
		return Run{}, story.SceneDocument{}, err
	}
	finalRun, err := s.runs.MarkAccepted(runID)
	if err != nil {
		return Run{}, story.SceneDocument{}, err
	}
	return finalRun, scene, nil
}

func (s *Service) registry() (agent.Registry, error) {
	current, ok := s.session.Current()
	if !ok {
		return agent.Registry{}, story.ErrNoActiveProject
	}
	return s.loader.Load(current.Path)
}

func findAgent(agents []agent.Agent, agentID string) (agent.Agent, error) {
	for _, item := range agents {
		if item.ID == strings.TrimSpace(agentID) {
			return item, nil
		}
	}
	return agent.Agent{}, ErrAgentNotFound
}

func findStyle(styles []agent.Style, styleID string) (agent.Style, error) {
	for _, item := range styles {
		if item.ID == strings.TrimSpace(styleID) {
			return item, nil
		}
	}
	return agent.Style{}, ErrStyleNotFound
}
