package story

import (
	"context"
	"testing"

	"storywork/internal/project"
)

// BDD trace:
//   - Requirement: Milestone 1, Story 1.2, create structure.
//   - Scenario: given a clean active project, when I create an arc, chapter, and
//     scene, then each mutation writes canonical files, rebuilds the index, and
//     creates exactly one Git checkpoint with the required message.
//   - Test purpose: verify the mutation service orchestrates file writes, index
//     rebuilds, ID generation, and commit messages for structure creation.
func TestCreateMutationsWriteFilesRebuildIndexAndCommit(t *testing.T) {
	t.Parallel()

	session := &fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true}
	files := &fakeFileStore{exists: map[string]bool{}}
	git := &fakeGitStore{clean: true}
	index := &fakeIndexStore{}
	ids := &fakeIDGenerator{ids: []string{
		"arc_00000000000000000001",
		"ch_00000000000000000001",
		"scn_00000000000000000001",
	}}
	service := NewService(session, files, git, index, ids)

	arcResult, err := service.CreateArc(context.Background(), "Act One")
	if err != nil {
		t.Fatalf("CreateArc() error = %v", err)
	}
	if arcResult.ChangedID != "arc_00000000000000000001" {
		t.Fatalf("CreateArc() changed_id = %q", arcResult.ChangedID)
	}
	if _, ok := files.writtenFiles["outline.yaml"]; !ok {
		t.Fatal("CreateArc() did not write outline.yaml")
	}
	if _, ok := files.writtenFiles["arcs/arc_00000000000000000001.yaml"]; !ok {
		t.Fatal("CreateArc() did not write arc file")
	}
	files.loadOutline = arcResult.Outline

	files.writtenFiles = nil
	chapterResult, err := service.CreateChapter(context.Background(), "arc_00000000000000000001", "Arrival")
	if err != nil {
		t.Fatalf("CreateChapter() error = %v", err)
	}
	if chapterResult.ChangedID != "ch_00000000000000000001" {
		t.Fatalf("CreateChapter() changed_id = %q", chapterResult.ChangedID)
	}
	if _, ok := files.writtenFiles["chapters/ch_00000000000000000001.yaml"]; !ok {
		t.Fatal("CreateChapter() did not write chapter file")
	}
	files.loadOutline = chapterResult.Outline

	files.writtenFiles = nil
	sceneResult, err := service.CreateScene(context.Background(), "ch_00000000000000000001", "The Station")
	if err != nil {
		t.Fatalf("CreateScene() error = %v", err)
	}
	if sceneResult.ChangedID != "scn_00000000000000000001" {
		t.Fatalf("CreateScene() changed_id = %q", sceneResult.ChangedID)
	}
	if _, ok := files.writtenFiles["scenes/scn_00000000000000000001.md"]; !ok {
		t.Fatal("CreateScene() did not write scene file")
	}

	if git.commitCalls != 3 {
		t.Fatalf("commit calls = %d, want 3", git.commitCalls)
	}
	wantMessages := []string{
		"Add arc arc_00000000000000000001",
		"Add chapter ch_00000000000000000001",
		"Add scene scn_00000000000000000001",
	}
	for i, want := range wantMessages {
		if git.commitMessages[i] != want {
			t.Fatalf("commit message %d = %q, want %q", i, git.commitMessages[i], want)
		}
	}
	if index.rebuildCalls != 3 {
		t.Fatalf("index rebuild calls = %d, want 3", index.rebuildCalls)
	}
}
