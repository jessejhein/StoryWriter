package projectcheck

import (
	"context"
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

// StoryReader provides read-only access to outline, codex, and progression files.
type StoryReader interface {
	Load(ctx context.Context, projectPath string) (story.Outline, error)
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
		return err
	}
	outline, err := v.files.Load(ctx, projectPath)
	if err != nil {
		return fmt.Errorf("outline validation failed: %w", err)
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
		return fmt.Errorf("codex validation failed: %w", err)
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
			return fmt.Errorf("progression validation failed: entry %q does not exist", entryID)
		}
		document, err := v.files.LoadProgressions(ctx, projectPath, entryID)
		if err != nil {
			return fmt.Errorf("progression validation failed for %q: %w", entryID, err)
		}
		if _, err := codex.NormalizeProgressions(entryID, document.Progressions, sceneIDs); err != nil {
			return fmt.Errorf("progression validation failed for %q: %w", entryID, err)
		}
	}
	if _, err := v.agents.LoadAgents(projectPath); err != nil {
		return fmt.Errorf("agent registry validation failed: %w", err)
	}
	if _, err := v.agents.LoadStyles(projectPath); err != nil {
		return fmt.Errorf("style registry validation failed: %w", err)
	}
	if err := v.validateImportArtifacts(ctx, projectPath); err != nil {
		return err
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
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".yaml")
		if _, err := store.Load(projectPath, id); err != nil {
			return fmt.Errorf("import candidate %q invalid: %w", id, err)
		}
	}
	return nil
}
