package story

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"storywork/internal/codex"
	"storywork/internal/project"
)

// NodeKind identifies the type of structure node being created.
type NodeKind string

const (
	NodeKindArc         NodeKind = "arc"
	NodeKindChapter     NodeKind = "chapter"
	NodeKindScene       NodeKind = "scene"
	NodeKindCharacter   NodeKind = "character"
	NodeKindLocation    NodeKind = "location"
	NodeKindLore        NodeKind = "lore"
	NodeKindCustom      NodeKind = "custom"
	NodeKindProgression NodeKind = "progression"
)

// Session resolves the active project for the current backend process.
type Session interface {
	Current() (project.Project, bool)
}

// FileStore loads, marshals, and atomically writes canonical story files.
type FileStore interface {
	Load(ctx context.Context, projectPath string) (Outline, error)
	LoadScene(ctx context.Context, projectPath, sceneID string) (SceneDocument, error)
	LoadCodexEntries(ctx context.Context, projectPath string) ([]codex.Entry, error)
	LoadCodexEntry(ctx context.Context, projectPath, entryID string) (codex.Entry, error)
	LoadProgressions(ctx context.Context, projectPath, entryID string) (codex.ProgressionDocument, error)
	Exists(ctx context.Context, projectPath, relativePath string) (bool, error)
	MarshalOutline(outline Outline) ([]byte, error)
	MarshalArc(arc Arc) ([]byte, error)
	MarshalChapter(chapter Chapter) ([]byte, error)
	MarshalScene(scene Scene) ([]byte, error)
	MarshalSceneDocument(scene SceneDocument) ([]byte, error)
	MarshalCodexEntry(entry codex.Entry) ([]byte, error)
	MarshalProgressions(document codex.ProgressionDocument) ([]byte, error)
	WriteFiles(ctx context.Context, projectPath string, files map[string][]byte) (func() error, error)
}

// GitStore guards mutation safety and records checkpoints.
type GitStore interface {
	IsClean(ctx context.Context, path string) (bool, error)
	CommitAll(ctx context.Context, path, message string) error
	UnstageAll(ctx context.Context, path string) error
}

// IndexStore rebuilds the disposable project index.
type IndexStore interface {
	Rebuild(ctx context.Context, projectPath string) error
}

// IDGenerator returns stable opaque IDs for new structure nodes.
type IDGenerator interface {
	Next(kind NodeKind) (string, error)
}

// MutationResult wraps the changed outline returned by create and reorder calls.
type MutationResult struct {
	ChangedID string  `json:"changed_id,omitempty"`
	Outline   Outline `json:"outline"`
}

// Service coordinates outline reads and structural mutations.
type Service struct {
	session Session
	files   FileStore
	git     GitStore
	index   IndexStore
	ids     IDGenerator
	mu      sync.RWMutex
}

// NewService creates the structural mutation service.
func NewService(session Session, files FileStore, git GitStore, index IndexStore, ids IDGenerator) *Service {
	return &Service{
		session: session,
		files:   files,
		git:     git,
		index:   index,
		ids:     ids,
	}
}

// Outline returns the current active project's outline.
func (s *Service) Outline(ctx context.Context) (Outline, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	current, err := s.currentProject()
	if err != nil {
		return Outline{}, err
	}
	return s.files.Load(ctx, current.Path)
}

// LoadScene returns one existing canonical scene for editor use.
func (s *Service) LoadScene(ctx context.Context, sceneID string) (SceneDocument, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	current, err := s.currentProject()
	if err != nil {
		return SceneDocument{}, err
	}
	if err := ValidateSceneID(sceneID); err != nil {
		return SceneDocument{}, err
	}
	outline, err := s.files.Load(ctx, current.Path)
	if err != nil {
		return SceneDocument{}, err
	}
	if _, err := findScene(outline, sceneID); err != nil {
		if errors.Is(err, ErrParentNotFound) {
			return SceneDocument{}, fmt.Errorf("scene %q: %w", sceneID, ErrSceneNotFound)
		}
		return SceneDocument{}, err
	}
	return s.files.LoadScene(ctx, current.Path, sceneID)
}

