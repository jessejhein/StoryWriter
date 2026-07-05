// BDD Scenario: 8.1.2 - Reject unsafe branch state; 8.2.1 - List exact changed files; 8.4.1 - Promote selected files to main
// Requirements: M8-R04, M8-R05, M8-R12, M8-R17
// Test purpose: Real Git adapters fail closed when a managed experiment ref is
// force-moved to unrelated history, and the API returns safe conflicts without
// mutating canon.

package app_test

import (
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"storywork/internal/app"
)

// Test: comparison and promotion routes reject rewritten managed experiment
// history with safe conflict responses and no canon mutation.
// Requirements: M8-R04, M8-R05, M8-R12, M8-R17.
func TestMilestone8RewrittenExperimentHistoryFailsClosed(t *testing.T) {
	configPath := t.TempDir()
	t.Setenv("STORYWORK_CONFIG_DIR", configPath)

	handler := app.NewHandler("test")
	projectPath := filepath.Join(t.TempDir(), "m8-history-guard")
	putJSON(t, handler, http.MethodPost, "/api/projects", map[string]any{"name": "M8 History Guard", "path": projectPath}, http.StatusCreated, nil)

	var outline struct {
		Outline struct {
			Arcs []struct {
				ID       string `json:"id"`
				Chapters []struct {
					ID     string `json:"id"`
					Scenes []struct {
						ID string `json:"id"`
					} `json:"scenes"`
				} `json:"chapters"`
			} `json:"arcs"`
		} `json:"outline"`
	}
	putJSON(t, handler, http.MethodPost, "/api/arcs", map[string]string{"title": "Act"}, http.StatusCreated, &outline)
	arcID := outline.Outline.Arcs[0].ID
	putJSON(t, handler, http.MethodPost, "/api/chapters", map[string]any{"arc_id": arcID, "title": "Chapter"}, http.StatusCreated, &outline)
	chapterID := outline.Outline.Arcs[0].Chapters[0].ID
	putJSON(t, handler, http.MethodPost, "/api/scenes", map[string]any{"chapter_id": chapterID, "title": "Opening"}, http.StatusCreated, &outline)
	sceneID := outline.Outline.Arcs[0].Chapters[0].Scenes[0].ID
	scenePath := "scenes/" + sceneID + ".md"

	providerCalls := setupM8RamificationProvider(t, scenePath)
	scene := loadScene(t, handler, sceneID)
	saveSceneMarkdown(t, handler, sceneID, scene, "Canon prose remains on main before the experiment history is rewritten.\n")

	var created map[string]any
	putJSON(t, handler, http.MethodPost, "/api/branches", map[string]string{"name": "History Guard"}, http.StatusCreated, &created)
	experimentID := created["active_experiment_id"].(string)
	experimentBranch := created["active_branch"].(string)
	experimentHead := created["experiment_head"].(string)

	mainHeadBefore, mainTreeBefore := recordMainRefAndTree(t, projectPath)
	if before := atomic.LoadInt64(providerCalls); before != 0 {
		t.Fatalf("provider calls before test = %d, want 0", before)
	}

	putJSON(t, handler, http.MethodPost, "/api/branches/switch", map[string]string{"target": "main"}, http.StatusOK, nil)
	unrelatedHead := createUnrelatedCommit(t, projectPath)
	updateManagedRef(t, projectPath, experimentBranch, unrelatedHead, experimentHead)

	putJSON(t, handler, http.MethodPost, "/api/branches/switch", map[string]any{
		"target":        experimentID,
		"expected_head": unrelatedHead,
	}, http.StatusOK, nil)

	getJSON(t, handler, "/api/branches/"+experimentID+"/comparison", http.StatusConflict, nil)
	putJSON(t, handler, http.MethodPost, "/api/branches/"+experimentID+"/promote", map[string]any{
		"paths":                    []string{scenePath},
		"expected_main_head":       mainHeadBefore,
		"expected_experiment_head": unrelatedHead,
		"comparison_fingerprint":   "sha256:" + strings.Repeat("a", 64),
	}, http.StatusConflict, nil)

	assertMainRefAndTreeUnchanged(t, projectPath, mainHeadBefore, mainTreeBefore)
	if gitActiveBranch(t, projectPath) != experimentBranch {
		t.Fatalf("active branch = %q, want %q", gitActiveBranch(t, projectPath), experimentBranch)
	}
	if got := atomic.LoadInt64(providerCalls); got != 0 {
		t.Fatalf("provider calls = %d, want 0", got)
	}
	assertClean(t, projectPath)
}

func createUnrelatedCommit(t *testing.T, projectPath string) string {
	t.Helper()
	if output, err := exec.Command("git", "-C", projectPath, "checkout", "--orphan", "rewrite-history-temp").CombinedOutput(); err != nil {
		t.Fatalf("git checkout --orphan: %v: %s", err, output)
	}
	if output, err := exec.Command("git", "-C", projectPath, "commit", "--allow-empty", "-m", "Create unrelated history").CombinedOutput(); err != nil {
		t.Fatalf("git commit --allow-empty: %v: %s", err, output)
	}
	head := gitRevParse(t, projectPath, "HEAD")
	if output, err := exec.Command("git", "-C", projectPath, "checkout", "main").CombinedOutput(); err != nil {
		t.Fatalf("git checkout main: %v: %s", err, output)
	}
	return head
}

func updateManagedRef(t *testing.T, projectPath, branchName, newHead, oldHead string) {
	t.Helper()
	if output, err := exec.Command("git", "-C", projectPath, "update-ref", "refs/heads/"+branchName, newHead, oldHead).CombinedOutput(); err != nil {
		t.Fatalf("git update-ref %s: %v: %s", branchName, err, output)
	}
}
