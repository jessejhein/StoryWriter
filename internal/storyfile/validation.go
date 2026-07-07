package storyfile

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"storywork/internal/story"
)

// ValidateCanonicalFiles rejects orphan arc, chapter, and scene files that are
// not referenced by the loaded outline.
func (s *Store) ValidateCanonicalFiles(_ context.Context, projectPath string, outline story.Outline) error {
	referencedArcs := make(map[string]struct{}, len(outline.Arcs))
	referencedChapters := make(map[string]struct{})
	referencedScenes := make(map[string]struct{})

	for _, arc := range outline.Arcs {
		referencedArcs[arc.ID] = struct{}{}
		for _, chapter := range arc.Chapters {
			referencedChapters[chapter.ID] = struct{}{}
			for _, scene := range chapter.Scenes {
				referencedScenes[scene.ID] = struct{}{}
			}
		}
	}

	if err := s.validateCanonicalDirectory(projectPath, "arcs", ".yaml", validateArcFileName, referencedArcs); err != nil {
		return err
	}
	if err := s.validateCanonicalDirectory(projectPath, "chapters", ".yaml", validateChapterFileName, referencedChapters); err != nil {
		return err
	}
	if err := s.validateCanonicalDirectory(projectPath, "scenes", ".md", validateSceneFileName, referencedScenes); err != nil {
		return err
	}
	return nil
}

func (s *Store) validateCanonicalDirectory(
	projectPath, directory, suffix string,
	validator func(string) error,
	referenced map[string]struct{},
) error {
	entries, err := os.ReadDir(filepath.Join(projectPath, directory))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.Name() == ".gitkeep" {
			continue
		}
		if entry.IsDir() {
			return invalidCanonical(fmt.Errorf("unexpected directory %s/%s", directory, entry.Name()))
		}
		if !strings.HasSuffix(entry.Name(), suffix) {
			return invalidCanonical(fmt.Errorf("unexpected file %s/%s", directory, entry.Name()))
		}
		id := strings.TrimSuffix(entry.Name(), suffix)
		if err := validator(id); err != nil {
			return invalidCanonical(fmt.Errorf("%s/%s: %w", directory, entry.Name(), err))
		}
		if _, ok := referenced[id]; !ok {
			return invalidCanonical(fmt.Errorf("orphan canonical file %s/%s", directory, entry.Name()))
		}
	}
	return nil
}

func validateArcFileName(id string) error {
	return story.ValidateArcID(id)
}

func validateChapterFileName(id string) error {
	return story.ValidateChapterID(id)
}

func validateSceneFileName(id string) error {
	return story.ValidateSceneID(id)
}
