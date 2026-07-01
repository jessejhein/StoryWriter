package importer

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"storywork/internal/codex"
	"storywork/internal/story"

	"gopkg.in/yaml.v3"
)

const CandidateVersion = 1

type CandidateKind string

const (
	CandidateKindCodex   CandidateKind = "codex"
	CandidateKindArc     CandidateKind = "arc"
	CandidateKindChapter CandidateKind = "chapter"
	CandidateKindScene   CandidateKind = "scene"
)

type CandidateStatus string

const (
	CandidateStatusPending   CandidateStatus = "pending"
	CandidateStatusMerged    CandidateStatus = "merged"
	CandidateStatusDiscarded CandidateStatus = "discarded"
	CandidateStatusAccepted  CandidateStatus = "accepted"
)

type Provenance struct {
	ChunkIDs []string `yaml:"chunk_ids" json:"chunk_ids"`
}

type CanonicalRef struct {
	Kind string `yaml:"kind" json:"kind"`
	ID   string `yaml:"id" json:"id"`
}

type CandidateDecision struct {
	ReplacementCandidateID *string        `yaml:"replacement_candidate_id"`
	CanonicalRefs          []CanonicalRef `yaml:"canonical_refs"`
}

type CodexProposal struct {
	Type        codex.EntryType `yaml:"type" json:"type"`
	Name        string          `yaml:"name" json:"name"`
	Aliases     []string        `yaml:"aliases" json:"aliases"`
	Tags        []string        `yaml:"tags" json:"tags"`
	Description string          `yaml:"description" json:"description"`
}

type ArcProposal struct {
	Title string `yaml:"title" json:"title"`
}

type ChapterProposal struct {
	Title             string `yaml:"title" json:"title"`
	ParentCandidateID string `yaml:"parent_candidate_id" json:"parent_candidate_id"`
}

type SceneProposal struct {
	Title             string `yaml:"title" json:"title"`
	ParentCandidateID string `yaml:"parent_candidate_id" json:"parent_candidate_id"`
}

type CandidateProposal struct {
	Codex   *CodexProposal   `yaml:"codex"`
	Arc     *ArcProposal     `yaml:"arc"`
	Chapter *ChapterProposal `yaml:"chapter"`
	Scene   *SceneProposal   `yaml:"scene"`
}

type Candidate struct {
	Version         int               `yaml:"version"`
	ID              string            `yaml:"id"`
	Kind            CandidateKind     `yaml:"kind"`
	ProposalVersion int               `yaml:"proposal_version"`
	Status          CandidateStatus   `yaml:"status"`
	Revision        string            `yaml:"revision"`
	Provenance      Provenance        `yaml:"provenance"`
	Proposal        CandidateProposal `yaml:"proposal"`
	Decision        CandidateDecision `yaml:"decision"`
}

var (
	ErrInvalidCandidate   = errors.New("invalid import candidate")
	ErrCandidateConflict  = errors.New("stale import candidate revision")
	ErrCandidateTerminal  = errors.New("candidate is not pending")
	ErrNoCandidateChanges = errors.New("candidate has no changes")
	ErrParentNotAccepted  = errors.New("parent candidate is not accepted")
)

type candidateHandler interface {
	Normalize(candidate Candidate) (Candidate, error)
}

type candidateRegistry struct {
	handlers map[string]candidateHandler
}

func newCandidateRegistry() candidateRegistry {
	registry := candidateRegistry{handlers: map[string]candidateHandler{}}
	registry.register(CandidateKindCodex, 1, candidateCodexHandler{})
	registry.register(CandidateKindArc, 1, candidateArcHandler{})
	registry.register(CandidateKindChapter, 1, candidateChapterHandler{})
	registry.register(CandidateKindScene, 1, candidateSceneHandler{})
	return registry
}

func (r candidateRegistry) register(kind CandidateKind, version int, handler candidateHandler) {
	r.handlers[candidateKey(kind, version)] = handler
}

func (r candidateRegistry) normalize(candidate Candidate) (Candidate, error) {
	handler, ok := r.handlers[candidateKey(candidate.Kind, candidate.ProposalVersion)]
	if !ok {
		return Candidate{}, fmt.Errorf("candidate kind %q proposal version %d is unsupported: %w", candidate.Kind, candidate.ProposalVersion, ErrInvalidCandidate)
	}
	return handler.Normalize(candidate)
}

