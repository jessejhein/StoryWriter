package story

// service.go coordinates active-project reads, canonical mutations, indexing, and checkpoints.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"storywork/internal/codex"
	"storywork/internal/mutation"
	"storywork/internal/project"
)

// NodeKind identifies the type of structure node being created.
type NodeKind string

const (
	// NodeKindArc identifies generated IDs for arcs.
	NodeKindArc NodeKind = "arc"
	// NodeKindChapter identifies generated IDs for chapters.
	NodeKindChapter NodeKind = "chapter"
	// NodeKindScene identifies generated IDs for scenes.
	NodeKindScene NodeKind = "scene"
	// NodeKindCharacter identifies generated IDs for character Codex entries.
	NodeKindCharacter NodeKind = "character"
	// NodeKindLocation identifies generated IDs for location Codex entries.
	NodeKindLocation NodeKind = "location"
	// NodeKindLore identifies generated IDs for lore Codex entries.
	NodeKindLore NodeKind = "lore"
	// NodeKindCustom identifies generated IDs for custom Codex entries.
	NodeKindCustom NodeKind = "custom"
	// NodeKindProgression identifies generated IDs for Codex progressions.
	NodeKindProgression NodeKind = "progression"
)

// Session resolves the active project for the current backend process.
type Session interface {
	// Current returns the active project when one has been selected.
	Current() (project.Project, bool)
}

// FileStore loads, marshals, and atomically writes canonical story files.
type FileStore interface {
	// Load returns the canonical outline and referenced structure files.
	Load(ctx context.Context, projectPath string) (Outline, error)
	// LoadScene returns one canonical scene document.
	LoadScene(ctx context.Context, projectPath, sceneID string) (SceneDocument, error)
	// LoadCodexEntries returns every canonical Codex entry.
	LoadCodexEntries(ctx context.Context, projectPath string) ([]codex.Entry, error)
	// LoadCodexEntry returns one canonical Codex entry.
	LoadCodexEntry(ctx context.Context, projectPath, entryID string) (codex.Entry, error)
	// LoadProgressions returns one canonical progression document.
	LoadProgressions(ctx context.Context, projectPath, entryID string) (codex.ProgressionDocument, error)
	// Exists reports whether one canonical relative path exists.
	Exists(ctx context.Context, projectPath, relativePath string) (bool, error)
	// MarshalOutline encodes the canonical outline ordering file.
	MarshalOutline(outline Outline) ([]byte, error)
	// MarshalArc encodes one canonical arc file.
	MarshalArc(arc Arc) ([]byte, error)
	// MarshalChapter encodes one canonical chapter file.
	MarshalChapter(chapter Chapter) ([]byte, error)
	// MarshalScene encodes a new canonical scene file.
	MarshalScene(scene Scene) ([]byte, error)
	// MarshalSceneDocument encodes a full canonical scene document.
	MarshalSceneDocument(scene SceneDocument) ([]byte, error)
	// MarshalCodexEntry encodes one canonical Codex entry file.
	MarshalCodexEntry(entry codex.Entry) ([]byte, error)
	// MarshalProgressions encodes one canonical progression document.
	MarshalProgressions(document codex.ProgressionDocument) ([]byte, error)
	// WriteFiles atomically replaces the supplied canonical files and returns a rollback closure.
	WriteFiles(ctx context.Context, projectPath string, files map[string][]byte) (func() error, error)
}

// GitStore guards mutation safety and records checkpoints.
type GitStore interface {
	// IsClean reports whether the project worktree is free of tracked changes.
	IsClean(ctx context.Context, path string) (bool, error)
	// CommitAll stages and commits the current canonical mutation.
	CommitAll(ctx context.Context, path, message string) error
	// UnstageAll removes staged changes without discarding the working tree.
	UnstageAll(ctx context.Context, path string) error
}

// IndexStore rebuilds the disposable project index.
type IndexStore interface {
	// Rebuild recreates the derived index from canonical project files.
	Rebuild(ctx context.Context, projectPath string) error
}

