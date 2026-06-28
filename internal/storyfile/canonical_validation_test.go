package storyfile

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// BDD trace:
//   - Requirement: Milestone 1, Story 1.1, view the outline; strict canonical
//     schemas in the Milestone 1 storage contract.
//   - Scenario: malformed canonical state is rejected rather than silently
//     repaired when an author requests the outline.
//   - Test purpose: verify every required schema/version/identity/title failure
//     is contextual and prevents an outline from loading.
func TestLoadRejectsEveryInvalidCanonicalStateClass(t *testing.T) {
	t.Parallel()

	valid := validCanonicalFixture()
	tests := []struct {
		name      string
		mutate    func(map[string]string)
		wantError string
	}{
		{name: "unsupported outline version", mutate: replaceFixture("outline.yaml", "version: 1", "version: 2"), wantError: "outline.yaml has unsupported version"},
		{name: "unsupported arc version", mutate: replaceFixture("arcs/arc_00000000000000000001.yaml", "version: 1", "version: 2"), wantError: "unsupported version"},
		{name: "unsupported chapter version", mutate: replaceFixture("chapters/ch_00000000000000000001.yaml", "version: 1", "version: 2"), wantError: "unsupported version"},
		{name: "unknown outline field", mutate: appendFixture("outline.yaml", "extra: true\n"), wantError: "field extra not found"},
		{name: "missing outline root", mutate: func(fixture map[string]string) { fixture["outline.yaml"] = "version: 1\n" }, wantError: "missing root"},
		{name: "missing arcs list", mutate: func(fixture map[string]string) { fixture["outline.yaml"] = "version: 1\nroot: {}\n" }, wantError: "missing arcs"},
		{name: "missing chapters list", mutate: replaceFixture("outline.yaml", "      chapters:\n        - id: ch_00000000000000000001\n          scenes:\n            - id: scn_00000000000000000001", "      unrelated: false"), wantError: "field unrelated not found"},
		{name: "omitted chapters list", mutate: replaceFixture("outline.yaml", "      chapters:\n        - id: ch_00000000000000000001\n          scenes:\n            - id: scn_00000000000000000001", ""), wantError: "missing chapters"},
		{name: "omitted scenes list", mutate: replaceFixture("outline.yaml", "          scenes:\n            - id: scn_00000000000000000001", ""), wantError: "missing scenes"},
		{name: "unknown scene field", mutate: replaceFixture("scenes/scn_00000000000000000001.md", "status: draft", "status: draft\nextra: true"), wantError: "field extra not found"},
		{name: "trailing YAML document", mutate: appendFixture("arcs/arc_00000000000000000001.yaml", "---\nversion: 1\n"), wantError: "unexpected extra YAML document"},
		{name: "duplicate arc ID", mutate: replaceFixture("outline.yaml", "    - id: arc_00000000000000000001", "    - id: arc_00000000000000000001\n      chapters: []\n    - id: arc_00000000000000000001"), wantError: "duplicate arc ID"},
		{name: "duplicate scene ID", mutate: replaceFixture("outline.yaml", "            - id: scn_00000000000000000001", "            - id: scn_00000000000000000001\n            - id: scn_00000000000000000001"), wantError: "duplicate scene ID"},
		{name: "arc file ID mismatch", mutate: replaceFixture("arcs/arc_00000000000000000001.yaml", "id: arc_00000000000000000001", "id: arc_00000000000000000002"), wantError: "does not match outline reference"},
		{name: "chapter file ID mismatch", mutate: replaceFixture("chapters/ch_00000000000000000001.yaml", "id: ch_00000000000000000001", "id: ch_00000000000000000002"), wantError: "does not match outline reference"},
		{name: "scene file ID mismatch", mutate: replaceFixture("scenes/scn_00000000000000000001.md", "id: scn_00000000000000000001", "id: scn_00000000000000000002"), wantError: "does not match outline reference"},
		{name: "empty arc title", mutate: replaceFixture("arcs/arc_00000000000000000001.yaml", "title: Act One", "title: '   '"), wantError: "invalid title"},
	}

	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			fixture := cloneFixture(valid)
			testCase.mutate(fixture)
			root := t.TempDir()
			for relativePath, contents := range fixture {
				absolutePath := filepath.Join(root, filepath.FromSlash(relativePath))
				if err := os.MkdirAll(filepath.Dir(absolutePath), 0o755); err != nil {
					t.Fatalf("MkdirAll(%s) error = %v", relativePath, err)
				}
				if err := os.WriteFile(absolutePath, []byte(contents), 0o644); err != nil {
					t.Fatalf("WriteFile(%s) error = %v", relativePath, err)
				}
			}

			_, err := New().Load(context.Background(), root)
			if err == nil || !strings.Contains(err.Error(), testCase.wantError) {
				t.Fatalf("Load() error = %v, want substring %q", err, testCase.wantError)
			}
		})
	}
}

func validCanonicalFixture() map[string]string {
	return map[string]string{
		"outline.yaml":                          "version: 1\nroot:\n  arcs:\n    - id: arc_00000000000000000001\n      chapters:\n        - id: ch_00000000000000000001\n          scenes:\n            - id: scn_00000000000000000001\n",
		"arcs/arc_00000000000000000001.yaml":    "version: 1\nid: arc_00000000000000000001\ntitle: Act One\n",
		"chapters/ch_00000000000000000001.yaml": "version: 1\nid: ch_00000000000000000001\narc_id: arc_00000000000000000001\ntitle: Arrival\n",
		"scenes/scn_00000000000000000001.md":    "---\nid: scn_00000000000000000001\ntitle: The Station\nchapter_id: ch_00000000000000000001\npov: \"\"\nstatus: draft\nexclude_from_ai: false\n---\n\n",
	}
}

func cloneFixture(source map[string]string) map[string]string {
	clone := make(map[string]string, len(source))
	for path, contents := range source {
		clone[path] = contents
	}
	return clone
}

func replaceFixture(path, old, replacement string) func(map[string]string) {
	return func(fixture map[string]string) {
		fixture[path] = strings.Replace(fixture[path], old, replacement, 1)
	}
}

func appendFixture(path, suffix string) func(map[string]string) {
	return func(fixture map[string]string) {
		fixture[path] += suffix
	}
}