func candidateKey(kind CandidateKind, version int) string {
	return string(kind) + ":" + fmt.Sprint(version)
}

var defaultCandidateRegistry = newCandidateRegistry()

func NormalizeCandidate(candidate Candidate) (Candidate, error) {
	if candidate.Version != CandidateVersion {
		return Candidate{}, fmt.Errorf("candidate version %d is unsupported: %w", candidate.Version, ErrInvalidCandidate)
	}
	if err := ValidateCandidateID(candidate.ID); err != nil {
		return Candidate{}, err
	}
	switch candidate.Status {
	case CandidateStatusPending, CandidateStatusMerged, CandidateStatusDiscarded, CandidateStatusAccepted:
	default:
		return Candidate{}, fmt.Errorf("candidate status %q is invalid: %w", candidate.Status, ErrInvalidCandidate)
	}
	if candidate.ProposalVersion < 1 {
		return Candidate{}, fmt.Errorf("candidate proposal version %d is invalid: %w", candidate.ProposalVersion, ErrInvalidCandidate)
	}
	if len(candidate.Provenance.ChunkIDs) == 0 || len(candidate.Provenance.ChunkIDs) > 100 {
		return Candidate{}, fmt.Errorf("candidate provenance count is invalid for status %q: %w", candidate.Status, ErrInvalidCandidate)
	}
	candidate.Provenance.ChunkIDs = append([]string(nil), candidate.Provenance.ChunkIDs...)
	slices.Sort(candidate.Provenance.ChunkIDs)
	candidate.Provenance.ChunkIDs = slices.Compact(candidate.Provenance.ChunkIDs)
	for _, chunkID := range candidate.Provenance.ChunkIDs {
		if err := ValidateChunkID(chunkID); err != nil {
			return Candidate{}, err
		}
	}
	normalized, err := defaultCandidateRegistry.normalize(candidate)
	if err != nil {
		return Candidate{}, err
	}
	expectedRevision, err := ComputeCandidateRevision(normalized)
	if err != nil {
		return Candidate{}, err
	}
	if normalized.Revision == "" {
		normalized.Revision = expectedRevision
	} else if normalized.Revision != expectedRevision {
		return Candidate{}, fmt.Errorf("candidate revision does not match canonical content: %w", ErrInvalidCandidate)
	}
	return normalized, nil
}

