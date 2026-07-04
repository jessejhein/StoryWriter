package branch

import (
	"context"
	"errors"
	"fmt"
)

// Service orchestrates branch lifecycle, comparison, promotion, and analysis.
type Service struct {
	repo        Repository
	index       Index
	coordinator MutationCoordinator
	session     ProjectSession
	validator   CanonicalValidator
	analyzer    Analyzer
	ids         IDGenerator
}

// NewService creates a branch service with injected dependencies.
func NewService(
	repo Repository,
	index Index,
	coordinator MutationCoordinator,
	session ProjectSession,
	validator CanonicalValidator,
	analyzer Analyzer,
	ids IDGenerator,
) *Service {
	return &Service{
		repo:        repo,
		index:       index,
		coordinator: coordinator,
		session:     session,
		validator:   validator,
		analyzer:    analyzer,
		ids:         ids,
	}
}

// Status returns the active branch state for the current project.
func (s *Service) Status(ctx context.Context) (RepositoryStatus, error) {
	path, err := s.session.ProjectPath()
	if err != nil {
		return RepositoryStatus{}, err
	}
	s.coordinator.RLock()
	defer s.coordinator.RUnlock()
	status, err := s.repo.Status(ctx, path)
	if err != nil {
		return RepositoryStatus{}, mapRepositoryError(err)
	}
	if status.IsDetached {
		return RepositoryStatus{}, ErrDetachedHEAD
	}
	if !status.IsCanon && !status.IsManaged {
		return RepositoryStatus{}, ErrUnmanagedBranch
	}
	return status, nil
}

// ListExperiments returns managed experiments deterministically.
func (s *Service) ListExperiments(ctx context.Context) ([]ExperimentRef, error) {
	path, err := s.session.ProjectPath()
	if err != nil {
		return nil, err
	}
	s.coordinator.RLock()
	defer s.coordinator.RUnlock()
	return s.repo.ListExperiments(ctx, path)
}

// CreateExperiment creates one experiment from current main and switches to it.
func (s *Service) CreateExperiment(ctx context.Context, name string) (RepositoryStatus, error) {
	path, err := s.session.ProjectPath()
	if err != nil {
		return RepositoryStatus{}, err
	}
	s.coordinator.Lock()
	defer s.coordinator.Unlock()

	status, err := s.repo.Status(ctx, path)
	if err != nil {
		return RepositoryStatus{}, mapRepositoryError(err)
	}
	if !status.IsClean {
		return RepositoryStatus{}, ErrDirtyWorktree
	}
	if status.IsDetached {
		return RepositoryStatus{}, ErrDetachedHEAD
	}
	if !status.IsCanon && !status.IsManaged {
		return RepositoryStatus{}, ErrUnmanagedBranch
	}
	mainHead, err := s.repo.ResolveCommit(ctx, path, CanonBranchName)
	if err != nil {
		return RepositoryStatus{}, ErrMainMissing
	}
	id, err := s.ids.NextExperimentID()
	if err != nil {
		return RepositoryStatus{}, err
	}
	ref, err := BranchRefFromName(name, id)
	if err != nil {
		return RepositoryStatus{}, err
	}
	experiment := ExperimentRef{ID: id, BranchName: ref, Head: mainHead}
	previous := status.ActiveBranch
	if err := s.repo.CreateAndSwitch(ctx, path, experiment, mainHead); err != nil {
		return RepositoryStatus{}, mapRepositoryError(err)
	}
	if err := s.index.Rebuild(ctx, path); err != nil {
		switchErr := s.repo.Switch(ctx, path, BranchRef(previous))
		rebuildErr := s.index.Rebuild(ctx, path)
		deleteErr := s.repo.DeleteExperiment(ctx, path, experiment, mainHead)
		return RepositoryStatus{}, JoinErrors(
			fmt.Errorf("index rebuild failed after checkout: %w", err),
			errors.Join(switchErr, rebuildErr, deleteErr),
		)
	}
	return s.repo.Status(ctx, path)
}

