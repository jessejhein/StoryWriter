package importer

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var (
	ErrInvalidSourceDirectory = fmt.Errorf("invalid source directory")
	ErrNoEligibleFiles        = fmt.Errorf("no eligible markdown files")
	ErrSymlinkRefused         = fmt.Errorf("symlink imports are not supported")
	ErrSourceChanged          = fmt.Errorf("source file changed while reading")
	ErrInvalidContent         = fmt.Errorf("invalid markdown source content")
)

type osFS struct{}

func (osFS) Lstat(path string) (fs.FileInfo, error)       { return os.Lstat(path) }
func (osFS) ReadDir(path string) ([]fs.DirEntry, error)   { return os.ReadDir(path) }
func (osFS) ReadFile(path string) ([]byte, error)         { return os.ReadFile(path) }
func (osFS) MkdirAll(path string, perm fs.FileMode) error { return os.MkdirAll(path, perm) }
func (osFS) WriteFile(path string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(path, data, perm)
}
func (osFS) RemoveAll(path string) error          { return os.RemoveAll(path) }
func (osFS) Rename(oldPath, newPath string) error { return os.Rename(oldPath, newPath) }
func (osFS) OpenFile(name string, flag int, perm fs.FileMode) (*os.File, error) {
	return os.OpenFile(name, flag, perm)
}

type sourceFileSystem interface {
	Lstat(path string) (fs.FileInfo, error)
	ReadDir(path string) ([]fs.DirEntry, error)
	ReadFile(path string) ([]byte, error)
	MkdirAll(path string, perm fs.FileMode) error
	WriteFile(path string, data []byte, perm fs.FileMode) error
	RemoveAll(path string) error
	Rename(oldPath, newPath string) error
	OpenFile(name string, flag int, perm fs.FileMode) (*os.File, error)
}

type PrepareSnapshotRequest struct {
	ProjectPath     string
	SourceDirectory string
	ImportID        string
	CreatedAt       time.Time
}

type PreparedSnapshot struct {
	manifest  ImportManifest
	files     []ImportFile
	stagePath string
	finalPath string
	fs        sourceFileSystem
}

type SourceStore struct {
	fs sourceFileSystem
}

func NewSourceStore() *SourceStore {
	return &SourceStore{fs: osFS{}}
}

func newSourceStoreForTests(files sourceFileSystem) *SourceStore {
	return &SourceStore{fs: files}
}

