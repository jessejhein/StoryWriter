package gitstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// BranchStatus reports active branch, cleanliness, and heads.
type BranchStatus struct {
	ActiveBranch string
	IsDetached   bool
	IsClean      bool
	MainHead     string
}

// ExperimentBranch is one managed experiment ref.
type ExperimentBranch struct {
	Ref  string
	Head string
}

// TreeChange is one changed path between two commits.
type TreeChange struct {
	Path   string
	Status byte // A, M, D
}

// TextBlob is UTF-8 text content or absence at a commit.
type TextBlob struct {
	Exists bool
	Bytes  []byte
}

// PathSnapshot records one path's presence at a source commit for rollback.
type PathSnapshot struct {
	Path         string
	Exists       bool
	SourceCommit string
}

// PromotionCommitInput is validated provenance for one promotion commit.
type PromotionCommitInput struct {
	ExperimentID     string
	SourceCommit     string
	BaseCommit       string
	ExpectedMainHead string
	Paths            []string
}

func experimentBaseRef(ref string) (string, error) {
	if err := validateBranchRef(ref); err != nil {
		return "", err
	}
	if ref == canonBranch {
		return "", fmt.Errorf("branch ref %q is invalid", ref)
	}
	suffix := strings.TrimPrefix(ref, experimentNamespace)
	parts := strings.Split(suffix, "-")
	if len(parts) < 2 {
		return "", fmt.Errorf("branch ref %q is invalid", ref)
	}
	hex := parts[len(parts)-1]
	if !experimentHexPattern.MatchString(hex) {
		return "", fmt.Errorf("branch ref %q is invalid", ref)
	}
	return "refs/storywork/experiment-base/brn_" + hex, nil
}

func writeUpdateRefTransaction(commands ...string) string {
	if len(commands) == 0 {
		return ""
	}
	return strings.Join(commands, "\n") + "\n"
}

// Status returns active branch, cleanliness, and current main head.
func (s *Store) Status(ctx context.Context, repoPath string) (BranchStatus, error) {
	active, detached, err := s.activeBranch(ctx, repoPath)
	if err != nil {
		return BranchStatus{}, err
	}
	clean, err := s.IsClean(ctx, repoPath)
	if err != nil {
		return BranchStatus{}, err
	}
	mainHead, err := s.ResolveCommit(ctx, repoPath, canonBranch)
	if err != nil {
		return BranchStatus{}, fmt.Errorf("resolve main head: %w", err)
	}
	return BranchStatus{
		ActiveBranch: active,
		IsDetached:   detached,
		IsClean:      clean,
		MainHead:     mainHead,
	}, nil
}

