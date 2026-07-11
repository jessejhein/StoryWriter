package action

// invitation_service.go integrates bounded invitation storage with action orchestration.

import (
	"context"
	"fmt"

	"storywork/internal/agent"
	"storywork/internal/contextpack"
	"storywork/internal/story"
)

type preparedInvitation struct {
	AgentID      string
	Scope        contextpack.Scope
	SceneID      string
	ChapterID    string
	Relationship InvitationRelationship
}

// WithInvitationStore injects the process-local invitation store.
func (s *Service) WithInvitationStore(store *InvitationStore) *Service {
	s.invitations = store
	return s
}

// WithInvitationIDGenerator injects the invitation ID generator.
func (s *Service) WithInvitationIDGenerator(generator InvitationIDGenerator) *Service {
	s.inviteIDs = generator
	return s
}

// RunInvitation executes one explicitly authorized follow-up invitation.
func (s *Service) RunInvitation(ctx context.Context, invitationID string, request InvitationRunRequest) (Run, error) {
	if err := validateInvitationID(invitationID); err != nil {
		return Run{}, err
	}
	if !registryRequestIDPattern.MatchString(request.StyleID) {
		return Run{}, fmt.Errorf("style_id %q is invalid: %w", request.StyleID, ErrInvalidRunRequest)
	}
	if err := story.ValidateRevision(request.ExpectedTargetRevision); err != nil {
		return Run{}, err
	}
	if s.invitations == nil {
		return Run{}, fmt.Errorf("invitation execution is not configured")
	}
	invitation, err := s.invitations.Claim(invitationID)
	if err != nil {
		return Run{}, err
	}
	if err := s.ensureInvitationSnapshotCurrent(ctx, invitation); err != nil {
		_ = s.invitations.Release(invitationID)
		return Run{}, err
	}
	release := true
	defer func() {
		if release {
			_ = s.invitations.Release(invitationID)
		}
	}()

	parent, ok := s.runs.Get(invitation.ParentRunID)
	if !ok {
		return Run{}, fmt.Errorf("parent run %q is unknown: %w", invitation.ParentRunID, ErrInvitationForbidden)
	}
	if parent.Status != RunAccepted && parent.Status != RunCompleted {
		return Run{}, fmt.Errorf("parent run %q is not terminal: %w", invitation.ParentRunID, ErrInvitationForbidden)
	}
	if invitation.ChainDepth >= maxInvitationChainDepth {
		return Run{}, fmt.Errorf("invitation chain depth exceeded: %w", ErrInvitationForbidden)
	}
	parentAgent, err := s.registryAgent(parent.AgentID)
	if err != nil {
		return Run{}, err
	}
	if !configuredInvitationTransition(parentAgent, invitation) {
		return Run{}, fmt.Errorf("invitation transition is no longer configured: %w", ErrInvitationForbidden)
	}

	target, err := s.targetForInvitation(invitation, request.ExpectedTargetRevision)
	if err != nil {
		return Run{}, err
	}
	agentDefinition, err := s.registryAgent(invitation.AgentID)
	if err != nil {
		return Run{}, err
	}
	if err := validateFollowUpTransition(parent.Scope, invitation.Scope); err != nil {
		return Run{}, err
	}
	tagged := TaggedRunRequest{AgentID: invitation.AgentID, StyleID: request.StyleID, Target: target}
	if err := validateAgentScope(agentDefinition, target); err != nil {
		return Run{}, err
	}

	child, err := s.RunTagged(ctx, tagged)
	if err != nil {
		return Run{}, err
	}
	child.ParentRunID = parent.RunID
	if parent.RootRunID != "" {
		child.RootRunID = parent.RootRunID
	} else {
		child.RootRunID = parent.RunID
	}
	child.ChainDepth = invitation.ChainDepth
	child.ParentRelationship = invitation.Relationship
	if err := s.runs.Update(child); err != nil {
		return Run{}, err
	}
	if err := s.invitations.Consume(invitationID); err != nil {
		return Run{}, err
	}
	release = false
	return child, nil
}

func configuredInvitationTransition(parent agent.Agent, invitation Invitation) bool {
	for _, rule := range parent.FollowUps.OnAccept {
		if rule.AgentID == invitation.AgentID && contextpack.Scope(rule.Scope) == invitation.Scope && InvitationRelationship(rule.Relationship) == invitation.Relationship {
			return true
		}
	}
	return false
}

func (s *Service) registryAgent(agentID string) (agent.Agent, error) {
	registry, err := s.registry()
	if err != nil {
		return agent.Agent{}, err
	}
	return findAgent(registry.Agents, agentID)
}

