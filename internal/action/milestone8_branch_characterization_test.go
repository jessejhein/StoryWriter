// BDD Scenario: 8.1.3 - Continue normal work in the experiment
// Requirements: M8-R03, M8-R19
// Test purpose: Real Git integration proves accepted AI patches commit only to
// the checked-out experiment branch, lineage lookup uses active ancestry, and
// main ref and tree remain byte-for-byte unchanged.

package action_test

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"storywork/internal/action"
	"storywork/internal/agent"
	"storywork/internal/contextpack"
	"storywork/internal/gitstore"
	"storywork/internal/index"
	"storywork/internal/project"
	"storywork/internal/story"
	"storywork/internal/storyfile"
	"storywork/internal/workspace"
)

const milestone8ExperimentRef = "branch/test-exp-0123abcd"

func setupMilestone8ActionProject(t *testing.T) (context.Context, string, *story.Service, *action.Service) {
	t.Helper()

	ctx := context.Background()
	projectPath := t.TempDir()
	git := gitstore.New("git")
	indexStore := index.New()
	projectService := project.NewService(git, indexStore, func() time.Time { return time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC) })
	created, err := projectService.Create(ctx, project.CreateRequest{Name: "Milestone 8 Action Novel", Path: projectPath})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	session := workspace.NewSession()
	session.Set(created)
	storyService := story.NewService(session, storyfile.New(), git, indexStore, &staticStoryIDGenerator{ids: []string{
		"arc_00000000000000000001", "ch_00000000000000000001", "scn_00000000000000000001",
	}})
	if _, err := storyService.CreateArc(ctx, "Act"); err != nil {
		t.Fatalf("CreateArc() error = %v", err)
	}
	if _, err := storyService.CreateChapter(ctx, "arc_00000000000000000001", "Chapter"); err != nil {
		t.Fatalf("CreateChapter() error = %v", err)
	}
	if _, err := storyService.CreateScene(ctx, "ch_00000000000000000001", "Scene"); err != nil {
		t.Fatalf("CreateScene() error = %v", err)
	}
	scene, err := storyService.LoadScene(ctx, "scn_00000000000000000001")
	if err != nil {
		t.Fatalf("LoadScene() error = %v", err)
	}
	selected := "Alpha beta gamma delta echo foxtrot golf hotel india juliet kilo lima mike november oscar papa quebec romeo sierra tango uniform victor whiskey xray yankee zulu."
	scene, err = storyService.SaveScene(ctx, scene.ID, story.SaveSceneRequest{
		Title: "Scene", FrontMatter: story.SceneFrontMatter{Status: "draft"},
		Markdown: selected + "\n", ExpectedRevision: scene.Revision,
	})
	if err != nil {
		t.Fatalf("SaveScene() error = %v", err)
	}

	actionService := action.NewService(session, agent.NewLoader(), storyService, storyService, agent.NewMockProvider(), nil, action.NewRunStore(), &staticRunIDGenerator{ids: []string{
		"run_aaaaaaaaaaaaaaaaaaaa", "run_bbbbbbbbbbbbbbbbbbbb",
	}}).WithMaterialSource(storyService).WithContextBuilder(contextpack.NewBuilder()).WithBodyAcceptor(storyService).
		WithInvitationStore(action.NewInvitationStore(100)).WithInvitationIDGenerator(&staticInvitationIDGenerator{ids: []string{
		"invite_aaaaaaaaaaaaaaaaaaaa", "invite_bbbbbbbbbbbbbbbbbbbb",
	}})

	return ctx, projectPath, storyService, actionService
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

func assertCommitOnExperimentNotMain(t *testing.T, ctx context.Context, projectPath, mainHeadBefore string) {
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
}

// Test: accepted AI patch commits to active branch only and lineage uses active ancestry.
// Requirements: M8-R03, M8-R19.
func TestMilestone8AcceptedAIPatchCommitsToActiveBranchOnly(t *testing.T) {
	t.Parallel()

	ctx, projectPath, storyService, actionService := setupMilestone8ActionProject(t)
	mainHeadBefore, mainTreeBefore := recordMainRefAndTree(t, ctx, projectPath)
	checkoutMilestone8ExperimentBranch(t, ctx, projectPath)

	scene, err := storyService.LoadScene(ctx, "scn_00000000000000000001")
	if err != nil {
		t.Fatalf("LoadScene() error = %v", err)
	}
	selected := "Alpha beta gamma delta echo foxtrot golf hotel india juliet kilo lima mike november oscar papa quebec romeo sierra tango uniform victor whiskey xray yankee zulu."
	startByte := 0
	endByte := len([]byte(selected))
	root, err := actionService.Run(ctx, action.RunRequest{
		AgentID: "line_polish", StyleID: "precise_editor", Surface: agent.SurfaceEditor, InputScope: agent.InputScopeSelection,
		SceneID: scene.ID, SceneRevision: scene.Revision,
		Selection: action.Selection{StartByte: startByte, EndByte: endByte, Text: selected},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	rootResult, err := actionService.Accept(ctx, root.RunID, scene.Revision)
	if err != nil {
		t.Fatalf("Accept(root) error = %v", err)
	}
	if len(rootResult.FollowUpInvitations) != 1 {
		t.Fatalf("follow-up invitations = %#v", rootResult.FollowUpInvitations)
	}

	assertMainRefAndTreeUnchanged(t, ctx, projectPath, mainHeadBefore, mainTreeBefore)
	assertCommitOnExperimentNotMain(t, ctx, projectPath, mainHeadBefore)

	rootMessage := gitShowBody(t, ctx, projectPath)
	if !strings.Contains(rootMessage, "Storywork-Operation-ID: run_aaaaaaaaaaaaaaaaaaaa") || !strings.Contains(rootMessage, "Storywork-Scope: selection:") {
		t.Fatalf("root commit = %q", rootMessage)
	}

	reloaded, err := storyService.LoadScene(ctx, scene.ID)
	if err != nil {
		t.Fatalf("LoadScene(reloaded) error = %v", err)
	}
	child, err := actionService.RunInvitation(ctx, rootResult.FollowUpInvitations[0].InvitationID, action.InvitationRunRequest{
		StyleID: "precise_editor", ExpectedTargetRevision: reloaded.Revision,
	})
	if err != nil {
		t.Fatalf("RunInvitation() error = %v", err)
	}
	if _, err := actionService.AcceptBody(ctx, child.RunID, reloaded.Revision); err != nil {
		t.Fatalf("AcceptBody(child) error = %v", err)
	}

	assertMainRefAndTreeUnchanged(t, ctx, projectPath, mainHeadBefore, mainTreeBefore)
	assertCommitOnExperimentNotMain(t, ctx, projectPath, mainHeadBefore)

	childMessage := gitShowBody(t, ctx, projectPath)
	if !strings.Contains(childMessage, "Storywork-Triggered-By: run_aaaaaaaaaaaaaaaaaaaa") {
		t.Fatalf("child commit = %q", childMessage)
	}

	git := gitstore.New("git")
	found, err := git.HasOperationInAncestry(ctx, projectPath, "run_aaaaaaaaaaaaaaaaaaaa")
	if err != nil || !found {
		t.Fatalf("HasOperationInAncestry(root) = %v, %v, want true on experiment ancestry", found, err)
	}

	output, err := exec.CommandContext(ctx, "git", "-C", projectPath, "show", "main:scenes/scn_00000000000000000001.md").CombinedOutput()
	if err == nil && strings.Contains(string(output), "Provider polished selection") {
		t.Fatalf("main tree contains experiment AI patch: %s", output)
	}
}
