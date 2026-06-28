// Package gitstore adapts the Git command-line client for story project history.
package gitstore

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
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

// CommitAll stages all non-ignored files and records a commit.
func (s *Store) CommitAll(ctx context.Context, path, message string) error {
	if _, err := s.run(ctx, "-C", path, "add", "--all"); err != nil {
		return fmt.Errorf("stage project files: %w", err)
	}
	if _, err := s.run(ctx,
		"-C", path,
		"-c", "user.name=AI Story Workshop",
		"-c", "user.email=storywork@localhost",
		"commit", "-m", message,
	); err != nil {
		return fmt.Errorf("commit project files: %w", err)
	}
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

func (s *Store) run(ctx context.Context, arguments ...string) (string, error) {
	command := exec.CommandContext(ctx, s.executable, arguments...)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %w: %s", strings.Join(command.Args, " "), err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}
