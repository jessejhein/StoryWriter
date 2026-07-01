package importer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"storywork/internal/agent"
	"storywork/internal/codex"
	"storywork/internal/extract"
	"storywork/internal/project"
	"storywork/internal/story"
)

type Session interface {
	Current() (project.Project, bool)
}

type GitStore interface {
	IsClean(ctx context.Context, path string) (bool, error)
	CommitAll(ctx context.Context, path, message string) error
	UnstageAll(ctx context.Context, path string) error
}

type IndexStore interface {
	Rebuild(ctx context.Context, projectPath string) error
}

type IDGenerator interface {
	NextImportID() (string, error)
	NextCandidateID() (string, error)
}

type StoryMutator interface {
	ApplyImportMutation(ctx context.Context, request story.ImportMutationRequest) (story.ImportMutationResult, error)
}

type ImportResponse struct {
	Import ImportSummary `json:"import"`
	Files  []ImportFile  `json:"files"`
}

type ExtractRequest struct {
	ImportID  string
	ChunkIDs  []string
	Mode      extract.Mode
	ProfileID string
	Model     string
}

type ExtractResponse struct {
	Candidates []Candidate
	Provider   agent.ProviderIdentity
}

type MergeRequest struct {
	OtherCandidateID      string
	ExpectedRevision      string
	OtherExpectedRevision string
	Proposal              CandidateProposal
}

type Service struct {
	session    Session
	git        GitStore
	index      IndexStore
	source     *SourceStore
	chunks     *ChunkStore
	candidates *CandidateStore
	extractor  extract.Extractor
	story      StoryMutator
	ids        IDGenerator
	now        func() time.Time
	locks      sync.Map
	claims     map[string]struct{}
}

func NewService(session Session, git GitStore, index IndexStore, source *SourceStore, ids IDGenerator, now func() time.Time) *Service {
	if source == nil {
		source = NewSourceStore()
	}
	if now == nil {
		now = time.Now
	}
	return &Service{
		session:    session,
		git:        git,
		index:      index,
		source:     source,
		chunks:     NewChunkStore(),
		candidates: NewCandidateStore(),
		ids:        ids,
		now:        now,
		claims:     map[string]struct{}{},
	}
}

func (s *Service) WithExtractor(extractor extract.Extractor) *Service {
	s.extractor = extractor
	return s
}

func (s *Service) WithStoryMutator(mutator StoryMutator) *Service {
	s.story = mutator
	return s
}

func (s *Service) ImportDirectory(ctx context.Context, sourceDirectory string) (ImportResponse, error) {
	current, err := s.currentProject()
	if err != nil {
		return ImportResponse{}, err
	}
	lock := s.projectLock(current.Path)
	lock.Lock()
	defer lock.Unlock()

	clean, err := s.git.IsClean(ctx, current.Path)
	if err != nil {
		return ImportResponse{}, err
	}
	if !clean {
		return ImportResponse{}, story.ErrDirtyWorktree
	}

	importID, err := s.reserveImportID(ctx, current.Path)
	if err != nil {
		return ImportResponse{}, err
	}
	prepared, err := s.source.PrepareSnapshot(ctx, PrepareSnapshotRequest{
		ProjectPath:     current.Path,
		SourceDirectory: sourceDirectory,
		ImportID:        importID,
		CreatedAt:       s.now().UTC(),
	})
	if err != nil {
		return ImportResponse{}, err
	}
	rollback, err := prepared.Publish()
	if err != nil {
		return ImportResponse{}, err
	}
	if err := s.index.Rebuild(ctx, current.Path); err != nil {
		return ImportResponse{}, s.rollbackMutation(ctx, current.Path, rollback, err)
	}
	if err := s.git.CommitAll(ctx, current.Path, "Import notes snapshot "+importID); err != nil {
		return ImportResponse{}, s.rollbackMutation(ctx, current.Path, rollback, err)
	}
	return ImportResponse{
		Import: prepared.Manifest().Summary(),
		Files:  prepared.Files(),
	}, nil
}