// CreateArc appends a new arc and checkpoints the mutation.
func (s *Service) CreateArc(ctx context.Context, title string) (MutationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, err := s.currentProject()
	if err != nil {
		return MutationResult{}, err
	}
	outline, err := s.requireCleanOutline(ctx, current.Path)
	if err != nil {
		return MutationResult{}, err
	}
	arcID, err := s.nextUnusedID(ctx, current.Path, NodeKindArc)
	if err != nil {
		return MutationResult{}, err
	}
	next, err := AddArc(outline, arcID, title)
	if err != nil {
		return MutationResult{}, err
	}
	arc := next.Arcs[len(next.Arcs)-1]
	outlineBytes, err := s.files.MarshalOutline(next)
	if err != nil {
		return MutationResult{}, err
	}
	arcBytes, err := s.files.MarshalArc(arc)
	if err != nil {
		return MutationResult{}, err
	}
	return s.persistMutation(ctx, current.Path, arcID, "Add arc "+arcID, map[string][]byte{
		"outline.yaml": outlineBytes,
		filepath.ToSlash(filepath.Join("arcs", arcID+".yaml")): arcBytes,
	})
}

// CreateChapter appends a chapter inside an existing arc.
func (s *Service) CreateChapter(ctx context.Context, arcID, title string) (MutationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, err := s.currentProject()
	if err != nil {
		return MutationResult{}, err
	}
	outline, err := s.requireCleanOutline(ctx, current.Path)
	if err != nil {
		return MutationResult{}, err
	}
	chapterID, err := s.nextUnusedID(ctx, current.Path, NodeKindChapter)
	if err != nil {
		return MutationResult{}, err
	}
	next, err := AddChapter(outline, arcID, chapterID, title)
	if err != nil {
		return MutationResult{}, err
	}
	chapter, err := findChapter(next, chapterID)
	if err != nil {
		return MutationResult{}, err
	}
	outlineBytes, err := s.files.MarshalOutline(next)
	if err != nil {
		return MutationResult{}, err
	}
	chapterBytes, err := s.files.MarshalChapter(chapter)
	if err != nil {
		return MutationResult{}, err
	}
	return s.persistMutation(ctx, current.Path, chapterID, "Add chapter "+chapterID, map[string][]byte{
		"outline.yaml": outlineBytes,
		filepath.ToSlash(filepath.Join("chapters", chapterID+".yaml")): chapterBytes,
	})
}

// CreateScene appends a scene inside an existing chapter.
func (s *Service) CreateScene(ctx context.Context, chapterID, title string) (MutationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, err := s.currentProject()
	if err != nil {
		return MutationResult{}, err
	}
	outline, err := s.requireCleanOutline(ctx, current.Path)
	if err != nil {
		return MutationResult{}, err
	}
	sceneID, err := s.nextUnusedID(ctx, current.Path, NodeKindScene)
	if err != nil {
		return MutationResult{}, err
	}
	next, err := AddScene(outline, chapterID, sceneID, title)
	if err != nil {
		return MutationResult{}, err
	}
	scene, err := findScene(next, sceneID)
	if err != nil {
		return MutationResult{}, err
	}
	outlineBytes, err := s.files.MarshalOutline(next)
	if err != nil {
		return MutationResult{}, err
	}
	sceneBytes, err := s.files.MarshalScene(scene)
	if err != nil {
		return MutationResult{}, err
	}
	return s.persistMutation(ctx, current.Path, sceneID, "Add scene "+sceneID, map[string][]byte{
		"outline.yaml": outlineBytes,
		filepath.ToSlash(filepath.Join("scenes", sceneID+".md")): sceneBytes,
	})
}

