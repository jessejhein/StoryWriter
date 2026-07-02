package importer

import "testing"

func FuzzNormalizePortableRelativePath(f *testing.F) {
	seeds := []string{
		"",
		"notes/characters.md",
		"notes\\characters.md",
		"../escape.md",
		".hidden/file.md",
		"notes/\x00bad.md",
		"notes/Cafe\u0301.md",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		_, _ = NormalizePortableRelativePath(input)
	})
}

func FuzzValidateStableIDs(f *testing.F) {
	seeds := []string{
		"imp_0123456789abcdef0123",
		"chk_0123456789abcdef0123",
		"cand_0123456789abcdef0123",
		"",
		"bad",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, value string) {
		_ = ValidateImportID(value)
		_ = ValidateChunkID(value)
		_ = ValidateCandidateID(value)
	})
}
