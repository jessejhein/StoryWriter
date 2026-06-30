// BDD Scenario: 3.1.2 - Create an entry
// Requirements: M3-R02, M3-R04, M3-R13, M3-R14, M3-R15
// Test purpose: Codex creation writes normalized canonical bytes with one index rebuild and one checkpoint.
package story

import (
	"context"
	"testing"

	"storywork/internal/codex"
	"storywork/internal/project"
)

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
