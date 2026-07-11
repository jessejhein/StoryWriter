// BDD Scenario: 8.1.3 - Continue normal work in the experiment
// Requirements: M8-R03, M8-R19
// Test purpose: Real Git integration proves scene, Codex, and progression
// mutations commit only to the checked-out experiment branch while main ref and
// tree remain byte-for-byte unchanged.

package story_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"storywork/internal/codex"
	"storywork/internal/gitstore"
	"storywork/internal/index"
	"storywork/internal/project"
	"storywork/internal/story"
	"storywork/internal/storyfile"
	"storywork/internal/workspace"
)

const milestone8ExperimentRef = "branch/test-exp-0123abcd"

func setupMilestone8StoryProject(t *testing.T) (context.Context, string, *story.Service, *gitstore.Store) {
	t.Helper()

	ctx := context.Background()
	projectPath := filepath.Join(t.TempDir(), "milestone8-story-novel")
	git := gitstore.New("git")
	disposableIndex := index.New()
	projectService := project.NewService(
		git,
		disposableIndex,
		func() time.Time { return time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC) },
	)
	created, err := projectService.Create(ctx, project.CreateRequest{Name: "Milestone 8 Story Novel", Path: projectPath})
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
			"char_0123456789abcdef0123",
			"prog_0123456789abcdef0123",
		}},
	)
	return ctx, projectPath, service, git
}

func checkoutMilestone8ExperimentBranch(t *testing.T, ctx context.Context, projectPath string) {
	t.Helper()
	output, err := exec.CommandContext(ctx, "git", "-C", projectPath, "checkout", "-b", milestone8ExperimentRef, "main").CombinedOutput()
	if err != nil {
		t.Fatalf("git checkout -b %s main: %v: %s", milestone8ExperimentRef, err, output)
	}
}

func gitRevParseMilestone8(t *testing.T, ctx context.Context, projectPath, ref string) string {
	t.Helper()
	output, err := exec.CommandContext(ctx, "git", "-C", projectPath, "rev-parse", ref).Output()
	if err != nil {
		t.Fatalf("git rev-parse %s: %v", ref, err)
	}
	return strings.TrimSpace(string(output))
}

func recordMainRefAndTree(t *testing.T, ctx context.Context, projectPath string) (string, string) {
	t.Helper()
	return gitRevParseMilestone8(t, ctx, projectPath, "main"), gitRevParseMilestone8(t, ctx, projectPath, "main^{tree}")
}

func assertMainRefAndTreeUnchanged(t *testing.T, ctx context.Context, projectPath, wantMainHead, wantMainTree string) {
	t.Helper()
	if got := gitRevParseMilestone8(t, ctx, projectPath, "main"); got != wantMainHead {
		t.Fatalf("main HEAD = %q, want %q", got, wantMainHead)
	}
	if got := gitRevParseMilestone8(t, ctx, projectPath, "main^{tree}"); got != wantMainTree {
		t.Fatalf("main tree = %q, want %q", got, wantMainTree)
	}
	output, err := exec.CommandContext(ctx, "git", "-C", projectPath, "diff", "main", "main").CombinedOutput()
	if err != nil {
		t.Fatalf("git diff main main: %v: %s", err, output)
	}
	if strings.TrimSpace(string(output)) != "" {
		t.Fatalf("git diff main main = %q, want empty", output)
	}
}

func assertCommitOnExperimentNotMain(t *testing.T, ctx context.Context, projectPath, mainHeadBefore, wantSubject string) {
	t.Helper()
	currentBranch, err := exec.CommandContext(ctx, "git", "-C", projectPath, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		t.Fatalf("git rev-parse --abbrev-ref HEAD: %v", err)
	}
	if got := strings.TrimSpace(string(currentBranch)); got != milestone8ExperimentRef {
		t.Fatalf("active branch = %q, want %q", got, milestone8ExperimentRef)
	}
	experimentHead := gitRevParseMilestone8(t, ctx, projectPath, milestone8ExperimentRef)
	if experimentHead == mainHeadBefore {
		t.Fatalf("experiment HEAD = main HEAD = %q, want new commit on experiment", mainHeadBefore)
	}
	if gitRevParseMilestone8(t, ctx, projectPath, "main") == experimentHead {
		t.Fatalf("main advanced to experiment HEAD %q", experimentHead)
	}
	subject, err := exec.CommandContext(ctx, "git", "-C", projectPath, "log", "-1", "--format=%s", experimentHead).Output()
	if err != nil {
		t.Fatalf("git log experiment HEAD: %v", err)
	}
	if got := strings.TrimSpace(string(subject)); got != wantSubject {
		t.Fatalf("experiment commit subject = %q, want %q", got, wantSubject)
	}
	isAncestor, err := exec.CommandContext(ctx, "git", "-C", projectPath, "merge-base", "--is-ancestor", experimentHead, "main").CombinedOutput()
	if err == nil {
		t.Fatalf("experiment HEAD %q is ancestor of main, want commit only on experiment", experimentHead)
	}
	if !strings.Contains(string(isAncestor), "1") && !strings.Contains(err.Error(), "exit status 1") {
		t.Fatalf("merge-base --is-ancestor experiment main: %v: %s", err, isAncestor)
	}
}

