package projectcheck

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"storywork/internal/agent"
	"storywork/internal/codex"
	"storywork/internal/importer"
	"storywork/internal/project"
	"storywork/internal/story"
	"storywork/internal/storyfile"
)

// ErrInvalidProject reports malformed, incomplete, or relationship-invalid
// canonical project state.
var ErrInvalidProject = errors.New("invalid canonical project")

// StoryReader provides read-only access to outline, codex, and progression files.
type StoryReader interface {
	Load(ctx context.Context, projectPath string) (story.Outline, error)
	ValidateCanonicalFiles(ctx context.Context, projectPath string, outline story.Outline) error
	LoadCodexEntries(ctx context.Context, projectPath string) ([]codex.Entry, error)
	LoadProgressions(ctx context.Context, projectPath, entryID string) (codex.ProgressionDocument, error)
}

// RegistryReader provides read-only access to agent and style registries.
type RegistryReader interface {
	LoadAgents(projectPath string) ([]agent.Agent, error)
	LoadStyles(projectPath string) ([]agent.Style, error)
}

// Validator composes owning-package read-only validators.
type Validator struct {
	files    StoryReader
	agents   RegistryReader
	metadata func(string) (string, string, error)
	imports  func(context.Context, string) error
}

// Option configures a Validator at construction time.
type Option func(*Validator)

// WithMetadataFunc overrides the project metadata validation function.
func WithMetadataFunc(fn func(string) (string, string, error)) Option {
	return func(v *Validator) { v.metadata = fn }
}

// WithImportsFunc overrides the import artifact validation function.
func WithImportsFunc(fn func(context.Context, string) error) Option {
	return func(v *Validator) { v.imports = fn }
}

// New creates a project validator with the default owning-package readers.
func New() *Validator {
	return &Validator{
		files:    storyfile.New(),
		agents:   agent.NewLoader(),
		metadata: project.ValidateMetadataFile,
		imports:  importer.ValidateStoredSnapshots,
	}
}

// NewWithReaders creates a project validator with injected readers for testing.
func NewWithReaders(files StoryReader, agents RegistryReader, opts ...Option) *Validator {
	v := &Validator{
		files:    files,
		agents:   agents,
		metadata: project.ValidateMetadataFile,
		imports:  importer.ValidateStoredSnapshots,
	}
	for _, opt := range opts {
		opt(v)
	}
	return v
}

// ValidateProject validates the complete canonical project at projectPath.
func (v *Validator) ValidateProject(ctx context.Context, projectPath string) error {
	if err := v.validateProjectMetadata(projectPath); err != nil {
		return classifyValidationError(err)
	}
	outline, err := v.files.Load(ctx, projectPath)
	if err != nil {
		return classifyValidationError(fmt.Errorf("outline validation failed: %w", err))
	}
	if err := v.files.ValidateCanonicalFiles(ctx, projectPath, outline); err != nil {
		return classifyValidationError(err)
	}
	sceneIDs := make(map[string]struct{})
	for _, arc := range outline.Arcs {
		for _, chapter := range arc.Chapters {
			for _, scene := range chapter.Scenes {
				sceneIDs[scene.ID] = struct{}{}
			}
		}
	}
	entries, err := v.files.LoadCodexEntries(ctx, projectPath)
	if err != nil {
		return classifyValidationError(fmt.Errorf("codex validation failed: %w", err))
	}
	if err := validateCodexArtifacts(projectPath); err != nil {
		return classifyValidationError(fmt.Errorf("codex validation failed: %w", err))
	}
	entryIDs := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		entryIDs[entry.ID] = struct{}{}
	}
	progressionFiles, err := filepath.Glob(filepath.Join(projectPath, "progressions", "*.yaml"))
	if err != nil {
		return fmt.Errorf("list progressions: %w", err)
	}
	for _, progressionFile := range progressionFiles {
		entryID := strings.TrimSuffix(filepath.Base(progressionFile), ".yaml")
		if _, ok := entryIDs[entryID]; !ok {
			return classifyValidationError(fmt.Errorf("progression validation failed: entry %q does not exist", entryID))
		}
		document, err := v.files.LoadProgressions(ctx, projectPath, entryID)
		if err != nil {
			return classifyValidationError(fmt.Errorf("progression validation failed for %q: %w", entryID, err))
		}
		if _, err := codex.NormalizeProgressions(entryID, document.Progressions, sceneIDs); err != nil {
			return classifyValidationError(fmt.Errorf("progression validation failed for %q: %w", entryID, err))
		}
	}
	if _, err := v.agents.LoadAgents(projectPath); err != nil {
		return classifyValidationError(fmt.Errorf("agent registry validation failed: %w", err))
	}
	if _, err := v.agents.LoadStyles(projectPath); err != nil {
		return classifyValidationError(fmt.Errorf("style registry validation failed: %w", err))
	}
	if err := v.validateImportArtifacts(ctx, projectPath); err != nil {
		return classifyValidationError(err)
	}
	return nil
}

