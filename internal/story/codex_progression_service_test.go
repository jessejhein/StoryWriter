// BDD Scenario: 3.2.1 - Save ordered progressions
// Requirements: M3-R05, M3-R06, M3-R13, M3-R14, M3-R15, M3-R17
// Test purpose: Plain-English description of the progression replacement mutation rules for stable IDs, ordered saves, dirty-worktree protection, and one checkpoint.
package story

import (
	"context"
	"testing"

	"storywork/internal/codex"
	"storywork/internal/project"
)

func TestSaveProgressionsAssignsIDsAndCommits(t *testing.T) {
	t.Parallel()

	description := "Gone."
	outline := mustSceneOutline(t)
	files := &fakeFileStore{
		loadOutline: outline,
		codexEntry: codex.Entry{
			ID:          "char_0123456789abcdef0123",
			Type:        codex.TypeCharacter,
			Name:        "Ben",
			Description: "Guide.",
			Aliases:     []string{},
			Tags:        []string{},
			Metadata:    map[string]string{},
		},
		codexProgressions: codex.ProgressionDocument{
			EntryID:      "char_0123456789abcdef0123",
			Progressions: []codex.Progression{},
			Revision:     nil,
		},
		progressionBytes: []byte("progressions"),
	}
	git := &fakeGitStore{clean: true}
	index := &fakeIndexStore{}
	service := NewService(
		&fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true},
		files,
		git,
		index,
		&fakeIDGenerator{ids: []string{"prog_0123456789abcdef0123"}},
	)

	// Test: saving a first progression document assigns missing progression IDs, writes one canonical file, rebuilds the index, and commits once.
	// Requirements: M3-R15
	document, err := service.SaveProgressions(context.Background(), "char_0123456789abcdef0123", codex.SaveProgressionsRequest{
		Progressions: []codex.Progression{{
			Anchor:  codex.ProgressionAnchor{Type: "scene", ID: "scn_00000000000000000001", Timing: "after"},
			Changes: codex.ProgressionChange{Description: &description},
		}},
		ExpectedRevision: nil,
	})
	if err != nil {
		t.Fatalf("SaveProgressions() error = %v", err)
	}
	if document.Progressions[0].ID != "prog_0123456789abcdef0123" {
		t.Fatalf("progression ID = %q", document.Progressions[0].ID)
	}
	if git.commitCalls != 1 || git.commitMessages[0] != "Edit progressions char_0123456789abcdef0123" {
		t.Fatalf("commit state = %d %#v", git.commitCalls, git.commitMessages)
	}
	if index.rebuildCalls != 1 {
		t.Fatalf("index rebuild calls = %d", index.rebuildCalls)
	}
}
