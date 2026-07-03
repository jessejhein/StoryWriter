// BDD Scenario: 8.2.3 - Keep comparisons bounded and safe
// Requirements: M8-R06, M8-R07
// Test purpose: Canonical project path allowlist and normalization reject unsafe
// paths before any Git or filesystem operation.

package branch_test

import (
	"errors"
	"strings"
	"testing"

	"storywork/internal/branch"
)

// Test: every allowed canonical family and extension is accepted.
// Requirements: M8-R07.
func TestValidateProjectPathAcceptsAllowedFamilies(t *testing.T) {
	t.Parallel()
	allowed := []string{
		"outline.yaml",
		"arcs/arc_00000000000000000001.yaml",
		"chapters/ch_00000000000000000001.yaml",
		"scenes/scn_00000000000000000001.md",
		"codex/characters/char_0123456789abcdef0123.yaml",
		"codex/locations/loc_0123456789abcdef0123.yaml",
		"codex/lore/lore_0123456789abcdef0123.yaml",
		"codex/custom/custom_0123456789abcdef0123.yaml",
		"progressions/char_0123456789abcdef0123.yaml",
		"agents/review.yaml",
		"styles/default.yaml",
		"imports/raw/imp_0123456789abcdef0123/manifest.yaml",
		"imports/raw/imp_0123456789abcdef0123/chapter.md",
		"imports/review/cand_0123456789abcdef0123.yaml",
	}
	for _, path := range allowed {
		if _, err := branch.ValidateProjectPath(path); err != nil {
			t.Fatalf("ValidateProjectPath(%q) error = %v", path, err)
		}
	}
}

// Test: absolute, traversal, backslash, NUL, control, and dot segments fail.
// Requirements: M8-R07.
func TestValidateProjectPathRejectsUnsafeSegments(t *testing.T) {
	t.Parallel()
	invalid := []string{
		"",
		"/etc/passwd",
		"../outline.yaml",
		"arcs/../outline.yaml",
		`scenes\scn_00000000000000000001.md`,
		"scenes/scn\x00.md",
		"scenes/\x01evil.md",
		"arcs/.hidden.yaml",
		"arcs/arc/../evil.yaml",
	}
	for _, path := range invalid {
		if _, err := branch.ValidateProjectPath(path); !errors.Is(err, branch.ErrInvalidProjectPath) {
			t.Fatalf("ValidateProjectPath(%q) = %v, want ErrInvalidProjectPath", path, err)
		}
	}
}

// Test: excluded root config, .storywork, credentials, and .gitkeep fail.
// Requirements: M8-R07.
func TestValidateProjectPathRejectsExcludedPaths(t *testing.T) {
	t.Parallel()
	excluded := []string{
		"project.yaml",
		".gitignore",
		".storywork/index.sqlite",
		"codex/.gitkeep",
		"random.txt",
		"database.sqlite",
	}
	for _, path := range excluded {
		if _, err := branch.ValidateProjectPath(path); !errors.Is(err, branch.ErrInvalidProjectPath) {
			t.Fatalf("ValidateProjectPath(%q) = %v, want ErrInvalidProjectPath", path, err)
		}
	}
}

// Test: slash normalization is validation, not repair.
// Requirements: M8-R07.
func TestValidateProjectPathDoesNotRepairPaths(t *testing.T) {
	t.Parallel()
	if _, err := branch.ValidateProjectPath("./outline.yaml"); !errors.Is(err, branch.ErrInvalidProjectPath) {
		t.Fatalf("ValidateProjectPath(./outline.yaml) = %v, want ErrInvalidProjectPath", err)
	}
}

// Test: strict UTF-8 and size bounds reject unsafe content.
// Requirements: M8-R07.
func TestValidateStrictUTF8EnforcesBounds(t *testing.T) {
	t.Parallel()
	if _, err := branch.ValidateStrictUTF8([]byte("hello")); err != nil {
		t.Fatalf("ValidateStrictUTF8() error = %v", err)
	}
	if _, err := branch.ValidateStrictUTF8([]byte{0xff, 0xfe, 0xfd}); !errors.Is(err, branch.ErrInvalidUTF8) {
		t.Fatalf("invalid UTF-8 = %v, want ErrInvalidUTF8", err)
	}
	large := []byte(strings.Repeat("a\n", branch.MaxFileLines+1))
	if _, err := branch.ValidateStrictUTF8(large); !errors.Is(err, branch.ErrFileTooLarge) {
		t.Fatalf("oversized lines = %v, want ErrFileTooLarge", err)
	}
}
