package story

import (
	"context"
	"errors"
	"sort"
	"strings"

	"storywork/internal/codex"
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
	isCleanErr     error
	isCleanCalls   int
	commitCalls    int
	unstageCalls   int
	commitMessages []string
	commitErr      error
	unstageErr     error
}

func (g *fakeGitStore) IsClean(context.Context, string) (bool, error) {
	g.isCleanCalls++
	return g.clean, g.isCleanErr
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
	loadOutline              Outline
	loadErr                  error
	reloadErr                error
	loadCalls                int
	loadHook                 func(int)
	marshaled                Outline
	reloadPending            bool
	exists                   map[string]bool
	existsErr                error
	marshalErr               error
	writeErr                 error
	writeCalls               int
	writtenFiles             map[string][]byte
	rollbackCalls            int
	rollbackErr              error
	scene                    SceneDocument
	loadSceneErr             error
	sceneBytes               []byte
	marshalSceneErr          error
	loadSceneCalls           int
	reloadedScene            *SceneDocument
	codexEntries             []codex.Entry
	loadCodexEntriesErr      error
	codexEntry               codex.Entry
	loadCodexEntryErr        error
	codexProgressions        codex.ProgressionDocument
	loadCodexProgressionsErr error
	codexEntryBytes          []byte
	marshalCodexEntryErr     error
	progressionBytes         []byte
	marshalProgressionsErr   error
}

func (s *fakeFileStore) Load(context.Context, string) (Outline, error) {
	s.loadCalls++
	if s.loadHook != nil {
		s.loadHook(s.loadCalls)
	}
	if s.reloadPending {
		s.reloadPending = false
		if s.reloadErr != nil {
			return Outline{}, s.reloadErr
		}
		return s.marshaled, nil
	}
	return s.loadOutline, s.loadErr
}

func (s *fakeFileStore) Exists(_ context.Context, _ string, relativePath string) (bool, error) {
	if s.existsErr != nil {
		return false, s.existsErr
	}
	return s.exists[relativePath], nil
}

func (s *fakeFileStore) LoadScene(context.Context, string, string) (SceneDocument, error) {
	s.loadSceneCalls++
	if s.loadSceneErr != nil {
		return SceneDocument{}, s.loadSceneErr
	}
	if s.reloadedScene != nil && s.loadSceneCalls > 1 {
		return *s.reloadedScene, nil
	}
	return s.scene, nil
}

func (s *fakeFileStore) LoadCodexEntries(context.Context, string) ([]codex.Entry, error) {
	if s.loadCodexEntriesErr != nil {
		return nil, s.loadCodexEntriesErr
	}
	return append([]codex.Entry(nil), s.codexEntries...), nil
}

func (s *fakeFileStore) LoadCodexEntry(context.Context, string, string) (codex.Entry, error) {
	if s.loadCodexEntryErr != nil {
		return codex.Entry{}, s.loadCodexEntryErr
	}
	return s.codexEntry, nil
}

func (s *fakeFileStore) LoadProgressions(context.Context, string, string) (codex.ProgressionDocument, error) {
	if s.loadCodexProgressionsErr != nil {
		return codex.ProgressionDocument{}, s.loadCodexProgressionsErr
	}
	return s.codexProgressions, nil
}

func (s *fakeFileStore) MarshalOutline(outline Outline) ([]byte, error) {
	if s.marshalErr != nil {
		return nil, s.marshalErr
	}
	s.marshaled = outline
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

func (s *fakeFileStore) MarshalSceneDocument(scene SceneDocument) ([]byte, error) {
	if s.marshalSceneErr != nil {
		return nil, s.marshalSceneErr
	}
	if s.sceneBytes != nil {
		return s.sceneBytes, nil
	}
	return []byte("scene-document"), nil
}

func (s *fakeFileStore) MarshalCodexEntry(entry codex.Entry) ([]byte, error) {
	if s.marshalCodexEntryErr != nil {
		return nil, s.marshalCodexEntryErr
	}
	if s.codexEntryBytes != nil {
		return s.codexEntryBytes, nil
	}
	return []byte("codex-entry"), nil
}

func (s *fakeFileStore) MarshalProgressions(document codex.ProgressionDocument) ([]byte, error) {
	if s.marshalProgressionsErr != nil {
		return nil, s.marshalProgressionsErr
	}
	if len(document.Progressions) == 0 {
		return []byte("progressions-empty"), nil
	}
	if s.progressionBytes != nil {
		return s.progressionBytes, nil
	}
	var builder strings.Builder
	builder.WriteString(document.EntryID)
	for _, progression := range document.Progressions {
		builder.WriteString("|")
		builder.WriteString(progression.ID)
		builder.WriteString("|")
		builder.WriteString(progression.Anchor.ID)
		builder.WriteString("|")
		builder.WriteString(progression.Anchor.Timing)
		builder.WriteString("|")
		if progression.Changes.Description != nil {
			builder.WriteString(*progression.Changes.Description)
		}
		keys := make([]string, 0, len(progression.Changes.Metadata))
		for key := range progression.Changes.Metadata {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			builder.WriteString("|")
			builder.WriteString(key)
			builder.WriteString("=")
			builder.WriteString(progression.Changes.Metadata[key])
		}
	}
	return []byte(builder.String()), nil
}

func (s *fakeFileStore) WriteFiles(_ context.Context, _ string, files map[string][]byte) (func() error, error) {
	if s.writeErr != nil {
		return nil, s.writeErr
	}
	s.writeCalls++
	s.writtenFiles = files
	s.reloadPending = true
	return func() error {
		s.rollbackCalls++
		return s.rollbackErr
	}, nil
}