func (s *Service) targetForInvitation(invitation Invitation, expectedRevision string) (TaggedTarget, error) {
	switch invitation.Scope {
	case contextpack.ScopeScene:
		if invitation.SceneID == "" {
			return TaggedTarget{}, fmt.Errorf("invitation scene target is missing: %w", ErrInvitationInvalid)
		}
		return TaggedTarget{
			Scope: contextpack.ScopeScene,
			Scene: &SceneTarget{SceneID: invitation.SceneID, SceneRevision: expectedRevision},
		}, nil
	case contextpack.ScopeChapterReview:
		if invitation.ChapterID == "" {
			return TaggedTarget{}, fmt.Errorf("invitation chapter target is missing: %w", ErrInvitationInvalid)
		}
		return TaggedTarget{
			Scope:   contextpack.ScopeChapterReview,
			Chapter: &ChapterReviewTarget{ChapterID: invitation.ChapterID, Fingerprint: expectedRevision},
		}, nil
	case contextpack.ScopeSelection:
		return TaggedTarget{}, fmt.Errorf("selection invitations are unsupported: %w", ErrInvitationForbidden)
	default:
		return TaggedTarget{}, fmt.Errorf("scope %q is unsupported: %w", invitation.Scope, ErrInvitationInvalid)
	}
}

func (s *Service) publishFollowUpInvitations(run Run, state string) ([]PublishedInvitation, error) {
	if s.invitations == nil {
		return nil, nil
	}
	agentDefinition, err := s.registryAgent(run.AgentID)
	if err != nil {
		return nil, err
	}
	offers, err := DecideFollowUps(agentDefinition, state, run.Scope, run.effectiveChainDepth())
	if err != nil {
		return nil, err
	}
	prepared := s.fillInvitationTargets(run, offers)
	return s.publishPreparedInvitations(run, prepared)
}

func (s *Service) publishPreparedInvitations(run Run, prepared []preparedInvitation) ([]PublishedInvitation, error) {
	if len(prepared) == 0 || s.invitations == nil {
		return nil, nil
	}
	if s.inviteIDs == nil {
		return nil, fmt.Errorf("invitation ID generator is not configured")
	}
	ids := make([]string, len(prepared))
	for index := range prepared {
		id, err := s.inviteIDs.Next()
		if err != nil {
			return nil, err
		}
		ids[index] = id
	}
	rootRunID := run.RunID
	if run.RootRunID != "" {
		rootRunID = run.RootRunID
	}
	chainDepth := run.effectiveChainDepth() + 1
	invitationBranch := run.Branch
	invitationHead := run.BranchHead
	if s.branches != nil && (run.Status == RunAccepted || invitationBranch == "" || invitationHead == "") {
		snapshot, err := s.branches.Snapshot(context.Background())
		if err != nil {
			return nil, err
		}
		invitationBranch = snapshot.Branch
		invitationHead = snapshot.Head
	}
	batch := make([]Invitation, 0, len(prepared))
	for index, offer := range prepared {
		invitation := Invitation{
			ID: ids[index], ParentRunID: run.RunID, RootRunID: rootRunID, ChainDepth: chainDepth,
			AgentID: offer.AgentID, Scope: offer.Scope, SceneID: offer.SceneID, ChapterID: offer.ChapterID,
			Relationship: offer.Relationship, Branch: invitationBranch, BranchHead: invitationHead,
		}
		batch = append(batch, invitation)
	}
	if err := s.invitations.PublishBatch(batch); err != nil {
		return nil, err
	}
	published := make([]PublishedInvitation, 0, len(batch))
	for _, invitation := range batch {
		published = append(published, publishedInvitation(invitation))
	}
	return published, nil
}

func publishedInvitation(invitation Invitation) PublishedInvitation {
	return PublishedInvitation{
		InvitationID: invitation.ID, ParentRunID: invitation.ParentRunID, RootRunID: invitation.RootRunID,
		ChainDepth: invitation.ChainDepth, AgentID: invitation.AgentID, Scope: string(invitation.Scope),
		SceneID: invitation.SceneID, ChapterID: invitation.ChapterID, Relationship: string(invitation.Relationship),
	}
}

func (s *Service) fillInvitationTargets(run Run, offers []FollowUpOffer) []preparedInvitation {
	prepared := make([]preparedInvitation, len(offers))
	for index, offer := range offers {
		item := preparedInvitation{
			AgentID: offer.AgentID, Scope: offer.Scope, Relationship: offer.Relationship,
		}
		switch offer.Scope {
		case contextpack.ScopeScene:
			item.SceneID = run.SceneID
		case contextpack.ScopeChapterReview:
			item.ChapterID = s.chapterIDForRun(run)
		}
		prepared[index] = item
	}
	return prepared
}

func (s *Service) chapterIDForRun(run Run) string {
	if run.ChapterID != "" {
		return run.ChapterID
	}
	if run.SceneID == "" || s.scenes == nil {
		return ""
	}
	scene, err := s.scenes.LoadScene(context.Background(), run.SceneID)
	if err != nil {
		return ""
	}
	return scene.ChapterID
}

func (run Run) effectiveChainDepth() int {
	if run.ChainDepth > 0 {
		return run.ChainDepth
	}
	return 1
}
