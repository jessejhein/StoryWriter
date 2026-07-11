package branch

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"unicode/utf8"
)

var allowedExactPaths = map[string]struct{}{
	"outline.yaml": {},
}

var allowedPrefixes = []struct {
	prefix      string
	extension   string
	allowNested bool
}{
	{"arcs/", ".yaml", false},
	{"chapters/", ".yaml", false},
	{"scenes/", ".md", false},
	{"codex/characters/", ".yaml", false},
	{"codex/locations/", ".yaml", false},
	{"codex/lore/", ".yaml", false},
	{"codex/custom/", ".yaml", false},
	{"progressions/", ".yaml", false},
	{"agents/", ".yaml", false},
	{"styles/", ".yaml", false},
	{"imports/review/", ".yaml", false},
}

// ValidateProjectPath validates one project-relative canonical text path.
func ValidateProjectPath(value string) (ProjectPath, error) {
	if value == "" {
		return "", fmt.Errorf("path is empty: %w", ErrInvalidProjectPath)
	}
	if strings.Contains(value, "\\") {
		return "", fmt.Errorf("path %q contains backslash: %w", value, ErrInvalidProjectPath)
	}
	if strings.ContainsRune(value, 0) {
		return "", fmt.Errorf("path %q contains NUL: %w", value, ErrInvalidProjectPath)
	}
	if !utf8.ValidString(value) {
		return "", fmt.Errorf("path %q is not valid UTF-8: %w", value, ErrInvalidProjectPath)
	}
	if path.IsAbs(value) {
		return "", fmt.Errorf("path %q is absolute: %w", value, ErrInvalidProjectPath)
	}
	cleaned := path.Clean(value)
	if cleaned != value {
		return "", fmt.Errorf("path %q is not canonical: %w", value, ErrInvalidProjectPath)
	}
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("path %q traverses: %w", value, ErrInvalidProjectPath)
	}
	for _, segment := range strings.Split(cleaned, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return "", fmt.Errorf("path %q has invalid segment: %w", value, ErrInvalidProjectPath)
		}
		if strings.HasPrefix(segment, ".") {
			return "", fmt.Errorf("path %q has hidden segment: %w", value, ErrInvalidProjectPath)
		}
		if containsControl(segment) {
			return "", fmt.Errorf("path %q contains control characters: %w", value, ErrInvalidProjectPath)
		}
	}
	if strings.HasPrefix(cleaned, ".storywork/") || cleaned == "project.yaml" || cleaned == ".gitignore" {
		return "", fmt.Errorf("path %q is excluded: %w", value, ErrInvalidProjectPath)
	}
	if strings.HasSuffix(cleaned, ".gitkeep") {
		return "", fmt.Errorf("path %q is excluded: %w", value, ErrInvalidProjectPath)
	}
	if !isAllowedProjectPath(cleaned) {
		return "", fmt.Errorf("path %q is not allowed: %w", value, ErrInvalidProjectPath)
	}
	return ProjectPath(cleaned), nil
}

func isAllowedProjectPath(value string) bool {
	if _, ok := allowedExactPaths[value]; ok {
		return true
	}
	if isAllowedRawImportPath(value) {
		return true
	}
	for _, rule := range allowedPrefixes {
		if !strings.HasPrefix(value, rule.prefix) || !strings.HasSuffix(value, rule.extension) {
			continue
		}
		remainder := strings.TrimSuffix(strings.TrimPrefix(value, rule.prefix), rule.extension)
		if remainder == "" {
			return false
		}
		if !rule.allowNested && strings.Contains(remainder, "/") {
			return false
		}
		return true
	}
	return false
}

func isAllowedRawImportPath(value string) bool {
	const (
		rawImportPrefix = "imports/raw/"
		rawFilesPrefix  = "files/"
		rawManifestName = "manifest.yaml"
	)
	if !strings.HasPrefix(value, rawImportPrefix) {
		return false
	}
	remainder := strings.TrimPrefix(value, rawImportPrefix)
	importID, afterID, ok := strings.Cut(remainder, "/")
	if !ok || !isRawImportID(importID) {
		return false
	}
	if afterID == rawManifestName {
		return true
	}
	filesPath, ok := strings.CutPrefix(afterID, rawFilesPrefix)
	if !ok || filesPath == "" {
		return false
	}
	// Raw import snapshot Markdown extensions mirror the Milestone 6 source
	// discovery policy while preserving imported relative path casing.
	lower := strings.ToLower(filesPath)
	return strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".markdown")
}

func isRawImportID(value string) bool {
	if len(value) != len("imp_0123456789abcdef0123") || !strings.HasPrefix(value, "imp_") {
		return false
	}
	for _, r := range strings.TrimPrefix(value, "imp_") {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

func containsControl(value string) bool {
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

// SortProjectPaths returns a byte-sorted copy of paths.
func SortProjectPaths(paths []ProjectPath) []ProjectPath {
	sorted := append([]ProjectPath(nil), paths...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})
	return sorted
}

// SortChangedFiles returns paths sorted by byte order.
func SortChangedFiles(files []ChangedFile) []ChangedFile {
	sorted := append([]ChangedFile(nil), files...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Path < sorted[j].Path
	})
	return sorted
}

// ValidateStrictUTF8 requires strict UTF-8 without NUL bytes.
func ValidateStrictUTF8(data []byte) (string, error) {
	if len(data) > MaxFileBytes {
		return "", fmt.Errorf("content exceeds %d bytes: %w", MaxFileBytes, ErrFileTooLarge)
	}
	if !utf8.Valid(data) {
		return "", fmt.Errorf("content is not valid UTF-8: %w", ErrInvalidUTF8)
	}
	text := string(data)
	if strings.ContainsRune(text, 0) {
		return "", fmt.Errorf("content contains NUL: %w", ErrInvalidUTF8)
	}
	lines := strings.Count(text, "\n")
	if len(text) > 0 && !strings.HasSuffix(text, "\n") {
		lines++
	}
	if lines > MaxFileLines {
		return "", fmt.Errorf("content exceeds %d lines: %w", MaxFileLines, ErrFileTooLarge)
	}
	return text, nil
}
