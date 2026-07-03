// BDD Scenario: 8.2.2 - Show side-by-side text
// Requirements: M8-R05, M8-R07
// Test purpose: Blob reads work at explicit commits without checkout.

package gitstore_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
