package storyfile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"storywork/internal/story"
)

// BDD trace:
//   - Requirement: Milestone 1, Story 1.1, view the outline.
//   - Scenario: given a valid project, when I request the outline, then the
//     canonical files load as ordered arcs, chapters, and scenes with stable IDs,
//     titles, and derived display labels; malformed canonical files are rejected.
//   - Test purpose: verify strict disk-to-model loading, including schema
//     validation for malformed YAML, unknown fields, missing files, duplicate IDs,
//     and parent mismatches.
func TestLoadRoundTripsAndRejectsInvalidCanonicalFiles(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	root := t.TempDir()
	store := New()
	mustMkdirAll(t, root, "arcs", "chapters", "scenes")

	empty := story.NewOutline()
	if _, err := store.WriteFiles(ctx, root, map[string][]byte{
		"outline.yaml": mustMarshalOutline(t, store, empty),
	}); err != nil {
		t.Fatalf("WriteFiles(empty outline) error = %v", err)
	}
	loaded, err := store.Load(ctx, root)
	if err != nil {
		t.Fatalf("Load(empty outline) error = %v", err)
	}
	if len(loaded.Arcs) != 0 {
		t.Fatalf("empty outline arcs = %d, want 0", len(loaded.Arcs))
	}

	populated := buildPopulatedOutline(t)
	if _, err := store.WriteFiles(ctx, root, map[string][]byte{
		"outline.yaml":                          mustMarshalOutline(t, store, populated),
		"arcs/arc_00000000000000000001.yaml":    mustMarshalArc(t, store, populated.Arcs[0]),
		"chapters/ch_00000000000000000001.yaml": mustMarshalChapter(t, store, populated.Arcs[0].Chapters[0]),
		"scenes/scn_00000000000000000001.md":    mustMarshalScene(t, store, populated.Arcs[0].Chapters[0].Scenes[0]),
	}); err != nil {
		t.Fatalf("WriteFiles(populated outline) error = %v", err)
	}
	loaded, err = store.Load(ctx, root)
	if err != nil {
		t.Fatalf("Load(populated outline) error = %v", err)
	}
	if got := loaded.Arcs[0].DisplayLabel; got != "Arc 1" {
		t.Fatalf("arc label = %q, want %q", got, "Arc 1")
	}
	if got := loaded.Arcs[0].Chapters[0].DisplayLabel; got != "Chapter 1.1" {
		t.Fatalf("chapter label = %q, want %q", got, "Chapter 1.1")
	}
	if got := loaded.Arcs[0].Chapters[0].Scenes[0].DisplayLabel; got != "Scene 1.1.1" {
		t.Fatalf("scene label = %q, want %q", got, "Scene 1.1.1")
	}

	cases := []struct {
		name      string
		files     map[string]string
		wantError string
	}{
		{
			name: "malformed outline yaml",
			files: map[string]string{
				"outline.yaml": "version: [\n",
			},
			wantError: "outline.yaml",
		},
		{
			name: "unknown arc field",
			files: map[string]string{
				"outline.yaml":                       "version: 1\nroot:\n  arcs:\n    - id: arc_00000000000000000001\n      chapters: []\n",
				"arcs/arc_00000000000000000001.yaml": "version: 1\nid: arc_00000000000000000001\ntitle: Act One\nextra: true\n",
			},
			wantError: "arcs/arc_00000000000000000001.yaml",
		},
		{
			name: "missing referenced chapter file",
			files: map[string]string{
				"outline.yaml":                       "version: 1\nroot:\n  arcs:\n    - id: arc_00000000000000000001\n      chapters:\n        - id: ch_00000000000000000001\n          scenes: []\n",
				"arcs/arc_00000000000000000001.yaml": "version: 1\nid: arc_00000000000000000001\ntitle: Act One\n",
			},
			wantError: "chapters/ch_00000000000000000001.yaml",
		},
		{
			name: "duplicate chapter id",
			files: map[string]string{
				"outline.yaml": strings.Join([]string{
					"version: 1",
					"root:",
					"  arcs:",
					"    - id: arc_00000000000000000001",
					"      chapters:",
					"        - id: ch_00000000000000000001",
					"          scenes: []",
					"        - id: ch_00000000000000000001",
					"          scenes: []",
					"",
				}, "\n"),
				"arcs/arc_00000000000000000001.yaml":    "version: 1\nid: arc_00000000000000000001\ntitle: Act One\n",
				"chapters/ch_00000000000000000001.yaml": "version: 1\nid: ch_00000000000000000001\narc_id: arc_00000000000000000001\ntitle: Arrival\n",
			},
			wantError: "duplicate",
		},
		{
			name: "scene parent mismatch",
			files: map[string]string{
				"outline.yaml": strings.Join([]string{
					"version: 1",
					"root:",
					"  arcs:",
					"    - id: arc_00000000000000000001",
					"      chapters:",
					"        - id: ch_00000000000000000001",
					"          scenes:",
					"            - id: scn_00000000000000000001",
					"",
				}, "\n"),
				"arcs/arc_00000000000000000001.yaml":    "version: 1\nid: arc_00000000000000000001\ntitle: Act One\n",
				"chapters/ch_00000000000000000001.yaml": "version: 1\nid: ch_00000000000000000001\narc_id: arc_00000000000000000001\ntitle: Arrival\n",
				"scenes/scn_00000000000000000001.md":    "---\nid: scn_00000000000000000001\ntitle: The Station\nchapter_id: ch_00000000000000000002\npov: \"\"\nstatus: draft\nexclude_from_ai: false\n---\n\n",
			},
			wantError: "chapter_id",
		},
	}

	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			caseRoot := t.TempDir()
			mustMkdirAll(t, caseRoot, "arcs", "chapters", "scenes")
			for relativePath, contents := range testCase.files {
				if err := os.WriteFile(filepath.Join(caseRoot, relativePath), []byte(contents), 0o644); err != nil {
					t.Fatalf("WriteFile(%s) error = %v", relativePath, err)
				}
			}

			_, err := store.Load(ctx, caseRoot)
			if err == nil {
				t.Fatal("Load() error = nil, want failure")
			}
			if !strings.Contains(err.Error(), testCase.wantError) {
				t.Fatalf("Load() error = %q, want substring %q", err, testCase.wantError)
			}
		})
	}

	caseRoot := t.TempDir()
	mustMkdirAll(t, caseRoot, "arcs", "chapters", "scenes")
	for relativePath, contents := range map[string]string{
		"outline.yaml": strings.Join([]string{
			"version: 1",
			"root:",
			"  arcs:",
			"    - id: arc_00000000000000000001",
			"      chapters:",
			"        - id: ch_00000000000000000001",
			"          scenes: []",
			"",
		}, "\n"),
		"arcs/arc_00000000000000000001.yaml":    "version: 1\nid: arc_00000000000000000001\ntitle: Act One\n",
		"chapters/ch_00000000000000000001.yaml": "version: 1\nid: ch_00000000000000000001\narc_id: arc_00000000000000000002\ntitle: Arrival\n",
	} {
		if err := os.WriteFile(filepath.Join(caseRoot, relativePath), []byte(contents), 0o644); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", relativePath, err)
		}
	}
	_, err = store.Load(ctx, caseRoot)
	if err == nil {
		t.Fatal("Load(parent mismatch) error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "arc_id") {
		t.Fatalf("Load(parent mismatch) error = %q, want arc_id detail", err)
	}

	if _, err := os.Stat(filepath.Join(root, "scenes", "scn_00000000000000000001.md")); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("scene file stat error = %v", err)
	}
}

func buildPopulatedOutline(t *testing.T) story.Outline {
	t.Helper()

	outline := story.NewOutline()
	var err error
	outline, err = story.AddArc(outline, "arc_00000000000000000001", "Act One")
	if err != nil {
		t.Fatalf("AddArc() error = %v", err)
	}
	outline, err = story.AddChapter(outline, "arc_00000000000000000001", "ch_00000000000000000001", "Arrival")
	if err != nil {
		t.Fatalf("AddChapter() error = %v", err)
	}
	outline, err = story.AddScene(outline, "ch_00000000000000000001", "scn_00000000000000000001", "The Station")
	if err != nil {
		t.Fatalf("AddScene() error = %v", err)
	}
	return outline
}

func mustMkdirAll(t *testing.T, root string, directories ...string) {
	t.Helper()
	for _, directory := range directories {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", directory, err)
		}
	}
}
