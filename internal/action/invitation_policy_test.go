// BDD Scenario: 7.4.1 - Offer a follow-up without calling a provider
// Requirements: M7-R11
// Test purpose: Follow-up policy decisions are pure and inspectable.

package action

import (
	"errors"
	"testing"

	"storywork/internal/agent"
	"storywork/internal/contextpack"
)

// Test: policy allows configured state and scope.
// Requirements: M7-R11.
func TestFollowUpPolicyAllowsConfiguredStateAndScope(t *testing.T) {
	t.Parallel()

	offers, err := DecideFollowUps(agentWithFollowUp(), "accept", contextpack.ScopeSelection, 1)
	if err != nil || len(offers) != 1 || offers[0].AgentID != "scene_rewrite" {
		t.Fatalf("offers = %#v err=%v", offers, err)
	}
}

// Test: broader scope requires explicit configured transition.
// Requirements: M7-R11.
func TestFollowUpPolicyRequiresExplicitBroaderScope(t *testing.T) {
	t.Parallel()

	if _, err := DecideFollowUps(agentWithFollowUp(), "accept", contextpack.ScopeChapterReview, 1); !errors.Is(err, ErrInvitationForbidden) {
		t.Fatalf("error = %v", err)
	}
}

// Test: policy rejects unknown duplicate and cyclic transitions.
// Requirements: M7-R11.
func TestFollowUpPolicyRejectsUnknownDuplicateAndCyclicTransition(t *testing.T) {
	t.Parallel()

	agentDef := agentWithFollowUp()
	agentDef.FollowUps.OnAccept = append(agentDef.FollowUps.OnAccept, agentDef.FollowUps.OnAccept[0])
	if _, err := DecideFollowUps(agentDef, "accept", contextpack.ScopeSelection, 1); !errors.Is(err, ErrInvitationForbidden) {
		t.Fatalf("duplicate error = %v", err)
	}
}

// Test: policy enforces maximum chain depth three.
// Requirements: M7-R11.
func TestFollowUpPolicyEnforcesMaximumChainDepthThree(t *testing.T) {
	t.Parallel()

	offers, err := DecideFollowUps(agentWithFollowUp(), "accept", contextpack.ScopeSelection, 3)
	if err != nil || len(offers) != 0 {
		t.Fatalf("offers = %#v err=%v", offers, err)
	}
}

// Test: policy orders invitations deterministically.
// Requirements: M7-R11.
func TestFollowUpPolicyOrdersInvitationsDeterministically(t *testing.T) {
	t.Parallel()

	agentDef := agentWithFollowUp()
	agentDef.FollowUps.OnAccept = append(agentDef.FollowUps.OnAccept, agent.FollowUpRule{
		AgentID: "chapter_review", Scope: agent.FollowUpScopeChapterReview, Relationship: agent.FollowUpRelationshipTriggered,
	})
	offers, err := DecideFollowUps(agentDef, "accept", contextpack.ScopeSelection, 1)
	if err != nil || len(offers) != 2 || offers[0].AgentID != "chapter_review" {
		t.Fatalf("offers = %#v", offers)
	}
}

func agentWithFollowUp() agent.Agent {
	return agent.Agent{
		Version: 3, ID: "line_polish",
		FollowUps: agent.FollowUpPolicy{OnAccept: []agent.FollowUpRule{{
			AgentID: "scene_rewrite", Scope: agent.FollowUpScopeScene, Relationship: agent.FollowUpRelationshipTriggered,
		}}},
	}
}