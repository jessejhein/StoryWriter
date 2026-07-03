// BDD Scenario: 7.3.2 - Return suggestions without canon mutation
// Requirements: M7-R04, M7-R17
// Test purpose: Invalid generated findings map to a provider-output gateway failure.

package api

import (
	"net/http"
	"testing"

	"storywork/internal/action"
)

// Test: invalid generated output maps to Bad Gateway.
// Requirements: M7-R17.
func TestMilestone7InvalidProviderOutputMapsToBadGateway(t *testing.T) {
	t.Parallel()

	if got := statusForStoryError(action.ErrProviderRejected); got != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", got, http.StatusBadGateway)
	}
}