// Reorder updates chapter or scene order and checkpoints the change.
func (s *Service) Reorder(ctx context.Context, request ReorderRequest) (MutationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, err := s.currentProject()
	if err != nil {
		return MutationResult{}, err
	}
	outline, err := s.requireCleanOutline(ctx, current.Path)
	if err != nil {
		return MutationResult{}, err
	}
	next, err := Reorder(outline, request)
	if err != nil {
		return MutationResult{}, err
	}

	message := ""
	switch request.ParentType {
	case "arc":
		message = "Reorder chapters in " + request.ParentID
	case "chapter":
		message = "Reorder scenes in " + request.ParentID
	default:
		return MutationResult{}, fmt.Errorf("parent_type %q: %w", request.ParentType, ErrInvalidReorder)
	}
	outlineBytes, err := s.files.MarshalOutline(next)
	if err != nil {
		return MutationResult{}, err
	}
	return s.persistMutation(ctx, current.Path, "", message, map[string][]byte{
		"outline.yaml": outlineBytes,
	})
}

// SaveScene validates and persists one canonical scene edit.
func (s *Service) SaveScene(ctx context.Context, sceneID string, request SaveSceneRequest) (SceneDocument, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, err := s.currentProject()
	if err != nil {
		return SceneDocument{}, err
	}
	if err := ValidateSceneID(sceneID); err != nil {
		return SceneDocument{}, err
	}
	request, err = ValidateSceneSaveRequest(request)
	if err != nil {
		return SceneDocument{}, err
	}

	clean, err := s.git.IsClean(ctx, current.Path)
	if err != nil {
		return SceneDocument{}, err
	}
	if !clean {
		return SceneDocument{}, ErrDirtyWorktree
	}

	outline, err := s.files.Load(ctx, current.Path)
	if err != nil {
		return SceneDocument{}, err
	}
	sceneRef, err := findScene(outline, sceneID)
	if err != nil {
		if errors.Is(err, ErrParentNotFound) {
			return SceneDocument{}, fmt.Errorf("scene %q: %w", sceneID, ErrSceneNotFound)
		}
		return SceneDocument{}, err
	}
	currentScene, err := s.files.LoadScene(ctx, current.Path, sceneID)
	if err != nil {
		return SceneDocument{}, err
	}
	if currentScene.ChapterID != sceneRef.ChapterID {
		return SceneDocument{}, fmt.Errorf("scene %q chapter mismatch: %w", sceneID, ErrSceneNotFound)
	}
	if currentScene.Revision != request.ExpectedRevision {
		return SceneDocument{}, fmt.Errorf("scene %q revision changed: %w", sceneID, ErrStaleRevision)
	}

	nextScene := currentScene
	nextScene.Title = request.Title
	nextScene.FrontMatter = request.FrontMatter
	nextScene.Markdown = request.Markdown

	sceneBytes, err := s.files.MarshalSceneDocument(nextScene)
	if err != nil {
		return SceneDocument{}, err
	}
	if bytes.Equal(sceneBytes, currentScene.Canonical) {
		return SceneDocument{}, ErrNoSceneChanges
	}

	rollback, err := s.files.WriteFiles(ctx, current.Path, map[string][]byte{
		filepath.ToSlash(filepath.Join("scenes", sceneID+".md")): sceneBytes,
	})
	if err != nil {
		return SceneDocument{}, err
	}
	if err := s.index.Rebuild(ctx, current.Path); err != nil {
		return SceneDocument{}, s.rollbackMutation(ctx, current.Path, rollback, err)
	}
	if err := s.git.CommitAll(ctx, current.Path, "Edit scene "+sceneID); err != nil {
		return SceneDocument{}, s.rollbackMutation(ctx, current.Path, rollback, err)
	}
	nextScene.Revision = ComputeRevision(sceneBytes)
	nextScene.Canonical = append([]byte(nil), sceneBytes...)
	return nextScene, nil
}

// CodexEntries returns the current active project's validated Codex list.
func (s *Service) CodexEntries(ctx context.Context) ([]codex.Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	current, err := s.currentProject()
	if err != nil {
		return nil, err
	}
	return s.files.LoadCodexEntries(ctx, current.Path)
}

// LoadCodexEntry returns one validated canonical Codex entry.
func (s *Service) LoadCodexEntry(ctx context.Context, entryID string) (codex.Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	current, err := s.currentProject()
	if err != nil {
		return codex.Entry{}, err
	}
	if err := codex.ValidateEntryID(entryID); err != nil {
		return codex.Entry{}, err
	}
	return s.files.LoadCodexEntry(ctx, current.Path, entryID)
}

