// BDD Scenario: 8.4.3 - Reject an invalid selected subset
// Requirements: M8-R14
// Test purpose: Full-project validation rejects malformed cross-file canonical state.

package projectcheck_test

import (
	"context"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"storywork/internal/agent"
	"storywork/internal/projectcheck"
)

func writeFixture(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, contents := range files {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// Test: one complete valid project snapshot passes.
// Requirements: M8-R14.
func TestValidateProjectAcceptsValidFixture(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "styles"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, map[string]string{
		"project.yaml":                          "version: 1\nid: proj_test\nname: Test\ncreated_at: \"2026-07-03T00:00:00Z\"\ndefault_branch: main\nsettings:\n  prose_format: markdown\n  vim_mode_default: true\n  ai_mutation_requires_acceptance: true\n",
		"outline.yaml":                          "version: 1\nroot:\n  arcs:\n    - id: arc_00000000000000000001\n      chapters:\n        - id: ch_00000000000000000001\n          scenes:\n            - id: scn_00000000000000000001\n",
		"arcs/arc_00000000000000000001.yaml":    "version: 1\nid: arc_00000000000000000001\ntitle: Act\n",
		"chapters/ch_00000000000000000001.yaml": "version: 1\nid: ch_00000000000000000001\narc_id: arc_00000000000000000001\ntitle: Chapter\n",
		"scenes/scn_00000000000000000001.md":    "---\nid: scn_00000000000000000001\ntitle: Scene\nchapter_id: ch_00000000000000000001\npov: \"\"\nstatus: draft\nexclude_from_ai: false\n---\n\n",
	})
	if err := projectcheck.New().ValidateProject(context.Background(), root); err != nil {
		t.Fatalf("ValidateProject() error = %v", err)
	}
}

// Test: malformed outline is rejected.
// Requirements: M8-R14.
func TestValidateProjectRejectsMalformedOutline(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFixture(t, root, map[string]string{
		"project.yaml": "version: 1\nid: proj_test\nname: Test\ncreated_at: \"2026-07-03T00:00:00Z\"\ndefault_branch: main\nsettings:\n  prose_format: markdown\n  vim_mode_default: true\n  ai_mutation_requires_acceptance: true\n",
		"outline.yaml": "version: 2\nroot:\n  arcs: []\n",
	})
	err := projectcheck.New().ValidateProject(context.Background(), root)
	if err == nil || !strings.Contains(err.Error(), "outline") {
		t.Fatalf("ValidateProject() error = %v", err)
	}
}

// Test: project metadata is strict and rejects duplicate or unknown fields.
// Requirements: M8-R14.
func TestValidateProjectRejectsNonStrictProjectMetadata(t *testing.T) {
	t.Parallel()
	for _, metadata := range []string{
		"version: 1\nid: proj_test\nid: duplicate\nname: Test\ncreated_at: \"2026-07-03T00:00:00Z\"\ndefault_branch: main\nsettings:\n  prose_format: markdown\n  vim_mode_default: true\n  ai_mutation_requires_acceptance: true\n",
		"version: 1\nid: proj_test\nname: Test\ncreated_at: \"2026-07-03T00:00:00Z\"\ndefault_branch: main\nunknown: true\nsettings:\n  prose_format: markdown\n  vim_mode_default: true\n  ai_mutation_requires_acceptance: true\n",
		"version: 1\nid: proj_test\nname: Test\ncreated_at: \"2026-07-03T00:00:00Z\"\ndefault_branch: main\nsettings:\n  prose_format: markdown\n  ai_mutation_requires_acceptance: true\n",
	} {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "agents"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(root, "styles"), 0o755); err != nil {
			t.Fatal(err)
		}
		writeFixture(t, root, map[string]string{"project.yaml": metadata, "outline.yaml": "version: 1\nroot:\n  arcs: []\n"})
		if err := projectcheck.New().ValidateProject(context.Background(), root); err == nil || !strings.Contains(err.Error(), "metadata") {
			t.Fatalf("ValidateProject() error = %v", err)
		}
	}
}

