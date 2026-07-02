package contextpack

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// LexicalEvidence records deterministic mention signals for one candidate entry.
type LexicalEvidence struct {
	NameMention  bool
	AliasMention bool
	TagMention   bool
	Occurrences  int
}

// HasMention reports whether any lexical evidence exists.
func (evidence LexicalEvidence) HasMention() bool {
	return evidence.NameMention || evidence.AliasMention || evidence.TagMention
}

func (evidence LexicalEvidence) tier() int {
	switch {
	case evidence.NameMention:
		return 3
	case evidence.AliasMention:
		return 2
	case evidence.TagMention:
		return 1
	default:
		return 0
	}
}

// RankedEntry is one lexical relevance result in deterministic order.
type RankedEntry struct {
	EntryID   string
	Candidate CodexEntryCandidate
	Evidence  LexicalEvidence
}

// ChapterSceneCodex pairs one chapter scene with its candidate entry and text.
type ChapterSceneCodex struct {
	SceneID string
	Entry   CodexEntryCandidate
	Text    string
}

// DeduplicatedCodexState groups equal resolved states across chapter scenes.
type DeduplicatedCodexState struct {
	State    CodexEntryState
	SceneIDs []string
}

var entryTypeOrder = map[string]int{
	"character": 0,
	"location":  1,
	"lore":      2,
	"custom":    3,
}

// ComputeLexicalEvidence computes bounded mention evidence for one candidate.
func ComputeLexicalEvidence(text string, entry CodexEntryCandidate) LexicalEvidence {
	foldedText := fold(text)
	evidence := LexicalEvidence{}
	if entry.Name != "" {
		count := countBoundedMentions(foldedText, fold(entry.Name))
		if count > 0 {
			evidence.NameMention = true
			evidence.Occurrences += count
		}
	}
	for _, alias := range entry.Aliases {
		count := countBoundedMentions(foldedText, fold(alias))
		if count > 0 {
			evidence.AliasMention = true
			evidence.Occurrences += count
		}
	}
	for _, tag := range entry.Tags {
		count := countBoundedMentions(foldedText, fold(tag))
		if count > 0 {
			evidence.TagMention = true
			evidence.Occurrences += count
		}
	}
	return evidence
}

// RankLexicalRelevance orders mentioned candidates deterministically.
func RankLexicalRelevance(text string, entries []CodexEntryCandidate) []RankedEntry {
	ranked := make([]RankedEntry, 0, len(entries))
	for _, entry := range entries {
		evidence := ComputeLexicalEvidence(text, entry)
		if !evidence.HasMention() {
			continue
		}
		ranked = append(ranked, RankedEntry{
			EntryID:   entry.EntryID,
			Candidate: entry,
			Evidence:  evidence,
		})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		left := ranked[i]
		right := ranked[j]
		if left.Evidence.tier() != right.Evidence.tier() {
			return left.Evidence.tier() > right.Evidence.tier()
		}
		if left.Evidence.Occurrences != right.Evidence.Occurrences {
			return left.Evidence.Occurrences > right.Evidence.Occurrences
		}
		if entryTypeOrder[left.Candidate.EntryType] != entryTypeOrder[right.Candidate.EntryType] {
			return entryTypeOrder[left.Candidate.EntryType] < entryTypeOrder[right.Candidate.EntryType]
		}
		if left.Candidate.Name != right.Candidate.Name {
			return left.Candidate.Name < right.Candidate.Name
		}
		return left.EntryID < right.EntryID
	})
	return ranked
}

// ResolveRelevantEntry resolves one candidate at targetSceneID and requires lexical evidence.
func ResolveRelevantEntry(entry CodexEntryCandidate, text string, sceneOrder []SceneOrderRef, targetSceneID string) (CodexEntryState, error) {
	evidence := ComputeLexicalEvidence(text, entry)
	if !evidence.HasMention() {
		return CodexEntryState{}, fmt.Errorf("entry %q has no lexical evidence", entry.EntryID)
	}
	return resolveActiveState(entry, sceneOrder, targetSceneID)
}

// ResolveActiveState resolves one candidate at targetSceneID without lexical filtering.
func ResolveActiveState(entry CodexEntryCandidate, sceneOrder []SceneOrderRef, targetSceneID string) (CodexEntryState, error) {
	return resolveActiveState(entry, sceneOrder, targetSceneID)
}