// CreateCodexEntry creates and checkpoints one new canonical Codex entry.
func (s *Service) CreateCodexEntry(ctx context.Context, request codex.SaveEntryRequest) (codex.Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, err := s.currentProject()
	if err != nil {
		return codex.Entry{}, err
	}
	request, err = codex.NormalizeCreateRequest(request)
	if err != nil {
		return codex.Entry{}, err
	}
	clean, err := s.git.IsClean(ctx, current.Path)
	if err != nil {
		return codex.Entry{}, err
	}
	if !clean {
		return codex.Entry{}, ErrDirtyWorktree
	}
	entryID, err := s.nextUnusedID(ctx, current.Path, codexNodeKind(request.Type))
	if err != nil {
		return codex.Entry{}, err
	}
	nextEntry, err := codex.NormalizeEntry(codex.Entry{
		ID:          entryID,
		Type:        request.Type,
		Name:        request.Name,
		Aliases:     request.Aliases,
		Tags:        request.Tags,
		Description: request.Description,
		Metadata:    request.Metadata,
	})
	if err != nil {
		return codex.Entry{}, err
	}
	entryBytes, err := s.files.MarshalCodexEntry(nextEntry)
	if err != nil {
		return codex.Entry{}, err
	}
	relativePath, err := codexEntryPath(nextEntry)
	if err != nil {
		return codex.Entry{}, err
	}
	rollback, err := s.files.WriteFiles(ctx, current.Path, map[string][]byte{relativePath: entryBytes})
	if err != nil {
		return codex.Entry{}, err
	}
	if err := s.index.Rebuild(ctx, current.Path); err != nil {
		return codex.Entry{}, s.rollbackMutation(ctx, current.Path, rollback, err)
	}
	if err := s.git.CommitAll(ctx, current.Path, "Create Codex entry "+nextEntry.ID); err != nil {
		return codex.Entry{}, s.rollbackMutation(ctx, current.Path, rollback, err)
	}
	nextEntry.Revision = codex.ComputeRevision(entryBytes)
	return nextEntry, nil
}

// UpdateCodexEntry edits one existing canonical Codex entry.
func (s *Service) UpdateCodexEntry(ctx context.Context, entryID string, request codex.SaveEntryRequest) (codex.Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, err := s.currentProject()
	if err != nil {
		return codex.Entry{}, err
	}
	if err := codex.ValidateEntryID(entryID); err != nil {
		return codex.Entry{}, err
	}
	clean, err := s.git.IsClean(ctx, current.Path)
	if err != nil {
		return codex.Entry{}, err
	}
	if !clean {
		return codex.Entry{}, ErrDirtyWorktree
	}
	currentEntry, err := s.files.LoadCodexEntry(ctx, current.Path, entryID)
	if err != nil {
		return codex.Entry{}, err
	}
	if currentEntry.Revision != request.ExpectedRevision {
		return codex.Entry{}, fmt.Errorf("entry %q revision changed: %w", entryID, ErrStaleRevision)
	}
	nextEntry, err := codex.NormalizeUpdateRequest(entryID, currentEntry, request)
	if err != nil {
		return codex.Entry{}, err
	}
	entryBytes, err := s.files.MarshalCodexEntry(nextEntry)
	if err != nil {
		return codex.Entry{}, err
	}
	currentBytes, err := s.files.MarshalCodexEntry(currentEntry)
	if err != nil {
		return codex.Entry{}, err
	}
	if bytes.Equal(entryBytes, currentBytes) {
		return codex.Entry{}, codex.ErrNoChanges
	}
	relativePath, err := codexEntryPath(nextEntry)
	if err != nil {
		return codex.Entry{}, err
	}
	rollback, err := s.files.WriteFiles(ctx, current.Path, map[string][]byte{relativePath: entryBytes})
	if err != nil {
		return codex.Entry{}, err
	}
	if err := s.index.Rebuild(ctx, current.Path); err != nil {
		return codex.Entry{}, s.rollbackMutation(ctx, current.Path, rollback, err)
	}
	if err := s.git.CommitAll(ctx, current.Path, "Edit Codex entry "+nextEntry.ID); err != nil {
		return codex.Entry{}, s.rollbackMutation(ctx, current.Path, rollback, err)
	}
	nextEntry.Revision = codex.ComputeRevision(entryBytes)
	return nextEntry, nil
}

