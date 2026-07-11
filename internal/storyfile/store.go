package storyfile

// store.go implements outline and scene file loading plus atomic canonical writes.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

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

// Store loads, validates, marshals, and atomically writes canonical story files.
type Store struct {
	readFile  func(string) ([]byte, error)
	writeFile func(string, []byte, os.FileMode) error
	mkdirAll  func(string, os.FileMode) error
	rename    func(string, string) error
	remove    func(string) error
	stat      func(string) (os.FileInfo, error)
}

// New creates a file-backed canonical store that reads and writes the local filesystem.
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
		return story.Outline{}, invalidCanonical(err)
	}
	if document.Version != story.OutlineVersion {
		return story.Outline{}, invalidCanonical(fmt.Errorf("outline.yaml has unsupported version %d", document.Version))
	}
	if document.Root == nil {
		return story.Outline{}, invalidCanonical(errors.New("outline.yaml is missing root"))
	}
	if document.Root.Arcs == nil {
		return story.Outline{}, invalidCanonical(errors.New("outline.yaml root is missing arcs"))
	}

	outline := story.NewOutline()
	seenArcs := make(map[string]struct{})
	seenChapters := make(map[string]struct{})
	seenScenes := make(map[string]struct{})

	for _, arcRef := range *document.Root.Arcs {
		if arcRef.Chapters == nil {
			return story.Outline{}, invalidCanonical(fmt.Errorf("outline.yaml arc %q is missing chapters", arcRef.ID))
		}
		if err := story.ValidateArcID(arcRef.ID); err != nil {
			return story.Outline{}, invalidCanonical(fmt.Errorf("outline.yaml arc ID %q: %w", arcRef.ID, err))
		}
		if _, exists := seenArcs[arcRef.ID]; exists {
			return story.Outline{}, invalidCanonical(fmt.Errorf("duplicate arc ID %q in outline.yaml", arcRef.ID))
		}
		seenArcs[arcRef.ID] = struct{}{}

		arcPath := filepath.Join(projectPath, "arcs", arcRef.ID+".yaml")
		arcBytes, err := s.readFile(arcPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return story.Outline{}, invalidCanonical(fmt.Errorf("missing %s", filepath.ToSlash(filepath.Join("arcs", arcRef.ID+".yaml"))))
			}
			return story.Outline{}, fmt.Errorf("read %s: %w", filepath.ToSlash(filepath.Join("arcs", arcRef.ID+".yaml")), err)
		}
		var arcFile arcDocument
		if err := decodeYAML(filepath.ToSlash(filepath.Join("arcs", arcRef.ID+".yaml")), arcBytes, &arcFile); err != nil {
			return story.Outline{}, invalidCanonical(err)
		}
		if arcFile.Version != story.OutlineVersion {
			return story.Outline{}, invalidCanonical(fmt.Errorf("arcs/%s.yaml has unsupported version %d", arcRef.ID, arcFile.Version))
		}
		if arcFile.ID != arcRef.ID {
			return story.Outline{}, invalidCanonical(fmt.Errorf("arcs/%s.yaml id %q does not match outline reference %q", arcRef.ID, arcFile.ID, arcRef.ID))
		}
		if _, err := story.ValidateTitle(arcFile.Title); err != nil {
			return story.Outline{}, invalidCanonical(fmt.Errorf("arcs/%s.yaml title: %w", arcRef.ID, err))
		}
		outline, err = story.AddArc(outline, arcFile.ID, arcFile.Title)
		if err != nil {
			return story.Outline{}, invalidCanonical(fmt.Errorf("load arc %q: %w", arcFile.ID, err))
		}

		for _, chapterRef := range *arcRef.Chapters {
			if chapterRef.Scenes == nil {
				return story.Outline{}, invalidCanonical(fmt.Errorf("outline.yaml chapter %q is missing scenes", chapterRef.ID))
			}
			if err := story.ValidateChapterID(chapterRef.ID); err != nil {
				return story.Outline{}, invalidCanonical(fmt.Errorf("outline.yaml chapter ID %q: %w", chapterRef.ID, err))
			}
			if _, exists := seenChapters[chapterRef.ID]; exists {
				return story.Outline{}, invalidCanonical(fmt.Errorf("duplicate chapter ID %q in outline.yaml", chapterRef.ID))
			}
			seenChapters[chapterRef.ID] = struct{}{}

			chapterPath := filepath.Join(projectPath, "chapters", chapterRef.ID+".yaml")
			chapterBytes, err := s.readFile(chapterPath)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return story.Outline{}, invalidCanonical(fmt.Errorf("missing %s", filepath.ToSlash(filepath.Join("chapters", chapterRef.ID+".yaml"))))
				}
				return story.Outline{}, fmt.Errorf("read %s: %w", filepath.ToSlash(filepath.Join("chapters", chapterRef.ID+".yaml")), err)
			}
			var chapterFile chapterDocument
			if err := decodeYAML(filepath.ToSlash(filepath.Join("chapters", chapterRef.ID+".yaml")), chapterBytes, &chapterFile); err != nil {
				return story.Outline{}, invalidCanonical(err)
			}
			if chapterFile.Version != story.OutlineVersion {
				return story.Outline{}, invalidCanonical(fmt.Errorf("chapters/%s.yaml has unsupported version %d", chapterRef.ID, chapterFile.Version))
			}
			if chapterFile.ID != chapterRef.ID {
				return story.Outline{}, invalidCanonical(fmt.Errorf("chapters/%s.yaml id %q does not match outline reference %q", chapterRef.ID, chapterFile.ID, chapterRef.ID))
			}
			if chapterFile.ArcID != arcRef.ID {
				return story.Outline{}, invalidCanonical(fmt.Errorf("chapters/%s.yaml arc_id %q does not match containing arc %q", chapterRef.ID, chapterFile.ArcID, arcRef.ID))
			}
			if _, err := story.ValidateTitle(chapterFile.Title); err != nil {
				return story.Outline{}, invalidCanonical(fmt.Errorf("chapters/%s.yaml title: %w", chapterRef.ID, err))
			}
			outline, err = story.AddChapter(outline, chapterFile.ArcID, chapterFile.ID, chapterFile.Title)
			if err != nil {
				return story.Outline{}, invalidCanonical(fmt.Errorf("load chapter %q: %w", chapterFile.ID, err))
			}

			for _, sceneRef := range *chapterRef.Scenes {
				if err := story.ValidateSceneID(sceneRef.ID); err != nil {
					return story.Outline{}, invalidCanonical(fmt.Errorf("outline.yaml scene ID %q: %w", sceneRef.ID, err))
				}
				if _, exists := seenScenes[sceneRef.ID]; exists {
					return story.Outline{}, invalidCanonical(fmt.Errorf("duplicate scene ID %q in outline.yaml", sceneRef.ID))
				}
				seenScenes[sceneRef.ID] = struct{}{}

				scenePath := filepath.Join(projectPath, "scenes", sceneRef.ID+".md")
				sceneBytes, err := s.readFile(scenePath)
				if err != nil {
					if errors.Is(err, os.ErrNotExist) {
						return story.Outline{}, invalidCanonical(fmt.Errorf("missing %s", filepath.ToSlash(filepath.Join("scenes", sceneRef.ID+".md"))))
					}
					return story.Outline{}, fmt.Errorf("read %s: %w", filepath.ToSlash(filepath.Join("scenes", sceneRef.ID+".md")), err)
				}
				sceneFile, err := parseScene(filepath.ToSlash(filepath.Join("scenes", sceneRef.ID+".md")), sceneBytes)
				if err != nil {
					return story.Outline{}, invalidCanonical(err)
				}
				if sceneFile.ID != sceneRef.ID {
					return story.Outline{}, invalidCanonical(fmt.Errorf("scenes/%s.md id %q does not match outline reference %q", sceneRef.ID, sceneFile.ID, sceneRef.ID))
				}
				if sceneFile.ChapterID != chapterRef.ID {
					return story.Outline{}, invalidCanonical(fmt.Errorf("scenes/%s.md chapter_id %q does not match containing chapter %q", sceneRef.ID, sceneFile.ChapterID, chapterRef.ID))
				}
				if _, err := story.ValidateTitle(sceneFile.Title); err != nil {
					return story.Outline{}, invalidCanonical(fmt.Errorf("scenes/%s.md title: %w", sceneRef.ID, err))
				}
				outline, err = story.AddScene(outline, sceneFile.ChapterID, sceneFile.ID, sceneFile.Title)
				if err != nil {
					return story.Outline{}, invalidCanonical(fmt.Errorf("load scene %q: %w", sceneFile.ID, err))
				}
			}
		}
	}

	return outline, nil
}

