package branch

import (
	"context"
	"errors"
	"fmt"
)

func promoteSelectedFiles(ctx context.Context, s *Service, request PromotionRequest) (PromotionResult, error) {
	if _, err := ValidateExperimentID(string(request.ExperimentID)); err != nil {
		return PromotionResult{}, err
	}
	raw := make([]string, len(request.Paths))
	for i, path := range request.Paths {
		raw[i] = string(path)
	}
	selected, err := ValidateSelectedPaths(raw)
	if err != nil {
		return PromotionResult{}, err
	}
	if _, err := ValidateCommitID(string(request.ExpectedMainHead)); err != nil {
		return PromotionResult{}, ErrStaleRef
	}
	if _, err := ValidateCommitID(string(request.ExpectedExperimentHead)); err != nil {
		return PromotionResult{}, ErrStaleRef
	}
	path, err := s.session.ProjectPath()
	if err != nil {
		return PromotionResult{}, err
	}
	s.coordinator.Lock()
	defer s.coordinator.Unlock()

	status, err := s.repo.Status(ctx, path)
	if err != nil {
		return PromotionResult{}, mapRepositoryError(err)
	}
	if !status.IsClean {
		return PromotionResult{}, ErrDirtyWorktree
	}
	if status.ExperimentID != request.ExperimentID || status.ExperimentHead != request.ExpectedExperimentHead {
		return PromotionResult{}, ErrStaleRef
	}
	comparison, err := s.buildComparison(ctx, path, status.ExperimentID)
	if err != nil {
		return PromotionResult{}, err
	}
	if comparison.MainHead != request.ExpectedMainHead {
		return PromotionResult{}, ErrStaleRef
	}
	if err := ValidateFingerprintMatch(request.ExpectedFingerprint, comparison.Fingerprint); err != nil {
		return PromotionResult{}, err
	}
	changedSet := IndexChangedFiles(comparison.Files)
	for _, path := range selected {
		if _, ok := changedSet[path]; !ok {
			return PromotionResult{}, fmt.Errorf("path %q is not changed: %w", path, ErrInvalidPromotion)
		}
	}
	mainChanged, err := s.repo.PathsChanged(ctx, path, comparison.BaseHead, comparison.MainHead)
	if err != nil {
		return PromotionResult{}, mapRepositoryError(err)
	}
	if conflicts := PromotionConflicts(selected, mainChanged); len(conflicts) > 0 {
		return PromotionResult{}, promotionConflictError(conflicts)
	}
	snapshots, err := s.repo.SnapshotMainPaths(ctx, path, comparison.MainHead, selected)
	if err != nil {
		return PromotionResult{}, mapRepositoryError(err)
	}
	if err := s.repo.Switch(ctx, path, CanonBranchName); err != nil {
		recoveryErr := s.repo.Switch(ctx, path, BranchRef(status.ActiveBranch))
		if rebuildErr := s.index.Rebuild(ctx, path); rebuildErr != nil {
			recoveryErr = errors.Join(recoveryErr, rebuildErr)
		}
		return PromotionResult{}, JoinErrors(mapRepositoryError(err), recoveryErr)
	}
	if err := s.repo.ApplyPaths(ctx, path, comparison.ExperimentHead, comparison.Files, selected); err != nil {
		rollbackErr := s.rollbackPromotion(ctx, path, snapshots, selected)
		return PromotionResult{}, JoinErrors(mapRepositoryError(err), rollbackErr)
	}
	if err := s.validator.ValidateProject(ctx, path); err != nil {
		rollbackErr := s.rollbackPromotion(ctx, path, snapshots, selected)
		return PromotionResult{}, JoinErrors(fmt.Errorf("canonical validation failed: %w", err), rollbackErr)
	}
	if err := s.index.Rebuild(ctx, path); err != nil {
		rollbackErr := s.rollbackPromotion(ctx, path, snapshots, selected)
		return PromotionResult{}, JoinErrors(fmt.Errorf("index rebuild failed: %w", err), rollbackErr)
	}
	if err := s.repo.StagePaths(ctx, path, selected); err != nil {
		rollbackErr := s.rollbackPromotion(ctx, path, snapshots, selected)
		return PromotionResult{}, JoinErrors(mapRepositoryError(err), rollbackErr)
	}
	newHead, err := s.repo.CommitPromotion(ctx, path, PromotionCommit{
		ExperimentID: comparison.ExperimentID,
		SourceCommit: comparison.ExperimentHead,
		BaseCommit:   comparison.BaseHead,
		Paths:        selected,
	})
	if err != nil {
		rollbackErr := s.rollbackPromotion(ctx, path, snapshots, selected)
		return PromotionResult{}, JoinErrors(mapRepositoryError(err), rollbackErr)
	}
	return PromotionResult{MainHead: newHead, PromotedPaths: selected, ExperimentID: comparison.ExperimentID}, nil
}

func (s *Service) rollbackPromotion(ctx context.Context, path string, snapshots []PathSnapshot, selected []ProjectPath) error {
	restoreErr := s.repo.RestoreSnapshots(ctx, path, snapshots)
	unstageErr := s.repo.UnstagePaths(ctx, path, selected)
	indexErr := s.index.Rebuild(ctx, path)
	return errors.Join(restoreErr, unstageErr, indexErr)
}

func promotionConflictError(paths []ProjectPath) error {
	return fmt.Errorf("%w: %v", ErrPromotionConflict, paths)
}

// ValidatePromotionPreflight is a pure helper for tests and policy checks.
func ValidatePromotionPreflight(
	comparison Comparison,
	selected []ProjectPath,
	mainChanged []ProjectPath,
	expectedMain CommitID,
	expectedExperiment CommitID,
	expectedFingerprint string,
) error {
	if comparison.MainHead != expectedMain || comparison.ExperimentHead != expectedExperiment {
		return ErrStaleRef
	}
	if err := ValidateFingerprintMatch(expectedFingerprint, comparison.Fingerprint); err != nil {
		return err
	}
	changed := IndexChangedFiles(comparison.Files)
	for _, path := range selected {
		if _, ok := changed[path]; !ok {
			return fmt.Errorf("path %q: %w", path, ErrInvalidPromotion)
		}
	}
	if conflicts := PromotionConflicts(selected, mainChanged); len(conflicts) > 0 {
		return promotionConflictError(conflicts)
	}
	return nil
}
