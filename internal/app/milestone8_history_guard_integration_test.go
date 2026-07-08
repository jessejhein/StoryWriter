// BDD Scenario: 8.1.2 - Reject unsafe branch state; 8.2.1 - List exact changed files; 8.4.1 - Promote selected files to main
// Requirements: M8-R04, M8-R05, M8-R12, M8-R17
// Test purpose: Real Git adapters fail closed when a managed experiment ref is
// force-moved to unrelated history, and the API returns safe conflicts without
// mutating canon or leaving the project on the rewritten experiment branch.

package app_test

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"storywork/internal/app"
)

// Test: comparison and promotion routes reject unrelated managed experiment
// history with safe conflict responses and no canon mutation.
// Requirements: M8-R04, M8-R05, M8-R12, M8-R17.
func TestMilestone8UnrelatedExperimentHistoryFailsClosed(t *testing.T) {
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
	}, http.StatusConflict, nil)

	getJSON(t, handler, "/api/branches/"+experimentID+"/comparison", http.StatusConflict, nil)
	putJSON(t, handler, http.MethodPost, "/api/branches/"+experimentID+"/ramifications", map[string]any{
		"goal":                     "Check rewritten history fallout.",
		"profile_id":               "local_test",
		"model":                    "ramification-model",
		"expected_main_head":       mainHeadBefore,
		"expected_experiment_head": unrelatedHead,
		"comparison_fingerprint":   "sha256:" + strings.Repeat("a", 64),
	}, http.StatusConflict, nil)
	putJSON(t, handler, http.MethodPost, "/api/branches/"+experimentID+"/promote", map[string]any{
		"paths":                    []string{scenePath},
		"expected_main_head":       mainHeadBefore,
		"expected_experiment_head": unrelatedHead,
		"comparison_fingerprint":   "sha256:" + strings.Repeat("a", 64),
	}, http.StatusConflict, nil)

	assertMainRefAndTreeUnchanged(t, projectPath, mainHeadBefore, mainTreeBefore)
	if gitActiveBranch(t, projectPath) != "main" {
		t.Fatalf("active branch = %q, want main", gitActiveBranch(t, projectPath))
	}
	if got := atomic.LoadInt64(providerCalls); got != 0 {
		t.Fatalf("provider calls = %d, want 0", got)
	}
	assertClean(t, projectPath)
}

// Test: comparison, analysis, promotion, switch, and discard reject related
// rewritten experiment history when the branch no longer descends from its
// recorded immutable base.
// Requirements: M8-R04, M8-R05, M8-R09, M8-R12, M8-R17.
func TestMilestone8RelatedRewrittenExperimentHistoryFailsClosed(t *testing.T) {
	configPath := t.TempDir()
	t.Setenv("STORYWORK_CONFIG_DIR", configPath)

	handler := app.NewHandler("test")
	projectPath := filepath.Join(t.TempDir(), "m8-related-history-guard")
	scenePath, experimentID, experimentBranch, experimentHead, providerCalls := createHistoryGuardExperiment(t, handler, projectPath)
	mainHeadBefore, mainTreeBefore := recordMainRefAndTree(t, projectPath)

	putJSON(t, handler, http.MethodPost, "/api/branches/switch", map[string]string{"target": "main"}, http.StatusOK, nil)
	relatedHead := createRelatedSiblingCommit(t, projectPath, experimentHead, scenePath)
	updateManagedRef(t, projectPath, experimentBranch, relatedHead, experimentHead)

	putJSON(t, handler, http.MethodPost, "/api/branches/switch", map[string]any{
		"target":        experimentID,
		"expected_head": relatedHead,
	}, http.StatusConflict, nil)
	getJSON(t, handler, "/api/branches/"+experimentID+"/comparison", http.StatusConflict, nil)
	putJSON(t, handler, http.MethodPost, "/api/branches/"+experimentID+"/ramifications", map[string]any{
		"goal":                     "Check related rewritten history fallout.",
		"profile_id":               "local_test",
		"model":                    "ramification-model",
		"expected_main_head":       mainHeadBefore,
		"expected_experiment_head": relatedHead,
		"comparison_fingerprint":   "sha256:" + strings.Repeat("b", 64),
	}, http.StatusConflict, nil)
	putJSON(t, handler, http.MethodPost, "/api/branches/"+experimentID+"/promote", map[string]any{
		"paths":                    []string{scenePath},
		"expected_main_head":       mainHeadBefore,
		"expected_experiment_head": relatedHead,
		"comparison_fingerprint":   "sha256:" + strings.Repeat("b", 64),
	}, http.StatusConflict, nil)
	putJSON(t, handler, http.MethodPost, "/api/branches/"+experimentID+"/discard", map[string]any{
		"expected_experiment_head": relatedHead,
	}, http.StatusConflict, nil)

	assertMainRefAndTreeUnchanged(t, projectPath, mainHeadBefore, mainTreeBefore)
	if gitActiveBranch(t, projectPath) != "main" {
		t.Fatalf("active branch = %q, want main", gitActiveBranch(t, projectPath))
	}
	if got := atomic.LoadInt64(providerCalls); got != 0 {
		t.Fatalf("provider calls = %d, want 0", got)
	}
	assertClean(t, projectPath)
}

