// BDD Scenario: 3.2.1 - Save ordered progressions
// Requirements: M3-R05, M3-R06, M3-R13, M3-R14, M3-R15, M3-R17
// Test purpose: Progression replacement preserves IDs and order while enforcing revisions, clean worktrees, no-ops, and one checkpoint.
package story

import (
	"context"
	"errors"
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

func TestSaveProgressionsPreservesExistingIDsAndAssignsOnlyNewRows(t *testing.T) {
	t.Parallel()

	currentRevision := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	oldDescription := "Guide."
	newDescription := "Gone."
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
			EntryID: "char_0123456789abcdef0123",
			Progressions: []codex.Progression{{
				ID:      "prog_0123456789abcdef0123",
				Anchor:  codex.ProgressionAnchor{Type: "scene", ID: "scn_00000000000000000001", Timing: "after"},
				Changes: codex.ProgressionChange{Description: &oldDescription},
			}},
			Revision: &currentRevision,
		},
	}
	service := NewService(
		&fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true},
		files,
		&fakeGitStore{clean: true},
		&fakeIndexStore{},
		&fakeIDGenerator{ids: []string{"prog_0123456789abcdef0999"}},
	)

	// Test: omitting an existing row ID preserves that stable ID, while a truly new row receives a fresh backend-generated ID.
	// Requirements: M3-R06
	document, err := service.SaveProgressions(context.Background(), "char_0123456789abcdef0123", codex.SaveProgressionsRequest{
		Progressions: []codex.Progression{
			{
				Anchor:  codex.ProgressionAnchor{Type: "scene", ID: "scn_00000000000000000001", Timing: "after"},
				Changes: codex.ProgressionChange{Description: &newDescription},
			},
			{
				Anchor:  codex.ProgressionAnchor{Type: "scene", ID: "scn_00000000000000000001", Timing: "before"},
				Changes: codex.ProgressionChange{Description: &newDescription},
			},
		},
		ExpectedRevision: &currentRevision,
	})
	if err != nil {
		t.Fatalf("SaveProgressions() error = %v", err)
	}
	if got := document.Progressions[0].ID; got != "prog_0123456789abcdef0123" {
		t.Fatalf("existing progression ID = %q", got)
	}
	if got := document.Progressions[1].ID; got != "prog_0123456789abcdef0999" {
		t.Fatalf("new progression ID = %q", got)
	}
}

func TestSaveProgressionsRejectsInventedProgressionID(t *testing.T) {
	t.Parallel()

	currentRevision := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	description := "Gone."
	files := &fakeFileStore{
		loadOutline: mustSceneOutline(t),
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
			Revision:     &currentRevision,
		},
	}
	service := NewService(
		&fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true},
		files,
		&fakeGitStore{clean: true},
		&fakeIndexStore{},
		&fakeIDGenerator{},
	)

	// Test: callers may not invent progression IDs for new rows; they must omit the ID and let the backend assign it.
	// Requirements: M3-R06
	_, err := service.SaveProgressions(context.Background(), "char_0123456789abcdef0123", codex.SaveProgressionsRequest{
		Progressions: []codex.Progression{{
			ID:      "prog_0123456789abcdef0999",
			Anchor:  codex.ProgressionAnchor{Type: "scene", ID: "scn_00000000000000000001", Timing: "after"},
			Changes: codex.ProgressionChange{Description: &description},
		}},
		ExpectedRevision: &currentRevision,
	})
	if !errors.Is(err, codex.ErrInvalidProgression) {
		t.Fatalf("SaveProgressions() error = %v", err)
	}
	if files.writeCalls != 0 {
		t.Fatalf("write calls = %d, want 0", files.writeCalls)
	}
}

func TestSaveProgressionsRejectsByteIdenticalCanonicalContent(t *testing.T) {
	t.Parallel()

	revision := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	description := "Gone."
	progression := codex.Progression{
		ID:      "prog_0123456789abcdef0123",
		Anchor:  codex.ProgressionAnchor{Type: "scene", ID: "scn_00000000000000000001", Timing: "after"},
		Changes: codex.ProgressionChange{Description: &description},
	}
	files := &fakeFileStore{
		loadOutline: mustSceneOutline(t),
		codexEntry: codex.Entry{
			ID: "char_0123456789abcdef0123", Type: codex.TypeCharacter, Name: "Ben",
			Aliases: []string{}, Tags: []string{}, Description: "Guide.", Metadata: map[string]string{},
		},
		codexProgressions: codex.ProgressionDocument{
			Version: codex.Version, EntryID: "char_0123456789abcdef0123",
			Progressions: []codex.Progression{progression}, Revision: &revision, Canonical: []byte("canonical-progressions"),
		},
		progressionBytes: []byte("canonical-progressions"),
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

	// Test: a progression replacement whose canonical bytes match storage returns no-change without persistence side effects.
	// Requirements: M3-R05, M3-R15
	_, err := service.SaveProgressions(context.Background(), "char_0123456789abcdef0123", codex.SaveProgressionsRequest{
		Progressions: []codex.Progression{progression}, ExpectedRevision: &revision,
	})
	if !errors.Is(err, codex.ErrNoChanges) {
		t.Fatalf("SaveProgressions() error = %v, want %v", err, codex.ErrNoChanges)
	}
	if files.writeCalls != 0 || index.rebuildCalls != 0 || git.commitCalls != 0 {
		t.Fatalf("write/rebuild/commit calls = %d/%d/%d, want 0/0/0", files.writeCalls, index.rebuildCalls, git.commitCalls)
	}
}

// Test: first progression creation rejects a fabricated revision without persistence side effects.
// Requirements: M3-R05, M3-R17
func TestSaveProgressionsRejectsRevisionForFirstDocument(t *testing.T) {
	t.Parallel()

	revision := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	description := "Gone."
	files := &fakeFileStore{
		loadOutline: mustSceneOutline(t),
		codexEntry: codex.Entry{
			ID: "char_0123456789abcdef0123", Type: codex.TypeCharacter, Name: "Ben",
			Aliases: []string{}, Tags: []string{}, Description: "Guide.", Metadata: map[string]string{},
		},
		codexProgressions: codex.ProgressionDocument{
			Version: codex.Version, EntryID: "char_0123456789abcdef0123",
			Progressions: []codex.Progression{}, Revision: nil,
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

	_, err := service.SaveProgressions(context.Background(), "char_0123456789abcdef0123", codex.SaveProgressionsRequest{
		Progressions: []codex.Progression{{
			Anchor:  codex.ProgressionAnchor{Type: "scene", ID: "scn_00000000000000000001", Timing: "after"},
			Changes: codex.ProgressionChange{Description: &description},
		}},
		ExpectedRevision: &revision,
	})
	if !errors.Is(err, codex.ErrInvalidRevision) {
		t.Fatalf("SaveProgressions() error = %v, want %v", err, codex.ErrInvalidRevision)
	}
	if files.writeCalls != 0 || index.rebuildCalls != 0 || git.commitCalls != 0 {
		t.Fatalf("write/rebuild/commit calls = %d/%d/%d, want 0/0/0", files.writeCalls, index.rebuildCalls, git.commitCalls)
	}
}
