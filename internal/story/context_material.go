package story

// context_material.go loads coherent canonical snapshots for action context assembly.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"

	"storywork/internal/codex"
	"storywork/internal/contextpack"
)

var (
	// ErrExcludedFromAI reports a scene flagged exclude_from_ai.
	ErrExcludedFromAI = errors.New("scene is excluded from AI")
)

// SelectionMaterialRequest addresses one UTF-8 selection inside a canonical scene.
type SelectionMaterialRequest struct {
	SceneID       string
	SceneRevision string
	StartByte     int
	EndByte       int
	SelectedText  string
}

// ChapterSceneRevision is one scene revision pair in chapter fingerprint order.
type ChapterSceneRevision struct {
	SceneID  string
	Revision string
}

// ContextMaterialResult is the typed snapshot consumed by the pure context builder.
type ContextMaterialResult struct {
	Material       contextpack.Material
	TargetRevision string
}

type contextSnapshot struct {
	projectPath  string
	outline      Outline
	outlineBytes []byte
}

// LoadSelectionMaterial loads one coherent selection snapshot under the read lock.
func (s *Service) LoadSelectionMaterial(ctx context.Context, request SelectionMaterialRequest) (ContextMaterialResult, error) {
	if err := ValidateSceneID(request.SceneID); err != nil {
		return ContextMaterialResult{}, err
	}
	if err := ValidateRevision(request.SceneRevision); err != nil {
		return ContextMaterialResult{}, err
	}

	s.mutations.RLock()
	defer s.mutations.RUnlock()

	snapshot, err := s.loadContextSnapshot(ctx)
	if err != nil {
		return ContextMaterialResult{}, err
	}
	scene, err := s.loadSceneLocked(ctx, snapshot, request.SceneID)
	if err != nil {
		return ContextMaterialResult{}, err
	}
	if scene.Revision != request.SceneRevision {
		return ContextMaterialResult{}, fmt.Errorf("scene %q revision changed: %w", request.SceneID, ErrStaleRevision)
	}
	selected, err := ValidateMarkdownSelection(scene.Markdown, request.StartByte, request.EndByte, request.SelectedText)
	if err != nil {
		return ContextMaterialResult{}, err
	}
	return ContextMaterialResult{
		Material: contextpack.Material{
			Scope:         contextpack.ScopeSelection,
			SelectionText: selected,
		},
		TargetRevision: scene.Revision,
	}, nil
}

// LoadSceneMaterial loads one coherent scene snapshot under the read lock.
func (s *Service) LoadSceneMaterial(ctx context.Context, sceneID, expectedRevision string) (ContextMaterialResult, error) {
	if err := ValidateSceneID(sceneID); err != nil {
		return ContextMaterialResult{}, err
	}
	if expectedRevision != "" {
		if err := ValidateRevision(expectedRevision); err != nil {
			return ContextMaterialResult{}, err
		}
	}

	s.mutations.RLock()
	defer s.mutations.RUnlock()

	snapshot, err := s.loadContextSnapshot(ctx)
	if err != nil {
		return ContextMaterialResult{}, err
	}
	scene, err := s.loadSceneLocked(ctx, snapshot, sceneID)
	if err != nil {
		return ContextMaterialResult{}, err
	}
	if expectedRevision != "" && scene.Revision != expectedRevision {
		return ContextMaterialResult{}, fmt.Errorf("scene %q revision changed: %w", sceneID, ErrStaleRevision)
	}
	sceneDocs, err := s.loadOutlineScenesLocked(ctx, snapshot, flattenOutlineScenes(snapshot.outline))
	if err != nil {
		return ContextMaterialResult{}, err
	}
	neighbors, err := outlineNeighborsForScene(snapshot.outline, sceneDocs, sceneID)
	if err != nil {
		return ContextMaterialResult{}, err
	}
	candidates, err := s.loadCodexCandidatesLocked(ctx, snapshot.projectPath)
	if err != nil {
		return ContextMaterialResult{}, err
	}
	return ContextMaterialResult{
		Material: contextpack.Material{
			Scope:            contextpack.ScopeScene,
			SceneMarkdown:    scene.Markdown,
			TargetSceneID:    sceneID,
			SceneOrder:       sceneOrderRefs(snapshot.outline),
			CodexCandidates:  candidates,
			OutlineNeighbors: neighbors,
		},
		TargetRevision: scene.Revision,
	}, nil
}

