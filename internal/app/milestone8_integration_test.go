// BDD Scenario: 8.1.1 - Create from current canon
// Requirements: M8-R01 through M8-R20
// Test purpose: Real adapters prove experiment lifecycle, comparison, analysis,
// promotion, and discard without mutating canon unexpectedly. Acceptance M8-33.

package app_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"storywork/internal/app"
	"storywork/internal/projectcheck"
)

func TestMilestone8AcceptanceM833HappyPath(t *testing.T) {
	configPath := t.TempDir()
	t.Setenv("STORYWORK_CONFIG_DIR", configPath)

	handler := app.NewHandler("test")
	projectPath := filepath.Join(t.TempDir(), "m8-novel")
	putJSON(t, handler, http.MethodPost, "/api/projects", map[string]any{"name": "M8 Novel", "path": projectPath}, http.StatusCreated, nil)

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
	canonMarkdown := "Canon opening prose stays on main while the hall remains quiet and still before any experiment work begins.\n"
	saveSceneMarkdown(t, handler, sceneID, scene, canonMarkdown)
	scene = loadScene(t, handler, sceneID)

	mainHeadBefore, mainTreeBefore := recordMainRefAndTree(t, projectPath)
	mainCommitsBefore := gitRevCount(t, projectPath)

	t.Run("create_experiment_from_main_leaves_canon_unchanged", func(t *testing.T) {
		var status map[string]any
		putJSON(t, handler, http.MethodPost, "/api/branches", map[string]string{"name": "What If"}, http.StatusCreated, &status)
		if status["active_kind"] != "experiment" {
			t.Fatalf("status = %#v", status)
		}
		if status["active_experiment_id"] == nil || status["active_experiment_id"] == "" {
			t.Fatal("create returned no experiment_id")
		}
		assertMainRefAndTreeUnchanged(t, projectPath, mainHeadBefore, mainTreeBefore)
		if gitRevCount(t, projectPath) != mainCommitsBefore {
			t.Fatal("experiment creation advanced main history")
		}
		assertClean(t, projectPath)
	})

	var experimentID string
	var experimentHead string
	t.Run("scene_mutation_commits_to_experiment_only", func(t *testing.T) {
		var status map[string]any
		getJSON(t, handler, "/api/branches/status", http.StatusOK, &status)
		experimentID = status["active_experiment_id"].(string)
		experimentHead = status["experiment_head"].(string)
		if !strings.HasPrefix(status["active_branch"].(string), "branch/") {
			t.Fatalf("active branch = %#v", status["active_branch"])
		}

		experimentMarkdown := "Experiment opening prose changes the mood while the hall stays tense and still after the rewrite on the branch.\n"
		saveSceneMarkdown(t, handler, sceneID, scene, experimentMarkdown)
		scene = loadScene(t, handler, sceneID)

		assertMainRefAndTreeUnchanged(t, projectPath, mainHeadBefore, mainTreeBefore)
		if gitRevParse(t, projectPath, "HEAD") == mainHeadBefore {
			t.Fatal("experiment branch did not advance")
		}
		if gitRevParse(t, projectPath, "main") != mainHeadBefore {
			t.Fatal("main advanced during experiment scene save")
		}
		mainScene, err := exec.Command("git", "-C", projectPath, "show", "main:"+scenePath).Output()
		if err != nil {
			t.Fatal(err)
		}
		mainSceneText := string(mainScene)
		if !strings.Contains(mainSceneText, canonMarkdown) {
			t.Fatalf("main scene missing canon prose: %q", mainScene)
		}
		if strings.Contains(mainSceneText, "Experiment opening prose") {
			t.Fatalf("main scene contains experiment prose: %q", mainScene)
		}
		experimentHead = gitRevParse(t, projectPath, "HEAD")
		assertClean(t, projectPath)
	})

	var comparison struct {
		MainHead       string `json:"main_head"`
		ExperimentHead string `json:"experiment_head"`
		Fingerprint    string `json:"fingerprint"`
		Files          []struct {
			Path   string `json:"path"`
			Status string `json:"status"`
		} `json:"files"`
	}
	t.Run("comparison_reads_blobs_without_checkout", func(t *testing.T) {
		beforeBranch := gitActiveBranch(t, projectPath)
		beforeExperimentHead := gitRevParse(t, projectPath, "HEAD")
		beforeCalls := atomic.LoadInt64(providerCalls)

		getJSON(t, handler, "/api/branches/"+experimentID+"/comparison", http.StatusOK, &comparison)
		if comparison.MainHead != mainHeadBefore || comparison.ExperimentHead != experimentHead {
			t.Fatalf("comparison heads = %#v", comparison)
		}
		if len(comparison.Files) == 0 {
			t.Fatal("comparison returned no changed files")
		}
		foundScene := false
		for _, file := range comparison.Files {
			if file.Path == scenePath && file.Status == "modified" {
				foundScene = true
			}
		}
		if !foundScene {
			t.Fatalf("comparison files = %#v", comparison.Files)
		}

		var fileComparison map[string]any
		getJSON(t, handler, "/api/branches/"+experimentID+"/comparison/file?path="+scenePath, http.StatusOK, &fileComparison)
		canon := fileComparison["canon"].(map[string]any)
		experiment := fileComparison["experiment"].(map[string]any)
		if !strings.Contains(canon["text"].(string), canonMarkdown) {
			t.Fatalf("canon side = %#v", canon)
		}
		if !strings.Contains(experiment["text"].(string), scene["markdown"].(string)) {
			t.Fatalf("experiment side = %#v", experiment)
		}

		if gitActiveBranch(t, projectPath) != beforeBranch {
			t.Fatalf("comparison changed active branch from %q", beforeBranch)
		}
		if gitRevParse(t, projectPath, "HEAD") != beforeExperimentHead {
			t.Fatal("comparison changed checked-out HEAD")
		}
		assertMainRefAndTreeUnchanged(t, projectPath, mainHeadBefore, mainTreeBefore)
		if atomic.LoadInt64(providerCalls) != beforeCalls {
			t.Fatal("comparison invoked provider")
		}
	})

	t.Run("ramification_analysis_returns_strict_findings", func(t *testing.T) {
		beforeCommits := gitRevCount(t, projectPath)
		beforeCalls := atomic.LoadInt64(providerCalls)

		var analysis map[string]any
		putJSON(t, handler, http.MethodPost, "/api/branches/"+experimentID+"/ramifications", map[string]any{
			"goal":                     "Explore consequences of the opening rewrite.",
			"profile_id":               "local_test",
			"model":                    "test-model",
			"expected_main_head":       comparison.MainHead,
			"expected_experiment_head": comparison.ExperimentHead,
			"comparison_fingerprint":   comparison.Fingerprint,
		}, http.StatusOK, &analysis)

		if atomic.LoadInt64(providerCalls) != beforeCalls+1 {
			t.Fatalf("provider calls = %d, want %d", atomic.LoadInt64(providerCalls), beforeCalls+1)
		}
		if analysis["summary"] == nil || analysis["summary"] == "" {
			t.Fatalf("analysis = %#v", analysis)
		}
		findings, ok := analysis["findings"].([]any)
		if !ok || len(findings) != 1 {
			t.Fatalf("findings = %#v", analysis["findings"])
		}
		finding := findings[0].(map[string]any)
		if finding["category"] != "continuity" || finding["severity"] != "medium" {
			t.Fatalf("finding = %#v", finding)
		}
		affected := finding["affected_paths"].([]any)
		if len(affected) != 1 || affected[0] != scenePath {
			t.Fatalf("affected_paths = %#v", affected)
		}
		if gitRevCount(t, projectPath) != beforeCommits {
			t.Fatal("ramification analysis mutated repository history")
		}
		assertMainRefAndTreeUnchanged(t, projectPath, mainHeadBefore, mainTreeBefore)
		assertClean(t, projectPath)
	})

	t.Run("promote_selected_files_creates_one_main_commit_with_trailers", func(t *testing.T) {
		if err := projectcheck.New().ValidateProject(context.Background(), projectPath); err != nil {
			t.Fatalf("pre-promotion validation failed: %v", err)
		}
		startMainCommits := gitRevCountRef(t, projectPath, "main")
		var promoted map[string]any
		putJSON(t, handler, http.MethodPost, "/api/branches/"+experimentID+"/promote", map[string]any{
			"paths":                    []string{scenePath},
			"expected_main_head":       comparison.MainHead,
			"expected_experiment_head": comparison.ExperimentHead,
			"comparison_fingerprint":   comparison.Fingerprint,
		}, http.StatusOK, &promoted)

		if gitRevCountRef(t, projectPath, "main") != startMainCommits+1 {
			t.Fatalf("main commit count = %d, want %d", gitRevCountRef(t, projectPath, "main"), startMainCommits+1)
		}
		message := gitShowBody(t, projectPath)
		if !strings.Contains(message, "Storywork-Experiment-ID: "+experimentID) {
			t.Fatalf("commit missing experiment trailer: %q", message)
		}
		if !strings.Contains(message, "Storywork-Source-Commit: "+comparison.ExperimentHead) {
			t.Fatalf("commit missing source trailer: %q", message)
		}
		if !strings.Contains(message, "Storywork-Base-Commit: "+comparison.MainHead) {
			t.Fatalf("commit missing base trailer: %q", message)
		}
		mainScene, err := exec.Command("git", "-C", projectPath, "show", "main:"+scenePath).Output()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(mainScene), scene["markdown"].(string)) {
			t.Fatalf("promoted scene = %q", mainScene)
		}
		if gitActiveBranch(t, projectPath) != "main" {
			t.Fatalf("active branch = %q, want main", gitActiveBranch(t, projectPath))
		}
		assertClean(t, projectPath)
	})

	t.Run("discard_switches_to_main_and_deletes_experiment", func(t *testing.T) {
		mainHeadAfterPromote := gitRevParse(t, projectPath, "main")
		var status map[string]any
		putJSON(t, handler, http.MethodPost, "/api/branches/"+experimentID+"/discard", map[string]any{
			"expected_experiment_head": experimentHead,
		}, http.StatusOK, &status)
		if status["active_branch"] != "main" || status["active_kind"] != "canon" {
			t.Fatalf("status = %#v", status)
		}
		if gitRevParse(t, projectPath, "main") != mainHeadAfterPromote {
			t.Fatal("discard changed main history")
		}
		var listed struct {
			Experiments []map[string]any `json:"experiments"`
		}
		getJSON(t, handler, "/api/branches", http.StatusOK, &listed)
		for _, experiment := range listed.Experiments {
			if experiment["experiment_id"] == experimentID {
				t.Fatalf("experiment still listed: %#v", listed.Experiments)
			}
		}
		refs, err := exec.Command("git", "-C", projectPath, "for-each-ref", "--format=%(refname:short)", "refs/heads/branch/").Output()
		if err != nil {
			t.Fatal(err)
		}
		for _, ref := range strings.Fields(string(refs)) {
			if strings.Contains(ref, string(experimentID[len("brn_"):])) {
				t.Fatalf("experiment ref still exists: %q", ref)
			}
		}
		assertClean(t, projectPath)
	})
}

