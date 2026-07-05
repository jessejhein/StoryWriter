package gitstore

import "errors"

// ErrDirtyWorktree is returned when a clean worktree is required but the
// repository has uncommitted or untracked changes.
var ErrDirtyWorktree = errors.New("worktree is not clean")

// ErrStaleExperimentHead is returned when the expected experiment head does
// not match the current ref at deletion time.
var ErrStaleExperimentHead = errors.New("stale experiment head")

// ErrDiffTooLarge is returned when a unified diff output exceeds the supplied
// byte budget.
var ErrDiffTooLarge = errors.New("diff exceeds byte budget")

// ErrBlobTooLarge is returned when a blob exceeds the supplied display budget.
var ErrBlobTooLarge = errors.New("blob exceeds byte budget")

// ErrPathListTooLarge is returned when machine-safe path/status output exceeds
// Storywork's changed-path limit.
var ErrPathListTooLarge = errors.New("changed path list exceeds limit")
