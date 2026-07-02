package agent

// BDD Scenario: 7.1.1 - Preview minimal Line Polish context
// Requirements: M7-R01, M7-R02
// Test purpose: Registry loading keeps old versions strict while dispatching version-3 agents and starter templates.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"storywork/internal/project"
)

func writeStyleDir(t *testing.T, projectPath string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(projectPath, "styles"), 0o755); err != nil {
		t.Fatalf("MkdirAll(styles) error = %v", err)
	}
	mustWriteFile(t, filepath.Join(projectPath, "styles", "precise_editor.yaml"), strings.Join([]string{
		"version: 2",
		"id: precise_editor",
		"name: Precise Editor",
		"provider_profile_id: mock_default",
		"model: mock",
		"parameters:",
		"  temperature: 0.2",
		"system_prompt: You are a careful prose editor.",
		"",
	}, "\n"))
}

// Test: strict version-3 YAML loads validated agents.
// Requirements: M7-R01, M7-R03, M7-R04.
func TestRegistryLoadsStrictVersion3Agents(t *testing.T) {
	t.Parallel()

	projectPath := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectPath, "agents"), 0o755); err != nil {
		t.Fatalf("MkdirAll(agents) error = %v", err)
	}
	writeStyleDir(t, projectPath)
	mustWriteFile(t, filepath.Join(projectPath, "agents", "scene_rewrite.yaml"), mustReadEmbeddedTemplate(t, "builtin_agent_scene_rewrite.yaml"))
	mustWriteFile(t, filepath.Join(projectPath, "agents", "chapter_review.yaml"), mustReadEmbeddedTemplate(t, "builtin_agent_chapter_review.yaml"))

	registry, err := NewLoader().Load(projectPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(registry.Agents) != 2 {
		t.Fatalf("agent count = %d, want 2", len(registry.Agents))
	}
	for _, item := range registry.Agents {
		if item.Version != 3 {
			t.Fatalf("agent %q version = %d, want 3", item.ID, item.Version)
		}
	}
}

// Test: version 1 and version 2 reject version-3-only fields.
// Requirements: M7-R19.
func TestRegistryRejectsVersion3FieldsInOlderVersions(t *testing.T) {
	t.Parallel()

	projectPath := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectPath, "agents"), 0o755); err != nil {
		t.Fatalf("MkdirAll(agents) error = %v", err)
	}
	mustWriteFile(t, filepath.Join(projectPath, "agents", "bad.yaml"), strings.Join([]string{
		"version: 2",
		"id: line_polish",
		"name: Line Polish",
		"description: Rewrite selected prose.",
		"applies_when:",
		"  surfaces: [editor]",
		"  input_scopes: [selection]",
		"  min_words: 20",
		"  max_words: 1500",
		"model_requirements:",
		"  min_context_tokens: 2048",
		"  supports_streaming: false",
		"  supports_structured_output: false",
		"context_policy:",
		"  required: [selected_text, style_sheet]",
		"  optional: [surrounding_paragraphs]",
		"  forbidden: [global_codex_rag, raw_import_notes]",
		"context_budget:",
		"  max_input_estimated_tokens: 4096",
		"  reserved_output_estimated_tokens: 1024",
		"rag_policy:",
		"  mode: none",
		"control:",
		"  output_mode: patch",
		"  requires_acceptance: true",
		"  can_modify_canon: false",
		"output:",
		"  type: replacement_text",
		"  requires_diff_preview: true",
		"",
	}, "\n"))
	if _, err := NewLoader().LoadAgents(projectPath); err == nil || !strings.Contains(err.Error(), "context_budget") {
		t.Fatalf("LoadAgents(v2 with budget) error = %v, want unknown field failure", err)
	}
}