// BDD Scenario: 8.4.1 - Promote selected files after unrelated canon advance
// Requirements: M8-R05, M8-R12, M8-R13, M8-R14, M8-R15, M8-R16, M8-R20
// Test purpose: Real adapters prove selected promotion succeeds when main
// advanced after the fork on a different allowed canonical path.
func TestMilestone8AcceptanceM835PromotionAllowsUnrelatedMainAdvancement(t *testing.T) {
	configPath := t.TempDir()
	t.Setenv("STORYWORK_CONFIG_DIR", configPath)

	handler := app.NewHandler("test")
	projectPath := filepath.Join(t.TempDir(), "m8-unrelated-main-advance")
	putJSON(t, handler, http.MethodPost, "/api/projects", map[string]any{"name": "M8 Unrelated Main Advance", "path": projectPath}, http.StatusCreated, nil)

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
	sceneAID := outline.Outline.Arcs[0].Chapters[0].Scenes[0].ID
	sceneAPath := "scenes/" + sceneAID + ".md"

	sceneA := loadScene(t, handler, sceneAID)
	saveSceneMarkdown(t, handler, sceneAID, sceneA, "Canon opening remains stable before any branch work begins.\n")
	sceneA = loadScene(t, handler, sceneAID)

	var createStatus map[string]any
	putJSON(t, handler, http.MethodPost, "/api/branches", map[string]string{"name": "Selective Promotion"}, http.StatusCreated, &createStatus)
	experimentID := createStatus["active_experiment_id"].(string)

	saveSceneMarkdown(t, handler, sceneAID, sceneA, "Experiment opening changes on the branch while canon stays untouched.\n")
	sceneA = loadScene(t, handler, sceneAID)
	experimentHead := gitRevParse(t, projectPath, "HEAD")

	putJSON(t, handler, http.MethodPost, "/api/branches/switch", map[string]string{"target": "main"}, http.StatusOK, nil)

	putJSON(t, handler, http.MethodPost, "/api/scenes", map[string]any{"chapter_id": chapterID, "title": "Aftermath"}, http.StatusCreated, &outline)
	sceneBID := outline.Outline.Arcs[0].Chapters[0].Scenes[1].ID
	sceneBPath := "scenes/" + sceneBID + ".md"
	sceneB := loadScene(t, handler, sceneBID)
	saveSceneMarkdown(t, handler, sceneBID, sceneB, "Main-only aftermath appears after the fork and must remain on canon.\n")
	mainHeadAfterSceneB := gitRevParse(t, projectPath, "main")

	putJSON(t, handler, http.MethodPost, "/api/branches/switch", map[string]any{
		"target":        experimentID,
		"expected_head": experimentHead,
	}, http.StatusOK, nil)

	var comparison struct {
		MainHead       string `json:"main_head"`
		ExperimentHead string `json:"experiment_head"`
		BaseHead       string `json:"base_head"`
		Fingerprint    string `json:"fingerprint"`
		Files          []struct {
			Path   string `json:"path"`
			Status string `json:"status"`
		} `json:"files"`
	}
	getJSON(t, handler, "/api/branches/"+experimentID+"/comparison", http.StatusOK, &comparison)

	if comparison.ExperimentHead != experimentHead {
		t.Fatalf("comparison experiment_head = %q, want %q", comparison.ExperimentHead, experimentHead)
	}

	foundSceneAModified := false
	foundSceneBDeleted := false
	for _, file := range comparison.Files {
		if file.Path == sceneAPath && file.Status == "modified" {
			foundSceneAModified = true
		}
		if file.Path == sceneBPath && file.Status == "deleted" {
			foundSceneBDeleted = true
		}
	}
	if !foundSceneAModified {
		t.Fatalf("comparison files missing modified experiment path: %#v", comparison.Files)
	}
	if !foundSceneBDeleted {
		t.Fatalf("comparison files missing deleted main-only path: %#v", comparison.Files)
	}

	var sceneBComparison struct {
		Canon struct {
			Exists bool   `json:"exists"`
			Text   string `json:"text"`
		} `json:"canon"`
		Experiment struct {
			Exists bool   `json:"exists"`
			Text   string `json:"text"`
		} `json:"experiment"`
	}
	getJSON(t, handler, "/api/branches/"+experimentID+"/comparison/file?path="+sceneBPath, http.StatusOK, &sceneBComparison)
	if !sceneBComparison.Canon.Exists || !strings.Contains(sceneBComparison.Canon.Text, "Main-only aftermath appears after the fork") {
		t.Fatalf("scene B canon side = %#v", sceneBComparison.Canon)
	}
	if sceneBComparison.Experiment.Exists {
		t.Fatalf("scene B experiment side unexpectedly exists: %#v", sceneBComparison.Experiment)
	}

	mainCommitsBeforePromotion := gitRevCountRef(t, projectPath, "main")
	var promoted struct {
		MainHead      string   `json:"main_head"`
		PromotedPaths []string `json:"promoted_paths"`
		ExperimentID  string   `json:"experiment_id"`
	}
	putJSON(t, handler, http.MethodPost, "/api/branches/"+experimentID+"/promote", map[string]any{
		"paths":                    []string{sceneAPath},
		"expected_main_head":       comparison.MainHead,
		"expected_experiment_head": comparison.ExperimentHead,
		"comparison_fingerprint":   comparison.Fingerprint,
	}, http.StatusOK, &promoted)

	if gitRevCountRef(t, projectPath, "main") != mainCommitsBeforePromotion+1 {
		t.Fatalf("main commit count = %d, want %d", gitRevCountRef(t, projectPath, "main"), mainCommitsBeforePromotion+1)
	}
	if len(promoted.PromotedPaths) != 1 || promoted.PromotedPaths[0] != sceneAPath {
		t.Fatalf("promoted paths = %#v", promoted.PromotedPaths)
	}
	if promoted.ExperimentID != experimentID {
		t.Fatalf("promoted experiment_id = %q, want %q", promoted.ExperimentID, experimentID)
	}

	message := gitShowBody(t, projectPath)
	if !strings.Contains(message, "Storywork-Experiment-ID: "+experimentID) {
		t.Fatalf("commit missing experiment trailer: %q", message)
	}
	if !strings.Contains(message, "Storywork-Source-Commit: "+comparison.ExperimentHead) {
		t.Fatalf("commit missing source trailer: %q", message)
	}
	if !strings.Contains(message, "Storywork-Base-Commit: "+comparison.BaseHead) {
		t.Fatalf("commit missing base trailer: %q", message)
	}
	if comparison.BaseHead == mainHeadAfterSceneB {
		t.Fatalf("comparison base_head = %q, want merge-base distinct from refreshed main head %q", comparison.BaseHead, mainHeadAfterSceneB)
	}

	mainSceneA, err := exec.Command("git", "-C", projectPath, "show", "main:"+sceneAPath).Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mainSceneA), sceneA["markdown"].(string)) {
		t.Fatalf("main scene A missing experiment content: %q", mainSceneA)
	}
	mainSceneB, err := exec.Command("git", "-C", projectPath, "show", "main:"+sceneBPath).Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mainSceneB), "Main-only aftermath appears after the fork") {
		t.Fatalf("main scene B missing main-only content: %q", mainSceneB)
	}
	if gitActiveBranch(t, projectPath) != "main" {
		t.Fatalf("active branch = %q, want main", gitActiveBranch(t, projectPath))
	}
	assertClean(t, projectPath)
}

