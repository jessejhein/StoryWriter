package importer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSourceStorePrepareSnapshotCopiesEligibleMarkdownFiles(t *testing.T) {
	t.Parallel()

	projectPath := t.TempDir()
	sourcePath := t.TempDir()
	writeTestFile(t, filepath.Join(sourcePath, "notes", "characters.md"), "# Characters\r\nMara\r")
	writeTestFile(t, filepath.Join(sourcePath, "notes", "ignore.txt"), "skip")
	writeTestFile(t, filepath.Join(sourcePath, ".hidden", "skip.md"), "skip")

	store := NewSourceStore()
	prepared, err := store.PrepareSnapshot(context.Background(), PrepareSnapshotRequest{
		ProjectPath:     projectPath,
		SourceDirectory: sourcePath,
		ImportID:        "imp_0123456789abcdef0123",
		CreatedAt:       time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("PrepareSnapshot() error = %v", err)
	}
	if len(prepared.Files()) != 1 {
		t.Fatalf("PrepareSnapshot() copied %d files, want 1", len(prepared.Files()))
	}
	rollback, err := prepared.Publish()
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	t.Cleanup(func() { _ = rollback() })

	importRoot := filepath.Join(projectPath, "imports", "raw", "imp_0123456789abcdef0123")
	markdownBytes, err := os.ReadFile(filepath.Join(importRoot, "files", "notes", "characters.md"))
	if err != nil {
		t.Fatalf("ReadFile(imported markdown) error = %v", err)
	}
	if string(markdownBytes) != "# Characters\nMara\n" {
		t.Fatalf("normalized markdown = %q", string(markdownBytes))
	}
	manifestBytes, err := os.ReadFile(filepath.Join(importRoot, "manifest.yaml"))
	if err != nil {
		t.Fatalf("ReadFile(manifest) error = %v", err)
	}
	if strings.Contains(string(manifestBytes), sourcePath) {
		t.Fatalf("manifest leaked source path: %s", manifestBytes)
	}
}

func TestSourceStorePrepareSnapshotRejectsInvalidSourceDirectory(t *testing.T) {
	t.Parallel()

	projectPath := t.TempDir()
	sourcePath := filepath.Join(projectPath, "notes")
	if err := os.MkdirAll(sourcePath, 0o755); err != nil {
		t.Fatalf("MkdirAll(sourcePath) error = %v", err)
	}

	store := NewSourceStore()
	_, err := store.PrepareSnapshot(context.Background(), PrepareSnapshotRequest{
		ProjectPath:     projectPath,
		SourceDirectory: sourcePath,
		ImportID:        "imp_0123456789abcdef0123",
		CreatedAt:       time.Now(),
	})
	if !errors.Is(err, ErrInvalidSourceDirectory) {
		t.Fatalf("PrepareSnapshot() error = %v, want %v", err, ErrInvalidSourceDirectory)
	}
}

func TestSourceStorePrepareSnapshotRejectsSymlinkAndInvalidUTF8(t *testing.T) {
	t.Parallel()

	projectPath := t.TempDir()
	sourcePath := t.TempDir()
	writeTestFile(t, filepath.Join(sourcePath, "notes.md"), "Alpha")
	symlinkPath := filepath.Join(sourcePath, "link.md")
	if err := os.Symlink(filepath.Join(sourcePath, "notes.md"), symlinkPath); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	store := NewSourceStore()
	_, err := store.PrepareSnapshot(context.Background(), PrepareSnapshotRequest{
		ProjectPath:     projectPath,
		SourceDirectory: sourcePath,
		ImportID:        "imp_0123456789abcdef0123",
		CreatedAt:       time.Now(),
	})
	if !errors.Is(err, ErrSymlinkRefused) {
		t.Fatalf("PrepareSnapshot() symlink error = %v, want %v", err, ErrSymlinkRefused)
	}

	if err := os.Remove(symlinkPath); err != nil {
		t.Fatalf("Remove(symlink) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourcePath, "broken.md"), []byte{0xff, 0xfe, 0xfd}, 0o644); err != nil {
		t.Fatalf("WriteFile(broken.md) error = %v", err)
	}
	_, err = store.PrepareSnapshot(context.Background(), PrepareSnapshotRequest{
		ProjectPath:     projectPath,
		SourceDirectory: sourcePath,
		ImportID:        "imp_0123456789abcdef0123",
		CreatedAt:       time.Now(),
	})
	if !errors.Is(err, ErrInvalidContent) {
		t.Fatalf("PrepareSnapshot() invalid UTF-8 error = %v, want %v", err, ErrInvalidContent)
	}
}

func TestPreparedSnapshotPublishNeverOverwritesExistingImport(t *testing.T) {
	t.Parallel()

	projectPath := t.TempDir()
	sourcePath := t.TempDir()
	writeTestFile(t, filepath.Join(sourcePath, "notes.md"), "Alpha")
	prepared, err := NewSourceStore().PrepareSnapshot(context.Background(), PrepareSnapshotRequest{
		ProjectPath: projectPath, SourceDirectory: sourcePath,
		ImportID: "imp_0123456789abcdef0123", CreatedAt: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	finalPath := filepath.Join(projectPath, "imports", "raw", "imp_0123456789abcdef0123")
	if err := os.MkdirAll(finalPath, 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(finalPath, "sentinel")
	if err := os.WriteFile(sentinel, []byte("preserve"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := prepared.Publish(); err == nil {
		t.Fatal("Publish() error = nil")
	}
	body, err := os.ReadFile(sentinel)
	if err != nil || string(body) != "preserve" {
		t.Fatalf("existing import was changed: body=%q err=%v", body, err)
	}
}

func writeTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