func (s *Store) activeBranch(ctx context.Context, repoPath string) (string, bool, error) {
	output, err := s.run(ctx, "-C", repoPath, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err == nil {
		return strings.TrimSpace(output), false, nil
	}
	head, headErr := s.run(ctx, "-C", repoPath, "rev-parse", "--verify", "HEAD^{commit}")
	if headErr != nil {
		return "", false, fmt.Errorf("inspect active branch: %w", err)
	}
	return strings.TrimSpace(head), true, nil
}

// ResolveCommit resolves one validated ref to a full lowercase commit id.
func (s *Store) ResolveCommit(ctx context.Context, repoPath, ref string) (string, error) {
	if ref != canonBranch && ref != "HEAD" {
		if err := validateBranchRef(ref); err != nil {
			if err := validateCommitID(ref); err != nil {
				return "", err
			}
		}
	}
	output, err := s.run(ctx, "-C", repoPath, "rev-parse", "--verify", ref+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("resolve commit %q: %w", ref, err)
	}
	commit := strings.TrimSpace(output)
	if err := validateCommitID(commit); err != nil {
		return "", err
	}
	return commit, nil
}

// ListExperiments returns managed branch/ refs sorted by ref name.
func (s *Store) ListExperiments(ctx context.Context, repoPath string) ([]ExperimentBranch, error) {
	output, err := s.run(ctx, "-C", repoPath, "for-each-ref", "--format=%(refname:strip=2)%00%(objectname)%00", "refs/heads/branch/")
	if err != nil {
		return nil, fmt.Errorf("list experiment refs: %w", err)
	}
	if strings.TrimSpace(output) == "" {
		return []ExperimentBranch{}, nil
	}
	normalized := strings.ReplaceAll(output, "\x00\n", "\x00")
	normalized = strings.TrimSuffix(normalized, "\n")
	records := strings.Split(strings.TrimSuffix(normalized, "\x00"), "\x00")
	if len(records)%2 != 0 {
		return nil, errors.New("malformed experiment ref listing")
	}
	var experiments []ExperimentBranch
	for i := 0; i < len(records); i += 2 {
		ref := records[i]
		if err := validateBranchRef(ref); err != nil {
			return nil, fmt.Errorf("reserved experiment ref %q is invalid: %w", ref, err)
		}
		head := records[i+1]
		if err := validateCommitID(head); err != nil {
			return nil, fmt.Errorf("experiment %q has invalid head: %w", ref, err)
		}
		experiments = append(experiments, ExperimentBranch{Ref: ref, Head: head})
	}
	return experiments, nil
}

// CreateAndSwitch creates one experiment ref at startCommit and checks it out.
func (s *Store) CreateAndSwitch(ctx context.Context, repoPath, ref, startCommit, baseCommit string) error {
	if err := validateBranchRef(ref); err != nil {
		return err
	}
	if err := validateCommitID(startCommit); err != nil {
		return err
	}
	if err := validateCommitID(baseCommit); err != nil {
		return err
	}
	clean, err := s.IsClean(ctx, repoPath)
	if err != nil {
		return err
	}
	if !clean {
		return ErrDirtyWorktree
	}
	previous, detached, err := s.activeBranch(ctx, repoPath)
	if err != nil {
		return err
	}
	if detached {
		return errors.New("detached HEAD is not supported")
	}
	baseRef, err := experimentBaseRef(ref)
	if err != nil {
		return err
	}
	transaction := writeUpdateRefTransaction(
		"create refs/heads/"+ref+" "+startCommit,
		"create "+baseRef+" "+baseCommit,
	)
	command := exec.CommandContext(ctx, s.executable, "-C", repoPath, "update-ref", "--stdin")
	command.Stdin = strings.NewReader(transaction)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w: %s", strings.Join(command.Args, " "), err, strings.TrimSpace(string(output)))
	}
	if _, err := s.run(ctx, "-C", repoPath, "checkout", ref); err != nil {
		cleanupCtx, cancel := cleanupContext(ctx)
		defer cancel()
		_, switchErr := s.run(cleanupCtx, "-C", repoPath, "checkout", previous)
		deleteTxn := writeUpdateRefTransaction(
			"delete refs/heads/"+ref+" "+startCommit,
			"delete "+baseRef+" "+baseCommit,
		)
		deleteCommand := exec.CommandContext(cleanupCtx, s.executable, "-C", repoPath, "update-ref", "--stdin")
		deleteCommand.Stdin = strings.NewReader(deleteTxn)
		deleteOutput, deleteErr := deleteCommand.CombinedOutput()
		if deleteErr != nil {
			deleteErr = fmt.Errorf("%s: %w: %s", strings.Join(deleteCommand.Args, " "), deleteErr, strings.TrimSpace(string(deleteOutput)))
		}
		return fmt.Errorf("checkout branch %q: %w", ref, errors.Join(err, switchErr, deleteErr))
	}
	return nil
}

// Switch checks out one validated ref when the worktree is clean.
func (s *Store) Switch(ctx context.Context, repoPath, ref string) error {
	if err := validateBranchRef(ref); err != nil {
		return err
	}
	clean, err := s.IsClean(ctx, repoPath)
	if err != nil {
		return err
	}
	if !clean {
		return ErrDirtyWorktree
	}
	if _, err := s.run(ctx, "-C", repoPath, "checkout", ref); err != nil {
		return fmt.Errorf("checkout branch %q: %w", ref, err)
	}
	return nil
}

