package action

// target.go validates explicit tagged action targets and legacy normalization.

import (
	"fmt"

	"storywork/internal/agent"
	"storywork/internal/contextpack"
	"storywork/internal/story"
)

// SelectionTarget addresses one UTF-8 byte range inside a canonical scene.
type SelectionTarget struct {
	SceneID       string
	SceneRevision string
	StartByte     int
	EndByte       int
	SelectedText  string
}

// SceneTarget addresses one canonical scene revision.
type SceneTarget struct {
	SceneID       string
	SceneRevision string
}

// ChapterReviewTarget addresses one chapter fingerprint snapshot.
type ChapterReviewTarget struct {
	ChapterID   string
	Fingerprint string
}

// TaggedTarget is the validated action target with exactly one scope payload.
type TaggedTarget struct {
	Scope     contextpack.Scope
	Selection *SelectionTarget
	Scene     *SceneTarget
	Chapter   *ChapterReviewTarget
}

// TaggedRunRequest is the scope-aware action run input.
type TaggedRunRequest struct {
	AgentID string
	StyleID string
	Surface agent.Surface
	Target  TaggedTarget
}

// ValidateSelectionTarget validates a selection-scoped target.
func ValidateSelectionTarget(target SelectionTarget) error {
	if err := story.ValidateSceneID(target.SceneID); err != nil {
		return err
	}
	if err := story.ValidateRevision(target.SceneRevision); err != nil {
		return err
	}
	if target.StartByte < 0 || target.EndByte <= target.StartByte {
		return fmt.Errorf("selection byte range is invalid: %w", ErrInvalidRunRequest)
	}
	if target.SelectedText == "" {
		return fmt.Errorf("selection text is required: %w", ErrInvalidRunRequest)
	}
	return nil
}

// ValidateSceneTarget validates a scene-scoped target.
func ValidateSceneTarget(target SceneTarget) error {
	if err := story.ValidateSceneID(target.SceneID); err != nil {
		return err
	}
	return story.ValidateRevision(target.SceneRevision)
}

// ValidateChapterReviewTarget validates a chapter-review target.
func ValidateChapterReviewTarget(target ChapterReviewTarget) error {
	if err := story.ValidateChapterID(target.ChapterID); err != nil {
		return err
	}
	return story.ValidateRevision(target.Fingerprint)
}

// ValidateTaggedTarget ensures exactly one payload matches the declared scope.
func ValidateTaggedTarget(target TaggedTarget) error {
	count := 0
	if target.Selection != nil {
		count++
	}
	if target.Scene != nil {
		count++
	}
	if target.Chapter != nil {
		count++
	}
	if count != 1 {
		return fmt.Errorf("target must include exactly one scope payload: %w", ErrInvalidRunRequest)
	}
	switch target.Scope {
	case contextpack.ScopeSelection:
		if target.Selection == nil {
			return fmt.Errorf("selection target is required: %w", ErrInvalidRunRequest)
		}
		return ValidateSelectionTarget(*target.Selection)
	case contextpack.ScopeScene:
		if target.Scene == nil {
			return fmt.Errorf("scene target is required: %w", ErrInvalidRunRequest)
		}
		return ValidateSceneTarget(*target.Scene)
	case contextpack.ScopeChapterReview:
		if target.Chapter == nil {
			return fmt.Errorf("chapter review target is required: %w", ErrInvalidRunRequest)
		}
		return ValidateChapterReviewTarget(*target.Chapter)
	default:
		return fmt.Errorf("scope %q is unsupported: %w", target.Scope, ErrInvalidRunRequest)
	}
}

// ValidateTaggedRunRequest validates tagged run syntax before orchestration.
func ValidateTaggedRunRequest(request TaggedRunRequest) error {
	if !registryRequestIDPattern.MatchString(request.AgentID) {
		return fmt.Errorf("agent_id %q is invalid: %w", request.AgentID, ErrInvalidRunRequest)
	}
	if !registryRequestIDPattern.MatchString(request.StyleID) {
		return fmt.Errorf("style_id %q is invalid: %w", request.StyleID, ErrInvalidRunRequest)
	}
	return ValidateTaggedTarget(request.Target)
}

// NormalizeLegacyRunRequest maps Milestone 4-6 selection bodies to tagged targets.
func NormalizeLegacyRunRequest(request RunRequest) (TaggedTarget, error) {
	if err := ValidateRunRequest(request); err != nil {
		return TaggedTarget{}, err
	}
	return TaggedTarget{
		Scope: contextpack.ScopeSelection,
		Selection: &SelectionTarget{
			SceneID:       request.SceneID,
			SceneRevision: request.SceneRevision,
			StartByte:     request.Selection.StartByte,
			EndByte:       request.Selection.EndByte,
			SelectedText:  request.Selection.Text,
		},
	}, nil
}

// ToContextpackTarget converts a validated tagged target for context assembly.
func ToContextpackTarget(target TaggedTarget) contextpack.Target {
	result := contextpack.Target{Scope: target.Scope}
	if target.Selection != nil {
		result.Selection = &contextpack.SelectionTarget{
			SceneID: target.Selection.SceneID, SceneRevision: target.Selection.SceneRevision,
			StartByte: target.Selection.StartByte, EndByte: target.Selection.EndByte,
			SelectedText: target.Selection.SelectedText,
		}
	}
	if target.Scene != nil {
		result.Scene = &contextpack.SceneTarget{
			SceneID: target.Scene.SceneID, SceneRevision: target.Scene.SceneRevision,
		}
	}
	if target.Chapter != nil {
		result.Chapter = &contextpack.ChapterReviewTarget{
			ChapterID: target.Chapter.ChapterID, Fingerprint: target.Chapter.Fingerprint,
		}
	}
	return result
}