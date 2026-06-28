package index_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"storywork/internal/index"
)

func TestRebuildIsIdempotentAndIndexesCanonicalFiles(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "project.yaml"), []byte("name: Test Novel\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := index.New()
	if err := store.Rebuild(ctx, dir); err != nil {
		t.Fatalf("first Rebuild() error = %v", err)
	}
	if err := store.Rebuild(ctx, dir); err != nil {
		t.Fatalf("second Rebuild() error = %v", err)
	}
	if err := store.Verify(ctx, dir); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}

	database, err := sql.Open("sqlite", filepath.Join(dir, ".storywork", "index.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	var count int
	if err := database.QueryRowContext(ctx, "SELECT COUNT(*) FROM files WHERE path = 'project.yaml'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("manifest count = %d, want 1", count)
	}
}
