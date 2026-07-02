// BDD Scenario: 7.5.1 - Record causal and dependency trailers
// Requirements: M7-R13, M7-R15
// Test purpose: Real Git integration proves accepted operations write exact commit trailers.

package action_test

import (
	"context"
	"errors"
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

// Test: accepted root and invited child operations write Git trailers in real project history.
// Requirements: M7-R13.
func TestMilestone7AcceptedOperationsWriteGitTrailers(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	projectPath := t.TempDir()
	git := gitstore.New("git")
	indexStore := index.New()
	projectService := project.NewService(git, indexStore, func() time.Time { return time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC) })
	created, err := projectService.Create(ctx, project.CreateRequest{Name: "Lineage Novel", Path: projectPath})
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
	childMessage := gitShowBody(t, ctx, projectPath)
	if !strings.Contains(childMessage, "Storywork-Triggered-By: run_aaaaaaaaaaaaaaaaaaaa") {
		t.Fatalf("child commit = %q", childMessage)
	}
}

type staticInvitationIDGenerator struct {
	ids []string
}

func (g *staticInvitationIDGenerator) Next() (string, error) {
	if len(g.ids) == 0 {
		return "", errors.New("no invitation IDs remaining")
	}
	next := g.ids[0]
	g.ids = g.ids[1:]
	return next, nil
}

func gitShowBody(t *testing.T, ctx context.Context, path string) string {
	t.Helper()
	output, err := exec.CommandContext(ctx, "git", "-C", path, "show", "--no-patch", "--format=%B", "HEAD").Output()
	if err != nil {
		t.Fatalf("git show: %v", err)
	}
	return string(output)
}
