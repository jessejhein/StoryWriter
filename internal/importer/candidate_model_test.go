package importer

import (
	"errors"
	"testing"
)

func TestNormalizeCandidateValidatesKindsProvenanceAndDecisionState(t *testing.T) {
	t.Parallel()

	candidate, err := NormalizeCandidate(Candidate{
		Version:         CandidateVersion,
		ID:              "cand_0123456789abcdef0123",
		Kind:            CandidateKindCodex,
		ProposalVersion: 1,
		Status:          CandidateStatusPending,
		Provenance: Provenance{
			ChunkIDs: []string{"chk_0123456789abcdef0124", "chk_0123456789abcdef0123"},
		},
		Proposal: CandidateProposal{
			Codex: &CodexProposal{
				Type:        "character",
				Name:        " Mara Venn ",
				Aliases:     []string{"Mara", "Mara"},
				Tags:        []string{"pilot", "pilot"},
				Description: "A cautious salvage pilot.",
			},
		},
	})
	if err != nil {
		t.Fatalf("NormalizeCandidate() error = %v", err)
	}
	if candidate.Revision == "" {
		t.Fatal("NormalizeCandidate() revision = empty")
	}
	if len(candidate.Decision.CanonicalRefs) != 0 {
		t.Fatalf("pending candidate canonical refs = %v", candidate.Decision.CanonicalRefs)
	}
	if len(candidate.Provenance.ChunkIDs) != 2 || candidate.Provenance.ChunkIDs[0] != "chk_0123456789abcdef0123" {
		t.Fatalf("normalized provenance = %v", candidate.Provenance.ChunkIDs)
	}
}

func TestNormalizeCandidateRejectsUnsupportedProposalVersionAndMismatchedRevision(t *testing.T) {
	t.Parallel()

	_, err := NormalizeCandidate(Candidate{
		Version:         CandidateVersion,
		ID:              "cand_0123456789abcdef0123",
		Kind:            CandidateKindArc,
		ProposalVersion: 2,
		Status:          CandidateStatusPending,
		Provenance:      Provenance{ChunkIDs: []string{"chk_0123456789abcdef0123"}},
		Proposal:        CandidateProposal{Arc: &ArcProposal{Title: "Act One"}},
	})
	if !errors.Is(err, ErrInvalidCandidate) {
		t.Fatalf("NormalizeCandidate() unsupported version error = %v, want %v", err, ErrInvalidCandidate)
	}

	_, err = NormalizeCandidate(Candidate{
		Version:         CandidateVersion,
		ID:              "cand_0123456789abcdef0123",
		Kind:            CandidateKindArc,
		ProposalVersion: 1,
		Status:          CandidateStatusPending,
		Revision:        "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Provenance:      Provenance{ChunkIDs: []string{"chk_0123456789abcdef0123"}},
		Proposal:        CandidateProposal{Arc: &ArcProposal{Title: "Act One"}},
	})
	if !errors.Is(err, ErrInvalidCandidate) {
		t.Fatalf("NormalizeCandidate() mismatched revision error = %v, want %v", err, ErrInvalidCandidate)
	}
}
