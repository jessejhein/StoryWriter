package contextpack

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidTarget reports an invalid action target payload.
	ErrInvalidTarget = errors.New("invalid context target")
)

// Scope identifies one explicit action target shape.
type Scope string

const (
	ScopeSelection     Scope = "selection"
	ScopeScene         Scope = "scene"
	ScopeChapterReview Scope = "chapter_review"
)

// Pack names one context slice referenced by policy and manifests.
type Pack string

const (
	PackSelectedText    Pack = "selected_text"
	PackStyleSheet      Pack = "style_sheet"
	PackCurrentScene    Pack = "current_scene"
	PackCurrentChapter  Pack = "current_chapter"
	PackOutlineNeighbor Pack = "outline_neighborhood"
	PackActiveCodex     Pack = "active_codex_at_position"
)

// RAGMode names deterministic context retrieval behavior.
type RAGMode string

const (
	RAGModeNone          RAGMode = "none"
	RAGModeTimelineAware RAGMode = "timeline_aware"
)

// OmissionReason explains why optional material was excluded.
type OmissionReason string

const (
	OmissionReasonBudget OmissionReason = "budget"
)

// Target is the validated action target used for context assembly.
type Target struct {
	Scope     Scope
	Selection *SelectionTarget
	Scene     *SceneTarget
	Chapter   *ChapterReviewTarget
}

// SelectionTarget addresses one UTF-8 byte range inside a canonical scene.
type SelectionTarget struct {
	SceneID       string
	SceneRevision string
	StartByte     int
	EndByte       int
	SelectedText  string
}

// SceneTarget addresses one canonical scene revision.
type SceneTarget struct {
	SceneID       string
	SceneRevision string
}

// ChapterReviewTarget addresses one chapter fingerprint snapshot.
type ChapterReviewTarget struct {
	ChapterID   string
	Fingerprint string
}

// Policy carries agent-declared required, optional, and forbidden packs.
type Policy struct {
	Required  []Pack
	Optional  []Pack
	Forbidden []Pack
}

// Budget stores conservative estimated-token limits.
type Budget struct {
	MaxInputEstimatedTokens       int
	ReservedOutputEstimatedTokens int
	ProviderMaxInputTokens        int
}

// StyleSheet is the redacted style instructions included in packets.
type StyleSheet struct {
	ID           string
	Name         string
	SystemPrompt string
}

// CodexEntryCandidate is one lexical relevance candidate with progression input.
type CodexEntryCandidate struct {
	EntryID      string
	EntryType    string
	Name         string
	Aliases      []string
	Tags         []string
	Description  string
	Metadata     map[string]string
	Progressions []ProgressionInput
}

// ProgressionInput is one canonical progression used during active-state resolution.
type ProgressionInput struct {
	ID            string
	AnchorSceneID string
	AnchorTiming  string
	Description   *string
	Metadata      map[string]string
}

// SceneOrderRef is one scene position in outline order.
type SceneOrderRef struct {
	ID string
}

// OutlineNeighbor is one bounded outline reference near the target.
type OutlineNeighbor struct {
	Kind string
	ID   string
	Text string
}

// Material is the typed canonical snapshot consumed by the pure builder.
type Material struct {
	Scope            Scope
	Style            StyleSheet
	SelectionText    string
	SceneMarkdown    string
	TargetSceneID    string
	ChapterScenes    []ChapterSceneText
	SceneOrder       []SceneOrderRef
	CodexCandidates  []CodexEntryCandidate
	OutlineNeighbors []OutlineNeighbor
}

// ChapterSceneText is one ordered scene body inside a chapter review target.
type ChapterSceneText struct {
	SceneID  string
	Markdown string
}

// CodexEntryState is one timeline-resolved Codex entry included in a packet.
type CodexEntryState struct {
	EntryID               string
	EntryType             string
	Name                  string
	Description           string
	Metadata              map[string]string
	AppliedProgressionIDs []string
}

// SelectionPacket is the minimal paragraph-scoped provider payload.
type SelectionPacket struct {
	SelectedText string
	Style        StyleSheet
}

// ScenePacket is the scene-scoped provider payload.
type ScenePacket struct {
	SceneMarkdown    string
	Style            StyleSheet
	ActiveCodex      []CodexEntryState
	OutlineNeighbors []OutlineNeighbor
}

