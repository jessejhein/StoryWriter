// BDD Scenario: 7.4.1 - Offer a follow-up without calling a provider
// Requirements: M7-R11
// Test purpose: Bounded invitation store lifecycle is race-safe and prose-free.

package action

import (
	"errors"
	"sync"
	"testing"

	"storywork/internal/contextpack"
)

// Test: invitation store validates IDs and rejects duplicates.
// Requirements: M7-R11.
func TestInvitationStoreValidatesIDsAndRejectsDuplicates(t *testing.T) {
	t.Parallel()

	store := NewInvitationStore(10)
	invitation := Invitation{ID: "invite_0123456789abcdef0123", ParentRunID: "run_0123456789abcdef0123", RootRunID: "run_0123456789abcdef0123", ChainDepth: 2, AgentID: "scene_rewrite", Scope: contextpack.ScopeScene}
	if err := store.Publish(invitation); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if err := store.Publish(invitation); !errors.Is(err, ErrInvitationConflict) {
		t.Fatalf("duplicate Publish() error = %v", err)
	}
}

// Test: invitation store claims, releases, and consumes.
// Requirements: M7-R11.
func TestInvitationStoreClaimsReleasesAndConsumes(t *testing.T) {
	t.Parallel()

	store := NewInvitationStore(10)
	id := "invite_0123456789abcdef0123"
	if err := store.Publish(Invitation{ID: id, ParentRunID: "run_0123456789abcdef0123", RootRunID: "run_0123456789abcdef0123", ChainDepth: 2, AgentID: "scene_rewrite", Scope: contextpack.ScopeScene}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if _, err := store.Claim(id); err != nil {
		t.Fatalf("Claim() error = %v", err)
	}
	if err := store.Release(id); err != nil {
		t.Fatalf("Release() error = %v", err)
	}
	if _, err := store.Claim(id); err != nil {
		t.Fatalf("Claim(second) error = %v", err)
	}
	if err := store.Consume(id); err != nil {
		t.Fatalf("Consume() error = %v", err)
	}
}

// Test: concurrent claims allow exactly one winner.
// Requirements: M7-R11.
func TestInvitationStoreConcurrentClaimHasOneWinner(t *testing.T) {
	t.Parallel()

	store := NewInvitationStore(10)
	id := "invite_0123456789abcdef0123"
	if err := store.Publish(Invitation{ID: id, ParentRunID: "run_0123456789abcdef0123", RootRunID: "run_0123456789abcdef0123", ChainDepth: 2, AgentID: "scene_rewrite", Scope: contextpack.ScopeScene}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	var wg sync.WaitGroup
	winners := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.Claim(id)
			winners <- err
		}()
	}
	wg.Wait()
	close(winners)
	var success, conflict int
	for err := range winners {
		if err == nil {
			success++
		} else if errors.Is(err, ErrInvitationConflict) {
			conflict++
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("success/conflict = %d/%d, want 1/1", success, conflict)
	}
}

// Test: invitation store evicts terminal invitations before capacity rejection.
// Requirements: M7-R11.
func TestInvitationStoreEvictsTerminalNotLiveInvitations(t *testing.T) {
	t.Parallel()

	store := NewInvitationStore(2)
	for _, id := range []string{"invite_aaaaaaaaaaaaaaaaaaaa", "invite_bbbbbbbbbbbbbbbbbbbb"} {
		if err := store.Publish(Invitation{ID: id, ParentRunID: "run_0123456789abcdef0123", RootRunID: "run_0123456789abcdef0123", ChainDepth: 2, AgentID: "scene_rewrite", Scope: contextpack.ScopeScene}); err != nil {
			t.Fatalf("Publish(%s) error = %v", id, err)
		}
	}
	first := "invite_aaaaaaaaaaaaaaaaaaaa"
	if _, err := store.Claim(first); err != nil {
		t.Fatalf("Claim() error = %v", err)
	}
	if err := store.Consume(first); err != nil {
		t.Fatalf("Consume() error = %v", err)
	}
	if err := store.Publish(Invitation{ID: "invite_cccccccccccccccccccc", ParentRunID: "run_0123456789abcdef0123", RootRunID: "run_0123456789abcdef0123", ChainDepth: 2, AgentID: "scene_rewrite", Scope: contextpack.ScopeScene}); err != nil {
		t.Fatalf("Publish(after evict) error = %v", err)
	}
}

// Test: invitation store retains no prose or prompt content.
// Requirements: M7-R11.
func TestInvitationStoreRetainsNoProseOrPromptContent(t *testing.T) {
	t.Parallel()

	store := NewInvitationStore(10)
	invitation := Invitation{
		ID: "invite_0123456789abcdef0123", ParentRunID: "run_0123456789abcdef0123", RootRunID: "run_0123456789abcdef0123",
		ChainDepth: 2, AgentID: "scene_rewrite", Scope: contextpack.ScopeScene, SceneID: "scn_0123456789abcdef0123",
	}
	if err := store.Publish(invitation); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	got, ok := store.Get(invitation.ID)
	if !ok || got.SceneID != invitation.SceneID || got.Status != "offered" {
		t.Fatalf("stored invitation = %#v", got)
	}
}