// LoadOutlineBytes returns the exact canonical outline.yaml bytes for optimistic fingerprints.
func (s *Store) LoadOutlineBytes(_ context.Context, projectPath string) ([]byte, error) {
	outlineBytes, err := s.readFile(filepath.Join(projectPath, "outline.yaml"))
	if err != nil {
		return nil, fmt.Errorf("read outline.yaml: %w", err)
	}
	return outlineBytes, nil
}

// LoadScene reads one canonical scene document and computes its revision.
func (s *Store) LoadScene(_ context.Context, projectPath, sceneID string) (story.SceneDocument, error) {
	if err := story.ValidateSceneID(sceneID); err != nil {
		return story.SceneDocument{}, err
	}
	relativePath := filepath.ToSlash(filepath.Join("scenes", sceneID+".md"))
	contents, err := s.readFile(filepath.Join(projectPath, "scenes", sceneID+".md"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return story.SceneDocument{}, fmt.Errorf("scene %q: %w", sceneID, story.ErrSceneNotFound)
		}
		return story.SceneDocument{}, fmt.Errorf("read %s: %w", relativePath, err)
	}
	return parseCanonicalScene(relativePath, contents)
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

// MarshalSceneDocument encodes the full canonical scene file.
func (s *Store) MarshalSceneDocument(scene story.SceneDocument) ([]byte, error) {
	if err := story.ValidateSceneID(scene.ID); err != nil {
		return nil, err
	}
	if err := story.ValidateChapterID(scene.ChapterID); err != nil {
		return nil, err
	}
	title, err := story.ValidateTitle(scene.Title)
	if err != nil {
		return nil, err
	}
	pov, err := story.ValidatePOV(scene.FrontMatter.POV)
	if err != nil {
		return nil, err
	}
	status, err := story.ValidateSceneStatus(scene.FrontMatter.Status)
	if err != nil {
		return nil, err
	}
	markdown, err := story.NormalizeMarkdown(scene.Markdown)
	if err != nil {
		return nil, err
	}

	var buffer bytes.Buffer
	buffer.WriteString("---\n")
	buffer.WriteString("id: ")
	buffer.WriteString(scene.ID)
	buffer.WriteByte('\n')
	buffer.WriteString("title: ")
	buffer.WriteString(quoteYAMLScalar(title))
	buffer.WriteByte('\n')
	buffer.WriteString("chapter_id: ")
	buffer.WriteString(scene.ChapterID)
	buffer.WriteByte('\n')
	buffer.WriteString("pov: ")
	buffer.WriteString(quoteYAMLScalar(pov))
	buffer.WriteByte('\n')
	buffer.WriteString("status: ")
	buffer.WriteString(status)
	buffer.WriteByte('\n')
	buffer.WriteString("exclude_from_ai: ")
	if scene.FrontMatter.ExcludeFromAI {
		buffer.WriteString("true\n")
	} else {
		buffer.WriteString("false\n")
	}
	buffer.WriteString("---\n\n")
	buffer.WriteString(markdown)
	if !strings.HasSuffix(buffer.String(), "\n") {
		buffer.WriteByte('\n')
	}
	return buffer.Bytes(), nil
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
	document, err := parseCanonicalScene(path, contents)
	if err != nil {
		return sceneFrontMatter{}, err
	}
	return sceneFrontMatter{
		ID:            document.ID,
		Title:         document.Title,
		ChapterID:     document.ChapterID,
		POV:           document.FrontMatter.POV,
		Status:        document.FrontMatter.Status,
		ExcludeFromAI: document.FrontMatter.ExcludeFromAI,
	}, nil
}

func parseCanonicalScene(path string, contents []byte) (story.SceneDocument, error) {
	if !utf8.Valid(contents) {
		return story.SceneDocument{}, fmt.Errorf("decode %s: scene file is not valid UTF-8", path)
	}
	text := string(contents)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	if strings.ContainsRune(text, '\x00') {
		return story.SceneDocument{}, fmt.Errorf("decode %s: scene file contains NUL byte", path)
	}
	if !strings.HasPrefix(text, "---\n") {
		return story.SceneDocument{}, fmt.Errorf("decode %s: missing YAML front matter", path)
	}
	rest := strings.TrimPrefix(text, "---\n")
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return story.SceneDocument{}, fmt.Errorf("decode %s: missing front matter terminator", path)
	}

	frontMatter, err := decodeSceneFrontMatter(path, rest[:end+1])
	if err != nil {
		return story.SceneDocument{}, err
	}
	title, err := story.ValidateTitle(frontMatter.Title)
	if err != nil {
		return story.SceneDocument{}, fmt.Errorf("decode %s: %w", path, err)
	}
	if err := story.ValidateSceneID(frontMatter.ID); err != nil {
		return story.SceneDocument{}, fmt.Errorf("decode %s: %w", path, err)
	}
	if err := story.ValidateChapterID(frontMatter.ChapterID); err != nil {
		return story.SceneDocument{}, fmt.Errorf("decode %s: %w", path, err)
	}
	pov, err := story.ValidatePOV(frontMatter.POV)
	if err != nil {
		return story.SceneDocument{}, fmt.Errorf("decode %s: %w", path, err)
	}
	status, err := story.ValidateSceneStatus(frontMatter.Status)
	if err != nil {
		return story.SceneDocument{}, fmt.Errorf("decode %s: %w", path, err)
	}
	markdownText := rest[end+5:]
	if !strings.HasPrefix(markdownText, "\n") {
		return story.SceneDocument{}, fmt.Errorf("decode %s: missing blank line after front matter", path)
	}
	markdown, err := story.NormalizeMarkdown(strings.TrimPrefix(markdownText, "\n"))
	if err != nil {
		return story.SceneDocument{}, fmt.Errorf("decode %s: %w", path, err)
	}
	canonical := append([]byte(nil), contents...)
	return story.SceneDocument{
		ID:        frontMatter.ID,
		ChapterID: frontMatter.ChapterID,
		Title:     title,
		FrontMatter: story.SceneFrontMatter{
			POV:           pov,
			Status:        status,
			ExcludeFromAI: frontMatter.ExcludeFromAI,
		},
		Markdown:  markdown,
		Revision:  story.ComputeRevision(canonical),
		Canonical: canonical,
	}, nil
}

