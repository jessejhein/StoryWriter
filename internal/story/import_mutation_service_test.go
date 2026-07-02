package story

import (
	"context"
	"testing"

	"storywork/internal/codex"
	"storywork/internal/project"
)

func TestApplyImportMutationCreatesCanonicalFilesWithoutCheckpoint(t *testing.T) {
	t.Parallel()

	files := &fakeFileStore{
		loadOutline:     NewOutline(),
		exists:          map[string]bool{},
		codexEntryBytes: []byte("entry"),
	}
	git := &fakeGitStore{clean: true}
	index := &fakeIndexStore{}
	service := NewService(
		&fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true},
		files,
		git,
		index,
		&fakeIDGenerator{ids: []string{"arc_0123456789abcdef0123", "char_0123456789abcdef0123"}},
	)

	arcResult, err := service.ApplyImportMutation(context.Background(), ImportMutationRequest{
		Kind:  ImportMutationArc,
		Title: "Act One",
	})
	if err != nil {
		t.Fatalf("ApplyImportMutation(arc) error = %v", err)
	}
	if arcResult.ID != "arc_0123456789abcdef0123" || arcResult.Kind != ImportMutationArc {
		t.Fatalf("arc result = %#v", arcResult)
	}
	if got := string(files.writtenFiles["arcs/arc_0123456789abcdef0123.yaml"]); got != "arc:arc_0123456789abcdef0123" {
		t.Fatalf("arc bytes = %q", got)
	}
	if git.commitCalls != 0 || index.rebuildCalls != 0 {
		t.Fatalf("unexpected side effects: commits=%d rebuilds=%d", git.commitCalls, index.rebuildCalls)
	}

	codexResult, err := service.ApplyImportMutation(context.Background(), ImportMutationRequest{
		Kind: ImportMutationCodex,
		Codex: codex.SaveEntryRequest{
			Type:        codex.TypeCharacter,
			Name:        "Mara Venn",
			Aliases:     []string{"Mara"},
			Tags:        []string{"pilot"},
			Description: "A cautious salvage pilot.",
		},
	})
	if err != nil {
		t.Fatalf("ApplyImportMutation(codex) error = %v", err)
	}
	if codexResult.ID != "char_0123456789abcdef0123" || codexResult.Kind != ImportMutationCodex {
		t.Fatalf("codex result = %#v", codexResult)
	}
	if got := string(files.writtenFiles["codex/characters/char_0123456789abcdef0123.yaml"]); got != "entry" {
		t.Fatalf("codex bytes = %q", got)
	}
}
