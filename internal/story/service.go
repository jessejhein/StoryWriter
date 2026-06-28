package story

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"storywork/internal/project"
)

// NodeKind identifies the type of structure node being created.
type NodeKind string

const (
	NodeKindArc     NodeKind = "arc"
	NodeKindChapter NodeKind = "chapter"
	NodeKindScene   NodeKind = "scene"
)

// Session resolves the active project for the current backend process.
type Session interface {
	Current() (project.Project, bool)
}

// FileStore loads, marshals, and atomically writes canonical story files.
type FileStore interface {
	Load(ctx context.Context, projectPath string) (Outline, error)
	LoadScene(ctx context.Context, projectPath, sceneID string) (SceneDocument, error)
	Exists(ctx context.Context, projectPath, relativePath string) (bool, error)
	MarshalOutline(outline Outline) ([]byte, error)
	MarshalArc(arc Arc) ([]byte, error)
	MarshalChapter(chapter Chapter) ([]byte, error)
	MarshalScene(scene Scene) ([]byte, error)
	MarshalSceneDocument(scene SceneDocument) ([]byte, error)
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
		return SceneDocument{}, s.rollbackMutation(ctx, current.Path, nil, err)
	}
	if err := s.index.Rebuild(ctx, current.Path); err != nil {
		return SceneDocument{}, s.rollbackMutation(ctx, current.Path, rollback, err)
	}
	if err := s.git.CommitAll(ctx, current.Path, "Edit scene "+sceneID); err != nil {
		return SceneDocument{}, s.rollbackMutation(ctx, current.Path, rollback, err)
	}
	reloaded, err := s.files.LoadScene(ctx, current.Path, sceneID)
	if err != nil {
		return SceneDocument{}, s.rollbackMutation(ctx, current.Path, rollback, err)
	}
	return reloaded, nil
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
	default:
		return "", fmt.Errorf("unknown node kind %q", kind)
	}
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
