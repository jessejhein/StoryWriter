// BDD Scenario: 3.1.4 - Reject invalid entry data
// Requirements: M3-R09
// Test purpose: The HTTP status policy maps wrapped story and Codex sentinels to the documented 400, 404, 409, and 500 classes.
package api

import (
	"errors"
	"net/http"
	"testing"

	"storywork/internal/codex"
	"storywork/internal/story"
)

// Test: wrapped conflict, bad-request, not-found, and unknown errors map to their documented HTTP statuses.
// Requirements: M3-R09
func TestStatusForStoryErrorMapsWrappedAndDefaultErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "conflict", err: errors.Join(errors.New("wrapped"), story.ErrDirtyWorktree), want: http.StatusConflict},
		{name: "bad request", err: errors.Join(errors.New("wrapped"), codex.ErrInvalidProgression), want: http.StatusBadRequest},
		{name: "not found", err: errors.Join(errors.New("wrapped"), codex.ErrEntryNotFound), want: http.StatusNotFound},
		{name: "default", err: errors.New("boom"), want: http.StatusInternalServerError},
	}

	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if got := statusForStoryError(testCase.err); got != testCase.want {
				t.Fatalf("statusForStoryError(%v) = %d, want %d", testCase.err, got, testCase.want)
			}
		})
	}
}