// LoadChapterMaterial loads one coherent chapter snapshot under the read lock.
func (s *Service) LoadChapterMaterial(ctx context.Context, chapterID string) (ContextMaterialResult, error) {
	if err := ValidateChapterID(chapterID); err != nil {
		return ContextMaterialResult{}, err
	}

	s.mutations.RLock()
	defer s.mutations.RUnlock()

	snapshot, err := s.loadContextSnapshot(ctx)
	if err != nil {
		return ContextMaterialResult{}, err
	}
	chapter, err := findChapter(snapshot.outline, chapterID)
	if err != nil {
		return ContextMaterialResult{}, err
	}
	orderedSceneIDs := make([]string, 0, len(chapter.Scenes))
	for _, scene := range chapter.Scenes {
		orderedSceneIDs = append(orderedSceneIDs, scene.ID)
	}
	sceneDocs, err := s.loadOutlineScenesLocked(ctx, snapshot, sceneRefsFromIDs(orderedSceneIDs))
	if err != nil {
		return ContextMaterialResult{}, err
	}
	chapterScenes := make([]contextpack.ChapterSceneText, 0, len(orderedSceneIDs))
	revisionPairs := make([]ChapterSceneRevision, 0, len(orderedSceneIDs))
	for _, sceneID := range orderedSceneIDs {
		scene := sceneDocs[sceneID]
		chapterScenes = append(chapterScenes, contextpack.ChapterSceneText{
			SceneID:  sceneID,
			Markdown: scene.Markdown,
		})
		revisionPairs = append(revisionPairs, ChapterSceneRevision{
			SceneID:  sceneID,
			Revision: scene.Revision,
		})
	}
	candidates, err := s.loadCodexCandidatesLocked(ctx, snapshot.projectPath)
	if err != nil {
		return ContextMaterialResult{}, err
	}
	return ContextMaterialResult{
		Material: contextpack.Material{
			Scope:            contextpack.ScopeChapterReview,
			ChapterScenes:    chapterScenes,
			SceneOrder:       sceneOrderRefs(snapshot.outline),
			CodexCandidates:  candidates,
			OutlineNeighbors: outlineNeighborsForChapter(snapshot.outline, chapterID),
		},
		TargetRevision: ComputeChapterFingerprint(snapshot.outlineBytes, revisionPairs),
	}, nil
}

// ComputeChapterFingerprint returns a deterministic SHA-256 over outline bytes and ordered scene revisions.
func ComputeChapterFingerprint(outlineBytes []byte, scenes []ChapterSceneRevision) string {
	digest := sha256.New()
	digest.Write(outlineBytes)
	for _, scene := range scenes {
		digest.Write([]byte(scene.SceneID))
		digest.Write([]byte(scene.Revision))
	}
	return "sha256:" + hex.EncodeToString(digest.Sum(nil))
}

func (s *Service) loadContextSnapshot(ctx context.Context) (contextSnapshot, error) {
	current, err := s.currentProject()
	if err != nil {
		return contextSnapshot{}, err
	}
	outline, err := s.files.Load(ctx, current.Path)
	if err != nil {
		return contextSnapshot{}, err
	}
	outlineBytes, err := s.files.MarshalOutline(outline)
	if err != nil {
		return contextSnapshot{}, err
	}
	return contextSnapshot{
		projectPath:  current.Path,
		outline:      outline,
		outlineBytes: outlineBytes,
	}, nil
}

func (s *Service) loadSceneLocked(ctx context.Context, snapshot contextSnapshot, sceneID string) (SceneDocument, error) {
	if _, err := findScene(snapshot.outline, sceneID); err != nil {
		if errors.Is(err, ErrParentNotFound) {
			return SceneDocument{}, fmt.Errorf("scene %q: %w", sceneID, ErrSceneNotFound)
		}
		return SceneDocument{}, err
	}
	scene, err := s.files.LoadScene(ctx, snapshot.projectPath, sceneID)
	if err != nil {
		return SceneDocument{}, err
	}
	if scene.FrontMatter.ExcludeFromAI {
		return SceneDocument{}, fmt.Errorf("scene %q: %w", sceneID, ErrExcludedFromAI)
	}
	return scene, nil
}

func (s *Service) loadOutlineScenesLocked(ctx context.Context, snapshot contextSnapshot, sceneOrder []codex.SceneRef) (map[string]SceneDocument, error) {
	result := make(map[string]SceneDocument, len(sceneOrder))
	for _, sceneRef := range sceneOrder {
		scene, err := s.loadSceneLocked(ctx, snapshot, sceneRef.ID)
		if err != nil {
			return nil, err
		}
		result[sceneRef.ID] = scene
	}
	return result, nil
}

func (s *Service) loadCodexCandidatesLocked(ctx context.Context, projectPath string) ([]contextpack.CodexEntryCandidate, error) {
	entries, err := s.files.LoadCodexEntries(ctx, projectPath)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ID < entries[j].ID
	})
	candidates := make([]contextpack.CodexEntryCandidate, 0, len(entries))
	for _, entry := range entries {
		progressions, err := s.files.LoadProgressions(ctx, projectPath, entry.ID)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, codexEntryCandidate(entry, progressions.Progressions))
	}
	return candidates, nil
}

