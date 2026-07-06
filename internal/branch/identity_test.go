// BDD Scenario: 8.1.1 - Create from current canon
// Requirements: M8-R01, M8-R02
// Test purpose: Experiment IDs, slug normalization, and managed branch refs are
// validated without calling Git or the filesystem.

package branch_test

import (
	"errors"
	"strings"
	"testing"

	"storywork/internal/branch"
)

type staticIDGenerator struct {
	id branch.ExperimentID
}

func (g *staticIDGenerator) NextExperimentID() (branch.ExperimentID, error) {
	return g.id, nil
}

// Test: exact brn_ ID validation and injected generation.
// Requirements: M8-R01.
func TestValidateExperimentIDAcceptsCanonicalShape(t *testing.T) {
	t.Parallel()
	id, err := branch.ValidateExperimentID("brn_0123456789abcdef0123")
	if err != nil {
		t.Fatalf("ValidateExperimentID() error = %v", err)
	}
	if id != "brn_0123456789abcdef0123" {
		t.Fatalf("id = %q", id)
	}
}

// Test: full lowercase SHA-1 and SHA-256 Git object identifiers are accepted.
// Requirements: M8-R06.
func TestValidateCommitIDAcceptsSupportedFullObjectIDs(t *testing.T) {
	t.Parallel()
	for _, value := range []string{strings.Repeat("a", 40), strings.Repeat("b", 64)} {
		if _, err := branch.ValidateCommitID(value); err != nil {
			t.Fatalf("ValidateCommitID(%d bytes) error = %v", len(value), err)
		}
	}
	for _, value := range []string{strings.Repeat("a", 39), strings.Repeat("A", 40), strings.Repeat("a", 63)} {
		if _, err := branch.ValidateCommitID(value); !errors.Is(err, branch.ErrInvalidCommitID) {
			t.Fatalf("ValidateCommitID(%q) error = %v", value, err)
		}
	}
}

// Test: exact brn_ ID validation rejects malformed values.
// Requirements: M8-R01.
func TestValidateExperimentIDRejectsMalformedValues(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"", "brn_", "brn_0123456789abcdef012", "BRN_0123456789abcdef0123", "run_0123456789abcdef0123"} {
		if _, err := branch.ValidateExperimentID(value); !errors.Is(err, branch.ErrInvalidExperimentID) {
			t.Fatalf("ValidateExperimentID(%q) = %v, want ErrInvalidExperimentID", value, err)
		}
	}
}

// Test: injected ID generator supplies experiment IDs for branch construction.
// Requirements: M8-R01.
func TestIDGeneratorInjectsExperimentIDs(t *testing.T) {
	t.Parallel()
	gen := &staticIDGenerator{id: "brn_0123456789abcdef0123"}
	got, err := gen.NextExperimentID()
	if err != nil {
		t.Fatalf("NextExperimentID() error = %v", err)
	}
	if got != "brn_0123456789abcdef0123" {
		t.Fatalf("id = %q", got)
	}
}

// Test: ASCII slug normalization and byte bounds.
// Requirements: M8-R01.
func TestNormalizeSlugDerivesCanonicalSlugs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		want string
	}{
		{name: "Obi-Wan lives", want: "obi-wan-lives"},
		{name: "  Yumina   Politics  ", want: "yumina-politics"},
		{name: "Arc2_Chapter-3", want: "arc2-chapter-3"},
	}
	for _, testCase := range tests {
		got, err := branch.NormalizeSlug(testCase.name)
		if err != nil {
			t.Fatalf("NormalizeSlug(%q) error = %v", testCase.name, err)
		}
		if got != testCase.want {
			t.Fatalf("NormalizeSlug(%q) = %q, want %q", testCase.name, got, testCase.want)
		}
	}
}

// Test: empty and unsupported names are rejected.
// Requirements: M8-R01.
func TestNormalizeSlugRejectsEmptyAndUnsupportedNames(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"", "   ", "!!!", "café", "chapter٢"} {
		if _, err := branch.NormalizeSlug(value); !errors.Is(err, branch.ErrInvalidExperimentName) {
			t.Fatalf("NormalizeSlug(%q) = %v, want ErrInvalidExperimentName", value, err)
		}
	}
}

// Test: reserved experiment names are rejected during slug normalization.
// Requirements: M8-R01.
func TestNormalizeSlugRejectsReservedNames(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"main"} {
		if _, err := branch.NormalizeSlug(value); !errors.Is(err, branch.ErrInvalidExperimentName) {
			t.Fatalf("NormalizeSlug(%q) = %v, want ErrInvalidExperimentName", value, err)
		}
	}
}

