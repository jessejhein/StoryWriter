package contextpack

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

var (
	// ErrForbiddenPack reports a forbidden or conflicting pack in policy.
	ErrForbiddenPack = errors.New("forbidden context pack")
	// ErrBudgetOverflow reports required context exceeding the input budget.
	ErrBudgetOverflow = errors.New("context budget overflow")
)

// Builder assembles typed packets and redacted manifests from policy and material.
type Builder struct{}

// NewBuilder returns the production context builder.
func NewBuilder() *Builder {
	return &Builder{}
}

// Build validates policy, applies deterministic budgeting, and returns one packet variant.
func (b *Builder) Build(request BuildRequest) (Packet, Manifest, error) {
	policy := clonePolicy(request.Policy)
	if err := validatePolicy(policy, request.Scope, request.RAGMode); err != nil {
		return nil, Manifest{}, err
	}
	if request.Estimator == nil {
		request.Estimator = ByteEstimator{}
	}
	maxInput := effectiveInputBudget(request.Budget)
	inputBudget := maxInput - request.Budget.ReservedOutputEstimatedTokens
	if inputBudget < 0 {
		return nil, Manifest{}, fmt.Errorf("reserved output exceeds input budget: %w", ErrBudgetOverflow)
	}

	manifest := Manifest{
		Scope:                   request.Scope,
		MaxInputEstimatedTokens: maxInput,
		RAGMode:                 request.RAGMode,
	}

	styleText := styleEstimateText(request.Material.Style)
	styleTokens, err := request.Estimator.SumChecked([]string{styleText})
	if err != nil {
		return nil, Manifest{}, err
	}
	if slices.Contains(policy.Required, PackStyleSheet) {
		manifest.PacksUsed = append(manifest.PacksUsed, PackStyleSheet)
	}
	usedTokens := styleTokens

	targetText, targetPack, err := targetTextForScope(request)
	if err != nil {
		return nil, Manifest{}, err
	}
	targetTokens, err := request.Estimator.SumChecked([]string{targetText})
	if err != nil {
		return nil, Manifest{}, err
	}
	if slices.Contains(policy.Required, targetPack) {
		manifest.PacksUsed = append(manifest.PacksUsed, targetPack)
	}
	if usedTokens+targetTokens > inputBudget {
		return nil, Manifest{}, fmt.Errorf("required target exceeds input budget: %w", ErrBudgetOverflow)
	}
	usedTokens += targetTokens

	var activeCodex []CodexEntryState
	if slices.Contains(policy.Required, PackActiveCodex) {
		activeCodex, err = resolveActiveCodexForScope(request)
		if err != nil {
			return nil, Manifest{}, err
		}
		for _, entry := range activeCodex {
			entryTokens, err := request.Estimator.SumChecked([]string{codexEstimateText(entry)})
			if err != nil {
				return nil, Manifest{}, err
			}
			if usedTokens+entryTokens > inputBudget {
				return nil, Manifest{}, fmt.Errorf("required codex exceeds input budget: %w", ErrBudgetOverflow)
			}
			usedTokens += entryTokens
			manifest.PacksUsed = appendIfMissing(manifest.PacksUsed, PackActiveCodex)
			manifest.ActiveCodex = append(manifest.ActiveCodex, ManifestCodexRefFromState(entry))
		}
	}

	var outlineNeighbors []OutlineNeighbor
	if slices.Contains(policy.Optional, PackOutlineNeighbor) && len(request.Material.OutlineNeighbors) > 0 {
		for _, neighbor := range request.Material.OutlineNeighbors {
			neighborTokens, err := request.Estimator.SumChecked([]string{neighbor.Text})
			if err != nil {
				return nil, Manifest{}, err
			}
			if usedTokens+neighborTokens > inputBudget {
				manifest.PacksOmitted = appendIfMissingOmission(manifest.PacksOmitted, PackOmission{
					Pack: PackOutlineNeighbor, Reason: OmissionReasonBudget,
				})
				break
			}
			usedTokens += neighborTokens
			outlineNeighbors = append(outlineNeighbors, neighbor)
			manifest.PacksUsed = appendIfMissing(manifest.PacksUsed, PackOutlineNeighbor)
			manifest.OutlineRefs = append(manifest.OutlineRefs, neighbor.ID)
		}
		if len(outlineNeighbors) == 0 && len(request.Material.OutlineNeighbors) > 0 &&
			!containsPackOmission(manifest.PacksOmitted, PackOutlineNeighbor) {
			manifest.PacksOmitted = append(manifest.PacksOmitted, PackOmission{
				Pack: PackOutlineNeighbor, Reason: OmissionReasonBudget,
			})
		}
	}

	manifest.EstimatedInputTokens = usedTokens

	switch request.Scope {
	case ScopeSelection:
		return SelectionPacket{
			SelectedText: request.Material.SelectionText,
			Style:        request.Material.Style,
		}, manifest, nil
	case ScopeScene:
		return ScenePacket{
			SceneMarkdown:    request.Material.SceneMarkdown,
			Style:            request.Material.Style,
			ActiveCodex:      cloneCodexEntryStates(activeCodex),
			OutlineNeighbors: cloneOutlineNeighbors(outlineNeighbors),
		}, manifest, nil
	case ScopeChapterReview:
		return ChapterReviewPacket{
			ChapterID:        chapterIDFromMaterial(request.Material),
			Style:            request.Material.Style,
			ChapterScenes:    cloneChapterScenes(request.Material.ChapterScenes),
			ActiveCodex:      cloneCodexEntryStates(activeCodex),
			OutlineNeighbors: cloneOutlineNeighbors(outlineNeighbors),
		}, manifest, nil
	default:
		return nil, Manifest{}, fmt.Errorf("scope %q is unsupported", request.Scope)
	}
}

