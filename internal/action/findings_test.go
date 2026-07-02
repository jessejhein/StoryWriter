// BDD Scenario: 7.3.2 - Return suggestions without canon mutation
// Requirements: M7-R04
// Test purpose: Chapter Review findings reject partial or malformed provider output.

package action

import (
	"errors"
	"strings"
	"testing"
)

// Test: parser accepts strict valid response.
// Requirements: M7-R04.
func TestParseFindingsAcceptsStrictValidResponse(t *testing.T) {
	t.Parallel()

	raw := `{"findings":[{"title":"Pacing","explanation":"The transition drags.","scene_ids":["scn_0123456789abcdef0123"],"follow_up_agent_ids":["scene_rewrite"]}]}`
	response, err := ParseFindings(raw, map[string]struct{}{"scene_rewrite": {}}, map[string]struct{}{"scn_0123456789abcdef0123": {}})
	if err != nil {
		t.Fatalf("ParseFindings() error = %v", err)
	}
	if len(response.Findings) != 1 {
		t.Fatalf("findings = %#v", response.Findings)
	}
}

// Test: zero findings is valid.
// Requirements: M7-R04.
func TestParseFindingsAcceptsZeroFindings(t *testing.T) {
	t.Parallel()

	response, err := ParseFindings(`{"findings":[]}`, nil, nil)
	if err != nil {
		t.Fatalf("ParseFindings() error = %v", err)
	}
	if len(response.Findings) != 0 {
		t.Fatalf("findings = %#v", response.Findings)
	}
}

// Test: parser rejects unknown, missing, null, and trailing JSON.
// Requirements: M7-R04.
func TestParseFindingsRejectsUnknownMissingNullWrongAndTrailing(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{
		`{"findings":[{"title":"Pacing","explanation":"x","scene_ids":["scn_missingaaaaaaaaaaaa"],"follow_up_agent_ids":[]}]}`,
		`{"findings":[{"title":"","explanation":"x","scene_ids":["scn_0123456789abcdef0123"],"follow_up_agent_ids":[]}]}`,
		`{"findings":null}`,
		`{"findings":[{"title":"Pacing","explanation":"x","scene_ids":["scn_0123456789abcdef0123"],"follow_up_agent_ids":[]}]} {}`,
	} {
		if _, err := ParseFindings(raw, nil, map[string]struct{}{"scn_0123456789abcdef0123": {}}); !errors.Is(err, ErrInvalidFindings) {
			t.Fatalf("ParseFindings(%q) error = %v, want ErrInvalidFindings", raw, err)
		}
	}
}

// Test: parser rejects fences, bounds, and mixed invalid rows.
// Requirements: M7-R04.
func TestParseFindingsRejectsFencesBoundsAndMixedInvalidRows(t *testing.T) {
	t.Parallel()

	longTitle := strings.Repeat("x", 201)
	if _, err := ParseFindings("```json\n{}\n```", nil, nil); !errors.Is(err, ErrInvalidFindings) {
		t.Fatalf("fence error = %v", err)
	}
	raw := `{"findings":[{"title":"` + longTitle + `","explanation":"x","scene_ids":["scn_0123456789abcdef0123"],"follow_up_agent_ids":[]}]}`
	if _, err := ParseFindings(raw, nil, map[string]struct{}{"scn_0123456789abcdef0123": {}}); !errors.Is(err, ErrInvalidFindings) {
		t.Fatalf("bounds error = %v", err)
	}
}

// Test: parser validates scene and follow-up references.
// Requirements: M7-R04.
func TestParseFindingsValidatesSceneAndFollowUpReferences(t *testing.T) {
	t.Parallel()

	raw := `{"findings":[{"title":"Pacing","explanation":"x","scene_ids":["scn_0123456789abcdef0123"],"follow_up_agent_ids":["unknown_agent"]}]}`
	if _, err := ParseFindings(raw, map[string]struct{}{"scene_rewrite": {}}, map[string]struct{}{"scn_0123456789abcdef0123": {}}); !errors.Is(err, ErrInvalidFindings) {
		t.Fatalf("follow-up error = %v", err)
	}
}