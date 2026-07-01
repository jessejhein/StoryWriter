package importer

import (
	"errors"
	"os"
	"path/filepath"
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

func TestCandidateStoreRejectsUnknownYAMLFields(t *testing.T) {
	t.Parallel()

	projectPath := t.TempDir()
	reviewPath := filepath.Join(projectPath, "imports", "review")
	if err := os.MkdirAll(reviewPath, 0o755); err != nil {
		t.Fatal(err)
	}
	created, err := NewCandidateStore().Create(projectPath, Candidate{
		Version: CandidateVersion, ID: "cand_0123456789abcdef0123", Kind: CandidateKindArc,
		ProposalVersion: 1, Status: CandidateStatusPending,
		Provenance: Provenance{ChunkIDs: []string{"chk_0123456789abcdef0123"}},
		Proposal:   CandidateProposal{Arc: &ArcProposal{Title: "Act One"}},
		Decision:   CandidateDecision{CanonicalRefs: []CanonicalRef{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(reviewPath, created.ID+".yaml")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body = append(body, []byte("unknown: unsafe\n")...)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := NewCandidateStore().Load(projectPath, "cand_0123456789abcdef0123"); !errors.Is(err, ErrInvalidCandidate) {
		t.Fatalf("Load() error = %v, want %v", err, ErrInvalidCandidate)
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

func TestNormalizeCandidateRejectsMalformedAcceptedCanonicalReference(t *testing.T) {
	t.Parallel()

	_, err := NormalizeCandidate(Candidate{
		Version: CandidateVersion, ID: "cand_0123456789abcdef0123", Kind: CandidateKindArc,
		ProposalVersion: 1, Status: CandidateStatusAccepted,
		Provenance: Provenance{ChunkIDs: []string{"chk_0123456789abcdef0123"}},
		Proposal:   CandidateProposal{Arc: &ArcProposal{Title: "Act One"}},
		Decision:   CandidateDecision{CanonicalRefs: []CanonicalRef{{Kind: "scene", ID: "not-an-id"}}},
	})
	if !errors.Is(err, ErrInvalidCandidate) {
		t.Fatalf("NormalizeCandidate() error = %v, want %v", err, ErrInvalidCandidate)
	}
}
