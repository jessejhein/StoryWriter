// BDD Scenario: 3.2.3 - Report malformed canonical progressions
// Requirements: M3-R05, M3-R18
// Test purpose: Service reads reject stored progression anchors absent from the current canonical outline.
package story

import (
	"context"
	"testing"

	"storywork/internal/codex"
	"storywork/internal/project"
)

func TestLoadProgressionsRejectsAnchorsMissingFromOutline(t *testing.T) {
	t.Parallel()

	description := "Gone."
	service := NewService(
		&fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true},
		&fakeFileStore{
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
				EntryID: "char_0123456789abcdef0123",
				Progressions: []codex.Progression{{
					ID:      "prog_0123456789abcdef0123",
					Anchor:  codex.ProgressionAnchor{Type: "scene", ID: "scn_99999999999999999999", Timing: "after"},
					Changes: codex.ProgressionChange{Description: &description},
				}},
			},
		},
		&fakeGitStore{clean: true},
		&fakeIndexStore{},
		&fakeIDGenerator{},
	)

	// Test: loading progressions fails when a canonical progression anchors to a scene absent from the current outline.
	// Requirements: M3-R18
	if _, err := service.LoadProgressions(context.Background(), "char_0123456789abcdef0123"); err == nil {
		t.Fatal("LoadProgressions() error = nil")
	}
}
