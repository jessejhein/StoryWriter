package branch

import (
	"context"
	"errors"
	"fmt"

	"storywork/internal/gitstore"
)

// GitRepository adapts gitstore.Store to the branch repository boundary.
type GitRepository struct {
	Store *gitstore.Store
}

func (r *GitRepository) Status(ctx context.Context, repoPath string) (RepositoryStatus, error) {
	status, err := r.Store.Status(ctx, repoPath)
	if err != nil {
		return RepositoryStatus{}, err
	}
	result := RepositoryStatus{
		ActiveBranch: status.ActiveBranch,
		IsCanon:      status.ActiveBranch == CanonBranchName && !status.IsDetached,
		IsManaged:    IsManagedExperimentRef(status.ActiveBranch),
		IsDetached:   status.IsDetached,
		IsClean:      status.IsClean,
		MainHead:     CommitID(status.MainHead),
	}
	if result.IsManaged {
		id, _, err := ParseManagedExperimentRef(status.ActiveBranch)
		if err != nil {
			return RepositoryStatus{}, err
		}
		head, err := r.Store.ResolveCommit(ctx, repoPath, status.ActiveBranch)
		if err != nil {
			return RepositoryStatus{}, err
		}
		result.ExperimentID = id
		result.ExperimentHead = CommitID(head)
	}
	return result, nil
}

func (r *GitRepository) ListExperiments(ctx context.Context, repoPath string) ([]ExperimentRef, error) {
	refs, err := r.Store.ListExperiments(ctx, repoPath)
	if err != nil {
		return nil, err
	}
	experiments := make([]ExperimentRef, 0, len(refs))
	for _, item := range refs {
		id, _, err := ParseManagedExperimentRef(item.Ref)
		if err != nil {
			continue
		}
		experiments = append(experiments, ExperimentRef{
			ID:         id,
			BranchName: BranchRef(item.Ref),
			Head:       CommitID(item.Head),
		})
	}
	return experiments, nil
}

func (r *GitRepository) CreateAndSwitch(ctx context.Context, repoPath string, ref ExperimentRef, start CommitID) error {
	return r.Store.CreateAndSwitch(ctx, repoPath, string(ref.BranchName), string(start))
}

func (r *GitRepository) Switch(ctx context.Context, repoPath string, ref BranchRef) error {
	return r.Store.Switch(ctx, repoPath, string(ref))
}

func (r *GitRepository) DeleteExperiment(ctx context.Context, repoPath string, ref ExperimentRef, expected CommitID) error {
	return r.Store.DeleteExperiment(ctx, repoPath, string(ref.BranchName), string(expected))
}

func (r *GitRepository) CompareTrees(ctx context.Context, repoPath string, left, right CommitID) ([]ChangedFile, error) {
	changes, err := r.Store.CompareTrees(ctx, repoPath, string(left), string(right))
	if err != nil {
		return nil, err
	}
	files := make([]ChangedFile, 0, len(changes))
	for _, change := range changes {
		status, err := ParseChangedStatus(change.Status)
		if err != nil {
			return nil, err
		}
		path, err := ValidateProjectPath(change.Path)
		if err != nil {
			return nil, err
		}
		files = append(files, ChangedFile{Path: path, Status: status})
	}
	return ValidateChangedFiles(files)
}

func (r *GitRepository) ReadTextBlob(ctx context.Context, repoPath string, commit CommitID, path ProjectPath) (TextSide, error) {
	blob, err := r.Store.ReadTextBlob(ctx, repoPath, string(commit), string(path))
	if err != nil {
		return TextSide{}, err
	}
	if !blob.Exists {
		return TextSide{Exists: false, Text: ""}, nil
	}
	text, err := ValidateStrictUTF8(blob.Bytes)
	if err != nil {
		return TextSide{}, err
	}
	return TextSide{Exists: true, Text: text}, nil
}

func (r *GitRepository) MergeBase(ctx context.Context, repoPath string, left, right CommitID) (CommitID, error) {
	base, err := r.Store.MergeBase(ctx, repoPath, string(left), string(right))
	if err != nil {
		return "", err
	}
	return CommitID(base), nil
}

func (r *GitRepository) PathsChanged(ctx context.Context, repoPath string, base, head CommitID) ([]ProjectPath, error) {
	paths, err := r.Store.PathsChanged(ctx, repoPath, string(base), string(head))
	if err != nil {
		return nil, err
	}
	result := make([]ProjectPath, 0, len(paths))
	for _, raw := range paths {
		path, err := ValidateProjectPath(raw)
		if err != nil {
			return nil, err
		}
		result = append(result, path)
	}
	return SortProjectPaths(result), nil
}

func (r *GitRepository) SnapshotMainPaths(ctx context.Context, repoPath string, mainHead CommitID, paths []ProjectPath) ([]PathSnapshot, error) {
	raw := make([]string, len(paths))
	for i, path := range paths {
		raw[i] = string(path)
	}
	snapshots, err := r.Store.SnapshotPaths(ctx, repoPath, string(mainHead), raw)
	if err != nil {
		return nil, err
	}
	result := make([]PathSnapshot, len(snapshots))
	for i, s := range snapshots {
		result[i] = PathSnapshot{Path: ProjectPath(s.Path), Exists: s.Exists, SourceCommit: CommitID(s.SourceCommit)}
	}
	return result, nil
}

