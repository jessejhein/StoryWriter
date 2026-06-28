// Package story defines the pure outline model and mutation rules.
package story

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

const (
	OutlineVersion = 1
	maxTitleRunes  = 200
)

var (
	ErrInvalidTitle    = errors.New("invalid title")
	ErrInvalidID       = errors.New("invalid ID")
	ErrParentNotFound  = errors.New("parent not found")
	ErrInvalidReorder  = errors.New("invalid reorder")
	ErrNoActiveProject = errors.New("no active project")
	ErrDirtyWorktree   = errors.New("story project has uncommitted changes")
	ErrSceneNotFound   = errors.New("scene not found")
	ErrInvalidPOV      = errors.New("invalid pov")
	ErrInvalidStatus   = errors.New("invalid status")
	ErrInvalidMarkdown = errors.New("invalid markdown")
	ErrInvalidRevision = errors.New("invalid revision")
	ErrStaleRevision   = errors.New("stale scene revision")
	ErrNoSceneChanges  = errors.New("scene save has no changes")
)

var revisionPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

var idPatterns = map[string]*regexp.Regexp{
	"arc":     regexp.MustCompile(`^arc_[0-9a-f]{20}$`),
	"chapter": regexp.MustCompile(`^ch_[0-9a-f]{20}$`),
	"scene":   regexp.MustCompile(`^scn_[0-9a-f]{20}$`),
}

// Outline is the read model returned by Milestone 1 APIs.
type Outline struct {
	Version int   `json:"version"`
	Arcs    []Arc `json:"arcs"`
}

// Arc is an ordered top-level story container.
type Arc struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	DisplayLabel string    `json:"display_label"`
	Chapters     []Chapter `json:"chapters"`
}

// Chapter belongs to exactly one arc.
type Chapter struct {
	ID           string  `json:"id"`
	ArcID        string  `json:"-"`
	Title        string  `json:"title"`
	DisplayLabel string  `json:"display_label"`
	Scenes       []Scene `json:"scenes"`
}

// Scene belongs to exactly one chapter.
type Scene struct {
	ID           string `json:"id"`
	ChapterID    string `json:"-"`
	Title        string `json:"title"`
	DisplayLabel string `json:"display_label"`
}

// SceneFrontMatter contains the editable canonical scene metadata.
type SceneFrontMatter struct {
	POV           string `json:"pov"`
	Status        string `json:"status"`
	ExcludeFromAI bool   `json:"exclude_from_ai"`
}

// SceneDocument is the editor-facing scene payload returned by Milestone 2.
type SceneDocument struct {
	ID          string           `json:"id"`
	ChapterID   string           `json:"chapter_id"`
	Title       string           `json:"title"`
	FrontMatter SceneFrontMatter `json:"frontmatter"`
	Markdown    string           `json:"markdown"`
	Revision    string           `json:"revision"`
	Canonical   []byte           `json:"-"`
}

// SaveSceneRequest is the validated save input for a canonical scene mutation.
type SaveSceneRequest struct {
	Title            string
	FrontMatter      SceneFrontMatter
	Markdown         string
	ExpectedRevision string
}

// ReorderRequest reorders direct children using stable IDs.
type ReorderRequest struct {
	ParentType      string   `json:"parent_type"`
	ParentID        string   `json:"parent_id"`
	OrderedChildIDs []string `json:"ordered_child_ids"`
}

// NewOutline returns an empty outline with the current schema version.
func NewOutline() Outline {
	return Outline{Version: OutlineVersion, Arcs: []Arc{}}
}

// ValidateArcID verifies an arc ID shape.
func ValidateArcID(id string) error {
	return validateID("arc", id)
}

// ValidateChapterID verifies a chapter ID shape.
func ValidateChapterID(id string) error {
	return validateID("chapter", id)
}

// ValidateSceneID verifies a scene ID shape.
func ValidateSceneID(id string) error {
	return validateID("scene", id)
}

