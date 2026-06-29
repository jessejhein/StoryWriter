// BDD Scenario: 3.2.1 - Save ordered progressions
// Requirements: M3-R02, M3-R03, M3-R05, M3-R07, M3-R08, M3-R13, M3-R14, M3-R15, M3-R17, M3-R18, M3-R21
// Test purpose: The real-adapter Milestone 3 acceptance path proves exact canonical Codex/progression bytes, one-commit checkpoints, stable anchors after reorder, full index manifest coverage, and zero side effects for stale/dirty mutations across a fresh service reload.
package story_test

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"storywork/internal/codex"
	"storywork/internal/gitstore"
	"storywork/internal/index"
	"storywork/internal/project"
	"storywork/internal/story"
	"storywork/internal/storyfile"
	"storywork/internal/workspace"
)

func TestMilestone3CodexWithRealAdapters(t *testing.T) {
	t.Parallel()

	// Test: the real-adapter Milestone 3 acceptance path preserves exact canonical Codex bytes, stable anchors, clean checkpoints, and stale/dirty protections across reload.
	// Requirements: M3-R02, M3-R03, M3-R05, M3-R07, M3-R08, M3-R15, M3-R17, M3-R21
	ctx := context.Background()
	projectPath := filepath.Join(t.TempDir(), "codex-novel")
	git := gitstore.New("git")
	disposableIndex := index.New()
	projectService := project.NewService(
		git,
		disposableIndex,
		func() time.Time { return time.Date(2026, time.June, 28, 12, 0, 0, 0, time.UTC) },
	)
	created, err := projectService.Create(ctx, project.CreateRequest{Name: "Codex Novel", Path: projectPath})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	session := workspace.NewSession()
	session.Set(created)
	service := story.NewService(
		session,
		storyfile.New(),
		git,
		disposableIndex,
		&staticIDGenerator{ids: []string{
			"arc_00000000000000000001",
			"ch_00000000000000000001",
			"scn_00000000000000000001",
			"scn_00000000000000000002",
			"scn_00000000000000000003",
			"char_0123456789abcdef0123",
			"prog_0123456789abcdef0123",
			"prog_0123456789abcdef0124",
		}},
	)

	if _, err := service.CreateArc(ctx, "Act One"); err != nil {
		t.Fatalf("CreateArc() error = %v", err)
	}
	if _, err := service.CreateChapter(ctx, "arc_00000000000000000001", "Arrival"); err != nil {
		t.Fatalf("CreateChapter() error = %v", err)
	}
	if _, err := service.CreateScene(ctx, "ch_00000000000000000001", "Docking"); err != nil {
		t.Fatalf("CreateScene(Docking) error = %v", err)
	}
	if _, err := service.CreateScene(ctx, "ch_00000000000000000001", "Debrief"); err != nil {
		t.Fatalf("CreateScene(Debrief) error = %v", err)
	}
	if _, err := service.CreateScene(ctx, "ch_00000000000000000001", "Aftermath"); err != nil {
		t.Fatalf("CreateScene(Aftermath) error = %v", err)
	}

	createdEntry, err := service.CreateCodexEntry(ctx, codex.SaveEntryRequest{
		Type:        codex.TypeCharacter,
		Name:        "Obi-Wan Kenobi",
		Aliases:     []string{"Ben"},
		Tags:        []string{"mentor", "jedi"},
		Description: "Guide.\n",
		Metadata:    map[string]string{"status": "alive"},
	})
	if err != nil {
		t.Fatalf("CreateCodexEntry() error = %v", err)
	}
	entryPath := filepath.Join(projectPath, "codex", "characters", createdEntry.ID+".yaml")
	entryBytes, err := os.ReadFile(entryPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", entryPath, err)
	}
	// Test: the canonical entry file matches the exact bytes the marshaled revision claims (acceptance step 2: verify exact canonical bytes).
	// Requirements: M3-R04, M3-R15
	if want := createdEntry.Canonical; !bytesEqual(entryBytes, want) {
		t.Fatalf("entry canonical bytes mismatch:\nwant:\n%s\ngot:\n%s", want, entryBytes)
	}
	if createdEntry.Revision != codex.ComputeRevision(entryBytes) {
		t.Fatalf("created entry revision = %q, want %q", createdEntry.Revision, codex.ComputeRevision(entryBytes))
	}

	updatedEntry, err := service.UpdateCodexEntry(ctx, createdEntry.ID, codex.SaveEntryRequest{
		Name:             "Obi-Wan Kenobi",
		Aliases:          []string{"Ben"},
		Tags:             []string{"mentor", "jedi"},
		Description:      "Gone, but influential.\n",
		Metadata:         map[string]string{"status": "alive", "role": "mentor"},
		ExpectedRevision: createdEntry.Revision,
	})
	if err != nil {
		t.Fatalf("UpdateCodexEntry() error = %v", err)
	}
	if updatedEntry.ID != createdEntry.ID || updatedEntry.Type != createdEntry.Type {
		t.Fatalf("updated entry = %#v", updatedEntry)
	}
	updatedEntryBytes, err := os.ReadFile(entryPath)
	if err != nil {
		t.Fatalf("ReadFile(updated %s) error = %v", entryPath, err)
	}
	// Test: the edited entry's ID and type are unchanged and the on-disk bytes match the marshaled canonical bytes (acceptance step 3).
	// Requirements: M3-R03, M3-R15
	if !bytesEqual(updatedEntryBytes, updatedEntry.Canonical) {
		t.Fatalf("updated entry canonical bytes mismatch:\nwant:\n%s\ngot:\n%s", updatedEntry.Canonical, updatedEntryBytes)
	}
	if updatedEntry.Revision == createdEntry.Revision {
		t.Fatalf("updated entry revision unchanged from %q", updatedEntry.Revision)
	}

	beforeDescription := "Already gone.\n"
	afterDescription := "Legend.\n"
	savedProgressions, err := service.SaveProgressions(ctx, createdEntry.ID, codex.SaveProgressionsRequest{
		Progressions: []codex.Progression{
			{
				Anchor:  codex.ProgressionAnchor{Type: "scene", ID: "scn_00000000000000000002", Timing: "before"},
				Changes: codex.ProgressionChange{Description: &beforeDescription, Metadata: map[string]string{"status": "deceased"}},
			},
			{
				Anchor:  codex.ProgressionAnchor{Type: "scene", ID: "scn_00000000000000000002", Timing: "after"},
				Changes: codex.ProgressionChange{Description: &afterDescription, Metadata: map[string]string{"role": "legend"}},
			},
		},
		ExpectedRevision: nil,
	})
	if err != nil {
		t.Fatalf("SaveProgressions() error = %v", err)
	}
	if len(savedProgressions.Progressions) != 2 || savedProgressions.Revision == nil {
		t.Fatalf("saved progressions = %#v", savedProgressions)
	}
	progressionsPath := filepath.Join(projectPath, "progressions", createdEntry.ID+".yaml")
	progressionBytes, err := os.ReadFile(progressionsPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", progressionsPath, err)
	}
	// Test: the canonical progression file matches the exact bytes the marshaled revision claims (acceptance step 4: verify exact bytes).
	// Requirements: M3-R05, M3-R15
	if !bytesEqual(progressionBytes, savedProgressions.Canonical) {
		t.Fatalf("progression canonical bytes mismatch:\nwant:\n%s\ngot:\n%s", savedProgressions.Canonical, progressionBytes)
	}
	if *savedProgressions.Revision != codex.ComputeRevision(progressionBytes) {
		t.Fatalf("saved progression revision = %q, want %q", *savedProgressions.Revision, codex.ComputeRevision(progressionBytes))
	}

	activeAtDocking, err := service.ResolveActiveCodexState(ctx, createdEntry.ID, "scn_00000000000000000001")
	if err != nil {
		t.Fatalf("ResolveActiveCodexState(docking) error = %v", err)
	}
	// Test: at the first scene the after-anchor progression is excluded and only the base entry applies (acceptance step 5).
	// Requirements: M3-R07
	if activeAtDocking.Entry.Description != "Gone, but influential.\n" || len(activeAtDocking.AppliedProgressionIDs) != 0 {
		t.Fatalf("docking active state = %#v", activeAtDocking)
	}

	activeAtDebrief, err := service.ResolveActiveCodexState(ctx, createdEntry.ID, "scn_00000000000000000002")
	if err != nil {
		t.Fatalf("ResolveActiveCodexState(debrief) error = %v", err)
	}
	// Test: at the anchor scene the before progression is included and the after progression is still excluded (acceptance step 5).
	// Requirements: M3-R07
	if activeAtDebrief.Entry.Description != "Already gone.\n" {
		t.Fatalf("debrief active state = %#v", activeAtDebrief)
	}

	if _, err := service.Reorder(ctx, story.ReorderRequest{
		ParentType:      "chapter",
		ParentID:        "ch_00000000000000000001",
		OrderedChildIDs: []string{"scn_00000000000000000002", "scn_00000000000000000001", "scn_00000000000000000003"},
	}); err != nil {
		t.Fatalf("Reorder() error = %v", err)
	}

	// Test: after reorder the stored anchor scene ID in the progression file is unchanged (acceptance step 6: stored anchor is unchanged).
	// Requirements: M3-R08
	reloadedProgressionBytes, err := os.ReadFile(progressionsPath)
	if err != nil {
		t.Fatalf("ReadFile(reordered %s) error = %v", progressionsPath, err)
	}
	if !bytesEqual(reloadedProgressionBytes, progressionBytes) {
		t.Fatalf("progression bytes changed after reorder:\nbefore:\n%s\nafter:\n%s", progressionBytes, reloadedProgressionBytes)
	}
	reorderedActiveAtDebrief, err := service.ResolveActiveCodexState(ctx, createdEntry.ID, "scn_00000000000000000002")
	if err != nil {
		t.Fatalf("ResolveActiveCodexState(reordered) error = %v", err)
	}
	// Test: after reorder, scene two is now first, so the before progression anchored at scene two is active (anchor index 0 <= target index 0) while the after progression is not (acceptance step 6: activation follows new chronology).
	// Requirements: M3-R08
	if reorderedActiveAtDebrief.Entry.Description != "Already gone.\n" {
		t.Fatalf("reordered active state at debrief = %#v", reorderedActiveAtDebrief)
	}
	if strings.Join(reorderedActiveAtDebrief.AppliedProgressionIDs, ",") != "prog_0123456789abcdef0123" {
		t.Fatalf("reordered applied progression IDs = %#v", reorderedActiveAtDebrief.AppliedProgressionIDs)
	}
	// Test: a later scene still includes the progression (acceptance step 6: a later scene still includes it).
	// Requirements: M3-R08
	reorderedActiveAtAftermath, err := service.ResolveActiveCodexState(ctx, createdEntry.ID, "scn_00000000000000000003")
	if err != nil {
		t.Fatalf("ResolveActiveCodexState(aftermath) error = %v", err)
	}
	if len(reorderedActiveAtAftermath.AppliedProgressionIDs) == 0 {
		t.Fatalf("aftermath applied IDs = %#v, want at least one", reorderedActiveAtAftermath.AppliedProgressionIDs)
	}

	// Test: all state reloads from disk through a fresh service instance (acceptance step 7).
	// Requirements: M3-R21
	freshService := story.NewService(session, storyfile.New(), git, disposableIndex, &staticIDGenerator{ids: []string{"unused"}})
	reloadedEntry, err := freshService.LoadCodexEntry(ctx, createdEntry.ID)
	if err != nil {
		t.Fatalf("LoadCodexEntry(fresh) error = %v", err)
	}
	if reloadedEntry.Revision != updatedEntry.Revision {
		t.Fatalf("reloaded revision = %q, want %q", reloadedEntry.Revision, updatedEntry.Revision)
	}
	reloadedProgressions, err := freshService.LoadProgressions(ctx, createdEntry.ID)
	if err != nil {
		t.Fatalf("LoadProgressions(fresh) error = %v", err)
	}
	if len(reloadedProgressions.Progressions) != 2 {
		t.Fatalf("reloaded progressions = %#v", reloadedProgressions)
	}

	// Capture pre-failure commit count and entry/progression file hashes so we can prove stale/dirty mutations leave history and canon untouched.
	preStaleCommitCount := gitCommitCount(t, ctx, projectPath)
	preStaleEntryHash := fileSHA256(t, entryPath)
	preStaleProgressionHash := fileSHA256(t, progressionsPath)

	if _, err := freshService.UpdateCodexEntry(ctx, createdEntry.ID, codex.SaveEntryRequest{
		Name:             "Obi-Wan Kenobi",
		Aliases:          []string{"Ben"},
		Tags:             []string{"mentor", "jedi"},
		Description:      "Should fail.\n",
		Metadata:         map[string]string{"status": "alive"},
		ExpectedRevision: createdEntry.Revision,
	}); !strings.Contains(err.Error(), story.ErrStaleRevision.Error()) {
		t.Fatalf("stale update error = %v", err)
	}
	// Test: a stale revision leaves the commit count, entry bytes, and progression bytes unchanged (acceptance step 9).
	// Requirements: M3-R17
	if got := gitCommitCount(t, ctx, projectPath); got != preStaleCommitCount {
		t.Fatalf("commit count after stale = %s, want %s", got, preStaleCommitCount)
	}
	if fileSHA256(t, entryPath) != preStaleEntryHash {
		t.Fatalf("entry bytes changed after stale revision")
	}
	if fileSHA256(t, progressionsPath) != preStaleProgressionHash {
		t.Fatalf("progression bytes changed after stale revision")
	}

	// Capture pre-dirty state too, then dirty the worktree and prove a create leaves everything unchanged.
	preDirtyCommitCount := preStaleCommitCount
	if err := os.WriteFile(filepath.Join(projectPath, "notes.txt"), []byte("dirty"), 0o644); err != nil {
		t.Fatalf("WriteFile(dirty) error = %v", err)
	}
	if _, err := freshService.CreateCodexEntry(ctx, codex.SaveEntryRequest{
		Type:        codex.TypeLocation,
		Name:        "Tatooine",
		Aliases:     []string{},
		Tags:        []string{},
		Description: "Desert.\n",
		Metadata:    map[string]string{},
	}); err == nil || !strings.Contains(err.Error(), story.ErrDirtyWorktree.Error()) {
		t.Fatalf("dirty create error = %v", err)
	}
	// Test: a dirty-worktree mutation leaves the commit count and every canonical file unchanged (acceptance step 9).
	// Requirements: M3-R14
	if got := gitCommitCount(t, ctx, projectPath); got != preDirtyCommitCount {
		t.Fatalf("commit count after dirty = %s, want %s", got, preDirtyCommitCount)
	}
	if fileSHA256(t, entryPath) != preStaleEntryHash {
		t.Fatalf("entry bytes changed after dirty worktree")
	}
	if fileSHA256(t, progressionsPath) != preStaleProgressionHash {
		t.Fatalf("progression bytes changed after dirty worktree")
	}
	if _, err := os.Stat(filepath.Join(projectPath, "codex", "locations", "loc_0123456789abcdef0123.yaml")); err == nil {
		t.Fatalf("dirty create wrote a new location file; rollback should have removed it")
	}
	_ = os.Remove(filepath.Join(projectPath, "notes.txt"))

	// Test: exactly one commit per successful mutation and a clean worktree (acceptance step 8: commit count + clean worktree).
	// Requirements: M3-R15
	if got := gitCommitCount(t, ctx, projectPath); got != "10" {
		t.Fatalf("commit count = %q, want 10", got)
	}
	clean, err := git.IsClean(ctx, projectPath)
	if err != nil {
		t.Fatalf("IsClean() error = %v", err)
	}
	if !clean {
		t.Fatal("IsClean() = false, want true")
	}

	// Test: the index manifest covers every canonical file (acceptance step 8: manifest coverage).
	// Requirements: M3-R21
	database, err := sql.Open("sqlite", filepath.Join(projectPath, ".storywork", "index.sqlite"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer database.Close()
	manifestPaths := []string{
		"outline.yaml",
		"arcs/arc_00000000000000000001.yaml",
		"chapters/ch_00000000000000000001.yaml",
		"scenes/scn_00000000000000000001.md",
		"scenes/scn_00000000000000000002.md",
		"scenes/scn_00000000000000000003.md",
		"codex/characters/" + createdEntry.ID + ".yaml",
		"progressions/" + createdEntry.ID + ".yaml",
	}
	for _, relativePath := range manifestPaths {
		var count int
		if err := database.QueryRowContext(ctx, "SELECT COUNT(*) FROM files WHERE path = ?", relativePath).Scan(&count); err != nil {
			t.Fatalf("QueryRow(%s) error = %v", relativePath, err)
		}
		if count != 1 {
			t.Fatalf("manifest count for %s = %d, want 1", relativePath, count)
		}
	}
}

func gitCommitCount(t *testing.T, ctx context.Context, projectPath string) string {
	t.Helper()
	cmd := exec.CommandContext(ctx, "git", "-C", projectPath, "rev-list", "--count", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-list: %v", err)
	}
	return strings.TrimSpace(string(output))
}

func fileSHA256(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	return codex.ComputeRevision(contents)
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