// Test: missing, malformed, and stale private experiment base refs fail closed
// before provider work, checkout, discard, or canon mutation.
// Requirements: M8-R04, M8-R05, M8-R09, M8-R12, M8-R17.
func TestMilestone8PrivateBaseRefCorruptionFailsClosed(t *testing.T) {
	tests := []struct {
		name           string
		mutate         func(*testing.T, string, string, string)
		comparisonCode int
		switchCode     int
		discardCode    int
		analysisCode   int
	}{
		{
			name: "missing",
			mutate: func(t *testing.T, projectPath, experimentID, experimentHead string) {
				deleteExperimentBaseRef(t, projectPath, experimentID)
			},
			comparisonCode: http.StatusInternalServerError,
			switchCode:     http.StatusInternalServerError,
			discardCode:    http.StatusInternalServerError,
			analysisCode:   http.StatusInternalServerError,
		},
		{
			name: "malformed",
			mutate: func(t *testing.T, projectPath, experimentID, experimentHead string) {
				writeMalformedExperimentBaseRef(t, projectPath, experimentID)
			},
			comparisonCode: http.StatusInternalServerError,
			switchCode:     http.StatusInternalServerError,
			discardCode:    http.StatusInternalServerError,
			analysisCode:   http.StatusInternalServerError,
		},
		{
			name: "stale",
			mutate: func(t *testing.T, projectPath, experimentID, experimentHead string) {
				mainHeadAfter := gitRevParse(t, projectPath, "main")
				updateExperimentBaseRef(t, projectPath, experimentID, mainHeadAfter, experimentHead)
			},
			comparisonCode: http.StatusConflict,
			switchCode:     http.StatusConflict,
			discardCode:    http.StatusConflict,
			analysisCode:   http.StatusConflict,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			configPath := t.TempDir()
			t.Setenv("STORYWORK_CONFIG_DIR", configPath)

			handler := app.NewHandler("test")
			projectPath := filepath.Join(t.TempDir(), "m8-base-ref-"+testCase.name)
			scenePath, experimentID, _, experimentHead, providerCalls := createHistoryGuardExperiment(t, handler, projectPath)

			putJSON(t, handler, http.MethodPost, "/api/branches/switch", map[string]string{"target": "main"}, http.StatusOK, nil)
			saveSceneMarkdown(t, handler, strings.TrimSuffix(filepath.Base(scenePath), ".md"), loadScene(t, handler, strings.TrimSuffix(filepath.Base(scenePath), ".md")), "Advance canon before corrupting the base ref.\n")
			mainHeadBefore, mainTreeBefore := recordMainRefAndTree(t, projectPath)
			testCase.mutate(t, projectPath, experimentID, experimentHead)

			getJSON(t, handler, "/api/branches/"+experimentID+"/comparison", testCase.comparisonCode, nil)
			putJSON(t, handler, http.MethodPost, "/api/branches/switch", map[string]any{
				"target":        experimentID,
				"expected_head": experimentHead,
			}, testCase.switchCode, nil)
			putJSON(t, handler, http.MethodPost, "/api/branches/"+experimentID+"/ramifications", map[string]any{
				"goal":                     "Check base ref corruption fallout.",
				"profile_id":               "local_test",
				"model":                    "ramification-model",
				"expected_main_head":       gitRevParse(t, projectPath, "main"),
				"expected_experiment_head": experimentHead,
				"comparison_fingerprint":   "sha256:" + strings.Repeat("c", 64),
			}, testCase.analysisCode, nil)
			putJSON(t, handler, http.MethodPost, "/api/branches/"+experimentID+"/discard", map[string]any{
				"expected_experiment_head": experimentHead,
			}, testCase.discardCode, nil)

			assertMainRefAndTreeUnchanged(t, projectPath, mainHeadBefore, mainTreeBefore)
			if got := atomic.LoadInt64(providerCalls); got != 0 {
				t.Fatalf("provider calls = %d, want 0", got)
			}
			assertClean(t, projectPath)
		})
	}
}