func ComputeCandidateRevision(candidate Candidate) (string, error) {
	clone := candidate
	clone.Revision = ""
	body, err := yaml.Marshal(clone)
	if err != nil {
		return "", fmt.Errorf("marshal candidate revision: %w", err)
	}
	digest := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func CandidateSortOrder(status CandidateStatus) int {
	switch status {
	case CandidateStatusPending:
		return 0
	case CandidateStatusMerged:
		return 1
	case CandidateStatusDiscarded:
		return 2
	case CandidateStatusAccepted:
		return 3
	default:
		return 4
	}
}

type CandidateStore struct{}

func NewCandidateStore() *CandidateStore {
	return &CandidateStore{}
}

func (s *CandidateStore) Create(projectPath string, candidate Candidate) (Candidate, error) {
	normalized, err := NormalizeCandidate(candidate)
	if err != nil {
		return Candidate{}, err
	}
	path := candidatePath(projectPath, normalized.ID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Candidate{}, fmt.Errorf("create candidate directory: %w", err)
	}
	body, err := yaml.Marshal(normalized)
	if err != nil {
		return Candidate{}, fmt.Errorf("marshal candidate: %w", err)
	}
	temporaryPath := path + ".tmp"
	if err := os.WriteFile(temporaryPath, body, 0o644); err != nil {
		return Candidate{}, fmt.Errorf("write candidate: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return Candidate{}, fmt.Errorf("replace candidate: %w", err)
	}
	return normalized, nil
}

func (s *CandidateStore) Load(projectPath, candidateID string) (Candidate, error) {
	if err := ValidateCandidateID(candidateID); err != nil {
		return Candidate{}, err
	}
	body, err := os.ReadFile(candidatePath(projectPath, candidateID))
	if err != nil {
		return Candidate{}, err
	}
	var candidate Candidate
	if err := yaml.Unmarshal(body, &candidate); err != nil {
		return Candidate{}, fmt.Errorf("decode candidate: %w", err)
	}
	return NormalizeCandidate(candidate)
}

func (s *CandidateStore) List(projectPath string) ([]Candidate, error) {
	entries, err := os.ReadDir(filepath.Join(projectPath, "imports", "review"))
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("list candidates: %w", err)
	}
	candidates := []Candidate{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		candidate, err := s.Load(projectPath, strings.TrimSuffix(entry.Name(), ".yaml"))
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}
	slices.SortFunc(candidates, func(left, right Candidate) int {
		if CandidateSortOrder(left.Status) != CandidateSortOrder(right.Status) {
			if CandidateSortOrder(left.Status) < CandidateSortOrder(right.Status) {
				return -1
			}
			return 1
		}
		if left.ID < right.ID {
			return -1
		}
		if left.ID > right.ID {
			return 1
		}
		return 0
	})
	return candidates, nil
}

func (s *CandidateStore) Replace(projectPath string, candidate Candidate, expectedRevision string) (Candidate, error) {
	current, err := s.Load(projectPath, candidate.ID)
	if err != nil {
		return Candidate{}, err
	}
	if current.Revision != expectedRevision {
		return Candidate{}, ErrCandidateConflict
	}
	return s.Create(projectPath, candidate)
}

func candidatePath(projectPath, candidateID string) string {
	return filepath.Join(projectPath, "imports", "review", candidateID+".yaml")
}

type candidateCodexHandler struct{}

func (candidateCodexHandler) Normalize(candidate Candidate) (Candidate, error) {
	if candidate.Kind != CandidateKindCodex {
		return Candidate{}, ErrInvalidCandidate
	}
	if candidate.Proposal.Codex == nil || candidate.Proposal.Arc != nil || candidate.Proposal.Chapter != nil || candidate.Proposal.Scene != nil {
		return Candidate{}, fmt.Errorf("candidate proposal must contain exactly one codex payload: %w", ErrInvalidCandidate)
	}
	aliases := normalizeSortedUniqueStrings(candidate.Proposal.Codex.Aliases)
	tags := normalizeSortedUniqueStrings(candidate.Proposal.Codex.Tags)
	request, err := codex.NormalizeCreateRequest(codex.SaveEntryRequest{
		Type:        candidate.Proposal.Codex.Type,
		Name:        candidate.Proposal.Codex.Name,
		Aliases:     aliases,
		Tags:        tags,
		Description: candidate.Proposal.Codex.Description,
	})
	if err != nil {
		return Candidate{}, err
	}
	candidate.Proposal.Codex = &CodexProposal{
		Type:        request.Type,
		Name:        request.Name,
		Aliases:     request.Aliases,
		Tags:        request.Tags,
		Description: request.Description,
	}
	return normalizeCandidateDecision(candidate)
}

type candidateArcHandler struct{}

func (candidateArcHandler) Normalize(candidate Candidate) (Candidate, error) {
	if candidate.Proposal.Arc == nil || candidate.Proposal.Codex != nil || candidate.Proposal.Chapter != nil || candidate.Proposal.Scene != nil {
		return Candidate{}, fmt.Errorf("candidate proposal must contain exactly one arc payload: %w", ErrInvalidCandidate)
	}
	title, err := story.ValidateTitle(candidate.Proposal.Arc.Title)
	if err != nil {
		return Candidate{}, err
	}
	candidate.Proposal.Arc = &ArcProposal{Title: title}
	return normalizeCandidateDecision(candidate)
}

type candidateChapterHandler struct{}

func (candidateChapterHandler) Normalize(candidate Candidate) (Candidate, error) {
	if candidate.Proposal.Chapter == nil || candidate.Proposal.Codex != nil || candidate.Proposal.Arc != nil || candidate.Proposal.Scene != nil {
		return Candidate{}, fmt.Errorf("candidate proposal must contain exactly one chapter payload: %w", ErrInvalidCandidate)
	}
	title, err := story.ValidateTitle(candidate.Proposal.Chapter.Title)
	if err != nil {
		return Candidate{}, err
	}
	if err := ValidateCandidateID(candidate.Proposal.Chapter.ParentCandidateID); err != nil {
		return Candidate{}, err
	}
	candidate.Proposal.Chapter = &ChapterProposal{Title: title, ParentCandidateID: candidate.Proposal.Chapter.ParentCandidateID}
	return normalizeCandidateDecision(candidate)
}

type candidateSceneHandler struct{}

func (candidateSceneHandler) Normalize(candidate Candidate) (Candidate, error) {
	if candidate.Proposal.Scene == nil || candidate.Proposal.Codex != nil || candidate.Proposal.Arc != nil || candidate.Proposal.Chapter != nil {
		return Candidate{}, fmt.Errorf("candidate proposal must contain exactly one scene payload: %w", ErrInvalidCandidate)
	}
	title, err := story.ValidateTitle(candidate.Proposal.Scene.Title)
	if err != nil {
		return Candidate{}, err
	}
	if err := ValidateCandidateID(candidate.Proposal.Scene.ParentCandidateID); err != nil {
		return Candidate{}, err
	}
	candidate.Proposal.Scene = &SceneProposal{Title: title, ParentCandidateID: candidate.Proposal.Scene.ParentCandidateID}
	return normalizeCandidateDecision(candidate)
}

func normalizeCandidateDecision(candidate Candidate) (Candidate, error) {
	switch candidate.Status {
	case CandidateStatusPending:
		candidate.Decision.ReplacementCandidateID = nil
		candidate.Decision.CanonicalRefs = []CanonicalRef{}
	case CandidateStatusMerged:
		if candidate.Decision.ReplacementCandidateID == nil {
			return Candidate{}, fmt.Errorf("merged candidate must record replacement ID: %w", ErrInvalidCandidate)
		}
		if err := ValidateCandidateID(*candidate.Decision.ReplacementCandidateID); err != nil {
			return Candidate{}, err
		}
		candidate.Decision.CanonicalRefs = []CanonicalRef{}
	case CandidateStatusDiscarded:
		candidate.Decision.ReplacementCandidateID = nil
		candidate.Decision.CanonicalRefs = []CanonicalRef{}
	case CandidateStatusAccepted:
		candidate.Decision.ReplacementCandidateID = nil
		if len(candidate.Decision.CanonicalRefs) != 1 {
			return Candidate{}, fmt.Errorf("accepted candidate must record exactly one canonical ref: %w", ErrInvalidCandidate)
		}
	default:
		return Candidate{}, ErrInvalidCandidate
	}
	return candidate, nil
}

func normalizeSortedUniqueStrings(values []string) []string {
	normalized := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	slices.Sort(normalized)
	return normalized
}

func ValidateCandidateKind(value string) (CandidateKind, error) {
	switch CandidateKind(value) {
	case CandidateKindCodex, CandidateKindArc, CandidateKindChapter, CandidateKindScene:
		return CandidateKind(value), nil
	default:
		return "", ErrInvalidCandidate
	}
}

func ValidateCandidateStatus(value string) (CandidateStatus, error) {
	switch CandidateStatus(value) {
	case CandidateStatusPending, CandidateStatusMerged, CandidateStatusDiscarded, CandidateStatusAccepted:
		return CandidateStatus(value), nil
	default:
		return "", ErrInvalidCandidate
	}
}

func proposalsEqual(left, right CandidateProposal) bool {
	return codexProposalEqual(left.Codex, right.Codex) &&
		arcProposalEqual(left.Arc, right.Arc) &&
		chapterProposalEqual(left.Chapter, right.Chapter) &&
		sceneProposalEqual(left.Scene, right.Scene)
}

func codexProposalEqual(left, right *CodexProposal) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.Type == right.Type &&
		left.Name == right.Name &&
		left.Description == right.Description &&
		slices.Equal(left.Aliases, right.Aliases) &&
		slices.Equal(left.Tags, right.Tags)
}

func arcProposalEqual(left, right *ArcProposal) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.Title == right.Title
}

func chapterProposalEqual(left, right *ChapterProposal) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.Title == right.Title && left.ParentCandidateID == right.ParentCandidateID
}

func sceneProposalEqual(left, right *SceneProposal) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.Title == right.Title && left.ParentCandidateID == right.ParentCandidateID
}

func unionChunkIDs(left, right []string) []string {
	combined := append(append([]string(nil), left...), right...)
	slices.Sort(combined)
	return slices.Compact(combined)
}