// IDGenerator returns stable opaque IDs for new structure nodes.
type IDGenerator interface {
	// Next returns the next generated ID for the requested node kind.
	Next(kind NodeKind) (string, error)
}

// MutationResult wraps the changed outline returned by create and reorder calls.
type MutationResult struct {
	ChangedID string  `json:"changed_id,omitempty"`
	Outline   Outline `json:"outline"`
}

// ImportMutationKind identifies the canonical artifact an importer acceptance creates.
type ImportMutationKind string

const (
	ImportMutationArc     ImportMutationKind = "arc"
	ImportMutationChapter ImportMutationKind = "chapter"
	ImportMutationScene   ImportMutationKind = "scene"
	ImportMutationCodex   ImportMutationKind = "codex"
)

// ImportMutationRequest describes one no-checkpoint canonical creation used by
// Milestone 6 candidate acceptance.
type ImportMutationRequest struct {
	Kind     ImportMutationKind
	ParentID string
	Title    string
	Codex    codex.SaveEntryRequest
}

// ImportMutationResult reports the new canonical artifact and a rollback handle.
type ImportMutationResult struct {
	Kind     ImportMutationKind
	ID       string
	Rollback func() error
}

// Service coordinates outline reads and structural mutations.
type Service struct {
	session   Session
	files     FileStore
	git       GitStore
	index     IndexStore
	ids       IDGenerator
	mutations *mutation.Coordinator
}

// NewService creates the active-project story service with the supplied boundaries.
func NewService(session Session, files FileStore, git GitStore, index IndexStore, ids IDGenerator) *Service {
	return &Service{
		session:   session,
		files:     files,
		git:       git,
		index:     index,
		ids:       ids,
		mutations: mutation.NewCoordinator(),
	}
}

// WithMutationCoordinator replaces the service-local lock with the shared
// application mutation boundary. It must be called before serving requests.
func (s *Service) WithMutationCoordinator(coordinator *mutation.Coordinator) *Service {
	if coordinator != nil {
		s.mutations = coordinator
	}
	return s
}

// Outline returns the current active project's outline.
func (s *Service) Outline(ctx context.Context) (Outline, error) {
	s.mutations.RLock()
	defer s.mutations.RUnlock()

	current, err := s.currentProject()
	if err != nil {
		return Outline{}, err
	}
	return s.files.Load(ctx, current.Path)
}

