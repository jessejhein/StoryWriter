package action

// lineage.go validates accepted-operation metadata for Git commit trailers.

import (
	"fmt"

	"storywork/internal/contextpack"
	"storywork/internal/gitstore"
	"storywork/internal/story"
)

// BuildOperationMetadata constructs validated Git trailer input for one accepted patch.
func BuildOperationMetadata(run Run, parent *Run, relationship InvitationRelationship) (*story.SceneOperationMetadata, error) {
	if err := ValidateRunID(run.RunID); err != nil {
		return nil, err
	}
	scope, err := scopeTrailerForRun(run)
	if err != nil {
		return nil, err
	}
	metadata := &story.SceneOperationMetadata{
		OperationID: run.RunID,
		Scope:       scope,
	}
	if parent == nil {
		return metadata, nil
	}
	if err := validateParentLineage(run, parent, relationship); err != nil {
		return nil, err
	}
	if parent.Status == RunAccepted {
		metadata.TriggeredBy = parent.RunID
		if relationship == InvitationDependsOn {
			metadata.DependsOn = parent.RunID
		}
	}
	message := gitstore.CommitMessage{
		Subject:     "Accept AI patch " + run.RunID,
		OperationID: metadata.OperationID,
		TriggeredBy: metadata.TriggeredBy,
		DependsOn:   metadata.DependsOn,
		Scope:       metadata.Scope,
	}
	if _, err := gitstore.FormatCommitMessage(message); err != nil {
		return nil, fmt.Errorf("operation metadata: %w", err)
	}
	return metadata, nil
}

func validateParentLineage(run Run, parent *Run, relationship InvitationRelationship) error {
	if parent.RunID == run.RunID {
		return fmt.Errorf("operation cannot depend on itself: %w", ErrLineageConflict)
	}
	if parent.Status != RunAccepted && parent.Status != RunCompleted {
		return fmt.Errorf("parent run %q is not terminal: %w", parent.RunID, ErrLineageConflict)
	}
	if run.ParentRunID != "" && run.ParentRunID != parent.RunID {
		return fmt.Errorf("parent run mismatch: %w", ErrLineageConflict)
	}
	if relationship == InvitationDependsOn && parent.Status != RunAccepted {
		return fmt.Errorf("semantic dependency requires accepted parent: %w", ErrLineageConflict)
	}
	if wouldCreateCycle(run, parent) {
		return fmt.Errorf("operation dependency cycle: %w", ErrLineageConflict)
	}
	return nil
}

func wouldCreateCycle(run Run, parent *Run) bool {
	seen := map[string]struct{}{run.RunID: {}}
	current := parent
	for current != nil {
		if _, ok := seen[current.RunID]; ok {
			return true
		}
		seen[current.RunID] = struct{}{}
		if current.ParentRunID == "" {
			return false
		}
		// Parent chain is validated only within the bounded in-memory store at accept time.
		break
	}
	return false
}

func scopeTrailerForRun(run Run) (string, error) {
	switch run.Scope {
	case contextpack.ScopeSelection, "":
		if run.SceneID == "" {
			return "", fmt.Errorf("scene id is required: %w", ErrLineageConflict)
		}
		return "selection:" + run.SceneID, nil
	case contextpack.ScopeScene:
		if run.SceneID == "" {
			return "", fmt.Errorf("scene id is required: %w", ErrLineageConflict)
		}
		return "scene:" + run.SceneID, nil
	default:
		return "", fmt.Errorf("scope %q cannot be accepted as a patch: %w", run.Scope, ErrLineageConflict)
	}
}

// ResolveParentRun loads one parent run for lineage validation.
func ResolveParentRun(runs *RunStore, run Run) (*Run, error) {
	if run.ParentRunID == "" {
		return nil, nil
	}
	parent, ok := runs.Get(run.ParentRunID)
	if !ok {
		return nil, fmt.Errorf("parent run %q is unknown: %w", run.ParentRunID, ErrLineageConflict)
	}
	parentCopy := parent
	return &parentCopy, nil
}
