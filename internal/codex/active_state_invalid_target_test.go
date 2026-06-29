// BDD Scenario: 3.3.5 - Reject an invalid resolution target
// Requirements: M3-R07, M3-R18
// Test purpose: Active-state resolution rejects malformed and absent target scene IDs without requiring adapters.
package codex

import (
	"errors"
	"testing"
)

func TestResolveActiveStateRejectsInvalidTargetScene(t *testing.T) {
	t.Parallel()

	entry := Entry{
		ID:          "char_0123456789abcdef0123",
		Type:        TypeCharacter,
		Name:        "Ben",
		Aliases:     []string{},
		Tags:        []string{},
		Description: "Guide.",
		Metadata:    map[string]string{},
	}
	orderedScenes := []SceneRef{{ID: "scn_0123456789abcdef0123"}}
	tests := []struct {
		name     string
		targetID string
		want     error
	}{
		{name: "malformed target ID", targetID: "not-a-scene", want: ErrInvalidID},
		{name: "absent target ID", targetID: "scn_0123456789abcdef0124", want: ErrSceneNotFound},
	}

	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			// Test: invalid resolution targets return the typed domain error and no state.
			// Requirements: M3-R07, M3-R18
			_, err := ResolveActiveState(entry, nil, orderedScenes, testCase.targetID)
			if !errors.Is(err, testCase.want) {
				t.Fatalf("ResolveActiveState() error = %v, want %v", err, testCase.want)
			}
		})
	}
}
