// BDD Scenario: 3.3.1 - Resolve before and after an anchor
// Requirements: M3-R07, M3-R08
// Test purpose: Service-level active-state reads use current outline chronology without mutating canon.
package story

import (
	"context"
	"testing"

	"storywork/internal/codex"
	"storywork/internal/project"
)

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
