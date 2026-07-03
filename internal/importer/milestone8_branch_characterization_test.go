// BDD Scenario: 8.1.3 - Continue normal work in the experiment
// Requirements: M8-R03, M8-R19
// Test purpose: Real Git integration proves import and review mutations commit
// only to the checked-out experiment branch while main ref and tree remain
// byte-for-byte unchanged.

package importer

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"storywork/internal/extract"
	"storywork/internal/gitstore"
	"storywork/internal/index"
	"storywork/internal/project"
	"storywork/internal/story"
	"storywork/internal/storyfile"
	"storywork/internal/workspace"
)

const milestone8ExperimentRef = "branch/test-exp-0123abcd"

type milestone8StoryIDGenerator struct {
	ids []string
}

func (g *milestone8StoryIDGenerator) Next(_ story.NodeKind) (string, error) {
	if len(g.ids) == 0 {
		return "", errors.New("no story IDs remaining")
	}
	next := g.ids[0]
	g.ids = g.ids[1:]
	return next, nil
}

func setupMilestone8ImporterProject(t *testing.T) (context.Context, string, string, *Service, *story.Service) {
	t.Helper()

	ctx := context.Background()
	projectPath := filepath.Join(t.TempDir(), "milestone8-import-novel")
	sourcePath := filepath.Join(t.TempDir(), "import-source")
	writeTestFile(t, filepath.Join(sourcePath, "notes.md"), "# Act One\nMara arrives.\n")

	git := gitstore.New("git")
	disposableIndex := index.New()
	projectService := project.NewService(
		git,
		disposableIndex,
		func() time.Time { return time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC) },
	)
	created, err := projectService.Create(ctx, project.CreateRequest{Name: "Milestone 8 Import Novel", Path: projectPath})
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
		&milestone8StoryIDGenerator{ids: []string{
			"arc_00000000000000000001",
			"ch_00000000000000000001",
			"scn_00000000000000000001",
			"char_0123456789abcdef0123",
		}},
	)
	importerService := NewService(
		session,
		git,
		disposableIndex,
		NewSourceStore(),
		&fakeImporterIDs{
			importIDs:    []string{"imp_0123456789abcdef0123"},
			candidateIDs: []string{"cand_0123456789abcdef0101", "cand_0123456789abcdef0102"},
		},
		func() time.Time { return time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC) },
	).WithExtractor(&fakeExtractor{result: extract.Result{
		Proposals: []extract.Proposal{
			{Kind: "arc", Arc: &extract.ArcProposal{Kind: "arc", LocalID: "arc_local", Title: "Act One"}},
			{Kind: "codex", Codex: &extract.CodexProposal{Kind: "codex", LocalID: "codex_local", Type: "character", Name: "Mara Venn", Aliases: []string{"Mara"}, Tags: []string{"pilot"}, Description: "A cautious salvage pilot."}},
		},
	}}).WithStoryMutator(storyService)

	return ctx, projectPath, sourcePath, importerService, storyService
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
}

// Test: import snapshot commits to active branch only.
// Requirements: M8-R03, M8-R19.
func TestMilestone8ImportSnapshotCommitsToActiveBranchOnly(t *testing.T) {
	t.Parallel()

	ctx, projectPath, sourcePath, importerService, _ := setupMilestone8ImporterProject(t)
	mainHeadBefore, mainTreeBefore := recordMainRefAndTree(t, ctx, projectPath)
	checkoutMilestone8ExperimentBranch(t, ctx, projectPath)

	response, err := importerService.ImportDirectory(ctx, sourcePath)
	if err != nil {
		t.Fatalf("ImportDirectory() error = %v", err)
	}
	if response.Import.ID != "imp_0123456789abcdef0123" {
		t.Fatalf("ImportDirectory() import id = %q", response.Import.ID)
	}

	assertMainRefAndTreeUnchanged(t, ctx, projectPath, mainHeadBefore, mainTreeBefore)
	assertCommitOnExperimentNotMain(t, ctx, projectPath, mainHeadBefore, "Import notes snapshot imp_0123456789abcdef0123")

	output, err := exec.CommandContext(ctx, "git", "-C", projectPath, "show", "main:imports/raw/imp_0123456789abcdef0123").CombinedOutput()
	if err == nil {
		t.Fatalf("main tree contains experiment import snapshot: %s", output)
	}
}

// Test: import review mutation commits to active branch only.
// Requirements: M8-R03, M8-R19.
func TestMilestone8ImportReviewMutationCommitsToActiveBranchOnly(t *testing.T) {
	t.Parallel()

	ctx, projectPath, sourcePath, importerService, _ := setupMilestone8ImporterProject(t)
	mainHeadBefore, mainTreeBefore := recordMainRefAndTree(t, ctx, projectPath)
	checkoutMilestone8ExperimentBranch(t, ctx, projectPath)

	imported, err := importerService.ImportDirectory(ctx, sourcePath)
	if err != nil {
		t.Fatalf("ImportDirectory() error = %v", err)
	}
	chunks, err := importerService.ListChunks(ctx, imported.Import.ID)
	if err != nil {
		t.Fatalf("ListChunks() error = %v", err)
	}
	extracted, err := importerService.Extract(ctx, ExtractRequest{
		ImportID:  imported.Import.ID,
		ChunkIDs:  []string{chunks[0].ID},
		Mode:      extract.ModeStructure,
		ProfileID: "local_ollama",
		Model:     "qwen2.5:7b",
	})
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if len(extracted.Candidates) != 2 {
		t.Fatalf("Extract() candidate count = %d, want 2", len(extracted.Candidates))
	}

	assertMainRefAndTreeUnchanged(t, ctx, projectPath, mainHeadBefore, mainTreeBefore)
	assertCommitOnExperimentNotMain(t, ctx, projectPath, mainHeadBefore, "Extract import candidates "+imported.Import.ID)

	arcCandidate := extracted.Candidates[0]
	edited, err := importerService.UpdateCandidate(ctx, arcCandidate.ID, arcCandidate.Revision, CandidateProposal{
		Arc: &ArcProposal{Title: "Act One Revised"},
	})
	if err != nil {
		t.Fatalf("UpdateCandidate() error = %v", err)
	}
	if edited.Proposal.Arc.Title != "Act One Revised" {
		t.Fatalf("edited candidate = %#v", edited)
	}

	assertMainRefAndTreeUnchanged(t, ctx, projectPath, mainHeadBefore, mainTreeBefore)
	assertCommitOnExperimentNotMain(t, ctx, projectPath, mainHeadBefore, "Edit import candidate "+arcCandidate.ID)

	accepted, refs, err := importerService.AcceptCandidate(ctx, edited.ID, edited.Revision)
	if err != nil {
		t.Fatalf("AcceptCandidate() error = %v", err)
	}
	if accepted.Status != CandidateStatusAccepted || len(refs) != 1 || refs[0].ID != "arc_00000000000000000001" {
		t.Fatalf("accepted candidate = %#v refs=%v", accepted, refs)
	}

	assertMainRefAndTreeUnchanged(t, ctx, projectPath, mainHeadBefore, mainTreeBefore)
	assertCommitOnExperimentNotMain(t, ctx, projectPath, mainHeadBefore, "Accept import candidate "+edited.ID)

	output, err := exec.CommandContext(ctx, "git", "-C", projectPath, "show", "main:arcs/arc_00000000000000000001.yaml").CombinedOutput()
	if err == nil {
		t.Fatalf("main tree contains experiment accepted arc: %s", output)
	}
}
