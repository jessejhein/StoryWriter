package story

import (
	"context"
	"errors"

	"storywork/internal/project"
)

type fakeSession struct {
	current project.Project
	ok      bool
}

func (s *fakeSession) Current() (project.Project, bool) {
	return s.current, s.ok
}

type fakeIDGenerator struct {
	ids []string
	err error
}

func (g *fakeIDGenerator) Next(_ NodeKind) (string, error) {
	if g.err != nil {
		return "", g.err
	}
	if len(g.ids) == 0 {
		return "", errors.New("no test IDs remaining")
	}
	next := g.ids[0]
	g.ids = g.ids[1:]
	return next, nil
}

type fakeGitStore struct {
	clean          bool
	isCleanCalls   int
	commitCalls    int
	unstageCalls   int
	commitMessages []string
	commitErr      error
	unstageErr     error
}

func (g *fakeGitStore) IsClean(context.Context, string) (bool, error) {
	g.isCleanCalls++
	return g.clean, nil
}

func (g *fakeGitStore) CommitAll(_ context.Context, _ string, message string) error {
	g.commitCalls++
	g.commitMessages = append(g.commitMessages, message)
	return g.commitErr
}

func (g *fakeGitStore) UnstageAll(context.Context, string) error {
	g.unstageCalls++
	return g.unstageErr
}

type fakeIndexStore struct {
	rebuildCalls int
	rebuildErr   error
}

func (i *fakeIndexStore) Rebuild(context.Context, string) error {
	i.rebuildCalls++
	return i.rebuildErr
}

type fakeFileStore struct {
	loadOutline   Outline
	loadErr       error
	exists        map[string]bool
	existsErr     error
	marshalErr    error
	writeErr      error
	writeCalls    int
	writtenFiles  map[string][]byte
	rollbackCalls int
}

func (s *fakeFileStore) Load(context.Context, string) (Outline, error) {
	return s.loadOutline, s.loadErr
}

func (s *fakeFileStore) Exists(_ context.Context, _ string, relativePath string) (bool, error) {
	if s.existsErr != nil {
		return false, s.existsErr
	}
	return s.exists[relativePath], nil
}

func (s *fakeFileStore) MarshalOutline(Outline) ([]byte, error) {
	if s.marshalErr != nil {
		return nil, s.marshalErr
	}
	return []byte("outline"), nil
}

func (s *fakeFileStore) MarshalArc(arc Arc) ([]byte, error) {
	if s.marshalErr != nil {
		return nil, s.marshalErr
	}
	return []byte("arc:" + arc.ID), nil
}

func (s *fakeFileStore) MarshalChapter(chapter Chapter) ([]byte, error) {
	if s.marshalErr != nil {
		return nil, s.marshalErr
	}
	return []byte("chapter:" + chapter.ID), nil
}

func (s *fakeFileStore) MarshalScene(scene Scene) ([]byte, error) {
	if s.marshalErr != nil {
		return nil, s.marshalErr
	}
	return []byte("scene:" + scene.ID), nil
}

func (s *fakeFileStore) WriteFiles(_ context.Context, _ string, files map[string][]byte) (func() error, error) {
	if s.writeErr != nil {
		return nil, s.writeErr
	}
	s.writeCalls++
	s.writtenFiles = files
	return func() error {
		s.rollbackCalls++
		return nil
	}, nil
}
