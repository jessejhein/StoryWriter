// BDD Scenario: 3.2.1 - Save ordered progressions
// Requirements: M3-R05, M3-R07, M3-R08, M3-R15, M3-R17, M3-R21
// Test purpose: Plain-English description of the full Milestone 3 backend path with real filesystem, Git, and SQLite adapters for Codex entry save, progression save, active-state resolution, reorder stability, and stale/dirty protection.
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
	if _, err := os.Stat(entryPath); err != nil {
		t.Fatalf("Stat(%s) error = %v", entryPath, err)
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

	activeAtDocking, err := service.ResolveActiveCodexState(ctx, createdEntry.ID, "scn_00000000000000000001")
	if err != nil {
		t.Fatalf("ResolveActiveCodexState(docking) error = %v", err)
	}
	if activeAtDocking.Entry.Description != "Gone, but influential.\n" {
		t.Fatalf("docking active state = %#v", activeAtDocking)
	}

	activeAtDebrief, err := service.ResolveActiveCodexState(ctx, createdEntry.ID, "scn_00000000000000000002")
	if err != nil {
		t.Fatalf("ResolveActiveCodexState(debrief) error = %v", err)
	}
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

	reorderedActiveAtDebrief, err := service.ResolveActiveCodexState(ctx, createdEntry.ID, "scn_00000000000000000002")
	if err != nil {
		t.Fatalf("ResolveActiveCodexState(reordered) error = %v", err)
	}
	if reorderedActiveAtDebrief.Entry.Description != "Already gone.\n" {
		t.Fatalf("reordered active state = %#v", reorderedActiveAtDebrief)
	}
	if strings.Join(reorderedActiveAtDebrief.AppliedProgressionIDs, ",") != "prog_0123456789abcdef0123" {
		t.Fatalf("reordered applied progression IDs = %#v", reorderedActiveAtDebrief.AppliedProgressionIDs)
	}

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
	_ = os.Remove(filepath.Join(projectPath, "notes.txt"))

	commitCount := exec.CommandContext(ctx, "git", "-C", projectPath, "rev-list", "--count", "HEAD")
	output, err := commitCount.Output()
	if err != nil {
		t.Fatalf("git rev-list: %v", err)
	}
	if strings.TrimSpace(string(output)) != "10" {
		t.Fatalf("commit count = %q, want 10", strings.TrimSpace(string(output)))
	}

	clean, err := git.IsClean(ctx, projectPath)
	if err != nil {
		t.Fatalf("IsClean() error = %v", err)
	}
	if !clean {
		t.Fatal("IsClean() = false, want true")
	}

	database, err := sql.Open("sqlite", filepath.Join(projectPath, ".storywork", "index.sqlite"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer database.Close()
	for _, relativePath := range []string{
		"codex/characters/" + createdEntry.ID + ".yaml",
		"progressions/" + createdEntry.ID + ".yaml",
		"scenes/scn_00000000000000000001.md",
	} {
		var count int
		if err := database.QueryRowContext(ctx, "SELECT COUNT(*) FROM files WHERE path = ?", relativePath).Scan(&count); err != nil {
			t.Fatalf("QueryRow(%s) error = %v", relativePath, err)
		}
		if count != 1 {
			t.Fatalf("manifest count for %s = %d, want 1", relativePath, count)
		}
	}
}
