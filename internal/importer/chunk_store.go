package importer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"gopkg.in/yaml.v3"
)

type ChunkStore struct{}

func NewChunkStore() *ChunkStore {
	return &ChunkStore{}
}

func (s *ChunkStore) ListOrRebuild(ctx context.Context, projectPath, importID string) ([]Chunk, error) {
	if err := ValidateImportID(importID); err != nil {
		return nil, err
	}
	cachePath := filepath.Join(projectPath, ".storywork", "import", importID, "chunks.json")
	manifestPath := filepath.Join(projectPath, "imports", "raw", importID, "manifest.yaml")
	if chunks, err := s.loadChunks(cachePath); err == nil {
		return chunks, nil
	}
	manifest, err := loadManifest(manifestPath)
	if err != nil {
		return nil, err
	}
	chunks := make([]Chunk, 0, len(manifest.Files))
	for _, file := range manifest.Files {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		sourceBytes, err := os.ReadFile(filepath.Join(projectPath, "imports", "raw", importID, "files", filepath.FromSlash(file.Path)))
		if err != nil {
			return nil, fmt.Errorf("read imported markdown %q: %w", file.Path, err)
		}
		fileChunks, err := ChunkMarkdown(importID, file.Path, string(sourceBytes))
		if err != nil {
			return nil, err
		}
		chunks = append(chunks, fileChunks...)
	}
	if err := s.writeChunks(cachePath, chunks); err != nil {
		return nil, err
	}
	return chunks, nil
}

func (s *ChunkStore) loadChunks(path string) ([]Chunk, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var chunks []Chunk
	if err := json.Unmarshal(body, &chunks); err != nil {
		return nil, err
	}
	for _, chunk := range chunks {
		if err := ValidateChunkID(chunk.ID); err != nil {
			return nil, err
		}
	}
	slices.SortFunc(chunks, func(left, right Chunk) int {
		if left.SourcePath != right.SourcePath {
			if left.SourcePath < right.SourcePath {
				return -1
			}
			return 1
		}
		if left.StartLine < right.StartLine {
			return -1
		}
		if left.StartLine > right.StartLine {
			return 1
		}
		return 0
	})
	return chunks, nil
}

func (s *ChunkStore) writeChunks(path string, chunks []Chunk) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create chunk cache directory: %w", err)
	}
	body, err := json.MarshalIndent(chunks, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal chunk cache: %w", err)
	}
	temporaryPath := path + ".tmp"
	if err := os.WriteFile(temporaryPath, body, 0o644); err != nil {
		return fmt.Errorf("write chunk cache: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace chunk cache: %w", err)
	}
	return nil
}

func loadManifest(path string) (ImportManifest, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return ImportManifest{}, fmt.Errorf("read import manifest: %w", err)
	}
	var decoded struct {
		Version   int          `yaml:"version"`
		ID        string       `yaml:"id"`
		CreatedAt string       `yaml:"created_at"`
		Files     []ImportFile `yaml:"files"`
	}
	if err := yaml.Unmarshal(body, &decoded); err != nil {
		return ImportManifest{}, fmt.Errorf("decode import manifest: %w", err)
	}
	createdAt, err := time.Parse(time.RFC3339, decoded.CreatedAt)
	if err != nil {
		return ImportManifest{}, fmt.Errorf("parse import manifest created_at: %w", err)
	}
	manifest := ImportManifest{
		Version:   decoded.Version,
		ID:        decoded.ID,
		CreatedAt: createdAt,
		Files:     decoded.Files,
	}
	if err := manifest.Validate(); err != nil {
		return ImportManifest{}, err
	}
	return manifest, nil
}