// ValidatePOV trims and validates a POV field.
func ValidatePOV(value string) (string, error) {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) > maxTitleRunes {
		return "", fmt.Errorf("pov must be at most %d characters: %w", maxTitleRunes, ErrInvalidPOV)
	}
	return value, nil
}

// ValidateSceneStatus validates the fixed canonical scene status values.
func ValidateSceneStatus(value string) (string, error) {
	switch value {
	case "draft", "revised", "final":
		return value, nil
	default:
		return "", fmt.Errorf("status %q is unsupported: %w", value, ErrInvalidStatus)
	}
}

// ValidateRevision validates the opaque SHA-256 revision token shape.
func ValidateRevision(value string) error {
	if !revisionPattern.MatchString(value) {
		return fmt.Errorf("revision %q is invalid: %w", value, ErrInvalidRevision)
	}
	return nil
}

// NormalizeMarkdown validates and canonicalizes request markdown text.
func NormalizeMarkdown(value string) (string, error) {
	if strings.ContainsRune(value, '\x00') {
		return "", fmt.Errorf("markdown contains NUL byte: %w", ErrInvalidMarkdown)
	}
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	if !utf8.ValidString(value) {
		return "", fmt.Errorf("markdown is not valid UTF-8: %w", ErrInvalidMarkdown)
	}
	if len([]byte(value)) > 5<<20 {
		return "", fmt.Errorf("markdown exceeds 5 MiB limit: %w", ErrInvalidMarkdown)
	}
	return value, nil
}

// ValidateSceneSaveRequest validates a scene save request after JSON decoding.
func ValidateSceneSaveRequest(request SaveSceneRequest) (SaveSceneRequest, error) {
	title, err := ValidateTitle(request.Title)
	if err != nil {
		return SaveSceneRequest{}, err
	}
	pov, err := ValidatePOV(request.FrontMatter.POV)
	if err != nil {
		return SaveSceneRequest{}, err
	}
	status, err := ValidateSceneStatus(request.FrontMatter.Status)
	if err != nil {
		return SaveSceneRequest{}, err
	}
	markdown, err := NormalizeMarkdown(request.Markdown)
	if err != nil {
		return SaveSceneRequest{}, err
	}
	if err := ValidateRevision(request.ExpectedRevision); err != nil {
		return SaveSceneRequest{}, err
	}

	request.Title = title
	request.FrontMatter.POV = pov
	request.FrontMatter.Status = status
	request.Markdown = markdown
	return request, nil
}

// ComputeRevision returns the fixed opaque revision token for canonical bytes.
func ComputeRevision(contents []byte) string {
	digest := sha256.Sum256(contents)
	return "sha256:" + hex.EncodeToString(digest[:])
}

// ValidateTitle trims and validates a title.
func ValidateTitle(title string) (string, error) {
	return normalizeTitle(title)
}

// AddArc appends a new arc to the outline.
func AddArc(outline Outline, id, title string) (Outline, error) {
	if err := validateID("arc", id); err != nil {
		return Outline{}, err
	}
	title, err := normalizeTitle(title)
	if err != nil {
		return Outline{}, err
	}
	if containsArcID(outline, id) {
		return Outline{}, fmt.Errorf("arc %q already exists: %w", id, ErrInvalidID)
	}

	next := cloneOutline(outline)
	next.Arcs = append(next.Arcs, Arc{ID: id, Title: title, Chapters: []Chapter{}})
	return withDisplayLabels(next), nil
}

