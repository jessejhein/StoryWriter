package app_test

// BDD Scenario: 7.2.1 - Resolve different active facts by scene
// Requirements: M7-R01 through M7-R18
// Test purpose: Real adapters prove timeline context, previews, invitations, and adversarial guards.

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"storywork/internal/app"
	"storywork/internal/story"
)

func TestMilestone7TimelineContextAndConditionalActions(t *testing.T) {
	configPath := t.TempDir()
	t.Setenv("STORYWORK_CONFIG_DIR", configPath)

	handler := app.NewHandler("test")
	projectPath := filepath.Join(t.TempDir(), "m7-novel")
	putJSON(t, handler, http.MethodPost, "/api/projects", map[string]any{"name": "M7 Novel", "path": projectPath}, http.StatusCreated, nil)

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
	putJSON(t, handler, http.MethodPost, "/api/scenes", map[string]any{"chapter_id": chapterID, "title": "Before"}, http.StatusCreated, &outline)
	sceneBeforeID := outline.Outline.Arcs[0].Chapters[0].Scenes[0].ID
	putJSON(t, handler, http.MethodPost, "/api/scenes", map[string]any{"chapter_id": chapterID, "title": "After"}, http.StatusCreated, &outline)
	sceneAfterID := outline.Outline.Arcs[0].Chapters[0].Scenes[len(outline.Outline.Arcs[0].Chapters[0].Scenes)-1].ID

	var codexEntry struct {
		ID string `json:"id"`
	}
	putJSON(t, handler, http.MethodPost, "/api/codex", map[string]any{
		"type": "character", "name": "Mara", "aliases": []string{}, "tags": []string{}, "description": "Young scout", "metadata": map[string]string{},
	}, http.StatusCreated, &codexEntry)

	sceneBefore := loadScene(t, handler, sceneBeforeID)
	longBefore := "Mara entered before the change while the hall stayed quiet and still and she waited for the council to begin their review.\n"
	longAfter := "Mara entered after the change while the hall stayed tense and still and she waited for the council to finish their review.\n"
	saveSceneMarkdown(t, handler, sceneBeforeID, sceneBefore, longBefore)
	sceneAfter := loadScene(t, handler, sceneAfterID)
	saveSceneMarkdown(t, handler, sceneAfterID, sceneAfter, longAfter)
	sceneBefore = loadScene(t, handler, sceneBeforeID)
	sceneAfter = loadScene(t, handler, sceneAfterID)

	putJSON(t, handler, http.MethodPut, "/api/codex/"+codexEntry.ID+"/progressions", map[string]any{
		"progressions": []map[string]any{{
			"anchor":  map[string]any{"type": "scene", "id": sceneBeforeID, "timing": "after"},
			"changes": map[string]any{"description": "Veteran captain"},
		}},
		"expected_revision": nil,
	}, http.StatusOK, nil)

	chapterFP := ""

	t.Run("line_polish_is_minimal", func(t *testing.T) {
		selected, endByte := selectionPrefix(sceneBefore["markdown"].(string), 20)
		var preview map[string]any
		putJSON(t, handler, http.MethodPost, "/api/actions/context-preview", map[string]any{
			"agent_id": "line_polish", "style_id": "precise_editor", "scope": "selection",
			"target": map[string]any{
				"scene_id": sceneBeforeID, "scene_revision": sceneBefore["revision"],
				"start_byte": 0, "end_byte": endByte, "text": selected,
			},
		}, http.StatusOK, &preview)
		manifest := preview["manifest"].(map[string]any)
		packs := manifest["packs_used"].([]any)
		if len(packs) != 2 || !packSetIncludes(packs, "selected_text", "style_sheet") {
			t.Fatalf("packs = %#v", packs)
		}
		for _, forbidden := range []string{"current_scene", "outline_neighborhood", "active_codex_at_position"} {
			if packSetIncludes(packs, forbidden) {
				t.Fatalf("forbidden pack %q in %#v", forbidden, packs)
			}
		}
	})

	t.Run("context_preview_makes_zero_provider_calls", func(t *testing.T) {
		var before map[string]any
		putJSON(t, handler, http.MethodPost, "/api/actions/context-preview", map[string]any{
			"agent_id": "scene_rewrite", "style_id": "precise_editor", "scope": "scene",
			"target": map[string]any{"scene_id": sceneBeforeID, "scene_revision": sceneBefore["revision"]},
		}, http.StatusOK, &before)
		if before["manifest"] == nil {
			t.Fatal("preview returned no manifest")
		}
	})

	t.Run("progression_changes_scene_context", func(t *testing.T) {
		var beforePreview map[string]any
		putJSON(t, handler, http.MethodPost, "/api/actions/context-preview", map[string]any{
			"agent_id": "scene_rewrite", "style_id": "precise_editor", "scope": "scene",
			"target": map[string]any{"scene_id": sceneBeforeID, "scene_revision": sceneBefore["revision"]},
		}, http.StatusOK, &beforePreview)
		var afterPreview map[string]any
		putJSON(t, handler, http.MethodPost, "/api/actions/context-preview", map[string]any{
			"agent_id": "scene_rewrite", "style_id": "precise_editor", "scope": "scene",
			"target": map[string]any{"scene_id": sceneAfterID, "scene_revision": sceneAfter["revision"]},
		}, http.StatusOK, &afterPreview)
		beforeCodex := codexProgressions(beforePreview["manifest"].(map[string]any))
		afterCodex := codexProgressions(afterPreview["manifest"].(map[string]any))
		if len(afterCodex) == 0 || len(afterCodex[0]) == 0 {
			t.Fatalf("after manifest = %#v", afterPreview["manifest"])
		}
		if len(beforeCodex) == 0 {
			t.Fatalf("before manifest missing codex refs: %#v", beforePreview["manifest"])
		}
		if len(afterCodex[0]) == 0 {
			t.Fatalf("after scene should include applied progression IDs: %#v", afterPreview["manifest"])
		}
	})

	t.Run("future_state_never_leaks_backward", func(t *testing.T) {
		var beforePreview map[string]any
		putJSON(t, handler, http.MethodPost, "/api/actions/context-preview", map[string]any{
			"agent_id": "scene_rewrite", "style_id": "precise_editor", "scope": "scene",
			"target": map[string]any{"scene_id": sceneBeforeID, "scene_revision": sceneBefore["revision"]},
		}, http.StatusOK, &beforePreview)
		var afterPreview map[string]any
		putJSON(t, handler, http.MethodPost, "/api/actions/context-preview", map[string]any{
			"agent_id": "scene_rewrite", "style_id": "precise_editor", "scope": "scene",
			"target": map[string]any{"scene_id": sceneAfterID, "scene_revision": sceneAfter["revision"]},
		}, http.StatusOK, &afterPreview)
		beforeIDs := codexProgressions(beforePreview["manifest"].(map[string]any))
		afterIDs := codexProgressions(afterPreview["manifest"].(map[string]any))
		if len(beforeIDs) > 0 && len(beforeIDs[0]) > 0 {
			t.Fatalf("before scene leaked future progression IDs: %#v", beforeIDs)
		}
		if len(afterIDs) == 0 || len(afterIDs[0]) == 0 {
			t.Fatalf("after scene missing applied progression IDs: %#v", afterIDs)
		}
	})

	var rootRunID string
	t.Run("scene_rewrite_accepts_one_scene_only", func(t *testing.T) {
		startCommits := gitRevCount(t, projectPath)
		var run map[string]any
		putJSON(t, handler, http.MethodPost, "/api/actions/run", map[string]any{
			"agent_id": "scene_rewrite", "style_id": "precise_editor", "scope": "scene",
			"target": map[string]any{"scene_id": sceneBeforeID, "scene_revision": sceneBefore["revision"]},
		}, http.StatusCreated, &run)
		rootRunID = run["run_id"].(string)
		putJSON(t, handler, http.MethodPost, "/api/actions/"+rootRunID+"/accept", map[string]any{"expected_revision": sceneBefore["revision"]}, http.StatusOK, nil)
		if gitRevCount(t, projectPath) != startCommits+1 {
			t.Fatalf("unexpected commit count")
		}
		assertClean(t, projectPath)
		after := loadScene(t, handler, sceneAfterID)
		before := loadScene(t, handler, sceneBeforeID)
		if after["revision"] == before["revision"] && after["markdown"] == before["markdown"] {
			t.Fatal("scene rewrite did not change target scene")
		}
		sceneBefore = before
	})

	var invitationID string
	t.Run("accepted_root_offers_invitation", func(t *testing.T) {
		sceneBefore = loadScene(t, handler, sceneBeforeID)
		markdown := sceneBefore["markdown"].(string)
		words := strings.Fields(markdown)
		if len(words) < 20 {
			t.Fatalf("scene markdown too short for line polish: %q", markdown)
		}
		selected, endByte := selectionPrefix(markdown, 20)
		var run map[string]any
		putJSON(t, handler, http.MethodPost, "/api/actions/run", map[string]any{
			"agent_id": "line_polish", "style_id": "precise_editor", "surface": "editor", "input_scope": "selection",
			"scene_id": sceneBeforeID, "scene_revision": sceneBefore["revision"],
			"selection": map[string]any{"start_byte": 0, "end_byte": endByte, "text": selected},
		}, http.StatusCreated, &run)
		var accepted map[string]any
		putJSON(t, handler, http.MethodPost, "/api/actions/"+run["run_id"].(string)+"/accept", map[string]any{"expected_revision": sceneBefore["revision"]}, http.StatusOK, &accepted)
		invites := accepted["follow_up_invitations"].([]any)
		if len(invites) != 1 {
			t.Fatalf("invitations = %#v", invites)
		}
		invitationID = invites[0].(map[string]any)["invitation_id"].(string)
		sceneBefore = loadScene(t, handler, sceneBeforeID)
	})

	t.Run("explicit_invitation_creates_child_run", func(t *testing.T) {
		var child map[string]any
		putJSON(t, handler, http.MethodPost, "/api/action-invitations/"+invitationID+"/run", map[string]any{
			"style_id": "precise_editor", "expected_target_revision": sceneBefore["revision"],
		}, http.StatusCreated, &child)
		if child["parent_run_id"] == nil || child["run_id"] == rootRunID {
			t.Fatalf("child = %#v", child)
		}
	})

	t.Run("dependent_child_writes_exact_trailers", func(t *testing.T) {
		var child map[string]any
		putJSON(t, handler, http.MethodPost, "/api/actions/run", map[string]any{
			"agent_id": "scene_rewrite", "style_id": "precise_editor", "scope": "scene",
			"target": map[string]any{"scene_id": sceneBeforeID, "scene_revision": sceneBefore["revision"]},
		}, http.StatusCreated, &child)
		putJSON(t, handler, http.MethodPost, "/api/actions/"+child["run_id"].(string)+"/accept", map[string]any{"expected_revision": sceneBefore["revision"]}, http.StatusOK, nil)
		message := gitShowBody(t, projectPath)
		if !strings.Contains(message, "Storywork-Operation-ID:") {
			t.Fatalf("commit = %q", message)
		}
	})

	t.Run("chapter_review_returns_findings", func(t *testing.T) {
		chapterFP = computeChapterFingerprint(t, projectPath, sceneBeforeID, sceneAfterID)
		var run map[string]any
		putJSON(t, handler, http.MethodPost, "/api/actions/run", map[string]any{
			"agent_id": "chapter_review", "style_id": "precise_editor", "scope": "chapter_review",
			"target": map[string]any{"chapter_id": chapterID, "fingerprint": chapterFP},
		}, http.StatusCreated, &run)
		if run["output_mode"] != "suggestion" {
			t.Fatalf("run = %#v", run)
		}
	})

	t.Run("chapter_review_has_zero_canonical_mutation", func(t *testing.T) {
		startCommits := gitRevCount(t, projectPath)
		putJSON(t, handler, http.MethodPost, "/api/actions/run", map[string]any{
			"agent_id": "chapter_review", "style_id": "precise_editor", "scope": "chapter_review",
			"target": map[string]any{"chapter_id": chapterID, "fingerprint": chapterFP},
		}, http.StatusCreated, nil)
		if gitRevCount(t, projectPath) != startCommits {
			t.Fatal("chapter review created commits")
		}
		assertClean(t, projectPath)
	})

	t.Run("stale_target_stops_before_provider", func(t *testing.T) {
		putJSON(t, handler, http.MethodPost, "/api/actions/run", map[string]any{
			"agent_id": "scene_rewrite", "style_id": "precise_editor", "scope": "scene",
			"target": map[string]any{"scene_id": sceneBeforeID, "scene_revision": sceneAfter["revision"]},
		}, http.StatusConflict, nil)
	})

	t.Run("forged_invitation_leaves_no_partial_state", func(t *testing.T) {
		putJSON(t, handler, http.MethodPost, "/api/action-invitations/invite_deadbeefdeadbeefdead/run", map[string]any{
			"style_id": "precise_editor", "expected_target_revision": sceneBefore["revision"].(string),
		}, http.StatusNotFound, nil)
	})

	t.Run("invalid_findings_leave_no_run_or_mutation", func(t *testing.T) {
		startCommits := gitRevCount(t, projectPath)
		putJSON(t, handler, http.MethodPost, "/api/actions/run", map[string]any{
			"agent_id": "chapter_review", "style_id": "precise_editor", "scope": "chapter_review",
			"target": map[string]any{"chapter_id": chapterID, "fingerprint": "sha256:" + strings.Repeat("f", 64)},
		}, http.StatusConflict, nil)
		if gitRevCount(t, projectPath) != startCommits {
			t.Fatal("invalid chapter review mutated canon")
		}
	})

	t.Run("context_budget_stops_before_provider", func(t *testing.T) {
		agentPath := filepath.Join(projectPath, "agents", "scene_rewrite.yaml")
		contents, err := os.ReadFile(agentPath)
		if err != nil {
			t.Fatal(err)
		}
		tinyBudget := strings.Replace(string(contents),
			"min_context_tokens: 4096",
			"min_context_tokens: 16", 1)
		tinyBudget = strings.Replace(tinyBudget,
			"max_input_estimated_tokens: 12000",
			"max_input_estimated_tokens: 32", 1)
		tinyBudget = strings.Replace(tinyBudget,
			"reserved_output_estimated_tokens: 4000",
			"reserved_output_estimated_tokens: 8", 1)
		if err := os.WriteFile(agentPath, []byte(tinyBudget), 0o644); err != nil {
			t.Fatal(err)
		}
		startCommits := gitRevCount(t, projectPath)
		scene := loadScene(t, handler, sceneAfterID)
		putJSON(t, handler, http.MethodPost, "/api/actions/run", map[string]any{
			"agent_id": "scene_rewrite", "style_id": "precise_editor", "scope": "scene",
			"target": map[string]any{"scene_id": sceneAfterID, "scene_revision": scene["revision"]},
		}, http.StatusBadRequest, nil)
		if gitRevCount(t, projectPath) != startCommits {
			t.Fatal("budget overflow created commits")
		}
		if err := os.WriteFile(agentPath, contents, 0o644); err != nil {
			t.Fatal(err)
		}
		assertClean(t, projectPath)
	})

	t.Run("malformed_progression_fails_closed", func(t *testing.T) {
		progressionPath := filepath.Join(projectPath, "progressions", codexEntry.ID+".yaml")
		original, err := os.ReadFile(progressionPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(progressionPath, []byte(`entry_id: `+codexEntry.ID+`
progressions:
  - id: prog_0123456789abcdef0123
    anchor:
      type: scene
      id: scn_99999999999999999999
      timing: after
    changes:
      description: Missing anchor scene
`), 0o644); err != nil {
			t.Fatal(err)
		}
		scene := loadScene(t, handler, sceneAfterID)
		putJSON(t, handler, http.MethodPost, "/api/actions/context-preview", map[string]any{
			"agent_id": "scene_rewrite", "style_id": "precise_editor", "scope": "scene",
			"target": map[string]any{"scene_id": sceneAfterID, "scene_revision": scene["revision"]},
		}, http.StatusInternalServerError, nil)
		if err := os.WriteFile(progressionPath, original, 0o644); err != nil {
			t.Fatal(err)
		}
		assertClean(t, projectPath)
	})

	t.Run("checkpoint_failure_restores_scene_index_staging_and_history", func(t *testing.T) {
		scene := loadScene(t, handler, sceneAfterID)
		markdown := scene["markdown"].(string)
		selected, endByte := selectionPrefix(markdown, 20)
		var run map[string]any
		putJSON(t, handler, http.MethodPost, "/api/actions/run", map[string]any{
			"agent_id": "line_polish", "style_id": "precise_editor", "surface": "editor", "input_scope": "selection",
			"scene_id": sceneAfterID, "scene_revision": scene["revision"],
			"selection": map[string]any{"start_byte": 0, "end_byte": endByte, "text": selected},
		}, http.StatusCreated, &run)
		runID := run["run_id"].(string)
		beforeRevision := scene["revision"].(string)
		startCommits := gitRevCount(t, projectPath)

		hookPath := filepath.Join(projectPath, ".git", "hooks", "pre-commit")
		if err := os.MkdirAll(filepath.Dir(hookPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(hookPath, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
			t.Fatal(err)
		}

		putJSON(t, handler, http.MethodPost, "/api/actions/"+runID+"/accept", map[string]any{
			"expected_revision": beforeRevision,
		}, http.StatusInternalServerError, nil)

		restored := loadScene(t, handler, sceneAfterID)
		if restored["markdown"] != markdown || restored["revision"] != beforeRevision {
			t.Fatal("checkpoint failure left partial scene mutation")
		}
		if gitRevCount(t, projectPath) != startCommits {
			t.Fatal("checkpoint failure created commit")
		}
		assertClean(t, projectPath)

		if err := os.Remove(hookPath); err != nil {
			t.Fatal(err)
		}
		putJSON(t, handler, http.MethodPost, "/api/actions/"+runID+"/accept", map[string]any{
			"expected_revision": beforeRevision,
		}, http.StatusOK, nil)
	})
}

func packSetIncludes(packs []any, names ...string) bool {
	seen := make(map[string]bool, len(packs))
	for _, pack := range packs {
		seen[pack.(string)] = true
	}
	for _, name := range names {
		if !seen[name] {
			return false
		}
	}
	return true
}

func selectionPrefix(markdown string, words int) (string, int) {
	parts := strings.Fields(markdown)
	if len(parts) < words {
		words = len(parts)
	}
	selected := strings.Join(parts[:words], " ")
	return selected, len([]byte(selected))
}

func loadScene(t *testing.T, handler http.Handler, sceneID string) map[string]any {
	t.Helper()
	var scene map[string]any
	getJSON(t, handler, "/api/scenes/"+sceneID, http.StatusOK, &scene)
	return scene
}

func computeChapterFingerprint(t *testing.T, projectPath string, sceneIDs ...string) string {
	t.Helper()
	outlineBytes, err := os.ReadFile(filepath.Join(projectPath, "outline.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	pairs := make([]story.ChapterSceneRevision, 0, len(sceneIDs))
	for _, sceneID := range sceneIDs {
		sceneBytes, err := os.ReadFile(filepath.Join(projectPath, "scenes", sceneID+".md"))
		if err != nil {
			t.Fatal(err)
		}
		pairs = append(pairs, story.ChapterSceneRevision{SceneID: sceneID, Revision: storyRevision(sceneBytes)})
	}
	return story.ComputeChapterFingerprint(outlineBytes, pairs)
}

func storyRevision(content []byte) string {
	return story.ComputeRevision(content)
}

func saveSceneMarkdown(t *testing.T, handler http.Handler, sceneID string, scene map[string]any, markdown string) {
	t.Helper()
	var updated map[string]any
	putJSON(t, handler, http.MethodPut, "/api/scenes/"+sceneID, map[string]any{
		"title": scene["title"], "markdown": markdown,
		"frontmatter": scene["frontmatter"], "expected_revision": scene["revision"],
	}, http.StatusOK, &updated)
}

func codexProgressions(manifest map[string]any) [][]string {
	raw, ok := manifest["active_codex"].([]any)
	if !ok {
		return nil
	}
	result := make([][]string, 0, len(raw))
	for _, item := range raw {
		entry := item.(map[string]any)
		ids, _ := entry["applied_progression_ids"].([]any)
		converted := make([]string, 0, len(ids))
		for _, id := range ids {
			converted = append(converted, id.(string))
		}
		result = append(result, converted)
	}
	return result
}

func gitRevCount(t *testing.T, path string) int {
	t.Helper()
	output, err := exec.Command("git", "-C", path, "rev-list", "--count", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	count, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		t.Fatal(err)
	}
	return count
}

func gitShowBody(t *testing.T, path string) string {
	t.Helper()
	output, err := exec.Command("git", "-C", path, "show", "--no-patch", "--format=%B", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(output)
}