// SwitchExperiment checks out one managed experiment exactly at the expected head.
func (s *Store) SwitchExperiment(ctx context.Context, repoPath, ref, expectedHead string) error {
	if err := validateBranchRef(ref); err != nil {
		return err
	}
	if err := validateCommitID(expectedHead); err != nil {
		return err
	}
	clean, err := s.IsClean(ctx, repoPath)
	if err != nil {
		return err
	}
	if !clean {
		return ErrDirtyWorktree
	}
	current, detached, err := s.activeBranch(ctx, repoPath)
	if err != nil {
		return err
	}
	if detached {
		return errors.New("detached HEAD is not supported")
	}
	before, err := s.ResolveCommit(ctx, repoPath, ref)
	if err != nil {
		return fmt.Errorf("resolve experiment head: %w", err)
	}
	if before != expectedHead {
		return ErrStaleExperimentHead
	}
	if _, err := s.run(ctx, "-C", repoPath, "checkout", ref); err != nil {
		return fmt.Errorf("checkout branch %q: %w", ref, err)
	}
	activeHead, err := s.ResolveCommit(ctx, repoPath, "HEAD")
	if err != nil {
		cleanupCtx, cancel := cleanupContext(ctx)
		defer cancel()
		_, switchErr := s.run(cleanupCtx, "-C", repoPath, "checkout", current)
		return fmt.Errorf("verify checkout head %q: %w", ref, errors.Join(err, switchErr))
	}
	after, err := s.ResolveCommit(ctx, repoPath, ref)
	if err != nil || activeHead != expectedHead || after != expectedHead {
		if err == nil {
			err = ErrStaleExperimentHead
		}
		cleanupCtx, cancel := cleanupContext(ctx)
		defer cancel()
		_, switchErr := s.run(cleanupCtx, "-C", repoPath, "checkout", current)
		return errors.Join(ErrStaleExperimentHead, errors.Join(err, switchErr))
	}
	return nil
}

// DeleteExperiment deletes one managed experiment ref at the expected head.
func (s *Store) DeleteExperiment(ctx context.Context, repoPath, ref, expectedHead, baseCommit string) error {
	if err := validateBranchRef(ref); err != nil {
		return err
	}
	if err := validateCommitID(expectedHead); err != nil {
		return err
	}
	if err := validateCommitID(baseCommit); err != nil {
		return err
	}
	baseRef, err := experimentBaseRef(ref)
	if err != nil {
		return err
	}
	transaction := writeUpdateRefTransaction(
		"delete refs/heads/"+ref+" "+expectedHead,
		"delete "+baseRef+" "+baseCommit,
	)
	command := exec.CommandContext(ctx, s.executable, "-C", repoPath, "update-ref", "--stdin")
	command.Stdin = strings.NewReader(transaction)
	output, err := command.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "cannot lock ref") {
			return errors.Join(ErrStaleExperimentHead, fmt.Errorf("%s: %w: %s", strings.Join(command.Args, " "), err, strings.TrimSpace(string(output))))
		}
		return fmt.Errorf("delete branch %q: %w: %s", ref, err, strings.TrimSpace(string(output)))
	}
	return nil
}

// CompareTrees diffs leftTree versus rightTree with rename detection disabled.
func (s *Store) CompareTrees(ctx context.Context, repoPath, leftTree, rightTree string) ([]TreeChange, error) {
	if err := validateCommitID(leftTree); err != nil {
		return nil, err
	}
	if err := validateCommitID(rightTree); err != nil {
		return nil, err
	}
	output, err := s.run(ctx, "-C", repoPath, "diff", "--no-renames", "--name-status", "-z", leftTree, rightTree)
	if err != nil {
		return nil, fmt.Errorf("compare trees: %w", err)
	}
	changes, err := parseNameStatusZ(output, maxChangedPaths)
	if err != nil {
		return nil, err
	}
	for _, change := range changes {
		if change.Status == 'A' || change.Status == 'M' {
			exists, _, err := s.inspectRegularTreePath(ctx, repoPath, rightTree, change.Path)
			if err != nil || !exists {
				if err == nil {
					err = errors.New("expected regular file is absent")
				}
				return nil, fmt.Errorf("inspect right tree path %q: %w", change.Path, err)
			}
		}
		if change.Status == 'D' || change.Status == 'M' {
			exists, _, err := s.inspectRegularTreePath(ctx, repoPath, leftTree, change.Path)
			if err != nil || !exists {
				if err == nil {
					err = errors.New("expected regular file is absent")
				}
				return nil, fmt.Errorf("inspect left tree path %q: %w", change.Path, err)
			}
		}
	}
	return changes, nil
}

// MergeBase returns the merge base between two commits.
func (s *Store) MergeBase(ctx context.Context, repoPath, left, right string) (string, error) {
	if err := validateCommitID(left); err != nil {
		return "", err
	}
	if err := validateCommitID(right); err != nil {
		return "", err
	}
	output, err := s.run(ctx, "-C", repoPath, "merge-base", left, right)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return "", ErrNoMergeBase
		}
		return "", fmt.Errorf("merge base: %w", err)
	}
	base := strings.TrimSpace(output)
	if err := validateCommitID(base); err != nil {
		return "", fmt.Errorf("invalid merge base: %w", err)
	}
	return base, nil
}

