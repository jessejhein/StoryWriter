// Package index maintains the disposable SQLite index for a story project.
package index

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE schema_version (version INTEGER NOT NULL);
INSERT INTO schema_version (version) VALUES (1);
CREATE TABLE project_manifest (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE files (
    path TEXT PRIMARY KEY,
    kind TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    updated_at TEXT NOT NULL
);`

// Store manages the derived project index.
type Store struct{}

// New creates an index store.
func New() *Store { return &Store{} }

// Init creates a fresh index from canonical project files.
func (s *Store) Init(ctx context.Context, projectPath string) error {
	return s.Rebuild(ctx, projectPath)
}

// Rebuild atomically replaces the index from canonical project files.
func (s *Store) Rebuild(ctx context.Context, projectPath string) error {
	storyworkPath := filepath.Join(projectPath, ".storywork")
	if err := os.MkdirAll(filepath.Join(storyworkPath, "tmp"), 0o755); err != nil {
		return fmt.Errorf("create index directory: %w", err)
	}

	temporaryPath := filepath.Join(storyworkPath, "tmp", "index.sqlite.rebuild")
	if err := os.Remove(temporaryPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale temporary index: %w", err)
	}
	if err := s.build(ctx, projectPath, temporaryPath); err != nil {
		_ = os.Remove(temporaryPath)
		return err
	}
	if err := os.Rename(temporaryPath, filepath.Join(storyworkPath, "index.sqlite")); err != nil {
		return fmt.Errorf("replace project index: %w", err)
	}
	return nil
}

// Verify checks that the index exists, is readable, and has the expected schema.
func (s *Store) Verify(ctx context.Context, projectPath string) error {
	databasePath := filepath.Join(projectPath, ".storywork", "index.sqlite")
	if _, err := os.Stat(databasePath); err != nil {
		return fmt.Errorf("stat project index: %w", err)
	}
	database, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return fmt.Errorf("open project index: %w", err)
	}
	defer database.Close()

	var version int
	if err := database.QueryRowContext(ctx, "SELECT version FROM schema_version LIMIT 1").Scan(&version); err != nil {
		return fmt.Errorf("read index schema version: %w", err)
	}
	if version != 1 {
		return fmt.Errorf("unsupported index schema version %d", version)
	}
	return nil
}

func (s *Store) build(ctx context.Context, projectPath, databasePath string) error {
	database, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return fmt.Errorf("open new project index: %w", err)
	}
	defer database.Close()

	if _, err := database.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("create project index schema: %w", err)
	}
	if _, err := database.ExecContext(ctx,
		"INSERT INTO project_manifest (key, value) VALUES ('schema_version', '1')",
	); err != nil {
		return fmt.Errorf("write project manifest: %w", err)
	}

	return filepath.WalkDir(projectPath, func(path string, entry fs.DirEntry, walkError error) error {
		if walkError != nil {
			return walkError
		}
		relativePath, err := filepath.Rel(projectPath, path)
		if err != nil {
			return err
		}
		if entry.IsDir() && (relativePath == ".git" || relativePath == ".storywork") {
			return filepath.SkipDir
		}
		if entry.IsDir() || !isCanonical(relativePath) {
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read canonical file %q: %w", relativePath, err)
		}
		digest := sha256.Sum256(contents)
		_, err = database.ExecContext(ctx,
			"INSERT INTO files (path, kind, content_hash, updated_at) VALUES (?, ?, ?, ?)",
			filepath.ToSlash(relativePath), fileKind(relativePath), hex.EncodeToString(digest[:]), time.Now().UTC().Format(time.RFC3339Nano),
		)
		return err
	})
}

func isCanonical(path string) bool {
	if filepath.Base(path) == ".gitkeep" {
		return false
	}
	if path == ".gitignore" {
		return true
	}
	extension := strings.ToLower(filepath.Ext(path))
	return extension == ".yaml" || extension == ".yml" || extension == ".md" || extension == ".jsonl"
}

func fileKind(path string) string {
	if path == "project.yaml" {
		return "project"
	}
	if path == "outline.yaml" {
		return "outline"
	}
	parent := strings.Split(filepath.ToSlash(path), "/")[0]
	if parent == ".gitignore" {
		return "configuration"
	}
	return parent
}
