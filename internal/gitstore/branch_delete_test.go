// BDD Scenario: 8.5.1 - Discard the active experiment
// Requirements: M8-R17
// Test purpose: Delete experiment removes only the managed ref.

package gitstore_test

import (
	"testing"
)

// Test: delete inactive experiment leaves main untouched.
// Requirements: M8-R17.
func TestDeleteExperimentLeavesMainUntouched(t *testing.T) {
	t.Parallel()
	ctx, dir, store := initTestRepo(t)
	mainHead, err := store.ResolveCommit(ctx, dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	ref := "branch/test-exp-0123456789abcdef0123"
	if err := store.CreateAndSwitch(ctx, dir, ref, mainHead, mainHead); err != nil {
		t.Fatal(err)
	}
	experimentHead, err := store.ResolveCommit(ctx, dir, ref)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Switch(ctx, dir, "main"); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteExperiment(ctx, dir, ref, experimentHead, mainHead); err != nil {
		t.Fatalf("DeleteExperiment() error = %v", err)
	}
	if got, _ := store.ResolveCommit(ctx, dir, "main"); got != mainHead {
		t.Fatalf("main head changed: %q -> %q", mainHead, got)
	}
	if _, err := store.ResolveExperimentBase(ctx, dir, ref); err == nil {
		t.Fatal("ResolveExperimentBase() error = nil, want deleted ref")
	}
}