// LoadScene returns one existing canonical scene for editor use.
func (s *Service) LoadScene(ctx context.Context, sceneID string) (SceneDocument, error) {
	s.mutations.RLock()
	defer s.mutations.RUnlock()

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
	s.mutations.Lock()
	defer s.mutations.Unlock()

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
	s.mutations.Lock()
	defer s.mutations.Unlock()

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
	s.mutations.Lock()
	defer s.mutations.Unlock()

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
	s.mutations.Lock()
	defer s.mutations.Unlock()

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
	s.mutations.Lock()
	defer s.mutations.Unlock()

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

// AcceptScenePatch validates and persists one reviewed AI replacement into canonical Markdown.
func (s *Service) AcceptScenePatch(ctx context.Context, request AcceptScenePatchRequest) (SceneDocument, error) {
	current, err := s.currentProject()
	if err != nil {
		return SceneDocument{}, err
	}
	if err := ValidateSceneID(request.SceneID); err != nil {
		return SceneDocument{}, err
	}
	if err := ValidateRevision(request.RunSceneRevision); err != nil {
		return SceneDocument{}, err
	}
	if err := ValidateRevision(request.ExpectedRevision); err != nil {
		return SceneDocument{}, err
	}
	if request.RunID == "" {
		return SceneDocument{}, fmt.Errorf("run_id is required: %w", ErrInvalidSelection)
	}
	s.mutations.Lock()
	defer s.mutations.Unlock()

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
	sceneRef, err := findScene(outline, request.SceneID)
	if err != nil {
		if errors.Is(err, ErrParentNotFound) {
			return SceneDocument{}, fmt.Errorf("scene %q: %w", request.SceneID, ErrSceneNotFound)
		}
		return SceneDocument{}, err
	}
	currentScene, err := s.files.LoadScene(ctx, current.Path, request.SceneID)
	if err != nil {
		return SceneDocument{}, err
	}
	if currentScene.ChapterID != sceneRef.ChapterID {
		return SceneDocument{}, fmt.Errorf("scene %q chapter mismatch: %w", request.SceneID, ErrSceneNotFound)
	}
	if currentScene.Revision != request.RunSceneRevision || currentScene.Revision != request.ExpectedRevision {
		return SceneDocument{}, fmt.Errorf("scene %q revision changed: %w", request.SceneID, ErrStaleRevision)
	}

	nextMarkdown, err := ReplaceMarkdownSelection(
		currentScene.Markdown,
		request.StartByte,
		request.EndByte,
		request.OriginalText,
		request.ReplacementText,
	)
	if err != nil {
		return SceneDocument{}, err
	}
	nextScene := currentScene
	nextScene.Markdown = nextMarkdown
	sceneBytes, err := s.files.MarshalSceneDocument(nextScene)
	if err != nil {
		return SceneDocument{}, err
	}
	if bytes.Equal(sceneBytes, currentScene.Canonical) {
		return SceneDocument{}, ErrNoSceneChanges
	}
	if err := s.persistFiles(ctx, current.Path, "Accept AI patch "+request.RunID, map[string][]byte{
		filepath.ToSlash(filepath.Join("scenes", request.SceneID+".md")): sceneBytes,
	}, nil); err != nil {
		return SceneDocument{}, err
	}
	nextScene.Revision = ComputeRevision(sceneBytes)
	nextScene.Canonical = append([]byte(nil), sceneBytes...)
	return nextScene, nil
}

// CodexEntries returns the current active project's validated Codex list.
func (s *Service) CodexEntries(ctx context.Context) ([]codex.Entry, error) {
	s.mutations.RLock()
	defer s.mutations.RUnlock()

	current, err := s.currentProject()
	if err != nil {
		return nil, err
	}
	return s.files.LoadCodexEntries(ctx, current.Path)
}

// LoadCodexEntry returns one validated canonical Codex entry.
func (s *Service) LoadCodexEntry(ctx context.Context, entryID string) (codex.Entry, error) {
	s.mutations.RLock()
	defer s.mutations.RUnlock()

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
	// Step 1: resolve the active project before acquiring the shared mutation lock.
	current, err := s.currentProject()
	if err != nil {
		return codex.Entry{}, err
	}
	s.mutations.Lock()
	defer s.mutations.Unlock()
	// Step 3: verify the worktree is clean before loading canonical state.
	clean, err := s.git.IsClean(ctx, current.Path)
	if err != nil {
		return codex.Entry{}, err
	}
	if !clean {
		return codex.Entry{}, ErrDirtyWorktree
	}
	// Step 6: validate and normalize the complete next document in memory.
	request, err = codex.NormalizeCreateRequest(request)
	if err != nil {
		return codex.Entry{}, err
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
	// Steps 9-11: atomically replace, rebuild the index, and checkpoint.
	if err := s.commitCodexMutation(ctx, current.Path, "Create Codex entry "+nextEntry.ID, relativePath, entryBytes); err != nil {
		return codex.Entry{}, err
	}
	// Step 12: return the representation of the exact bytes written.
	nextEntry.Revision = codex.ComputeRevision(entryBytes)
	nextEntry.Canonical = append([]byte(nil), entryBytes...)
	return nextEntry, nil
}

// UpdateCodexEntry edits one existing canonical Codex entry.
func (s *Service) UpdateCodexEntry(ctx context.Context, entryID string, request codex.SaveEntryRequest) (codex.Entry, error) {
	// Step 1: resolve the active project and validate route/request syntax before
	// acquiring the lock so 400s never touch the shared lock or the worktree check.
	current, err := s.currentProject()
	if err != nil {
		return codex.Entry{}, err
	}
	if err := codex.ValidateEntryID(entryID); err != nil {
		return codex.Entry{}, err
	}
	if err := codex.ValidateRevision(request.ExpectedRevision); err != nil {
		return codex.Entry{}, err
	}
	s.mutations.Lock()
	defer s.mutations.Unlock()
	// Step 3: verify the worktree is clean.
	clean, err := s.git.IsClean(ctx, current.Path)
	if err != nil {
		return codex.Entry{}, err
	}
	if !clean {
		return codex.Entry{}, ErrDirtyWorktree
	}
	// Step 4: strictly reload the canonical state needed for the decision.
	currentEntry, err := s.files.LoadCodexEntry(ctx, current.Path, entryID)
	if err != nil {
		return codex.Entry{}, err
	}
	// Step 5: compare the expected revision against the loaded canonical state.
	if currentEntry.Revision != request.ExpectedRevision {
		return codex.Entry{}, fmt.Errorf("entry %q revision changed: %w", entryID, ErrStaleRevision)
	}
	// Step 6: validate and normalize the complete next document in memory.
	nextEntry, err := codex.NormalizeUpdateRequest(entryID, currentEntry, request)
	if err != nil {
		return codex.Entry{}, err
	}
	entryBytes, err := s.files.MarshalCodexEntry(nextEntry)
	if err != nil {
		return codex.Entry{}, err
	}
	// Step 7: detect byte-identical no-op updates against the stored canonical bytes.
	if bytes.Equal(entryBytes, currentEntry.Canonical) {
		return codex.Entry{}, codex.ErrNoChanges
	}
	relativePath, err := codexEntryPath(nextEntry)
	if err != nil {
		return codex.Entry{}, err
	}
	// Steps 9-11: atomically replace, rebuild the index, and checkpoint.
	if err := s.commitCodexMutation(ctx, current.Path, "Edit Codex entry "+nextEntry.ID, relativePath, entryBytes); err != nil {
		return codex.Entry{}, err
	}
	// Step 12: return the representation of the exact bytes written.
	nextEntry.Revision = codex.ComputeRevision(entryBytes)
	nextEntry.Canonical = append([]byte(nil), entryBytes...)
	return nextEntry, nil
}

// LoadProgressions returns one entry's canonical progression document or an empty logical document.
func (s *Service) LoadProgressions(ctx context.Context, entryID string) (codex.ProgressionDocument, error) {
	s.mutations.RLock()
	defer s.mutations.RUnlock()

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
	// Step 1: resolve the active project and validate route/request syntax before
	// acquiring the lock so 400s never touch the shared lock or the worktree check.
	current, err := s.currentProject()
	if err != nil {
		return codex.ProgressionDocument{}, err
	}
	if err := codex.ValidateEntryID(entryID); err != nil {
		return codex.ProgressionDocument{}, err
	}
	if err := validateProgressionExpectedRevision(request.ExpectedRevision); err != nil {
		return codex.ProgressionDocument{}, err
	}
	s.mutations.Lock()
	defer s.mutations.Unlock()
	// Step 3: verify the worktree is clean.
	clean, err := s.git.IsClean(ctx, current.Path)
	if err != nil {
		return codex.ProgressionDocument{}, err
	}
	if !clean {
		return codex.ProgressionDocument{}, ErrDirtyWorktree
	}
	// Step 4: strictly reload all canonical state needed for the decision.
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
	// Step 5: compare the expected revision against the loaded canonical state.
	// A null expected_revision is only valid for first creation; an existing
	// document requires a non-null revision token, and a mismatch is stale.
	if request.ExpectedRevision == nil && currentDocument.Revision != nil {
		return codex.ProgressionDocument{}, codex.ErrInvalidRevision
	}
	if request.ExpectedRevision != nil && currentDocument.Revision == nil {
		return codex.ProgressionDocument{}, codex.ErrInvalidRevision
	}
	if request.ExpectedRevision != nil && currentDocument.Revision != nil && *request.ExpectedRevision != *currentDocument.Revision {
		return codex.ProgressionDocument{}, fmt.Errorf("progressions %q revision changed: %w", entryID, ErrStaleRevision)
	}
	// Saving an empty list when no progression file exists is a no-op error.
	if len(request.Progressions) == 0 && currentDocument.Revision == nil {
		return codex.ProgressionDocument{}, codex.ErrNoChanges
	}
	// Step 6: validate and normalize the complete next document in memory.
	sceneIDs := outlineSceneIDs(outline)
	progressions := append([]codex.Progression(nil), request.Progressions...)
	progressions, needsID, err := codex.ReconcileProgressionIDs(currentDocument.Progressions, progressions)
	if err != nil {
		return codex.ProgressionDocument{}, err
	}
	for _, index := range needsID {
		id, err := s.ids.Next(NodeKindProgression)
		if err != nil {
			return codex.ProgressionDocument{}, err
		}
		progressions[index].ID = id
	}
	progressions, err = codex.NormalizeProgressions(entryID, progressions, sceneIDs)
	if err != nil {
		return codex.ProgressionDocument{}, err
	}
	nextDocument := codex.ProgressionDocument{
		Version:      codex.Version,
		EntryID:      entryID,
		Progressions: progressions,
	}
	documentBytes, err := s.files.MarshalProgressions(nextDocument)
	if err != nil {
		return codex.ProgressionDocument{}, err
	}
	// Step 7: detect byte-identical no-op updates against the stored canonical bytes.
	if currentDocument.Canonical != nil && bytes.Equal(documentBytes, currentDocument.Canonical) {
		return codex.ProgressionDocument{}, codex.ErrNoChanges
	}
	// Steps 9-11: atomically replace, rebuild the index, and checkpoint.
	relativePath := filepath.ToSlash(filepath.Join("progressions", entryID+".yaml"))
	if err := s.commitCodexMutation(ctx, current.Path, "Edit progressions "+entryID, relativePath, documentBytes); err != nil {
		return codex.ProgressionDocument{}, err
	}
	// Step 12: return the representation of the exact bytes written.
	revision := codex.ComputeRevision(documentBytes)
	nextDocument.Revision = &revision
	nextDocument.Canonical = append([]byte(nil), documentBytes...)
	return nextDocument, nil
}

// ResolveActiveCodexState reads one entry and resolves its active state for a target scene.
func (s *Service) ResolveActiveCodexState(ctx context.Context, entryID, sceneID string) (codex.ActiveState, error) {
	s.mutations.RLock()
	defer s.mutations.RUnlock()

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

// ApplyImportMutation creates one canonical artifact without rebuilding the
// index or creating a checkpoint. The caller owns any surrounding transaction.
func (s *Service) ApplyImportMutation(ctx context.Context, request ImportMutationRequest) (ImportMutationResult, error) {
	s.mutations.Lock()
	defer s.mutations.Unlock()
	return s.ApplyImportMutationInTransaction(ctx, request)
}

// ApplyImportMutationInTransaction creates one canonical artifact while the
// caller holds the shared application mutation coordinator.
func (s *Service) ApplyImportMutationInTransaction(ctx context.Context, request ImportMutationRequest) (ImportMutationResult, error) {
	current, err := s.currentProject()
	if err != nil {
		return ImportMutationResult{}, err
	}

	switch request.Kind {
	case ImportMutationArc:
		return s.applyImportArc(ctx, current.Path, request.Title)
	case ImportMutationChapter:
		return s.applyImportChapter(ctx, current.Path, request.ParentID, request.Title)
	case ImportMutationScene:
		return s.applyImportScene(ctx, current.Path, request.ParentID, request.Title)
	case ImportMutationCodex:
		return s.applyImportCodex(ctx, current.Path, request.Codex)
	default:
		return ImportMutationResult{}, fmt.Errorf("unknown import mutation kind %q", request.Kind)
	}
}

func (s *Service) persistMutation(ctx context.Context, projectPath, changedID, message string, files map[string][]byte) (MutationResult, error) {
	var reloaded Outline
	if err := s.persistFiles(ctx, projectPath, message, files, func() error {
		next, err := s.files.Load(ctx, projectPath)
		if err != nil {
			return err
		}
		reloaded = next
		return nil
	}); err != nil {
		return MutationResult{}, err
	}
	return MutationResult{ChangedID: changedID, Outline: reloaded}, nil
}

// commitCodexMutation atomically replaces one canonical file, rebuilds the
// disposable index, and creates exactly one Git commit. On write failure it
// returns immediately without rollback work; on index or checkpoint failure
// it restores the target, unstages app changes, and rebuilds the index from
// restored files. The caller owns pre-request validation and no-op detection.
func (s *Service) commitCodexMutation(ctx context.Context, projectPath, message, relativePath string, contents []byte) error {
	return s.persistFiles(ctx, projectPath, message, map[string][]byte{relativePath: contents}, nil)
}

func (s *Service) persistFiles(ctx context.Context, projectPath, message string, files map[string][]byte, afterWrite func() error) error {
	rollback, err := s.files.WriteFiles(ctx, projectPath, files)
	if err != nil {
		return err
	}
	if afterWrite != nil {
		if err := afterWrite(); err != nil {
			return s.rollbackMutation(ctx, projectPath, rollback, err)
		}
	}
	if err := s.index.Rebuild(ctx, projectPath); err != nil {
		return s.rollbackMutation(ctx, projectPath, rollback, err)
	}
	if err := s.git.CommitAll(ctx, projectPath, message); err != nil {
		return s.rollbackMutation(ctx, projectPath, rollback, err)
	}
	return nil
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

func (s *Service) applyImportArc(ctx context.Context, projectPath, title string) (ImportMutationResult, error) {
	outline, err := s.files.Load(ctx, projectPath)
	if err != nil {
		return ImportMutationResult{}, err
	}
	arcID, err := s.nextUnusedID(ctx, projectPath, NodeKindArc)
	if err != nil {
		return ImportMutationResult{}, err
	}
	next, err := AddArc(outline, arcID, title)
	if err != nil {
		return ImportMutationResult{}, err
	}
	arc := next.Arcs[len(next.Arcs)-1]
	outlineBytes, err := s.files.MarshalOutline(next)
	if err != nil {
		return ImportMutationResult{}, err
	}
	arcBytes, err := s.files.MarshalArc(arc)
	if err != nil {
		return ImportMutationResult{}, err
	}
	rollback, err := s.files.WriteFiles(ctx, projectPath, map[string][]byte{
		"outline.yaml": outlineBytes,
		filepath.ToSlash(filepath.Join("arcs", arcID+".yaml")): arcBytes,
	})
	if err != nil {
		return ImportMutationResult{}, err
	}
	return ImportMutationResult{Kind: ImportMutationArc, ID: arcID, Rollback: rollback}, nil
}

func (s *Service) applyImportChapter(ctx context.Context, projectPath, arcID, title string) (ImportMutationResult, error) {
	outline, err := s.files.Load(ctx, projectPath)
	if err != nil {
		return ImportMutationResult{}, err
	}
	chapterID, err := s.nextUnusedID(ctx, projectPath, NodeKindChapter)
	if err != nil {
		return ImportMutationResult{}, err
	}
	next, err := AddChapter(outline, arcID, chapterID, title)
	if err != nil {
		return ImportMutationResult{}, err
	}
	chapter, err := findChapter(next, chapterID)
	if err != nil {
		return ImportMutationResult{}, err
	}
	outlineBytes, err := s.files.MarshalOutline(next)
	if err != nil {
		return ImportMutationResult{}, err
	}
	chapterBytes, err := s.files.MarshalChapter(chapter)
	if err != nil {
		return ImportMutationResult{}, err
	}
	rollback, err := s.files.WriteFiles(ctx, projectPath, map[string][]byte{
		"outline.yaml": outlineBytes,
		filepath.ToSlash(filepath.Join("chapters", chapterID+".yaml")): chapterBytes,
	})
	if err != nil {
		return ImportMutationResult{}, err
	}
	return ImportMutationResult{Kind: ImportMutationChapter, ID: chapterID, Rollback: rollback}, nil
}

func (s *Service) applyImportScene(ctx context.Context, projectPath, chapterID, title string) (ImportMutationResult, error) {
	outline, err := s.files.Load(ctx, projectPath)
	if err != nil {
		return ImportMutationResult{}, err
	}
	sceneID, err := s.nextUnusedID(ctx, projectPath, NodeKindScene)
	if err != nil {
		return ImportMutationResult{}, err
	}
	next, err := AddScene(outline, chapterID, sceneID, title)
	if err != nil {
		return ImportMutationResult{}, err
	}
	scene, err := findScene(next, sceneID)
	if err != nil {
		return ImportMutationResult{}, err
	}
	outlineBytes, err := s.files.MarshalOutline(next)
	if err != nil {
		return ImportMutationResult{}, err
	}
	sceneBytes, err := s.files.MarshalScene(scene)
	if err != nil {
		return ImportMutationResult{}, err
	}
	rollback, err := s.files.WriteFiles(ctx, projectPath, map[string][]byte{
		"outline.yaml": outlineBytes,
		filepath.ToSlash(filepath.Join("scenes", sceneID+".md")): sceneBytes,
	})
	if err != nil {
		return ImportMutationResult{}, err
	}
	return ImportMutationResult{Kind: ImportMutationScene, ID: sceneID, Rollback: rollback}, nil
}

func (s *Service) applyImportCodex(ctx context.Context, projectPath string, request codex.SaveEntryRequest) (ImportMutationResult, error) {
	request, err := codex.NormalizeCreateRequest(request)
	if err != nil {
		return ImportMutationResult{}, err
	}
	entryID, err := s.nextUnusedID(ctx, projectPath, codexNodeKind(request.Type))
	if err != nil {
		return ImportMutationResult{}, err
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
		return ImportMutationResult{}, err
	}
	entryBytes, err := s.files.MarshalCodexEntry(nextEntry)
	if err != nil {
		return ImportMutationResult{}, err
	}
	relativePath, err := codexEntryPath(nextEntry)
	if err != nil {
		return ImportMutationResult{}, err
	}
	rollback, err := s.files.WriteFiles(ctx, projectPath, map[string][]byte{relativePath: entryBytes})
	if err != nil {
		return ImportMutationResult{}, err
	}
	return ImportMutationResult{Kind: ImportMutationCodex, ID: entryID, Rollback: rollback}, nil
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

// validateProgressionExpectedRevision validates the request-side shape of a
// progression save's expected revision token. A nil token selects first-creation
// mode; a present token must match the SHA-256 revision shape. The value
// comparison against the loaded canonical state happens under the lock.
func validateProgressionExpectedRevision(expected *string) error {
	if expected == nil {
		return nil
	}
	if *expected == "" {
		return codex.ErrInvalidRevision
	}
	return codex.ValidateRevision(*expected)
}

// outlineSceneIDs builds the membership set used to validate stable anchors.
func outlineSceneIDs(outline Outline) map[string]struct{} {
	sceneIDs := make(map[string]struct{})
	for _, scene := range flattenOutlineScenes(outline) {
		sceneIDs[scene.ID] = struct{}{}
	}
	return sceneIDs
}

// flattenOutlineScenes converts canonical hierarchy order into active-state chronology.
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

// validateProgressionAnchors rejects stored anchors absent from the current outline.
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