// SwitchTarget switches to main or one experiment by id and expected head.
func (s *Service) SwitchTarget(ctx context.Context, target string, expectedHead *CommitID) (RepositoryStatus, error) {
	path, err := s.session.ProjectPath()
	if err != nil {
		return RepositoryStatus{}, err
	}
	resolved, isMain, err := ValidateSwitchTarget(target)
	if err != nil {
		return RepositoryStatus{}, err
	}
	if (isMain && expectedHead != nil) || (!isMain && expectedHead == nil) {
		return RepositoryStatus{}, fmt.Errorf("switch target and expected head do not match: %w", ErrInvalidBranchRef)
	}
	if expectedHead != nil {
		if _, err := ValidateCommitID(string(*expectedHead)); err != nil {
			return RepositoryStatus{}, err
		}
	}
	s.coordinator.Lock()
	defer s.coordinator.Unlock()

	status, err := s.repo.Status(ctx, path)
	if err != nil {
		return RepositoryStatus{}, mapRepositoryError(err)
	}
	if !status.IsClean {
		return RepositoryStatus{}, ErrDirtyWorktree
	}
	if status.IsDetached && status.ActiveBranch == "" {
		return RepositoryStatus{}, ErrDetachedHEAD
	}
	if !status.IsCanon && !status.IsManaged {
		return RepositoryStatus{}, ErrUnmanagedBranch
	}
	previous := status.ActiveBranch
	var ref BranchRef
	if isMain {
		ref = CanonBranchName
	} else {
		experiments, err := s.repo.ListExperiments(ctx, path)
		if err != nil {
			return RepositoryStatus{}, err
		}
		var found *ExperimentRef
		for i := range experiments {
			if experiments[i].ID == ExperimentID(resolved) {
				found = &experiments[i]
				break
			}
		}
		if found == nil {
			return RepositoryStatus{}, ErrExperimentNotFound
		}
		if expectedHead != nil && found.Head != *expectedHead {
			return RepositoryStatus{}, ErrStaleRef
		}
		ref = found.BranchName
	}
	if err := s.repo.Switch(ctx, path, ref); err != nil {
		return RepositoryStatus{}, mapRepositoryError(err)
	}
	if err := s.index.Rebuild(ctx, path); err != nil {
		switchErr := s.repo.Switch(ctx, path, BranchRef(previous))
		rebuildErr := s.index.Rebuild(ctx, path)
		return RepositoryStatus{}, JoinErrors(
			fmt.Errorf("index rebuild failed after checkout: %w", err),
			errors.Join(switchErr, rebuildErr),
		)
	}
	return s.repo.Status(ctx, path)
}

// LoadComparison compares current main and one experiment head read-only.
func (s *Service) LoadComparison(ctx context.Context, experimentID string) (Comparison, error) {
	id, err := ValidateExperimentID(experimentID)
	if err != nil {
		return Comparison{}, err
	}
	path, err := s.session.ProjectPath()
	if err != nil {
		return Comparison{}, err
	}
	s.coordinator.RLock()
	defer s.coordinator.RUnlock()
	return s.buildComparison(ctx, path, id)
}

func (s *Service) buildComparison(ctx context.Context, path string, id ExperimentID) (Comparison, error) {
	experiments, err := s.repo.ListExperiments(ctx, path)
	if err != nil {
		return Comparison{}, err
	}
	var found *ExperimentRef
	for i := range experiments {
		if experiments[i].ID == id {
			found = &experiments[i]
			break
		}
	}
	if found == nil {
		return Comparison{}, ErrExperimentNotFound
	}
	mainHead, err := s.repo.ResolveCommit(ctx, path, CanonBranchName)
	if err != nil {
		return Comparison{}, ErrMainMissing
	}
	experimentHead := found.Head
	baseHead, err := s.repo.MergeBase(ctx, path, mainHead, experimentHead)
	if err != nil {
		return Comparison{}, mapRepositoryError(err)
	}
	if err := s.validateExperimentAncestry(ctx, path, baseHead, experimentHead); err != nil {
		return Comparison{}, err
	}
	files, err := s.repo.CompareTrees(ctx, path, mainHead, experimentHead)
	if err != nil {
		return Comparison{}, mapRepositoryError(err)
	}
	fingerprint, err := ComputeFingerprint(mainHead, experimentHead, baseHead, files)
	if err != nil {
		return Comparison{}, err
	}
	return Comparison{
		ExperimentID:   id,
		BranchName:     found.BranchName,
		MainHead:       mainHead,
		ExperimentHead: experimentHead,
		BaseHead:       baseHead,
		Fingerprint:    fingerprint,
		Files:          files,
	}, nil
}

func (s *Service) validateExperimentAncestry(ctx context.Context, path string, base, experiment CommitID) error {
	changed, err := s.repo.PathsChanged(ctx, path, base, experiment)
	if err != nil {
		return mapRepositoryError(err)
	}
	_ = changed
	return nil
}