// LoadProgressions returns one entry's canonical progression document or an empty logical document.
func (s *Service) LoadProgressions(ctx context.Context, entryID string) (codex.ProgressionDocument, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	current, err := s.currentProject()
	if err != nil {
		return codex.ProgressionDocument{}, err
	}
	if err := codex.ValidateEntryID(entryID); err != nil {
		return codex.ProgressionDocument{}, err
	}
	outline, err := s.files.Load(ctx, current.Path)
	if err != nil {
		return codex.ProgressionDocument{}, err
	}
	if _, err := s.files.LoadCodexEntry(ctx, current.Path, entryID); err != nil {
		return codex.ProgressionDocument{}, err
	}
	document, err := s.files.LoadProgressions(ctx, current.Path, entryID)
	if err != nil {
		return codex.ProgressionDocument{}, err
	}
	if err := validateProgressionAnchors(outlineSceneIDs(outline), document.Progressions); err != nil {
		return codex.ProgressionDocument{}, err
	}
	return document, nil
}

// SaveProgressions replaces one entry's ordered canonical progression list.
func (s *Service) SaveProgressions(ctx context.Context, entryID string, request codex.SaveProgressionsRequest) (codex.ProgressionDocument, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, err := s.currentProject()
	if err != nil {
		return codex.ProgressionDocument{}, err
	}
	if err := codex.ValidateEntryID(entryID); err != nil {
		return codex.ProgressionDocument{}, err
	}
	clean, err := s.git.IsClean(ctx, current.Path)
	if err != nil {
		return codex.ProgressionDocument{}, err
	}
	if !clean {
		return codex.ProgressionDocument{}, ErrDirtyWorktree
	}
	outline, err := s.files.Load(ctx, current.Path)
	if err != nil {
		return codex.ProgressionDocument{}, err
	}
	if _, err := s.files.LoadCodexEntry(ctx, current.Path, entryID); err != nil {
		return codex.ProgressionDocument{}, err
	}
	currentDocument, err := s.files.LoadProgressions(ctx, current.Path, entryID)
	if err != nil {
		return codex.ProgressionDocument{}, err
	}
	if err := validateProgressionExpectedRevision(currentDocument.Revision, request.ExpectedRevision); err != nil {
		return codex.ProgressionDocument{}, err
	}
	if request.ExpectedRevision != nil && currentDocument.Revision != nil && *request.ExpectedRevision != *currentDocument.Revision {
		return codex.ProgressionDocument{}, fmt.Errorf("progressions %q revision changed: %w", entryID, ErrStaleRevision)
	}
	sceneIDs := outlineSceneIDs(outline)
	progressions := append([]codex.Progression(nil), request.Progressions...)
	if len(progressions) == 0 && currentDocument.Revision == nil {
		return codex.ProgressionDocument{}, codex.ErrNoChanges
	}
	progressions, err = s.assignProgressionIDs(progressions)
	if err != nil {
		return codex.ProgressionDocument{}, err
	}
	progressions, err = codex.NormalizeProgressions(entryID, progressions, sceneIDs)
	if err != nil {
		return codex.ProgressionDocument{}, err
	}
	nextDocument := codex.ProgressionDocument{
		EntryID:      entryID,
		Progressions: progressions,
	}
	documentBytes, err := s.files.MarshalProgressions(nextDocument)
	if err != nil {
		return codex.ProgressionDocument{}, err
	}
	currentBytes, err := s.files.MarshalProgressions(codex.ProgressionDocument{
		EntryID:      currentDocument.EntryID,
		Progressions: currentDocument.Progressions,
	})
	if err == nil && bytes.Equal(documentBytes, currentBytes) {
		return codex.ProgressionDocument{}, codex.ErrNoChanges
	}
	relativePath := filepath.ToSlash(filepath.Join("progressions", entryID+".yaml"))
	rollback, err := s.files.WriteFiles(ctx, current.Path, map[string][]byte{relativePath: documentBytes})
	if err != nil {
		return codex.ProgressionDocument{}, err
	}
	if err := s.index.Rebuild(ctx, current.Path); err != nil {
		return codex.ProgressionDocument{}, s.rollbackMutation(ctx, current.Path, rollback, err)
	}
	if err := s.git.CommitAll(ctx, current.Path, "Edit progressions "+entryID); err != nil {
		return codex.ProgressionDocument{}, s.rollbackMutation(ctx, current.Path, rollback, err)
	}
	revision := codex.ComputeRevision(documentBytes)
	nextDocument.Revision = &revision
	return nextDocument, nil
}