// IsAncestor reports whether ancestor is in descendant's history.
func (s *Store) IsAncestor(ctx context.Context, repoPath, ancestor, descendant string) (bool, error) {
	if err := validateCommitID(ancestor); err != nil {
		return false, err
	}
	if err := validateCommitID(descendant); err != nil {
		return false, err
	}
	command := exec.CommandContext(ctx, s.executable, "-C", repoPath, "merge-base", "--is-ancestor", ancestor, descendant)
	output, err := command.CombinedOutput()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("%s: %w: %s", strings.Join(command.Args, " "), err, strings.TrimSpace(string(output)))
}

// PathsChanged returns paths changed between baseCommit and headCommit.
func (s *Store) PathsChanged(ctx context.Context, repoPath, baseCommit, headCommit string) ([]string, error) {
	if err := validateCommitID(baseCommit); err != nil {
		return nil, err
	}
	if err := validateCommitID(headCommit); err != nil {
		return nil, err
	}
	output, err := s.runLimitedText(ctx, maxPathListBytes, "-C", repoPath, "diff", "--no-renames", "--name-only", "-z", baseCommit, headCommit)
	if err != nil {
		return nil, fmt.Errorf("paths changed: %w", err)
	}
	return parsePathListZ(output, maxChangedPaths)
}

// ReadTextBlob reads one regular-file blob at commit without checkout.
func (s *Store) ReadTextBlob(ctx context.Context, repoPath, commit, projectPath string) (TextBlob, error) {
	if err := validateCommitID(commit); err != nil {
		return TextBlob{}, err
	}
	if err := validateProjectPath(projectPath); err != nil {
		return TextBlob{}, err
	}
	exists, objectID, err := s.inspectRegularTreePath(ctx, repoPath, commit, projectPath)
	if err != nil {
		return TextBlob{}, err
	}
	if !exists {
		return TextBlob{Exists: false}, nil
	}
	sizeOutput, err := s.run(ctx, "-C", repoPath, "cat-file", "-s", objectID)
	if err != nil {
		return TextBlob{}, fmt.Errorf("inspect blob size %q at %s: %w", projectPath, commit, err)
	}
	size, err := strconv.ParseInt(strings.TrimSpace(sizeOutput), 10, 64)
	if err != nil {
		return TextBlob{}, fmt.Errorf("parse blob size %q at %s: %w", projectPath, commit, err)
	}
	if size > 5<<20 {
		return TextBlob{}, ErrBlobTooLarge
	}
	output, err := s.runBytesLimited(ctx, 5<<20, "-C", repoPath, "cat-file", "blob", objectID)
	if err != nil {
		return TextBlob{}, fmt.Errorf("read blob %q at %s: %w", projectPath, commit, err)
	}
	return TextBlob{Exists: true, Bytes: output}, nil
}

// ResolveExperimentBase resolves the immutable base provenance for a managed experiment.
func (s *Store) ResolveExperimentBase(ctx context.Context, repoPath, ref string) (string, error) {
	baseRef, err := experimentBaseRef(ref)
	if err != nil {
		return "", err
	}
	output, err := s.run(ctx, "-C", repoPath, "rev-parse", "--verify", baseRef+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("resolve experiment base %q: %w", ref, err)
	}
	base := strings.TrimSpace(output)
	if err := validateCommitID(base); err != nil {
		return "", err
	}
	return base, nil
}

func (s *Store) inspectRegularTreePath(ctx context.Context, repoPath, commit, projectPath string) (bool, string, error) {
	entry, err := s.runBytes(ctx, "-C", repoPath, "ls-tree", "-z", commit, "--", projectPath)
	if err != nil {
		return false, "", fmt.Errorf("inspect blob %q at %s: %w", projectPath, commit, err)
	}
	if len(entry) == 0 {
		return false, "", nil
	}
	metadata, listedPath, ok := strings.Cut(strings.TrimSuffix(string(entry), "\x00"), "\t")
	fields := strings.Fields(metadata)
	if !ok || listedPath != projectPath || len(fields) != 3 || fields[1] != "blob" || (fields[0] != "100644" && fields[0] != "100755") {
		return false, "", fmt.Errorf("path %q is not a regular file", projectPath)
	}
	if err := validateCommitID(fields[2]); err != nil {
		return false, "", fmt.Errorf("path %q has invalid blob id: %w", projectPath, err)
	}
	return true, fields[2], nil
}

