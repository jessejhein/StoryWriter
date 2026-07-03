// BDD Scenario: 8.3.2 - Return strict findings only
// Requirements: M8-R09, M8-R10
// Test purpose: Ramification output parsing rejects malformed provider JSON.

package branch_test

import (
	"errors"
	"strings"
	"testing"

	"storywork/internal/branch"
)

// Test: valid zero findings are accepted.
// Requirements: M8-R10.
func TestParseRamificationOutputAcceptsZeroFindings(t *testing.T) {
	t.Parallel()
	summary, findings, err := branch.ParseRamificationOutput([]byte(`{"summary":"ok","findings":[]}`), map[branch.ProjectPath]struct{}{"outline.yaml": {}})
	if err != nil {
		t.Fatalf("ParseRamificationOutput() error = %v", err)
	}
	if summary != "ok" || len(findings) != 0 {
		t.Fatalf("summary=%q findings=%#v", summary, findings)
	}
}

// Test: unknown fields reject the whole output.
// Requirements: M8-R10.
func TestParseRamificationOutputRejectsUnknownFields(t *testing.T) {
	t.Parallel()
	_, _, err := branch.ParseRamificationOutput([]byte(`{"summary":"ok","findings":[],"patch":"nope"}`), map[branch.ProjectPath]struct{}{})
	if !errors.Is(err, branch.ErrInvalidAnalysisOutput) {
		t.Fatalf("err = %v", err)
	}
}

// Test: missing, null, duplicate, wrongly typed, trailing, fenced, oversized,
// invalid enum, and unreviewed-path output rejects the entire response.
// Requirements: M8-R10.
func TestParseRamificationOutputRejectsEveryStrictContractViolation(t *testing.T) {
	t.Parallel()
	allowed := map[branch.ProjectPath]struct{}{"outline.yaml": {}}
	validFinding := `{"category":"plot","severity":"low","title":"Title","explanation":"Explanation","affected_paths":["outline.yaml"],"recommended_action":"Review."}`
	tests := []struct {
		name string
		raw  string
	}{
		{name: "missing findings", raw: `{"summary":"ok"}`},
		{name: "null findings", raw: `{"summary":"ok","findings":null}`},
		{name: "duplicate envelope key", raw: `{"summary":"ok","summary":"again","findings":[]}`},
		{name: "duplicate finding key", raw: `{"summary":"ok","findings":[{"category":"plot","category":"world","severity":"low","title":"Title","explanation":"Explanation","affected_paths":["outline.yaml"],"recommended_action":"Review."}]}`},
		{name: "wrong findings type", raw: `{"summary":"ok","findings":{}}`},
		{name: "trailing object", raw: `{"summary":"ok","findings":[]} {}`},
		{name: "fenced", raw: "```json\n{\"summary\":\"ok\",\"findings\":[]}\n```"},
		{name: "too many findings", raw: `{"summary":"ok","findings":[` + strings.TrimSuffix(strings.Repeat(validFinding+",", 31), ",") + `]}`},
		{name: "invalid category", raw: `{"summary":"ok","findings":[{"category":"patch","severity":"low","title":"Title","explanation":"Explanation","affected_paths":["outline.yaml"],"recommended_action":"Review."}]}`},
		{name: "unreviewed path", raw: `{"summary":"ok","findings":[{"category":"plot","severity":"low","title":"Title","explanation":"Explanation","affected_paths":["scenes/scn_0123456789abcdef0123.md"],"recommended_action":"Review."}]}`},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			_, _, err := branch.ParseRamificationOutput([]byte(testCase.raw), allowed)
			if !errors.Is(err, branch.ErrInvalidAnalysisOutput) {
				t.Fatalf("error = %v, want ErrInvalidAnalysisOutput", err)
			}
		})
	}
}