// Test: branch/<slug>-<hex> round trip and managed-ref recognition.
// Requirements: M8-R01.
func TestBuildAndParseManagedBranchRefRoundTrip(t *testing.T) {
	t.Parallel()
	id := branch.ExperimentID("brn_0123456789abcdef0123")
	ref, err := branch.BranchRefFromName("Obi-Wan lives", id)
	if err != nil {
		t.Fatalf("BranchRefFromName() error = %v", err)
	}
	want := branch.BranchRef("branch/obi-wan-lives-0123456789abcdef0123")
	if ref != want {
		t.Fatalf("ref = %q, want %q", ref, want)
	}
	parsedID, slug, err := branch.ParseManagedExperimentRef(string(ref))
	if err != nil {
		t.Fatalf("ParseManagedExperimentRef() error = %v", err)
	}
	if parsedID != id || slug != "obi-wan-lives" {
		t.Fatalf("parsed = (%q, %q)", parsedID, slug)
	}
	if !branch.IsManagedExperimentRef(string(ref)) {
		t.Fatal("IsManagedExperimentRef() = false, want true")
	}
}

// Test: BuildBranchRef accepts only already-normalized slugs.
// Requirements: M8-R01.
func TestBuildBranchRefAcceptsOnlyNormalizedSlugs(t *testing.T) {
	t.Parallel()
	id := branch.ExperimentID("brn_0123456789abcdef0123")
	valid, err := branch.BuildBranchRef("obi-wan-lives", id)
	if err != nil {
		t.Fatalf("BuildBranchRef(normalized) error = %v", err)
	}
	if valid != branch.BranchRef("branch/obi-wan-lives-0123456789abcdef0123") {
		t.Fatalf("ref = %q", valid)
	}
	for _, raw := range []string{"Obi-Wan lives", "obi_wan", "obi wan", "UPPER"} {
		if _, err := branch.BuildBranchRef(raw, id); !errors.Is(err, branch.ErrInvalidExperimentName) {
			t.Fatalf("BuildBranchRef(%q) = %v, want ErrInvalidExperimentName", raw, err)
		}
	}
}

// Test: reserved slugs are rejected during branch ref construction.
// Requirements: M8-R01.
func TestBuildBranchRefRejectsReservedSlugs(t *testing.T) {
	t.Parallel()
	id := branch.ExperimentID("brn_0123456789abcdef0123")
	if _, err := branch.BuildBranchRef("main", id); !errors.Is(err, branch.ErrInvalidExperimentName) {
		t.Fatalf("BuildBranchRef(main) = %v, want ErrInvalidExperimentName", err)
	}
}

// Test: reserved experiment names are rejected before managed ref generation.
// Requirements: M8-R01.
func TestBranchRefFromNameRejectsReservedNames(t *testing.T) {
	t.Parallel()
	id := branch.ExperimentID("brn_0123456789abcdef0123")
	if _, err := branch.BranchRefFromName("main", id); !errors.Is(err, branch.ErrInvalidExperimentName) {
		t.Fatalf("BranchRefFromName(main) = %v, want ErrInvalidExperimentName", err)
	}
}

// Test: Git-invalid and ref-injection forms are rejected; main is accepted.
// Requirements: M8-R01.
func TestValidateBranchRefRejectsUnsafeRefsAndAcceptsMain(t *testing.T) {
	t.Parallel()
	if err := branch.ValidateBranchRef(branch.CanonBranchName); err != nil {
		t.Fatalf("ValidateBranchRef(main) error = %v", err)
	}
	invalid := []string{
		"main\x00",
		"branch/",
		"branch/../evil-0123456789abcdef0123",
		"branch/obi..wan-0123456789abcdef0123",
		"branch/obi-wan-lives-0123456789abcdef0123.lock",
		"branch/main-0123456789abcdef0123",
		"feature/obi-wan",
		"branch/obi-wan-lives",
	}
	for _, value := range invalid {
		if err := branch.ValidateBranchRef(value); err == nil {
			t.Fatalf("ValidateBranchRef(%q) = nil, want error", value)
		}
	}
}

// Test: reserved experiment refs are rejected by branch parsing helpers.
// Requirements: M8-R01, M8-R06.
func TestParseManagedExperimentRefRejectsReservedSlug(t *testing.T) {
	t.Parallel()
	value := "branch/main-0123456789abcdef0123"
	if _, _, err := branch.ParseManagedExperimentRef(value); !errors.Is(err, branch.ErrInvalidBranchRef) {
		t.Fatalf("ParseManagedExperimentRef(%q) = %v, want ErrInvalidBranchRef", value, err)
	}
	if branch.IsManagedExperimentRef(value) {
		t.Fatalf("IsManagedExperimentRef(%q) = true, want false", value)
	}
}