// Test: scene save commits to whichever branch is checked out.
// Requirements: M8-R03, M8-R19.
func TestMilestone8SceneSaveCommitsToCheckedOutExperimentBranch(t *testing.T) {
	t.Parallel()

	ctx, projectPath, service, git := setupMilestone8StoryProject(t)
	if _, err := service.CreateArc(ctx, "Act One"); err != nil {
		t.Fatalf("CreateArc() error = %v", err)
	}
	if _, err := service.CreateChapter(ctx, "arc_00000000000000000001", "Arrival"); err != nil {
		t.Fatalf("CreateChapter() error = %v", err)
	}
	if _, err := service.CreateScene(ctx, "ch_00000000000000000001", "The Duel"); err != nil {
		t.Fatalf("CreateScene() error = %v", err)
	}

	mainHeadBefore, mainTreeBefore := recordMainRefAndTree(t, ctx, projectPath)
	checkoutMilestone8ExperimentBranch(t, ctx, projectPath)

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
		Markdown:         "Experiment scene prose.\n",
		ExpectedRevision: loaded.Revision,
	})
	if err != nil {
		t.Fatalf("SaveScene() error = %v", err)
	}
	if saved.Markdown != "Experiment scene prose.\n" {
		t.Fatalf("saved scene markdown = %q", saved.Markdown)
	}

	assertMainRefAndTreeUnchanged(t, ctx, projectPath, mainHeadBefore, mainTreeBefore)
	assertCommitOnExperimentNotMain(t, ctx, projectPath, mainHeadBefore, "Edit scene scn_00000000000000000001")

	clean, err := git.IsClean(ctx, projectPath)
	if err != nil {
		t.Fatalf("IsClean() error = %v", err)
	}
	if !clean {
		t.Fatal("IsClean() = false, want true")
	}
}

// Test: Codex and progression mutations commit to active branch only.
// Requirements: M8-R03, M8-R19.
func TestMilestone8CodexAndProgressionMutationCommitsToActiveBranchOnly(t *testing.T) {
	t.Parallel()

	ctx, projectPath, service, git := setupMilestone8StoryProject(t)
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

	mainHeadBefore, mainTreeBefore := recordMainRefAndTree(t, ctx, projectPath)
	checkoutMilestone8ExperimentBranch(t, ctx, projectPath)

	updatedEntry, err := service.UpdateCodexEntry(ctx, createdEntry.ID, codex.SaveEntryRequest{
		Name:             "Obi-Wan Kenobi",
		Aliases:          []string{"Ben"},
		Tags:             []string{"mentor", "jedi"},
		Description:      "Experiment branch description.\n",
		Metadata:         map[string]string{"status": "alive", "role": "mentor"},
		ExpectedRevision: createdEntry.Revision,
	})
	if err != nil {
		t.Fatalf("UpdateCodexEntry() error = %v", err)
	}
	assertMainRefAndTreeUnchanged(t, ctx, projectPath, mainHeadBefore, mainTreeBefore)
	assertCommitOnExperimentNotMain(t, ctx, projectPath, mainHeadBefore, "Edit Codex entry "+createdEntry.ID)

	afterDescription := "Legend on experiment branch.\n"
	savedProgressions, err := service.SaveProgressions(ctx, createdEntry.ID, codex.SaveProgressionsRequest{
		Progressions: []codex.Progression{
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
	if len(savedProgressions.Progressions) != 1 {
		t.Fatalf("saved progressions = %#v", savedProgressions)
	}

	assertMainRefAndTreeUnchanged(t, ctx, projectPath, mainHeadBefore, mainTreeBefore)
	assertCommitOnExperimentNotMain(t, ctx, projectPath, mainHeadBefore, "Edit progressions "+createdEntry.ID)

	if updatedEntry.Revision == createdEntry.Revision {
		t.Fatalf("updated entry revision unchanged from %q", createdEntry.Revision)
	}

	clean, err := git.IsClean(ctx, projectPath)
	if err != nil {
		t.Fatalf("IsClean() error = %v", err)
	}
	if !clean {
		t.Fatal("IsClean() = false, want true")
	}

	output, err := exec.CommandContext(ctx, "git", "-C", projectPath, "show", "main:codex/characters/"+createdEntry.ID+".yaml").CombinedOutput()
	if err != nil {
		t.Fatalf("git show main codex entry: %v: %s", err, output)
	}
	if strings.Contains(string(output), "Experiment branch description") || strings.Contains(string(output), "Legend on experiment branch") {
		t.Fatalf("main tree contains experiment codex bytes: %s", output)
	}
	if !strings.Contains(string(output), "Guide.") {
		t.Fatalf("main tree lost original codex bytes: %s", output)
	}
}
