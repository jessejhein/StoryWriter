package story_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"storywork/internal/gitstore"
	"storywork/internal/index"
	"storywork/internal/project"
	"storywork/internal/story"
	"storywork/internal/storyfile"
	"storywork/internal/workspace"
)

// BDD trace:
//   - Requirement: M2-R01, M2-R02, M2-R10, M2-R11.
//   - Scenario: 2.2.2 — Reload saved content.
//   - Test purpose: verify the real file, Git, and SQLite adapters preserve a
//     successful scene save across reload with exactly one checkpoint and a
//     clean worktree.
func TestMilestone2SceneSaveWithRealAdapters(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	projectPath := filepath.Join(t.TempDir(), "scene-editor-novel")
	git := gitstore.New("git")
	disposableIndex := index.New()
	projectService := project.NewService(
		git,
		disposableIndex,
		func() time.Time { return time.Date(2026, time.June, 28, 12, 0, 0, 0, time.UTC) },
	)
	created, err := projectService.Create(ctx, project.CreateRequest{Name: "Scene Editor Novel", Path: projectPath})
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
		}},
	)

	if _, err := service.CreateArc(ctx, "Act One"); err != nil {
		t.Fatalf("CreateArc() error = %v", err)
	}
	if _, err := service.CreateChapter(ctx, "arc_00000000000000000001", "Arrival"); err != nil {
		t.Fatalf("CreateChapter() error = %v", err)
	}
	if _, err := service.CreateScene(ctx, "ch_00000000000000000001", "The Duel"); err != nil {
		t.Fatalf("CreateScene() error = %v", err)
	}

	loaded, err := service.LoadScene(ctx, "scn_00000000000000000001")
	if err != nil {
		t.Fatalf("LoadScene() error = %v", err)
	}

	saved, err := service.SaveScene(ctx, loaded.ID, story.SaveSceneRequest{
		Title: "The Duel Revised",
		FrontMatter: story.SceneFrontMatter{
			POV:           "Luke",
			Status:        "revised",
			ExcludeFromAI: true,
		},
		Markdown:         "Revised scene prose.\n",
		ExpectedRevision: loaded.Revision,
	})
	if err != nil {
		t.Fatalf("SaveScene() error = %v", err)
	}
	if saved.Title != "The Duel Revised" || saved.Markdown != "Revised scene prose.\n" {
		t.Fatalf("saved scene = %#v", saved)
	}

	reloaded, err := storyfile.New().LoadScene(ctx, projectPath, loaded.ID)
	if err != nil {
		t.Fatalf("LoadScene(from disk) error = %v", err)
	}
	if reloaded.Revision != saved.Revision {
		t.Fatalf("revision = %q, want %q", reloaded.Revision, saved.Revision)
	}
	if reloaded.FrontMatter.Status != "revised" || !reloaded.FrontMatter.ExcludeFromAI {
		t.Fatalf("front matter = %#v", reloaded.FrontMatter)
	}

	commitCount := exec.CommandContext(ctx, "git", "-C", projectPath, "rev-list", "--count", "HEAD")
	output, err := commitCount.Output()
	if err != nil {
		t.Fatalf("git rev-list: %v", err)
	}
	if strings.TrimSpace(string(output)) != "5" {
		t.Fatalf("commit count = %q, want 5", strings.TrimSpace(string(output)))
	}

	clean, err := git.IsClean(ctx, projectPath)
	if err != nil {
		t.Fatalf("IsClean() error = %v", err)
	}
	if !clean {
		t.Fatal("IsClean() = false, want true")
	}
}
