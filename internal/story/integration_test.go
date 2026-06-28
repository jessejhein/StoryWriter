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

	"storywork/internal/gitstore"
	"storywork/internal/index"
	"storywork/internal/project"
	"storywork/internal/story"
	"storywork/internal/storyfile"
	"storywork/internal/workspace"
)

type staticIDGenerator struct {
	ids []string
}

func (g *staticIDGenerator) Next(_ story.NodeKind) (string, error) {
	next := g.ids[0]
	g.ids = g.ids[1:]
	return next, nil
}

// BDD trace:
//   - Requirement: Milestone 1, Stories 1.2 to 1.4.
//   - Scenario: creating and reordering structure writes canonical files, keeps
//     stable IDs across reload, records one checkpoint per successful mutation,
//     leaves the worktree clean, and updates the derived index.
//   - Test purpose: verify the full Milestone 1 backend path with the real file,
//     Git, and SQLite adapters.
func TestMilestone1EndToEndWithRealAdapters(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	projectPath := filepath.Join(t.TempDir(), "test-novel")
	git := gitstore.New("git")
	disposableIndex := index.New()
	projectService := project.NewService(
		git,
		disposableIndex,
		func() time.Time { return time.Date(2026, time.June, 27, 12, 0, 0, 0, time.UTC) },
	)
	created, err := projectService.Create(ctx, project.CreateRequest{Name: "Test Novel", Path: projectPath})
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
			"arc_00000000000000000002",
			"ch_00000000000000000001",
			"ch_00000000000000000002",
			"scn_00000000000000000001",
			"scn_00000000000000000002",
		}},
	)

	if _, err := service.CreateArc(ctx, "Act One"); err != nil {
		t.Fatalf("CreateArc(Act One) error = %v", err)
	}
	if _, err := service.CreateArc(ctx, "Act Two"); err != nil {
		t.Fatalf("CreateArc(Act Two) error = %v", err)
	}
	if _, err := service.CreateChapter(ctx, "arc_00000000000000000001", "Arrival"); err != nil {
		t.Fatalf("CreateChapter(Arrival) error = %v", err)
	}
	if _, err := service.CreateChapter(ctx, "arc_00000000000000000001", "Departure"); err != nil {
		t.Fatalf("CreateChapter(Departure) error = %v", err)
	}
	if _, err := service.CreateScene(ctx, "ch_00000000000000000001", "The Station"); err != nil {
		t.Fatalf("CreateScene(The Station) error = %v", err)
	}
	if _, err := service.CreateScene(ctx, "ch_00000000000000000001", "The Gate"); err != nil {
		t.Fatalf("CreateScene(The Gate) error = %v", err)
	}
	if _, err := service.Reorder(ctx, story.ReorderRequest{
		ParentType:      "arc",
		ParentID:        "arc_00000000000000000001",
		OrderedChildIDs: []string{"ch_00000000000000000002", "ch_00000000000000000001"},
	}); err != nil {
		t.Fatalf("Reorder(chapters) error = %v", err)
	}
	result, err := service.Reorder(ctx, story.ReorderRequest{
		ParentType:      "chapter",
		ParentID:        "ch_00000000000000000001",
		OrderedChildIDs: []string{"scn_00000000000000000002", "scn_00000000000000000001"},
	})
	if err != nil {
		t.Fatalf("Reorder(scenes) error = %v", err)
	}

	loaded, err := storyfile.New().Load(ctx, projectPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loaded.Arcs) != 2 {
		t.Fatalf("arc count = %d, want 2", len(loaded.Arcs))
	}
	if loaded.Arcs[0].Chapters[0].ID != "ch_00000000000000000002" {
		t.Fatalf("first chapter ID = %q", loaded.Arcs[0].Chapters[0].ID)
	}
	if loaded.Arcs[0].Chapters[1].Scenes[0].ID != "scn_00000000000000000002" {
		t.Fatalf("first scene ID after reload = %q", loaded.Arcs[0].Chapters[1].Scenes[0].ID)
	}
	if result.Outline.Arcs[0].Chapters[1].Scenes[1].ID != "scn_00000000000000000001" {
		t.Fatalf("result outline did not preserve stable IDs: %#v", result.Outline)
	}

	for _, relativePath := range []string{
		"arcs/arc_00000000000000000001.yaml",
		"arcs/arc_00000000000000000002.yaml",
		"chapters/ch_00000000000000000001.yaml",
		"chapters/ch_00000000000000000002.yaml",
		"scenes/scn_00000000000000000001.md",
		"scenes/scn_00000000000000000002.md",
		"outline.yaml",
	} {
		if _, err := os.Stat(filepath.Join(projectPath, relativePath)); err != nil {
			t.Fatalf("Stat(%s) error = %v", relativePath, err)
		}
	}

	commitCount := exec.CommandContext(ctx, "git", "-C", projectPath, "rev-list", "--count", "HEAD")
	output, err := commitCount.Output()
	if err != nil {
		t.Fatalf("git rev-list: %v", err)
	}
	if strings.TrimSpace(string(output)) != "9" {
		t.Fatalf("commit count = %q, want 9", strings.TrimSpace(string(output)))
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
		"arcs/arc_00000000000000000001.yaml",
		"chapters/ch_00000000000000000001.yaml",
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
