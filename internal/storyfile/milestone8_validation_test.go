// BDD Scenario: 8.4.3 - Reject an invalid selected subset
// Requirements: M8-R14, M8-R15
// Test purpose: Storyfile validation rejects orphan arc, chapter, and scene
// files instead of silently ignoring them.

package storyfile_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"storywork/internal/storyfile"
)

func writeStoryFixture(t *testing.T, root string, files map[string]string) {
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

// Test: orphan canonical story files fail validation.
// Requirements: M8-R14, M8-R15.
func TestValidateCanonicalFilesRejectsOrphanStoryFiles(t *testing.T) {
	t.Parallel()
	store := storyfile.New()
	tests := []struct {
		name     string
		files    map[string]string
		wantPath string
	}{
		{
			name: "orphan arc",
			files: map[string]string{
				"outline.yaml":                       "version: 1\nroot:\n  arcs: []\n",
				"arcs/arc_0123456789abcdef0123.yaml": "version: 1\nid: arc_0123456789abcdef0123\ntitle: Act\n",
			},
			wantPath: "arcs/arc_0123456789abcdef0123.yaml",
		},
		{
			name: "orphan chapter",
			files: map[string]string{
				"outline.yaml":                          "version: 1\nroot:\n  arcs: []\n",
				"chapters/ch_0123456789abcdef0123.yaml": "version: 1\nid: ch_0123456789abcdef0123\narc_id: arc_0123456789abcdef0123\ntitle: Chapter\n",
			},
			wantPath: "chapters/ch_0123456789abcdef0123.yaml",
		},
		{
			name: "orphan scene",
			files: map[string]string{
				"outline.yaml":                       "version: 1\nroot:\n  arcs: []\n",
				"scenes/scn_0123456789abcdef0123.md": "---\nid: scn_0123456789abcdef0123\ntitle: Scene\nchapter_id: ch_0123456789abcdef0123\npov: \"\"\nstatus: draft\nexclude_from_ai: false\n---\n\n",
			},
			wantPath: "scenes/scn_0123456789abcdef0123.md",
		},
	}
	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeStoryFixture(t, root, testCase.files)
			outline, err := store.Load(context.Background(), root)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			err = store.ValidateCanonicalFiles(context.Background(), root, outline)
			if err == nil {
				t.Fatal("ValidateCanonicalFiles() error = nil, want failure")
			}
			if want := testCase.wantPath; want != "" && !strings.Contains(err.Error(), want) {
				t.Fatalf("ValidateCanonicalFiles() error = %v, want detail for %q", err, want)
			}
			if !errors.Is(err, storyfile.ErrInvalidCanonicalState) {
				t.Fatalf("ValidateCanonicalFiles() error = %v, want errors.Is invalid canonical state", err)
			}
		})
	}
}

// Test: .gitkeep placeholders in canonical directories are ignored by the
// validator.
// Requirements: M8-R14, M8-R15.
func TestValidateCanonicalFilesIgnoresGitkeepPlaceholders(t *testing.T) {
	t.Parallel()
	store := storyfile.New()
	root := t.TempDir()
	writeStoryFixture(t, root, map[string]string{
		"outline.yaml":                          "version: 1\nroot:\n  arcs:\n    - id: arc_00000000000000000001\n      chapters:\n        - id: ch_00000000000000000001\n          scenes:\n            - id: scn_00000000000000000001\n",
		"arcs/.gitkeep":                         "",
		"chapters/.gitkeep":                     "",
		"scenes/.gitkeep":                       "",
		"arcs/arc_00000000000000000001.yaml":    "version: 1\nid: arc_00000000000000000001\ntitle: Act\n",
		"chapters/ch_00000000000000000001.yaml": "version: 1\nid: ch_00000000000000000001\narc_id: arc_00000000000000000001\ntitle: Chapter\n",
		"scenes/scn_00000000000000000001.md":    "---\nid: scn_00000000000000000001\ntitle: Scene\nchapter_id: ch_00000000000000000001\npov: \"\"\nstatus: draft\nexclude_from_ai: false\n---\n\n",
	})
	outline, err := store.Load(context.Background(), root)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if err := store.ValidateCanonicalFiles(context.Background(), root, outline); err != nil {
		t.Fatalf("ValidateCanonicalFiles() error = %v", err)
	}
}
