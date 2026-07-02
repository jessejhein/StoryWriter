package action

// scene_run.go orchestrates tagged action runs across selection, scene, and chapter scopes.

import (
	"context"
	"fmt"

	"storywork/internal/agent"
	"storywork/internal/contextpack"
	"storywork/internal/story"
)

// RunTagged executes one scope-aware action run with context rebuild and validation.
func (s *Service) RunTagged(ctx context.Context, request TaggedRunRequest) (Run, error) {
	if err := ValidateTaggedRunRequest(request); err != nil {
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
	styleDefinition, err := findStyle(registry.Styles, request.StyleID)
	if err != nil {
		return Run{}, err
	}
	if compatible, err := s.styleCompatible(ctx, agentDefinition, styleDefinition); err != nil {
		return Run{}, err
	} else if !compatible {
		return Run{}, fmt.Errorf("style %q is incompatible with agent %q: %w", styleDefinition.ID, agentDefinition.ID, ErrProviderInvalid)
	}
	if err := validateAgentScope(agentDefinition, request.Target); err != nil {
		return Run{}, err
	}
	packet, manifest, materialResult, err := s.buildRunContext(ctx, request, agentDefinition, styleDefinition)
	if err != nil {
		return Run{}, err
	}
	generated, err := s.provider.Generate(ctx, agent.GenerateRequest{
		Agent:       agentDefinition,
		Style:       styleDefinition,
		TypedPacket: packet,
		Manifest:    manifest,
	})
	if err != nil {
		return Run{}, mapProviderError(err)
	}
	return s.insertTaggedRun(ctx, request, agentDefinition, manifest, materialResult, generated)
}

func (s *Service) buildRunContext(ctx context.Context, request TaggedRunRequest, agentDefinition agent.Agent, styleDefinition agent.Style) (contextpack.Packet, contextpack.Manifest, story.ContextMaterialResult, error) {
	if s.material == nil || s.builder == nil {
		return nil, contextpack.Manifest{}, story.ContextMaterialResult{}, fmt.Errorf("context assembly is not configured")
	}
	materialResult, err := s.loadMaterial(ctx, request.Target)
	if err != nil {
		return nil, contextpack.Manifest{}, story.ContextMaterialResult{}, err
	}
	material := materialResult.Material
	material.Style = contextpack.StyleSheet{
		ID: styleDefinition.ID, Name: styleDefinition.Name, SystemPrompt: styleDefinition.SystemPrompt,
	}
	packet, manifest, err := s.builder.Build(contextpack.BuildRequest{
		Scope:     request.Target.Scope,
		Policy:    agentPolicy(agentDefinition),
		Budget:    agentBudget(agentDefinition),
		RAGMode:   contextpack.RAGMode(agentDefinition.RAGPolicy.Mode),
		Material:  material,
		Estimator: contextpack.ByteEstimator{},
	})
	if err != nil {
		return nil, contextpack.Manifest{}, story.ContextMaterialResult{}, err
	}
	return packet, manifest, materialResult, nil
}

func (s *Service) insertTaggedRun(ctx context.Context, request TaggedRunRequest, agentDefinition agent.Agent, manifest contextpack.Manifest, materialResult story.ContextMaterialResult, generated agent.GenerateResponse) (Run, error) {
	run := Run{
		AgentID:  agentDefinition.ID,
		StyleID:  request.StyleID,
		Scope:    request.Target.Scope,
		Manifest: manifest,
		Provider: generated.Provider,
	}
	switch request.Target.Scope {
	case contextpack.ScopeSelection:
		selection := request.Target.Selection
		scene, err := s.scenes.LoadScene(ctx, selection.SceneID)
		if err != nil {
			return Run{}, err
		}
		if scene.Revision != selection.SceneRevision {
			return Run{}, fmt.Errorf("scene %q revision changed: %w", selection.SceneID, story.ErrStaleRevision)
		}
		selected, err := story.ValidateMarkdownSelection(scene.Markdown, selection.StartByte, selection.EndByte, selection.SelectedText)
		if err != nil {
			return Run{}, err
		}
		replacement, err := validateGeneratedReplacement(generated.Replacement)
		if err != nil {
			return Run{}, err
		}
		if replacement == selected {
			return Run{}, story.ErrNoSceneChanges
		}
		run.Status = RunPending
		run.SceneID = selection.SceneID
		run.SceneRevision = selection.SceneRevision
		run.Selection = Selection{StartByte: selection.StartByte, EndByte: selection.EndByte}
		run.OriginalText = selected
		run.Replacement = replacement
		run.ContextSummary = manifestToContextSummary(manifest)
	case contextpack.ScopeScene:
		sceneMarkdown := materialResult.Material.SceneMarkdown
		replacement, err := validateGeneratedReplacement(generated.Replacement)
		if err != nil {
			return Run{}, err
		}
		if replacement == sceneMarkdown {
			return Run{}, story.ErrNoSceneChanges
		}
		run.Status = RunPending
		run.SceneID = request.Target.Scene.SceneID
		run.SceneRevision = request.Target.Scene.SceneRevision
		run.OriginalText = sceneMarkdown
		run.Replacement = replacement
	case contextpack.ScopeChapterReview:
		findings, invitations, err := s.parseChapterReviewOutput(generated.Replacement, agentDefinition, materialResult)
		if err != nil {
			return Run{}, err
		}
		run.Status = RunCompleted
		run.ChapterID = request.Target.Chapter.ChapterID
		run.ChapterFingerprint = request.Target.Chapter.Fingerprint
		run.Findings = findings.Findings
		inserted, err := s.insertRun(run)
		if err != nil {
			return Run{}, err
		}
		if _, err := s.publishPreparedInvitations(inserted, invitations); err != nil {
			return Run{}, err
		}
		return inserted, nil
	default:
		return Run{}, fmt.Errorf("scope %q is unsupported: %w", request.Target.Scope, ErrInvalidRunRequest)
	}
	return s.insertRun(run)
}

func (s *Service) parseChapterReviewOutput(raw string, agentDefinition agent.Agent, materialResult story.ContextMaterialResult) (FindingsResponse, []preparedInvitation, error) {
	allowedScenes := map[string]struct{}{}
	for _, scene := range materialResult.Material.ChapterScenes {
		allowedScenes[scene.SceneID] = struct{}{}
	}
	findings, err := ParseFindings(raw, allowedFollowUpsFromAgent(agentDefinition), allowedScenes)
	if err != nil {
		return FindingsResponse{}, nil, fmt.Errorf("%w: %w", ErrProviderInvalid, err)
	}
	return findings, findingsInvitations(findings), nil
}

func findingsInvitations(findings FindingsResponse) []preparedInvitation {
	seen := map[string]struct{}{}
	prepared := make([]preparedInvitation, 0)
	for _, finding := range findings.Findings {
		for _, agentID := range finding.FollowUpAgentIDs {
			key := agentID + "|scene|" + finding.SceneIDs[0]
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			prepared = append(prepared, preparedInvitation{
				AgentID: agentID, Scope: contextpack.ScopeScene, SceneID: finding.SceneIDs[0],
				Relationship: InvitationTriggered,
			})
		}
	}
	return prepared
}

func manifestToContextSummary(manifest contextpack.Manifest) agent.ContextSummary {
	packs := make([]agent.ContextPack, len(manifest.PacksUsed))
	for index, pack := range manifest.PacksUsed {
		packs[index] = agent.ContextPack(pack)
	}
	return agent.ContextSummary{PacksUsed: packs, RAGMode: agent.RAGMode(manifest.RAGMode)}
}

// AcceptBody applies one pending scene-scoped body patch to canonical markdown.
func (s *Service) AcceptBody(ctx context.Context, runID, expectedRevision string) (AcceptResult, error) {
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
	if run.Scope != contextpack.ScopeScene {
		_ = s.runs.ReleasePending(runID)
		return AcceptResult{}, fmt.Errorf("run %q is not a scene patch: %w", runID, ErrRunConflict)
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
	if s.bodyAcceptor == nil {
		_ = s.runs.ReleasePending(runID)
		return AcceptResult{}, fmt.Errorf("scene body acceptance is not configured")
	}
	scene, err := s.bodyAcceptor.AcceptSceneBodyPatch(ctx, story.AcceptSceneBodyPatchRequest{
		RunID: run.RunID, SceneID: run.SceneID, RunSceneRevision: run.SceneRevision,
		ExpectedRevision: expectedRevision, OriginalMarkdown: run.OriginalText,
		ReplacementMarkdown: run.Replacement, Operation: operation,
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

func invitationRelationshipForRun(run Run, parent *Run) InvitationRelationship {
	if run.ParentRelationship != "" {
		return run.ParentRelationship
	}
	return InvitationTriggered
}
