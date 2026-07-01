package importer

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestStableIdentifiersValidateKnownShapes(t *testing.T) {
	t.Parallel()

	validators := []struct {
		name  string
		value string
		fn    func(string) error
	}{
		{name: "import", value: "imp_0123456789abcdef0123", fn: ValidateImportID},
		{name: "chunk", value: "chk_0123456789abcdef0123", fn: ValidateChunkID},
		{name: "candidate", value: "cand_0123456789abcdef0123", fn: ValidateCandidateID},
	}

	for _, testCase := range validators {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if err := testCase.fn(testCase.value); err != nil {
				t.Fatalf("%s validator returned error: %v", testCase.name, err)
			}
			for _, invalid := range []string{"", "bad", testCase.value[:len(testCase.value)-1], testCase.value + "0", "IMP_0123456789abcdef0123"} {
				if err := testCase.fn(invalid); !errors.Is(err, ErrInvalidID) {
					t.Fatalf("%s validator(%q) error = %v, want %v", testCase.name, invalid, err, ErrInvalidID)
				}
			}
		})
	}
}

func TestManifestValidateEnforcesCanonicalForm(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	manifest := ImportManifest{
		Version:   ManifestVersion,
		ID:        "imp_0123456789abcdef0123",
		CreatedAt: createdAt,
		Files: []ImportFile{
			{Path: "notes/characters.md", Bytes: 1240, SHA256: digestA},
			{Path: "notes/empty.md", Bytes: 0, SHA256: digestB},
		},
	}
	if err := manifest.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if manifest.CreatedAt.Location() != time.UTC {
		t.Fatalf("created_at location = %v, want UTC", manifest.CreatedAt.Location())
	}

	summary := manifest.Summary()
	if summary.FileCount != 2 {
		t.Fatalf("Summary().FileCount = %d, want 2", summary.FileCount)
	}
	if summary.TotalBytes != 1240 {
		t.Fatalf("Summary().TotalBytes = %d, want 1240", summary.TotalBytes)
	}
	if summary.CreatedAt != "2026-06-30T12:00:00Z" {
		t.Fatalf("Summary().CreatedAt = %q", summary.CreatedAt)
	}

	encoded, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("json.Marshal(summary) error = %v", err)
	}
	if string(encoded) != `{"id":"imp_0123456789abcdef0123","created_at":"2026-06-30T12:00:00Z","file_count":2,"total_bytes":1240}` {
		t.Fatalf("json summary = %s", encoded)
	}
}

func TestManifestValidateRejectsInvalidMetadataAndLimits(t *testing.T) {
	t.Parallel()

	base := ImportManifest{
		Version:   ManifestVersion,
		ID:        "imp_0123456789abcdef0123",
		CreatedAt: time.Date(2026, 6, 30, 7, 0, 0, 0, time.FixedZone("CDT", -5*60*60)),
		Files: []ImportFile{
			{Path: "notes/characters.md", Bytes: maxImportFileBytes, SHA256: digestA},
		},
	}

	testCases := []struct {
		name    string
		mutate  func(*ImportManifest)
		wantErr error
	}{
		{name: "wrong version", mutate: func(m *ImportManifest) { m.Version = 0 }, wantErr: ErrInvalidManifest},
		{name: "empty files", mutate: func(m *ImportManifest) { m.Files = nil }, wantErr: ErrInvalidManifest},
		{name: "unsorted files", mutate: func(m *ImportManifest) {
			m.Files = []ImportFile{
				{Path: "z.md", Bytes: 1, SHA256: digestA},
				{Path: "a.md", Bytes: 1, SHA256: digestB},
			}
		}, wantErr: ErrInvalidManifest},
		{name: "too many files", mutate: func(m *ImportManifest) {
			m.Files = make([]ImportFile, maxImportFiles+1)
			for index := range m.Files {
				m.Files[index] = ImportFile{Path: filePathForIndex(index), Bytes: 1, SHA256: digestA}
			}
		}, wantErr: ErrInvalidManifest},
		{name: "file too large", mutate: func(m *ImportManifest) { m.Files[0].Bytes = maxImportFileBytes + 1 }, wantErr: ErrInvalidManifest},
		{name: "batch too large", mutate: func(m *ImportManifest) {
			m.Files = []ImportFile{
				{Path: "a.md", Bytes: maxImportBatchBytes / 2, SHA256: digestA},
				{Path: "b.md", Bytes: (maxImportBatchBytes / 2) + 1, SHA256: digestB},
			}
		}, wantErr: ErrInvalidManifest},
		{name: "invalid digest", mutate: func(m *ImportManifest) { m.Files[0].SHA256 = "bad" }, wantErr: ErrInvalidManifest},
		{name: "invalid path", mutate: func(m *ImportManifest) { m.Files[0].Path = "../escape.md" }, wantErr: ErrInvalidPath},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			manifest := base
			manifest.Files = slices.Clone(base.Files)
			testCase.mutate(&manifest)
			if err := manifest.Validate(); !errors.Is(err, testCase.wantErr) {
				t.Fatalf("Validate() error = %v, want %v", err, testCase.wantErr)
			}
		})
	}
}

