package importer

import (
	"context"
	"errors"
	"testing"
	"time"

	"storywork/internal/project"
	"storywork/internal/story"
)

func TestImportServiceRequiresActiveCleanProject(t *testing.T) {
	t.Parallel()

	service := NewService(fakeImporterSession{}, &importerGitStub{}, &importerIndexStub{}, NewSourceStore(), fakeImporterIDs{}, time.Now)
	if _, err := service.ImportDirectory(context.Background(), "/tmp/notes"); !errors.Is(err, story.ErrNoActiveProject) {
		t.Fatalf("ImportDirectory() error = %v, want %v", err, story.ErrNoActiveProject)
	}

	service = NewService(fakeImporterSession{current: project.Project{Path: t.TempDir()}, ok: true}, &importerGitStub{clean: false}, &importerIndexStub{}, NewSourceStore(), fakeImporterIDs{}, time.Now)
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
		fakeImporterIDs{importIDs: []string{"imp_0123456789abcdef0123"}},
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

type fakeImporterSession struct {
	current project.Project
	ok      bool
}

func (s fakeImporterSession) Current() (project.Project, bool) {
	return s.current, s.ok
}

type importerGitStub struct {
	clean          bool
	commitMessages []string
}

func (s importerGitStub) IsClean(context.Context, string) (bool, error) {
	return s.clean, nil
}

func (s *importerGitStub) CommitAll(_ context.Context, _ string, message string) error {
	s.commitMessages = append(s.commitMessages, message)
	return nil
}

func (s importerGitStub) UnstageAll(context.Context, string) error {
	return nil
}

type importerIndexStub struct {
	rebuildCalls int
}

func (s *importerIndexStub) Rebuild(context.Context, string) error {
	s.rebuildCalls++
	return nil
}

type fakeImporterIDs struct {
	importIDs []string
	index     int
}

func (g fakeImporterIDs) NextImportID() (string, error) {
	if g.index >= len(g.importIDs) {
		return "", errors.New("no import IDs left")
	}
	return g.importIDs[g.index], nil
}

func (g fakeImporterIDs) NextCandidateID() (string, error) {
	return "cand_0123456789abcdef0123", nil
}