func (s *SourceStore) PrepareSnapshot(ctx context.Context, request PrepareSnapshotRequest) (*PreparedSnapshot, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if err := ValidateImportID(request.ImportID); err != nil {
		return nil, err
	}
	if err := validateSourceDirectory(request.ProjectPath, request.SourceDirectory); err != nil {
		return nil, err
	}
	eligibleFiles, err := s.discoverEligibleFiles(request.SourceDirectory)
	if err != nil {
		return nil, err
	}
	if len(eligibleFiles) == 0 {
		return nil, ErrNoEligibleFiles
	}

	stagePath := filepath.Join(request.ProjectPath, "imports", "raw", "."+request.ImportID+".tmp")
	finalPath := filepath.Join(request.ProjectPath, "imports", "raw", request.ImportID)
	if err := s.fs.RemoveAll(stagePath); err != nil {
		return nil, fmt.Errorf("remove stale import stage %q: %w", stagePath, err)
	}
	if err := s.fs.MkdirAll(filepath.Join(stagePath, "files"), 0o755); err != nil {
		return nil, fmt.Errorf("create import stage %q: %w", stagePath, err)
	}

	manifest := ImportManifest{
		Version:   ManifestVersion,
		ID:        request.ImportID,
		CreatedAt: request.CreatedAt.UTC(),
		Files:     make([]ImportFile, 0, len(eligibleFiles)),
	}

	success := false
	defer func() {
		if !success {
			_ = s.fs.RemoveAll(stagePath)
		}
	}()

	for _, sourcePath := range eligibleFiles {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		relativePath, err := filepath.Rel(request.SourceDirectory, sourcePath)
		if err != nil {
			return nil, fmt.Errorf("compute source relative path for %q: %w", sourcePath, err)
		}
		normalizedPath, err := NormalizePortableRelativePath(filepath.ToSlash(relativePath))
		if err != nil {
			return nil, err
		}
		fileRecord, contents, err := s.readEligibleFile(sourcePath, normalizedPath)
		if err != nil {
			return nil, err
		}
		manifest.Files = append(manifest.Files, fileRecord)
		destinationPath := filepath.Join(stagePath, "files", filepath.FromSlash(normalizedPath))
		if err := s.fs.MkdirAll(filepath.Dir(destinationPath), 0o755); err != nil {
			return nil, fmt.Errorf("create staged import directory %q: %w", destinationPath, err)
		}
		if err := s.fs.WriteFile(destinationPath, contents, 0o644); err != nil {
			return nil, fmt.Errorf("write staged import file %q: %w", normalizedPath, err)
		}
	}

	if err := manifest.Validate(); err != nil {
		return nil, err
	}
	manifestBytes, err := marshalManifest(manifest)
	if err != nil {
		return nil, err
	}
	if err := s.fs.WriteFile(filepath.Join(stagePath, "manifest.yaml"), manifestBytes, 0o644); err != nil {
		return nil, fmt.Errorf("write staged import manifest: %w", err)
	}
	if err := syncDirectory(stagePath); err != nil {
		return nil, err
	}
	success = true
	return &PreparedSnapshot{
		manifest:  manifest,
		files:     slices.Clone(manifest.Files),
		stagePath: stagePath,
		finalPath: finalPath,
		fs:        s.fs,
	}, nil
}

func (snapshot *PreparedSnapshot) Manifest() ImportManifest {
	return snapshot.manifest
}

func (snapshot *PreparedSnapshot) Files() []ImportFile {
	return slices.Clone(snapshot.files)
}

func (snapshot *PreparedSnapshot) Publish() (func() error, error) {
	if _, err := snapshot.fs.Lstat(snapshot.finalPath); err == nil {
		return nil, fmt.Errorf("publish import snapshot: destination already exists")
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("inspect import destination: %w", err)
	}
	if err := snapshot.fs.Rename(snapshot.stagePath, snapshot.finalPath); err != nil {
		return nil, fmt.Errorf("publish import snapshot: %w", err)
	}
	return func() error {
		if err := snapshot.fs.RemoveAll(snapshot.finalPath); err != nil {
			return fmt.Errorf("rollback import snapshot: %w", err)
		}
		return nil
	}, nil
}

