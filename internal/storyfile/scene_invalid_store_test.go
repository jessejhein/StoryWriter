package storyfile

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// BDD trace:
//   - Requirement: M2-R01, M2-R02, M2-R09.
//   - Scenario: 2.1.4 — Malformed canonical scene.
//   - Test purpose: verify malformed or unsupported scene front matter is
//     rejected instead of being silently repaired or partially loaded.
func TestLoadSceneRejectsMalformedCanonicalFiles(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := New()

	cases := []struct {
		name      string
		contents  string
		wantError string
	}{
		{
			name:      "unknown field",
			contents:  "---\nid: scn_00000000000000000001\ntitle: \"The Duel\"\nchapter_id: ch_00000000000000000001\npov: \"Luke\"\nstatus: draft\nexclude_from_ai: false\nmood: tense\n---\n\nText.\n",
			wantError: "field mood not found",
		},
		{
			name:      "duplicate field",
			contents:  "---\nid: scn_00000000000000000001\ntitle: \"One\"\ntitle: \"Two\"\nchapter_id: ch_00000000000000000001\npov: \"Luke\"\nstatus: draft\nexclude_from_ai: false\n---\n\nText.\n",
			wantError: "duplicate front matter field",
		},
		{
			name:      "missing field",
			contents:  "---\nid: scn_00000000000000000001\ntitle: \"The Duel\"\nchapter_id: ch_00000000000000000001\npov: \"Luke\"\nexclude_from_ai: false\n---\n\nText.\n",
			wantError: "missing front matter field",
		},
		{
			name:      "invalid status",
			contents:  "---\nid: scn_00000000000000000001\ntitle: \"The Duel\"\nchapter_id: ch_00000000000000000001\npov: \"Luke\"\nstatus: broken\nexclude_from_ai: false\n---\n\nText.\n",
			wantError: "unsupported",
		},
	}

	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			mustMkdirAll(t, root, "scenes")
			path := filepath.Join(root, "scenes", "scn_00000000000000000001.md")
			if err := os.WriteFile(path, []byte(testCase.contents), 0o644); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}

			_, err := store.LoadScene(ctx, root, "scn_00000000000000000001")
			if err == nil {
				t.Fatal("LoadScene() error = nil, want failure")
			}
			if !strings.Contains(err.Error(), testCase.wantError) {
				t.Fatalf("LoadScene() error = %q, want substring %q", err, testCase.wantError)
			}
		})
	}
}
