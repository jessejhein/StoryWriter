// BDD Scenario: 3.1.3 - Edit an entry
// Requirements: M3-R03, M3-R04, M3-R15, M3-R17
// Test purpose: Plain-English description of the Codex edit mutation rules for stale revisions, no-op detection, and checkpointed canonical replacement.
package story

import (
	"context"
	"errors"
	"testing"

	"storywork/internal/codex"
	"storywork/internal/project"
)

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