// SnapshotPaths captures blob object ids and modes for paths at one commit.
func (s *Store) SnapshotPaths(ctx context.Context, repoPath, commit string, paths []string) ([]PathSnapshot, error) {
	if err := validateCommitID(commit); err != nil {
		return nil, err
	}
	snapshots := make([]PathSnapshot, 0, len(paths))
	for _, projectPath := range paths {
		if err := validateProjectPath(projectPath); err != nil {
			return nil, err
		}
		exists, _, err := s.inspectRegularTreePath(ctx, repoPath, commit, projectPath)
		if err != nil {
			return nil, fmt.Errorf("inspect blob %q at %s: %w", projectPath, commit, err)
		}
		if !exists {
			snapshots = append(snapshots, PathSnapshot{Path: projectPath, Exists: false, SourceCommit: commit})
			continue
		}
		snapshots = append(snapshots, PathSnapshot{Path: projectPath, Exists: true, SourceCommit: commit})
	}
	return snapshots, nil
}

// ApplyPaths applies experiment commit blobs and deletions onto the worktree.
func (s *Store) ApplyPaths(ctx context.Context, repoPath, experimentCommit string, changes []TreeChange, selected []string) error {
	if err := validateCommitID(experimentCommit); err != nil {
		return err
	}
	selectedSet := make(map[string]struct{}, len(selected))
	for _, projectPath := range selected {
		if err := validateProjectPath(projectPath); err != nil {
			return err
		}
		selectedSet[projectPath] = struct{}{}
	}
	changeByPath := make(map[string]byte, len(changes))
	for _, change := range changes {
		changeByPath[change.Path] = change.Status
	}
	ordered := make([]string, 0, len(selectedSet))
	for projectPath := range selectedSet {
		ordered = append(ordered, projectPath)
	}
	sort.Strings(ordered)
	for _, projectPath := range ordered {
		status, ok := changeByPath[projectPath]
		if !ok {
			return fmt.Errorf("path %q is not changed", projectPath)
		}
		switch status {
		case 'D':
			if err := os.Remove(filepath.Join(repoPath, filepath.FromSlash(projectPath))); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("delete %q: %w", projectPath, err)
			}
		case 'A', 'M':
			if _, err := s.run(ctx, "-C", repoPath, "restore", "--source="+experimentCommit, "--worktree", "--", projectPath); err != nil {
				return fmt.Errorf("apply %q: %w", projectPath, err)
			}
		default:
			return fmt.Errorf("unsupported status for %q", projectPath)
		}
	}
	return nil
}

// StagePaths stages exactly the selected paths.
func (s *Store) StagePaths(ctx context.Context, repoPath string, paths []string) error {
	if len(paths) == 0 {
		return errors.New("paths are required")
	}
	args := []string{"-C", repoPath, "add", "--"}
	for _, projectPath := range paths {
		if err := validateProjectPath(projectPath); err != nil {
			return err
		}
		args = append(args, projectPath)
	}
	if _, err := s.run(ctx, args...); err != nil {
		return fmt.Errorf("stage paths: %w", err)
	}
	return nil
}

// UnstagePaths removes app-created staging for selected paths.
func (s *Store) UnstagePaths(ctx context.Context, repoPath string, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	args := []string{"-C", repoPath, "restore", "--staged", "--"}
	for _, projectPath := range paths {
		if err := validateProjectPath(projectPath); err != nil {
			return err
		}
		args = append(args, projectPath)
	}
	if _, err := s.run(ctx, args...); err != nil {
		return fmt.Errorf("unstage paths: %w", err)
	}
	return nil
}

