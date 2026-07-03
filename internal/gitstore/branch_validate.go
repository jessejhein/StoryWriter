package gitstore

import (
	"fmt"
	"path"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	canonBranch         = "main"
	experimentNamespace = "branch/"
)

var (
	commitIDPattern      = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
	experimentHexPattern = regexp.MustCompile(`^[0-9a-f]{20}$`)
	slugPattern          = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	refInjectionPattern  = regexp.MustCompile(`[\x00-\x1f\x7f~^:?*[]`)
)

func validateCommitID(value string) error {
	if !commitIDPattern.MatchString(value) {
		return fmt.Errorf("commit id %q is invalid", value)
	}
	return nil
}

func validateBranchRef(value string) error {
	if value == canonBranch {
		return nil
	}
	if !strings.HasPrefix(value, experimentNamespace) {
		return fmt.Errorf("branch ref %q is invalid", value)
	}
	suffix := strings.TrimPrefix(value, experimentNamespace)
	if suffix == "" || strings.Contains(suffix, "//") || strings.Contains(suffix, "..") || strings.HasSuffix(suffix, ".lock") {
		return fmt.Errorf("branch ref %q is invalid", value)
	}
	if refInjectionPattern.MatchString(value) {
		return fmt.Errorf("branch ref %q is invalid", value)
	}
	parts := strings.Split(suffix, "-")
	if len(parts) < 2 {
		return fmt.Errorf("branch ref %q is invalid", value)
	}
	hex := parts[len(parts)-1]
	if !experimentHexPattern.MatchString(hex) {
		return fmt.Errorf("branch ref %q is invalid", value)
	}
	slug := strings.Join(parts[:len(parts)-1], "-")
	if !slugPattern.MatchString(slug) || len(slug) == 0 || len(slug) > 48 {
		return fmt.Errorf("branch ref %q is invalid", value)
	}
	return nil
}

func validateProjectPath(value string) error {
	if value == "" || strings.Contains(value, "\\") || strings.ContainsRune(value, 0) || !utf8.ValidString(value) {
		return fmt.Errorf("path %q is invalid", value)
	}
	if path.IsAbs(value) {
		return fmt.Errorf("path %q is invalid", value)
	}
	cleaned := path.Clean(value)
	if cleaned != value || cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return fmt.Errorf("path %q is invalid", value)
	}
	for _, segment := range strings.Split(cleaned, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return fmt.Errorf("path %q is invalid", value)
		}
	}
	return nil
}
