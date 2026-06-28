// Package storyfile loads and writes canonical Milestone 1 story files.
package storyfile

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"storywork/internal/story"
)

type outlineDocument struct {
	Version int          `yaml:"version"`
	Root    *outlineRoot `yaml:"root"`
}

type outlineRoot struct {
	Arcs *[]outlineArcRef `yaml:"arcs"`
}

type outlineArcRef struct {
	ID       string               `yaml:"id"`
	Chapters *[]outlineChapterRef `yaml:"chapters"`
}

type outlineChapterRef struct {
	ID     string             `yaml:"id"`
	Scenes *[]outlineSceneRef `yaml:"scenes"`
}

type outlineSceneRef struct {
	ID string `yaml:"id"`
}

type arcDocument struct {
	Version int    `yaml:"version"`
	ID      string `yaml:"id"`
	Title   string `yaml:"title"`
}

type chapterDocument struct {
	Version int    `yaml:"version"`
	ID      string `yaml:"id"`
	ArcID   string `yaml:"arc_id"`
	Title   string `yaml:"title"`
}

type sceneFrontMatter struct {
	ID            string `yaml:"id"`
	Title         string `yaml:"title"`
	ChapterID     string `yaml:"chapter_id"`
	POV           string `yaml:"pov"`
	Status        string `yaml:"status"`
	ExcludeFromAI bool   `yaml:"exclude_from_ai"`
}

type snapshotEntry struct {
	relativePath string
	existed      bool
	contents     []byte
}

// Store loads and writes canonical story files.
type Store struct {
	readFile  func(string) ([]byte, error)
	writeFile func(string, []byte, os.FileMode) error
	mkdirAll  func(string, os.FileMode) error
	rename    func(string, string) error
	remove    func(string) error
	stat      func(string) (os.FileInfo, error)
}

// New creates a story file store.
func New() *Store {
	return &Store{
		readFile:  os.ReadFile,
		writeFile: os.WriteFile,
		mkdirAll:  os.MkdirAll,
		rename:    os.Rename,
		remove:    os.Remove,
		stat:      os.Stat,
	}
}

