// BDD Scenario: 3.2.3 - Report malformed canonical progressions
// Requirements: M3-R05, M3-R18
// Test purpose: Strict progression reads reject malformed, unsupported, typed, and path-inconsistent canonical documents.
package storyfile

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// Test: malformed, unsupported, typed, or inconsistent progression documents are rejected rather than repaired.
// Requirements: M3-R05, M3-R18
func TestLoadProgressionsRejectsMalformedCanonicalDocuments(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		contents string
	}{
		{name: "entry ID disagrees with filename", contents: "version: 1\nentry_id: char_99999999999999999999\nprogressions: []\n"},
		{name: "unsupported version", contents: "version: 2\nentry_id: char_0123456789abcdef0123\nprogressions: []\n"},
		{name: "unknown top-level field", contents: "version: 1\nentry_id: char_0123456789abcdef0123\nprogressions: []\nextra: true\n"},
		{name: "malformed progression ID", contents: "version: 1\nentry_id: char_0123456789abcdef0123\nprogressions:\n  - id: bad\n    anchor:\n      type: scene\n      id: scn_0123456789abcdef0123\n      timing: after\n    changes:\n      description: Changed.\n"},
		{name: "non-scene anchor", contents: "version: 1\nentry_id: char_0123456789abcdef0123\nprogressions:\n  - id: prog_0123456789abcdef0123\n    anchor:\n      type: chapter\n      id: scn_0123456789abcdef0123\n      timing: after\n    changes:\n      description: Changed.\n"},
		{name: "duplicate anchor field", contents: "version: 1\nentry_id: char_0123456789abcdef0123\nprogressions:\n  - id: prog_0123456789abcdef0123\n    anchor:\n      type: scene\n      id: scn_0123456789abcdef0123\n      id: scn_0123456789abcdef0124\n      timing: after\n    changes:\n      description: Changed.\n"},
		{name: "invalid timing", contents: "version: 1\nentry_id: char_0123456789abcdef0123\nprogressions:\n  - id: prog_0123456789abcdef0123\n    anchor:\n      type: scene\n      id: scn_0123456789abcdef0123\n      timing: during\n    changes:\n      description: Changed.\n"},
		{name: "ineffective changes", contents: "version: 1\nentry_id: char_0123456789abcdef0123\nprogressions:\n  - id: prog_0123456789abcdef0123\n    anchor:\n      type: scene\n      id: scn_0123456789abcdef0123\n      timing: after\n    changes: {}\n"},
		{name: "empty metadata", contents: "version: 1\nentry_id: char_0123456789abcdef0123\nprogressions:\n  - id: prog_0123456789abcdef0123\n    anchor:\n      type: scene\n      id: scn_0123456789abcdef0123\n      timing: after\n    changes:\n      description: Changed.\n      metadata: {}\n"},
		{name: "numeric description", contents: "version: 1\nentry_id: char_0123456789abcdef0123\nprogressions:\n  - id: prog_0123456789abcdef0123\n    anchor:\n      type: scene\n      id: scn_0123456789abcdef0123\n      timing: after\n    changes:\n      description: 42\n"},
		{name: "boolean metadata value", contents: "version: 1\nentry_id: char_0123456789abcdef0123\nprogressions:\n  - id: prog_0123456789abcdef0123\n    anchor:\n      type: scene\n      id: scn_0123456789abcdef0123\n      timing: after\n    changes:\n      metadata:\n        active: true\n"},
	}

	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			mustMkdirAll(t, root, "progressions")
			path := filepath.Join(root, "progressions", "char_0123456789abcdef0123.yaml")
			if err := os.WriteFile(path, []byte(testCase.contents), 0o644); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}

			if _, err := New().LoadProgressions(context.Background(), root, "char_0123456789abcdef0123"); err == nil {
				t.Fatal("LoadProgressions() error = nil")
			}
		})
	}
}
