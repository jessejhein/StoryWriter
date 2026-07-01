package importer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"storywork/internal/agent"
	"storywork/internal/extract"
	"storywork/internal/project"
	"storywork/internal/story"
)

func TestImportServiceRequiresActiveCleanProject(t *testing.T) {
	t.Parallel()

	service := NewService(fakeImporterSession{}, &importerGitStub{}, &importerIndexStub{}, NewSourceStore(), &fakeImporterIDs{}, time.Now)
	if _, err := service.ImportDirectory(context.Background(), "/tmp/notes"); !errors.Is(err, story.ErrNoActiveProject) {
		t.Fatalf("ImportDirectory() error = %v, want %v", err, story.ErrNoActiveProject)
	}

	service = NewService(fakeImporterSession{current: project.Project{Path: t.TempDir()}, ok: true}, &importerGitStub{clean: false}, &importerIndexStub{}, NewSourceStore(), &fakeImporterIDs{}, time.Now)
	if _, err := service.ImportDirectory(context.Background(), t.TempDir()); !errors.Is(err, story.ErrDirtyWorktree) {
		t.Fatalf("ImportDirectory() dirty error = %v, want %v", err, story.ErrDirtyWorktree)
	}
}

func TestImportServicePublishesSnapshotRebuildsIndexAndCommits(t *testing.T) {
	t.Parallel()

	projectPath := t.TempDir()
	sourcePath := t.TempDir()
	writeTestFile(t, sourcePath+"/notes.md", "Alpha")

	git := &importerGitStub{clean: true}
	index := &importerIndexStub{}
	service := NewService(
		fakeImporterSession{current: project.Project{Path: projectPath}, ok: true},
		git,
		index,
		NewSourceStore(),
		&fakeImporterIDs{importIDs: []string{"imp_0123456789abcdef0123"}},
		func() time.Time { return time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC) },
	)
	response, err := service.ImportDirectory(context.Background(), sourcePath)
	if err != nil {
		t.Fatalf("ImportDirectory() error = %v", err)
	}
	if response.Import.ID != "imp_0123456789abcdef0123" {
		t.Fatalf("ImportDirectory() import id = %q", response.Import.ID)
	}
	if index.rebuildCalls != 1 {
		t.Fatalf("index rebuild calls = %d, want 1", index.rebuildCalls)
	}
	if len(git.commitMessages) != 1 || git.commitMessages[0] != "Import notes snapshot imp_0123456789abcdef0123" {
		t.Fatalf("commit messages = %v", git.commitMessages)
	}
}

func TestExtractPublishesValidatedCandidatesWithoutCanonMutation(t *testing.T) {
	t.Parallel()

	projectPath := t.TempDir()
	sourcePath := t.TempDir()
	writeTestFile(t, sourcePath+"/notes.md", "# Act One\nMara arrives.\n")

	git := &importerGitStub{clean: true}
	index := &importerIndexStub{}
	ids := &fakeImporterIDs{
		importIDs:    []string{"imp_0123456789abcdef0123"},
		candidateIDs: []string{"cand_0123456789abcdef0101", "cand_0123456789abcdef0102", "cand_0123456789abcdef0103"},
	}
	extractor := fakeExtractor{result: extract.Result{
		Proposals: []extract.Proposal{
			{Kind: "arc", Arc: &extract.ArcProposal{Kind: "arc", LocalID: "arc_local", Title: "Act One"}},
			{Kind: "chapter", Chapter: &extract.ChapterProposal{Kind: "chapter", LocalID: "chapter_local", Title: "Arrival", ParentLocalID: "arc_local"}},
			{Kind: "codex", Codex: &extract.CodexProposal{Kind: "codex", LocalID: "codex_local", Type: "character", Name: "Mara Venn", Aliases: []string{"Mara"}, Tags: []string{"pilot"}, Description: "A cautious salvage pilot."}},
		},
		Provider: agent.ProviderIdentity{ProfileID: "local_ollama", Type: "ollama", Model: "qwen2.5:7b"},
	}}
	service := NewService(
		fakeImporterSession{current: project.Project{Path: projectPath}, ok: true},
		git,
		index,
		NewSourceStore(),
		ids,
		func() time.Time { return time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC) },
	).WithExtractor(&extractor)

	imported, err := service.ImportDirectory(context.Background(), sourcePath)
	if err != nil {
		t.Fatalf("ImportDirectory() error = %v", err)
	}
	chunks, err := service.ListChunks(context.Background(), imported.Import.ID)
	if err != nil {
		t.Fatalf("ListChunks() error = %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("ListChunks() count = %d, want 1", len(chunks))
	}

	response, err := service.Extract(context.Background(), ExtractRequest{
		ImportID:  imported.Import.ID,
		ChunkIDs:  []string{chunks[0].ID},
		Mode:      extract.ModeStructure,
		ProfileID: "local_ollama",
		Model:     "qwen2.5:7b",
	})
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if response.Provider.ProfileID != "local_ollama" || response.Provider.Type != "ollama" || response.Provider.Model != "qwen2.5:7b" {
		t.Fatalf("Extract() provider = %+v", response.Provider)
	}
	if len(response.Candidates) != 3 {
		t.Fatalf("Extract() candidate count = %d, want 3", len(response.Candidates))
	}
	if len(extractor.requests) != 1 || len(extractor.requests[0].Chunks) != 1 || extractor.requests[0].Chunks[0].ID != chunks[0].ID {
		t.Fatalf("extractor requests = %+v", extractor.requests)
	}
	chapter := response.Candidates[1]
	if chapter.Kind != CandidateKindChapter || chapter.Proposal.Chapter.ParentCandidateID != "cand_0123456789abcdef0101" {
		t.Fatalf("chapter candidate = %#v", chapter)
	}
	if index.rebuildCalls != 2 {
		t.Fatalf("index rebuild calls = %d, want 2", index.rebuildCalls)
	}
	if len(git.commitMessages) != 2 || git.commitMessages[1] != "Extract import candidates imp_0123456789abcdef0123" {
		t.Fatalf("commit messages = %v", git.commitMessages)
	}
	for _, path := range []string{
		filepath.Join(projectPath, "arcs"),
		filepath.Join(projectPath, "chapters"),
		filepath.Join(projectPath, "scenes"),
		filepath.Join(projectPath, "codex"),
	} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("extract mutated canon path %s err=%v", path, err)
		}
	}
}