// BDD Scenario: 8.3.3 - Reject stale, oversized, or malformed analysis
// Requirements: M8-R09, M8-R12, M8-R13, M8-R14, M8-R15
// Test purpose: Adversarial guards stop stale refs, canon conflicts, and
// invalid promotion before provider or checkout side effects. Acceptance M8-34.

func TestMilestone8AcceptanceM834Adversarial(t *testing.T) {
	configPath := t.TempDir()
	t.Setenv("STORYWORK_CONFIG_DIR", configPath)

	handler := app.NewHandler("test")
	projectPath := filepath.Join(t.TempDir(), "m8-adversarial")
	putJSON(t, handler, http.MethodPost, "/api/projects", map[string]any{"name": "M8 Adversarial", "path": projectPath}, http.StatusCreated, nil)

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
	putJSON(t, handler, http.MethodPost, "/api/scenes", map[string]any{"chapter_id": chapterID, "title": "Conflict"}, http.StatusCreated, &outline)
	sceneID := outline.Outline.Arcs[0].Chapters[0].Scenes[0].ID
	scenePath := "scenes/" + sceneID + ".md"
	providerCalls := setupM8RamificationProvider(t, scenePath)

	scene := loadScene(t, handler, sceneID)
	saveSceneMarkdown(t, handler, sceneID, scene, "Canon prose remains stable while the council waits and the room stays quiet before any branch edits happen.\n")
	scene = loadScene(t, handler, sceneID)

	var createStatus map[string]any
	putJSON(t, handler, http.MethodPost, "/api/branches", map[string]string{"name": "Conflict Test"}, http.StatusCreated, &createStatus)
	experimentID := createStatus["active_experiment_id"].(string)

	saveSceneMarkdown(t, handler, sceneID, scene, "Experiment prose shifts the tone while the council waits and the room stays tense after the branch-only rewrite.\n")
	scene = loadScene(t, handler, sceneID)

	var comparison map[string]any
	getJSON(t, handler, "/api/branches/"+experimentID+"/comparison", http.StatusOK, &comparison)

	t.Run("stale_fingerprint_and_refs_rejected_before_provider_or_checkout", func(t *testing.T) {
		beforeCalls := atomic.LoadInt64(providerCalls)
		beforeBranch := gitActiveBranch(t, projectPath)
		beforeMain := gitRevParse(t, projectPath, "main")
		beforeCommits := gitRevCount(t, projectPath)

		putJSON(t, handler, http.MethodPost, "/api/branches/"+experimentID+"/ramifications", map[string]any{
			"goal":                     "Should fail before provider.",
			"profile_id":               "local_test",
			"model":                    "test-model",
			"expected_main_head":       comparison["main_head"],
			"expected_experiment_head": comparison["experiment_head"],
			"comparison_fingerprint":   "sha256:" + strings.Repeat("f", 64),
		}, http.StatusConflict, nil)
		if atomic.LoadInt64(providerCalls) != beforeCalls {
			t.Fatal("stale ramification fingerprint reached provider")
		}

		putJSON(t, handler, http.MethodPost, "/api/branches/"+experimentID+"/promote", map[string]any{
			"paths":                    []string{scenePath},
			"expected_main_head":       strings.Repeat("a", 40),
			"expected_experiment_head": comparison["experiment_head"],
			"comparison_fingerprint":   comparison["fingerprint"],
		}, http.StatusConflict, nil)

		putJSON(t, handler, http.MethodPost, "/api/branches/"+experimentID+"/promote", map[string]any{
			"paths":                    []string{scenePath},
			"expected_main_head":       comparison["main_head"],
			"expected_experiment_head": comparison["experiment_head"],
			"comparison_fingerprint":   "sha256:" + strings.Repeat("e", 64),
		}, http.StatusConflict, nil)

		if gitActiveBranch(t, projectPath) != beforeBranch {
			t.Fatal("stale promotion changed active branch")
		}
		if gitRevParse(t, projectPath, "main") != beforeMain || gitRevCount(t, projectPath) != beforeCommits {
			t.Fatal("stale promotion mutated main")
		}
		assertClean(t, projectPath)
	})

	t.Run("changed_on_main_path_conflicts_before_promotion_checkout", func(t *testing.T) {
		putJSON(t, handler, http.MethodPost, "/api/branches/switch", map[string]string{"target": "main"}, http.StatusOK, nil)
		mainScene := loadScene(t, handler, sceneID)
		saveSceneMarkdown(t, handler, sceneID, mainScene, "Main-only rewrite changes canon after the fork while the council reconvenes and the room feels different now.\n")

		putJSON(t, handler, http.MethodPost, "/api/branches/switch", map[string]any{
			"target":        experimentID,
			"expected_head": comparison["experiment_head"],
		}, http.StatusOK, nil)

		var refreshed map[string]any
		getJSON(t, handler, "/api/branches/"+experimentID+"/comparison", http.StatusOK, &refreshed)
		beforeBranch := gitActiveBranch(t, projectPath)
		beforeMain := gitRevParse(t, projectPath, "main")
		beforeCommits := gitRevCount(t, projectPath)

		putJSON(t, handler, http.MethodPost, "/api/branches/"+experimentID+"/promote", map[string]any{
			"paths":                    []string{scenePath},
			"expected_main_head":       refreshed["main_head"],
			"expected_experiment_head": refreshed["experiment_head"],
			"comparison_fingerprint":   refreshed["fingerprint"],
		}, http.StatusConflict, nil)

		if gitActiveBranch(t, projectPath) != beforeBranch {
			t.Fatal("conflict promotion switched branches")
		}
		if gitRevParse(t, projectPath, "main") != beforeMain || gitRevCount(t, projectPath) != beforeCommits {
			t.Fatal("conflict promotion mutated main")
		}
		assertClean(t, projectPath)
	})

	t.Run("invalid_promotion_subset_rolls_back_main", func(t *testing.T) {
		invalidOutline := "version: 2\nroot:\n  arcs: []\n"
		if err := os.WriteFile(filepath.Join(projectPath, "outline.yaml"), []byte(invalidOutline), 0o644); err != nil {
			t.Fatal(err)
		}
		gitCommitAll(t, projectPath, "Invalidate outline on experiment")

		var refreshed map[string]any
		getJSON(t, handler, "/api/branches/"+experimentID+"/comparison", http.StatusOK, &refreshed)
		mainHeadBefore := gitRevParse(t, projectPath, "main")
		mainTreeBefore := gitRevParse(t, projectPath, "main^{tree}")
		mainSceneBefore, err := exec.Command("git", "-C", projectPath, "show", "main:"+scenePath).Output()
		if err != nil {
			t.Fatal(err)
		}

		putJSON(t, handler, http.MethodPost, "/api/branches/"+experimentID+"/promote", map[string]any{
			"paths":                    []string{"outline.yaml"},
			"expected_main_head":       refreshed["main_head"],
			"expected_experiment_head": refreshed["experiment_head"],
			"comparison_fingerprint":   refreshed["fingerprint"],
		}, http.StatusConflict, nil)

		if gitRevParse(t, projectPath, "main") != mainHeadBefore {
			t.Fatal("invalid promotion advanced main")
		}
		if gitRevParse(t, projectPath, "main^{tree}") != mainTreeBefore {
			t.Fatal("invalid promotion changed main tree")
		}
		mainSceneAfter, err := exec.Command("git", "-C", projectPath, "show", "main:"+scenePath).Output()
		if err != nil {
			t.Fatal(err)
		}
		if string(mainSceneAfter) != string(mainSceneBefore) {
			t.Fatal("invalid promotion left partial main scene bytes")
		}
		assertClean(t, projectPath)
	})
}

