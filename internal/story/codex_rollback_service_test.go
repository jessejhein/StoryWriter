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

func TestCreateCodexEntryStopsBeforeRollbackWorkWhenAtomicWriteFails(t *testing.T) {
	t.Parallel()

	cause := errors.New("write failed")
	files := &fakeFileStore{codexEntryBytes: []byte("entry"), writeErr: cause}
	git := &fakeGitStore{clean: true}
	index := &fakeIndexStore{}
	service := NewService(
		&fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true},
		files,
		git,
		index,
		&fakeIDGenerator{ids: []string{"char_0123456789abcdef0123"}},
	)

	// Test: a new-entry write failure returns immediately without rollback, unstage, index rebuild, or checkpoint side effects.
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
	if files.rollbackCalls != 0 || git.unstageCalls != 0 || index.rebuildCalls != 0 || git.commitCalls != 0 {
		t.Fatalf("rollback/unstage/rebuild/commit = %d/%d/%d/%d", files.rollbackCalls, git.unstageCalls, index.rebuildCalls, git.commitCalls)
	}
}

func TestCreateCodexEntryRollsBackWhenIndexRebuildFails(t *testing.T) {
	t.Parallel()

	cause := errors.New("index failed")
	files := &fakeFileStore{codexEntryBytes: []byte("entry")}
	git := &fakeGitStore{clean: true}
	index := &fakeIndexStore{rebuildErr: cause}
	service := NewService(
		&fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true},
		files,
		git,
		index,
		&fakeIDGenerator{ids: []string{"char_0123456789abcdef0123"}},
	)

	// Test: a new-entry index failure restores the previous state, unstages the write, and never creates a checkpoint.
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
	if files.rollbackCalls != 1 || git.unstageCalls != 1 || index.rebuildCalls != 2 || git.commitCalls != 0 {
		t.Fatalf("rollback/unstage/rebuild/commit = %d/%d/%d/%d", files.rollbackCalls, git.unstageCalls, index.rebuildCalls, git.commitCalls)
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
func TestSaveProgressionsRollsBackNewDocumentWhenCommitFails(t *testing.T) {
	t.Parallel()

	cause := errors.New("commit failed")
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
			Revision:     nil,
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

	// Test: a failed checkpoint for a first progression document restores the no-file state, unstages changes, and rebuilds the restored index.
	// Requirements: M3-R16
	_, err := service.SaveProgressions(context.Background(), "char_0123456789abcdef0123", codex.SaveProgressionsRequest{
		Progressions: []codex.Progression{{
			Anchor:  codex.ProgressionAnchor{Type: "scene", ID: "scn_00000000000000000001", Timing: "after"},
			Changes: codex.ProgressionChange{Description: &newDescription},
		}},
		ExpectedRevision: nil,
	})
	if !errors.Is(err, cause) {
		t.Fatalf("SaveProgressions() error = %v, want %v", err, cause)
	}
	if files.rollbackCalls != 1 || git.unstageCalls != 1 || index.rebuildCalls != 2 || git.commitCalls != 1 {
		t.Fatalf("rollback/unstage/rebuild/commit = %d/%d/%d/%d", files.rollbackCalls, git.unstageCalls, index.rebuildCalls, git.commitCalls)
	}
}

// BDD Scenario: 3.4.4 - Roll back failures
// Requirements: M3-R15, M3-R16
// Test purpose: Plain-English description of rollback behavior for replacing an existing progression document after write and index failures.
func TestSaveProgressionsExistingDocumentWriteFailureHasNoRollbackSideEffects(t *testing.T) {
	t.Parallel()

	cause := errors.New("write failed")
	currentRevision := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	newDescription := "Gone."
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
			Revision:     &currentRevision,
		},
	}
	git := &fakeGitStore{clean: true}
	index := &fakeIndexStore{}
	service := NewService(
		&fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true},
		files,
		git,
		index,
		&fakeIDGenerator{ids: []string{"prog_0123456789abcdef0999"}},
	)

	// Test: replacing an existing progression document stops before rollback work when the atomic write fails.
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
	if files.rollbackCalls != 0 || git.unstageCalls != 0 || index.rebuildCalls != 0 || git.commitCalls != 0 {
		t.Fatalf("rollback/unstage/rebuild/commit = %d/%d/%d/%d", files.rollbackCalls, git.unstageCalls, index.rebuildCalls, git.commitCalls)
	}
}

