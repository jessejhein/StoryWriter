package importer

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

const (
	ManifestVersion     = 1
	maxImportFiles      = 500
	maxImportFileBytes  = 5 << 20
	maxImportBatchBytes = 50 << 20
)

var (
	ErrInvalidID           = errors.New("invalid import identifier")
	ErrInvalidManifest     = errors.New("invalid import manifest")
	ErrInvalidPath         = errors.New("invalid portable import path")
	ErrCaseFoldedCollision = errors.New("portable path case-folded collision")

	importIDPattern    = regexp.MustCompile(`^imp_[0-9a-f]{20}$`)
	chunkIDPattern     = regexp.MustCompile(`^chk_[0-9a-f]{20}$`)
	candidateIDPattern = regexp.MustCompile(`^cand_[0-9a-f]{20}$`)
	digestPattern      = regexp.MustCompile(`^[0-9a-f]{64}$`)
	revisionPattern    = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

type ImportFile struct {
	Path   string `yaml:"path" json:"path"`
	Bytes  int64  `yaml:"bytes" json:"bytes"`
	SHA256 string `yaml:"sha256" json:"sha256"`
}

type ImportManifest struct {
	Version   int          `yaml:"version"`
	ID        string       `yaml:"id"`
	CreatedAt time.Time    `yaml:"created_at"`
	Files     []ImportFile `yaml:"files"`
}

type ImportSummary struct {
	ID         string `json:"id"`
	CreatedAt  string `json:"created_at"`
	FileCount  int    `json:"file_count"`
	TotalBytes int64  `json:"total_bytes"`
}

func ValidateImportID(value string) error {
	return validateStableID(importIDPattern, value)
}

func ValidateChunkID(value string) error {
	return validateStableID(chunkIDPattern, value)
}

func ValidateCandidateID(value string) error {
	return validateStableID(candidateIDPattern, value)
}

// ValidateCandidateRevision rejects malformed optimistic concurrency tokens.
func ValidateCandidateRevision(value string) error {
	if !revisionPattern.MatchString(value) {
		return fmt.Errorf("candidate revision is invalid: %w", ErrInvalidCandidate)
	}
	return nil
}

func (manifest ImportManifest) Summary() ImportSummary {
	totalBytes := int64(0)
	for _, file := range manifest.Files {
		totalBytes += file.Bytes
	}
	return ImportSummary{
		ID:         manifest.ID,
		CreatedAt:  manifest.CreatedAt.UTC().Format(time.RFC3339),
		FileCount:  len(manifest.Files),
		TotalBytes: totalBytes,
	}
}

func (manifest *ImportManifest) Validate() error {
	if manifest.Version != ManifestVersion {
		return fmt.Errorf("manifest version %d is unsupported: %w", manifest.Version, ErrInvalidManifest)
	}
	if err := ValidateImportID(manifest.ID); err != nil {
		return fmt.Errorf("manifest id %q is invalid: %w", manifest.ID, ErrInvalidManifest)
	}
	if manifest.CreatedAt.IsZero() {
		return fmt.Errorf("manifest created_at is required: %w", ErrInvalidManifest)
	}
	manifest.CreatedAt = manifest.CreatedAt.UTC()
	if len(manifest.Files) == 0 {
		return fmt.Errorf("manifest requires at least one file: %w", ErrInvalidManifest)
	}
	if len(manifest.Files) > maxImportFiles {
		return fmt.Errorf("manifest exceeds %d files: %w", maxImportFiles, ErrInvalidManifest)
	}
	totalBytes := int64(0)
	paths := make([]string, 0, len(manifest.Files))
	previousPath := ""
	for index := range manifest.Files {
		file := &manifest.Files[index]
		normalizedPath, err := NormalizePortableRelativePath(file.Path)
		if err != nil {
			return err
		}
		file.Path = normalizedPath
		if previousPath != "" && previousPath >= file.Path {
			return fmt.Errorf("manifest files must be sorted by path: %w", ErrInvalidManifest)
		}
		previousPath = file.Path
		if file.Bytes < 0 || file.Bytes > maxImportFileBytes {
			return fmt.Errorf("manifest file %q exceeds byte bounds: %w", file.Path, ErrInvalidManifest)
		}
		if !digestPattern.MatchString(file.SHA256) {
			return fmt.Errorf("manifest file %q digest is invalid: %w", file.Path, ErrInvalidManifest)
		}
		totalBytes += file.Bytes
		paths = append(paths, file.Path)
	}
	if totalBytes > maxImportBatchBytes {
		return fmt.Errorf("manifest exceeds %d total bytes: %w", maxImportBatchBytes, ErrInvalidManifest)
	}
	if err := ValidatePortablePathSet(paths); err != nil {
		if errors.Is(err, ErrCaseFoldedCollision) {
			return err
		}
		return fmt.Errorf("manifest path set invalid: %w", ErrInvalidManifest)
	}
	return nil
}

func DiscoverEligibleRelativePaths(paths []string) []string {
	filtered := make([]string, 0, len(paths))
	for _, candidate := range paths {
		normalized, err := NormalizePortableRelativePath(candidate)
		if err != nil {
			continue
		}
		if !isEligibleMarkdownPath(normalized) {
			continue
		}
		filtered = append(filtered, normalized)
	}
	slices.Sort(filtered)
	return filtered
}

func NormalizePortableRelativePath(value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("portable path is empty: %w", ErrInvalidPath)
	}
	value = strings.ReplaceAll(value, "\\", "/")
	if !utf8.ValidString(value) {
		return "", fmt.Errorf("portable path is not valid UTF-8: %w", ErrInvalidPath)
	}
	value = normalizePortablePathUnicode(value)
	if path.IsAbs(value) {
		return "", fmt.Errorf("portable path %q must be relative: %w", value, ErrInvalidPath)
	}
	for _, part := range strings.Split(value, "/") {
		if part == "." || part == ".." {
			return "", fmt.Errorf("portable path %q contains traversal segment %q: %w", value, part, ErrInvalidPath)
		}
	}
	cleaned := path.Clean(value)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("portable path %q escapes root: %w", value, ErrInvalidPath)
	}
	if strings.HasPrefix(cleaned, "/") {
		return "", fmt.Errorf("portable path %q must not be absolute: %w", value, ErrInvalidPath)
	}
	parts := strings.Split(cleaned, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("portable path %q has invalid segment %q: %w", value, part, ErrInvalidPath)
		}
		if strings.HasPrefix(part, ".") {
			return "", fmt.Errorf("portable path %q includes hidden segment %q: %w", value, part, ErrInvalidPath)
		}
		if containsControlRune(part) {
			return "", fmt.Errorf("portable path %q includes control characters: %w", value, ErrInvalidPath)
		}
	}
	return cleaned, nil
}

