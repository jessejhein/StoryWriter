package story

import (
	"errors"
	"testing"
)

// BDD trace:
//   - Requirements: M4-R09, M4-R13.
//   - Scenario: 4.3.3, 4.4.2.
//   - Test purpose: verify the canonical scene-selection helpers enforce UTF-8
//     byte boundaries, selected-text matches, and exact replacement semantics for
//     ASCII and multibyte Markdown.
func TestValidateAndReplaceMarkdownSelection(t *testing.T) {
	t.Parallel()

	markdown := "Alpha\nLuz ágil vuela\nOmega\n"
	start := len([]byte("Alpha\n"))
	end := start + len([]byte("Luz ágil"))

	selected, err := ValidateMarkdownSelection(markdown, start, end, "Luz ágil")
	if err != nil {
		t.Fatalf("ValidateMarkdownSelection() error = %v", err)
	}
	if selected != "Luz ágil" {
		t.Fatalf("selected = %q, want %q", selected, "Luz ágil")
	}

	replaced, err := ReplaceMarkdownSelection(markdown, start, end, "Luz ágil", "Mock polished: Luz ágil")
	if err != nil {
		t.Fatalf("ReplaceMarkdownSelection() error = %v", err)
	}
	if replaced != "Alpha\nMock polished: Luz ágil vuela\nOmega\n" {
		t.Fatalf("replaced = %q", replaced)
	}

	midRune := start + len([]byte("Luz "))
	if _, err := ValidateMarkdownSelection(markdown, midRune+1, end, "gil"); !errors.Is(err, ErrInvalidSelection) {
		t.Fatalf("ValidateMarkdownSelection(mid-rune) error = %v, want ErrInvalidSelection", err)
	}
	if _, err := ValidateMarkdownSelection(markdown, start, end, "Wrong"); !errors.Is(err, ErrInvalidSelection) {
		t.Fatalf("ValidateMarkdownSelection(mismatch) error = %v, want ErrInvalidSelection", err)
	}
	if _, err := ReplaceMarkdownSelection(markdown, start, end, "Luz ágil", "Luz ágil"); !errors.Is(err, ErrNoSceneChanges) {
		t.Fatalf("ReplaceMarkdownSelection(no-op) error = %v, want ErrNoSceneChanges", err)
	}
}