// LoadFileComparison returns bounded side-by-side content for one changed path.
func (s *Service) LoadFileComparison(ctx context.Context, experimentID, rawPath string) (FileComparison, error) {
	id, err := ValidateExperimentID(experimentID)
	if err != nil {
		return FileComparison{}, err
	}
	path, err := ValidateProjectPath(rawPath)
	if err != nil {
		return FileComparison{}, err
	}
	projectPath, err := s.session.ProjectPath()
	if err != nil {
		return FileComparison{}, err
	}
	s.coordinator.RLock()
	defer s.coordinator.RUnlock()

	comparison, err := s.buildComparison(ctx, projectPath, id)
	if err != nil {
		return FileComparison{}, err
	}
	index := IndexChangedFiles(comparison.Files)
	file, ok := index[path]
	if !ok {
		return FileComparison{}, ErrPathNotInComparison
	}
	canon, err := s.repo.ReadTextBlob(ctx, projectPath, comparison.MainHead, path)
	if err != nil {
		return FileComparison{}, err
	}
	experiment, err := s.repo.ReadTextBlob(ctx, projectPath, comparison.ExperimentHead, path)
	if err != nil {
		return FileComparison{}, err
	}
	return FileComparison{
		Path:           path,
		Status:         file.Status,
		MainHead:       comparison.MainHead,
		ExperimentHead: comparison.ExperimentHead,
		Fingerprint:    comparison.Fingerprint,
		Canon:          canon,
		Experiment:     experiment,
	}, nil
}

// DiscardExperiment deletes one experiment, switching to main first when active.
func (s *Service) DiscardExperiment(ctx context.Context, experimentID string, expectedHead CommitID) (RepositoryStatus, error) {
	id, err := ValidateExperimentID(experimentID)
	if err != nil {
		return RepositoryStatus{}, err
	}
	if _, err := ValidateCommitID(string(expectedHead)); err != nil {
		return RepositoryStatus{}, err
	}
	path, err := s.session.ProjectPath()
	if err != nil {
		return RepositoryStatus{}, err
	}
	s.coordinator.Lock()
	defer s.coordinator.Unlock()

	status, err := s.repo.Status(ctx, path)
	if err != nil {
		return RepositoryStatus{}, mapRepositoryError(err)
	}
	if !status.IsClean {
		return RepositoryStatus{}, ErrDirtyWorktree
	}
	if status.IsDetached {
		return RepositoryStatus{}, ErrDetachedHEAD
	}
	if !status.IsCanon && !status.IsManaged {
		return RepositoryStatus{}, ErrUnmanagedBranch
	}
	experiments, err := s.repo.ListExperiments(ctx, path)
	if err != nil {
		return RepositoryStatus{}, err
	}
	var found *ExperimentRef
	for i := range experiments {
		if experiments[i].ID == id {
			found = &experiments[i]
			break
		}
	}
	if found == nil {
		return RepositoryStatus{}, ErrExperimentNotFound
	}
	if found.Head != expectedHead {
		return RepositoryStatus{}, ErrStaleRef
	}
	previous := status.ActiveBranch
	if status.ExperimentID == id {
		if err := s.repo.Switch(ctx, path, CanonBranchName); err != nil {
			return RepositoryStatus{}, mapRepositoryError(err)
		}
		if err := s.index.Rebuild(ctx, path); err != nil {
			switchErr := s.repo.Switch(ctx, path, BranchRef(previous))
			rebuildErr := s.index.Rebuild(ctx, path)
			return RepositoryStatus{}, JoinErrors(
				fmt.Errorf("index rebuild failed after checkout: %w", err),
				errors.Join(switchErr, rebuildErr),
			)
		}
	}
	if err := s.repo.DeleteExperiment(ctx, path, *found, expectedHead); err != nil {
		if status.ExperimentID == id {
			switchErr := s.repo.Switch(ctx, path, found.BranchName)
			rebuildErr := s.index.Rebuild(ctx, path)
			return RepositoryStatus{}, JoinErrors(mapRepositoryError(err), errors.Join(switchErr, rebuildErr))
		}
		return RepositoryStatus{}, mapRepositoryError(err)
	}
	return s.repo.Status(ctx, path)
}

// PromoteSelectedFiles conservatively promotes whole selected files to main.
func (s *Service) PromoteSelectedFiles(ctx context.Context, request PromotionRequest) (PromotionResult, error) {
	return promoteSelectedFiles(ctx, s, request)
}

// AnalyzeRamifications runs explicit bounded ramification analysis.
func (s *Service) AnalyzeRamifications(ctx context.Context, experimentID string, request AnalysisRequest) (AnalysisResult, error) {
	return (&RamificationService{Service: s}).Run(ctx, experimentID, request)
}

// SessionAdapter wraps a function returning the active project path.
type SessionAdapter struct {
	PathFn func() (string, bool)
}

func (a SessionAdapter) ProjectPath() (string, error) {
	path, ok := a.PathFn()
	if !ok || path == "" {
		return "", ErrNoActiveProject
	}
	return path, nil
}

// JoinErrors combines rollback and original errors.
func JoinErrors(original, recovery error) error {
	if recovery == nil {
		return original
	}
	if original == nil {
		return recovery
	}
	return errors.Join(original, fmt.Errorf("recovery failed: %w", recovery))
}