// AddChapter appends a new chapter to an existing arc.
func AddChapter(outline Outline, arcID, id, title string) (Outline, error) {
	if err := validateID("arc", arcID); err != nil {
		return Outline{}, err
	}
	if err := validateID("chapter", id); err != nil {
		return Outline{}, err
	}
	title, err := normalizeTitle(title)
	if err != nil {
		return Outline{}, err
	}
	if containsChapterID(outline, id) {
		return Outline{}, fmt.Errorf("chapter %q already exists: %w", id, ErrInvalidID)
	}

	next := cloneOutline(outline)
	for i := range next.Arcs {
		if next.Arcs[i].ID != arcID {
			continue
		}
		next.Arcs[i].Chapters = append(next.Arcs[i].Chapters, Chapter{
			ID:     id,
			ArcID:  arcID,
			Title:  title,
			Scenes: []Scene{},
		})
		return withDisplayLabels(next), nil
	}
	return Outline{}, fmt.Errorf("arc %q: %w", arcID, ErrParentNotFound)
}

// AddScene appends a new scene to an existing chapter.
func AddScene(outline Outline, chapterID, id, title string) (Outline, error) {
	if err := validateID("chapter", chapterID); err != nil {
		return Outline{}, err
	}
	if err := validateID("scene", id); err != nil {
		return Outline{}, err
	}
	title, err := normalizeTitle(title)
	if err != nil {
		return Outline{}, err
	}
	if containsSceneID(outline, id) {
		return Outline{}, fmt.Errorf("scene %q already exists: %w", id, ErrInvalidID)
	}

	next := cloneOutline(outline)
	for i := range next.Arcs {
		for j := range next.Arcs[i].Chapters {
			if next.Arcs[i].Chapters[j].ID != chapterID {
				continue
			}
			next.Arcs[i].Chapters[j].Scenes = append(next.Arcs[i].Chapters[j].Scenes, Scene{
				ID:        id,
				ChapterID: chapterID,
				Title:     title,
			})
			return withDisplayLabels(next), nil
		}
	}
	return Outline{}, fmt.Errorf("chapter %q: %w", chapterID, ErrParentNotFound)
}

// Reorder returns a new outline with chapters or scenes reordered.
func Reorder(outline Outline, request ReorderRequest) (Outline, error) {
	switch request.ParentType {
	case "arc":
		if err := validateID("arc", request.ParentID); err != nil {
			return Outline{}, err
		}
		return reorderChapters(outline, request.ParentID, request.OrderedChildIDs)
	case "chapter":
		if err := validateID("chapter", request.ParentID); err != nil {
			return Outline{}, err
		}
		return reorderScenes(outline, request.ParentID, request.OrderedChildIDs)
	default:
		return Outline{}, fmt.Errorf("parent_type %q: %w", request.ParentType, ErrInvalidReorder)
	}
}

func reorderChapters(outline Outline, arcID string, ordered []string) (Outline, error) {
	next := cloneOutline(outline)
	for i := range next.Arcs {
		if next.Arcs[i].ID != arcID {
			continue
		}
		reordered, err := reorderByID(next.Arcs[i].Chapters, ordered, func(chapter Chapter) string { return chapter.ID })
		if err != nil {
			return Outline{}, err
		}
		next.Arcs[i].Chapters = reordered
		return withDisplayLabels(next), nil
	}
	return Outline{}, fmt.Errorf("arc %q: %w", arcID, ErrParentNotFound)
}

func reorderScenes(outline Outline, chapterID string, ordered []string) (Outline, error) {
	next := cloneOutline(outline)
	for i := range next.Arcs {
		for j := range next.Arcs[i].Chapters {
			if next.Arcs[i].Chapters[j].ID != chapterID {
				continue
			}
			reordered, err := reorderByID(next.Arcs[i].Chapters[j].Scenes, ordered, func(scene Scene) string { return scene.ID })
			if err != nil {
				return Outline{}, err
			}
			next.Arcs[i].Chapters[j].Scenes = reordered
			return withDisplayLabels(next), nil
		}
	}
	return Outline{}, fmt.Errorf("chapter %q: %w", chapterID, ErrParentNotFound)
}

