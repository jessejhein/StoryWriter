package action

// context_preview.go orchestrates zero-side-effect context inspection.

import (
	"context"
	"fmt"

	"storywork/internal/agent"
	"storywork/internal/contextpack"
	"storywork/internal/story"
)

// ContextMaterialSource loads coherent canonical snapshots for context assembly.
type ContextMaterialSource interface {
	LoadSelectionMaterial(context.Context, story.SelectionMaterialRequest) (story.ContextMaterialResult, error)
	LoadSceneMaterial(context.Context, string, string) (story.ContextMaterialResult, error)
	LoadChapterMaterial(context.Context, string) (story.ContextMaterialResult, error)
}

// ContextBuilder assembles typed packets and redacted manifests.
type ContextBuilder interface {
	Build(contextpack.BuildRequest) (contextpack.Packet, contextpack.Manifest, error)
}

// ContextPreviewResult is the redacted preview response.
type ContextPreviewResult struct {
	Manifest       contextpack.Manifest
	TargetRevision string
}

// WithMaterialSource injects the story-owned material loader.
func (s *Service) WithMaterialSource(source ContextMaterialSource) *Service {
	s.material = source
	return s
}

// WithContextBuilder injects the pure context builder.
func (s *Service) WithContextBuilder(builder ContextBuilder) *Service {
	s.builder = builder
	return s
}

// WithBodyAcceptor injects the scene body patch acceptor.
func (s *Service) WithBodyAcceptor(acceptor BodyPatchAcceptor) *Service {
	s.bodyAcceptor = acceptor
	return s
}

// PreviewContext validates and builds a redacted manifest without provider or run side effects.
func (s *Service) PreviewContext(ctx context.Context, request TaggedRunRequest) (ContextPreviewResult, error) {
	if err := ValidateTaggedRunRequest(request); err != nil {
		return ContextPreviewResult{}, err
	}
	if s.material == nil || s.builder == nil {
		return ContextPreviewResult{}, fmt.Errorf("context preview is not configured")
	}
	registry, err := s.registry()
	if err != nil {
		return ContextPreviewResult{}, err
	}
	agentDefinition, err := findAgent(registry.Agents, request.AgentID)
	if err != nil {
		return ContextPreviewResult{}, err
	}
	styleDefinition, err := findStyle(registry.Styles, request.StyleID)
	if err != nil {
		return ContextPreviewResult{}, err
	}
	if compatible, err := s.styleCompatible(ctx, agentDefinition, styleDefinition); err != nil {
		return ContextPreviewResult{}, err
	} else if !compatible {
		return ContextPreviewResult{}, fmt.Errorf("style %q is incompatible with agent %q: %w", styleDefinition.ID, agentDefinition.ID, ErrProviderInvalid)
	}
	if err := validateAgentScope(agentDefinition, request.Target); err != nil {
		return ContextPreviewResult{}, err
	}
	materialResult, err := s.loadMaterial(ctx, request.Target)
	if err != nil {
		return ContextPreviewResult{}, err
	}
	material := materialResult.Material
	material.Style = contextpack.StyleSheet{
		ID: styleDefinition.ID, Name: styleDefinition.Name, SystemPrompt: styleDefinition.SystemPrompt,
	}
	_, manifest, err := s.builder.Build(contextpack.BuildRequest{
		Scope:     request.Target.Scope,
		Policy:    agentPolicy(agentDefinition),
		Budget:    agentBudget(agentDefinition),
		RAGMode:   contextpack.RAGMode(agentDefinition.RAGPolicy.Mode),
		Material:  material,
		Estimator: contextpack.ByteEstimator{},
	})
	if err != nil {
		return ContextPreviewResult{}, err
	}
	return ContextPreviewResult{Manifest: manifest, TargetRevision: materialResult.TargetRevision}, nil
}

