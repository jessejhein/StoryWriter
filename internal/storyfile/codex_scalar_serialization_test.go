// BDD Scenario: 3.1.2 - Create an entry
// Requirements: M3-R04, M3-R18
// Test purpose: Canonical entry serialization preserves numeric-looking and timestamp-looking strings as YAML strings.
package storyfile

import (
	"strings"
	"testing"

	"storywork/internal/codex"
)

// Test: values that YAML would implicitly type are quoted so the canonical schema remains string-typed.
// Requirements: M3-R04, M3-R18
func TestMarshalCodexEntryQuotesImplicitYAMLTypes(t *testing.T) {
	t.Parallel()

	contents, err := New().MarshalCodexEntry(codex.Entry{
		ID:          "char_0123456789abcdef0123",
		Type:        codex.TypeCharacter,
		Name:        "123",
		Aliases:     []string{"2026-06-29"},
		Tags:        []string{},
		Description: "1.5",
		Metadata:    map[string]string{"rank": "0x10"},
	})
	if err != nil {
		t.Fatalf("MarshalCodexEntry() error = %v", err)
	}
	for _, want := range []string{
		`name: "123"`,
		`  - "2026-06-29"`,
		`description: "1.5"`,
		`  rank: "0x10"`,
	} {
		if !strings.Contains(string(contents), want) {
			t.Fatalf("canonical bytes lack %q:\n%s", want, contents)
		}
	}
}