func reorderByID[T any](values []T, ordered []string, id func(T) string) ([]T, error) {
	if len(values) != len(ordered) {
		return nil, fmt.Errorf("expected %d child IDs, got %d: %w", len(values), len(ordered), ErrInvalidReorder)
	}

	index := make(map[string]T, len(values))
	for _, value := range values {
		index[id(value)] = value
	}
	next := make([]T, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, childID := range ordered {
		value, ok := index[childID]
		if !ok {
			return nil, fmt.Errorf("unknown child ID %q: %w", childID, ErrInvalidReorder)
		}
		if _, exists := seen[childID]; exists {
			return nil, fmt.Errorf("duplicate child ID %q: %w", childID, ErrInvalidReorder)
		}
		seen[childID] = struct{}{}
		next = append(next, value)
	}
	if len(seen) != len(values) {
		return nil, fmt.Errorf("child IDs do not match current children: %w", ErrInvalidReorder)
	}
	return next, nil
}

func withDisplayLabels(outline Outline) Outline {
	outline.Version = OutlineVersion
	for arcIndex := range outline.Arcs {
		outline.Arcs[arcIndex].DisplayLabel = fmt.Sprintf("Arc %d", arcIndex+1)
		for chapterIndex := range outline.Arcs[arcIndex].Chapters {
			outline.Arcs[arcIndex].Chapters[chapterIndex].DisplayLabel = fmt.Sprintf("Chapter %d.%d", arcIndex+1, chapterIndex+1)
			for sceneIndex := range outline.Arcs[arcIndex].Chapters[chapterIndex].Scenes {
				outline.Arcs[arcIndex].Chapters[chapterIndex].Scenes[sceneIndex].DisplayLabel = fmt.Sprintf("Scene %d.%d.%d", arcIndex+1, chapterIndex+1, sceneIndex+1)
			}
		}
	}
	return outline
}

func normalizeTitle(title string) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", fmt.Errorf("title is required: %w", ErrInvalidTitle)
	}
	if utf8.RuneCountInString(title) > maxTitleRunes {
		return "", fmt.Errorf("title must be at most %d characters: %w", maxTitleRunes, ErrInvalidTitle)
	}
	return title, nil
}

func validateID(kind, id string) error {
	pattern, ok := idPatterns[kind]
	if !ok {
		panic("unknown ID kind: " + kind)
	}
	if !pattern.MatchString(id) {
		return fmt.Errorf("%s ID %q is invalid: %w", kind, id, ErrInvalidID)
	}
	return nil
}

func cloneOutline(outline Outline) Outline {
	next := Outline{
		Version: outline.Version,
		Arcs:    make([]Arc, len(outline.Arcs)),
	}
	for i, arc := range outline.Arcs {
		next.Arcs[i] = Arc{
			ID:           arc.ID,
			Title:        arc.Title,
			DisplayLabel: arc.DisplayLabel,
			Chapters:     make([]Chapter, len(arc.Chapters)),
		}
		for j, chapter := range arc.Chapters {
			next.Arcs[i].Chapters[j] = Chapter{
				ID:           chapter.ID,
				ArcID:        chapter.ArcID,
				Title:        chapter.Title,
				DisplayLabel: chapter.DisplayLabel,
				Scenes:       slices.Clone(chapter.Scenes),
			}
		}
	}
	return next
}

func containsArcID(outline Outline, id string) bool {
	for _, arc := range outline.Arcs {
		if arc.ID == id {
			return true
		}
	}
	return false
}

func containsChapterID(outline Outline, id string) bool {
	for _, arc := range outline.Arcs {
		for _, chapter := range arc.Chapters {
			if chapter.ID == id {
				return true
			}
		}
	}
	return false
}

func containsSceneID(outline Outline, id string) bool {
	for _, arc := range outline.Arcs {
		for _, chapter := range arc.Chapters {
			for _, scene := range chapter.Scenes {
				if scene.ID == id {
					return true
				}
			}
		}
	}
	return false
}
