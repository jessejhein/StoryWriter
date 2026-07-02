package contextpack

// BDD Scenario: 7.2.2 - Exclude irrelevant Codex entries
// Requirements: M7-R06
// Test purpose: Lexical mention evidence is deterministic and boundary-safe before ranking.

import (
	"testing"
)

// Test: lexical evidence matches canonical names and aliases.
// Requirements: M7-R06.
func TestLexicalEvidenceMatchesCanonicalNamesAndAliases(t *testing.T) {
	t.Parallel()

	entry := CodexEntryCandidate{
		EntryID: "char_0123456789abcdef0123", EntryType: "character", Name: "Ann Vale", Aliases: []string{"Annie"},
	}
	evidence := ComputeLexicalEvidence("Ann Vale met Annie at the gate.", entry)
	if !evidence.NameMention || !evidence.AliasMention || evidence.Occurrences < 2 {
		t.Fatalf("evidence = %#v", evidence)
	}
}

// Test: lexical evidence matches tags with punctuation boundaries.
// Requirements: M7-R06.
func TestLexicalEvidenceMatchesTagsAndPunctuationBoundaries(t *testing.T) {
	t.Parallel()

	entry := CodexEntryCandidate{
		EntryID: "char_0123456789abcdef0123", EntryType: "character", Name: "Ann Vale", Tags: []string{"captain"},
	}
	evidence := ComputeLexicalEvidence("The captain—Ann Vale—waited.", entry)
	if !evidence.TagMention || evidence.Occurrences < 1 {
		t.Fatalf("evidence = %#v", evidence)
	}
}

// Test: lexical evidence case-folds Unicode names.
// Requirements: M7-R06.
func TestLexicalEvidenceCaseFoldsUnicode(t *testing.T) {
	t.Parallel()

	entry := CodexEntryCandidate{
		EntryID: "char_0123456789abcdef0123", EntryType: "character", Name: "Åsa",
	}
	evidence := ComputeLexicalEvidence("åsa waited by the dock.", entry)
	if !evidence.NameMention || evidence.Occurrences != 1 {
		t.Fatalf("evidence = %#v", evidence)
	}
}

// Test: lexical evidence does not match Ann inside annual.
// Requirements: M7-R06.
func TestLexicalEvidenceDoesNotMatchAnnInsideAnnual(t *testing.T) {
	t.Parallel()

	entry := CodexEntryCandidate{
		EntryID: "char_0123456789abcdef0123", EntryType: "character", Name: "Ann",
	}
	evidence := ComputeLexicalEvidence("The annual festival began.", entry)
	if evidence.HasMention() {
		t.Fatalf("evidence = %#v, want no mention", evidence)
	}
}

// Test: lexical evidence counts repeated mentions.
// Requirements: M7-R06.
func TestLexicalEvidenceCountsRepeatedMentions(t *testing.T) {
	t.Parallel()

	entry := CodexEntryCandidate{
		EntryID: "char_0123456789abcdef0123", EntryType: "character", Name: "Ann",
	}
	evidence := ComputeLexicalEvidence("Ann saw Ann and another Ann.", entry)
	if evidence.Occurrences != 3 {
		t.Fatalf("occurrences = %d, want 3", evidence.Occurrences)
	}
}
