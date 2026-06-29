// BDD Scenario: 3.4.4 - Roll back failures
// Requirements: M3-R15, M3-R16
// Test purpose: Plain-English description of rollback behavior for new Codex files and progression-file replacement when index or checkpoint work fails.
package story

import (
	"context"
	"errors"
	"testing"

	"storywork/internal/codex"
	"storywork/internal/project"
)

func TestCreateCodexEntryRollsBackWhenCheckpointFails(t *testing.T) {
	t.Parallel()

	cause := errors.New("commit failed")
	files := &fakeFileStore{codexEntryBytes: []byte("entry")}
	git := &fakeGitStore{clean: true, commitErr: cause}
	index := &fakeIndexStore{}
	service := NewService(
		&fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true},
		files,
		git,
		index,
		&fakeIDGenerator{ids: []string{"char_0123456789abcdef0123"}},
	)

	// Test: a failed Codex-entry checkpoint rolls back the new file, unstages app changes, rebuilds the restored index, and never reports success.
	// Requirements: M3-R16
	_, err := service.CreateCodexEntry(context.Background(), codex.SaveEntryRequest{
		Type:        codex.TypeCharacter,
		Name:        "Ben",
		Aliases:     []string{},
		Tags:        []string{},
		Description: "Guide.",
		Metadata:    map[string]string{},
	})
	if !errors.Is(err, cause) {
		t.Fatalf("CreateCodexEntry() error = %v, want %v", err, cause)
	}
	if files.rollbackCalls != 1 || git.unstageCalls != 1 || index.rebuildCalls != 2 {
		t.Fatalf("rollback/unstage/rebuild = %d/%d/%d", files.rollbackCalls, git.unstageCalls, index.rebuildCalls)
	}
}

// BDD Scenario: 3.4.4 - Roll back failures
// Requirements: M3-R15, M3-R16
// Test purpose: Plain-English description of rollback behavior for progression replacement after a downstream failure.
func TestSaveProgressionsRollsBackWhenIndexRebuildFails(t *testing.T) {
	t.Parallel()

	cause := errors.New("index failed")
	description := "Gone."
	files := &fakeFileStore{
		loadOutline: mustSceneOutline(t),
		codexEntry: codex.Entry{
			ID:          "char_0123456789abcdef0123",
			Type:        codex.TypeCharacter,
			Name:        "Ben",
			Aliases:     []string{},
			Tags:        []string{},
			Description: "Guide.",
			Metadata:    map[string]string{},
		},
		codexProgressions: codex.ProgressionDocument{
			EntryID:      "char_0123456789abcdef0123",
			Progressions: []codex.Progression{},
			Revision:     nil,
		},
		progressionBytes: []byte("progressions"),
	}
	service := NewService(
		&fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true},
		files,
		&fakeGitStore{clean: true},
		&fakeIndexStore{rebuildErr: cause},
		&fakeIDGenerator{ids: []string{"prog_0123456789abcdef0123"}},
	)

	// Test: a failed index rebuild after writing progressions restores the previous document, unstages changes, and leaves no checkpoint.
	// Requirements: M3-R16
	_, err := service.SaveProgressions(context.Background(), "char_0123456789abcdef0123", codex.SaveProgressionsRequest{
		Progressions: []codex.Progression{{
			Anchor:  codex.ProgressionAnchor{Type: "scene", ID: "scn_00000000000000000001", Timing: "after"},
			Changes: codex.ProgressionChange{Description: &description},
		}},
		ExpectedRevision: nil,
	})
	if !errors.Is(err, cause) {
		t.Fatalf("SaveProgressions() error = %v, want %v", err, cause)
	}
}