// RestorePathSnapshots restores selected paths from captured snapshots.
func (s *Store) RestorePathSnapshots(ctx context.Context, repoPath string, snapshots []PathSnapshot) error {
	var restoreErr error
	for _, snapshot := range snapshots {
		if err := validateProjectPath(snapshot.Path); err != nil {
			restoreErr = errors.Join(restoreErr, err)
			continue
		}
		if err := validateCommitID(snapshot.SourceCommit); err != nil {
			restoreErr = errors.Join(restoreErr, err)
			continue
		}
		if !snapshot.Exists {
			absolute := filepath.Join(repoPath, filepath.FromSlash(snapshot.Path))
			if err := os.Remove(absolute); err != nil && !os.IsNotExist(err) {
				restoreErr = errors.Join(restoreErr, fmt.Errorf("restore deletion %q: %w", snapshot.Path, err))
			}
			continue
		}
		if _, err := s.run(ctx, "-C", repoPath, "restore", "--source="+snapshot.SourceCommit, "--worktree", "--", snapshot.Path); err != nil {
			restoreErr = errors.Join(restoreErr, fmt.Errorf("restore %q: %w", snapshot.Path, err))
		}
	}
	return restoreErr
}

// CommitPromotion creates one provenance commit for selected paths.
func (s *Store) CommitPromotion(ctx context.Context, repoPath string, input PromotionCommitInput) (string, error) {
	message, err := FormatPromotionMessage(PromotionMessage{
		ExperimentID: input.ExperimentID,
		SourceCommit: input.SourceCommit,
		BaseCommit:   input.BaseCommit,
	})
	if err != nil {
		return "", err
	}
	if len(input.Paths) == 0 {
		return "", errors.New("paths are required")
	}
	if err := validateCommitID(input.ExpectedMainHead); err != nil {
		return "", err
	}
	for _, projectPath := range input.Paths {
		if err := validateProjectPath(projectPath); err != nil {
			return "", err
		}
	}
	treeOutput, err := s.run(ctx, "-C", repoPath, "write-tree")
	if err != nil {
		return "", fmt.Errorf("write promotion tree: %w", err)
	}
	treeID := strings.TrimSpace(treeOutput)
	if err := validateCommitID(treeID); err != nil {
		return "", err
	}
	diffOutput, err := s.runLimitedText(ctx, maxPathListBytes, "-C", repoPath, "diff-tree", "--no-renames", "--name-status", "-r", "-z", input.ExpectedMainHead, treeID)
	if err != nil {
		return "", fmt.Errorf("audit promotion tree: %w", err)
	}
	changes, err := parseNameStatusZ(diffOutput, len(input.Paths))
	if err != nil {
		return "", fmt.Errorf("audit promotion tree: %w", err)
	}
	if err := validateExactPromotionPaths(changes, input.Paths); err != nil {
		return "", fmt.Errorf("audit promotion tree: %w", err)
	}
	active, detached, err := s.activeBranch(ctx, repoPath)
	if err != nil {
		return "", fmt.Errorf("verify promotion head: %w", err)
	}
	if detached || active != canonBranch {
		return "", errors.New("promotion did not leave main active")
	}
	command := []string{
		"-C", repoPath,
		"-c", "user.name=AI Story Workshop",
		"-c", "user.email=storywork@localhost",
		"commit-tree", treeID,
		"-p", input.ExpectedMainHead,
		"-m", message,
	}
	cmd := exec.CommandContext(ctx, s.executable, command...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %w: %s", strings.Join(cmd.Args, " "), err, strings.TrimSpace(string(output)))
	}
	head := strings.TrimSpace(string(output))
	if err := validateCommitID(head); err != nil {
		return "", err
	}
	if _, err := s.run(ctx, "-C", repoPath, "update-ref", "refs/heads/main", head, input.ExpectedMainHead); err != nil {
		return "", fmt.Errorf("publish promotion commit: %w", err)
	}
	return head, nil
}

func parseNameStatusZ(output string, maxCount int) ([]TreeChange, error) {
	if strings.TrimSpace(output) == "" {
		return []TreeChange{}, nil
	}
	records := strings.Split(strings.TrimSuffix(output, "\x00"), "\x00")
	var changes []TreeChange
	for i := 0; i < len(records); {
		if records[i] == "" {
			i++
			continue
		}
		var status byte
		var path string
		if len(records[i]) == 1 {
			status = records[i][0]
			i++
			if i >= len(records) {
				return nil, fmt.Errorf("unexpected name-status record %q", string(status))
			}
			path = records[i]
			i++
		} else if strings.Contains(records[i], "\t") {
			parts := strings.SplitN(records[i], "\t", 2)
			if len(parts[0]) != 1 {
				return nil, fmt.Errorf("unexpected name-status record %q", records[i])
			}
			status = parts[0][0]
			path = parts[1]
			i++
		} else {
			return nil, fmt.Errorf("unexpected name-status record %q", records[i])
		}
		switch status {
		case 'A', 'M', 'D':
		default:
			return nil, fmt.Errorf("unsupported status %q", string(status))
		}
		if err := validateProjectPath(path); err != nil {
			return nil, err
		}
		changes = append(changes, TreeChange{Path: path, Status: status})
		if maxCount > 0 && len(changes) > maxCount {
			return nil, ErrPathListTooLarge
		}
	}
	return changes, nil
}