func ValidatePortablePathSet(paths []string) error {
	seen := make(map[string]string, len(paths))
	for _, value := range paths {
		normalized, err := NormalizePortableRelativePath(value)
		if err != nil {
			return err
		}
		key := strings.ToLower(normalized)
		if existing, ok := seen[key]; ok && existing != normalized {
			return fmt.Errorf("portable paths %q and %q collide after case-folding: %w", existing, normalized, ErrCaseFoldedCollision)
		}
		seen[key] = normalized
	}
	return nil
}

func CanonicalSHA256(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func NormalizeMarkdownText(value string) (string, error) {
	if strings.ContainsRune(value, '\x00') {
		return "", fmt.Errorf("markdown contains NUL bytes: %w", ErrInvalidContent)
	}
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	if !utf8.ValidString(value) {
		return "", fmt.Errorf("markdown is not valid UTF-8: %w", ErrInvalidContent)
	}
	return value, nil
}

func validateStableID(pattern *regexp.Regexp, value string) error {
	if !pattern.MatchString(value) {
		return fmt.Errorf("identifier %q is invalid: %w", value, ErrInvalidID)
	}
	return nil
}

func isEligibleMarkdownPath(value string) bool {
	if hasHiddenComponent(value) {
		return false
	}
	lower := strings.ToLower(value)
	return strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".markdown")
}

func hasHiddenComponent(value string) bool {
	for _, part := range strings.Split(value, "/") {
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}

func containsControlRune(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func normalizePortablePathUnicode(value string) string {
	return norm.NFC.String(value)
}

const (
	digestA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	digestB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)
