package provider

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

type Store struct {
	path         string
	mu           sync.Mutex
	readFile     func(string) ([]byte, error)
	mkdirAll     func(string, os.FileMode) error
	rename       func(string, string) error
	remove       func(string) error
	openTempFile func(string, os.FileMode) (tempFile, error)
	syncDir      func(string) error
	lstat        func(string) (os.FileInfo, error)
}

func NewStore(path string) *Store {
	return &Store{
		path:         filepath.Clean(path),
		readFile:     os.ReadFile,
		mkdirAll:     os.MkdirAll,
		rename:       os.Rename,
		remove:       os.Remove,
		openTempFile: defaultOpenTempFile,
		syncDir:      defaultSyncDir,
		lstat:        os.Lstat,
	}
}

func (s *Store) Load(ctx context.Context) ([]Profile, *string, error) {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *Store) Save(ctx context.Context, profiles []Profile, expectedRevision *string) ([]Profile, *string, error) {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()

	currentProfiles, currentRevision, err := s.loadLocked()
	if err != nil {
		return nil, nil, err
	}
	switch {
	case currentRevision == nil && expectedRevision != nil:
		return nil, nil, ErrProfileRevisionConflict
	case currentRevision != nil && expectedRevision == nil:
		return nil, nil, ErrProfileRevisionConflict
	case currentRevision != nil && expectedRevision != nil && *currentRevision != *expectedRevision:
		return nil, nil, ErrProfileRevisionConflict
	}

	normalized, canonical, revision, err := ValidateProfiles(profiles)
	if err != nil {
		return nil, nil, err
	}
	if currentRevision != nil && *currentRevision == revision && profilesEqual(currentProfiles, normalized) {
		return nil, nil, ErrNoProfileChanges
	}
	if err := s.writeCanonicalLocked(canonical); err != nil {
		return nil, nil, err
	}
	return normalized, &revision, nil
}

func (s *Store) loadLocked() ([]Profile, *string, error) {
	info, err := s.lstat(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return []Profile{}, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("inspect %s: %w: %w", s.path, err, ErrProfileStore)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, nil, fmt.Errorf("%s is a symbolic link: %w", s.path, ErrProfileStore)
	}
	if !info.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("%s is not a regular file: %w", s.path, ErrProfileStore)
	}
	contents, err := s.readFile(s.path)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w: %w", s.path, err, ErrProfileStore)
	}
	if len(contents) == 0 {
		return nil, nil, fmt.Errorf("%s is empty: %w", s.path, ErrProfileStore)
	}
	document, err := decodeCanonical(contents)
	if err != nil {
		return nil, nil, fmt.Errorf("load %s: %w: %w", s.path, err, ErrProfileStore)
	}
	normalized, canonical, revision, err := ValidateProfiles(document.Profiles)
	if err != nil {
		return nil, nil, fmt.Errorf("load %s: %w: %w", s.path, err, ErrProfileStore)
	}
	if !bytes.Equal(contents, canonical) {
		return nil, nil, fmt.Errorf("%s canonical bytes are malformed or non-normalized: %w", s.path, ErrProfileStore)
	}
	return normalized, &revision, nil
}

func (s *Store) writeCanonicalLocked(contents []byte) error {
	dir := filepath.Dir(s.path)
	if err := s.mkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create provider config dir: %w: %w", err, ErrProfileStore)
	}
	tmpFile, err := s.openTempFile(filepath.Join(dir, ".providers.yaml.tmp"), 0o600)
	if err != nil {
		return fmt.Errorf("create temp provider config: %w: %w", err, ErrProfileStore)
	}
	tmpPath := tmpFile.Name()
	cleanup := true
	closed := false
	defer func() {
		if !closed {
			_ = tmpFile.Close()
		}
		if cleanup {
			_ = s.remove(tmpPath)
		}
	}()
	if _, err := tmpFile.Write(contents); err != nil {
		return fmt.Errorf("write temp provider config: %w: %w", err, ErrProfileStore)
	}
	if err := tmpFile.Chmod(0o600); err != nil {
		return fmt.Errorf("chmod temp provider config: %w: %w", err, ErrProfileStore)
	}
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("sync temp provider config: %w: %w", err, ErrProfileStore)
	}
	if err := tmpFile.Close(); err != nil {
		closed = true
		return fmt.Errorf("close temp provider config: %w: %w", err, ErrProfileStore)
	}
	closed = true
	if err := s.rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("rename temp provider config: %w: %w", err, ErrProfileStore)
	}
	if err := s.syncDir(dir); err != nil {
		return fmt.Errorf("sync provider config dir: %w: %w", err, ErrProfileStore)
	}
	cleanup = false
	return nil
}

type yamlDocument struct {
	Version  int       `yaml:"version"`
	Profiles []Profile `yaml:"profiles"`
}

func decodeCanonical(contents []byte) (yamlDocument, error) {
	var document yamlDocument
	decoder := yaml.NewDecoder(bytes.NewReader(contents))
	decoder.KnownFields(true)
	if err := decoder.Decode(&document); err != nil {
		return yamlDocument{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return yamlDocument{}, errors.New("multiple YAML documents are not supported")
		}
		return yamlDocument{}, err
	}
	if document.Version != Version {
		return yamlDocument{}, fmt.Errorf("version %d is unsupported", document.Version)
	}
	if document.Profiles == nil {
		return yamlDocument{}, errors.New("profiles is required")
	}
	return document, nil
}

type tempFile interface {
	Write([]byte) (int, error)
	Chmod(os.FileMode) error
	Sync() error
	Close() error
	Name() string
}

func defaultOpenTempFile(path string, mode os.FileMode) (tempFile, error) {
	return os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode)
}

func defaultSyncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func profilesEqual(left, right []Profile) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i].ID != right[i].ID ||
			left[i].Name != right[i].Name ||
			left[i].Type != right[i].Type ||
			left[i].BaseURL != right[i].BaseURL ||
			left[i].Auth != right[i].Auth ||
			left[i].Capabilities != right[i].Capabilities {
			return false
		}
	}
	return true
}
