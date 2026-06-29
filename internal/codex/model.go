package codex

// model.go defines Codex entries, progression validation, and active-state resolution.

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	// Version is the canonical schema version for Codex and progression documents.
	Version               = 1
	maxNameRunes          = 200
	maxTagRunes           = 64
	maxDescriptionBytes   = 64 << 10
	maxMetadataEntries    = 100
	maxMetadataKeyRunes   = 100
	maxMetadataValueBytes = 4 << 10
)

var (
	// ErrInvalidType reports an unsupported Codex entry type.
	ErrInvalidType = errors.New("invalid codex type")
	// ErrInvalidID reports a malformed Codex or progression ID.
	ErrInvalidID = errors.New("invalid codex ID")
	// ErrInvalidName reports an invalid canonical entry name.
	ErrInvalidName = errors.New("invalid codex name")
	// ErrInvalidAlias reports invalid aliases for a Codex entry.
	ErrInvalidAlias = errors.New("invalid codex alias")
	// ErrInvalidTag reports invalid Codex tags.
	ErrInvalidTag = errors.New("invalid codex tag")
	// ErrInvalidDescription reports invalid canonical description content.
	ErrInvalidDescription = errors.New("invalid codex description")
	// ErrInvalidMetadata reports invalid Codex metadata keys or values.
	ErrInvalidMetadata = errors.New("invalid codex metadata")
	// ErrInvalidRevision reports an invalid Codex revision token.
	ErrInvalidRevision = errors.New("invalid codex revision")
	// ErrInvalidProgression reports invalid progression structure or changes.
	ErrInvalidProgression = errors.New("invalid codex progression")
	// ErrEntryNotFound reports a missing canonical Codex entry.
	ErrEntryNotFound = errors.New("codex entry not found")
	// ErrSceneNotFound reports a missing scene used during active-state resolution.
	ErrSceneNotFound = errors.New("scene not found")
	// ErrNoChanges reports a Codex save request with no effective canonical change.
	ErrNoChanges = errors.New("codex save has no changes")
)

var (
	revisionPattern      = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	tagPattern           = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
	characterIDPattern   = regexp.MustCompile(`^char_[0-9a-f]{20}$`)
	locationIDPattern    = regexp.MustCompile(`^loc_[0-9a-f]{20}$`)
	loreIDPattern        = regexp.MustCompile(`^lore_[0-9a-f]{20}$`)
	customIDPattern      = regexp.MustCompile(`^custom_[0-9a-f]{20}$`)
	progressionIDPattern = regexp.MustCompile(`^prog_[0-9a-f]{20}$`)
	sceneIDPattern       = regexp.MustCompile(`^scn_[0-9a-f]{20}$`)
)

var typeOrder = map[EntryType]int{
	TypeCharacter: 0,
	TypeLocation:  1,
	TypeLore:      2,
	TypeCustom:    3,
}

// EntryType identifies one supported Codex entry category.
type EntryType string

const (
	// TypeCharacter stores canonical character facts.
	TypeCharacter EntryType = "character"
	// TypeLocation stores canonical location facts.
	TypeLocation EntryType = "location"
	// TypeLore stores canonical lore facts.
	TypeLore EntryType = "lore"
	// TypeCustom stores canonical custom Codex facts.
	TypeCustom EntryType = "custom"
)

// Entry is one canonical Codex entry plus its optional loaded revision.
// Canonical holds the exact canonical bytes the entry was loaded from; it is
// excluded from JSON so transport responses stay canonical-shape only.
type Entry struct {
	Version     int               `json:"version,omitempty"`
	ID          string            `json:"id"`
	Type        EntryType         `json:"type"`
	Name        string            `json:"name"`
	Aliases     []string          `json:"aliases"`
	Tags        []string          `json:"tags"`
	Description string            `json:"description"`
	Metadata    map[string]string `json:"metadata"`
	Revision    string            `json:"revision,omitempty"`
	Canonical   []byte            `json:"-"`
}

