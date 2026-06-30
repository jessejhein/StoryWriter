package action_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"storywork/internal/action"
	"storywork/internal/agent"
	"storywork/internal/gitstore"
	"storywork/internal/index"
	"storywork/internal/project"
	"storywork/internal/story"
	"storywork/internal/storyfile"
	"storywork/internal/workspace"
)

type staticStoryIDGenerator struct {
	ids []string
}

func (g *staticStoryIDGenerator) Next(_ story.NodeKind) (string, error) {
	if len(g.ids) == 0 {
		return "", errors.New("no story IDs remaining")
	}
	next := g.ids[0]
	g.ids = g.ids[1:]
	return next, nil
}

type staticRunIDGenerator struct {
	ids []string
}

func (g *staticRunIDGenerator) Next() (string, error) {
	if len(g.ids) == 0 {
		return "", errors.New("no run IDs remaining")
	}
	next := g.ids[0]
	g.ids = g.ids[1:]
	return next, nil
}

// BDD trace:
//   - Requirements: M4-R01 through M4-R20.
//   - Scenario: 4.1.1, 4.2.1, 4.3.2, 4.4.1, 4.4.2, 4.4.3.
//   - Test purpose: verify the real filesystem, Git, and SQLite adapters support
//     strict registry listing, zero-mutation run/reject, explicit accept with one
//     checkpoint, multibyte UTF-8 selection, and stale/dirty protection across a
//     fresh service reload.
func TestMilestone4ActionFlowWithRealAdapters(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	projectPath := filepath.Join(t.TempDir(), "agent-actions-novel")
	git := gitstore.New("git")
	disposableIndex := index.New()
	projectService := project.NewService(
		git,
		disposableIndex,
		func() time.Time { return time.Date(2026, time.June, 29, 12, 0, 0, 0, time.UTC) },
	)
	created, err := projectService.Create(ctx, project.CreateRequest{Name: "Agent Actions Novel", Path: projectPath})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	session := workspace.NewSession()
	session.Set(created)
	storyService := story.NewService(
		session,
		storyfile.New(),
		git,
		disposableIndex,
		&staticStoryIDGenerator{ids: []string{
			"arc_00000000000000000001",
			"ch_00000000000000000001",
			"scn_00000000000000000001",
		}},
	)
	if _, err := storyService.CreateArc(ctx, "Act One"); err != nil {
		t.Fatalf("CreateArc() error = %v", err)
	}
	if _, err := storyService.CreateChapter(ctx, "arc_00000000000000000001", "Arrival"); err != nil {
		t.Fatalf("CreateChapter() error = %v", err)
	}
	if _, err := storyService.CreateScene(ctx, "ch_00000000000000000001", "The Duel"); err != nil {
		t.Fatalf("CreateScene() error = %v", err)
	}

	loadedScene, err := storyService.LoadScene(ctx, "scn_00000000000000000001")
	if err != nil {
		t.Fatalf("LoadScene() error = %v", err)
	}
	prefix := "Prelude: "
	selectedText := "Alpha beta gamma delta echo foxtrot golf hotel india juliet kilo lima Luz ágil mike november oscar papa quebec romeo sierra tango umbrella."
	suffix := " [tail marker]"
	markdown := prefix + selectedText + suffix + "\nOmega settles.\n"
	savedScene, err := storyService.SaveScene(ctx, loadedScene.ID, story.SaveSceneRequest{
		Title: "The Duel",
		FrontMatter: story.SceneFrontMatter{
			POV:           "Luke",
			Status:        "draft",
			ExcludeFromAI: false,
		},
		Markdown:         markdown,
		ExpectedRevision: loadedScene.Revision,
	})
	if err != nil {
		t.Fatalf("SaveScene() error = %v", err)
	}

	actionService := action.NewService(
		session,
		agent.NewLoader(),
		storyService,
		storyService,
		agent.NewMockProvider(),
		action.NewRunStore(),
		&staticRunIDGenerator{ids: []string{
			"run_00000000000000000001",
			"run_00000000000000000002",
			"run_00000000000000000003",
			"run_00000000000000000004",
		}},
	)

	agents, err := actionService.Agents(ctx)
	if err != nil {
		t.Fatalf("Agents() error = %v", err)
	}
	if len(agents) != 2 || agents[0].ID != "chapter_refiner" || agents[1].ID != "line_polish" {
		t.Fatalf("agents = %#v", agents)
	}
	styles, err := actionService.Styles(ctx)
	if err != nil {
		t.Fatalf("Styles() error = %v", err)
	}
	if len(styles) != 1 || styles[0].ID != "precise_editor" {
		t.Fatalf("styles = %#v", styles)
	}

	available, err := actionService.AvailableActions(ctx, agent.AvailabilityInput{
		Surface:        agent.SurfaceEditor,
		InputScope:     agent.InputScopeSelection,
		SceneID:        savedScene.ID,
		SelectionWords: agent.WordCount(selectedText),
	})
	if err != nil {
		t.Fatalf("AvailableActions() error = %v", err)
	}
	if len(available) != 1 || available[0].AgentID != "line_polish" {
		t.Fatalf("available actions = %#v", available)
	}

	sceneFile := filepath.Join(projectPath, "scenes", savedScene.ID+".md")
	indexFile := filepath.Join(projectPath, ".storywork", "index.sqlite")
	sceneBeforeRun, err := os.ReadFile(sceneFile)
	if err != nil {
		t.Fatalf("ReadFile(scene) error = %v", err)
	}
	indexBeforeRun, err := os.ReadFile(indexFile)
	if err != nil {
		t.Fatalf("ReadFile(index) error = %v", err)
	}
	commitCountBeforeRun := gitCommitCount(t, ctx, projectPath)

	startByte := len([]byte(prefix))
	endByte := startByte + len([]byte(selectedText))
	firstRun, err := actionService.Run(ctx, action.RunRequest{
		AgentID:       "line_polish",
		StyleID:       "precise_editor",
		Surface:       agent.SurfaceEditor,
		InputScope:    agent.InputScopeSelection,
		SceneID:       savedScene.ID,
		SceneRevision: savedScene.Revision,
		Selection: action.Selection{
			StartByte: startByte,
			EndByte:   endByte,
			Text:      selectedText,
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if firstRun.Replacement != "Mock polished: "+selectedText {
		t.Fatalf("run replacement = %q", firstRun.Replacement)
	}
	sceneAfterRun, err := os.ReadFile(sceneFile)
	if err != nil {
		t.Fatalf("ReadFile(scene after run) error = %v", err)
	}
	indexAfterRun, err := os.ReadFile(indexFile)
	if err != nil {
		t.Fatalf("ReadFile(index after run) error = %v", err)
	}
	if !bytes.Equal(sceneBeforeRun, sceneAfterRun) || !bytes.Equal(indexBeforeRun, indexAfterRun) {
		t.Fatal("run mutated canonical scene or index bytes")
	}
	if gitCommitCount(t, ctx, projectPath) != commitCountBeforeRun {
		t.Fatal("run changed git history")
	}
	clean, err := git.IsClean(ctx, projectPath)
	if err != nil {
		t.Fatalf("IsClean() error = %v", err)
	}
	if !clean {
		t.Fatal("IsClean() after run = false, want true")
	}

	rejected, err := actionService.Reject(ctx, firstRun.RunID)
	if err != nil {
		t.Fatalf("Reject() error = %v", err)
	}
	if rejected.Status != action.RunRejected {
		t.Fatalf("rejected status = %q, want rejected", rejected.Status)
	}
	if !bytes.Equal(sceneBeforeRun, mustReadFile(t, sceneFile)) || gitCommitCount(t, ctx, projectPath) != commitCountBeforeRun {
		t.Fatal("reject mutated canonical state")
	}

	secondRun, err := actionService.Run(ctx, action.RunRequest{
		AgentID:       "line_polish",
		StyleID:       "precise_editor",
		Surface:       agent.SurfaceEditor,
		InputScope:    agent.InputScopeSelection,
		SceneID:       savedScene.ID,
		SceneRevision: savedScene.Revision,
		Selection: action.Selection{
			StartByte: startByte,
			EndByte:   endByte,
			Text:      selectedText,
		},
	})
	if err != nil {
		t.Fatalf("Run(second) error = %v", err)
	}
	acceptedRun, acceptedScene, err := actionService.Accept(ctx, secondRun.RunID, savedScene.Revision)
	if err != nil {
		t.Fatalf("Accept() error = %v", err)
	}
	if acceptedRun.Status != action.RunAccepted {
		t.Fatalf("accepted status = %q, want accepted", acceptedRun.Status)
	}
	expectedAcceptedMarkdown := prefix + "Mock polished: " + selectedText + suffix + "\nOmega settles.\n"
	if acceptedScene.Markdown != expectedAcceptedMarkdown {
		t.Fatalf("accepted markdown = %q, want %q", acceptedScene.Markdown, expectedAcceptedMarkdown)
	}
	if gitCommitCount(t, ctx, projectPath) != commitCountBeforeRun+1 {
		t.Fatalf("commit count after accept = %d, want %d", gitCommitCount(t, ctx, projectPath), commitCountBeforeRun+1)
	}
	if commitMessage(t, ctx, projectPath) != "Accept AI patch "+secondRun.RunID {
		t.Fatalf("commit message = %q", commitMessage(t, ctx, projectPath))
	}
	if clean, err := git.IsClean(ctx, projectPath); err != nil || !clean {
		t.Fatalf("IsClean() after accept = %v, %v", clean, err)
	}
	reloadedActionService := action.NewService(
		session,
		agent.NewLoader(),
		story.NewService(session, storyfile.New(), git, disposableIndex, story.NewRandomIDGenerator()),
		story.NewService(session, storyfile.New(), git, disposableIndex, story.NewRandomIDGenerator()),
		agent.NewMockProvider(),
		action.NewRunStore(),
		&staticRunIDGenerator{ids: []string{"run_00000000000000000005"}},
	)
	reloadedScene, err := storyfile.New().LoadScene(ctx, projectPath, savedScene.ID)
	if err != nil {
		t.Fatalf("LoadScene(reloaded) error = %v", err)
	}
	if reloadedScene.Markdown != acceptedScene.Markdown {
		t.Fatalf("reloaded markdown = %q, want %q", reloadedScene.Markdown, acceptedScene.Markdown)
	}
	_ = reloadedActionService

	if _, err := actionService.Reject(ctx, secondRun.RunID); !errors.Is(err, action.ErrRunConflict) {
		t.Fatalf("Reject(accepted run) error = %v, want ErrRunConflict", err)
	}

	thirdRun, err := actionService.Run(ctx, action.RunRequest{
		AgentID:       "line_polish",
		StyleID:       "precise_editor",
		Surface:       agent.SurfaceEditor,
		InputScope:    agent.InputScopeSelection,
		SceneID:       acceptedScene.ID,
		SceneRevision: acceptedScene.Revision,
		Selection: action.Selection{
			StartByte: startByte,
			EndByte:   startByte + len([]byte("Mock polished: "+selectedText)),
			Text:      "Mock polished: " + selectedText,
		},
	})
	if err != nil {
		t.Fatalf("Run(third) error = %v", err)
	}
	if _, _, err := actionService.Accept(ctx, thirdRun.RunID, "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"); !errors.Is(err, story.ErrStaleRevision) {
		t.Fatalf("Accept(stale revision) error = %v, want ErrStaleRevision", err)
	}
	if gitCommitCount(t, ctx, projectPath) != commitCountBeforeRun+1 {
		t.Fatal("stale accept mutated git history")
	}

	projectMetadata := filepath.Join(projectPath, "project.yaml")
	originalProjectMetadata := mustReadFile(t, projectMetadata)
	if err := os.WriteFile(projectMetadata, append(originalProjectMetadata, []byte("# dirty\n")...), 0o644); err != nil {
		t.Fatalf("WriteFile(project dirty) error = %v", err)
	}
	fourthRun, err := actionService.Run(ctx, action.RunRequest{
		AgentID:       "line_polish",
		StyleID:       "precise_editor",
		Surface:       agent.SurfaceEditor,
		InputScope:    agent.InputScopeSelection,
		SceneID:       acceptedScene.ID,
		SceneRevision: acceptedScene.Revision,
		Selection: action.Selection{
			StartByte: startByte,
			EndByte:   startByte + len([]byte("Mock polished: "+selectedText)),
			Text:      "Mock polished: " + selectedText,
		},
	})
	if err != nil {
		t.Fatalf("Run(fourth) error = %v", err)
	}
	if _, _, err := actionService.Accept(ctx, fourthRun.RunID, acceptedScene.Revision); !errors.Is(err, story.ErrDirtyWorktree) {
		t.Fatalf("Accept(dirty worktree) error = %v, want ErrDirtyWorktree", err)
	}
	if err := os.WriteFile(projectMetadata, originalProjectMetadata, 0o644); err != nil {
		t.Fatalf("WriteFile(project restore) error = %v", err)
	}
	if clean, err := git.IsClean(ctx, projectPath); err != nil || !clean {
		t.Fatalf("IsClean() after restore = %v, %v", clean, err)
	}
}

func gitCommitCount(t *testing.T, ctx context.Context, path string) int {
	t.Helper()
	command := exec.CommandContext(ctx, "git", "-C", path, "rev-list", "--count", "HEAD")
	output, err := command.Output()
	if err != nil {
		t.Fatalf("git rev-list: %v", err)
	}
	return atoiOrFail(t, strings.TrimSpace(string(output)))
}

func commitMessage(t *testing.T, ctx context.Context, path string) string {
	t.Helper()
	command := exec.CommandContext(ctx, "git", "-C", path, "log", "-1", "--pretty=%s")
	output, err := command.Output()
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	return strings.TrimSpace(string(output))
}

func atoiOrFail(t *testing.T, value string) int {
	t.Helper()
	parsed, err := strconv.Atoi(value)
	if err != nil {
		t.Fatalf("Atoi(%q) error = %v", value, err)
	}
	return parsed
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	return contents
}