// Test: orphan progression documents and malformed tracked raw import
// manifests are rejected by the composed validator.
// Requirements: M8-R14.
func TestValidateProjectRejectsOrphanProgressionsAndMalformedRawImports(t *testing.T) {
	t.Parallel()
	base := map[string]string{
		"project.yaml": "version: 1\nid: proj_test\nname: Test\ncreated_at: \"2026-07-03T00:00:00Z\"\ndefault_branch: main\nsettings:\n  prose_format: markdown\n  vim_mode_default: true\n  ai_mutation_requires_acceptance: true\n",
		"outline.yaml": "version: 1\nroot:\n  arcs: []\n",
	}
	t.Run("orphan progression", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "agents"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(root, "styles"), 0o755); err != nil {
			t.Fatal(err)
		}
		files := maps.Clone(base)
		files["progressions/char_0123456789abcdef0123.yaml"] = "version: 1\nentry_id: char_0123456789abcdef0123\nprogressions: []\n"
		writeFixture(t, root, files)
		if err := projectcheck.New().ValidateProject(context.Background(), root); err == nil || !strings.Contains(err.Error(), "progression") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("malformed raw manifest", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "agents"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(root, "styles"), 0o755); err != nil {
			t.Fatal(err)
		}
		files := maps.Clone(base)
		files["imports/raw/imp_0123456789abcdef0123/manifest.yaml"] = "version: 1\nid: imp_0123456789abcdef0123\nunknown: true\n"
		writeFixture(t, root, files)
		if err := projectcheck.New().ValidateProject(context.Background(), root); err == nil || !strings.Contains(err.Error(), "import") {
			t.Fatalf("error = %v", err)
		}
	})
}

// Test: raw import snapshots and review candidates fail closed on unexpected
// tracked artifacts outside the owning schemas.
// Requirements: M8-R14, M8-R15.
func TestValidateProjectRejectsUnexpectedImportArtifacts(t *testing.T) {
	t.Parallel()
	base := map[string]string{
		"project.yaml": "version: 1\nid: proj_test\nname: Test\ncreated_at: \"2026-07-03T00:00:00Z\"\ndefault_branch: main\nsettings:\n  prose_format: markdown\n  vim_mode_default: true\n  ai_mutation_requires_acceptance: true\n",
		"outline.yaml": "version: 1\nroot:\n  arcs: []\n",
		"imports/raw/imp_0123456789abcdef0123/manifest.yaml":        "version: 1\nid: imp_0123456789abcdef0123\ncreated_at: \"2026-07-03T00:00:00Z\"\nfiles:\n  - path: notes/scene.md\n    bytes: 6\n    sha256: 5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03\n",
		"imports/raw/imp_0123456789abcdef0123/files/notes/scene.md": "hello\n",
	}
	t.Run("extra raw snapshot file", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "agents"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(root, "styles"), 0o755); err != nil {
			t.Fatal(err)
		}
		files := maps.Clone(base)
		files["imports/raw/imp_0123456789abcdef0123/files/notes/extra.md"] = "extra\n"
		writeFixture(t, root, files)
		if err := projectcheck.New().ValidateProject(context.Background(), root); err == nil || !strings.Contains(err.Error(), "raw import") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("non yaml review artifact", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "agents"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(root, "styles"), 0o755); err != nil {
			t.Fatal(err)
		}
		files := maps.Clone(base)
		files["imports/review/unexpected.txt"] = "not yaml\n"
		writeFixture(t, root, files)
		if err := projectcheck.New().ValidateProject(context.Background(), root); err == nil || !strings.Contains(err.Error(), "candidate") {
			t.Fatalf("error = %v", err)
		}
	})
}

// Test: malformed canonical outline state is classified as an invalid
// promotion subset so callers can return a 409 without leaking infrastructure
// details.
// Requirements: M8-R14, M8-R15.
func TestValidateProjectClassifiesInvalidCanonicalState(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "styles"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, map[string]string{
		"project.yaml": "version: 1\nid: proj_test\nname: Test\ncreated_at: \"2026-07-03T00:00:00Z\"\ndefault_branch: main\nsettings:\n  prose_format: markdown\n  vim_mode_default: true\n  ai_mutation_requires_acceptance: true\n",
		"outline.yaml": "version: 2\nroot:\n  arcs: []\n",
	})
	err := projectcheck.New().ValidateProject(context.Background(), root)
	if !errors.Is(err, projectcheck.ErrInvalidProject) {
		t.Fatalf("ValidateProject() error = %v", err)
	}
}

// Test: infrastructure failures remain distinct from invalid canonical state.
// Requirements: M8-R14, M8-R15.
func TestValidateProjectLeavesInfrastructureFailuresUnclassified(t *testing.T) {
	t.Parallel()
	infraErr := errors.New("filesystem offline")
	v := projectcheck.NewWithReaders(
		&fakeStoryReader{outlineErr: infraErr},
		&fakeRegistryReader{agents: []agent.Agent{}, styles: []agent.Style{}},
		projectcheck.WithMetadataFunc(noopMetadata),
		projectcheck.WithImportsFunc(noopImports),
	)
	err := v.ValidateProject(context.Background(), t.TempDir())
	if !errors.Is(err, infraErr) {
		t.Fatalf("ValidateProject() error = %v, want %v", err, infraErr)
	}
	if errors.Is(err, projectcheck.ErrInvalidProject) {
		t.Fatalf("ValidateProject() error = %v, want no invalid-project classification", err)
	}
}