func (r *GitRepository) ApplyPaths(ctx context.Context, repoPath string, experiment CommitID, files []ChangedFile, selected []ProjectPath) error {
	changes := make([]gitstore.TreeChange, len(files))
	for i, file := range files {
		var code byte
		switch file.Status {
		case StatusAdded:
			code = 'A'
		case StatusModified:
			code = 'M'
		case StatusDeleted:
			code = 'D'
		}
		changes[i] = gitstore.TreeChange{Path: string(file.Path), Status: code}
	}
	raw := make([]string, len(selected))
	for i, path := range selected {
		raw[i] = string(path)
	}
	return r.Store.ApplyPaths(ctx, repoPath, string(experiment), changes, raw)
}

func (r *GitRepository) StagePaths(ctx context.Context, repoPath string, paths []ProjectPath) error {
	raw := make([]string, len(paths))
	for i, path := range paths {
		raw[i] = string(path)
	}
	return r.Store.StagePaths(ctx, repoPath, raw)
}

func (r *GitRepository) UnstagePaths(ctx context.Context, repoPath string, paths []ProjectPath) error {
	raw := make([]string, len(paths))
	for i, path := range paths {
		raw[i] = string(path)
	}
	return r.Store.UnstagePaths(ctx, repoPath, raw)
}

func (r *GitRepository) RestoreSnapshots(ctx context.Context, repoPath string, snapshots []PathSnapshot) error {
	raw := make([]gitstore.PathSnapshot, len(snapshots))
	for i, s := range snapshots {
		raw[i] = gitstore.PathSnapshot{Path: string(s.Path), Exists: s.Exists, SourceCommit: string(s.SourceCommit)}
	}
	return r.Store.RestorePathSnapshots(ctx, repoPath, raw)
}

func (r *GitRepository) CommitPromotion(ctx context.Context, repoPath string, commit PromotionCommit) (CommitID, error) {
	paths := make([]string, len(commit.Paths))
	for i, path := range commit.Paths {
		paths[i] = string(path)
	}
	head, err := r.Store.CommitPromotion(ctx, repoPath, gitstore.PromotionCommitInput{
		ExperimentID: string(commit.ExperimentID),
		SourceCommit: string(commit.SourceCommit),
		BaseCommit:   string(commit.BaseCommit),
		Paths:        paths,
	})
	if err != nil {
		return "", err
	}
	return CommitID(head), nil
}

func (r *GitRepository) ResolveCommit(ctx context.Context, repoPath, ref string) (CommitID, error) {
	head, err := r.Store.ResolveCommit(ctx, repoPath, ref)
	if err != nil {
		return "", err
	}
	return CommitID(head), nil
}

func (r *GitRepository) IsClean(ctx context.Context, repoPath string) (bool, error) {
	return r.Store.IsClean(ctx, repoPath)
}

func (r *GitRepository) UnifiedDiff(ctx context.Context, repoPath string, mainHead, experimentHead CommitID, paths []ProjectPath, maxBytes int) (string, error) {
	raw := make([]string, len(paths))
	for i, path := range paths {
		raw[i] = string(path)
	}
	diff, err := r.Store.UnifiedDiff(ctx, repoPath, string(mainHead), string(experimentHead), raw, maxBytes)
	if err != nil {
		return "", mapRepositoryError(err)
	}
	text, err := ValidateStrictUTF8([]byte(diff))
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrInvalidAnalysis, err)
	}
	return text, nil
}

// Repository is the Git boundary consumed by branch orchestration.
type Repository interface {
	Status(context.Context, string) (RepositoryStatus, error)
	ListExperiments(context.Context, string) ([]ExperimentRef, error)
	CreateAndSwitch(context.Context, string, ExperimentRef, CommitID) error
	Switch(context.Context, string, BranchRef) error
	DeleteExperiment(context.Context, string, ExperimentRef, CommitID) error
	CompareTrees(context.Context, string, CommitID, CommitID) ([]ChangedFile, error)
	ReadTextBlob(context.Context, string, CommitID, ProjectPath) (TextSide, error)
	MergeBase(context.Context, string, CommitID, CommitID) (CommitID, error)
	PathsChanged(context.Context, string, CommitID, CommitID) ([]ProjectPath, error)
	UnifiedDiff(context.Context, string, CommitID, CommitID, []ProjectPath, int) (string, error)
	SnapshotMainPaths(context.Context, string, CommitID, []ProjectPath) ([]PathSnapshot, error)
	ApplyPaths(context.Context, string, CommitID, []ChangedFile, []ProjectPath) error
	StagePaths(context.Context, string, []ProjectPath) error
	UnstagePaths(context.Context, string, []ProjectPath) error
	RestoreSnapshots(context.Context, string, []PathSnapshot) error
	CommitPromotion(context.Context, string, PromotionCommit) (CommitID, error)
	ResolveCommit(context.Context, string, string) (CommitID, error)
	IsClean(context.Context, string) (bool, error)
}

// Index rebuilds the disposable active-tree index.
type Index interface {
	Rebuild(context.Context, string) error
}

// CanonicalValidator validates a complete on-disk project snapshot.
type CanonicalValidator interface {
	ValidateProject(context.Context, string) error
}

// MutationCoordinator serializes branch-changing operations.
type MutationCoordinator interface {
	Lock()
	Unlock()
	RLock()
	RUnlock()
}

// ProjectSession supplies the active project path.
type ProjectSession interface {
	ProjectPath() (string, error)
}

// ErrNoActiveProject is returned when no project is active.
func mapRepositoryError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, gitstore.ErrDirtyWorktree):
		return errors.Join(ErrDirtyWorktree, err)
	case errors.Is(err, gitstore.ErrStaleExperimentHead):
		return errors.Join(ErrStaleRef, err)
	case errors.Is(err, gitstore.ErrDiffTooLarge):
		return errors.Join(ErrAnalysisBudget, err)
	default:
		return errors.Join(ErrRepositoryState, err)
	}
}