// ResolveActiveCodexState reads one entry and resolves its active state for a target scene.
func (s *Service) ResolveActiveCodexState(ctx context.Context, entryID, sceneID string) (codex.ActiveState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	current, err := s.currentProject()
	if err != nil {
		return codex.ActiveState{}, err
	}
	if err := codex.ValidateEntryID(entryID); err != nil {
		return codex.ActiveState{}, err
	}
	if err := codex.ValidateSceneID(sceneID); err != nil {
		return codex.ActiveState{}, err
	}
	outline, err := s.files.Load(ctx, current.Path)
	if err != nil {
		return codex.ActiveState{}, err
	}
	entry, err := s.files.LoadCodexEntry(ctx, current.Path, entryID)
	if err != nil {
		return codex.ActiveState{}, err
	}
	progressions, err := s.files.LoadProgressions(ctx, current.Path, entryID)
	if err != nil {
		return codex.ActiveState{}, err
	}
	return codex.ResolveActiveState(entry, progressions.Progressions, flattenOutlineScenes(outline), sceneID)
}

func (s *Service) persistMutation(ctx context.Context, projectPath, changedID, message string, files map[string][]byte) (MutationResult, error) {
	rollback, err := s.files.WriteFiles(ctx, projectPath, files)
	if err != nil {
		return MutationResult{}, s.rollbackMutation(ctx, projectPath, nil, err)
	}
	reloaded, err := s.files.Load(ctx, projectPath)
	if err != nil {
		return MutationResult{}, s.rollbackMutation(ctx, projectPath, rollback, err)
	}
	if err := s.index.Rebuild(ctx, projectPath); err != nil {
		return MutationResult{}, s.rollbackMutation(ctx, projectPath, rollback, err)
	}
	if err := s.git.CommitAll(ctx, projectPath, message); err != nil {
		return MutationResult{}, s.rollbackMutation(ctx, projectPath, rollback, err)
	}
	return MutationResult{ChangedID: changedID, Outline: reloaded}, nil
}