// Exists reports whether a canonical relative path already exists.
func (s *Store) Exists(_ context.Context, projectPath, relativePath string) (bool, error) {
	_, err := s.stat(filepath.Join(projectPath, filepath.FromSlash(relativePath)))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

// Load reads the canonical outline and referenced entity files.
func (s *Store) Load(_ context.Context, projectPath string) (story.Outline, error) {
	outlineBytes, err := s.readFile(filepath.Join(projectPath, "outline.yaml"))
	if err != nil {
		return story.Outline{}, fmt.Errorf("read outline.yaml: %w", err)
	}

	var document outlineDocument
	if err := decodeYAML("outline.yaml", outlineBytes, &document); err != nil {
		return story.Outline{}, err
	}
	if document.Version != story.OutlineVersion {
		return story.Outline{}, fmt.Errorf("outline.yaml has unsupported version %d", document.Version)
	}
	if document.Root == nil {
		return story.Outline{}, errors.New("outline.yaml is missing root")
	}
	if document.Root.Arcs == nil {
		return story.Outline{}, errors.New("outline.yaml root is missing arcs")
	}

	outline := story.NewOutline()
	seenArcs := make(map[string]struct{})
	seenChapters := make(map[string]struct{})
	seenScenes := make(map[string]struct{})

	for _, arcRef := range *document.Root.Arcs {
		if arcRef.Chapters == nil {
			return story.Outline{}, fmt.Errorf("outline.yaml arc %q is missing chapters", arcRef.ID)
		}
		if err := story.ValidateArcID(arcRef.ID); err != nil {
			return story.Outline{}, fmt.Errorf("outline.yaml arc ID %q: %w", arcRef.ID, err)
		}
		if _, exists := seenArcs[arcRef.ID]; exists {
			return story.Outline{}, fmt.Errorf("duplicate arc ID %q in outline.yaml", arcRef.ID)
		}
		seenArcs[arcRef.ID] = struct{}{}

		arcPath := filepath.Join(projectPath, "arcs", arcRef.ID+".yaml")
		arcBytes, err := s.readFile(arcPath)
		if err != nil {
			return story.Outline{}, fmt.Errorf("read %s: %w", filepath.ToSlash(filepath.Join("arcs", arcRef.ID+".yaml")), err)
		}
		var arcFile arcDocument
		if err := decodeYAML(filepath.ToSlash(filepath.Join("arcs", arcRef.ID+".yaml")), arcBytes, &arcFile); err != nil {
			return story.Outline{}, err
		}
		if arcFile.Version != story.OutlineVersion {
			return story.Outline{}, fmt.Errorf("arcs/%s.yaml has unsupported version %d", arcRef.ID, arcFile.Version)
		}
		if arcFile.ID != arcRef.ID {
			return story.Outline{}, fmt.Errorf("arcs/%s.yaml id %q does not match outline reference %q", arcRef.ID, arcFile.ID, arcRef.ID)
		}
		if _, err := story.ValidateTitle(arcFile.Title); err != nil {
			return story.Outline{}, fmt.Errorf("arcs/%s.yaml title: %w", arcRef.ID, err)
		}
		outline, err = story.AddArc(outline, arcFile.ID, arcFile.Title)
		if err != nil {
			return story.Outline{}, fmt.Errorf("load arc %q: %w", arcFile.ID, err)
		}

		for _, chapterRef := range *arcRef.Chapters {
			if chapterRef.Scenes == nil {
				return story.Outline{}, fmt.Errorf("outline.yaml chapter %q is missing scenes", chapterRef.ID)
			}
			if err := story.ValidateChapterID(chapterRef.ID); err != nil {
				return story.Outline{}, fmt.Errorf("outline.yaml chapter ID %q: %w", chapterRef.ID, err)
			}
			if _, exists := seenChapters[chapterRef.ID]; exists {
				return story.Outline{}, fmt.Errorf("duplicate chapter ID %q in outline.yaml", chapterRef.ID)
			}
			seenChapters[chapterRef.ID] = struct{}{}

			chapterPath := filepath.Join(projectPath, "chapters", chapterRef.ID+".yaml")
			chapterBytes, err := s.readFile(chapterPath)
			if err != nil {
				return story.Outline{}, fmt.Errorf("read %s: %w", filepath.ToSlash(filepath.Join("chapters", chapterRef.ID+".yaml")), err)
			}
			var chapterFile chapterDocument
			if err := decodeYAML(filepath.ToSlash(filepath.Join("chapters", chapterRef.ID+".yaml")), chapterBytes, &chapterFile); err != nil {
				return story.Outline{}, err
			}
			if chapterFile.Version != story.OutlineVersion {
				return story.Outline{}, fmt.Errorf("chapters/%s.yaml has unsupported version %d", chapterRef.ID, chapterFile.Version)
			}
			if chapterFile.ID != chapterRef.ID {
				return story.Outline{}, fmt.Errorf("chapters/%s.yaml id %q does not match outline reference %q", chapterRef.ID, chapterFile.ID, chapterRef.ID)
			}
			if chapterFile.ArcID != arcRef.ID {
				return story.Outline{}, fmt.Errorf("chapters/%s.yaml arc_id %q does not match containing arc %q", chapterRef.ID, chapterFile.ArcID, arcRef.ID)
			}
			if _, err := story.ValidateTitle(chapterFile.Title); err != nil {
				return story.Outline{}, fmt.Errorf("chapters/%s.yaml title: %w", chapterRef.ID, err)
			}
			outline, err = story.AddChapter(outline, chapterFile.ArcID, chapterFile.ID, chapterFile.Title)
			if err != nil {
				return story.Outline{}, fmt.Errorf("load chapter %q: %w", chapterFile.ID, err)
			}

			for _, sceneRef := range *chapterRef.Scenes {
				if err := story.ValidateSceneID(sceneRef.ID); err != nil {
					return story.Outline{}, fmt.Errorf("outline.yaml scene ID %q: %w", sceneRef.ID, err)
				}
				if _, exists := seenScenes[sceneRef.ID]; exists {
					return story.Outline{}, fmt.Errorf("duplicate scene ID %q in outline.yaml", sceneRef.ID)
				}
				seenScenes[sceneRef.ID] = struct{}{}

				scenePath := filepath.Join(projectPath, "scenes", sceneRef.ID+".md")
				sceneBytes, err := s.readFile(scenePath)
				if err != nil {
					return story.Outline{}, fmt.Errorf("read %s: %w", filepath.ToSlash(filepath.Join("scenes", sceneRef.ID+".md")), err)
				}
				sceneFile, err := parseScene(filepath.ToSlash(filepath.Join("scenes", sceneRef.ID+".md")), sceneBytes)
				if err != nil {
					return story.Outline{}, err
				}
				if sceneFile.ID != sceneRef.ID {
					return story.Outline{}, fmt.Errorf("scenes/%s.md id %q does not match outline reference %q", sceneRef.ID, sceneFile.ID, sceneRef.ID)
				}
				if sceneFile.ChapterID != chapterRef.ID {
					return story.Outline{}, fmt.Errorf("scenes/%s.md chapter_id %q does not match containing chapter %q", sceneRef.ID, sceneFile.ChapterID, chapterRef.ID)
				}
				if _, err := story.ValidateTitle(sceneFile.Title); err != nil {
					return story.Outline{}, fmt.Errorf("scenes/%s.md title: %w", sceneRef.ID, err)
				}
				outline, err = story.AddScene(outline, sceneFile.ChapterID, sceneFile.ID, sceneFile.Title)
				if err != nil {
					return story.Outline{}, fmt.Errorf("load scene %q: %w", sceneFile.ID, err)
				}
			}
		}
	}

	return outline, nil
}

// MarshalOutline encodes outline ordering only.
func (s *Store) MarshalOutline(outline story.Outline) ([]byte, error) {
	arcs := make([]outlineArcRef, len(outline.Arcs))
	document := outlineDocument{
		Version: story.OutlineVersion,
		Root: &outlineRoot{
			Arcs: &arcs,
		},
	}
	for i, arc := range outline.Arcs {
		chapters := make([]outlineChapterRef, len(arc.Chapters))
		(*document.Root.Arcs)[i] = outlineArcRef{
			ID:       arc.ID,
			Chapters: &chapters,
		}
		for j, chapter := range arc.Chapters {
			scenes := make([]outlineSceneRef, len(chapter.Scenes))
			(*(*document.Root.Arcs)[i].Chapters)[j] = outlineChapterRef{
				ID:     chapter.ID,
				Scenes: &scenes,
			}
			for k, scene := range chapter.Scenes {
				(*(*(*document.Root.Arcs)[i].Chapters)[j].Scenes)[k] = outlineSceneRef{ID: scene.ID}
			}
		}
	}
	return marshalYAML(document)
}

// MarshalArc encodes an arc file.
func (s *Store) MarshalArc(arc story.Arc) ([]byte, error) {
	title, err := story.ValidateTitle(arc.Title)
	if err != nil {
		return nil, err
	}
	if err := story.ValidateArcID(arc.ID); err != nil {
		return nil, err
	}
	return marshalYAML(arcDocument{
		Version: story.OutlineVersion,
		ID:      arc.ID,
		Title:   title,
	})
}

// MarshalChapter encodes a chapter file.
func (s *Store) MarshalChapter(chapter story.Chapter) ([]byte, error) {
	title, err := story.ValidateTitle(chapter.Title)
	if err != nil {
		return nil, err
	}
	if err := story.ValidateChapterID(chapter.ID); err != nil {
		return nil, err
	}
	if err := story.ValidateArcID(chapter.ArcID); err != nil {
		return nil, err
	}
	return marshalYAML(chapterDocument{
		Version: story.OutlineVersion,
		ID:      chapter.ID,
		ArcID:   chapter.ArcID,
		Title:   title,
	})
}

// MarshalScene encodes a scene front matter block plus empty body.
func (s *Store) MarshalScene(scene story.Scene) ([]byte, error) {
	title, err := story.ValidateTitle(scene.Title)
	if err != nil {
		return nil, err
	}
	if err := story.ValidateSceneID(scene.ID); err != nil {
		return nil, err
	}
	if err := story.ValidateChapterID(scene.ChapterID); err != nil {
		return nil, err
	}

	frontMatter, err := marshalYAML(sceneFrontMatter{
		ID:            scene.ID,
		Title:         title,
		ChapterID:     scene.ChapterID,
		POV:           "",
		Status:        "draft",
		ExcludeFromAI: false,
	})
	if err != nil {
		return nil, err
	}
	return []byte("---\n" + string(frontMatter) + "---\n\n"), nil
}

// WriteFiles atomically replaces the requested relative paths and returns a
// rollback function for later recovery.
func (s *Store) WriteFiles(_ context.Context, projectPath string, files map[string][]byte) (func() error, error) {
	if len(files) == 0 {
		return func() error { return nil }, nil
	}

	paths := make([]string, 0, len(files))
	snapshots := make([]snapshotEntry, 0, len(files))
	for relativePath := range files {
		paths = append(paths, relativePath)
		absolutePath := filepath.Join(projectPath, filepath.FromSlash(relativePath))
		contents, err := s.readFile(absolutePath)
		if err == nil {
			snapshots = append(snapshots, snapshotEntry{relativePath: relativePath, existed: true, contents: contents})
			continue
		}
		if errors.Is(err, os.ErrNotExist) {
			snapshots = append(snapshots, snapshotEntry{relativePath: relativePath})
			continue
		}
		return nil, fmt.Errorf("snapshot %s: %w", relativePath, err)
	}
	sort.Strings(paths)

	rollback := func() error {
		var rollbackErrors []error
		for _, snapshot := range snapshots {
			absolutePath := filepath.Join(projectPath, filepath.FromSlash(snapshot.relativePath))
			if snapshot.existed {
				if err := s.writeFile(absolutePath, snapshot.contents, 0o644); err != nil {
					rollbackErrors = append(rollbackErrors, fmt.Errorf("restore %s: %w", snapshot.relativePath, err))
				}
				continue
			}
			if err := s.remove(absolutePath); err != nil && !errors.Is(err, os.ErrNotExist) {
				rollbackErrors = append(rollbackErrors, fmt.Errorf("remove %s: %w", snapshot.relativePath, err))
			}
		}
		return errors.Join(rollbackErrors...)
	}

	for _, relativePath := range paths {
		absolutePath := filepath.Join(projectPath, filepath.FromSlash(relativePath))
		if err := s.mkdirAll(filepath.Dir(absolutePath), 0o755); err != nil {
			rollbackError := rollback()
			return nil, errors.Join(fmt.Errorf("create parent directory for %s: %w", relativePath, err), rollbackError)
		}
		tempFile, err := os.CreateTemp(filepath.Dir(absolutePath), ".storywork-write-*")
		if err != nil {
			rollbackError := rollback()
			return nil, errors.Join(fmt.Errorf("create temp file for %s: %w", relativePath, err), rollbackError)
		}
		tempPath := tempFile.Name()
		if _, err := tempFile.Write(files[relativePath]); err != nil {
			_ = tempFile.Close()
			_ = os.Remove(tempPath)
			rollbackError := rollback()
			return nil, errors.Join(fmt.Errorf("write temp file for %s: %w", relativePath, err), rollbackError)
		}
		if err := tempFile.Close(); err != nil {
			_ = os.Remove(tempPath)
			rollbackError := rollback()
			return nil, errors.Join(fmt.Errorf("close temp file for %s: %w", relativePath, err), rollbackError)
		}
		if err := s.rename(tempPath, absolutePath); err != nil {
			_ = os.Remove(tempPath)
			rollbackError := rollback()
			return nil, errors.Join(fmt.Errorf("replace %s: %w", relativePath, err), rollbackError)
		}
	}

	return rollback, nil
}

func decodeYAML(path string, contents []byte, target any) error {
	decoder := yaml.NewDecoder(bytes.NewReader(contents))
	decoder.KnownFields(true)
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		if err == nil {
			return fmt.Errorf("decode %s: unexpected extra YAML document", path)
		}
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func marshalYAML(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(2)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func parseScene(path string, contents []byte) (sceneFrontMatter, error) {
	text := string(contents)
	if !strings.HasPrefix(text, "---\n") {
		return sceneFrontMatter{}, fmt.Errorf("decode %s: missing YAML front matter", path)
	}
	rest := strings.TrimPrefix(text, "---\n")
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return sceneFrontMatter{}, fmt.Errorf("decode %s: missing front matter terminator", path)
	}

	frontMatterText := rest[:end+1]
	var frontMatter sceneFrontMatter
	if err := decodeYAML(path, []byte(frontMatterText), &frontMatter); err != nil {
		return sceneFrontMatter{}, err
	}
	return frontMatter, nil
}