// ChapterReviewPacket is the chapter-scoped suggestion payload.
type ChapterReviewPacket struct {
	ChapterID        string
	Style            StyleSheet
	ChapterScenes    []ChapterSceneText
	ActiveCodex      []CodexEntryState
	OutlineNeighbors []OutlineNeighbor
}

// Packet is one typed context payload variant.
type Packet interface {
	Scope() Scope
}

func (SelectionPacket) Scope() Scope     { return ScopeSelection }
func (ScenePacket) Scope() Scope         { return ScopeScene }
func (ChapterReviewPacket) Scope() Scope { return ScopeChapterReview }

// PackOmission records one omitted optional pack.
type PackOmission struct {
	Pack   Pack           `json:"pack"`
	Reason OmissionReason `json:"reason"`
}

// ManifestCodexRef is a redacted active-Codex reference for inspection APIs.
type ManifestCodexRef struct {
	EntryID               string   `json:"entry_id"`
	AppliedProgressionIDs []string `json:"applied_progression_ids"`
}

// Manifest is the redacted public context summary.
type Manifest struct {
	Scope                   Scope              `json:"scope"`
	PacksUsed               []Pack             `json:"packs_used"`
	PacksOmitted            []PackOmission     `json:"packs_omitted"`
	EstimatedInputTokens    int                `json:"estimated_input_tokens"`
	MaxInputEstimatedTokens int                `json:"max_input_estimated_tokens"`
	RAGMode                 RAGMode            `json:"rag_mode"`
	ActiveCodex             []ManifestCodexRef `json:"active_codex,omitempty"`
	OutlineRefs             []string           `json:"outline_refs,omitempty"`
}

// BuildRequest carries policy, material, and budgeting inputs.
type BuildRequest struct {
	Scope     Scope
	Policy    Policy
	Budget    Budget
	RAGMode   RAGMode
	Material  Material
	Estimator Estimator
}

// ValidateTarget ensures the target contains exactly one scope payload.
func ValidateTarget(target Target) error {
	count := 0
	if target.Selection != nil {
		count++
	}
	if target.Scene != nil {
		count++
	}
	if target.Chapter != nil {
		count++
	}
	if count != 1 {
		return fmt.Errorf("target must include exactly one scope payload: %w", ErrInvalidTarget)
	}
	switch target.Scope {
	case ScopeSelection:
		if target.Selection == nil {
			return fmt.Errorf("selection target is required: %w", ErrInvalidTarget)
		}
	case ScopeScene:
		if target.Scene == nil {
			return fmt.Errorf("scene target is required: %w", ErrInvalidTarget)
		}
	case ScopeChapterReview:
		if target.Chapter == nil {
			return fmt.Errorf("chapter review target is required: %w", ErrInvalidTarget)
		}
	default:
		return fmt.Errorf("scope %q is unsupported: %w", target.Scope, ErrInvalidTarget)
	}
	return nil
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func cloneStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	return append([]string(nil), values...)
}

func cloneCodexEntryState(entry CodexEntryState) CodexEntryState {
	return CodexEntryState{
		EntryID:               entry.EntryID,
		EntryType:             entry.EntryType,
		Name:                  entry.Name,
		Description:           entry.Description,
		Metadata:              cloneStringMap(entry.Metadata),
		AppliedProgressionIDs: cloneStringSlice(entry.AppliedProgressionIDs),
	}
}

func cloneCodexEntryStates(entries []CodexEntryState) []CodexEntryState {
	if len(entries) == 0 {
		return nil
	}
	cloned := make([]CodexEntryState, len(entries))
	for index, entry := range entries {
		cloned[index] = cloneCodexEntryState(entry)
	}
	return cloned
}

func cloneOutlineNeighbors(neighbors []OutlineNeighbor) []OutlineNeighbor {
	if len(neighbors) == 0 {
		return nil
	}
	cloned := make([]OutlineNeighbor, len(neighbors))
	copy(cloned, neighbors)
	return cloned
}

func cloneChapterScenes(scenes []ChapterSceneText) []ChapterSceneText {
	if len(scenes) == 0 {
		return nil
	}
	cloned := make([]ChapterSceneText, len(scenes))
	copy(cloned, scenes)
	return cloned
}
