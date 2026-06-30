package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// BDD trace:
//   - Requirements: M4-R01, M4-R02, M4-R03.
//   - Scenario: 4.1.1, 4.1.2.
//   - Test purpose: verify strict YAML registry loading, deterministic ordering,
//     duplicate rejection, old-schema rejection, multiple-document rejection, and
//     symlink refusal without silent repair.
func TestLoaderStrictlyLoadsAndRejectsRegistryFiles(t *testing.T) {
	t.Parallel()

	projectPath := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectPath, "agents"), 0o755); err != nil {
		t.Fatalf("MkdirAll(agents) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(projectPath, "styles"), 0o755); err != nil {
		t.Fatalf("MkdirAll(styles) error = %v", err)
	}
	mustWriteFile(t, filepath.Join(projectPath, "agents", "b.yaml"), strings.Join([]string{
		"version: 1",
		"id: line_polish",
		"name: Line Polish",
		"description: Rewrite selected prose.",
		"applies_when:",
		"  surfaces: [editor]",
		"  input_scopes: [selection]",
		"  min_words: 20",
		"  max_words: 1500",
		"context_policy:",
		"  required: [selected_text, style_sheet]",
		"  optional: [surrounding_paragraphs]",
		"  forbidden: [global_codex_rag, raw_import_notes]",
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
	mustWriteFile(t, filepath.Join(projectPath, "agents", "a.yaml"), strings.Join([]string{
		"version: 1",
		"id: chapter_refiner",
		"name: Chapter Refiner",
		"description: Refine a chapter.",
		"applies_when:",
		"  surfaces: [chapter_view]",
		"  input_scopes: [chapter]",
		"  min_words: 1000",
		"  max_words: 12000",
		"context_policy:",
		"  required: [current_chapter, chapter_summary, style_sheet]",
		"  optional: [arc_summary]",
		"  forbidden: [raw_import_notes]",
		"rag_policy:",
		"  mode: none",
		"control:",
		"  output_mode: patch",
		"  requires_acceptance: true",
		"  can_modify_canon: false",
		"output:",
		"  type: revised_text",
		"  requires_diff_preview: true",
		"",
	}, "\n"))
	mustWriteFile(t, filepath.Join(projectPath, "styles", "style.yaml"), strings.Join([]string{
		"version: 1",
		"id: precise_editor",
		"name: Precise Editor",
		"provider_profile_id: mock_default",
		"model: mock",
		"parameters:",
		"  temperature: 0.2",
		"system_prompt: You are a careful prose editor.",
		"",
	}, "\n"))

	loader := NewLoader()
	registry, err := loader.Load(projectPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := registry.Agents[0].ID; got != "chapter_refiner" {
		t.Fatalf("agent ordering[0] = %q, want chapter_refiner", got)
	}
	if got := registry.Agents[1].ID; got != "line_polish" {
		t.Fatalf("agent ordering[1] = %q, want line_polish", got)
	}
	if len(registry.Styles) != 1 || registry.Styles[0].ID != "precise_editor" {
		t.Fatalf("styles = %#v", registry.Styles)
	}

	badProject := t.TempDir()
	if err := os.MkdirAll(filepath.Join(badProject, "agents"), 0o755); err != nil {
		t.Fatalf("MkdirAll(bad agents) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(badProject, "styles"), 0o755); err != nil {
		t.Fatalf("MkdirAll(bad styles) error = %v", err)
	}
	mustWriteFile(t, filepath.Join(badProject, "agents", "old.yaml"), "id: line_polish\nname: Line Polish\n")
	if _, err := loader.LoadAgents(badProject); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("LoadAgents(old schema) error = %v, want unsupported", err)
	}

	mustWriteFile(t, filepath.Join(badProject, "agents", "dup_1.yaml"), strings.Join([]string{
		"version: 1", "id: line_polish", "name: A", "description: A", "applies_when:", "  surfaces: [editor]", "  input_scopes: [selection]", "  min_words: 20", "  max_words: 1500",
		"context_policy:", "  required: [selected_text, style_sheet]", "  optional: [surrounding_paragraphs]", "  forbidden: [global_codex_rag, raw_import_notes]",
		"rag_policy:", "  mode: none", "control:", "  output_mode: patch", "  requires_acceptance: true", "  can_modify_canon: false",
		"output:", "  type: replacement_text", "  requires_diff_preview: true", "",
	}, "\n"))
	mustWriteFile(t, filepath.Join(badProject, "agents", "dup_2.yaml"), strings.Join([]string{
		"version: 1", "id: line_polish", "name: B", "description: B", "applies_when:", "  surfaces: [editor]", "  input_scopes: [selection]", "  min_words: 20", "  max_words: 1500",
		"context_policy:", "  required: [selected_text, style_sheet]", "  optional: [surrounding_paragraphs]", "  forbidden: [global_codex_rag, raw_import_notes]",
		"rag_policy:", "  mode: none", "control:", "  output_mode: patch", "  requires_acceptance: true", "  can_modify_canon: false",
		"output:", "  type: replacement_text", "  requires_diff_preview: true", "",
	}, "\n"))
	if _, err := loader.LoadAgents(badProject); err == nil || !strings.Contains(err.Error(), "duplicate agent id") {
		t.Fatalf("LoadAgents(duplicate) error = %v, want duplicate detail", err)
	}

	multiDocProject := t.TempDir()
	if err := os.MkdirAll(filepath.Join(multiDocProject, "agents"), 0o755); err != nil {
		t.Fatalf("MkdirAll(multi agents) error = %v", err)
	}
	mustWriteFile(t, filepath.Join(multiDocProject, "agents", "multi.yaml"), strings.Join([]string{
		"version: 1",
		"id: line_polish",
		"name: Line Polish",
		"description: Rewrite selected prose.",
		"applies_when:",
		"  surfaces: [editor]",
		"  input_scopes: [selection]",
		"  min_words: 20",
		"  max_words: 1500",
		"context_policy:",
		"  required: [selected_text, style_sheet]",
		"  optional: [surrounding_paragraphs]",
		"  forbidden: [global_codex_rag, raw_import_notes]",
		"rag_policy:",
		"  mode: none",
		"control:",
		"  output_mode: patch",
		"  requires_acceptance: true",
		"  can_modify_canon: false",
		"output:",
		"  type: replacement_text",
		"  requires_diff_preview: true",
		"---",
		"version: 1",
		"id: extra",
		"name: Extra",
		"description: Extra",
		"",
	}, "\n"))
	if _, err := loader.LoadAgents(multiDocProject); err == nil || !strings.Contains(err.Error(), "multiple YAML documents") {
		t.Fatalf("LoadAgents(multi doc) error = %v, want multiple-doc failure", err)
	}

	symlinkProject := t.TempDir()
	if err := os.MkdirAll(filepath.Join(symlinkProject, "styles"), 0o755); err != nil {
		t.Fatalf("MkdirAll(symlink styles) error = %v", err)
	}
	targetPath := filepath.Join(symlinkProject, "target.yaml")
	mustWriteFile(t, targetPath, "version: 1\nid: precise_editor\nname: Precise Editor\nprovider_profile_id: mock_default\nmodel: mock\nparameters:\n  temperature: 0.2\nsystem_prompt: prompt\n")
	if err := os.Symlink(targetPath, filepath.Join(symlinkProject, "styles", "linked.yaml")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}
	if _, err := loader.LoadStyles(symlinkProject); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("LoadStyles(symlink) error = %v, want symlink failure", err)
	}
}

// BDD trace:
//   - Requirements: M4-R02, M4-R03.
//   - Scenario: 4.1.2.
//   - Test purpose: verify required style parameters cannot silently receive Go
//     zero values when their YAML keys are absent.
func TestLoaderRejectsStyleWithoutRequiredTemperature(t *testing.T) {
	t.Parallel()

	projectPath := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectPath, "styles"), 0o755); err != nil {
		t.Fatalf("MkdirAll(styles) error = %v", err)
	}
	mustWriteFile(t, filepath.Join(projectPath, "styles", "style.yaml"), strings.Join([]string{
		"version: 1",
		"id: precise_editor",
		"name: Precise Editor",
		"provider_profile_id: mock_default",
		"model: mock",
		"parameters: {}",
		"system_prompt: You are a careful prose editor.",
		"",
	}, "\n"))

	if _, err := NewLoader().LoadStyles(projectPath); err == nil || !strings.Contains(err.Error(), "temperature is required") {
		t.Fatalf("LoadStyles(missing temperature) error = %v, want required-field failure", err)
	}
}

func mustWriteFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}