func clonePolicy(policy Policy) Policy {
	return Policy{
		Required:  append([]Pack(nil), policy.Required...),
		Optional:  append([]Pack(nil), policy.Optional...),
		Forbidden: append([]Pack(nil), policy.Forbidden...),
	}
}

func validatePolicy(policy Policy, scope Scope, ragMode RAGMode) error {
	known := map[Pack]struct{}{
		PackSelectedText: {}, PackStyleSheet: {}, PackCurrentScene: {}, PackCurrentChapter: {},
		PackOutlineNeighbor: {}, PackActiveCodex: {},
	}
	seen := map[Pack]struct{}{}
	for _, pack := range append(append([]Pack{}, policy.Required...), append(policy.Optional, policy.Forbidden...)...) {
		if _, ok := known[pack]; !ok {
			return fmt.Errorf("context pack %q is unsupported: %w", pack, ErrForbiddenPack)
		}
	}
	for _, pack := range policy.Required {
		if _, ok := seen[pack]; ok {
			return fmt.Errorf("required pack %q is duplicated: %w", pack, ErrForbiddenPack)
		}
		if slices.Contains(policy.Forbidden, pack) {
			return fmt.Errorf("required pack %q is forbidden: %w", pack, ErrForbiddenPack)
		}
		seen[pack] = struct{}{}
	}
	for _, pack := range policy.Optional {
		if slices.Contains(policy.Required, pack) || slices.Contains(policy.Forbidden, pack) {
			return fmt.Errorf("optional pack %q conflicts with policy: %w", pack, ErrForbiddenPack)
		}
	}
	for _, pack := range policy.Forbidden {
		if slices.Contains(policy.Required, pack) {
			return fmt.Errorf("forbidden pack %q is required: %w", pack, ErrForbiddenPack)
		}
	}
	if scope == ScopeSelection && ragMode != RAGModeNone {
		return fmt.Errorf("selection scope requires rag mode none: %w", ErrForbiddenPack)
	}
	return nil
}

