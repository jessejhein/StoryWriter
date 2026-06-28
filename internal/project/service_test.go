package project_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"storywork/internal/gitstore"
	"storywork/internal/index"
	"storywork/internal/project"
)

// BDD trace:
// - Requirement: Milestone 0, Story 0.1, create project folder.
// - Scenario: given an empty directory path, when I create a project named
//   "Test Novel", then the app writes project starter files, initializes Git,
//   creates the SQLite index, and records a first commit.
// - Test purpose: verify the project service creates the canonical folder
//   layout, initializes the adapters, and writes the first commit.
func TestCreateWritesStarterProjectAndInitializesStores(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	projectPath := filepath.Join(root, "test-novel")
	service := project.NewService(
		gitstore.New("git"),
		index.New(),
		func() time.Time { return time.Date(2026, time.June, 27, 12, 0, 0, 0, time.UTC) },
	)

	created, err := service.Create(context.Background(), project.CreateRequest{
		Name: "Test Novel",
		Path: projectPath,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID != "proj_test_novel" {
		t.Fatalf("ID = %q, want proj_test_novel", created.ID)
	}

	wantPaths := []string{
		"project.yaml", "outline.yaml", ".gitignore",
		"agents/line_polish.yaml", "styles/precise_editor.yaml",
		"arcs", "chapters", "scenes", "codex/characters", "codex/locations",
		"codex/lore", "codex/custom", "progressions", "imports/raw",
		"imports/processed", ".storywork/tmp", ".storywork/index.sqlite", ".git",
	}
	for _, relativePath := range wantPaths {
		if _, err := os.Stat(filepath.Join(projectPath, relativePath)); err != nil {
			t.Errorf("starter path %q: %v", relativePath, err)
		}
	}

	command := exec.Command("git", "-C", projectPath, "rev-list", "--count", "HEAD")
	output, err := command.Output()
	if err != nil {
		t.Fatalf("git rev-list: %v", err)
	}
	if string(output) != "1\n" {
		t.Fatalf("commit count = %q, want 1", output)
	}
}

// BDD trace:
// - Requirement: Milestone 0, Story 0.2, open existing project.
// - Scenario: given a valid project folder, when I open the project, then the
//   app loads project metadata and verifies or rebuilds the SQLite index.
// - Test purpose: verify opening a valid project rebuilds a missing index and
//   preserves the project metadata.
func TestOpenValidProjectRebuildsMissingIndex(t *testing.T) {
	t.Parallel()

	projectPath := filepath.Join(t.TempDir(), "test-novel")
	service := project.NewService(gitstore.New("git"), index.New(), time.Now)
	if _, err := service.Create(context.Background(), project.CreateRequest{Name: "Test Novel", Path: projectPath}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := os.Remove(filepath.Join(projectPath, ".storywork", "index.sqlite")); err != nil {
		t.Fatal(err)
	}

	opened, err := service.Open(context.Background(), projectPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if opened.Name != "Test Novel" {
		t.Fatalf("Name = %q, want Test Novel", opened.Name)
	}
	if _, err := os.Stat(filepath.Join(projectPath, ".storywork", "index.sqlite")); err != nil {
		t.Fatalf("rebuilt index: %v", err)
	}
}

// BDD trace:
// - Requirement: Milestone 0, Story 0.2, open existing project.
// - Scenario: given a valid project folder, when I open the project, then the
//   app loads project metadata and verifies or rebuilds the SQLite index.
// - Test purpose: verify opening an invalid project fails with a useful error.
func TestOpenRejectsInvalidProject(t *testing.T) {
	t.Parallel()

	service := project.NewService(gitstore.New("git"), index.New(), time.Now)
	_, err := service.Open(context.Background(), t.TempDir())
	if err == nil {
		t.Fatal("Open() error = nil, want invalid project error")
	}
}