func (s *SourceStore) discoverEligibleFiles(sourceDirectory string) ([]string, error) {
	var discovered []string
	var walk func(string) error
	walk = func(currentPath string) error {
		info, err := s.fs.Lstat(currentPath)
		if err != nil {
			return fmt.Errorf("stat source path %q: %w", currentPath, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("source path %q is a symlink: %w", currentPath, ErrSymlinkRefused)
		}
		if info.IsDir() {
			entries, err := s.fs.ReadDir(currentPath)
			if err != nil {
				return fmt.Errorf("read source directory %q: %w", currentPath, err)
			}
			slices.SortFunc(entries, func(left, right fs.DirEntry) int {
				return strings.Compare(left.Name(), right.Name())
			})
			for _, entry := range entries {
				name := entry.Name()
				if strings.HasPrefix(name, ".") {
					continue
				}
				if err := walk(filepath.Join(currentPath, name)); err != nil {
					return err
				}
			}
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		lower := strings.ToLower(info.Name())
		if !strings.HasSuffix(lower, ".md") && !strings.HasSuffix(lower, ".markdown") {
			return nil
		}
		discovered = append(discovered, currentPath)
		return nil
	}
	if err := walk(sourceDirectory); err != nil {
		return nil, err
	}
	slices.SortFunc(discovered, func(left, right string) int {
		return strings.Compare(filepath.ToSlash(left), filepath.ToSlash(right))
	})
	return discovered, nil
}

func (s *SourceStore) readEligibleFile(sourcePath, normalizedPath string) (ImportFile, []byte, error) {
	beforeInfo, err := s.fs.Lstat(sourcePath)
	if err != nil {
		return ImportFile{}, nil, fmt.Errorf("stat source file %q: %w", sourcePath, err)
	}
	if beforeInfo.Mode()&os.ModeSymlink != 0 {
		return ImportFile{}, nil, fmt.Errorf("source file %q is a symlink: %w", sourcePath, ErrSymlinkRefused)
	}
	if !beforeInfo.Mode().IsRegular() {
		return ImportFile{}, nil, fmt.Errorf("source file %q is not regular: %w", sourcePath, ErrInvalidSourceDirectory)
	}
	raw, err := s.fs.ReadFile(sourcePath)
	if err != nil {
		return ImportFile{}, nil, fmt.Errorf("read source file %q: %w", sourcePath, err)
	}
	normalized, err := NormalizeMarkdownText(string(raw))
	if err != nil {
		return ImportFile{}, nil, fmt.Errorf("normalize source file %q: %w", sourcePath, err)
	}
	normalizedBytes := []byte(normalized)
	if len(normalizedBytes) > maxImportFileBytes {
		return ImportFile{}, nil, fmt.Errorf("source file %q exceeds %d bytes after normalization: %w", sourcePath, maxImportFileBytes, ErrInvalidContent)
	}
	afterInfo, err := s.fs.Lstat(sourcePath)
	if err != nil {
		return ImportFile{}, nil, fmt.Errorf("restat source file %q: %w", sourcePath, err)
	}
	if beforeInfo.Size() != afterInfo.Size() || beforeInfo.ModTime() != afterInfo.ModTime() || beforeInfo.Mode() != afterInfo.Mode() {
		return ImportFile{}, nil, fmt.Errorf("source file %q changed while reading: %w", sourcePath, ErrSourceChanged)
	}
	return ImportFile{
		Path:   normalizedPath,
		Bytes:  int64(len(normalizedBytes)),
		SHA256: CanonicalSHA256(normalizedBytes),
	}, normalizedBytes, nil
}

func validateSourceDirectory(projectPath, sourceDirectory string) error {
	projectPath = filepath.Clean(strings.TrimSpace(projectPath))
	sourceDirectory = filepath.Clean(strings.TrimSpace(sourceDirectory))
	if projectPath == "" || sourceDirectory == "" || !filepath.IsAbs(sourceDirectory) {
		return fmt.Errorf("source directory must be an absolute path outside the project: %w", ErrInvalidSourceDirectory)
	}
	sourceInfo, err := os.Stat(sourceDirectory)
	if err != nil {
		return fmt.Errorf("stat source directory %q: %w", sourceDirectory, ErrInvalidSourceDirectory)
	}
	if !sourceInfo.IsDir() {
		return fmt.Errorf("source path %q is not a directory: %w", sourceDirectory, ErrInvalidSourceDirectory)
	}
	if insidePath(sourceDirectory, projectPath) || insidePath(projectPath, sourceDirectory) {
		return fmt.Errorf("source directory %q overlaps project %q: %w", sourceDirectory, projectPath, ErrInvalidSourceDirectory)
	}
	return nil
}

func insidePath(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

func marshalManifest(manifest ImportManifest) ([]byte, error) {
	type encodedManifest struct {
		Version   int          `yaml:"version"`
		ID        string       `yaml:"id"`
		CreatedAt string       `yaml:"created_at"`
		Files     []ImportFile `yaml:"files"`
	}
	encoded, err := yaml.Marshal(encodedManifest{
		Version:   manifest.Version,
		ID:        manifest.ID,
		CreatedAt: manifest.CreatedAt.UTC().Format(time.RFC3339),
		Files:     manifest.Files,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal import manifest: %w", err)
	}
	return encoded, nil
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open directory %q for sync: %w", path, err)
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("sync directory %q: %w", path, err)
	}
	return nil
}