func resolveActiveState(entry CodexEntryCandidate, sceneOrder []SceneOrderRef, targetSceneID string) (CodexEntryState, error) {
	sceneIndex := make(map[string]int, len(sceneOrder))
	targetIndex := -1
	for index, scene := range sceneOrder {
		sceneIndex[scene.ID] = index
		if scene.ID == targetSceneID {
			targetIndex = index
		}
	}
	if targetIndex < 0 {
		return CodexEntryState{}, fmt.Errorf("scene %q not found", targetSceneID)
	}
	type orderedProgression struct {
		index       int
		sceneIndex  int
		timingRank  int
		progression ProgressionInput
	}
	active := make([]orderedProgression, 0, len(entry.Progressions))
	for index, progression := range entry.Progressions {
		anchorIndex, ok := sceneIndex[progression.AnchorSceneID]
		if !ok {
			return CodexEntryState{}, fmt.Errorf("scene anchor %q is absent from the current outline", progression.AnchorSceneID)
		}
		timingRank := 1
		isActive := anchorIndex < targetIndex
		if progression.AnchorTiming == "before" {
			timingRank = 0
			isActive = anchorIndex <= targetIndex
		}
		if isActive {
			active = append(active, orderedProgression{
				index:       index,
				sceneIndex:  anchorIndex,
				timingRank:  timingRank,
				progression: progression,
			})
		}
	}
	sort.SliceStable(active, func(i, j int) bool {
		if active[i].sceneIndex != active[j].sceneIndex {
			return active[i].sceneIndex < active[j].sceneIndex
		}
		if active[i].timingRank != active[j].timingRank {
			return active[i].timingRank < active[j].timingRank
		}
		return active[i].index < active[j].index
	})
	description := entry.Description
	metadata := cloneStringMap(entry.Metadata)
	applied := make([]string, 0, len(active))
	for _, item := range active {
		if item.progression.Description != nil {
			description = *item.progression.Description
		}
		for key, value := range item.progression.Metadata {
			if metadata == nil {
				metadata = map[string]string{}
			}
			metadata[key] = value
		}
		applied = append(applied, item.progression.ID)
	}
	return CodexEntryState{
		EntryID:               entry.EntryID,
		EntryType:             entry.EntryType,
		Name:                  entry.Name,
		Description:           description,
		Metadata:              metadata,
		AppliedProgressionIDs: cloneStringSlice(applied),
	}, nil
}

// DeduplicateChapterCodexStates resolves and deduplicates chapter scene states.
func DeduplicateChapterCodexStates(scenes []ChapterSceneCodex, sceneOrder []SceneOrderRef) ([]DeduplicatedCodexState, error) {
	groups := make([]DeduplicatedCodexState, 0)
	for _, scene := range scenes {
		state, err := resolveActiveState(scene.Entry, sceneOrder, scene.SceneID)
		if err != nil {
			return nil, err
		}
		key := stateKey(state)
		if len(groups) > 0 && stateKey(groups[len(groups)-1].State) == key {
			groups[len(groups)-1].SceneIDs = append(groups[len(groups)-1].SceneIDs, scene.SceneID)
			continue
		}
		found := false
		for index := range groups {
			if stateKey(groups[index].State) == key {
				groups[index].SceneIDs = append(groups[index].SceneIDs, scene.SceneID)
				found = true
				break
			}
		}
		if found {
			continue
		}
		groups = append(groups, DeduplicatedCodexState{
			State:    state,
			SceneIDs: []string{scene.SceneID},
		})
	}
	return groups, nil
}

// ManifestCodexRefFromState converts one resolved state into a redacted manifest ref.
func ManifestCodexRefFromState(state CodexEntryState) ManifestCodexRef {
	return ManifestCodexRef{
		EntryID:               state.EntryID,
		AppliedProgressionIDs: cloneStringSlice(state.AppliedProgressionIDs),
	}
}

func stateKey(state CodexEntryState) string {
	metadataKeys := make([]string, 0, len(state.Metadata))
	for key := range state.Metadata {
		metadataKeys = append(metadataKeys, key)
	}
	sort.Strings(metadataKeys)
	var builder strings.Builder
	builder.WriteString(state.Description)
	for _, key := range metadataKeys {
		builder.WriteString("|")
		builder.WriteString(key)
		builder.WriteString("=")
		builder.WriteString(state.Metadata[key])
	}
	builder.WriteString("|")
	builder.WriteString(strings.Join(state.AppliedProgressionIDs, ","))
	return builder.String()
}

func fold(value string) string {
	return strings.Map(func(r rune) rune {
		return unicode.ToLower(r)
	}, value)
}

func countBoundedMentions(foldedText, foldedTerm string) int {
	if foldedTerm == "" {
		return 0
	}
	count := 0
	start := 0
	for {
		index := strings.Index(foldedText[start:], foldedTerm)
		if index < 0 {
			return count
		}
		absolute := start + index
		if hasBoundedMatch(foldedText, absolute, len(foldedTerm)) {
			count++
		}
		start = absolute + len(foldedTerm)
	}
}

func hasBoundedMatch(text string, start, length int) bool {
	if start > 0 {
		r, _ := utf8.DecodeLastRuneInString(text[:start])
		if isMentionChar(r) {
			return false
		}
	}
	end := start + length
	if end < len(text) {
		r, _ := utf8.DecodeRuneInString(text[end:])
		if isMentionChar(r) {
			return false
		}
	}
	return true
}

func isMentionChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}
