package importer

// validation.go provides read-only validation for tracked import snapshots.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// ValidateStoredSnapshots validates every tracked raw import manifest and file
// without rebuilding or writing derived chunk data.
func ValidateStoredSnapshots(ctx context.Context, projectPath string) error {
	rawRoot := filepath.Join(projectPath, "imports", "raw")
	entries, err := os.ReadDir(rawRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read raw imports: %w", err)
	}
	for _, entry := range entries {
		if entry.Name() == ".gitkeep" {
			continue
		}
		if !entry.IsDir() || ValidateImportID(entry.Name()) != nil {
			return fmt.Errorf("invalid raw import entry %q: %w", entry.Name(), ErrInvalidManifest)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		manifest, err := loadManifest(filepath.Join(rawRoot, entry.Name(), "manifest.yaml"))
		if err != nil {
			return err
		}
		if manifest.ID != entry.Name() {
			return fmt.Errorf("manifest identifier does not match directory: %w", ErrInvalidManifest)
		}
		for _, file := range manifest.Files {
			path := filepath.Join(rawRoot, entry.Name(), "files", filepath.FromSlash(file.Path))
			info, err := os.Lstat(path)
			if err != nil || !info.Mode().IsRegular() {
				return fmt.Errorf("invalid imported markdown %q: %w", file.Path, ErrInvalidManifest)
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read imported markdown %q: %w", file.Path, err)
			}
			if int64(len(body)) != file.Bytes || CanonicalSHA256(body) != file.SHA256 || !utf8.Valid(body) || strings.ContainsRune(string(body), 0) {
				return fmt.Errorf("imported markdown %q does not match its manifest: %w", file.Path, ErrInvalidManifest)
			}
		}
	}
	return nil
}
