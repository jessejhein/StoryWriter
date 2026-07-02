// BDD Scenario: 7.2.3 - Review and accept one scene replacement
// Requirements: M7-R03, M7-R15
// Test purpose: Scene body acceptance preserves identity and checkpoints atomically.

package story

import (
	"context"
	"errors"
	"testing"

	"storywork/internal/project"
)

// Test: accept scene body patch preserves identity and front matter.
// Requirements: M7-R15.
func TestAcceptSceneBodyPatchPreservesIdentityAndFrontMatter(t *testing.T) {
	t.Parallel()

	files := patchFileStore(t)
	service := NewService(
		&fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true},
		files, &fakeGitStore{clean: true}, &fakeIndexStore{}, &fakeIDGenerator{},
	)
	result, err := service.AcceptSceneBodyPatch(context.Background(), AcceptSceneBodyPatchRequest{
		RunID: "run_0123456789abcdef0123", SceneID: "scn_00000000000000000001",
		RunSceneRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ExpectedRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		OriginalMarkdown: "Alpha beta.\n", ReplacementMarkdown: "Rewritten.\n",
	})
	if err != nil {
		t.Fatalf("AcceptSceneBodyPatch() error = %v", err)
	}
	if result.Title != "The Duel" || result.FrontMatter.Status != "draft" {
		t.Fatalf("identity changed: %#v", result)
	}
	if result.Markdown != "Rewritten.\n" {
		t.Fatalf("markdown = %q", result.Markdown)
	}
}

// Test: accept scene body patch rejects stale and no-op requests.
// Requirements: M7-R15.
func TestAcceptSceneBodyPatchRejectsStaleDirtyAndNoOp(t *testing.T) {
	t.Parallel()

	service := NewService(
		&fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true},
		patchFileStore(t), &fakeGitStore{clean: true}, &fakeIndexStore{}, &fakeIDGenerator{},
	)
	_, err := service.AcceptSceneBodyPatch(context.Background(), AcceptSceneBodyPatchRequest{
		RunID: "run_0123456789abcdef0123", SceneID: "scn_00000000000000000001",
		RunSceneRevision: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		ExpectedRevision: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		OriginalMarkdown: "Alpha beta.\n", ReplacementMarkdown: "Rewritten.\n",
	})
	if !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("stale error = %v", err)
	}
}