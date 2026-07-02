package action

// service.go orchestrates registry lookup, provider execution, and run lifecycle.

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"storywork/internal/agent"
	"storywork/internal/provider"
	"storywork/internal/story"
)

// Service coordinates action registry access, provider execution, and run storage.
type Service struct {
	session  Session
	loader   RegistryLoader
	scenes   SceneLoader
	acceptor PatchAcceptor
	provider agent.TextGenerator
	resolver ProfileResolver
	runs     *RunStore
	ids      RunIDGenerator
}

// NewService creates the action orchestration service.
func NewService(session Session, loader RegistryLoader, scenes SceneLoader, acceptor PatchAcceptor, provider agent.TextGenerator, resolver ProfileResolver, runs *RunStore, ids RunIDGenerator) *Service {
	return &Service{
		session:  session,
		loader:   loader,
		scenes:   scenes,
		acceptor: acceptor,
		provider: provider,
		resolver: resolver,
		runs:     runs,
		ids:      ids,
	}
}

// Agents returns the project-local agent registry.
func (s *Service) Agents(ctx context.Context) ([]agent.Agent, error) {
	registry, err := s.registry()
	if err != nil {
		return nil, err
	}
	return append([]agent.Agent(nil), registry.Agents...), nil
}

// Styles returns the project-local style registry.
func (s *Service) Styles(ctx context.Context) ([]agent.Style, error) {
	registry, err := s.registry()
	if err != nil {
		return nil, err
	}
	return append([]agent.Style(nil), registry.Styles...), nil
}

// AvailableActions returns applicable actions for the current author state.
func (s *Service) AvailableActions(ctx context.Context, input agent.AvailabilityInput) ([]AvailableAction, error) {
	registry, err := s.registry()
	if err != nil {
		return nil, err
	}
	decisions := agent.ApplicableAgents(registry.Agents, input)
	styleIDs := make([]string, 0, len(registry.Styles))
	result := make([]AvailableAction, 0, len(decisions))
	for _, decision := range decisions {
		if !decision.Applicable {
			continue
		}
		compatibleStyles := make([]agent.Style, 0, len(registry.Styles))
		for _, style := range registry.Styles {
			compatible, err := s.styleCompatible(ctx, decision.Agent, style)
			if err != nil {
				return nil, err
			}
			if compatible {
				compatibleStyles = append(compatibleStyles, style)
			}
		}
		if len(compatibleStyles) == 0 {
			continue
		}
		sort.Slice(compatibleStyles, func(i, j int) bool {
			if compatibleStyles[i].Name != compatibleStyles[j].Name {
				return compatibleStyles[i].Name < compatibleStyles[j].Name
			}
			return compatibleStyles[i].ID < compatibleStyles[j].ID
		})
		styleIDs = styleIDs[:0]
		for _, style := range compatibleStyles {
			styleIDs = append(styleIDs, style.ID)
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

// Run executes one legacy selection-scoped action run.
func (s *Service) Run(ctx context.Context, request RunRequest) (Run, error) {
	if err := ValidateRunRequest(request); err != nil {
		return Run{}, err
	}
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
	if compatible, err := s.styleCompatible(ctx, agentDefinition, styleDefinition); err != nil {
		return Run{}, err
	} else if !compatible {
		return Run{}, fmt.Errorf("style %q is incompatible with agent %q: %w", styleDefinition.ID, agentDefinition.ID, ErrProviderInvalid)
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
		return Run{}, mapProviderError(err)
	}
	replacement, err := validateGeneratedReplacement(generated.Replacement)
	if err != nil {
		return Run{}, err
	}
	if replacement == selected {
		return Run{}, story.ErrNoSceneChanges
	}
	run := Run{
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
		Replacement:    replacement,
		ContextSummary: summary,
		Provider:       generated.Provider,
	}
	return s.insertRun(run)
}

// Reject discards one pending run without mutating canon.
func (s *Service) Reject(ctx context.Context, runID string) (Run, error) {
	_ = ctx
	if err := ValidateRunID(runID); err != nil {
		return Run{}, err
	}
	return s.runs.MarkRejected(runID)
}

// Accept applies one pending selection patch to canonical scene markdown.
func (s *Service) Accept(ctx context.Context, runID, expectedRevision string) (Run, story.SceneDocument, error) {
	if err := ValidateRunID(runID); err != nil {
		return Run{}, story.SceneDocument{}, err
	}
	if err := story.ValidateRevision(expectedRevision); err != nil {
		return Run{}, story.SceneDocument{}, err
	}
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

func validateGeneratedReplacement(replacement string) (string, error) {
	normalized, err := agent.NormalizeGeneratedReplacement(replacement)
	if errors.Is(err, agent.ErrProviderInvalid) {
		return "", fmt.Errorf("%w: %w", ErrProviderInvalid, err)
	}
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrProviderRejected, err)
	}
	return normalized, nil
}

func (s *Service) styleCompatible(ctx context.Context, agentDefinition agent.Agent, style agent.Style) (bool, error) {
	if s.resolver == nil || (style.Version == 1 && style.ProviderProfileID == "mock_default" && style.Model == "mock") {
		return true, nil
	}
	resolved, found, err := s.resolver.Resolve(ctx, style.ProviderProfileID)
	if err != nil {
		return false, err
	}
	var profileRef *provider.Profile
	if found {
		profileCopy := resolved.Profile
		profileRef = &profileCopy
	}
	decision := agent.ExecutableCompatibility(agentDefinition, style, profileRef, resolved.Readiness)
	return decision.Compatible, nil
}

func (s *Service) insertRun(run Run) (Run, error) {
	for attempts := 0; attempts < 5; attempts++ {
		runID, err := s.ids.Next()
		if err != nil {
			return Run{}, err
		}
		if err := ValidateRunID(runID); err != nil {
			return Run{}, fmt.Errorf("generated run ID: %w", err)
		}
		run.RunID = runID
		if err := s.runs.Insert(run); err == nil {
			return run, nil
		} else if !errors.Is(err, ErrDuplicateRunID) {
			return Run{}, err
		}
	}
	return Run{}, errors.New("generate unique action run ID after 5 attempts")
}

func mapProviderError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, agent.ErrProviderOffline) {
		return fmt.Errorf("%w: %w", ErrProviderUnavailable, err)
	}
	if errors.Is(err, agent.ErrProviderInvalid) {
		return fmt.Errorf("%w: %w", ErrProviderInvalid, err)
	}
	if errors.Is(err, agent.ErrProviderRejected) {
		return fmt.Errorf("%w: %w", ErrProviderRejected, err)
	}
	return err
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