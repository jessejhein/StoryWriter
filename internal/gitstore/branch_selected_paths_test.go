// BDD Scenario: 8.4.2 - Reject a path changed on canon
// Requirements: M8-R12, M8-R13
// Test purpose: Path-limited main divergence checks only diff the selected
// paths and keep unrelated canon-only changes out of promotion conflicts.

package gitstore_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"storywork/internal/gitstore"
)

// Test: selected-path comparison uses `--` before pathspecs and ignores
// unrelated canon-only changes.
// Requirements: M8-R12, M8-R13.
func TestSelectedPathsChangedUsesPathspecFilter(t *testing.T) {
	t.Parallel()
	ctx, dir, store := initTestRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, "scenes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scenes", "scn_00000000000000000001.md"), []byte("---\nid: scn_00000000000000000001\n---\n\ncanon\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.CommitAll(ctx, dir, "add scene"); err != nil {
		t.Fatal(err)
	}
	mainHead, err := store.ResolveCommit(ctx, dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	ref := "branch/pathspec-0123456789abcdef0123"
	if err := store.CreateAndSwitch(ctx, dir, ref, mainHead, mainHead); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scenes", "scn_00000000000000000001.md"), []byte("---\nid: scn_00000000000000000001\n---\n\nexperiment\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.CommitAll(ctx, dir, "experiment scene"); err != nil {
		t.Fatal(err)
	}
	experimentHead, err := store.ResolveCommit(ctx, dir, ref)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Switch(ctx, dir, "main"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "project.yaml"), []byte("version: 1\nid: proj_test\nname: Changed on main\ncreated_at: \"2026-07-03T00:00:00Z\"\ndefault_branch: main\nsettings:\n  prose_format: markdown\n  vim_mode_default: true\n  ai_mutation_requires_acceptance: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.CommitAll(ctx, dir, "main-only project change"); err != nil {
		t.Fatal(err)
	}
	currentMain, err := store.ResolveCommit(ctx, dir, "main")
	if err != nil {
		t.Fatal(err)
	}

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(t.TempDir(), "git-args.log")
	scriptPath := filepath.Join(t.TempDir(), "git-wrapper.sh")
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s ' \"$@\" >> %q\nprintf '\\n' >> %q\nexec %q \"$@\"\n", logPath, logPath, realGit)
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	wrapped := gitstore.New(scriptPath)
	paths, err := wrapped.SelectedPathsChanged(ctx, dir, currentMain, experimentHead, []string{"scenes/scn_00000000000000000001.md"})
	if err != nil {
		t.Fatalf("SelectedPathsChanged() error = %v", err)
	}
	if len(paths) != 1 || paths[0] != "scenes/scn_00000000000000000001.md" {
		t.Fatalf("paths = %#v", paths)
	}
	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	log := string(logBytes)
	if !strings.Contains(log, "diff --no-renames --name-only -z "+currentMain+" "+experimentHead+" -- scenes/scn_00000000000000000001.md") {
		t.Fatalf("git args did not include selected pathspec filter: %q", log)
	}
	if strings.Contains(log, "project.yaml") {
		t.Fatalf("git args leaked unrelated canon-only path: %q", log)
	}
}
