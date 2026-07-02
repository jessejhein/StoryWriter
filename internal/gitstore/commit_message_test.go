package gitstore

// BDD Scenario: 7.5.1 - Record causal and dependency trailers
// Requirements: M7-R13, M7-R14
// Test purpose: Pure commit message formatting and validation before Git execution.

import (
	"strings"
	"testing"
)

// Test: subject-only commits preserve existing single-line behavior.
// Requirements: M7-R13.
func TestCommitMessageFormatsSubjectOnlyCommit(t *testing.T) {
	t.Parallel()

	got, err := FormatCommitMessage(CommitMessage{Subject: "Save scene scn_0123456789abcdef0123"})
	if err != nil {
		t.Fatalf("FormatCommitMessage() error = %v", err)
	}
	if want := "Save scene scn_0123456789abcdef0123"; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}

// Test: root accepted operations include operation ID and scope trailers.
// Requirements: M7-R13.
func TestCommitMessageFormatsRootOperationTrailers(t *testing.T) {
	t.Parallel()

	got, err := FormatCommitMessage(CommitMessage{
		Subject:     "Accept AI patch run_aaaaaaaaaaaaaaaaaaaa",
		OperationID: "run_aaaaaaaaaaaaaaaaaaaa",
		Scope:       "selection:scn_0123456789abcdef0123",
	})
	if err != nil {
		t.Fatalf("FormatCommitMessage() error = %v", err)
	}
	want := strings.Join([]string{
		"Accept AI patch run_aaaaaaaaaaaaaaaaaaaa",
		"",
		"Storywork-Operation-ID: run_aaaaaaaaaaaaaaaaaaaa",
		"Storywork-Scope: selection:scn_0123456789abcdef0123",
		"",
	}, "\n")
	if got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}

// Test: dependent operations include trigger and dependency trailers.
// Requirements: M7-R13, M7-R14.
func TestCommitMessageFormatsTriggeredAndDependentTrailers(t *testing.T) {
	t.Parallel()

	got, err := FormatCommitMessage(CommitMessage{
		Subject:     "Accept AI patch run_bbbbbbbbbbbbbbbbbbbb",
		OperationID: "run_bbbbbbbbbbbbbbbbbbbb",
		TriggeredBy: "run_aaaaaaaaaaaaaaaaaaaa",
		DependsOn:   "run_aaaaaaaaaaaaaaaaaaaa",
		Scope:       "scene:scn_0123456789abcdef0123",
	})
	if err != nil {
		t.Fatalf("FormatCommitMessage() error = %v", err)
	}
	want := strings.Join([]string{
		"Accept AI patch run_bbbbbbbbbbbbbbbbbbbb",
		"",
		"Storywork-Operation-ID: run_bbbbbbbbbbbbbbbbbbbb",
		"Storywork-Triggered-By: run_aaaaaaaaaaaaaaaaaaaa",
		"Storywork-Depends-On: run_aaaaaaaaaaaaaaaaaaaa",
		"Storywork-Scope: scene:scn_0123456789abcdef0123",
		"",
	}, "\n")
	if got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}

// Test: commit metadata rejects author text injection and invalid IDs.
// Requirements: M7-R13.
func TestCommitMessageRejectsInjectionAndInvalidIDs(t *testing.T) {
	t.Parallel()

	cases := []CommitMessage{
		{Subject: "Accept AI patch run_aaaaaaaaaaaaaaaaaaaa\nInjected", OperationID: "run_aaaaaaaaaaaaaaaaaaaa", Scope: "selection:scn_0123456789abcdef0123"},
		{Subject: "Accept AI patch run_aaaaaaaaaaaaaaaaaaaa", OperationID: "run_bad", Scope: "selection:scn_0123456789abcdef0123"},
		{Subject: "Accept AI patch run_aaaaaaaaaaaaaaaaaaaa", OperationID: "run_aaaaaaaaaaaaaaaaaaaa", Scope: "selection:bad"},
		{Subject: "Accept AI patch run_aaaaaaaaaaaaaaaaaaaa", OperationID: "run_aaaaaaaaaaaaaaaaaaaa", Scope: "selection:scn_0123456789abcdef0123", TriggeredBy: "run_bad"},
	}
	for _, message := range cases {
		if _, err := FormatCommitMessage(message); err == nil {
			t.Fatalf("FormatCommitMessage(%#v) error = nil, want failure", message)
		}
	}
}

// Test: dependency metadata rejects self, duplicate, and cyclic references.
// Requirements: M7-R13, M7-R14.
func TestCommitMessageRejectsSelfDuplicateAndInvalidDependency(t *testing.T) {
	t.Parallel()

	runID := "run_aaaaaaaaaaaaaaaaaaaa"
	triggerOnly := CommitMessage{
		Subject:     "Accept AI patch run_bbbbbbbbbbbbbbbbbbbb",
		OperationID: "run_bbbbbbbbbbbbbbbbbbbb",
		TriggeredBy: "run_aaaaaaaaaaaaaaaaaaaa",
		Scope:       "scene:scn_0123456789abcdef0123",
	}
	if got, err := FormatCommitMessage(triggerOnly); err != nil {
		t.Fatalf("trigger-only message error = %v", err)
	} else if strings.Contains(got, "Storywork-Depends-On:") {
		t.Fatalf("trigger-only message included dependency trailer: %q", got)
	}
	cases := []CommitMessage{
		{Subject: "Accept AI patch " + runID, OperationID: runID, Scope: "scene:scn_0123456789abcdef0123", TriggeredBy: runID},
		{Subject: "Accept AI patch " + runID, OperationID: runID, Scope: "scene:scn_0123456789abcdef0123", DependsOn: runID},
		{Subject: "Accept AI patch run_bbbbbbbbbbbbbbbbbbbb", OperationID: "run_bbbbbbbbbbbbbbbbbbbb", Scope: "scene:scn_0123456789abcdef0123", DependsOn: "run_aaaaaaaaaaaaaaaaaaaa"},
	}
	for _, message := range cases {
		if _, err := FormatCommitMessage(message); err == nil {
			t.Fatalf("FormatCommitMessage(%#v) error = nil, want failure", message)
		}
	}
}