func parsePathListZ(output string, maxCount int) ([]string, error) {
	if strings.TrimSpace(output) == "" {
		return []string{}, nil
	}
	records := strings.Split(strings.TrimSuffix(output, "\x00"), "\x00")
	paths := make([]string, 0, len(records))
	for _, record := range records {
		if record == "" {
			continue
		}
		if err := validateProjectPath(record); err != nil {
			return nil, err
		}
		paths = append(paths, record)
		if maxCount > 0 && len(paths) > maxCount {
			return nil, ErrPathListTooLarge
		}
	}
	return paths, nil
}

const maxPathListBytes = 512 << 10
const maxChangedPaths = 500

func cleanupContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
}

func (s *Store) runLimitedText(ctx context.Context, maxBytes int, arguments ...string) (string, error) {
	data, err := s.runBytesLimited(ctx, maxBytes, arguments...)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func validateExactPromotionPaths(changes []TreeChange, selected []string) error {
	if len(changes) != len(selected) {
		return errors.New("promotion tree touched unexpected paths")
	}
	sortedSelected := append([]string(nil), selected...)
	sort.Strings(sortedSelected)
	for index, change := range changes {
		if sortedSelected[index] != change.Path {
			return errors.New("promotion tree touched unexpected paths")
		}
		switch change.Status {
		case 'A', 'M', 'D':
		default:
			return fmt.Errorf("promotion tree used unsupported status %q", string(change.Status))
		}
	}
	return nil
}

func (s *Store) runBytes(ctx context.Context, arguments ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, s.executable, arguments...)
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", strings.Join(command.Args, " "), err)
	}
	return output, nil
}

// runBytesLimited executes a command, reading at most maxBytes+1 bytes from
// stdout. If more than maxBytes are produced, the child process is killed and
// reaped, and ErrDiffTooLarge is returned with no partial output.
func (s *Store) runBytesLimited(ctx context.Context, maxBytes int, arguments ...string) ([]byte, error) {
	if maxBytes <= 0 {
		return nil, ErrDiffTooLarge
	}
	command := exec.CommandContext(ctx, s.executable, arguments...)
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", strings.Join(command.Args, " "), err)
	}
	if err := command.Start(); err != nil {
		return nil, fmt.Errorf("%s: %w", strings.Join(command.Args, " "), err)
	}
	limited := io.LimitReader(stdout, int64(maxBytes)+1)
	data, readErr := io.ReadAll(limited)
	if readErr != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		return nil, fmt.Errorf("%s: %w", strings.Join(command.Args, " "), readErr)
	}
	if len(data) > maxBytes {
		_ = command.Process.Kill()
		_ = command.Wait()
		return nil, ErrDiffTooLarge
	}
	if err := command.Wait(); err != nil {
		return nil, fmt.Errorf("%s: %w", strings.Join(command.Args, " "), err)
	}
	return data, nil
}

// UnifiedDiff produces a real unified diff between two commits for the given
// validated paths, bounded by maxBytes. Rename detection and external diff
// drivers are disabled.
func (s *Store) UnifiedDiff(ctx context.Context, repoPath, leftCommit, rightCommit string, paths []string, maxBytes int) (string, error) {
	if err := validateCommitID(leftCommit); err != nil {
		return "", err
	}
	if err := validateCommitID(rightCommit); err != nil {
		return "", err
	}
	for _, projectPath := range paths {
		if err := validateProjectPath(projectPath); err != nil {
			return "", err
		}
	}
	args := []string{"-C", repoPath, "diff", "--no-renames", "--no-ext-diff", "--unified=3", "--text", leftCommit, rightCommit, "--"}
	args = append(args, paths...)
	output, err := s.runBytesLimited(ctx, maxBytes, args...)
	if err != nil {
		return "", fmt.Errorf("unified diff: %w", err)
	}
	return string(output), nil
}
