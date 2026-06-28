package workspace_test

import (
	"sync"
	"testing"

	"storywork/internal/project"
	"storywork/internal/workspace"
)

// BDD trace:
//   - Requirement: Milestone 1 fixed design decision, active project session.
//   - Scenario: after create or open succeeds, the backend keeps exactly one
//     active project for outline routes until it is replaced or the process
//     restarts.
//   - Test purpose: verify empty state, replacement, and concurrent reads all
//     observe the current active project safely.
func TestSessionTracksCurrentProject(t *testing.T) {
	t.Parallel()

	session := workspace.NewSession()
	if _, ok := session.Current(); ok {
		t.Fatal("Current() ok = true, want false for empty session")
	}

	first := project.Project{ID: "proj_first", Path: "/tmp/first"}
	session.Set(first)
	current, ok := session.Current()
	if !ok {
		t.Fatal("Current() ok = false, want true after Set()")
	}
	if current != first {
		t.Fatalf("Current() = %#v, want %#v", current, first)
	}

	second := project.Project{ID: "proj_second", Path: "/tmp/second"}
	session.Set(second)
	current, ok = session.Current()
	if !ok {
		t.Fatal("Current() ok = false after replacement")
	}
	if current != second {
		t.Fatalf("Current() = %#v, want %#v", current, second)
	}

	var wait sync.WaitGroup
	for range 32 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			got, ok := session.Current()
			if !ok {
				t.Error("Current() ok = false during concurrent read")
				return
			}
			if got != second {
				t.Errorf("Current() = %#v, want %#v", got, second)
			}
		}()
	}
	wait.Wait()
}
