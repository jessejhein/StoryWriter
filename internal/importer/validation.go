package importer

// validation.go provides read-only validation for tracked import snapshots.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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
		filesRoot := filepath.Join(rawRoot, entry.Name(), "files")
		manifestPaths := make([]string, 0, len(manifest.Files))
		manifestPathSet := make(map[string]struct{}, len(manifest.Files))
		for _, file := range manifest.Files {
			manifestPaths = append(manifestPaths, file.Path)
			manifestPathSet[file.Path] = struct{}{}
			path := filepath.Join(filesRoot, filepath.FromSlash(file.Path))
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
		if err := validateSnapshotTree(filesRoot, manifestPaths, manifestPathSet); err != nil {
			return err
		}
	}
	return nil
}

func validateSnapshotTree(filesRoot string, manifestPaths []string, manifestPathSet map[string]struct{}) error {
	actualPaths := make([]string, 0, len(manifestPaths))
	err := filepath.WalkDir(filesRoot, func(current string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk imported snapshot: %w", walkErr)
		}
		if current == filesRoot {
			return nil
		}
		relative, err := filepath.Rel(filesRoot, current)
		if err != nil {
			return fmt.Errorf("rel imported snapshot: %w", err)
		}
		normalized, err := NormalizePortableRelativePath(filepath.ToSlash(relative))
		if err != nil {
			return fmt.Errorf("unexpected imported snapshot path %q: %w", relative, ErrInvalidManifest)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("unexpected imported snapshot path %q: %w", normalized, ErrInvalidManifest)
		}
		if entry.IsDir() {
			prefix := normalized + "/"
			for _, manifestPath := range manifestPaths {
				if strings.HasPrefix(manifestPath, prefix) {
					return nil
				}
			}
			return fmt.Errorf("unexpected imported snapshot path %q: %w", normalized, ErrInvalidManifest)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("unexpected imported snapshot path %q: %w", normalized, ErrInvalidManifest)
		}
		actualPaths = append(actualPaths, normalized)
		if _, ok := manifestPathSet[normalized]; !ok {
			return fmt.Errorf("unexpected imported snapshot path %q: %w", normalized, ErrInvalidManifest)
		}
		return nil
	})
	if err != nil {
		return err
	}
	slices.Sort(actualPaths)
	expectedPaths := slices.Clone(manifestPaths)
	slices.Sort(expectedPaths)
	if !slices.Equal(actualPaths, expectedPaths) {
		return fmt.Errorf("imported snapshot file set does not match manifest: %w", ErrInvalidManifest)
	}
	return nil
}
