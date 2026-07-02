package story

// BDD Scenario: 7.5.1 - Record causal and dependency trailers
// Requirements: M7-R13, M7-R14, M7-R15
// Test purpose: Accepted scene patches write validated Git operation trailers without weakening rollback safety.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"storywork/internal/gitstore"
	"storywork/internal/project"
)

// Test: valid operation metadata produces exact commit trailers on acceptance.
// Requirements: M7-R13.
func TestAcceptScenePatchCommitsValidatedOperationMetadata(t *testing.T) {
	t.Parallel()

	git := &fakeGitStore{clean: true}
	service := NewService(
		&fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true},
		patchFileStore(t), git, &fakeIndexStore{}, &fakeIDGenerator{},
	)
	request := patchRequest()
	request.Operation = &SceneOperationMetadata{
		OperationID: "run_0123456789abcdef0123",
		TriggeredBy: "run_aaaaaaaaaaaaaaaaaaaa",
		DependsOn:   "run_aaaaaaaaaaaaaaaaaaaa",
		Scope:       "selection:scn_00000000000000000001",
	}

	if _, err := service.AcceptScenePatch(context.Background(), request); err != nil {
		t.Fatalf("AcceptScenePatch() error = %v", err)
	}
	if git.commitMessageCalls != 1 {
		t.Fatalf("commit message calls = %d, want 1", git.commitMessageCalls)
	}
	want, err := gitstore.FormatCommitMessage(gitstore.CommitMessage{
		Subject:     "Accept AI patch run_0123456789abcdef0123",
		OperationID: "run_0123456789abcdef0123",
		TriggeredBy: "run_aaaaaaaaaaaaaaaaaaaa",
		DependsOn:   "run_aaaaaaaaaaaaaaaaaaaa",
		Scope:       "selection:scn_00000000000000000001",
	})
	if err != nil {
		t.Fatalf("FormatCommitMessage() error = %v", err)
	}
	if git.commitMessages[0] != want {
		t.Fatalf("commit message = %q, want %q", git.commitMessages[0], want)
	}
}

// Test: invalid operation metadata fails before canonical writes.
// Requirements: M7-R13, M7-R15.
func TestAcceptScenePatchRejectsInvalidMetadataBeforeWrite(t *testing.T) {
	t.Parallel()

	files := patchFileStore(t)
	git := &fakeGitStore{clean: true}
	service := NewService(
		&fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true},
		files, git, &fakeIndexStore{}, &fakeIDGenerator{},
	)
	request := patchRequest()
	request.Operation = &SceneOperationMetadata{
		OperationID: "run_bad",
		Scope:       "selection:scn_00000000000000000001",
	}

	_, err := service.AcceptScenePatch(context.Background(), request)
	if err == nil || !errors.Is(err, gitstore.ErrInvalidCommitMessage) {
		t.Fatalf("AcceptScenePatch() error = %v, want %v", err, gitstore.ErrInvalidCommitMessage)
	}
	if files.writeCalls != 0 {
		t.Fatalf("write calls = %d, want 0 before invalid metadata rejection", files.writeCalls)
	}
	if git.commitMessageCalls != 0 {
		t.Fatalf("commit message calls = %d, want 0", git.commitMessageCalls)
	}
}

// Test: trailer commit failure rolls back bytes like other persistence boundaries.
// Requirements: M7-R15.
func TestAcceptScenePatchTrailerCommitFailureRollsBackBytes(t *testing.T) {
	t.Parallel()

	cause := errors.New("trailer commit failed")
	files := patchFileStore(t)
	git := &fakeGitStore{clean: true, commitMessageErr: cause}
	service := NewService(
		&fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true},
		files, git, &fakeIndexStore{}, &fakeIDGenerator{},
	)
	request := patchRequest()
	request.Operation = &SceneOperationMetadata{
		OperationID: "run_0123456789abcdef0123",
		Scope:       "selection:scn_00000000000000000001",
	}

	_, err := service.AcceptScenePatch(context.Background(), request)
	if !errors.Is(err, cause) {
		t.Fatalf("AcceptScenePatch() error = %v, want %v", err, cause)
	}
	if files.rollbackCalls != 1 || git.unstageCalls != 1 {
		t.Fatalf("rollback/unstage = %d/%d, want 1/1", files.rollbackCalls, git.unstageCalls)
	}
	if git.commitMessageCalls != 1 {
		t.Fatalf("commit message calls = %d, want 1", git.commitMessageCalls)
	}
	if !strings.Contains(git.commitMessages[0], "Storywork-Operation-ID:") {
		t.Fatalf("attempted commit message = %q, want operation trailer", git.commitMessages[0])
	}
}
