// BDD Scenario: 8.2.2 - Show side-by-side text
// Requirements: M8-R05, M8-R07
// Test purpose: Blob reads work at explicit commits without checkout.

package gitstore_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"storywork/internal/gitstore"
)

// Test: existing and missing blobs at explicit commits.
// Requirements: M8-R05.
func TestReadTextBlobWithoutCheckout(t *testing.T) {
	t.Parallel()
	ctx, dir, store := initTestRepo(t)
	mainHead, err := store.ResolveCommit(ctx, dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	blob, err := store.ReadTextBlob(ctx, dir, mainHead, "outline.yaml")
	if err != nil {
		t.Fatalf("ReadTextBlob() error = %v", err)
	}
	if !blob.Exists || !strings.Contains(string(blob.Bytes), "version:") {
		t.Fatalf("blob = %#v", blob)
	}
	missing, err := store.ReadTextBlob(ctx, dir, mainHead, "scenes/scn_00000000000000000001.md")
	if err != nil {
		t.Fatalf("ReadTextBlob(missing) error = %v", err)
	}
	if missing.Exists {
		t.Fatalf("missing.Exists = true, want false")
	}
}

// Test: symlink entries and repository failures are errors, not missing blobs.
// Requirements: M8-R06, M8-R07.
func TestReadTextBlobRejectsNonRegularEntriesAndDoesNotHideGitErrors(t *testing.T) {
	t.Parallel()
	ctx, dir, store := initTestRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, "scenes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target.md", filepath.Join(dir, "scenes", "scn_0123456789abcdef0123.md")); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"-C", dir, "add", "scenes/scn_0123456789abcdef0123.md"}, {"-C", dir, "-c", "user.name=test", "-c", "user.email=test@example.test", "commit", "-m", "add symlink"}} {
		if output, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	head, err := store.ResolveCommit(ctx, dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadTextBlob(ctx, dir, head, "scenes/scn_0123456789abcdef0123.md"); err == nil {
		t.Fatal("ReadTextBlob(symlink) error = nil")
	}
	if _, err := store.ReadTextBlob(context.Background(), filepath.Join(dir, "missing"), head, "outline.yaml"); err == nil {
		t.Fatal("ReadTextBlob(invalid repository) error = nil")
	}
}

// Test: blob reads enforce the 5 MiB boundary before returning content and do
// not return partial oversized data.
// Requirements: M8-R07.
func TestReadTextBlobEnforcesByteBudget(t *testing.T) {
	t.Parallel()
	ctx, dir, store := initTestRepo(t)
	withinLimit := strings.Repeat("a", 5<<20)
	if err := os.WriteFile(filepath.Join(dir, "outline.yaml"), []byte(withinLimit), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.CommitAll(ctx, dir, "within limit"); err != nil {
		t.Fatal(err)
	}
	head, err := store.ResolveCommit(ctx, dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	blob, err := store.ReadTextBlob(ctx, dir, head, "outline.yaml")
	if err != nil {
		t.Fatalf("ReadTextBlob(within) error = %v", err)
	}
	if len(blob.Bytes) != len(withinLimit) {
		t.Fatalf("len(blob.Bytes) = %d, want %d", len(blob.Bytes), len(withinLimit))
	}

	overLimit := strings.Repeat("b", (5<<20)+1)
	if err := os.WriteFile(filepath.Join(dir, "outline.yaml"), []byte(overLimit), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.CommitAll(ctx, dir, "over limit"); err != nil {
		t.Fatal(err)
	}
	head, err = store.ResolveCommit(ctx, dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.ReadTextBlob(ctx, dir, head, "outline.yaml")
	if !errors.Is(err, gitstore.ErrBlobTooLarge) {
		t.Fatalf("ReadTextBlob(over) error = %v", err)
	}
}