func (s *Service) assignProgressionIDs(progressions []codex.Progression) ([]codex.Progression, error) {
	next := append([]codex.Progression(nil), progressions...)
	for index := range next {
		if next[index].ID != "" {
			continue
		}
		id, err := s.ids.Next(NodeKindProgression)
		if err != nil {
			return nil, err
		}
		next[index].ID = id
	}
	return next, nil
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

func (s *Service) currentProject() (project.Project, error) {
	current, ok := s.session.Current()
	if !ok {
		return project.Project{}, ErrNoActiveProject
	}
	return current, nil
}

func (s *Service) requireCleanOutline(ctx context.Context, projectPath string) (Outline, error) {
	clean, err := s.git.IsClean(ctx, projectPath)
	if err != nil {
		return Outline{}, err
	}
	if !clean {
		return Outline{}, ErrDirtyWorktree
	}
	return s.files.Load(ctx, projectPath)
}

func (s *Service) nextUnusedID(ctx context.Context, projectPath string, kind NodeKind) (string, error) {
	for range 5 {
		next, err := s.ids.Next(kind)
		if err != nil {
			return "", err
		}
		relativePath, err := entityPath(kind, next)
		if err != nil {
			return "", err
		}
		exists, err := s.files.Exists(ctx, projectPath, relativePath)
		if err != nil {
			return "", err
		}
		if !exists {
			return next, nil
		}
	}
	return "", errors.New("generate stable ID: too many collisions")
}

func entityPath(kind NodeKind, id string) (string, error) {
	switch kind {
	case NodeKindArc:
		return filepath.ToSlash(filepath.Join("arcs", id+".yaml")), nil
	case NodeKindChapter:
		return filepath.ToSlash(filepath.Join("chapters", id+".yaml")), nil
	case NodeKindScene:
		return filepath.ToSlash(filepath.Join("scenes", id+".md")), nil
	case NodeKindCharacter:
		return filepath.ToSlash(filepath.Join("codex", "characters", id+".yaml")), nil
	case NodeKindLocation:
		return filepath.ToSlash(filepath.Join("codex", "locations", id+".yaml")), nil
	case NodeKindLore:
		return filepath.ToSlash(filepath.Join("codex", "lore", id+".yaml")), nil
	case NodeKindCustom:
		return filepath.ToSlash(filepath.Join("codex", "custom", id+".yaml")), nil
	case NodeKindProgression:
		return filepath.ToSlash(filepath.Join("progressions", id+".yaml")), nil
	default:
		return "", fmt.Errorf("unknown node kind %q", kind)
	}
}

func codexNodeKind(entryType codex.EntryType) NodeKind {
	switch entryType {
	case codex.TypeCharacter:
		return NodeKindCharacter
	case codex.TypeLocation:
		return NodeKindLocation
	case codex.TypeLore:
		return NodeKindLore
	case codex.TypeCustom:
		return NodeKindCustom
	default:
		return NodeKind("")
	}
}

func codexEntryPath(entry codex.Entry) (string, error) {
	return entityPath(codexNodeKind(entry.Type), entry.ID)
}

func validateProgressionExpectedRevision(current, expected *string) error {
	if expected == nil {
		if current != nil {
			return codex.ErrInvalidRevision
		}
		return nil
	}
	if *expected == "" {
		return codex.ErrInvalidRevision
	}
	if err := codex.ValidateRevision(*expected); err != nil {
		return err
	}
	return nil
}

func outlineSceneIDs(outline Outline) map[string]struct{} {
	sceneIDs := make(map[string]struct{})
	for _, scene := range flattenOutlineScenes(outline) {
		sceneIDs[scene.ID] = struct{}{}
	}
	return sceneIDs
}

func flattenOutlineScenes(outline Outline) []codex.SceneRef {
	scenes := make([]codex.SceneRef, 0)
	for _, arc := range outline.Arcs {
		for _, chapter := range arc.Chapters {
			for _, scene := range chapter.Scenes {
				scenes = append(scenes, codex.SceneRef{ID: scene.ID})
			}
		}
	}
	return scenes
}

func validateProgressionAnchors(sceneIDs map[string]struct{}, progressions []codex.Progression) error {
	for _, progression := range progressions {
		if _, ok := sceneIDs[progression.Anchor.ID]; !ok {
			return fmt.Errorf("progression %q references scene anchor %q absent from the current outline", progression.ID, progression.Anchor.ID)
		}
	}
	return nil
}

func findChapter(outline Outline, chapterID string) (Chapter, error) {
	for _, arc := range outline.Arcs {
		for _, chapter := range arc.Chapters {
			if chapter.ID == chapterID {
				return chapter, nil
			}
		}
	}
	return Chapter{}, fmt.Errorf("chapter %q: %w", chapterID, ErrParentNotFound)
}

func findScene(outline Outline, sceneID string) (Scene, error) {
	for _, arc := range outline.Arcs {
		for _, chapter := range arc.Chapters {
			for _, scene := range chapter.Scenes {
				if scene.ID == sceneID {
					return scene, nil
				}
			}
		}
	}
	return Scene{}, fmt.Errorf("scene %q: %w", sceneID, ErrParentNotFound)
}
