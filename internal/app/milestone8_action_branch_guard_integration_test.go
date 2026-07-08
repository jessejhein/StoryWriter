// BDD Scenario: 8.1.3 - Continue normal work in the experiment; 8.5.1 - Discard the active experiment
// Requirements: M8-R18, M8-R19, M8-R20
// Test purpose: Real app integration proves transient action runs and invitations
// cannot be executed after the active branch changes.

package app_test

import (
	"net/http"
	"path/filepath"
	"testing"

	"storywork/internal/app"
)

// Test: branch changes invalidate transient action runs and invitations before
// they can mutate a different active branch.
// Requirements: M8-R18, M8-R19.
func TestMilestone8BranchChangeInvalidatesTransientActionState(t *testing.T) {
	configPath := t.TempDir()
	t.Setenv("STORYWORK_CONFIG_DIR", configPath)

	handler := app.NewHandler("test")
	projectPath := filepath.Join(t.TempDir(), "m8-action-branch-guard")
	putJSON(t, handler, http.MethodPost, "/api/projects", map[string]any{"name": "M8 Action Guard", "path": projectPath}, http.StatusCreated, nil)

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

	initialScene := loadScene(t, handler, sceneID)
	longMarkdown := "Alpha beta gamma delta echo foxtrot golf hotel india juliet kilo lima mike november oscar papa quebec romeo sierra tango uniform victor whiskey xray yankee zulu.\n"
	saveSceneMarkdown(t, handler, sceneID, initialScene, longMarkdown)
	sceneBefore := loadScene(t, handler, sceneID)
	selected, endByte := selectionPrefix(sceneBefore["markdown"].(string), 20)

	var created map[string]any
	putJSON(t, handler, http.MethodPost, "/api/branches", map[string]string{"name": "Action Guard"}, http.StatusCreated, &created)
	experimentID := created["active_experiment_id"].(string)
	experimentHead := created["experiment_head"].(string)

	var experimentRun map[string]any
	putJSON(t, handler, http.MethodPost, "/api/actions/run", map[string]any{
		"agent_id": "line_polish", "style_id": "precise_editor", "surface": "editor", "input_scope": "selection",
		"scene_id": sceneID, "scene_revision": sceneBefore["revision"],
		"selection": map[string]any{"start_byte": 0, "end_byte": endByte, "text": selected},
	}, http.StatusCreated, &experimentRun)

	putJSON(t, handler, http.MethodPost, "/api/branches/switch", map[string]string{"target": "main"}, http.StatusOK, nil)
	putJSON(t, handler, http.MethodPost, "/api/actions/"+experimentRun["run_id"].(string)+"/accept", map[string]any{
		"expected_revision": sceneBefore["revision"],
	}, http.StatusConflict, nil)

	mainScene := loadScene(t, handler, sceneID)
	var mainAccepted map[string]any
	var mainRun map[string]any
	putJSON(t, handler, http.MethodPost, "/api/actions/run", map[string]any{
		"agent_id": "line_polish", "style_id": "precise_editor", "surface": "editor", "input_scope": "selection",
		"scene_id": sceneID, "scene_revision": mainScene["revision"],
		"selection": map[string]any{"start_byte": 0, "end_byte": endByte, "text": selected},
	}, http.StatusCreated, &mainRun)
	putJSON(t, handler, http.MethodPost, "/api/actions/"+mainRun["run_id"].(string)+"/accept", map[string]any{
		"expected_revision": mainScene["revision"],
	}, http.StatusOK, &mainAccepted)

	invitations := mainAccepted["follow_up_invitations"].([]any)
	if len(invitations) != 1 {
		t.Fatalf("follow_up_invitations = %#v", invitations)
	}
	invitationID := invitations[0].(map[string]any)["invitation_id"].(string)

	putJSON(t, handler, http.MethodPost, "/api/branches/switch", map[string]any{
		"target":        experimentID,
		"expected_head": experimentHead,
	}, http.StatusOK, nil)

	putJSON(t, handler, http.MethodPost, "/api/action-invitations/"+invitationID+"/run", map[string]any{
		"style_id": "precise_editor", "expected_target_revision": loadScene(t, handler, sceneID)["revision"],
	}, http.StatusConflict, nil)

	assertClean(t, projectPath)
}