func codexEntryCandidate(entry codex.Entry, progressions []codex.Progression) contextpack.CodexEntryCandidate {
	inputs := make([]contextpack.ProgressionInput, 0, len(progressions))
	for _, progression := range progressions {
		inputs = append(inputs, contextpack.ProgressionInput{
			ID:            progression.ID,
			AnchorSceneID: progression.Anchor.ID,
			AnchorTiming:  progression.Anchor.Timing,
			Description:   progression.Changes.Description,
			Metadata:      cloneMetadata(progression.Changes.Metadata),
		})
	}
	return contextpack.CodexEntryCandidate{
		EntryID:      entry.ID,
		EntryType:    string(entry.Type),
		Name:         entry.Name,
		Aliases:      append([]string(nil), entry.Aliases...),
		Tags:         append([]string(nil), entry.Tags...),
		Description:  entry.Description,
		Metadata:     cloneMetadata(entry.Metadata),
		Progressions: inputs,
	}
}

func cloneMetadata(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func sceneOrderRefs(outline Outline) []contextpack.SceneOrderRef {
	flat := flattenOutlineScenes(outline)
	refs := make([]contextpack.SceneOrderRef, len(flat))
	for index, scene := range flat {
		refs[index] = contextpack.SceneOrderRef{ID: scene.ID}
	}
	return refs
}

func sceneRefsFromIDs(sceneIDs []string) []codex.SceneRef {
	refs := make([]codex.SceneRef, len(sceneIDs))
	for index, sceneID := range sceneIDs {
		refs[index] = codex.SceneRef{ID: sceneID}
	}
	return refs
}

type flatOutlinePosition struct {
	sceneID    string
	chapterID  string
	chapter    Chapter
	scene      Scene
	flatIndex  int
	chapterPos int
}

func flattenOutlinePositions(outline Outline) []flatOutlinePosition {
	positions := make([]flatOutlinePosition, 0)
	flatIndex := 0
	chapterPos := 0
	for _, arc := range outline.Arcs {
		for _, chapter := range arc.Chapters {
			chapterPos++
			for _, scene := range chapter.Scenes {
				positions = append(positions, flatOutlinePosition{
					sceneID: scene.ID, chapterID: chapter.ID,
					chapter: chapter, scene: scene,
					flatIndex: flatIndex, chapterPos: chapterPos,
				})
				flatIndex++
			}
		}
	}
	return positions
}

func outlineNeighborsForScene(outline Outline, scenes map[string]SceneDocument, targetSceneID string) ([]contextpack.OutlineNeighbor, error) {
	positions := flattenOutlinePositions(outline)
	targetIndex := -1
	var targetChapterID string
	for index, position := range positions {
		if position.sceneID == targetSceneID {
			targetIndex = index
			targetChapterID = position.chapterID
			break
		}
	}
	if targetIndex < 0 {
		return nil, fmt.Errorf("scene %q: %w", targetSceneID, ErrSceneNotFound)
	}

	neighbors := make([]contextpack.OutlineNeighbor, 0, 4)
	if targetIndex > 0 {
		prevID := positions[targetIndex-1].sceneID
		neighbors = append(neighbors, contextpack.OutlineNeighbor{
			Kind: "scene", ID: prevID, Text: scenes[prevID].Markdown,
		})
	}
	if targetIndex+1 < len(positions) {
		nextID := positions[targetIndex+1].sceneID
		neighbors = append(neighbors, contextpack.OutlineNeighbor{
			Kind: "scene", ID: nextID, Text: scenes[nextID].Markdown,
		})
	}
	chapters := flattenOutlineChapters(outline)
	chapterIndex := -1
	for index, chapter := range chapters {
		if chapter.ID == targetChapterID {
			chapterIndex = index
			break
		}
	}
	if chapterIndex > 0 {
		prev := chapters[chapterIndex-1]
		neighbors = append(neighbors, contextpack.OutlineNeighbor{
			Kind: "chapter", ID: prev.ID, Text: prev.Title,
		})
	}
	if chapterIndex >= 0 && chapterIndex+1 < len(chapters) {
		next := chapters[chapterIndex+1]
		neighbors = append(neighbors, contextpack.OutlineNeighbor{
			Kind: "chapter", ID: next.ID, Text: next.Title,
		})
	}
	return neighbors, nil
}

func outlineNeighborsForChapter(outline Outline, chapterID string) []contextpack.OutlineNeighbor {
	chapters := flattenOutlineChapters(outline)
	chapterIndex := -1
	for index, chapter := range chapters {
		if chapter.ID == chapterID {
			chapterIndex = index
			break
		}
	}
	if chapterIndex < 0 {
		return nil
	}
	neighbors := make([]contextpack.OutlineNeighbor, 0, 2)
	if chapterIndex > 0 {
		prev := chapters[chapterIndex-1]
		neighbors = append(neighbors, contextpack.OutlineNeighbor{
			Kind: "chapter", ID: prev.ID, Text: prev.Title,
		})
	}
	if chapterIndex+1 < len(chapters) {
		next := chapters[chapterIndex+1]
		neighbors = append(neighbors, contextpack.OutlineNeighbor{
			Kind: "chapter", ID: next.ID, Text: next.Title,
		})
	}
	return neighbors
}

func flattenOutlineChapters(outline Outline) []Chapter {
	chapters := make([]Chapter, 0)
	for _, arc := range outline.Arcs {
		chapters = append(chapters, arc.Chapters...)
	}
	return chapters
}