// ProgressionDocument stores one entry's ordered canonical timeline changes.
// Version mirrors the canonical schema version but is not part of the HTTP
// response shape, so it is excluded from JSON serialization. Canonical holds
// the exact canonical bytes the document was loaded from.
type ProgressionDocument struct {
	Version      int               `json:"-"`
	EntryID      string            `json:"entry_id"`
	Progressions []Progression     `json:"progressions"`
	Revision     *string           `json:"revision"`
	Canonical    []byte            `json:"-"`
}

// Progression applies one change set at a stable scene anchor.
type Progression struct {
	ID      string            `json:"id,omitempty"`
	Anchor  ProgressionAnchor `json:"anchor"`
	Changes ProgressionChange `json:"changes"`
}

// ProgressionAnchor identifies the stable scene and timing for one progression.
type ProgressionAnchor struct {
	Type   string `json:"type"`
	ID     string `json:"id"`
	Timing string `json:"timing"`
}

// ProgressionChange contains the canonical fields a progression may override.
type ProgressionChange struct {
	Description *string           `json:"description,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// ActiveState is the resolved Codex entry state as of one target scene.
type ActiveState struct {
	SceneID               string   `json:"scene_id"`
	Entry                 Entry    `json:"entry"`
	AppliedProgressionIDs []string `json:"applied_progression_ids"`
}

// SaveEntryRequest carries normalized create or update input from callers.
type SaveEntryRequest struct {
	Type             EntryType
	Name             string
	Aliases          []string
	Tags             []string
	Description      string
	Metadata         map[string]string
	ExpectedRevision string
}

// SaveProgressionsRequest carries one full ordered progression replacement request.
type SaveProgressionsRequest struct {
	Progressions     []Progression
	ExpectedRevision *string
}

// ValidateEntryType verifies that value is one supported Codex entry type.
func ValidateEntryType(value EntryType) (EntryType, error) {
	switch value {
	case TypeCharacter, TypeLocation, TypeLore, TypeCustom:
		return value, nil
	default:
		return "", fmt.Errorf("type %q is unsupported: %w", value, ErrInvalidType)
	}
}

// ValidateEntryID verifies the shape of one stable Codex entry ID.
func ValidateEntryID(id string) error {
	switch {
	case characterIDPattern.MatchString(id):
		return nil
	case locationIDPattern.MatchString(id):
		return nil
	case loreIDPattern.MatchString(id):
		return nil
	case customIDPattern.MatchString(id):
		return nil
	default:
		return fmt.Errorf("entry ID %q is invalid: %w", id, ErrInvalidID)
	}
}

// ValidateEntryIDForType verifies that id matches both the generic ID shape and the requested entry type.
func ValidateEntryIDForType(entryType EntryType, id string) error {
	if _, err := ValidateEntryType(entryType); err != nil {
		return err
	}
	if err := ValidateEntryID(id); err != nil {
		return err
	}
	if prefixForType(entryType) != strings.SplitN(id, "_", 2)[0]+"_" {
		return fmt.Errorf("entry ID %q does not match type %q: %w", id, entryType, ErrInvalidID)
	}
	return nil
}

// ValidateProgressionID verifies the shape of one stable progression ID.
func ValidateProgressionID(id string) error {
	if !progressionIDPattern.MatchString(id) {
		return fmt.Errorf("progression ID %q is invalid: %w", id, ErrInvalidID)
	}
	return nil
}

// ValidateSceneID verifies the shape of a stable scene ID used by progressions.
func ValidateSceneID(id string) error {
	if !sceneIDPattern.MatchString(id) {
		return fmt.Errorf("scene ID %q is invalid: %w", id, ErrInvalidID)
	}
	return nil
}

// ValidateRevision verifies the opaque SHA-256 revision token shape.
func ValidateRevision(value string) error {
	if !revisionPattern.MatchString(value) {
		return fmt.Errorf("revision %q is invalid: %w", value, ErrInvalidRevision)
	}
	return nil
}

// NormalizeCreateRequest trims and validates a create request before ID assignment.
func NormalizeCreateRequest(request SaveEntryRequest) (SaveEntryRequest, error) {
	entryType, err := ValidateEntryType(request.Type)
	if err != nil {
		return SaveEntryRequest{}, err
	}
	entry, err := NormalizeEntry(Entry{
		Version:     Version,
		Type:        entryType,
		Name:        request.Name,
		Aliases:     request.Aliases,
		Tags:        request.Tags,
		Description: request.Description,
		Metadata:    request.Metadata,
	})
	if err != nil {
		return SaveEntryRequest{}, err
	}
	request.Type = entry.Type
	request.Name = entry.Name
	request.Aliases = entry.Aliases
	request.Tags = entry.Tags
	request.Description = entry.Description
	request.Metadata = entry.Metadata
	request.ExpectedRevision = ""
	return request, nil
}

// NormalizeUpdateRequest trims, validates, and applies mutable fields to an existing entry.
func NormalizeUpdateRequest(entryID string, current Entry, request SaveEntryRequest) (Entry, error) {
	if err := ValidateEntryID(entryID); err != nil {
		return Entry{}, err
	}
	if err := ValidateRevision(request.ExpectedRevision); err != nil {
		return Entry{}, err
	}
	next, err := NormalizeEntry(Entry{
		Version:     Version,
		ID:          current.ID,
		Type:        current.Type,
		Name:        request.Name,
		Aliases:     request.Aliases,
		Tags:        request.Tags,
		Description: request.Description,
		Metadata:    request.Metadata,
	})
	if err != nil {
		return Entry{}, err
	}
	if next.ID != current.ID || next.Type != current.Type {
		return Entry{}, fmt.Errorf("immutable codex fields changed: %w", ErrInvalidID)
	}
	return next, nil
}

// NormalizeEntry canonicalizes one Codex entry independent of storage or transport.
func NormalizeEntry(entry Entry) (Entry, error) {
	entry.Version = Version
	entry.Metadata = cloneMetadata(entry.Metadata)
	var err error
	if entry.ID != "" {
		if err = ValidateEntryIDForType(entry.Type, entry.ID); err != nil {
			return Entry{}, err
		}
	} else if _, err = ValidateEntryType(entry.Type); err != nil {
		return Entry{}, err
	}

	entry.Name, err = normalizeName(entry.Name)
	if err != nil {
		return Entry{}, err
	}
	entry.Aliases, err = normalizeAliases(entry.Name, entry.Aliases)
	if err != nil {
		return Entry{}, err
	}
	entry.Tags, err = normalizeTags(entry.Tags)
	if err != nil {
		return Entry{}, err
	}
	entry.Description, err = normalizeDescription(entry.Description)
	if err != nil {
		return Entry{}, err
	}
	entry.Metadata, err = normalizeMetadata(entry.Metadata)
	if err != nil {
		return Entry{}, err
	}
	return entry, nil
}

// NormalizeProgressions canonicalizes a progression list and validates its scene anchors against the supplied outline.
func NormalizeProgressions(entryID string, progressions []Progression, sceneIDs map[string]struct{}) ([]Progression, error) {
	return normalizeProgressionList(entryID, progressions, sceneIDs)
}

// NormalizeStoredProgressions canonicalizes stored progressions without validating them against a current outline.
func NormalizeStoredProgressions(entryID string, progressions []Progression) ([]Progression, error) {
	return normalizeProgressionList(entryID, progressions, nil)
}

func normalizeProgressionList(entryID string, progressions []Progression, sceneIDs map[string]struct{}) ([]Progression, error) {
	if err := ValidateEntryID(entryID); err != nil {
		return nil, err
	}
	seenIDs := make(map[string]struct{}, len(progressions))
	seenAnchors := make(map[string]struct{}, len(progressions))
	next := make([]Progression, 0, len(progressions))
	for index, progression := range progressions {
		normalized, err := normalizeProgression(progression)
		if err != nil {
			return nil, fmt.Errorf("progression %d: %w", index, err)
		}
		if normalized.ID != "" {
			if _, exists := seenIDs[normalized.ID]; exists {
				return nil, fmt.Errorf("progression %q is duplicated: %w", normalized.ID, ErrInvalidProgression)
			}
			seenIDs[normalized.ID] = struct{}{}
		}
		if sceneIDs != nil {
			if _, ok := sceneIDs[normalized.Anchor.ID]; !ok {
				return nil, fmt.Errorf("scene anchor %q is unknown: %w", normalized.Anchor.ID, ErrInvalidProgression)
			}
		}
		anchorKey := normalized.Anchor.ID + ":" + normalized.Anchor.Timing
		if _, exists := seenAnchors[anchorKey]; exists {
			return nil, fmt.Errorf("anchor %s is duplicated: %w", anchorKey, ErrInvalidProgression)
		}
		seenAnchors[anchorKey] = struct{}{}
		next = append(next, normalized)
	}
	return next, nil
}

// SceneRef is the minimal scene-order input required for active-state resolution.
type SceneRef struct {
	ID string
}

// ResolveActiveState deterministically resolves one entry's state at targetSceneID using the supplied current outline order.
func ResolveActiveState(entry Entry, progressions []Progression, orderedScenes []SceneRef, targetSceneID string) (ActiveState, error) {
	if err := ValidateSceneID(targetSceneID); err != nil {
		return ActiveState{}, err
	}
	base, err := NormalizeEntry(entry)
	if err != nil {
		return ActiveState{}, err
	}
	sceneIndex := make(map[string]int, len(orderedScenes))
	targetIndex := -1
	for index, scene := range orderedScenes {
		if err := ValidateSceneID(scene.ID); err != nil {
			return ActiveState{}, err
		}
		sceneIndex[scene.ID] = index
		if scene.ID == targetSceneID {
			targetIndex = index
		}
	}
	if targetIndex < 0 {
		return ActiveState{}, fmt.Errorf("scene %q: %w", targetSceneID, ErrSceneNotFound)
	}
	normalizedProgressions, err := NormalizeStoredProgressions(base.ID, progressions)
	if err != nil {
		return ActiveState{}, err
	}
	type orderedProgression struct {
		index       int
		sceneIndex  int
		timingRank  int
		progression Progression
	}
	active := make([]orderedProgression, 0, len(normalizedProgressions))
	for index, progression := range normalizedProgressions {
		anchorIndex, ok := sceneIndex[progression.Anchor.ID]
		if !ok {
			return ActiveState{}, fmt.Errorf("scene anchor %q is absent from the current outline", progression.Anchor.ID)
		}
		timingRank := 1
		isActive := anchorIndex < targetIndex
		if progression.Anchor.Timing == "before" {
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
	resolved := base
	// The resolved entry is a derived projection, not a canonical document, so it
	// carries no revision and no schema version tag.
	resolved.Revision = ""
	resolved.Version = 0
	applied := make([]string, 0, len(active))
	for _, item := range active {
		if item.progression.Changes.Description != nil {
			resolved.Description = *item.progression.Changes.Description
		}
		for key, value := range item.progression.Changes.Metadata {
			resolved.Metadata[key] = value
		}
		applied = append(applied, item.progression.ID)
	}
	return ActiveState{
		SceneID:               targetSceneID,
		Entry:                 resolved,
		AppliedProgressionIDs: applied,
	}, nil
}

// ComputeRevision returns the fixed revision token for canonical bytes.
func ComputeRevision(contents []byte) string {
	digest := sha256.Sum256(contents)
	return "sha256:" + hex.EncodeToString(digest[:])
}

// SortEntries orders entries by type, then display name, then stable ID.
func SortEntries(entries []Entry) {
	sort.Slice(entries, func(i, j int) bool {
		if typeOrder[entries[i].Type] != typeOrder[entries[j].Type] {
			return typeOrder[entries[i].Type] < typeOrder[entries[j].Type]
		}
		if entries[i].Name != entries[j].Name {
			return entries[i].Name < entries[j].Name
		}
		return entries[i].ID < entries[j].ID
	})
}

// DirectoryForType returns the canonical subdirectory name for one entry type.
func DirectoryForType(entryType EntryType) (string, error) {
	switch entryType {
	case TypeCharacter:
		return "characters", nil
	case TypeLocation:
		return "locations", nil
	case TypeLore:
		return "lore", nil
	case TypeCustom:
		return "custom", nil
	default:
		return "", fmt.Errorf("type %q is unsupported: %w", entryType, ErrInvalidType)
	}
}

// TypeForID infers the entry type from one validated stable entry ID.
func TypeForID(id string) (EntryType, error) {
	switch {
	case characterIDPattern.MatchString(id):
		return TypeCharacter, nil
	case locationIDPattern.MatchString(id):
		return TypeLocation, nil
	case loreIDPattern.MatchString(id):
		return TypeLore, nil
	case customIDPattern.MatchString(id):
		return TypeCustom, nil
	default:
		return "", fmt.Errorf("entry ID %q is invalid: %w", id, ErrInvalidID)
	}
}

func prefixForType(entryType EntryType) string {
	switch entryType {
	case TypeCharacter:
		return "char_"
	case TypeLocation:
		return "loc_"
	case TypeLore:
		return "lore_"
	case TypeCustom:
		return "custom_"
	default:
		return ""
	}
}

func normalizeName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("name must not be empty: %w", ErrInvalidName)
	}
	if utf8.RuneCountInString(value) > maxNameRunes {
		return "", fmt.Errorf("name must be at most %d characters: %w", maxNameRunes, ErrInvalidName)
	}
	return value, nil
}

func normalizeAliases(name string, aliases []string) ([]string, error) {
	seen := make(map[string]struct{}, len(aliases))
	next := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		trimmed := strings.TrimSpace(alias)
		if trimmed == "" {
			return nil, fmt.Errorf("alias must not be empty: %w", ErrInvalidAlias)
		}
		if utf8.RuneCountInString(trimmed) > maxNameRunes {
			return nil, fmt.Errorf("alias %q must be at most %d characters: %w", trimmed, maxNameRunes, ErrInvalidAlias)
		}
		if trimmed == name {
			return nil, fmt.Errorf("alias %q duplicates name: %w", trimmed, ErrInvalidAlias)
		}
		if _, exists := seen[trimmed]; exists {
			return nil, fmt.Errorf("alias %q is duplicated: %w", trimmed, ErrInvalidAlias)
		}
		seen[trimmed] = struct{}{}
		next = append(next, trimmed)
	}
	return next, nil
}

func normalizeTags(tags []string) ([]string, error) {
	seen := make(map[string]struct{}, len(tags))
	next := make([]string, 0, len(tags))
	for _, tag := range tags {
		trimmed := strings.TrimSpace(tag)
		if trimmed == "" {
			return nil, fmt.Errorf("tag must not be empty: %w", ErrInvalidTag)
		}
		if utf8.RuneCountInString(trimmed) > maxTagRunes {
			return nil, fmt.Errorf("tag %q must be at most %d characters: %w", trimmed, maxTagRunes, ErrInvalidTag)
		}
		if !tagPattern.MatchString(trimmed) {
			return nil, fmt.Errorf("tag %q is invalid: %w", trimmed, ErrInvalidTag)
		}
		if _, exists := seen[trimmed]; exists {
			return nil, fmt.Errorf("tag %q is duplicated: %w", trimmed, ErrInvalidTag)
		}
		seen[trimmed] = struct{}{}
		next = append(next, trimmed)
	}
	sort.Strings(next)
	return next, nil
}

func normalizeDescription(value string) (string, error) {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	if strings.ContainsRune(value, '\x00') {
		return "", fmt.Errorf("description contains NUL byte: %w", ErrInvalidDescription)
	}
	if !utf8.ValidString(value) {
		return "", fmt.Errorf("description is not valid UTF-8: %w", ErrInvalidDescription)
	}
	if len([]byte(value)) > maxDescriptionBytes {
		return "", fmt.Errorf("description exceeds %d bytes: %w", maxDescriptionBytes, ErrInvalidDescription)
	}
	return value, nil
}

func normalizeMetadata(metadata map[string]string) (map[string]string, error) {
	if metadata == nil {
		return map[string]string{}, nil
	}
	if len(metadata) > maxMetadataEntries {
		return nil, fmt.Errorf("metadata exceeds %d entries: %w", maxMetadataEntries, ErrInvalidMetadata)
	}
	trimmed := make(map[string]string, len(metadata))
	for key, value := range metadata {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" {
			return nil, fmt.Errorf("metadata key must not be empty: %w", ErrInvalidMetadata)
		}
		if utf8.RuneCountInString(normalizedKey) > maxMetadataKeyRunes {
			return nil, fmt.Errorf("metadata key %q is too long: %w", normalizedKey, ErrInvalidMetadata)
		}
		if _, exists := trimmed[normalizedKey]; exists {
			return nil, fmt.Errorf("metadata key %q is duplicated after trim: %w", normalizedKey, ErrInvalidMetadata)
		}
		normalizedValue := strings.ReplaceAll(value, "\r\n", "\n")
		normalizedValue = strings.ReplaceAll(normalizedValue, "\r", "\n")
		if strings.ContainsRune(normalizedValue, '\x00') {
			return nil, fmt.Errorf("metadata value for %q contains NUL byte: %w", normalizedKey, ErrInvalidMetadata)
		}
		if !utf8.ValidString(normalizedValue) {
			return nil, fmt.Errorf("metadata value for %q is not valid UTF-8: %w", normalizedKey, ErrInvalidMetadata)
		}
		if len([]byte(normalizedValue)) > maxMetadataValueBytes {
			return nil, fmt.Errorf("metadata value for %q exceeds %d bytes: %w", normalizedKey, maxMetadataValueBytes, ErrInvalidMetadata)
		}
		trimmed[normalizedKey] = normalizedValue
	}
	return trimmed, nil
}

func normalizeProgression(progression Progression) (Progression, error) {
	if progression.ID != "" {
		if err := ValidateProgressionID(progression.ID); err != nil {
			return Progression{}, err
		}
	}
	if progression.Anchor.Type != "scene" {
		return Progression{}, fmt.Errorf("anchor type %q is unsupported: %w", progression.Anchor.Type, ErrInvalidProgression)
	}
	if err := ValidateSceneID(progression.Anchor.ID); err != nil {
		return Progression{}, err
	}
	switch progression.Anchor.Timing {
	case "before", "after":
	default:
		return Progression{}, fmt.Errorf("anchor timing %q is unsupported: %w", progression.Anchor.Timing, ErrInvalidProgression)
	}
	changes, err := normalizeProgressionChange(progression.Changes)
	if err != nil {
		return Progression{}, err
	}
	progression.Changes = changes
	return progression, nil
}

func normalizeProgressionChange(changes ProgressionChange) (ProgressionChange, error) {
	hasDescription := changes.Description != nil
	metadataPresent := changes.Metadata != nil
	if hasDescription {
		description, err := normalizeDescription(*changes.Description)
		if err != nil {
			return ProgressionChange{}, err
		}
		changes.Description = &description
	}
	if metadataPresent {
		if len(changes.Metadata) == 0 {
			return ProgressionChange{}, fmt.Errorf("progression metadata must be non-empty when present: %w", ErrInvalidProgression)
		}
		metadata, err := normalizeMetadata(changes.Metadata)
		if err != nil {
			return ProgressionChange{}, err
		}
		changes.Metadata = metadata
	} else {
		changes.Metadata = nil
	}
	if !hasDescription && !metadataPresent {
		return ProgressionChange{}, fmt.Errorf("progression must change description or metadata: %w", ErrInvalidProgression)
	}
	return changes, nil
}

func cloneMetadata(metadata map[string]string) map[string]string {
	if metadata == nil {
		return map[string]string{}
	}
	cloned := make(map[string]string, len(metadata))
	for key, value := range metadata {
		cloned[key] = value
	}
	return cloned
}
