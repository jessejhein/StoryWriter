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

type Service struct {
	session    Session
	git        GitStore
	index      IndexStore
	source     *SourceStore
	chunks     *ChunkStore
	candidates *CandidateStore
	extractor  extract.Extractor
	ids        IDGenerator
	now        func() time.Time
	locks      sync.Map
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
	}
}

func (s *Service) WithExtractor(extractor extract.Extractor) *Service {
	s.extractor = extractor
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

func (s *Service) Extract(ctx context.Context, request ExtractRequest) ([]Candidate, error) {
	if s.extractor == nil {
		return nil, fmt.Errorf("extractor is not configured: %w", extract.ErrInvalidRequest)
	}
	current, err := s.currentProject()
	if err != nil {
		return nil, err
	}
	lock := s.projectLock(current.Path)
	lock.Lock()
	defer lock.Unlock()

	clean, err := s.git.IsClean(ctx, current.Path)
	if err != nil {
		return nil, err
	}
	if !clean {
		return nil, story.ErrDirtyWorktree
	}
	availableChunks, err := s.chunks.ListOrRebuild(ctx, current.Path, request.ImportID)
	if err != nil {
		return nil, err
	}
	chunkByID := make(map[string]Chunk, len(availableChunks))
	for _, chunk := range availableChunks {
		chunkByID[chunk.ID] = chunk
	}
	selectedIDs := append([]string(nil), request.ChunkIDs...)
	slices.Sort(selectedIDs)
	selectedIDs = slices.Compact(selectedIDs)
	if len(selectedIDs) == 0 {
		return nil, extract.ErrInvalidRequest
	}
	selectedChunks := make([]extract.Chunk, 0, len(selectedIDs))
	for _, chunkID := range selectedIDs {
		chunk, ok := chunkByID[chunkID]
		if !ok {
			return nil, extract.ErrInvalidRequest
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
		return nil, err
	}
	candidates, err := s.buildCandidatesFromProposals(request.ImportID, selectedIDs, result.Proposals)
	if err != nil {
		return nil, err
	}
	createdIDs := []string{}
	for _, candidate := range candidates {
		if _, err := s.candidates.Create(current.Path, candidate); err != nil {
			_ = rollbackCandidates(current.Path, createdIDs)
			return nil, err
		}
		createdIDs = append(createdIDs, candidate.ID)
	}
	rollback := func() error { return rollbackCandidates(current.Path, createdIDs) }
	if err := s.index.Rebuild(ctx, current.Path); err != nil {
		return nil, s.rollbackMutation(ctx, current.Path, rollback, err)
	}
	if err := s.git.CommitAll(ctx, current.Path, "Extract import candidates "+request.ImportID); err != nil {
		return nil, s.rollbackMutation(ctx, current.Path, rollback, err)
	}
	return s.candidates.List(current.Path)
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
