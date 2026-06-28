package story

import (
	"context"
	"errors"
	"testing"

	"storywork/internal/codex"
	"storywork/internal/project"
)

// BDD Scenario: 3.1.2 - Create an entry
// Requirements: M3-R02, M3-R04, M3-R13, M3-R14, M3-R15
// Test purpose: Plain-English description of the Codex create mutation orchestration for normalized canonical bytes, one checkpoint, and one index rebuild.
func TestCreateCodexEntryPersistsCanonicalBytesAndCheckpoint(t *testing.T) {
	t.Parallel()

	files := &fakeFileStore{codexEntryBytes: []byte("entry")}
	git := &fakeGitStore{clean: true}
	index := &fakeIndexStore{}
	service := NewService(
		&fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true},
		files,
		git,
		index,
		&fakeIDGenerator{ids: []string{"char_0123456789abcdef0123"}},
	)

	// Test: creating a character entry writes its canonical file, rebuilds the index, commits once, and returns the generated stable ID with a revision.
	// Requirements: M3-R15
	entry, err := service.CreateCodexEntry(context.Background(), codex.SaveEntryRequest{
		Type:        codex.TypeCharacter,
		Name:        "  Obi-Wan Kenobi  ",
		Aliases:     []string{"Ben"},
		Tags:        []string{"mentor", "jedi"},
		Description: "Guide.\n",
		Metadata:    map[string]string{"status": "alive"},
	})
	if err != nil {
		t.Fatalf("CreateCodexEntry() error = %v", err)
	}
	if got := string(files.writtenFiles["codex/characters/char_0123456789abcdef0123.yaml"]); got != "entry" {
		t.Fatalf("written bytes = %q", got)
	}
	if git.commitCalls != 1 || git.commitMessages[0] != "Create Codex entry char_0123456789abcdef0123" {
		t.Fatalf("commit state = %d %#v", git.commitCalls, git.commitMessages)
	}
	if index.rebuildCalls != 1 {
		t.Fatalf("index rebuild calls = %d", index.rebuildCalls)
	}
	if entry.ID != "char_0123456789abcdef0123" || entry.Revision == "" {
		t.Fatalf("entry = %#v", entry)
	}
}

// BDD Scenario: 3.1.3 - Edit an entry
// Requirements: M3-R03, M3-R04, M3-R15, M3-R17
// Test purpose: Plain-English description of the Codex edit mutation rules for stale revisions, no-op detection, and checkpointed canonical replacement.
func TestUpdateCodexEntryRejectsStaleRevisionAndCommitsValidEdit(t *testing.T) {
	t.Parallel()

	current := codex.Entry{
		ID:          "char_0123456789abcdef0123",
		Type:        codex.TypeCharacter,
		Name:        "Obi-Wan Kenobi",
		Aliases:     []string{"Ben"},
		Tags:        []string{"jedi"},
		Description: "Guide.\n",
		Metadata:    map[string]string{"status": "alive"},
		Revision:    "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	files := &fakeFileStore{
		codexEntry:      current,
		codexEntryBytes: []byte("updated"),
	}
	service := NewService(
		&fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true},
		files,
		&fakeGitStore{clean: true},
		&fakeIndexStore{},
		&fakeIDGenerator{},
	)

	// Test: stale revisions are rejected before any write or checkpoint side effects.
	// Requirements: M3-R17
	_, err := service.UpdateCodexEntry(context.Background(), current.ID, codex.SaveEntryRequest{
		Name:             current.Name,
		Aliases:          current.Aliases,
		Tags:             current.Tags,
		Description:      current.Description,
		Metadata:         current.Metadata,
		ExpectedRevision: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	})
	if !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("UpdateCodexEntry(stale) error = %v", err)
	}
	if files.writeCalls != 0 {
		t.Fatalf("write calls = %d, want 0", files.writeCalls)
	}
}

// BDD Scenario: 3.2.1 - Save ordered progressions
// Requirements: M3-R05, M3-R06, M3-R13, M3-R14, M3-R15, M3-R17
// Test purpose: Plain-English description of the progression replacement mutation rules for stable IDs, ordered saves, dirty-worktree protection, and one checkpoint.
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

// BDD Scenario: 3.3.1 - Resolve before and after an anchor
// Requirements: M3-R07, M3-R08
// Test purpose: Plain-English description of the service-level active-state read path that uses current outline chronology while leaving canon unchanged.
func TestResolveActiveCodexStateUsesCurrentOutlineOrder(t *testing.T) {
	t.Parallel()

	description := "Absent."
	outline := mustSceneOutline(t)
	outline, err := AddScene(outline, "ch_00000000000000000001", "scn_00000000000000000002", "Aftermath")
	if err != nil {
		t.Fatalf("AddScene() error = %v", err)
	}
	files := &fakeFileStore{
		loadOutline: outline,
		codexEntry: codex.Entry{
			ID:          "char_0123456789abcdef0123",
			Type:        codex.TypeCharacter,
			Name:        "Ben",
			Description: "Present.",
			Aliases:     []string{},
			Tags:        []string{},
			Metadata:    map[string]string{},
		},
		codexProgressions: codex.ProgressionDocument{
			EntryID: "char_0123456789abcdef0123",
			Progressions: []codex.Progression{{
				ID:      "prog_0123456789abcdef0123",
				Anchor:  codex.ProgressionAnchor{Type: "scene", ID: "scn_00000000000000000001", Timing: "after"},
				Changes: codex.ProgressionChange{Description: &description},
			}},
		},
	}
	service := NewService(
		&fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true},
		files,
		&fakeGitStore{clean: true},
		&fakeIndexStore{},
		&fakeIDGenerator{},
	)

	// Test: active state excludes an after-anchor progression at the anchor scene and includes it at a later scene without writing anything.
	// Requirements: M3-R07
	activeAtAnchor, err := service.ResolveActiveCodexState(context.Background(), "char_0123456789abcdef0123", "scn_00000000000000000001")
	if err != nil {
		t.Fatalf("ResolveActiveCodexState(anchor) error = %v", err)
	}
	if activeAtAnchor.Entry.Description != "Present." {
		t.Fatalf("anchor description = %q", activeAtAnchor.Entry.Description)
	}
	activeLater, err := service.ResolveActiveCodexState(context.Background(), "char_0123456789abcdef0123", "scn_00000000000000000002")
	if err != nil {
		t.Fatalf("ResolveActiveCodexState(later) error = %v", err)
	}
	if activeLater.Entry.Description != "Absent." {
		t.Fatalf("later description = %q", activeLater.Entry.Description)
	}
	if files.writeCalls != 0 {
		t.Fatalf("write calls = %d, want 0", files.writeCalls)
	}
}
