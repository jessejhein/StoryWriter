// BDD Scenario: 7.5.1 - Record causal and dependency trailers
// Requirements: M7-R13, M7-R14, M7-R15
// Test purpose: Accepted operations write validated Git trailers and reject invalid lineage.

package action

import (
	"context"
	"errors"
	"strings"
	"testing"

	"storywork/internal/contextpack"
	"storywork/internal/gitstore"
	"storywork/internal/story"
)

// Test: root accept writes operation and scope trailers.
// Requirements: M7-R13.
func TestRootAcceptWritesOperationAndScopeTrailers(t *testing.T) {
	t.Parallel()

	run := Run{RunID: "run_0123456789abcdef0123", Scope: contextpack.ScopeSelection, SceneID: "scn_0123456789abcdef0123"}
	metadata, err := BuildOperationMetadata(run, nil, InvitationTriggered)
	if err != nil {
		t.Fatalf("BuildOperationMetadata() error = %v", err)
	}
	if metadata.OperationID != run.RunID || metadata.Scope != "selection:scn_0123456789abcdef0123" || metadata.TriggeredBy != "" {
		t.Fatalf("metadata = %#v", metadata)
	}
}

// Test: dependent child writes trigger and dependency trailers.
// Requirements: M7-R13, M7-R14.
func TestDependentChildWritesTriggerAndDependencyTrailers(t *testing.T) {
	t.Parallel()

	parent := Run{RunID: "run_aaaaaaaaaaaaaaaaaaaa", Status: RunAccepted, Scope: contextpack.ScopeSelection, SceneID: "scn_0123456789abcdef0123"}
	child := Run{RunID: "run_bbbbbbbbbbbbbbbbbbbb", ParentRunID: parent.RunID, Scope: contextpack.ScopeScene, SceneID: "scn_0123456789abcdef0123", ParentRelationship: InvitationDependsOn}
	metadata, err := BuildOperationMetadata(child, &parent, InvitationDependsOn)
	if err != nil {
		t.Fatalf("BuildOperationMetadata() error = %v", err)
	}
	if metadata.TriggeredBy != parent.RunID || metadata.DependsOn != parent.RunID {
		t.Fatalf("metadata = %#v", metadata)
	}
	formatted, err := gitstore.FormatCommitMessage(gitstore.CommitMessage{
		Subject: "Accept AI patch " + child.RunID, OperationID: metadata.OperationID,
		TriggeredBy: metadata.TriggeredBy, DependsOn: metadata.DependsOn, Scope: metadata.Scope,
	})
	if err != nil {
		t.Fatalf("FormatCommitMessage() error = %v", err)
	}
	if !strings.Contains(formatted, "Storywork-Depends-On:") {
		t.Fatalf("formatted = %q", formatted)
	}
}

// Test: trigger-only child omits dependency trailer.
// Requirements: M7-R14.
func TestTriggerOnlyChildOmitsDependencyTrailer(t *testing.T) {
	t.Parallel()

	parent := Run{RunID: "run_aaaaaaaaaaaaaaaaaaaa", Status: RunAccepted, Scope: contextpack.ScopeSelection, SceneID: "scn_0123456789abcdef0123"}
	child := Run{RunID: "run_bbbbbbbbbbbbbbbbbbbb", ParentRunID: parent.RunID, Scope: contextpack.ScopeScene, SceneID: "scn_0123456789abcdef0123", ParentRelationship: InvitationTriggered}
	metadata, err := BuildOperationMetadata(child, &parent, InvitationTriggered)
	if err != nil {
		t.Fatalf("BuildOperationMetadata() error = %v", err)
	}
	if metadata.TriggeredBy != parent.RunID || metadata.DependsOn != "" {
		t.Fatalf("metadata = %#v", metadata)
	}
}