func createHistoryGuardExperiment(t *testing.T, handler http.Handler, projectPath string) (scenePath, experimentID, experimentBranch, experimentHead string, providerCalls *int64) {
	t.Helper()
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
	scenePath = "scenes/" + sceneID + ".md"

	providerCalls = setupM8RamificationProvider(t, scenePath)
	scene := loadScene(t, handler, sceneID)
	saveSceneMarkdown(t, handler, sceneID, scene, "Canon prose remains on main before the experiment history is rewritten.\n")

	var created map[string]any
	putJSON(t, handler, http.MethodPost, "/api/branches", map[string]string{"name": "History Guard"}, http.StatusCreated, &created)
	return scenePath, created["active_experiment_id"].(string), created["active_branch"].(string), created["experiment_head"].(string), providerCalls
}

func createRelatedSiblingCommit(t *testing.T, projectPath, originalHead, scenePath string) string {
	t.Helper()
	parent := gitRevParse(t, projectPath, originalHead+"^")
	if output, err := exec.Command("git", "-C", projectPath, "checkout", "-b", "rewrite-history-temp", parent).CombinedOutput(); err != nil {
		t.Fatalf("git checkout -b rewrite-history-temp: %v: %s", err, output)
	}
	if err := os.WriteFile(filepath.Join(projectPath, scenePath), []byte("---\nid: related-rewrite\nchapter_id: ch_0123456789abcdef0123\ntitle: Related Rewrite\norder: 1\n---\n\nRelated rewritten experiment prose.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("git", "-C", projectPath, "add", "--", scenePath).CombinedOutput(); err != nil {
		t.Fatalf("git add related rewrite: %v: %s", err, output)
	}
	if output, err := exec.Command("git", "-C", projectPath, "-c", "user.name=test", "-c", "user.email=test@example.test", "commit", "-m", "Create related rewritten history").CombinedOutput(); err != nil {
		t.Fatalf("git commit related rewrite: %v: %s", err, output)
	}
	head := gitRevParse(t, projectPath, "HEAD")
	if output, err := exec.Command("git", "-C", projectPath, "checkout", "main").CombinedOutput(); err != nil {
		t.Fatalf("git checkout main: %v: %s", err, output)
	}
	return head
}

func experimentBaseRef(experimentID string) string {
	return "refs/storywork/experiment-base/" + experimentID
}

func deleteExperimentBaseRef(t *testing.T, projectPath, experimentID string) {
	t.Helper()
	ref := experimentBaseRef(experimentID)
	old := gitRevParse(t, projectPath, ref)
	if output, err := exec.Command("git", "-C", projectPath, "update-ref", "-d", ref, old).CombinedOutput(); err != nil {
		t.Fatalf("git update-ref -d %s: %v: %s", ref, err, output)
	}
}

func updateExperimentBaseRef(t *testing.T, projectPath, experimentID, newHead, oldHead string) {
	t.Helper()
	ref := experimentBaseRef(experimentID)
	if output, err := exec.Command("git", "-C", projectPath, "update-ref", ref, newHead, oldHead).CombinedOutput(); err != nil {
		t.Fatalf("git update-ref %s: %v: %s", ref, err, output)
	}
}

func writeMalformedExperimentBaseRef(t *testing.T, projectPath, experimentID string) {
	t.Helper()
	refPath := filepath.Join(projectPath, ".git", "refs", "storywork", "experiment-base", experimentID)
	if err := os.WriteFile(refPath, []byte("not-a-commit\n"), 0o644); err != nil {
		t.Fatalf("write malformed base ref: %v", err)
	}
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