func (s *Service) ListImports(ctx context.Context) ([]ImportSummary, error) {
	current, err := s.currentProject()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(filepath.Join(current.Path, "imports", "raw"))
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("list imports: %w", err)
	}
	summaries := []ImportSummary{}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		manifest, err := loadManifest(filepath.Join(current.Path, "imports", "raw", entry.Name(), "manifest.yaml"))
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, manifest.Summary())
	}
	slices.SortFunc(summaries, func(left, right ImportSummary) int {
		if left.CreatedAt != right.CreatedAt {
			if left.CreatedAt < right.CreatedAt {
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
	return summaries, nil
}

func (s *Service) ListChunks(ctx context.Context, importID string) ([]Chunk, error) {
	current, err := s.currentProject()
	if err != nil {
		return nil, err
	}
	return s.chunks.ListOrRebuild(ctx, current.Path, importID)
}

func (s *Service) Extract(ctx context.Context, request ExtractRequest) (ExtractResponse, error) {
	if s.extractor == nil {
		return ExtractResponse{}, fmt.Errorf("extractor is not configured: %w", extract.ErrInvalidRequest)
	}
	current, err := s.currentProject()
	if err != nil {
		return ExtractResponse{}, err
	}
	lock := s.projectLock(current.Path)
	lock.Lock()
	defer lock.Unlock()

	clean, err := s.git.IsClean(ctx, current.Path)
	if err != nil {
		return ExtractResponse{}, err
	}
	if !clean {
		return ExtractResponse{}, story.ErrDirtyWorktree
	}
	availableChunks, err := s.chunks.ListOrRebuild(ctx, current.Path, request.ImportID)
	if err != nil {
		return ExtractResponse{}, err
	}
	chunkByID := make(map[string]Chunk, len(availableChunks))
	for _, chunk := range availableChunks {
		chunkByID[chunk.ID] = chunk
	}
	selectedIDs := append([]string(nil), request.ChunkIDs...)
	slices.Sort(selectedIDs)
	selectedIDs = slices.Compact(selectedIDs)
	if len(selectedIDs) == 0 {
		return ExtractResponse{}, extract.ErrInvalidRequest
	}
	selectedChunks := make([]extract.Chunk, 0, len(selectedIDs))
	for _, chunkID := range selectedIDs {
		chunk, ok := chunkByID[chunkID]
		if !ok {
			return ExtractResponse{}, extract.ErrInvalidRequest
		}
		selectedChunks = append(selectedChunks, extract.Chunk{
			ID:         chunk.ID,
			ImportID:   chunk.ImportID,
			SourcePath: chunk.SourcePath,
			StartLine:  chunk.StartLine,
			EndLine:    chunk.EndLine,
			Text:       chunk.Text,
		})
	}
	result, err := s.extractor.Extract(ctx, extract.Request{
		Chunks:    selectedChunks,
		Mode:      request.Mode,
		ProfileID: request.ProfileID,
		Model:     request.Model,
	})
	if err != nil {
		return ExtractResponse{}, err
	}
	candidates, err := s.buildCandidatesFromProposals(request.ImportID, selectedIDs, result.Proposals)
	if err != nil {
		return ExtractResponse{}, err
	}
	createdIDs := []string{}
	for _, candidate := range candidates {
		if _, err := s.candidates.Create(current.Path, candidate); err != nil {
			_ = rollbackCandidates(current.Path, createdIDs)
			return ExtractResponse{}, err
		}
		createdIDs = append(createdIDs, candidate.ID)
	}
	rollback := func() error { return rollbackCandidates(current.Path, createdIDs) }
	if err := s.index.Rebuild(ctx, current.Path); err != nil {
		return ExtractResponse{}, s.rollbackMutation(ctx, current.Path, rollback, err)
	}
	if err := s.git.CommitAll(ctx, current.Path, "Extract import candidates "+request.ImportID); err != nil {
		return ExtractResponse{}, s.rollbackMutation(ctx, current.Path, rollback, err)
	}
	listed, err := s.candidates.List(current.Path)
	if err != nil {
		return ExtractResponse{}, err
	}
	return ExtractResponse{Candidates: listed, Provider: result.Provider}, nil
}

func (s *Service) ListCandidates(ctx context.Context) ([]Candidate, error) {
	current, err := s.currentProject()
	if err != nil {
		return nil, err
	}
	return s.candidates.List(current.Path)
}

func (s *Service) LoadCandidate(ctx context.Context, candidateID string) (Candidate, error) {
	current, err := s.currentProject()
	if err != nil {
		return Candidate{}, err
	}
	return s.candidates.Load(current.Path, candidateID)
}

func (s *Service) ListCandidatesFiltered(ctx context.Context, status *CandidateStatus, kind *CandidateKind) ([]Candidate, error) {
	candidates, err := s.ListCandidates(ctx)
	if err != nil {
		return nil, err
	}
	filtered := make([]Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		if status != nil && candidate.Status != *status {
			continue
		}
		if kind != nil && candidate.Kind != *kind {
			continue
		}
		filtered = append(filtered, candidate)
	}
	return filtered, nil
}

func (s *Service) UpdateCandidate(ctx context.Context, candidateID string, expectedRevision string, proposal CandidateProposal) (Candidate, error) {
	current, projectPath, err := s.claimPendingCandidate(ctx, candidateID, expectedRevision)
	if err != nil {
		return Candidate{}, err
	}
	defer s.releaseClaim(projectPath, candidateID)

	next := current
	next.Proposal = proposal
	next.Revision = ""
	normalized, err := NormalizeCandidate(next)
	if err != nil {
		return Candidate{}, err
	}
	if proposalsEqual(current.Proposal, normalized.Proposal) {
		return Candidate{}, ErrNoCandidateChanges
	}
	return s.replaceCandidateAndCommit(ctx, projectPath, "Edit import candidate "+candidateID, current, normalized)
}

func (s *Service) MergeCandidates(ctx context.Context, candidateID string, request MergeRequest) (Candidate, []string, error) {
	current, projectPath, err := s.claimPendingCandidate(ctx, candidateID, request.ExpectedRevision)
	if err != nil {
		return Candidate{}, nil, err
	}
	defer s.releaseClaim(projectPath, candidateID)
	other, err := s.claimAdditionalPendingCandidate(ctx, projectPath, request.OtherCandidateID, request.OtherExpectedRevision, candidateID)
	if err != nil {
		return Candidate{}, nil, err
	}
	defer s.releaseClaim(projectPath, request.OtherCandidateID)

	if current.Kind != CandidateKindCodex || other.Kind != CandidateKindCodex {
		return Candidate{}, nil, ErrInvalidCandidate
	}
	if current.Proposal.Codex.Type != other.Proposal.Codex.Type {
		return Candidate{}, nil, ErrInvalidCandidate
	}
	replacementID, err := s.ids.NextCandidateID()
	if err != nil {
		return Candidate{}, nil, err
	}
	replacement := Candidate{
		Version:         CandidateVersion,
		ID:              replacementID,
		Kind:            current.Kind,
		ProposalVersion: current.ProposalVersion,
		Status:          CandidateStatusPending,
		Provenance: Provenance{
			ChunkIDs: unionChunkIDs(current.Provenance.ChunkIDs, other.Provenance.ChunkIDs),
		},
		Proposal: request.Proposal,
		Decision: CandidateDecision{CanonicalRefs: []CanonicalRef{}},
	}
	replacement, err = NormalizeCandidate(replacement)
	if err != nil {
		return Candidate{}, nil, err
	}
	current.Status = CandidateStatusMerged
	current.Decision.ReplacementCandidateID = &replacement.ID
	current.Decision.CanonicalRefs = []CanonicalRef{}
	current.Revision = ""
	current, err = NormalizeCandidate(current)
	if err != nil {
		return Candidate{}, nil, err
	}
	other.Status = CandidateStatusMerged
	other.Decision.ReplacementCandidateID = &replacement.ID
	other.Decision.CanonicalRefs = []CanonicalRef{}
	other.Revision = ""
	other, err = NormalizeCandidate(other)
	if err != nil {
		return Candidate{}, nil, err
	}
	if err := s.replaceCandidatesAtomically(ctx, projectPath, "Merge import candidates "+candidateID+" "+request.OtherCandidateID, []Candidate{current, other, replacement}); err != nil {
		return Candidate{}, nil, err
	}
	return replacement, []string{candidateID, request.OtherCandidateID}, nil
}

func (s *Service) DiscardCandidate(ctx context.Context, candidateID, expectedRevision string) (Candidate, error) {
	current, projectPath, err := s.claimPendingCandidate(ctx, candidateID, expectedRevision)
	if err != nil {
		return Candidate{}, err
	}
	defer s.releaseClaim(projectPath, candidateID)
	current.Status = CandidateStatusDiscarded
	current.Decision.ReplacementCandidateID = nil
	current.Decision.CanonicalRefs = []CanonicalRef{}
	current.Revision = ""
	current, err = NormalizeCandidate(current)
	if err != nil {
		return Candidate{}, err
	}
	return s.replaceCandidateAndCommit(ctx, projectPath, "Discard import candidate "+candidateID, Candidate{ID: candidateID}, current)
}

func (s *Service) AcceptCandidate(ctx context.Context, candidateID, expectedRevision string) (Candidate, []CanonicalRef, error) {
	if s.story == nil {
		return Candidate{}, nil, fmt.Errorf("story mutator is not configured: %w", ErrInvalidCandidate)
	}
	current, projectPath, err := s.claimPendingCandidate(ctx, candidateID, expectedRevision)
	if err != nil {
		return Candidate{}, nil, err
	}
	defer s.releaseClaim(projectPath, candidateID)

	mutationRequest, err := s.acceptanceMutationRequest(ctx, projectPath, current)
	if err != nil {
		return Candidate{}, nil, err
	}
	mutationResult, err := s.story.ApplyImportMutation(ctx, mutationRequest)
	if err != nil {
		return Candidate{}, nil, err
	}
	canonicalRefs := []CanonicalRef{{Kind: string(mutationResult.Kind), ID: mutationResult.ID}}
	current.Status = CandidateStatusAccepted
	current.Decision.ReplacementCandidateID = nil
	current.Decision.CanonicalRefs = canonicalRefs
	current.Revision = ""
	current, err = NormalizeCandidate(current)
	if err != nil {
		if mutationResult.Rollback != nil {
			_ = mutationResult.Rollback()
		}
		return Candidate{}, nil, err
	}
	snapshot, err := snapshotPaths([]string{candidatePath(projectPath, candidateID)})
	if err != nil {
		if mutationResult.Rollback != nil {
			_ = mutationResult.Rollback()
		}
		return Candidate{}, nil, err
	}
	if _, err := s.candidates.Create(projectPath, current); err != nil {
		if mutationResult.Rollback != nil {
			_ = mutationResult.Rollback()
		}
		_ = restoreSnapshot(snapshot)
		return Candidate{}, nil, err
	}
	rollback := func() error {
		var errs []error
		if err := restoreSnapshot(snapshot); err != nil {
			errs = append(errs, err)
		}
		if mutationResult.Rollback != nil {
			if err := mutationResult.Rollback(); err != nil {
				errs = append(errs, err)
			}
		}
		return errors.Join(errs...)
	}
	if err := s.index.Rebuild(ctx, projectPath); err != nil {
		return Candidate{}, nil, s.rollbackMutation(ctx, projectPath, rollback, err)
	}
	if err := s.git.CommitAll(ctx, projectPath, "Accept import candidate "+candidateID); err != nil {
		return Candidate{}, nil, s.rollbackMutation(ctx, projectPath, rollback, err)
	}
	return current, canonicalRefs, nil
}

func (s *Service) rollbackMutation(ctx context.Context, projectPath string, rollback func() error, cause error) error {
	var joined []error
	joined = append(joined, cause)
	if rollback != nil {
		if err := rollback(); err != nil {
			joined = append(joined, err)
		}
	}
	if err := s.git.UnstageAll(ctx, projectPath); err != nil {
		joined = append(joined, err)
	}
	if err := s.index.Rebuild(ctx, projectPath); err != nil {
		joined = append(joined, err)
	}
	return errors.Join(joined...)
}

func (s *Service) reserveImportID(_ context.Context, projectPath string) (string, error) {
	for range 5 {
		nextID, err := s.ids.NextImportID()
		if err != nil {
			return "", err
		}
		if err := ValidateImportID(nextID); err != nil {
			return "", err
		}
		if _, err := s.source.fs.Lstat(filepath.Join(projectPath, "imports", "raw", nextID)); err == nil {
			continue
		}
		return nextID, nil
	}
	return "", fmt.Errorf("allocate import identifier: %w", ErrInvalidID)
}

func (s *Service) currentProject() (project.Project, error) {
	current, ok := s.session.Current()
	if !ok {
		return project.Project{}, story.ErrNoActiveProject
	}
	return current, nil
}

func (s *Service) projectLock(projectPath string) *sync.Mutex {
	lock, _ := s.locks.LoadOrStore(projectPath, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

func (s *Service) claimPendingCandidate(ctx context.Context, candidateID, expectedRevision string) (Candidate, string, error) {
	current, err := s.currentProject()
	if err != nil {
		return Candidate{}, "", err
	}
	lock := s.projectLock(current.Path)
	lock.Lock()
	defer lock.Unlock()

	if _, ok := s.claims[current.Path+":"+candidateID]; ok {
		return Candidate{}, "", ErrCandidateConflict
	}
	clean, err := s.git.IsClean(ctx, current.Path)
	if err != nil {
		return Candidate{}, "", err
	}
	if !clean {
		return Candidate{}, "", story.ErrDirtyWorktree
	}
	candidate, err := s.candidates.Load(current.Path, candidateID)
	if err != nil {
		return Candidate{}, "", err
	}
	if candidate.Status != CandidateStatusPending {
		return Candidate{}, "", ErrCandidateTerminal
	}
	if candidate.Revision != expectedRevision {
		return Candidate{}, "", ErrCandidateConflict
	}
	s.claims[current.Path+":"+candidateID] = struct{}{}
	return candidate, current.Path, nil
}

func (s *Service) claimAdditionalPendingCandidate(ctx context.Context, projectPath, candidateID, expectedRevision, primaryID string) (Candidate, error) {
	lock := s.projectLock(projectPath)
	lock.Lock()
	defer lock.Unlock()
	if candidateID == primaryID {
		return Candidate{}, ErrInvalidCandidate
	}
	if _, ok := s.claims[projectPath+":"+candidateID]; ok {
		return Candidate{}, ErrCandidateConflict
	}
	candidate, err := s.candidates.Load(projectPath, candidateID)
	if err != nil {
		return Candidate{}, err
	}
	if candidate.Status != CandidateStatusPending {
		return Candidate{}, ErrCandidateTerminal
	}
	if candidate.Revision != expectedRevision {
		return Candidate{}, ErrCandidateConflict
	}
	s.claims[projectPath+":"+candidateID] = struct{}{}
	return candidate, nil
}

func (s *Service) releaseClaim(projectPath, candidateID string) {
	lock := s.projectLock(projectPath)
	lock.Lock()
	defer lock.Unlock()
	delete(s.claims, projectPath+":"+candidateID)
}

func (s *Service) replaceCandidateAndCommit(ctx context.Context, projectPath, message string, current Candidate, next Candidate) (Candidate, error) {
	path := candidatePath(projectPath, next.ID)
	snapshot, err := snapshotPaths([]string{path})
	if err != nil {
		return Candidate{}, err
	}
	if _, err := s.candidates.Create(projectPath, next); err != nil {
		_ = restoreSnapshot(snapshot)
		return Candidate{}, err
	}
	rollback := func() error { return restoreSnapshot(snapshot) }
	if err := s.index.Rebuild(ctx, projectPath); err != nil {
		return Candidate{}, s.rollbackMutation(ctx, projectPath, rollback, err)
	}
	if err := s.git.CommitAll(ctx, projectPath, message); err != nil {
		return Candidate{}, s.rollbackMutation(ctx, projectPath, rollback, err)
	}
	return next, nil
}

func (s *Service) replaceCandidatesAtomically(ctx context.Context, projectPath, message string, candidates []Candidate) error {
	paths := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		paths = append(paths, candidatePath(projectPath, candidate.ID))
	}
	snapshot, err := snapshotPaths(paths)
	if err != nil {
		return err
	}
	for _, candidate := range candidates {
		if _, err := s.candidates.Create(projectPath, candidate); err != nil {
			_ = restoreSnapshot(snapshot)
			return err
		}
	}
	rollback := func() error { return restoreSnapshot(snapshot) }
	if err := s.index.Rebuild(ctx, projectPath); err != nil {
		return s.rollbackMutation(ctx, projectPath, rollback, err)
	}
	if err := s.git.CommitAll(ctx, projectPath, message); err != nil {
		return s.rollbackMutation(ctx, projectPath, rollback, err)
	}
	return nil
}

func (s *Service) acceptanceMutationRequest(ctx context.Context, projectPath string, candidate Candidate) (story.ImportMutationRequest, error) {
	switch candidate.Kind {
	case CandidateKindArc:
		return story.ImportMutationRequest{Kind: story.ImportMutationArc, Title: candidate.Proposal.Arc.Title}, nil
	case CandidateKindChapter:
		parent, err := s.candidates.Load(projectPath, candidate.Proposal.Chapter.ParentCandidateID)
		if err != nil {
			return story.ImportMutationRequest{}, err
		}
		if parent.Status != CandidateStatusAccepted || parent.Kind != CandidateKindArc || len(parent.Decision.CanonicalRefs) != 1 {
			return story.ImportMutationRequest{}, ErrParentNotAccepted
		}
		return story.ImportMutationRequest{Kind: story.ImportMutationChapter, ParentID: parent.Decision.CanonicalRefs[0].ID, Title: candidate.Proposal.Chapter.Title}, nil
	case CandidateKindScene:
		parent, err := s.candidates.Load(projectPath, candidate.Proposal.Scene.ParentCandidateID)
		if err != nil {
			return story.ImportMutationRequest{}, err
		}
		if parent.Status != CandidateStatusAccepted || parent.Kind != CandidateKindChapter || len(parent.Decision.CanonicalRefs) != 1 {
			return story.ImportMutationRequest{}, ErrParentNotAccepted
		}
		return story.ImportMutationRequest{Kind: story.ImportMutationScene, ParentID: parent.Decision.CanonicalRefs[0].ID, Title: candidate.Proposal.Scene.Title}, nil
	case CandidateKindCodex:
		return story.ImportMutationRequest{
			Kind: story.ImportMutationCodex,
			Codex: codex.SaveEntryRequest{
				Type:        candidate.Proposal.Codex.Type,
				Name:        candidate.Proposal.Codex.Name,
				Aliases:     candidate.Proposal.Codex.Aliases,
				Tags:        candidate.Proposal.Codex.Tags,
				Description: candidate.Proposal.Codex.Description,
			},
		}, nil
	default:
		return story.ImportMutationRequest{}, ErrInvalidCandidate
	}
}

func (s *Service) buildCandidatesFromProposals(importID string, chunkIDs []string, proposals []extract.Proposal) ([]Candidate, error) {
	candidateIDs := make(map[string]string, len(proposals))
	proposalKinds := make(map[string]string, len(proposals))
	for _, proposal := range proposals {
		localID := proposalLocalID(proposal)
		nextID, err := s.ids.NextCandidateID()
		if err != nil {
			return nil, err
		}
		if err := ValidateCandidateID(nextID); err != nil {
			return nil, err
		}
		candidateIDs[localID] = nextID
		proposalKinds[localID] = proposal.Kind
	}
	candidates := make([]Candidate, 0, len(proposals))
	for _, proposal := range proposals {
		candidate := Candidate{
			Version:         CandidateVersion,
			ID:              candidateIDs[proposalLocalID(proposal)],
			ProposalVersion: 1,
			Status:          CandidateStatusPending,
			Provenance:      Provenance{ChunkIDs: append([]string(nil), chunkIDs...)},
			Decision:        CandidateDecision{CanonicalRefs: []CanonicalRef{}},
		}
		switch proposal.Kind {
		case "codex":
			candidate.Kind = CandidateKindCodex
			candidate.Proposal.Codex = &CodexProposal{
				Type:        codex.EntryType(proposal.Codex.Type),
				Name:        proposal.Codex.Name,
				Aliases:     proposal.Codex.Aliases,
				Tags:        proposal.Codex.Tags,
				Description: proposal.Codex.Description,
			}
		case "arc":
			candidate.Kind = CandidateKindArc
			candidate.Proposal.Arc = &ArcProposal{Title: proposal.Arc.Title}
		case "chapter":
			if proposalKinds[proposal.Chapter.ParentLocalID] != "arc" {
				return nil, extract.ErrInvalidResponse
			}
			candidate.Kind = CandidateKindChapter
			candidate.Proposal.Chapter = &ChapterProposal{
				Title:             proposal.Chapter.Title,
				ParentCandidateID: candidateIDs[proposal.Chapter.ParentLocalID],
			}
		case "scene":
			if proposalKinds[proposal.Scene.ParentLocalID] != "chapter" {
				return nil, extract.ErrInvalidResponse
			}
			candidate.Kind = CandidateKindScene
			candidate.Proposal.Scene = &SceneProposal{
				Title:             proposal.Scene.Title,
				ParentCandidateID: candidateIDs[proposal.Scene.ParentLocalID],
			}
		default:
			return nil, extract.ErrInvalidResponse
		}
		normalized, err := NormalizeCandidate(candidate)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, normalized)
	}
	return candidates, nil
}

func proposalLocalID(proposal extract.Proposal) string {
	switch proposal.Kind {
	case "codex":
		return proposal.Codex.LocalID
	case "arc":
		return proposal.Arc.LocalID
	case "chapter":
		return proposal.Chapter.LocalID
	case "scene":
		return proposal.Scene.LocalID
	default:
		return ""
	}
}

func rollbackCandidates(projectPath string, candidateIDs []string) error {
	var joined []error
	for _, candidateID := range candidateIDs {
		if err := os.Remove(candidatePath(projectPath, candidateID)); err != nil && !os.IsNotExist(err) {
			joined = append(joined, err)
		}
	}
	return errors.Join(joined...)
}

type fileSnapshot struct {
	path    string
	exists  bool
	content []byte
}

func snapshotPaths(paths []string) ([]fileSnapshot, error) {
	snapshots := make([]fileSnapshot, 0, len(paths))
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				snapshots = append(snapshots, fileSnapshot{path: path})
				continue
			}
			return nil, err
		}
		snapshots = append(snapshots, fileSnapshot{path: path, exists: true, content: content})
	}
	return snapshots, nil
}

func restoreSnapshot(snapshots []fileSnapshot) error {
	var errs []error
	for _, snapshot := range snapshots {
		if snapshot.exists {
			if err := os.MkdirAll(filepath.Dir(snapshot.path), 0o755); err != nil {
				errs = append(errs, err)
				continue
			}
			if err := os.WriteFile(snapshot.path, snapshot.content, 0o644); err != nil {
				errs = append(errs, err)
			}
			continue
		}
		if err := os.Remove(snapshot.path); err != nil && !os.IsNotExist(err) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
