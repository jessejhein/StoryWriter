package importer

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestChunkMarkdownSplitsDeterministicallyAtHeadingAndBlankBoundaries(t *testing.T) {
	t.Parallel()

	text := "# One\nAlpha\n\n# Two\nBeta\n\n# Three\nGamma\n"
	chunks, err := ChunkMarkdown("imp_0123456789abcdef0123", "notes/example.md", text)
	if err != nil {
		t.Fatalf("ChunkMarkdown() error = %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("ChunkMarkdown() chunk count = %d, want 1", len(chunks))
	}
	if chunks[0].StartLine != 1 || chunks[0].EndLine != 8 {
		t.Fatalf("chunk lines = %d..%d", chunks[0].StartLine, chunks[0].EndLine)
	}
	if chunks[0].Text != text {
		t.Fatalf("chunk text = %q", chunks[0].Text)
	}
}

func TestChunkStoreRebuildsStructurallyValidButCorruptCache(t *testing.T) {
	t.Parallel()

	projectPath := t.TempDir()
	sourcePath := t.TempDir()
	writeTestFile(t, filepath.Join(sourcePath, "notes.md"), "# One\nAlpha\n")
	prepared, err := NewSourceStore().PrepareSnapshot(context.Background(), PrepareSnapshotRequest{
		ProjectPath: projectPath, SourceDirectory: sourcePath,
		ImportID: "imp_0123456789abcdef0123", CreatedAt: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := prepared.Publish(); err != nil {
		t.Fatal(err)
	}
	store := NewChunkStore()
	chunks, err := store.ListOrRebuild(context.Background(), projectPath, "imp_0123456789abcdef0123")
	if err != nil || len(chunks) != 1 {
		t.Fatalf("initial chunks = %+v, err = %v", chunks, err)
	}
	corrupt := append([]Chunk(nil), chunks...)
	corrupt[0].Text = "tampered"
	body, err := json.Marshal(corrupt)
	if err != nil {
		t.Fatal(err)
	}
	cachePath := filepath.Join(projectPath, ".storywork", "import", "imp_0123456789abcdef0123", "chunks.json")
	if err := os.WriteFile(cachePath, body, 0o644); err != nil {
		t.Fatal(err)
	}

	rebuilt, err := store.ListOrRebuild(context.Background(), projectPath, "imp_0123456789abcdef0123")
	if err != nil {
		t.Fatal(err)
	}
	if len(rebuilt) != 1 || rebuilt[0].Text != "# One\nAlpha\n" {
		t.Fatalf("rebuilt chunks = %+v", rebuilt)
	}
}

func TestChunkStoreRejectsCanonicalSnapshotDigestMismatch(t *testing.T) {
	t.Parallel()

	projectPath := t.TempDir()
	sourcePath := t.TempDir()
	writeTestFile(t, filepath.Join(sourcePath, "notes.md"), "Alpha\n")
	prepared, err := NewSourceStore().PrepareSnapshot(context.Background(), PrepareSnapshotRequest{
		ProjectPath: projectPath, SourceDirectory: sourcePath,
		ImportID: "imp_0123456789abcdef0123", CreatedAt: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := prepared.Publish(); err != nil {
		t.Fatal(err)
	}
	importedPath := filepath.Join(projectPath, "imports", "raw", "imp_0123456789abcdef0123", "files", "notes.md")
	if err := os.WriteFile(importedPath, []byte("Changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err = NewChunkStore().ListOrRebuild(context.Background(), projectPath, "imp_0123456789abcdef0123")
	if !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("ListOrRebuild() error = %v, want %v", err, ErrInvalidManifest)
	}
}

func TestChunkStoreRejectsManifestWithTrailingYAMLDocument(t *testing.T) {
	t.Parallel()

	projectPath := t.TempDir()
	sourcePath := t.TempDir()
	writeTestFile(t, filepath.Join(sourcePath, "notes.md"), "Alpha\n")
	prepared, err := NewSourceStore().PrepareSnapshot(context.Background(), PrepareSnapshotRequest{
		ProjectPath: projectPath, SourceDirectory: sourcePath,
		ImportID: "imp_0123456789abcdef0123", CreatedAt: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := prepared.Publish(); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(projectPath, "imports", "raw", "imp_0123456789abcdef0123", "manifest.yaml")
	body, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, append(body, []byte("---\nversion: 1\n")...), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := NewChunkStore().ListOrRebuild(context.Background(), projectPath, "imp_0123456789abcdef0123"); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("ListOrRebuild() error = %v, want %v", err, ErrInvalidManifest)
	}
}

func TestChunkMarkdownHandlesOversizedLinesWithoutSplittingRunesOrLines(t *testing.T) {
	t.Parallel()

	longLine := slices.Repeat([]byte("a"), maxChunkBytes+100)
	text := string(longLine) + "\n# Next\n"
	chunks, err := ChunkMarkdown("imp_0123456789abcdef0123", "notes/long.md", text)
	if err != nil {
		t.Fatalf("ChunkMarkdown() error = %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("ChunkMarkdown() chunk count = %d, want 2", len(chunks))
	}
	if chunks[0].StartLine != 1 || chunks[0].EndLine != 1 {
		t.Fatalf("oversized line chunk lines = %d..%d", chunks[0].StartLine, chunks[0].EndLine)
	}
	if chunks[1].StartLine != 2 || chunks[1].EndLine != 2 {
		t.Fatalf("second chunk lines = %d..%d", chunks[1].StartLine, chunks[1].EndLine)
	}
}