func validateCodexArtifacts(projectPath string) error {
	allowedDirectories := make(map[string]struct{}, 4)
	for _, entryType := range []codex.EntryType{codex.TypeCharacter, codex.TypeLocation, codex.TypeLore, codex.TypeCustom} {
		directory, err := codex.DirectoryForType(entryType)
		if err != nil {
			return err
		}
		allowedDirectories[directory] = struct{}{}
	}
	root := filepath.Join(projectPath, "codex")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.Name() == ".gitkeep" {
			continue
		}
		if !entry.IsDir() {
			return fmt.Errorf("unexpected file codex/%s: %w", entry.Name(), storyfile.ErrInvalidCanonicalState)
		}
		if _, ok := allowedDirectories[entry.Name()]; !ok {
			return fmt.Errorf("unexpected directory codex/%s: %w", entry.Name(), storyfile.ErrInvalidCanonicalState)
		}
		childEntries, err := os.ReadDir(filepath.Join(root, entry.Name()))
		if err != nil {
			return err
		}
		for _, child := range childEntries {
			if child.Name() == ".gitkeep" {
				continue
			}
			if child.IsDir() {
				return fmt.Errorf("unexpected directory codex/%s/%s: %w", entry.Name(), child.Name(), storyfile.ErrInvalidCanonicalState)
			}
			if filepath.Ext(child.Name()) != ".yaml" {
				return fmt.Errorf("unexpected file codex/%s/%s: %w", entry.Name(), child.Name(), storyfile.ErrInvalidCanonicalState)
			}
		}
	}
	return nil
}

func (v *Validator) validateProjectMetadata(projectPath string) error {
	_, _, err := v.metadata(filepath.Join(projectPath, "project.yaml"))
	if err != nil {
		return fmt.Errorf("project metadata validation failed: %w", err)
	}
	return nil
}

func (v *Validator) validateImportArtifacts(ctx context.Context, projectPath string) error {
	if err := v.imports(ctx, projectPath); err != nil {
		return fmt.Errorf("raw import validation failed: %w", err)
	}
	reviewDir := filepath.Join(projectPath, "imports", "review")
	entries, err := os.ReadDir(reviewDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	store := importer.NewCandidateStore()
	for _, entry := range entries {
		if entry.IsDir() {
			return fmt.Errorf("import candidate %q invalid: %w", entry.Name(), importer.ErrInvalidCandidate)
		}
		if !strings.HasSuffix(entry.Name(), ".yaml") {
			return fmt.Errorf("import candidate %q invalid: %w", entry.Name(), importer.ErrInvalidCandidate)
		}
		id := strings.TrimSuffix(entry.Name(), ".yaml")
		if err := importer.ValidateCandidateID(id); err != nil {
			return fmt.Errorf("import candidate %q invalid: %w", id, err)
		}
		if _, err := store.Load(projectPath, id); err != nil {
			return fmt.Errorf("import candidate %q invalid: %w", id, err)
		}
	}
	return nil
}

func classifyValidationError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, project.ErrInvalidMetadata) ||
		errors.Is(err, storyfile.ErrInvalidCanonicalState) ||
		errors.Is(err, story.ErrInvalidTitle) ||
		errors.Is(err, story.ErrInvalidID) ||
		errors.Is(err, story.ErrParentNotFound) ||
		errors.Is(err, story.ErrInvalidPOV) ||
		errors.Is(err, story.ErrInvalidStatus) ||
		errors.Is(err, story.ErrInvalidMarkdown) ||
		errors.Is(err, story.ErrInvalidRevision) ||
		errors.Is(err, story.ErrInvalidSelection) ||
		errors.Is(err, codex.ErrInvalidType) ||
		errors.Is(err, codex.ErrInvalidID) ||
		errors.Is(err, codex.ErrInvalidName) ||
		errors.Is(err, codex.ErrInvalidAlias) ||
		errors.Is(err, codex.ErrInvalidTag) ||
		errors.Is(err, codex.ErrInvalidDescription) ||
		errors.Is(err, codex.ErrInvalidMetadata) ||
		errors.Is(err, codex.ErrInvalidRevision) ||
		errors.Is(err, codex.ErrInvalidProgression) ||
		errors.Is(err, codex.ErrEntryNotFound) ||
		errors.Is(err, codex.ErrSceneNotFound) ||
		errors.Is(err, agent.ErrInvalidAgent) ||
		errors.Is(err, agent.ErrInvalidStyle) ||
		errors.Is(err, importer.ErrInvalidID) ||
		errors.Is(err, importer.ErrInvalidManifest) ||
		errors.Is(err, importer.ErrInvalidPath) ||
		errors.Is(err, importer.ErrCaseFoldedCollision) ||
		errors.Is(err, importer.ErrInvalidCandidate) {
		return errors.Join(ErrInvalidProject, err)
	}
	return err
}
