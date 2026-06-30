package agent

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Registry struct {
	Agents []Agent
	Styles []Style
}

type Loader struct{}

func NewLoader() *Loader {
	return &Loader{}
}

func (l *Loader) Load(projectPath string) (Registry, error) {
	agents, err := l.LoadAgents(projectPath)
	if err != nil {
		return Registry{}, err
	}
	styles, err := l.LoadStyles(projectPath)
	if err != nil {
		return Registry{}, err
	}
	return Registry{Agents: agents, Styles: styles}, nil
}

func (l *Loader) LoadAgents(projectPath string) ([]Agent, error) {
	files, err := readRegistryFiles(filepath.Join(projectPath, "agents"))
	if err != nil {
		return nil, err
	}
	items := make([]Agent, 0, len(files))
	seen := map[string]string{}
	for _, file := range files {
		contents, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", slashPath(file, projectPath), err)
		}
		var document struct {
			Version     int    `yaml:"version"`
			ID          string `yaml:"id"`
			Name        string `yaml:"name"`
			Description string `yaml:"description"`
			AppliesWhen struct {
				Surfaces    []Surface    `yaml:"surfaces"`
				InputScopes []InputScope `yaml:"input_scopes"`
				MinWords    int          `yaml:"min_words"`
				MaxWords    int          `yaml:"max_words"`
			} `yaml:"applies_when"`
			ContextPolicy struct {
				Required  []ContextPack `yaml:"required"`
				Optional  []ContextPack `yaml:"optional"`
				Forbidden []ContextPack `yaml:"forbidden"`
			} `yaml:"context_policy"`
			RAGPolicy struct {
				Mode RAGMode `yaml:"mode"`
			} `yaml:"rag_policy"`
			Control struct {
				OutputMode         OutputMode `yaml:"output_mode"`
				RequiresAcceptance bool       `yaml:"requires_acceptance"`
				CanModifyCanon     bool       `yaml:"can_modify_canon"`
			} `yaml:"control"`
			Output struct {
				Type                OutputType `yaml:"type"`
				RequiresDiffPreview bool       `yaml:"requires_diff_preview"`
			} `yaml:"output"`
		}
		if err := decodeSingleYAML(contents, &document); err != nil {
			return nil, fmt.Errorf("%s: %w", slashPath(file, projectPath), err)
		}
		item, err := ValidateAgent(Agent{
			Version:     document.Version,
			ID:          document.ID,
			Name:        document.Name,
			Description: document.Description,
			AppliesWhen: ApplicabilityRule{
				Surfaces:    document.AppliesWhen.Surfaces,
				InputScopes: document.AppliesWhen.InputScopes,
				MinWords:    document.AppliesWhen.MinWords,
				MaxWords:    document.AppliesWhen.MaxWords,
			},
			ContextPolicy: ContextPolicy{
				Required:  document.ContextPolicy.Required,
				Optional:  document.ContextPolicy.Optional,
				Forbidden: document.ContextPolicy.Forbidden,
			},
			RAGPolicy: RAGPolicy{Mode: document.RAGPolicy.Mode},
			Control: Control{
				OutputMode:         document.Control.OutputMode,
				RequiresAcceptance: document.Control.RequiresAcceptance,
				CanModifyCanon:     document.Control.CanModifyCanon,
			},
			Output: Output{
				Type:                document.Output.Type,
				RequiresDiffPreview: document.Output.RequiresDiffPreview,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("%s: %w", slashPath(file, projectPath), err)
		}
		if prior, ok := seen[item.ID]; ok {
			return nil, fmt.Errorf("duplicate agent id %q in %s and %s: %w", item.ID, prior, slashPath(file, projectPath), ErrRegistryLoad)
		}
		seen[item.ID] = slashPath(file, projectPath)
		items = append(items, item)
	}
	SortAgents(items)
	return items, nil
}

func (l *Loader) LoadStyles(projectPath string) ([]Style, error) {
	files, err := readRegistryFiles(filepath.Join(projectPath, "styles"))
	if err != nil {
		return nil, err
	}
	items := make([]Style, 0, len(files))
	seen := map[string]string{}
	for _, file := range files {
		contents, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", slashPath(file, projectPath), err)
		}
		var document struct {
			Version           int    `yaml:"version"`
			ID                string `yaml:"id"`
			Name              string `yaml:"name"`
			ProviderProfileID string `yaml:"provider_profile_id"`
			Model             string `yaml:"model"`
			Parameters        struct {
				Temperature float64 `yaml:"temperature"`
			} `yaml:"parameters"`
			SystemPrompt string `yaml:"system_prompt"`
		}
		if err := decodeSingleYAML(contents, &document); err != nil {
			return nil, fmt.Errorf("%s: %w", slashPath(file, projectPath), err)
		}
		item, err := ValidateStyle(Style{
			Version:           document.Version,
			ID:                document.ID,
			Name:              document.Name,
			ProviderProfileID: document.ProviderProfileID,
			Model:             document.Model,
			Temperature:       document.Parameters.Temperature,
			SystemPrompt:      document.SystemPrompt,
		})
		if err != nil {
			return nil, fmt.Errorf("%s: %w", slashPath(file, projectPath), err)
		}
		if prior, ok := seen[item.ID]; ok {
			return nil, fmt.Errorf("duplicate style id %q in %s and %s: %w", item.ID, prior, slashPath(file, projectPath), ErrRegistryLoad)
		}
		seen[item.ID] = slashPath(file, projectPath)
		items = append(items, item)
	}
	SortStyles(items)
	return items, nil
}

func decodeSingleYAML(contents []byte, target any) error {
	decoder := yaml.NewDecoder(bytes.NewReader(contents))
	decoder.KnownFields(true)
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("invalid YAML: multiple YAML documents are not supported")
		}
		return fmt.Errorf("invalid YAML: %w", err)
	}
	return nil
}

func readRegistryFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		info, err := os.Lstat(path)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("%s is a symbolic link: %w", path, ErrRegistryLoad)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("%s is not a regular file: %w", path, ErrRegistryLoad)
		}
		files = append(files, path)
	}
	return files, nil
}

func slashPath(path, projectPath string) string {
	relative, err := filepath.Rel(projectPath, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relative)
}