// BDD Scenario: 3.4.4 - Roll back failures
// Requirements: M3-R15, M3-R16
// Test purpose: Plain-English description of rollback behavior for replacing an existing progression document after a failed index rebuild.
func TestSaveProgressionsRollsBackExistingDocumentWhenIndexRebuildFails(t *testing.T) {
	t.Parallel()

	cause := errors.New("index failed")
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
	git := &fakeGitStore{clean: true}
	index := &fakeIndexStore{rebuildErr: cause}
	service := NewService(
		&fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true},
		files,
		git,
		index,
		&fakeIDGenerator{ids: []string{"prog_0123456789abcdef0999"}},
	)

	// Test: a failed index rebuild while replacing an existing progression document restores prior bytes and leaves history unchanged.
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
	if files.rollbackCalls != 1 || git.unstageCalls != 1 || index.rebuildCalls != 2 || git.commitCalls != 0 {
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

func TestUpdateCodexEntryStopsBeforeRollbackWorkWhenAtomicWriteFails(t *testing.T) {
	t.Parallel()

	cause := errors.New("write failed")
	files := &fakeFileStore{
		codexEntryBytesSequence: [][]byte{[]byte("updated"), []byte("current")},
		writeErr:                cause,
		codexEntry: codex.Entry{
			ID:          "char_0123456789abcdef0123",
			Type:        codex.TypeCharacter,
			Name:        "Ben",
			Aliases:     []string{},
			Tags:        []string{},
			Description: "Guide.",
			Metadata:    map[string]string{},
			Revision:    "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	}
	git := &fakeGitStore{clean: true}
	index := &fakeIndexStore{}
	service := NewService(
		&fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true},
		files,
		git,
		index,
		&fakeIDGenerator{},
	)

	// Test: an existing-entry write failure returns immediately without rollback, unstage, index rebuild, or checkpoint side effects.
	// Requirements: M3-R16
	_, err := service.UpdateCodexEntry(context.Background(), "char_0123456789abcdef0123", codex.SaveEntryRequest{
		Name:             "Ben Kenobi",
		Aliases:          []string{},
		Tags:             []string{},
		Description:      "Guide.",
		Metadata:         map[string]string{},
		ExpectedRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})
	if !errors.Is(err, cause) {
		t.Fatalf("UpdateCodexEntry() error = %v, want %v", err, cause)
	}
	if files.rollbackCalls != 0 || git.unstageCalls != 0 || index.rebuildCalls != 0 || git.commitCalls != 0 {
		t.Fatalf("rollback/unstage/rebuild/commit = %d/%d/%d/%d", files.rollbackCalls, git.unstageCalls, index.rebuildCalls, git.commitCalls)
	}
}

func TestUpdateCodexEntryRollsBackWhenIndexRebuildFails(t *testing.T) {
	t.Parallel()

	cause := errors.New("index failed")
	files := &fakeFileStore{
		codexEntryBytesSequence: [][]byte{[]byte("updated"), []byte("current")},
		codexEntry: codex.Entry{
			ID:          "char_0123456789abcdef0123",
			Type:        codex.TypeCharacter,
			Name:        "Ben",
			Aliases:     []string{},
			Tags:        []string{},
			Description: "Guide.",
			Metadata:    map[string]string{},
			Revision:    "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	}
	git := &fakeGitStore{clean: true}
	index := &fakeIndexStore{rebuildErr: cause}
	service := NewService(
		&fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true},
		files,
		git,
		index,
		&fakeIDGenerator{},
	)

	// Test: an existing-entry index failure restores previous bytes, unstages changes, and leaves no checkpoint.
	// Requirements: M3-R16
	_, err := service.UpdateCodexEntry(context.Background(), "char_0123456789abcdef0123", codex.SaveEntryRequest{
		Name:             "Ben Kenobi",
		Aliases:          []string{},
		Tags:             []string{},
		Description:      "Guide.",
		Metadata:         map[string]string{},
		ExpectedRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})
	if !errors.Is(err, cause) {
		t.Fatalf("UpdateCodexEntry() error = %v, want %v", err, cause)
	}
	if files.rollbackCalls != 1 || git.unstageCalls != 1 || index.rebuildCalls != 2 || git.commitCalls != 0 {
		t.Fatalf("rollback/unstage/rebuild/commit = %d/%d/%d/%d", files.rollbackCalls, git.unstageCalls, index.rebuildCalls, git.commitCalls)
	}
}

func TestUpdateCodexEntryRollsBackWhenCheckpointFails(t *testing.T) {
	t.Parallel()

	cause := errors.New("commit failed")
	files := &fakeFileStore{
		codexEntryBytesSequence: [][]byte{[]byte("updated"), []byte("current")},
		codexEntry: codex.Entry{
			ID:          "char_0123456789abcdef0123",
			Type:        codex.TypeCharacter,
			Name:        "Ben",
			Aliases:     []string{},
			Tags:        []string{},
			Description: "Guide.",
			Metadata:    map[string]string{},
			Revision:    "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	}
	git := &fakeGitStore{clean: true, commitErr: cause}
	index := &fakeIndexStore{}
	service := NewService(
		&fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true},
		files,
		git,
		index,
		&fakeIDGenerator{},
	)

	// Test: a failed existing-entry checkpoint restores previous bytes, unstages the mutation, rebuilds the restored index, and surfaces the failure.
	// Requirements: M3-R16
	_, err := service.UpdateCodexEntry(context.Background(), "char_0123456789abcdef0123", codex.SaveEntryRequest{
		Name:             "Ben Kenobi",
		Aliases:          []string{},
		Tags:             []string{},
		Description:      "Guide.",
		Metadata:         map[string]string{},
		ExpectedRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})
	if !errors.Is(err, cause) {
		t.Fatalf("UpdateCodexEntry() error = %v, want %v", err, cause)
	}
	if files.rollbackCalls != 1 || git.unstageCalls != 1 || index.rebuildCalls != 2 || git.commitCalls != 1 {
		t.Fatalf("rollback/unstage/rebuild/commit = %d/%d/%d/%d", files.rollbackCalls, git.unstageCalls, index.rebuildCalls, git.commitCalls)
	}
}
