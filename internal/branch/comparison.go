package branch

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
)

const fingerprintVersion = byte(1)

// ComputeFingerprint returns sha256: over the versioned comparison stream.
func ComputeFingerprint(mainHead, experimentHead, baseHead CommitID, files []ChangedFile) (string, error) {
	if _, err := ValidateCommitID(string(mainHead)); err != nil {
		return "", err
	}
	if _, err := ValidateCommitID(string(experimentHead)); err != nil {
		return "", err
	}
	if _, err := ValidateCommitID(string(baseHead)); err != nil {
		return "", err
	}
	sorted := SortChangedFiles(files)
	if len(sorted) > MaxChangedPaths {
		return "", fmt.Errorf("%d changed paths: %w", len(sorted), ErrTooManyChangedPaths)
	}
	hasher := sha256.New()
	hasher.Write([]byte{fingerprintVersion})
	hasher.Write([]byte(mainHead))
	hasher.Write([]byte{0})
	hasher.Write([]byte(experimentHead))
	hasher.Write([]byte{0})
	hasher.Write([]byte(baseHead))
	hasher.Write([]byte{0})
	for _, file := range sorted {
		hasher.Write([]byte(file.Status))
		hasher.Write([]byte{0})
		hasher.Write([]byte(file.Path))
		hasher.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}

// ValidateFingerprintMatch compares expected and actual fingerprints.
func ValidateFingerprintMatch(expected, actual string) error {
	if err := ValidateFingerprint(expected); err != nil {
		return err
	}
	if expected != actual {
		return fmt.Errorf("expected %q got %q: %w", expected, actual, ErrStaleFingerprint)
	}
	return nil
}

// IndexChangedFiles builds a path lookup from changed files.
func IndexChangedFiles(files []ChangedFile) map[ProjectPath]ChangedFile {
	index := make(map[ProjectPath]ChangedFile, len(files))
	for _, file := range files {
		index[file.Path] = file
	}
	return index
}

// ValidateChangedFiles enforces count, sorting, and status/path validity.
func ValidateChangedFiles(files []ChangedFile) ([]ChangedFile, error) {
	if len(files) > MaxChangedPaths {
		return nil, fmt.Errorf("%d changed paths: %w", len(files), ErrTooManyChangedPaths)
	}
	normalized := make([]ChangedFile, 0, len(files))
	for _, file := range files {
		path, err := ValidateProjectPath(string(file.Path))
		if err != nil {
			return nil, err
		}
		switch file.Status {
		case StatusAdded, StatusModified, StatusDeleted:
		default:
			return nil, fmt.Errorf("status %q for %q: %w", file.Status, path, ErrInvalidChangedStatus)
		}
		normalized = append(normalized, ChangedFile{Path: path, Status: file.Status})
	}
	sorted := SortChangedFiles(normalized)
	for i := 1; i < len(sorted); i++ {
		if sorted[i].Path == sorted[i-1].Path {
			return nil, fmt.Errorf("duplicate path %q: %w", sorted[i].Path, ErrInvalidChangedStatus)
		}
	}
	return sorted, nil
}

// IntersectChangedPaths returns sorted paths present in both sets.
func IntersectChangedPaths(selected []ProjectPath, changed []ProjectPath) []ProjectPath {
	changedSet := make(map[ProjectPath]struct{}, len(changed))
	for _, path := range changed {
		changedSet[path] = struct{}{}
	}
	var intersection []ProjectPath
	for _, path := range selected {
		if _, ok := changedSet[path]; ok {
			intersection = append(intersection, path)
		}
	}
	return SortProjectPaths(intersection)
}

// ChangedPathSet converts a path list to a set.
func ChangedPathSet(paths []ProjectPath) map[ProjectPath]struct{} {
	set := make(map[ProjectPath]struct{}, len(paths))
	for _, path := range paths {
		set[path] = struct{}{}
	}
	return set
}

// ValidateSelectedPaths requires a unique sorted non-empty promotable selection.
func ValidateSelectedPaths(paths []string) ([]ProjectPath, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("paths are required: %w", ErrInvalidPromotion)
	}
	seen := make(map[ProjectPath]struct{}, len(paths))
	selected := make([]ProjectPath, 0, len(paths))
	for _, raw := range paths {
		path, err := ValidateProjectPath(raw)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[path]; ok {
			return nil, fmt.Errorf("duplicate path %q: %w", path, ErrInvalidPromotion)
		}
		seen[path] = struct{}{}
		selected = append(selected, path)
	}
	return SortProjectPaths(selected), nil
}

// PromotionConflicts returns sorted selected paths changed on main since base.
func PromotionConflicts(selected, mainChanged []ProjectPath) []ProjectPath {
	mainSet := ChangedPathSet(mainChanged)
	var conflicts []ProjectPath
	for _, path := range selected {
		if _, ok := mainSet[path]; ok {
			conflicts = append(conflicts, path)
		}
	}
	sort.Slice(conflicts, func(i, j int) bool { return conflicts[i] < conflicts[j] })
	return conflicts
}