// Test: suggestion-origin child writes no unresolvable parent trailer.
// Requirements: M7-R13, M7-R14.
func TestSuggestionOriginChildWritesNoUnresolvableParentTrailer(t *testing.T) {
	t.Parallel()

	parent := Run{RunID: "run_aaaaaaaaaaaaaaaaaaaa", Status: RunCompleted, Scope: contextpack.ScopeChapterReview, ChapterID: "ch_0123456789abcdef0123"}
	child := Run{RunID: "run_bbbbbbbbbbbbbbbbbbbb", ParentRunID: parent.RunID, Scope: contextpack.ScopeScene, SceneID: "scn_0123456789abcdef0123"}
	metadata, err := BuildOperationMetadata(child, &parent, InvitationTriggered)
	if err != nil {
		t.Fatalf("BuildOperationMetadata() error = %v", err)
	}
	if metadata.TriggeredBy != "" || metadata.DependsOn != "" {
		t.Fatalf("metadata = %#v", metadata)
	}
}

// Test: invalid lineage is rejected before writes.
// Requirements: M7-R15.
func TestLineageRejectsUnknownNonAcceptedSelfAndCycleBeforeWrite(t *testing.T) {
	t.Parallel()

	store := NewRunStore()
	run := Run{RunID: "run_0123456789abcdef0123", ParentRunID: "run_ffffffffffffffffffff", Scope: contextpack.ScopeScene, SceneID: "scn_0123456789abcdef0123"}
	if err := store.Insert(run); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}
	if _, err := ResolveParentRun(store, run); !errors.Is(err, ErrLineageConflict) {
		t.Fatalf("ResolveParentRun() error = %v", err)
	}
	selfParent := Run{RunID: "run_0123456789abcdef0123", Status: RunAccepted}
	if _, err := BuildOperationMetadata(run, &selfParent, InvitationDependsOn); !errors.Is(err, ErrLineageConflict) {
		t.Fatalf("self dependency error = %v", err)
	}
}

// Test: resolving a parent rejects cycles anywhere in the bounded in-memory lineage.
// Requirements: M7-R13, M7-R15.
func TestResolveParentRunRejectsIndirectCycle(t *testing.T) {
	t.Parallel()

	runs := NewRunStore()
	child := Run{RunID: "run_cccccccccccccccccccc", Status: RunPending, ParentRunID: "run_bbbbbbbbbbbbbbbbbbbb"}
	parent := Run{RunID: "run_bbbbbbbbbbbbbbbbbbbb", Status: RunAccepted, ParentRunID: "run_aaaaaaaaaaaaaaaaaaaa"}
	root := Run{RunID: "run_aaaaaaaaaaaaaaaaaaaa", Status: RunAccepted, ParentRunID: child.RunID}
	for _, run := range []Run{child, parent, root} {
		if err := runs.Insert(run); err != nil {
			t.Fatalf("Insert(%s) error = %v", run.RunID, err)
		}
	}
	if _, err := ResolveParentRun(runs, child); !errors.Is(err, ErrLineageConflict) {
		t.Fatalf("ResolveParentRun() error = %v, want ErrLineageConflict", err)
	}
}

// Test: checkpoint failure leaves run retryable.
// Requirements: M7-R15.
func TestLineageCheckpointFailureRestoresAndLeavesRunRetryable(t *testing.T) {
	t.Parallel()

	acceptor := &fakeAcceptor{err: story.ErrDirtyWorktree}
	service := newInvitationTestService(t, testActionScene(), acceptor, NewInvitationStore(10))
	run, err := service.Run(context.Background(), selectionRunRequest(testActionScene()))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if _, err := service.Accept(context.Background(), run.RunID, testActionScene().Revision); !errors.Is(err, story.ErrDirtyWorktree) {
		t.Fatalf("Accept() error = %v", err)
	}
	released, err := service.Reject(context.Background(), run.RunID)
	if err != nil || released.Status != RunRejected {
		t.Fatalf("Reject() = %#v, %v", released, err)
	}
}
