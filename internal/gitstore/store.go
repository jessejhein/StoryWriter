package gitstore

// store.go implements the Git adapter used for project integrity checks and checkpoints.

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"bytes"
)

// Store runs Git operations without changing user-global configuration.
type Store struct {
	executable string
}

// New creates a Git store. executable is normally "git".
func New(executable string) *Store {
	return &Store{executable: executable}
}

// Init creates a repository whose initial branch is main.
func (s *Store) Init(ctx context.Context, path string) error {
	if _, err := s.run(ctx, "-C", path, "init", "-b", "main"); err != nil {
		return fmt.Errorf("initialize Git repository: %w", err)
	}
	return nil
}

// CommitAll stages all non-ignored files and records a subject-only commit.
func (s *Store) CommitAll(ctx context.Context, path, message string) error {
	formatted, err := FormatCommitMessage(CommitMessage{Subject: message})
	if err != nil {
		return fmt.Errorf("format commit message: %w", err)
	}
	return s.commitFormatted(ctx, path, formatted)
}

// CommitAllMessage stages all non-ignored files and records one validated commit body.
func (s *Store) CommitAllMessage(ctx context.Context, path string, message CommitMessage) error {
	formatted, err := FormatCommitMessage(message)
	if err != nil {
		return fmt.Errorf("format commit message: %w", err)
	}
	return s.commitFormatted(ctx, path, formatted)
}

func (s *Store) commitFormatted(ctx context.Context, path, message string) error {
	if _, err := s.run(ctx, "-C", path, "add", "--all"); err != nil {
		return fmt.Errorf("stage project files: %w", err)
	}
	command := exec.CommandContext(ctx, s.executable,
		"-C", path,
		"-c", "user.name=AI Story Workshop",
		"-c", "user.email=storywork@localhost",
		"commit", "-F", "-",
	)
	command.Stdin = strings.NewReader(message)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w: %s", strings.Join(command.Args, " "), err, strings.TrimSpace(string(output)))
	}
	_ = bytes.TrimSpace(output)
	return nil
}

// IsRepo reports whether path is the root of a working-tree repository.
func (s *Store) IsRepo(ctx context.Context, path string) (bool, error) {
	output, err := s.run(ctx, "-C", path, "rev-parse", "--show-toplevel")
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return false, nil
		}
		return false, fmt.Errorf("inspect Git repository: %w", err)
	}
	return strings.TrimSpace(output) == path, nil
}

// IsClean reports whether the working tree has no tracked or staged changes.
func (s *Store) IsClean(ctx context.Context, path string) (bool, error) {
	output, err := s.run(ctx, "-C", path, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("inspect Git worktree: %w", err)
	}
	return strings.TrimSpace(output) == "", nil
}

// UnstageAll removes staged changes without discarding the working tree.
func (s *Store) UnstageAll(ctx context.Context, path string) error {
	if _, err := s.run(ctx, "-C", path, "reset", "--mixed", "HEAD"); err != nil {
		return fmt.Errorf("unstage project files: %w", err)
	}
	return nil
}

func (s *Store) run(ctx context.Context, arguments ...string) (string, error) {
	command := exec.CommandContext(ctx, s.executable, arguments...)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %w: %s", strings.Join(command.Args, " "), err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}