func setupM8RamificationProvider(t *testing.T, scenePath string) *int64 {
	t.Helper()
	var calls int64
	providerOutput := strings.Replace(`{"summary":"Experiment changes the opening scene.","findings":[{"category":"continuity","severity":"medium","title":"Tone shift in opening","explanation":"The revised scene changes the established mood.","affected_paths":["__SCENE_PATH__"],"recommended_action":"Review neighboring scenes before promotion."}]}`, "__SCENE_PATH__", scenePath, 1)
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		atomic.AddInt64(&calls, 1)
		if request.URL.Path != "/api/chat" || request.Method != http.MethodPost {
			t.Errorf("provider request = %s %s", request.Method, request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{"message": map[string]string{"content": providerOutput}})
	}))
	t.Cleanup(provider.Close)

	handler := app.NewHandler("test")
	putJSON(t, handler, http.MethodPut, "/api/provider-profiles", map[string]any{
		"profiles": []map[string]any{{
			"id": "local_test", "name": "Local Test", "type": "ollama", "base_url": provider.URL,
			"auth":         map[string]any{"type": "none", "credential_env": ""},
			"capabilities": map[string]any{"chat": true, "streaming": false, "structured_output": false, "max_context_tokens": 8192},
		}},
		"expected_revision": nil,
	}, http.StatusOK, nil)
	return &calls
}

