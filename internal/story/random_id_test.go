package story_test

import (
	"testing"

	"storywork/internal/story"
)

// BDD trace:
//   - Requirement: Milestone 1 fixed design decision, stable IDs.
//   - Scenario: production-generated arc, chapter, and scene IDs use the exact
//     prefix plus 20 lowercase hexadecimal characters.
//   - Test purpose: verify the production generator's externally visible ID
//     contract for every supported node kind.
func TestRandomIDGeneratorProducesValidIDsForEveryNodeKind(t *testing.T) {
	t.Parallel()

	generator := story.NewRandomIDGenerator()
	tests := []struct {
		kind     story.NodeKind
		validate func(string) error
	}{
		{kind: story.NodeKindArc, validate: story.ValidateArcID},
		{kind: story.NodeKindChapter, validate: story.ValidateChapterID},
		{kind: story.NodeKindScene, validate: story.ValidateSceneID},
	}
	for _, testCase := range tests {
		id, err := generator.Next(testCase.kind)
		if err != nil {
			t.Fatalf("Next(%s) error = %v", testCase.kind, err)
		}
		if err := testCase.validate(id); err != nil {
			t.Fatalf("Next(%s) = %q: %v", testCase.kind, id, err)
		}
	}

	if _, err := generator.Next(story.NodeKind("unsupported")); err == nil {
		t.Fatal("Next(unsupported) error = nil")
	}
}
