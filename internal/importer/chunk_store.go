package importer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"storywork/internal/agent"

	"gopkg.in/yaml.v3"
)

type ChunkStore struct{}

// ExtractionAttempt records a successful derived extraction without retaining
// prompts, source text, credentials, endpoints, or raw provider output.
type ExtractionAttempt struct {
	Version      int                    `json:"version"`
	ImportID     string                 `json:"import_id"`
	Mode         string                 `json:"mode"`
	ChunkIDs     []string               `json:"chunk_ids"`
	CandidateIDs []string               `json:"candidate_ids"`
	Provider     agent.ProviderIdentity `json:"provider"`
}

func NewChunkStore() *ChunkStore {
	return &ChunkStore{}
}

func (s *ChunkStore) ListOrRebuild(ctx context.Context, projectPath, importID string) ([]Chunk, error) {
	if err := ValidateImportID(importID); err != nil {
		return nil, err
	}
	cachePath := filepath.Join(projectPath, ".storywork", "import", importID, "chunks.json")
	manifestPath := filepath.Join(projectPath, "imports", "raw", importID, "manifest.yaml")
	manifest, err := loadManifest(manifestPath)
	if err != nil {
		return nil, err
	}
	if manifest.ID != importID {
		return nil, fmt.Errorf("manifest identifier does not match import directory: %w", ErrInvalidManifest)
	}
	chunks := make([]Chunk, 0, len(manifest.Files))
	seenChunkIDs := make(map[string]Chunk)
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
		if int64(len(sourceBytes)) != file.Bytes || CanonicalSHA256(sourceBytes) != file.SHA256 {
			return nil, fmt.Errorf("imported markdown %q does not match its manifest: %w", file.Path, ErrInvalidManifest)
		}
		fileChunks, err := ChunkMarkdown(importID, file.Path, string(sourceBytes))
		if err != nil {
			return nil, err
		}
		for _, chunk := range fileChunks {
			if existing, found := seenChunkIDs[chunk.ID]; found && existing != chunk {
				return nil, fmt.Errorf("chunk identifier collision: %w", ErrInvalidManifest)
			}
			seenChunkIDs[chunk.ID] = chunk
			chunks = append(chunks, chunk)
		}
	}
	if cached, err := s.loadChunks(cachePath); err == nil && slices.Equal(cached, chunks) {
		return cached, nil
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
		if err := validateStoredChunk(chunk); err != nil {
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

func validateStoredChunk(chunk Chunk) error {
	if err := ValidateChunkID(chunk.ID); err != nil {
		return err
	}
	if err := ValidateImportID(chunk.ImportID); err != nil {
		return err
	}
	normalizedPath, err := NormalizePortableRelativePath(chunk.SourcePath)
	if err != nil {
		return err
	}
	if normalizedPath != chunk.SourcePath {
		return fmt.Errorf("stored chunk path is not normalized")
	}
	if chunk.StartLine < 1 || chunk.EndLine < chunk.StartLine || chunk.Text == "" || CanonicalSHA256([]byte(chunk.Text)) != chunk.SHA256 {
		return fmt.Errorf("stored chunk content is invalid")
	}
	return nil
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

// RecordExtractionAttempt atomically writes rebuildable metadata and returns a
// rollback function for the surrounding candidate-publication transaction.
func (s *ChunkStore) RecordExtractionAttempt(projectPath string, attempt ExtractionAttempt) (func() error, error) {
	if err := ValidateImportID(attempt.ImportID); err != nil {
		return nil, err
	}
	if len(attempt.CandidateIDs) == 0 {
		return nil, fmt.Errorf("extraction attempt requires candidates")
	}
	for _, candidateID := range attempt.CandidateIDs {
		if err := ValidateCandidateID(candidateID); err != nil {
			return nil, err
		}
	}
	path := filepath.Join(projectPath, ".storywork", "import", attempt.ImportID, "attempts", attempt.CandidateIDs[0]+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create extraction attempt directory: %w", err)
	}
	body, err := json.MarshalIndent(attempt, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal extraction attempt: %w", err)
	}
	temporaryPath := path + ".tmp"
	if err := os.WriteFile(temporaryPath, body, 0o600); err != nil {
		return nil, fmt.Errorf("write extraction attempt: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		_ = os.Remove(temporaryPath)
		return nil, fmt.Errorf("replace extraction attempt: %w", err)
	}
	return func() error {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}, nil
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
	decoder := yaml.NewDecoder(bytes.NewReader(body))
	decoder.KnownFields(true)
	if err := decoder.Decode(&decoded); err != nil {
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
