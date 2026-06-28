package storyfile

import (
	"context"
	"testing"

	"storywork/internal/story"
)

// BDD trace:
//   - Requirement: M2-R01, M2-R02, M2-R09.
//   - Scenario: 2.1.1 — Load a valid scene.
//   - Test purpose: verify canonical scene files round-trip with the supported
//     metadata, Markdown body, deterministic serialization, and revision hash.
func TestSceneRoundTripsWithCanonicalSerialization(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	root := t.TempDir()
	store := New()
	mustMkdirAll(t, root, "scenes")

	scene := story.SceneDocument{
		ID:        "scn_00000000000000000001",
		ChapterID: "ch_00000000000000000001",
		Title:     "The Duel",
		FrontMatter: story.SceneFrontMatter{
			POV:           "Luke",
			Status:        "revised",
			ExcludeFromAI: true,
		},
		Markdown: "First line.\n\nSecond line.\n",
	}

	contents, err := store.MarshalSceneDocument(scene)
	if err != nil {
		t.Fatalf("MarshalSceneDocument() error = %v", err)
	}
	want := "---\nid: scn_00000000000000000001\ntitle: \"The Duel\"\nchapter_id: ch_00000000000000000001\npov: \"Luke\"\nstatus: revised\nexclude_from_ai: true\n---\n\nFirst line.\n\nSecond line.\n"
	if string(contents) != want {
		t.Fatalf("MarshalSceneDocument() = %q, want %q", string(contents), want)
	}

	if _, err := store.WriteFiles(ctx, root, map[string][]byte{"scenes/scn_00000000000000000001.md": contents}); err != nil {
		t.Fatalf("WriteFiles() error = %v", err)
	}

	loaded, err := store.LoadScene(ctx, root, "scn_00000000000000000001")
	if err != nil {
		t.Fatalf("LoadScene() error = %v", err)
	}
	if loaded.ID != scene.ID || loaded.ChapterID != scene.ChapterID || loaded.Title != scene.Title {
		t.Fatalf("LoadScene() identity = %#v", loaded)
	}
	if loaded.FrontMatter != scene.FrontMatter {
		t.Fatalf("front matter = %#v, want %#v", loaded.FrontMatter, scene.FrontMatter)
	}
	if loaded.Markdown != scene.Markdown {
		t.Fatalf("markdown = %q, want %q", loaded.Markdown, scene.Markdown)
	}
	if loaded.Revision != story.ComputeRevision(contents) {
		t.Fatalf("revision = %q, want %q", loaded.Revision, story.ComputeRevision(contents))
	}
}
