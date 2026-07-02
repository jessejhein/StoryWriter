package action

// invitation.go implements pure follow-up policy and bounded invitation storage.

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"sync"

	"storywork/internal/agent"
	"storywork/internal/contextpack"
)

const maxInvitationChainDepth = 3

var (
	ErrInvitationNotFound  = errors.New("action invitation not found")
	ErrInvitationConflict  = errors.New("action invitation conflict")
	ErrInvitationInvalid   = errors.New("invalid action invitation")
	ErrInvitationForbidden = errors.New("forbidden action invitation")
	invitationIDPattern    = regexp.MustCompile(`^invite_[0-9a-f]{20}$`)
)

// InvitationRelationship names causality versus semantic dependency.
type InvitationRelationship string

const (
	InvitationTriggered InvitationRelationship = "triggered"
	InvitationDependsOn InvitationRelationship = "depends_on"
)

// Invitation is one process-local follow-up offer.
type Invitation struct {
	ID           string
	ParentRunID  string
	RootRunID    string
	ChainDepth   int
	AgentID      string
	Scope        contextpack.Scope
	SceneID      string
	ChapterID    string
	Relationship InvitationRelationship
	Status       string
}

// InvitationRunRequest authorizes one explicit invitation execution.
type InvitationRunRequest struct {
	StyleID                string
	ExpectedTargetRevision string
}

// FollowUpOffer is one pure policy decision before storage.
type FollowUpOffer struct {
	AgentID      string
	Scope        contextpack.Scope
	SceneID      string
	ChapterID    string
	Relationship InvitationRelationship
}

// DecideFollowUps evaluates configured follow-up rules for one lifecycle state.
func DecideFollowUps(agentDefinition agent.Agent, state string, parentScope contextpack.Scope, chainDepth int) ([]FollowUpOffer, error) {
	if chainDepth >= maxInvitationChainDepth {
		return nil, nil
	}
	var rules []agent.FollowUpRule
	switch state {
	case "accept":
		rules = agentDefinition.FollowUps.OnAccept
	default:
		return nil, nil
	}
	offers := make([]FollowUpOffer, 0, len(rules))
	seen := map[string]struct{}{}
	for _, rule := range rules {
		scope := contextpack.Scope(rule.Scope)
		if err := validateFollowUpTransition(parentScope, scope); err != nil {
			return nil, err
		}
		key := rule.AgentID + "|" + string(scope)
		if _, ok := seen[key]; ok {
			return nil, fmt.Errorf("duplicate follow-up %q: %w", key, ErrInvitationForbidden)
		}
		seen[key] = struct{}{}
		offers = append(offers, FollowUpOffer{
			AgentID: rule.AgentID, Scope: scope,
			Relationship: InvitationRelationship(rule.Relationship),
		})
	}
	sort.Slice(offers, func(i, j int) bool {
		if offers[i].AgentID != offers[j].AgentID {
			return offers[i].AgentID < offers[j].AgentID
		}
		return offers[i].Scope < offers[j].Scope
	})
	return offers, nil
}

func validateFollowUpTransition(parentScope, childScope contextpack.Scope) error {
	if childScope == parentScope {
		return nil
	}
	allowed := map[contextpack.Scope]map[contextpack.Scope]struct{}{
		contextpack.ScopeSelection:     {contextpack.ScopeScene: {}, contextpack.ScopeChapterReview: {}},
		contextpack.ScopeScene:         {contextpack.ScopeChapterReview: {}},
		contextpack.ScopeChapterReview: {},
	}
	if next, ok := allowed[parentScope]; !ok {
		return fmt.Errorf("unsupported parent scope %q: %w", parentScope, ErrInvitationForbidden)
	} else if _, ok := next[childScope]; !ok {
		return fmt.Errorf("scope transition %q -> %q is unsupported: %w", parentScope, childScope, ErrInvitationForbidden)
	}
	return nil
}

// InvitationStore retains bounded process-local invitations.
type InvitationStore struct {
	mu          sync.Mutex
	invitations map[string]Invitation
	order       []string
	limit       int
}

// NewInvitationStore creates one bounded invitation store.
func NewInvitationStore(limit int) *InvitationStore {
	if limit <= 0 {
		limit = 1000
	}
	return &InvitationStore{invitations: make(map[string]Invitation), limit: limit}
}

// Publish stores one validated invitation.
func (s *InvitationStore) Publish(invitation Invitation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := validateInvitationID(invitation.ID); err != nil {
		return err
	}
	if _, exists := s.invitations[invitation.ID]; exists {
		return ErrInvitationConflict
	}
	s.evictTerminalLocked()
	if len(s.invitations) >= s.limit {
		return ErrRunCapacity
	}
	invitation.Status = "offered"
	s.invitations[invitation.ID] = invitation
	s.order = append(s.order, invitation.ID)
	return nil
}

// Claim marks one invitation claimed for execution.
func (s *InvitationStore) Claim(invitationID string) (Invitation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	invitation, ok := s.invitations[invitationID]
	if !ok {
		return Invitation{}, ErrInvitationNotFound
	}
	if invitation.Status != "offered" {
		return Invitation{}, ErrInvitationConflict
	}
	invitation.Status = "claimed"
	s.invitations[invitationID] = invitation
	return invitation, nil
}

// Release returns one claimed invitation to offered state.
func (s *InvitationStore) Release(invitationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	invitation, ok := s.invitations[invitationID]
	if !ok {
		return ErrInvitationNotFound
	}
	if invitation.Status != "claimed" {
		return ErrInvitationConflict
	}
	invitation.Status = "offered"
	s.invitations[invitationID] = invitation
	return nil
}

// Consume marks one claimed invitation consumed.
func (s *InvitationStore) Consume(invitationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	invitation, ok := s.invitations[invitationID]
	if !ok {
		return ErrInvitationNotFound
	}
	if invitation.Status != "claimed" {
		return ErrInvitationConflict
	}
	invitation.Status = "consumed"
	s.invitations[invitationID] = invitation
	return nil
}

func (s *InvitationStore) Get(invitationID string) (Invitation, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	invitation, ok := s.invitations[invitationID]
	return invitation, ok
}

func (s *InvitationStore) evictTerminalLocked() {
	for _, id := range append([]string(nil), s.order...) {
		invitation := s.invitations[id]
		if invitation.Status == "consumed" {
			delete(s.invitations, id)
			s.order = removeID(s.order, id)
			return
		}
	}
}

// ValidateInvitationID validates invitation identifier syntax.
func ValidateInvitationID(id string) error {
	return validateInvitationID(id)
}

func validateInvitationID(id string) error {
	if !invitationIDPattern.MatchString(id) {
		return fmt.Errorf("invitation_id %q is invalid: %w", id, ErrInvitationInvalid)
	}
	return nil
}

// NewInvitationID returns one random invitation identifier.
func NewInvitationID() (string, error) {
	var raw [10]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate invitation ID: %w", err)
	}
	return "invite_" + hex.EncodeToString(raw[:]), nil
}