func (s *Service) loadMaterial(ctx context.Context, target TaggedTarget) (story.ContextMaterialResult, error) {
	switch target.Scope {
	case contextpack.ScopeSelection:
		selection := target.Selection
		return s.material.LoadSelectionMaterial(ctx, story.SelectionMaterialRequest{
			SceneID: selection.SceneID, SceneRevision: selection.SceneRevision,
			StartByte: selection.StartByte, EndByte: selection.EndByte, SelectedText: selection.SelectedText,
		})
	case contextpack.ScopeScene:
		return s.material.LoadSceneMaterial(ctx, target.Scene.SceneID, target.Scene.SceneRevision)
	case contextpack.ScopeChapterReview:
		result, err := s.material.LoadChapterMaterial(ctx, target.Chapter.ChapterID)
		if err != nil {
			return story.ContextMaterialResult{}, err
		}
		if result.TargetRevision != target.Chapter.Fingerprint {
			return story.ContextMaterialResult{}, fmt.Errorf("chapter %q fingerprint changed: %w", target.Chapter.ChapterID, story.ErrStaleRevision)
		}
		return result, nil
	default:
		return story.ContextMaterialResult{}, fmt.Errorf("scope %q is unsupported: %w", target.Scope, ErrInvalidRunRequest)
	}
}

func validateAgentScope(agentDefinition agent.Agent, target TaggedTarget) error {
	input := agent.AvailabilityInput{}
	switch target.Scope {
	case contextpack.ScopeSelection:
		input.Surface = agent.SurfaceEditor
		input.InputScope = agent.InputScopeSelection
		if target.Selection != nil {
			input.SceneID = target.Selection.SceneID
			input.SelectionWords = agent.WordCount(target.Selection.SelectedText)
		}
	case contextpack.ScopeScene:
		input.Surface = agent.SurfaceEditor
		input.InputScope = agent.InputScopeScene
		if target.Scene != nil {
			input.SceneID = target.Scene.SceneID
		}
		input.SelectionWords = agentDefinition.AppliesWhen.MinWords
	case contextpack.ScopeChapterReview:
		input.Surface = agent.SurfaceChapterView
		input.InputScope = agent.InputScopeChapterReview
		input.SelectionWords = agentDefinition.AppliesWhen.MinWords
	default:
		return fmt.Errorf("scope %q is unsupported: %w", target.Scope, ErrInvalidRunRequest)
	}
	decisions := agent.ApplicableAgents([]agent.Agent{agentDefinition}, input)
	if len(decisions) != 1 || !decisions[0].Applicable {
		return fmt.Errorf("%s: %w", decisions[0].ExcludedReason, agent.ErrInapplicable)
	}
	return nil
}

func agentPolicy(agentDefinition agent.Agent) contextpack.Policy {
	return contextpack.Policy{
		Required:  filterKnownPacks(convertPacks(agentDefinition.ContextPolicy.Required)),
		Optional:  filterKnownPacks(convertPacks(agentDefinition.ContextPolicy.Optional)),
		Forbidden: filterKnownPacks(convertPacks(agentDefinition.ContextPolicy.Forbidden)),
	}
}

func filterKnownPacks(packs []contextpack.Pack) []contextpack.Pack {
	known := map[contextpack.Pack]struct{}{
		contextpack.PackSelectedText: {}, contextpack.PackStyleSheet: {},
		contextpack.PackCurrentScene: {}, contextpack.PackCurrentChapter: {},
		contextpack.PackOutlineNeighbor: {}, contextpack.PackActiveCodex: {},
	}
	filtered := make([]contextpack.Pack, 0, len(packs))
	for _, pack := range packs {
		if _, ok := known[pack]; ok {
			filtered = append(filtered, pack)
		}
	}
	return filtered
}

func agentBudget(agentDefinition agent.Agent) contextpack.Budget {
	budget := contextpack.Budget{
		MaxInputEstimatedTokens:       agentDefinition.ContextBudget.MaxInputEstimatedTokens,
		ReservedOutputEstimatedTokens: agentDefinition.ContextBudget.ReservedOutputEstimatedTokens,
	}
	if budget.MaxInputEstimatedTokens == 0 {
		budget.MaxInputEstimatedTokens = max(agentDefinition.ModelRequirements.MinContextTokens*2, 8000)
	}
	if budget.ReservedOutputEstimatedTokens == 0 {
		budget.ReservedOutputEstimatedTokens = 1000
	}
	return budget
}

func convertPacks(packs []agent.ContextPack) []contextpack.Pack {
	converted := make([]contextpack.Pack, len(packs))
	for index, pack := range packs {
		converted[index] = contextpack.Pack(pack)
	}
	return converted
}
