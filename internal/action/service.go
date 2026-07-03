package action

// service.go orchestrates registry lookup, provider execution, and run lifecycle.

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"storywork/internal/agent"
	"storywork/internal/contextpack"
	"storywork/internal/provider"
	"storywork/internal/story"
)

// Service coordinates action registry access, provider execution, and run storage.
type Service struct {
	session      Session
	loader       RegistryLoader
	scenes       SceneLoader
	acceptor     PatchAcceptor
	bodyAcceptor BodyPatchAcceptor
	provider     agent.TextGenerator
	resolver     ProfileResolver
	runs         *RunStore
	ids          RunIDGenerator
	material     ContextMaterialSource
	builder      ContextBuilder
	invitations  *InvitationStore
	inviteIDs    InvitationIDGenerator
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

// Run executes one legacy selection-scoped action run through the shared contextpack path.
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
	target, err := NormalizeLegacyRunRequest(request)
	if err != nil {
		return Run{}, err
	}
	surface := request.Surface
	if surface == "" {
		surface = agent.SurfaceEditor
	}
	return s.RunTagged(ctx, TaggedRunRequest{
		AgentID: request.AgentID,
		StyleID: request.StyleID,
		Surface: surface,
		Target:  target,
	})
}

// Reject discards one pending run without mutating canon.
func (s *Service) Reject(ctx context.Context, runID string) (Run, error) {
	_ = ctx
	if err := ValidateRunID(runID); err != nil {
		return Run{}, err
	}
	return s.runs.MarkRejected(runID)
}

// AcceptRun dispatches patch acceptance to the correct scope-specific handler.
func (s *Service) AcceptRun(ctx context.Context, runID, expectedRevision string) (AcceptResult, error) {
	run, ok := s.runs.Get(runID)
	if !ok {
		return AcceptResult{}, ErrRunNotFound
	}
	if run.Scope == contextpack.ScopeScene {
		return s.AcceptBody(ctx, runID, expectedRevision)
	}
	return s.Accept(ctx, runID, expectedRevision)
}

// Accept applies one pending selection patch to canonical scene markdown.
func (s *Service) Accept(ctx context.Context, runID, expectedRevision string) (AcceptResult, error) {
	if err := ValidateRunID(runID); err != nil {
		return AcceptResult{}, err
	}
	if err := story.ValidateRevision(expectedRevision); err != nil {
		return AcceptResult{}, err
	}
	run, err := s.runs.ClaimAccepting(runID)
	if err != nil {
		return AcceptResult{}, err
	}
	if run.Scope == contextpack.ScopeScene {
		_ = s.runs.ReleasePending(runID)
		return AcceptResult{}, fmt.Errorf("scene body runs require AcceptBody: %w", ErrRunConflict)
	}
	if run.Scope == contextpack.ScopeChapterReview || run.Status == RunCompleted {
		_ = s.runs.ReleasePending(runID)
		return AcceptResult{}, ErrSuggestionNoAccept
	}
	parent, err := ResolveParentRun(s.runs, run)
	if err != nil {
		_ = s.runs.ReleasePending(runID)
		return AcceptResult{}, err
	}
	relationship := invitationRelationshipForRun(run, parent)
	operation, err := BuildOperationMetadata(run, parent, relationship)
	if err != nil {
		_ = s.runs.ReleasePending(runID)
		return AcceptResult{}, err
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
		Operation:        operation,
	})
	if err != nil {
		_ = s.runs.ReleasePending(runID)
		return AcceptResult{}, err
	}
	finalRun, err := s.runs.MarkAccepted(runID)
	if err != nil {
		return AcceptResult{}, err
	}
	invitations, err := s.publishFollowUpInvitations(finalRun, "accept")
	if err != nil {
		return AcceptResult{}, err
	}
	return AcceptResult{Run: finalRun, Scene: scene, FollowUpInvitations: invitations}, nil
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
	compatible, _, err := s.styleCompatibility(ctx, agentDefinition, style)
	return compatible, err
}

func (s *Service) styleCompatibility(ctx context.Context, agentDefinition agent.Agent, style agent.Style) (bool, int, error) {
	if s.resolver == nil || (style.Version == 1 && style.ProviderProfileID == "mock_default" && style.Model == "mock") {
		return true, 0, nil
	}
	resolved, found, err := s.resolver.Resolve(ctx, style.ProviderProfileID)
	if err != nil {
		return false, 0, err
	}
	var profileRef *provider.Profile
	if found {
		profileCopy := resolved.Profile
		profileRef = &profileCopy
	}
	decision := agent.ExecutableCompatibility(agentDefinition, style, profileRef, resolved.Readiness)
	if !decision.Compatible || profileRef == nil {
		return decision.Compatible, 0, nil
	}
	return true, profileRef.Capabilities.MaxContextTokens, nil
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