func TestNormalizePortableRelativePathRejectsUnsafeComponents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr error
	}{
		{name: "portable path", input: "notes/characters.md", want: "notes/characters.md"},
		{name: "normalize separators and nfc", input: "notes\\Cafe\u0301.md", want: "notes/Caf\u00e9.md"},
		{name: "reject absolute", input: "/notes.md", wantErr: ErrInvalidPath},
		{name: "reject traversal", input: "notes/../secret.md", wantErr: ErrInvalidPath},
		{name: "reject hidden", input: ".hidden/notes.md", wantErr: ErrInvalidPath},
		{name: "reject hidden child", input: "notes/.hidden.md", wantErr: ErrInvalidPath},
		{name: "reject control", input: "notes/\x00bad.md", wantErr: ErrInvalidPath},
		{name: "reject empty", input: "", wantErr: ErrInvalidPath},
	}

	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got, err := NormalizePortableRelativePath(testCase.input)
			if testCase.wantErr != nil {
				if !errors.Is(err, testCase.wantErr) {
					t.Fatalf("NormalizePortableRelativePath(%q) error = %v, want %v", testCase.input, err, testCase.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizePortableRelativePath(%q) error = %v", testCase.input, err)
			}
			if got != testCase.want {
				t.Fatalf("NormalizePortableRelativePath(%q) = %q, want %q", testCase.input, got, testCase.want)
			}
		})
	}
}

func TestDetectCaseFoldedCollisionRejectsPortableConflicts(t *testing.T) {
	t.Parallel()

	err := ValidatePortablePathSet([]string{"Notes/Characters.md", "notes/characters.md"})
	if !errors.Is(err, ErrCaseFoldedCollision) {
		t.Fatalf("ValidatePortablePathSet(collision) error = %v, want %v", err, ErrCaseFoldedCollision)
	}
}

func TestDiscoveryPolicyFiltersAndOrdersEligibleMarkdownPaths(t *testing.T) {
	t.Parallel()

	paths := []string{
		"zeta/readme.markdown",
		"notes/.hidden.md",
		"notes/scene.txt",
		"Alpha/intro.MD",
		".git/config.md",
		"middle/outline.md",
	}
	got := DiscoverEligibleRelativePaths(paths)
	want := []string{"Alpha/intro.MD", "middle/outline.md", "zeta/readme.markdown"}
	if !slices.Equal(got, want) {
		t.Fatalf("DiscoverEligibleRelativePaths() = %v, want %v", got, want)
	}
}

func TestManifestSummaryOmitsExternalSourcePath(t *testing.T) {
	t.Parallel()

	manifest := ImportManifest{
		Version:   ManifestVersion,
		ID:        "imp_0123456789abcdef0123",
		CreatedAt: time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC),
		Files: []ImportFile{
			{Path: "notes/characters.md", Bytes: 10, SHA256: digestA},
		},
	}
	summary := manifest.Summary()
	encoded, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("json.Marshal(summary) error = %v", err)
	}
	if string(encoded) == "" || string(encoded) == `{"source_directory":"/tmp/notes"}` {
		t.Fatalf("summary unexpectedly preserved source details: %s", encoded)
	}
	if containsString(string(encoded), "/tmp/notes") {
		t.Fatalf("summary leaked external source path: %s", encoded)
	}
}

func filePathForIndex(index int) string {
	return "dir/file_" + paddedIndex(index) + ".md"
}

func paddedIndex(index int) string {
	const digits = "0123456789"
	value := [3]byte{'0', '0', '0'}
	value[0] = digits[(index/100)%10]
	value[1] = digits[(index/10)%10]
	value[2] = digits[index%10]
	return string(value[:])
}

func containsString(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