// Test: version 3 rejects unknown, duplicate, and null fields.
// Requirements: M7-R07.
func TestRegistryRejectsUnknownDuplicateAndNullV3Fields(t *testing.T) {
	t.Parallel()

	projectPath := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectPath, "agents"), 0o755); err != nil {
		t.Fatalf("MkdirAll(agents) error = %v", err)
	}
	mustWriteFile(t, filepath.Join(projectPath, "agents", "unknown.yaml"), strings.Join([]string{
		"version: 3",
		"id: scene_rewrite",
		"name: Scene Rewrite",
		"description: Rewrite one scene.",
		"extra: true",
		"applies_when:",
		"  surfaces: [editor]",
		"  input_scopes: [scene]",
		"  min_words: 1",
		"  max_words: 12000",
		"model_requirements:",
		"  min_context_tokens: 4096",
		"  supports_streaming: false",
		"  supports_structured_output: false",
		"context_policy:",
		"  required: [current_scene, style_sheet, active_codex_at_position]",
		"  optional: [outline_neighborhood]",
		"  forbidden: [global_codex_rag, raw_import_notes, prior_chat]",
		"context_budget:",
		"  max_input_estimated_tokens: 12000",
		"  reserved_output_estimated_tokens: 4000",
		"rag_policy:",
		"  mode: timeline_aware",
		"control:",
		"  output_mode: patch",
		"  requires_acceptance: true",
		"  can_modify_canon: false",
		"output:",
		"  type: revised_text",
		"  requires_diff_preview: true",
		"",
	}, "\n"))
	if _, err := NewLoader().LoadAgents(projectPath); err == nil || !strings.Contains(err.Error(), "extra") {
		t.Fatalf("LoadAgents(unknown field) error = %v, want strict YAML failure", err)
	}
}

// Test: starter projects expose minimal Line Polish, Scene Rewrite, and Chapter Review.
// Requirements: M7-R02, M7-R03, M7-R04.
func TestStarterAgentsExposeMinimalLinePolishSceneRewriteAndReview(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	projectPath := filepath.Join(root, "milestone-seven")
	service := project.NewService(&projectGitStub{}, &projectIndexStub{}, fixedProjectNow)
	if _, err := service.Create(context.Background(), project.CreateRequest{
		Name: "Milestone Seven",
		Path: projectPath,
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	root = projectPath
	registry, err := NewLoader().Load(root)
	if err != nil {
		t.Fatalf("Load(starter) error = %v", err)
	}
	byID := map[string]Agent{}
	for _, item := range registry.Agents {
		byID[item.ID] = item
	}
	linePolish, ok := byID["line_polish"]
	if !ok || linePolish.Version != 3 {
		t.Fatalf("line_polish = %#v", linePolish)
	}
	if !containsPacks(linePolish.ContextPolicy.Required, ContextSelectedText, ContextStyleSheet) {
		t.Fatalf("line_polish required packs = %#v", linePolish.ContextPolicy.Required)
	}
	sceneRewrite, ok := byID["scene_rewrite"]
	if !ok || sceneRewrite.Version != 3 || sceneRewrite.RAGPolicy.Mode != RAGModeTimelineAware {
		t.Fatalf("scene_rewrite = %#v", sceneRewrite)
	}
	chapterReview, ok := byID["chapter_review"]
	if !ok || chapterReview.Version != 3 || chapterReview.Control.OutputMode != OutputModeSuggestion {
		t.Fatalf("chapter_review = %#v", chapterReview)
	}
	if _, ok := byID["chapter_refiner"]; ok {
		t.Fatal("starter project still exposes legacy chapter_refiner agent")
	}
}

type projectGitStub struct{}

func (p *projectGitStub) Init(context.Context, string) error { return nil }
func (p *projectGitStub) CommitAll(context.Context, string, string) error {
	return nil
}
func (p *projectGitStub) IsRepo(context.Context, string) (bool, error) { return true, nil }

type projectIndexStub struct{}

func (p *projectIndexStub) Init(context.Context, string) error    { return nil }
func (p *projectIndexStub) Rebuild(context.Context, string) error { return nil }
func (p *projectIndexStub) Verify(context.Context, string) error  { return nil }

var fixedProjectNow = func() time.Time { return timeFromRFC3339("2026-07-02T12:00:00Z") }

func timeFromRFC3339(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}
	return parsed
}

func containsPacks(values []ContextPack, packs ...ContextPack) bool {
	for _, pack := range packs {
		found := false
		for _, value := range values {
			if value == pack {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func mustReadEmbeddedTemplate(t *testing.T, name string) string {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join("..", "..", "templates", name))
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", name, err)
	}
	return string(contents)
}
