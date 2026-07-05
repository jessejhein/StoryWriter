package branch

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

const (
	experimentIDPrefix = "brn_"
	experimentHexLen   = 20
	slugMaxBytes       = 48
)

var (
	experimentIDPattern     = regexp.MustCompile(`^brn_[0-9a-f]{20}$`)
	experimentHexPattern    = regexp.MustCompile(`^[0-9a-f]{20}$`)
	slugPattern             = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	refInjectionPattern     = regexp.MustCompile(`[\x00-\x1f\x7f~^:?*[]`)
	reservedExperimentSlugs = map[string]struct{}{
		"main": {},
	}
)

// IDGenerator creates experiment identifiers for tests and production.
type IDGenerator interface {
	NextExperimentID() (ExperimentID, error)
}

// ValidateExperimentID requires brn_ plus 20 lowercase hex characters.
func ValidateExperimentID(value string) (ExperimentID, error) {
	if !experimentIDPattern.MatchString(value) {
		return "", fmt.Errorf("experiment id %q: %w", value, ErrInvalidExperimentID)
	}
	return ExperimentID(value), nil
}

// ExperimentHex returns the 20-character hex suffix without the brn_ prefix.
func ExperimentHex(id ExperimentID) (string, error) {
	if _, err := ValidateExperimentID(string(id)); err != nil {
		return "", err
	}
	return string(id)[len(experimentIDPrefix):], nil
}

// NormalizeSlug derives a display slug from a trimmed author name.
func NormalizeSlug(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", fmt.Errorf("experiment name is empty: %w", ErrInvalidExperimentName)
	}
	var builder strings.Builder
	lastHyphen := false
	for _, r := range trimmed {
		switch {
		case unicode.IsLetter(r) && r <= unicode.MaxASCII:
			if unicode.IsUpper(r) {
				r = unicode.ToLower(r)
			}
			builder.WriteRune(r)
			lastHyphen = false
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastHyphen = false
		case r == ' ' || r == '-' || r == '_':
			if builder.Len() == 0 || lastHyphen {
				continue
			}
			builder.WriteByte('-')
			lastHyphen = true
		default:
			return "", fmt.Errorf("experiment name contains unsupported character: %w", ErrInvalidExperimentName)
		}
	}
	slug := strings.Trim(builder.String(), "-")
	if slug == "" {
		return "", fmt.Errorf("normalized experiment name is empty: %w", ErrInvalidExperimentName)
	}
	if len(slug) > slugMaxBytes {
		return "", fmt.Errorf("normalized experiment name exceeds %d bytes: %w", slugMaxBytes, ErrInvalidExperimentName)
	}
	if !slugPattern.MatchString(slug) {
		return "", fmt.Errorf("normalized experiment name %q is invalid: %w", slug, ErrInvalidExperimentName)
	}
	if _, reserved := reservedExperimentSlugs[slug]; reserved {
		return "", fmt.Errorf("normalized experiment name %q is reserved: %w", slug, ErrInvalidExperimentName)
	}
	return slug, nil
}

// ValidateSlug accepts only an already-normalized slug.
func ValidateSlug(slug string) error {
	if !slugPattern.MatchString(slug) || len(slug) == 0 || len(slug) > slugMaxBytes {
		return fmt.Errorf("slug %q: %w", slug, ErrInvalidExperimentName)
	}
	if _, reserved := reservedExperimentSlugs[slug]; reserved {
		return fmt.Errorf("slug %q is reserved: %w", slug, ErrInvalidExperimentName)
	}
	return nil
}

// BuildBranchRef constructs branch/<slug>-<hex> from a validated normalized slug.
func BuildBranchRef(slug string, id ExperimentID) (BranchRef, error) {
	if err := ValidateSlug(slug); err != nil {
		return "", err
	}
	hex, err := ExperimentHex(id)
	if err != nil {
		return "", err
	}
	ref := BranchRef(ExperimentNamespace + slug + "-" + hex)
	if err := ValidateBranchRef(string(ref)); err != nil {
		return "", err
	}
	return ref, nil
}

// BranchRefFromName builds a managed ref from an author name and experiment id.
func BranchRefFromName(name string, id ExperimentID) (BranchRef, error) {
	slug, err := NormalizeSlug(name)
	if err != nil {
		return "", err
	}
	return BuildBranchRef(slug, id)
}

// ValidateBranchRef accepts only managed branch/ refs with safe characters.
func ValidateBranchRef(value string) error {
	if value == CanonBranchName {
		return nil
	}
	if !strings.HasPrefix(value, ExperimentNamespace) {
		return fmt.Errorf("branch ref %q: %w", value, ErrInvalidBranchRef)
	}
	suffix := strings.TrimPrefix(value, ExperimentNamespace)
	if suffix == "" {
		return fmt.Errorf("branch ref %q: %w", value, ErrInvalidBranchRef)
	}
	if strings.Contains(suffix, "//") || strings.Contains(suffix, "..") || strings.HasSuffix(suffix, ".lock") {
		return fmt.Errorf("branch ref %q: %w", value, ErrInvalidBranchRef)
	}
	if refInjectionPattern.MatchString(value) {
		return fmt.Errorf("branch ref %q: %w", value, ErrInvalidBranchRef)
	}
	parts := strings.Split(suffix, "-")
	if len(parts) < 2 {
		return fmt.Errorf("branch ref %q: %w", value, ErrInvalidBranchRef)
	}
	hex := parts[len(parts)-1]
	if !experimentHexPattern.MatchString(hex) {
		return fmt.Errorf("branch ref %q: %w", value, ErrInvalidBranchRef)
	}
	slug := strings.Join(parts[:len(parts)-1], "-")
	if !slugPattern.MatchString(slug) || len(slug) == 0 || len(slug) > slugMaxBytes {
		return fmt.Errorf("branch ref %q: %w", value, ErrInvalidBranchRef)
	}
	return nil
}

// ParseManagedExperimentRef extracts experiment id and slug from a managed ref.
func ParseManagedExperimentRef(value string) (ExperimentID, string, error) {
	if err := ValidateBranchRef(value); err != nil {
		return "", "", err
	}
	if value == CanonBranchName {
		return "", "", fmt.Errorf("branch ref %q: %w", value, ErrInvalidBranchRef)
	}
	suffix := strings.TrimPrefix(value, ExperimentNamespace)
	parts := strings.Split(suffix, "-")
	hex := parts[len(parts)-1]
	slug := strings.Join(parts[:len(parts)-1], "-")
	id, err := ValidateExperimentID(experimentIDPrefix + hex)
	if err != nil {
		return "", "", err
	}
	return id, slug, nil
}

// IsManagedExperimentRef reports whether value is a validated branch/ ref.
func IsManagedExperimentRef(value string) bool {
	return ValidateBranchRef(value) == nil && value != CanonBranchName && strings.HasPrefix(value, ExperimentNamespace)
}

// ValidateSwitchTarget accepts main or a managed experiment id.
func ValidateSwitchTarget(target string) (string, bool, error) {
	if target == CanonBranchName {
		return CanonBranchName, true, nil
	}
	id, err := ValidateExperimentID(target)
	if err != nil {
		return "", false, err
	}
	return string(id), false, nil
}