func decodeSceneFrontMatter(path, text string) (sceneFrontMatter, error) {
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(text), &node); err != nil {
		return sceneFrontMatter{}, fmt.Errorf("decode %s: %w", path, err)
	}
	if len(node.Content) != 1 || node.Content[0].Kind != yaml.MappingNode {
		return sceneFrontMatter{}, fmt.Errorf("decode %s: front matter must be a mapping", path)
	}
	mapping := node.Content[0]
	seen := map[string]struct{}{}
	values := map[string]*yaml.Node{}
	for i := 0; i < len(mapping.Content); i += 2 {
		key := mapping.Content[i].Value
		value := mapping.Content[i+1]
		if _, exists := seen[key]; exists {
			return sceneFrontMatter{}, fmt.Errorf("decode %s: duplicate front matter field %q", path, key)
		}
		seen[key] = struct{}{}
		values[key] = value
		switch key {
		case "id", "title", "chapter_id", "pov", "status", "exclude_from_ai":
		default:
			return sceneFrontMatter{}, fmt.Errorf("decode %s: field %s not found", path, key)
		}
	}
	for _, key := range []string{"id", "title", "chapter_id", "pov", "status", "exclude_from_ai"} {
		if _, ok := values[key]; !ok {
			return sceneFrontMatter{}, fmt.Errorf("decode %s: missing front matter field %q", path, key)
		}
	}
	var matter sceneFrontMatter
	if err := values["id"].Decode(&matter.ID); err != nil {
		return sceneFrontMatter{}, fmt.Errorf("decode %s id: %w", path, err)
	}
	if err := values["title"].Decode(&matter.Title); err != nil {
		return sceneFrontMatter{}, fmt.Errorf("decode %s title: %w", path, err)
	}
	if err := values["chapter_id"].Decode(&matter.ChapterID); err != nil {
		return sceneFrontMatter{}, fmt.Errorf("decode %s chapter_id: %w", path, err)
	}
	if err := values["pov"].Decode(&matter.POV); err != nil {
		return sceneFrontMatter{}, fmt.Errorf("decode %s pov: %w", path, err)
	}
	if err := values["status"].Decode(&matter.Status); err != nil {
		return sceneFrontMatter{}, fmt.Errorf("decode %s status: %w", path, err)
	}
	if err := values["exclude_from_ai"].Decode(&matter.ExcludeFromAI); err != nil {
		return sceneFrontMatter{}, fmt.Errorf("decode %s exclude_from_ai: %w", path, err)
	}
	return matter, nil
}

func quoteYAMLScalar(value string) string {
	return strconv.Quote(value)
}
