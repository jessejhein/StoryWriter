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
	if files.rollbackCalls != 1 {
		t.Fatalf("rollback calls = %d, want 1", files.rollbackCalls)
	}
}

// BDD Scenario: 3.4.4 - Roll back failures
// Requirements: M3-R15, M3-R16
// Test purpose: Plain-English description of progression-save failure handling for pre-write errors and replacement rollback after a failed checkpoint.
func TestSaveProgressionsWriteFailureHasNoRollbackSideEffects(t *testing.T) {
	t.Parallel()

	cause := errors.New("write failed")
	description := "Gone."
	files := &fakeFileStore{
		loadOutline: mustSceneOutline(t),
		writeErr:    cause,
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

	// Test: a write failure returns immediately without rollback, unstage, index rebuild, or checkpoint side effects because canonical bytes never changed.
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
	if files.rollbackCalls != 0 || git.unstageCalls != 0 || index.rebuildCalls != 0 || git.commitCalls != 0 {
		t.Fatalf("rollback/unstage/rebuild/commit = %d/%d/%d/%d", files.rollbackCalls, git.unstageCalls, index.rebuildCalls, git.commitCalls)
	}
}

// BDD Scenario: 3.4.4 - Roll back failures
// Requirements: M3-R15, M3-R16
// Test purpose: Plain-English description of rollback behavior for replacing an existing progression document after a failed commit.
func TestSaveProgressionsRollsBackExistingDocumentWhenCommitFails(t *testing.T) {
	t.Parallel()

	cause := errors.New("commit failed")
	currentRevision := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	newDescription := "Gone."
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
			Revision:     &currentRevision,
		},
		progressionBytes: []byte("progressions"),
	}
	git := &fakeGitStore{clean: true, commitErr: cause}
	index := &fakeIndexStore{}
	service := NewService(
		&fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true},
		files,
		git,
		index,
		&fakeIDGenerator{ids: []string{"prog_0123456789abcdef0999"}},
	)

	// Test: a failed checkpoint while replacing an existing progression document restores the prior bytes, unstages changes, rebuilds the restored index, and surfaces the failure.
	// Requirements: M3-R16
	_, err := service.SaveProgressions(context.Background(), "char_0123456789abcdef0123", codex.SaveProgressionsRequest{
		Progressions: []codex.Progression{{
			Anchor:  codex.ProgressionAnchor{Type: "scene", ID: "scn_00000000000000000001", Timing: "after"},
			Changes: codex.ProgressionChange{Description: &newDescription},
		}},
		ExpectedRevision: &currentRevision,
	})
	if !errors.Is(err, cause) {
		t.Fatalf("SaveProgressions() error = %v, want %v", err, cause)
	}
	if files.rollbackCalls != 1 || git.unstageCalls != 1 || index.rebuildCalls != 2 || git.commitCalls != 1 {
		t.Fatalf("rollback/unstage/rebuild/commit = %d/%d/%d/%d", files.rollbackCalls, git.unstageCalls, index.rebuildCalls, git.commitCalls)
	}
}
