package storyfile

import (
	"context"
	"testing"

	"storywork/internal/story"
)

// BDD trace:
//   - Requirement: Milestone 1, Story 1.2, create structure.
//   - Scenario: given a clean active project, when I create canonical arc,
//     chapter, and scene files, then they serialize deterministically and the new
//     scene starts with strict front matter plus an empty Markdown body.
//   - Test purpose: verify file serialization is deterministic and can be written
//     and loaded back without losing canonical structure.
func TestMarshalAndWriteFilesAreDeterministic(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	root := t.TempDir()
	store := New()
	mustMkdirAll(t, root, "arcs", "chapters", "scenes")

	outline := buildPopulatedOutline(t)
	arc := outline.Arcs[0]
	chapter := arc.Chapters[0]
	scene := chapter.Scenes[0]

	firstOutline := mustMarshalOutline(t, store, outline)
	secondOutline := mustMarshalOutline(t, store, outline)
	if string(firstOutline) != string(secondOutline) {
		t.Fatalf("MarshalOutline() output changed between calls:\n%s\n---\n%s", firstOutline, secondOutline)
	}

	sceneBytes := mustMarshalScene(t, store, scene)
	wantScene := "---\nid: scn_00000000000000000001\ntitle: The Station\nchapter_id: ch_00000000000000000001\npov: \"\"\nstatus: draft\nexclude_from_ai: false\n---\n\n"
	if string(sceneBytes) != wantScene {
		t.Fatalf("MarshalScene() = %q, want %q", string(sceneBytes), wantScene)
	}

	if _, err := store.WriteFiles(ctx, root, map[string][]byte{
		"outline.yaml":                          firstOutline,
		"arcs/arc_00000000000000000001.yaml":    mustMarshalArc(t, store, arc),
		"chapters/ch_00000000000000000001.yaml": mustMarshalChapter(t, store, chapter),
		"scenes/scn_00000000000000000001.md":    sceneBytes,
	}); err != nil {
		t.Fatalf("WriteFiles() error = %v", err)
	}

	loaded, err := store.Load(ctx, root)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := loaded.Arcs[0].Title; got != arc.Title {
		t.Fatalf("arc title = %q, want %q", got, arc.Title)
	}
	if got := loaded.Arcs[0].Chapters[0].Title; got != chapter.Title {
		t.Fatalf("chapter title = %q, want %q", got, chapter.Title)
	}
	if got := loaded.Arcs[0].Chapters[0].Scenes[0].Title; got != scene.Title {
		t.Fatalf("scene title = %q, want %q", got, scene.Title)
	}
}

func mustMarshalOutline(t *testing.T, store *Store, outline story.Outline) []byte {
	t.Helper()

	contents, err := store.MarshalOutline(outline)
	if err != nil {
		t.Fatalf("MarshalOutline() error = %v", err)
	}
	return contents
}

func mustMarshalArc(t *testing.T, store *Store, arc story.Arc) []byte {
	t.Helper()

	contents, err := store.MarshalArc(arc)
	if err != nil {
		t.Fatalf("MarshalArc() error = %v", err)
	}
	return contents
}

func mustMarshalChapter(t *testing.T, store *Store, chapter story.Chapter) []byte {
	t.Helper()

	contents, err := store.MarshalChapter(chapter)
	if err != nil {
		t.Fatalf("MarshalChapter() error = %v", err)
	}
	return contents
}

func mustMarshalScene(t *testing.T, store *Store, scene story.Scene) []byte {
	t.Helper()

	contents, err := store.MarshalScene(scene)
	if err != nil {
		t.Fatalf("MarshalScene() error = %v", err)
	}
	return contents
}