func effectiveInputBudget(budget Budget) int {
	maxInput := budget.MaxInputEstimatedTokens
	if budget.ProviderMaxInputTokens > 0 && budget.ProviderMaxInputTokens < maxInput {
		maxInput = budget.ProviderMaxInputTokens
	}
	return maxInput
}

func targetTextForScope(request BuildRequest) (string, Pack, error) {
	switch request.Scope {
	case ScopeSelection:
		return request.Material.SelectionText, PackSelectedText, nil
	case ScopeScene:
		return request.Material.SceneMarkdown, PackCurrentScene, nil
	case ScopeChapterReview:
		var builder strings.Builder
		for _, scene := range request.Material.ChapterScenes {
			builder.WriteString(scene.Markdown)
		}
		return builder.String(), PackCurrentChapter, nil
	default:
		return "", "", fmt.Errorf("scope %q is unsupported", request.Scope)
	}
}

func resolveActiveCodexForScope(request BuildRequest) ([]CodexEntryState, error) {
	switch request.Scope {
	case ScopeScene:
		targetSceneID, err := targetSceneIDFromMaterial(request.Material)
		if err != nil {
			return nil, err
		}
		ranked := RankLexicalRelevance(request.Material.SceneMarkdown, request.Material.CodexCandidates)
		states := make([]CodexEntryState, 0, len(ranked))
		for _, item := range ranked {
			state, err := resolveActiveState(item.Candidate, request.Material.SceneOrder, targetSceneID)
			if err != nil {
				return nil, err
			}
			states = append(states, state)
		}
		return states, nil
	case ScopeChapterReview:
		sceneCodex := make([]ChapterSceneCodex, 0, len(request.Material.ChapterScenes))
		for _, scene := range request.Material.ChapterScenes {
			for _, candidate := range request.Material.CodexCandidates {
				if !ComputeLexicalEvidence(scene.Markdown, candidate).HasMention() {
					continue
				}
				sceneCodex = append(sceneCodex, ChapterSceneCodex{
					SceneID: scene.SceneID,
					Entry:   candidate,
					Text:    scene.Markdown,
				})
			}
		}
		groups, err := DeduplicateChapterCodexStates(sceneCodex, request.Material.SceneOrder)
		if err != nil {
			return nil, err
		}
		states := make([]CodexEntryState, 0, len(groups))
		for _, group := range groups {
			states = append(states, group.State)
		}
		return states, nil
	default:
		return nil, fmt.Errorf("active codex is unsupported for scope %q", request.Scope)
	}
}

func styleEstimateText(style StyleSheet) string {
	return style.ID + style.Name + style.SystemPrompt
}

func codexEstimateText(entry CodexEntryState) string {
	return entry.EntryID + entry.Name + entry.Description
}

func targetSceneIDFromMaterial(material Material) (string, error) {
	if material.TargetSceneID != "" {
		return material.TargetSceneID, nil
	}
	if len(material.SceneOrder) == 0 {
		return "", fmt.Errorf("scene order is required for active codex resolution")
	}
	return material.SceneOrder[len(material.SceneOrder)-1].ID, nil
}

func chapterIDFromMaterial(material Material) string {
	for _, neighbor := range material.OutlineNeighbors {
		if neighbor.Kind == "chapter" {
			return neighbor.ID
		}
	}
	if len(material.ChapterScenes) > 0 {
		return ""
	}
	return ""
}

func appendIfMissing(values []Pack, pack Pack) []Pack {
	if slices.Contains(values, pack) {
		return values
	}
	return append(values, pack)
}

func appendIfMissingOmission(values []PackOmission, omission PackOmission) []PackOmission {
	if containsPackOmission(values, omission.Pack) {
		return values
	}
	return append(values, omission)
}

func containsPackOmission(values []PackOmission, pack Pack) bool {
	for _, value := range values {
		if value.Pack == pack {
			return true
		}
	}
	return false
}
