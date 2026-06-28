// Package project creates and opens portable story project folders.
package project

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	starter "storywork/templates"
)

var nonIdentifier = regexp.MustCompile(`[^a-z0-9]+`)

// GitStore is the project history boundary.
type GitStore interface {
	Init(ctx context.Context, path string) error
	CommitAll(ctx context.Context, path, message string) error
	IsRepo(ctx context.Context, path string) (bool, error)
}

// IndexStore is the disposable index boundary.
type IndexStore interface {
	Init(ctx context.Context, projectPath string) error
	Rebuild(ctx context.Context, projectPath string) error
	Verify(ctx context.Context, projectPath string) error
}

// CreateRequest contains author-controlled project metadata.
type CreateRequest struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// Project is the transport-safe project summary.
type Project struct {
	ID               string `json:"project_id"`
	Name             string `json:"name,omitempty"`
	Path             string `json:"path"`
	GitInitialized   bool   `json:"git_initialized"`
	IndexInitialized bool   `json:"index_initialized"`
}

// Service coordinates canonical files, Git history, and the derived index.
type Service struct {
	git   GitStore
	index IndexStore
	now   func() time.Time
}

// NewService creates a project service.
func NewService(git GitStore, index IndexStore, now func() time.Time) *Service {
	return &Service{git: git, index: index, now: now}
}

// Create writes a new project and refuses to overwrite a non-empty directory.
func (s *Service) Create(ctx context.Context, request CreateRequest) (Project, error) {
	request.Name = strings.TrimSpace(request.Name)
	request.Path = filepath.Clean(strings.TrimSpace(request.Path))
	if request.Name == "" {
		return Project{}, errors.New("project name is required")
	}
	if request.Path == "." || !filepath.IsAbs(request.Path) {
		return Project{}, errors.New("project path must be absolute")
	}
	if err := ensureEmptyPath(request.Path); err != nil {
		return Project{}, err
	}
	createdPath := false
	creationComplete := false
	if _, err := os.Stat(request.Path); os.IsNotExist(err) {
		createdPath = true
	}
	if err := os.MkdirAll(request.Path, 0o755); err != nil {
		return Project{}, fmt.Errorf("create project directory: %w", err)
	}
	if createdPath {
		defer func() {
			if !creationComplete {
				_ = os.RemoveAll(request.Path)
			}
		}()
	}

	projectID := identifier(request.Name)
	if err := writeStarters(request.Path, projectID, request.Name, s.now().UTC()); err != nil {
		return Project{}, err
	}
	if err := s.git.Init(ctx, request.Path); err != nil {
		return Project{}, err
	}
	if err := s.index.Init(ctx, request.Path); err != nil {
		return Project{}, err
	}
	if err := s.git.CommitAll(ctx, request.Path, "Initialize story project"); err != nil {
		return Project{}, err
	}
	creationComplete = true

	return Project{ID: projectID, Name: request.Name, Path: request.Path, GitInitialized: true, IndexInitialized: true}, nil
}

// Open validates canonical metadata and Git, rebuilding a missing or invalid index.
func (s *Service) Open(ctx context.Context, path string) (Project, error) {
	path = filepath.Clean(strings.TrimSpace(path))
	if !filepath.IsAbs(path) {
		return Project{}, errors.New("project path must be absolute")
	}
	id, name, err := readMetadata(filepath.Join(path, "project.yaml"))
	if err != nil {
		return Project{}, fmt.Errorf("open project: %w", err)
	}
	isRepo, err := s.git.IsRepo(ctx, path)
	if err != nil {
		return Project{}, err
	}
	if !isRepo {
		return Project{}, errors.New("open project: directory is not a Git repository root")
	}
	if err := s.index.Verify(ctx, path); err != nil {
		if rebuildError := s.index.Rebuild(ctx, path); rebuildError != nil {
			return Project{}, fmt.Errorf("rebuild project index after verification failed (%v): %w", err, rebuildError)
		}
	}
	return Project{ID: id, Name: name, Path: path, GitInitialized: true, IndexInitialized: true}, nil
}

func ensureEmptyPath(path string) error {
	entries, err := os.ReadDir(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect project directory: %w", err)
	}
	if len(entries) != 0 {
		return errors.New("project directory must be empty")
	}
	return nil
}

func writeStarters(root, id, name string, createdAt time.Time) error {
	directories := []string{
		"arcs", "chapters", "scenes", "codex/characters", "codex/locations", "codex/lore", "codex/custom",
		"progressions", "agents", "styles", "imports/raw", "imports/processed", ".storywork/tmp",
	}
	for _, directory := range directories {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			return fmt.Errorf("create starter directory %q: %w", directory, err)
		}
		if !strings.HasPrefix(directory, ".storywork") {
			if err := os.WriteFile(filepath.Join(root, directory, ".gitkeep"), nil, 0o644); err != nil {
				return fmt.Errorf("write starter directory marker: %w", err)
			}
		}
	}

	projectTemplate, err := fs.ReadFile(starter.Files, "project.yaml")
	if err != nil {
		return fmt.Errorf("read embedded project template: %w", err)
	}
	projectContents := strings.ReplaceAll(string(projectTemplate), "proj_example", id)
	projectContents = strings.ReplaceAll(projectContents, "Example Novel", yamlScalar(name))
	projectContents = strings.ReplaceAll(projectContents, "REPLACE_ME", createdAt.Format(time.RFC3339))
	files := map[string][]byte{
		"project.yaml":               []byte(projectContents),
		"outline.yaml":               mustReadTemplate("outline.yaml"),
		".gitignore":                 mustReadTemplate("story_project.gitignore"),
		"agents/line_polish.yaml":    mustReadTemplate("builtin_agent_line_polish.yaml"),
		"styles/precise_editor.yaml": mustReadTemplate("builtin_style_precise_editor.yaml"),
	}
	for path, contents := range files {
		if contents == nil {
			return fmt.Errorf("read embedded starter template for %q", path)
		}
		if err := os.WriteFile(filepath.Join(root, path), contents, 0o644); err != nil {
			return fmt.Errorf("write starter file %q: %w", path, err)
		}
	}
	return nil
}

func mustReadTemplate(name string) []byte {
	contents, err := fs.ReadFile(starter.Files, name)
	if err != nil {
		return nil
	}
	return contents
}

func identifier(name string) string {
	normalized := nonIdentifier.ReplaceAllString(strings.ToLower(name), "_")
	normalized = strings.Trim(normalized, "_")
	if normalized == "" {
		normalized = "project"
	}
	return "proj_" + normalized
}

func yamlScalar(value string) string {
	return strconv.Quote(value)
}

func readMetadata(path string) (string, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", "", fmt.Errorf("read project.yaml: %w", err)
	}
	defer file.Close()

	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		key, value, found := strings.Cut(line, ":")
		if found && (key == "id" || key == "name") {
			values[key] = strings.Trim(strings.TrimSpace(value), `"'`)
		}
	}
	if err := scanner.Err(); err != nil {
		return "", "", fmt.Errorf("scan project.yaml: %w", err)
	}
	if values["id"] == "" || values["name"] == "" {
		return "", "", errors.New("project.yaml must contain id and name")
	}
	return values["id"], values["name"], nil
}
