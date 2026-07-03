// BDD Scenario: 8.4.1 - Promote selected files to main
// Requirements: M8-R16
// Test purpose: Promotion commit messages use exact provenance trailers only.

package gitstore_test

import (
	"strings"
	"testing"

	"storywork/internal/gitstore"
)

// Test: exact subject and trailer order.
// Requirements: M8-R16.
func TestFormatPromotionMessageExactBytes(t *testing.T) {
	t.Parallel()
	message, err := gitstore.FormatPromotionMessage(gitstore.PromotionMessage{
		ExperimentID: "brn_0123456789abcdef0123",
		SourceCommit: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		BaseCommit:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})
	if err != nil {
		t.Fatalf("FormatPromotionMessage() error = %v", err)
	}
	want := "Promote what-if brn_0123456789abcdef0123\n\nStorywork-Experiment-ID: brn_0123456789abcdef0123\nStorywork-Source-Commit: bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\nStorywork-Base-Commit: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n"
	if message != want {
		t.Fatalf("message = %q, want %q", message, want)
	}
	if strings.Contains(message, "\r") {
		t.Fatal("message contains CR")
	}
}