func recordMainRefAndTree(t *testing.T, projectPath string) (string, string) {
	t.Helper()
	return gitRevParse(t, projectPath, "main"), gitRevParse(t, projectPath, "main^{tree}")
}

func assertMainRefAndTreeUnchanged(t *testing.T, projectPath, wantMainHead, wantMainTree string) {
	t.Helper()
	if got := gitRevParse(t, projectPath, "main"); got != wantMainHead {
		t.Fatalf("main HEAD = %q, want %q", got, wantMainHead)
	}
	if got := gitRevParse(t, projectPath, "main^{tree}"); got != wantMainTree {
		t.Fatalf("main tree = %q, want %q", got, wantMainTree)
	}
}

func gitRevParse(t *testing.T, projectPath, ref string) string {
	t.Helper()
	output, err := exec.Command("git", "-C", projectPath, "rev-parse", ref).Output()
	if err != nil {
		t.Fatalf("git rev-parse %s: %v", ref, err)
	}
	return strings.TrimSpace(string(output))
}

func gitActiveBranch(t *testing.T, projectPath string) string {
	t.Helper()
	output, err := exec.Command("git", "-C", projectPath, "symbolic-ref", "--short", "HEAD").Output()
	if err != nil {
		t.Fatalf("git symbolic-ref HEAD: %v", err)
	}
	return strings.TrimSpace(string(output))
}

func gitRevCountRef(t *testing.T, projectPath, ref string) int {
	t.Helper()
	output, err := exec.Command("git", "-C", projectPath, "rev-list", "--count", ref).Output()
	if err != nil {
		t.Fatalf("git rev-list --count %s: %v", ref, err)
	}
	count, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		t.Fatal(err)
	}
	return count
}

func gitCommitAll(t *testing.T, projectPath, message string) {
	t.Helper()
	if output, err := exec.Command("git", "-C", projectPath, "add", "-A").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, output)
	}
	if output, err := exec.Command("git", "-C", projectPath, "commit", "-m", message).CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, output)
	}
}