func TestReviewOperationsEditDiscardMergeAndAccept(t *testing.T) {
	t.Parallel()

	projectPath := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectPath, "imports", "review"), 0o755); err != nil {
		t.Fatalf("MkdirAll(review) error = %v", err)
	}
	git := &importerGitStub{clean: true}
	index := &importerIndexStub{}
	ids := &fakeImporterIDs{candidateIDs: []string{"cand_0123456789abcdef0999"}}
	service := NewService(
		fakeImporterSession{current: project.Project{Path: projectPath}, ok: true},
		git,
		index,
		NewSourceStore(),
		ids,
		time.Now,
	).WithStoryMutator(&fakeStoryMutator{result: story.ImportMutationResult{Kind: story.ImportMutationCodex, ID: "char_0123456789abcdef0123", Rollback: func() error { return nil }}})

	store := NewCandidateStore()
	seed, err := store.Create(projectPath, Candidate{
		Version:         CandidateVersion,
		ID:              "cand_0123456789abcdef0123",
		Kind:            CandidateKindCodex,
		ProposalVersion: 1,
		Status:          CandidateStatusPending,
		Provenance:      Provenance{ChunkIDs: []string{"chk_0123456789abcdef0123"}},
		Proposal: CandidateProposal{Codex: &CodexProposal{
			Type:        "character",
			Name:        "Mara Venn",
			Aliases:     []string{"Mara"},
			Tags:        []string{"pilot"},
			Description: "A cautious salvage pilot.",
		}},
		Decision: CandidateDecision{CanonicalRefs: []CanonicalRef{}},
	})
	if err != nil {
		t.Fatalf("Create(seed) error = %v", err)
	}

	edited, err := service.UpdateCandidate(context.Background(), seed.ID, seed.Revision, CandidateProposal{
		Codex: &CodexProposal{
			Type:        "character",
			Name:        "Mara Venn",
			Aliases:     []string{"Captain Mara"},
			Tags:        []string{"pilot"},
			Description: "Edited author text.",
		},
	})
	if err != nil {
		t.Fatalf("UpdateCandidate() error = %v", err)
	}
	if edited.Revision == seed.Revision || edited.Proposal.Codex.Description != "Edited author text." {
		t.Fatalf("edited candidate = %#v", edited)
	}

	other, err := store.Create(projectPath, Candidate{
		Version:         CandidateVersion,
		ID:              "cand_0123456789abcdef0456",
		Kind:            CandidateKindCodex,
		ProposalVersion: 1,
		Status:          CandidateStatusPending,
		Provenance:      Provenance{ChunkIDs: []string{"chk_0123456789abcdef0456"}},
		Proposal: CandidateProposal{Codex: &CodexProposal{
			Type:        "character",
			Name:        "Mara Venn",
			Aliases:     []string{"Mara"},
			Tags:        []string{"salvage"},
			Description: "Second source.",
		}},
		Decision: CandidateDecision{CanonicalRefs: []CanonicalRef{}},
	})
	if err != nil {
		t.Fatalf("Create(other) error = %v", err)
	}
	merged, mergedIDs, err := service.MergeCandidates(context.Background(), edited.ID, MergeRequest{
		OtherCandidateID:      other.ID,
		ExpectedRevision:      edited.Revision,
		OtherExpectedRevision: other.Revision,
		Proposal: CandidateProposal{Codex: &CodexProposal{
			Type:        "character",
			Name:        "Mara Venn",
			Aliases:     []string{"Mara"},
			Tags:        []string{"pilot"},
			Description: "Merged author text.",
		}},
	})
	if err != nil {
		t.Fatalf("MergeCandidates() error = %v", err)
	}
	if len(mergedIDs) != 2 || merged.Status != CandidateStatusPending {
		t.Fatalf("merge result = %#v ids=%v", merged, mergedIDs)
	}

	discardTarget, err := store.Create(projectPath, Candidate{
		Version:         CandidateVersion,
		ID:              "cand_0123456789abcdef0777",
		Kind:            CandidateKindArc,
		ProposalVersion: 1,
		Status:          CandidateStatusPending,
		Provenance:      Provenance{ChunkIDs: []string{"chk_0123456789abcdef0777"}},
		Proposal:        CandidateProposal{Arc: &ArcProposal{Title: "Act One"}},
		Decision:        CandidateDecision{CanonicalRefs: []CanonicalRef{}},
	})
	if err != nil {
		t.Fatalf("Create(discardTarget) error = %v", err)
	}
	discarded, err := service.DiscardCandidate(context.Background(), discardTarget.ID, discardTarget.Revision)
	if err != nil {
		t.Fatalf("DiscardCandidate() error = %v", err)
	}
	if discarded.Status != CandidateStatusDiscarded {
		t.Fatalf("discarded candidate = %#v", discarded)
	}

	accepted, refs, err := service.AcceptCandidate(context.Background(), merged.ID, merged.Revision)
	if err != nil {
		t.Fatalf("AcceptCandidate() error = %v", err)
	}
	if accepted.Status != CandidateStatusAccepted || len(refs) != 1 || refs[0].ID != "char_0123456789abcdef0123" {
		t.Fatalf("accepted candidate = %#v refs=%v", accepted, refs)
	}
}

type fakeImporterSession struct {
	current project.Project
	ok      bool
}

type fakeExtractor struct {
	result   extract.Result
	err      error
	requests []extract.Request
}

func (f *fakeExtractor) Extract(_ context.Context, request extract.Request) (extract.Result, error) {
	f.requests = append(f.requests, request)
	if f.err != nil {
		return extract.Result{}, f.err
	}
	return f.result, nil
}

func (s fakeImporterSession) Current() (project.Project, bool) {
	return s.current, s.ok
}

type importerGitStub struct {
	clean          bool
	commitMessages []string
	commitErr      error
	unstageErr     error
}

func (s importerGitStub) IsClean(context.Context, string) (bool, error) {
	return s.clean, nil
}

func (s *importerGitStub) CommitAll(_ context.Context, _ string, message string) error {
	s.commitMessages = append(s.commitMessages, message)
	return s.commitErr
}

func (s importerGitStub) UnstageAll(context.Context, string) error {
	return s.unstageErr
}

type importerIndexStub struct {
	rebuildCalls int
	rebuildErr   error
}

func (s *importerIndexStub) Rebuild(context.Context, string) error {
	s.rebuildCalls++
	return s.rebuildErr
}

type fakeImporterIDs struct {
	importIDs      []string
	candidateIDs   []string
	importIndex    int
	candidateIndex int
}

func (g *fakeImporterIDs) NextImportID() (string, error) {
	if g.importIndex >= len(g.importIDs) {
		return "", errors.New("no import IDs left")
	}
	next := g.importIDs[g.importIndex]
	g.importIndex++
	return next, nil
}

func (g *fakeImporterIDs) NextCandidateID() (string, error) {
	if g.candidateIndex >= len(g.candidateIDs) {
		return "", errors.New("no candidate IDs left")
	}
	next := g.candidateIDs[g.candidateIndex]
	g.candidateIndex++
	return next, nil
}

type fakeStoryMutator struct {
	result story.ImportMutationResult
	err    error
}

func (f *fakeStoryMutator) ApplyImportMutation(context.Context, story.ImportMutationRequest) (story.ImportMutationResult, error) {
	if f.err != nil {
		return story.ImportMutationResult{}, f.err
	}
	return f.result, nil
}
