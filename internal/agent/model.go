package agent

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"storywork/internal/provider"
)

const (
	maxIDLength          = 64
	maxNameRunes         = 100
	maxDescriptionRunes  = 500
	maxModelRunes        = 200
	maxSystemPromptRunes = 4000
)

var (
	ErrInvalidAgent = errors.New("invalid agent definition")
	ErrInvalidStyle = errors.New("invalid style definition")
	ErrRegistryLoad = errors.New("registry load failed")
	ErrInapplicable = errors.New("agent is not applicable")
)

var registryIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

type Surface string

const (
	SurfaceEditor      Surface = "editor"
	SurfaceChapterView Surface = "chapter_view"
)

type InputScope string

const (
	InputScopeSelection     InputScope = "selection"
	InputScopeScene         InputScope = "scene"
	InputScopeChapter       InputScope = "chapter"
	InputScopeChapterReview InputScope = "chapter_review"
)

type ContextPack string

const (
	ContextSelectedText     ContextPack = "selected_text"
	ContextSurrounding      ContextPack = "surrounding_paragraphs"
	ContextCurrentScene     ContextPack = "current_scene"
	ContextCurrentChapter   ContextPack = "current_chapter"
	ContextChapterSummary   ContextPack = "chapter_summary"
	ContextArcSummary       ContextPack = "arc_summary"
	ContextOutlineNeighbor  ContextPack = "outline_neighborhood"
	ContextActiveCodex      ContextPack = "active_codex_at_position"
	ContextGlobalCodexRAG   ContextPack = "global_codex_rag"
	ContextSeriesCodexRAG   ContextPack = "series_codex_rag"
	ContextRawImportNotes   ContextPack = "raw_import_notes"
	ContextPriorChat        ContextPack = "prior_chat"
	ContextVoiceSheet       ContextPack = "voice_sheet"
	ContextStyleSheet       ContextPack = "style_sheet"
	ContextContinuityEvents ContextPack = "continuity_events"
)

var allowedContextPacks = map[ContextPack]struct{}{
	ContextSelectedText:     {},
	ContextSurrounding:      {},
	ContextCurrentScene:     {},
	ContextCurrentChapter:   {},
	ContextChapterSummary:   {},
	ContextArcSummary:       {},
	ContextOutlineNeighbor:  {},
	ContextActiveCodex:      {},
	ContextGlobalCodexRAG:   {},
	ContextSeriesCodexRAG:   {},
	ContextRawImportNotes:   {},
	ContextPriorChat:        {},
	ContextVoiceSheet:       {},
	ContextStyleSheet:       {},
	ContextContinuityEvents: {},
}

type OutputMode string

const (
	OutputModePatch      OutputMode = "patch"
	OutputModeSuggestion OutputMode = "suggestion"
)

type OutputType string

const (
	OutputTypeReplacementText   OutputType = "replacement_text"
	OutputTypeRevisedText       OutputType = "revised_text"
	OutputTypeEditorialFindings OutputType = "editorial_findings"
)

type RAGMode string

const (
	RAGModeNone          RAGMode = "none"
	RAGModeTimelineAware RAGMode = "timeline_aware"
)

type FollowUpScope string

const (
	FollowUpScopeSelection     FollowUpScope = "selection"
	FollowUpScopeScene         FollowUpScope = "scene"
	FollowUpScopeChapterReview FollowUpScope = "chapter_review"
)

type FollowUpRelationship string

const (
	FollowUpRelationshipTriggered FollowUpRelationship = "triggered"
	FollowUpRelationshipDependsOn FollowUpRelationship = "depends_on"
)

// ContextBudget stores conservative estimated-token limits for version-3 agents.
type ContextBudget struct {
	MaxInputEstimatedTokens       int
	ReservedOutputEstimatedTokens int
}

// FollowUpRule names one deterministic follow-up invitation.
type FollowUpRule struct {
	AgentID      string
	Scope        FollowUpScope
	Relationship FollowUpRelationship
}

// FollowUpPolicy stores version-3 follow-up invitations keyed by lifecycle state.
type FollowUpPolicy struct {
	OnAccept []FollowUpRule
}

type Agent struct {
	Version           int
	ID                string
	Name              string
	Description       string
	AppliesWhen       ApplicabilityRule
	ModelRequirements ModelRequirements
	ContextPolicy     ContextPolicy
	ContextBudget     ContextBudget
	RAGPolicy         RAGPolicy
	FollowUps         FollowUpPolicy
	Control           Control
	Output            Output
}

type ModelRequirements struct {
	MinContextTokens         int
	SupportsStreaming        bool
	SupportsStructuredOutput bool
}

type ApplicabilityRule struct {
	Surfaces    []Surface
	InputScopes []InputScope
	MinWords    int
	MaxWords    int
}

type ContextPolicy struct {
	Required  []ContextPack
	Optional  []ContextPack
	Forbidden []ContextPack
}

type RAGPolicy struct {
	Mode RAGMode
}

type Control struct {
	OutputMode         OutputMode
	RequiresAcceptance bool
	CanModifyCanon     bool
}

type Output struct {
	Type                OutputType
	RequiresDiffPreview bool
}

type Style struct {
	Version           int
	ID                string
	Name              string
	ProviderProfileID string
	Model             string
	Temperature       float64
	SystemPrompt      string
}

type AvailabilityInput struct {
	Surface        Surface
	InputScope     InputScope
	SceneID        string
	SelectionWords int
}

type AvailabilityDecision struct {
	Agent          Agent
	Applicable     bool
	ExcludedReason string
}

type ContextPacket struct {
	SelectedText string
	Style        Style
}

type ContextSummary struct {
	PacksUsed []ContextPack `json:"packs_used"`
	RAGMode   RAGMode       `json:"rag_mode"`
}

type BuildContextRequest struct {
	Agent                  Agent
	Style                  Style
	SelectedText           string
	RequestedOptionalPacks []ContextPack
}

func ValidateAgent(agent Agent) (Agent, error) {
	if agent.Version == 3 {
		return ValidateAgentV3(agent)
	}
	agent.ID = strings.TrimSpace(agent.ID)
	agent.Name = strings.TrimSpace(agent.Name)
	agent.Description = strings.TrimSpace(agent.Description)
	if agent.Version != 1 && agent.Version != 2 {
		return Agent{}, fmt.Errorf("agent %q version %d is unsupported: %w", agent.ID, agent.Version, ErrInvalidAgent)
	}
	if !agent.ContextBudget.isZero() || !agent.FollowUps.isZero() {
		return Agent{}, fmt.Errorf("agent %q version %d cannot include version-3 fields: %w", agent.ID, agent.Version, ErrInvalidAgent)
	}
	if err := validateRegistryID(agent.ID); err != nil {
		return Agent{}, fmt.Errorf("agent id: %w", err)
	}
	if err := validateRunes("agent name", agent.Name, maxNameRunes); err != nil {
		return Agent{}, fmt.Errorf("%w: %w", ErrInvalidAgent, err)
	}
	if err := validateRunes("agent description", agent.Description, maxDescriptionRunes); err != nil {
		return Agent{}, fmt.Errorf("%w: %w", ErrInvalidAgent, err)
	}
	switch agent.Version {
	case 1:
		agent.ModelRequirements = ModelRequirements{
			MinContextTokens:         1,
			SupportsStreaming:        false,
			SupportsStructuredOutput: false,
		}
	case 2:
		if agent.ModelRequirements.MinContextTokens < 1 || agent.ModelRequirements.MinContextTokens > 10_000_000 {
			return Agent{}, fmt.Errorf("agent %q min_context_tokens %d is invalid: %w", agent.ID, agent.ModelRequirements.MinContextTokens, ErrInvalidAgent)
		}
	}
	if err := validateApplicability(agent.AppliesWhen); err != nil {
		return Agent{}, err
	}
	if err := validateContextPolicy(agent.ContextPolicy); err != nil {
		return Agent{}, err
	}
	if agent.RAGPolicy.Mode != RAGModeNone {
		return Agent{}, fmt.Errorf("agent %q rag mode %q is unsupported: %w", agent.ID, agent.RAGPolicy.Mode, ErrInvalidAgent)
	}
	if agent.Control.OutputMode != OutputModePatch || !agent.Control.RequiresAcceptance || agent.Control.CanModifyCanon {
		return Agent{}, fmt.Errorf("agent %q control is unsupported: %w", agent.ID, ErrInvalidAgent)
	}
	if agent.Output.Type != OutputTypeReplacementText && agent.Output.Type != OutputTypeRevisedText {
		return Agent{}, fmt.Errorf("agent %q output type %q is unsupported: %w", agent.ID, agent.Output.Type, ErrInvalidAgent)
	}
	if !agent.Output.RequiresDiffPreview {
		return Agent{}, fmt.Errorf("agent %q must require diff preview: %w", agent.ID, ErrInvalidAgent)
	}
	if slices.Contains(agent.AppliesWhen.InputScopes, InputScopeSelection) {
		if !slices.Contains(agent.ContextPolicy.Required, ContextSelectedText) || !slices.Contains(agent.ContextPolicy.Required, ContextStyleSheet) {
			return Agent{}, fmt.Errorf("agent %q selection execution requires selected_text and style_sheet: %w", agent.ID, ErrInvalidAgent)
		}
		if !slices.Contains(agent.ContextPolicy.Forbidden, ContextGlobalCodexRAG) || !slices.Contains(agent.ContextPolicy.Forbidden, ContextRawImportNotes) {
			return Agent{}, fmt.Errorf("agent %q selection execution must forbid global_codex_rag and raw_import_notes: %w", agent.ID, ErrInvalidAgent)
		}
	}
	return agent, nil
}

func ValidateStyle(style Style) (Style, error) {
	style.ID = strings.TrimSpace(style.ID)
	style.Name = strings.TrimSpace(style.Name)
	style.ProviderProfileID = strings.TrimSpace(style.ProviderProfileID)
	style.Model = strings.TrimSpace(style.Model)
	style.SystemPrompt = strings.TrimSpace(style.SystemPrompt)
	if style.Version != 1 && style.Version != 2 {
		return Style{}, fmt.Errorf("style %q version %d is unsupported: %w", style.ID, style.Version, ErrInvalidStyle)
	}
	if err := validateRegistryID(style.ID); err != nil {
		return Style{}, fmt.Errorf("style id: %w", err)
	}
	if err := validateRunes("style name", style.Name, maxNameRunes); err != nil {
		return Style{}, fmt.Errorf("%w: %w", ErrInvalidStyle, err)
	}
	switch style.Version {
	case 1:
		if style.ProviderProfileID != "mock_default" {
			return Style{}, fmt.Errorf("style %q provider_profile_id %q is unsupported: %w", style.ID, style.ProviderProfileID, ErrInvalidStyle)
		}
		if style.Model != "mock" {
			return Style{}, fmt.Errorf("style %q model %q is unsupported: %w", style.ID, style.Model, ErrInvalidStyle)
		}
	case 2:
		if err := validateRegistryID(style.ProviderProfileID); err != nil {
			return Style{}, fmt.Errorf("style %q provider_profile_id: %w", style.ID, err)
		}
		if !utf8.ValidString(style.Model) || strings.TrimSpace(style.Model) == "" || utf8.RuneCountInString(style.Model) > maxModelRunes {
			return Style{}, fmt.Errorf("style %q model %q is invalid: %w", style.ID, style.Model, ErrInvalidStyle)
		}
		for _, r := range style.Model {
			if unicode.IsControl(r) {
				return Style{}, fmt.Errorf("style %q model %q is invalid: %w", style.ID, style.Model, ErrInvalidStyle)
			}
		}
	}
	if style.Temperature < 0 || style.Temperature > 2 {
		return Style{}, fmt.Errorf("style %q temperature %.2f is invalid: %w", style.ID, style.Temperature, ErrInvalidStyle)
	}
	if err := validateRunes("style system_prompt", style.SystemPrompt, maxSystemPromptRunes); err != nil {
		return Style{}, fmt.Errorf("%w: %w", ErrInvalidStyle, err)
	}
	return style, nil
}

type CompatibilityReason string

const (
	CompatibilityMock                        CompatibilityReason = "mock"
	CompatibilityProfileNotFound             CompatibilityReason = "profile_not_found"
	CompatibilityMissingCredential           CompatibilityReason = "missing_credential"
	CompatibilityChatUnsupported             CompatibilityReason = "chat_unsupported"
	CompatibilityContextLimitTooSmall        CompatibilityReason = "context_limit_too_small"
	CompatibilityStreamingUnsupported        CompatibilityReason = "streaming_unsupported"
	CompatibilityStructuredOutputUnsupported CompatibilityReason = "structured_output_unsupported"
	CompatibilityCompatible                  CompatibilityReason = "compatible"
)

type CompatibilityDecision struct {
	Compatible bool
	Reason     CompatibilityReason
}

func Compatibility(agent Agent, style Style, profile *provider.Profile, readiness provider.Readiness) CompatibilityDecision {
	if style.Version == 1 && style.ProviderProfileID == "mock_default" && style.Model == "mock" {
		return CompatibilityDecision{Compatible: true, Reason: CompatibilityMock}
	}
	if profile == nil {
		return CompatibilityDecision{Reason: CompatibilityProfileNotFound}
	}
	if profile.Auth.Type == provider.AuthTypeBearerEnv && readiness != provider.ReadinessReady {
		return CompatibilityDecision{Reason: CompatibilityMissingCredential}
	}
	if !profile.Capabilities.Chat {
		return CompatibilityDecision{Reason: CompatibilityChatUnsupported}
	}
	if profile.Capabilities.MaxContextTokens < agent.ModelRequirements.MinContextTokens {
		return CompatibilityDecision{Reason: CompatibilityContextLimitTooSmall}
	}
	if agent.ModelRequirements.SupportsStreaming && !profile.Capabilities.Streaming {
		return CompatibilityDecision{Reason: CompatibilityStreamingUnsupported}
	}
	if agent.ModelRequirements.SupportsStructuredOutput && !profile.Capabilities.StructuredOutput {
		return CompatibilityDecision{Reason: CompatibilityStructuredOutputUnsupported}
	}
	return CompatibilityDecision{Compatible: true, Reason: CompatibilityCompatible}
}

// ExecutableCompatibility additionally accounts for capabilities not implemented
// by the current adapters, even when a profile declares them.
func ExecutableCompatibility(agent Agent, style Style, profile *provider.Profile, readiness provider.Readiness) CompatibilityDecision {
	decision := Compatibility(agent, style, profile, readiness)
	if !decision.Compatible || decision.Reason == CompatibilityMock {
		return decision
	}
	if agent.ModelRequirements.SupportsStreaming {
		return CompatibilityDecision{Reason: CompatibilityStreamingUnsupported}
	}
	if agent.ModelRequirements.SupportsStructuredOutput {
		return CompatibilityDecision{Reason: CompatibilityStructuredOutputUnsupported}
	}
	return decision
}

func ApplicableAgents(agents []Agent, input AvailabilityInput) []AvailabilityDecision {
	decisions := make([]AvailabilityDecision, 0, len(agents))
	for _, item := range agents {
		decision := AvailabilityDecision{Agent: item}
		switch {
		case !slices.Contains(item.AppliesWhen.Surfaces, input.Surface):
			decision.ExcludedReason = "surface does not match"
		case !slices.Contains(item.AppliesWhen.InputScopes, input.InputScope):
			decision.ExcludedReason = "input scope does not match"
		case input.InputScope == InputScopeSelection && strings.TrimSpace(input.SceneID) == "":
			decision.ExcludedReason = "scene_id is required for editor selections"
		case input.SelectionWords < item.AppliesWhen.MinWords:
			decision.ExcludedReason = "selection is too short"
		case input.SelectionWords > item.AppliesWhen.MaxWords:
			decision.ExcludedReason = "selection is too long"
		default:
			decision.Applicable = true
		}
		decisions = append(decisions, decision)
	}
	sort.Slice(decisions, func(i, j int) bool {
		left := decisions[i].Agent
		right := decisions[j].Agent
		if left.Name != right.Name {
			return left.Name < right.Name
		}
		return left.ID < right.ID
	})
	return decisions
}

func WordCount(text string) int {
	return len(strings.FieldsFunc(strings.TrimSpace(text), unicode.IsSpace))
}

func BuildContext(request BuildContextRequest) (ContextPacket, ContextSummary, error) {
	if _, err := ValidateAgent(request.Agent); err != nil {
		return ContextPacket{}, ContextSummary{}, err
	}
	if _, err := ValidateStyle(request.Style); err != nil {
		return ContextPacket{}, ContextSummary{}, err
	}
	for _, pack := range request.RequestedOptionalPacks {
		if slices.Contains(request.Agent.ContextPolicy.Forbidden, pack) {
			return ContextPacket{}, ContextSummary{}, fmt.Errorf("context pack %q is forbidden: %w", pack, ErrInvalidAgent)
		}
		if !slices.Contains(request.Agent.ContextPolicy.Optional, pack) {
			return ContextPacket{}, ContextSummary{}, fmt.Errorf("context pack %q is not an allowed optional pack: %w", pack, ErrInvalidAgent)
		}
	}
	packet := ContextPacket{
		SelectedText: request.SelectedText,
		Style:        request.Style,
	}
	summary := ContextSummary{
		PacksUsed: []ContextPack{ContextSelectedText, ContextStyleSheet},
		RAGMode:   request.Agent.RAGPolicy.Mode,
	}
	return packet, summary, nil
}

func SortAgents(agents []Agent) {
	sort.Slice(agents, func(i, j int) bool {
		if agents[i].Name != agents[j].Name {
			return agents[i].Name < agents[j].Name
		}
		return agents[i].ID < agents[j].ID
	})
}

func SortStyles(styles []Style) {
	sort.Slice(styles, func(i, j int) bool {
		if styles[i].Name != styles[j].Name {
			return styles[i].Name < styles[j].Name
		}
		return styles[i].ID < styles[j].ID
	})
}

func validateRegistryID(id string) error {
	if !registryIDPattern.MatchString(id) || len(id) > maxIDLength {
		return fmt.Errorf("%q is invalid: %w", id, ErrInvalidAgent)
	}
	return nil
}

func validateRunes(label, value string, max int) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", label)
	}
	if !utf8.ValidString(value) {
		return fmt.Errorf("%s must be valid UTF-8", label)
	}
	if utf8.RuneCountInString(value) > max {
		return fmt.Errorf("%s exceeds %d runes", label, max)
	}
	return nil
}

func validateApplicability(rule ApplicabilityRule) error {
	if len(rule.Surfaces) == 0 || len(rule.InputScopes) == 0 {
		return fmt.Errorf("agent applicability must include surfaces and input scopes: %w", ErrInvalidAgent)
	}
	if err := validateDistinctSurfaces(rule.Surfaces); err != nil {
		return err
	}
	if err := validateDistinctScopes(rule.InputScopes); err != nil {
		return err
	}
	if rule.MinWords < 0 || rule.MaxWords < 0 || rule.MinWords > rule.MaxWords {
		return fmt.Errorf("agent word bounds are invalid: %w", ErrInvalidAgent)
	}
	return nil
}

func validateDistinctSurfaces(values []Surface) error {
	seen := map[Surface]struct{}{}
	for _, value := range values {
		if value != SurfaceEditor && value != SurfaceChapterView {
			return fmt.Errorf("surface %q is unsupported: %w", value, ErrInvalidAgent)
		}
		if _, ok := seen[value]; ok {
			return fmt.Errorf("surface %q is duplicated: %w", value, ErrInvalidAgent)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func validateDistinctScopes(values []InputScope) error {
	seen := map[InputScope]struct{}{}
	for _, value := range values {
		if value != InputScopeSelection && value != InputScopeChapter {
			return fmt.Errorf("input scope %q is unsupported: %w", value, ErrInvalidAgent)
		}
		if _, ok := seen[value]; ok {
			return fmt.Errorf("input scope %q is duplicated: %w", value, ErrInvalidAgent)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func validateContextPolicy(policy ContextPolicy) error {
	if len(policy.Required) == 0 {
		return fmt.Errorf("required context is empty: %w", ErrInvalidAgent)
	}
	required := map[ContextPack]struct{}{}
	optional := map[ContextPack]struct{}{}
	forbidden := map[ContextPack]struct{}{}
	for _, item := range policy.Required {
		if err := validateContextPack(item); err != nil {
			return err
		}
		if _, ok := required[item]; ok {
			return fmt.Errorf("required context %q is duplicated: %w", item, ErrInvalidAgent)
		}
		required[item] = struct{}{}
	}
	for _, item := range policy.Optional {
		if err := validateContextPack(item); err != nil {
			return err
		}
		if _, ok := optional[item]; ok {
			return fmt.Errorf("optional context %q is duplicated: %w", item, ErrInvalidAgent)
		}
		if _, ok := required[item]; ok {
			return fmt.Errorf("context pack %q appears in required and optional: %w", item, ErrInvalidAgent)
		}
		optional[item] = struct{}{}
	}
	for _, item := range policy.Forbidden {
		if err := validateContextPack(item); err != nil {
			return err
		}
		if _, ok := forbidden[item]; ok {
			return fmt.Errorf("forbidden context %q is duplicated: %w", item, ErrInvalidAgent)
		}
		if _, ok := required[item]; ok {
			return fmt.Errorf("context pack %q appears in required and forbidden: %w", item, ErrInvalidAgent)
		}
		if _, ok := optional[item]; ok {
			return fmt.Errorf("context pack %q appears in optional and forbidden: %w", item, ErrInvalidAgent)
		}
		forbidden[item] = struct{}{}
	}
	return nil
}

func validateContextPack(pack ContextPack) error {
	if _, ok := allowedContextPacks[pack]; !ok {
		return fmt.Errorf("context pack %q is unsupported: %w", pack, ErrInvalidAgent)
	}
	return nil
}

// ValidateAgentV3 validates version-3 agent definitions with budgets and follow-up policy.
func ValidateAgentV3(agent Agent) (Agent, error) {
	agent.ID = strings.TrimSpace(agent.ID)
	agent.Name = strings.TrimSpace(agent.Name)
	agent.Description = strings.TrimSpace(agent.Description)
	if agent.Version != 3 {
		return Agent{}, fmt.Errorf("agent %q version %d is unsupported: %w", agent.ID, agent.Version, ErrInvalidAgent)
	}
	if err := validateRegistryID(agent.ID); err != nil {
		return Agent{}, fmt.Errorf("agent id: %w", err)
	}
	if err := validateRunes("agent name", agent.Name, maxNameRunes); err != nil {
		return Agent{}, fmt.Errorf("%w: %w", ErrInvalidAgent, err)
	}
	if err := validateRunes("agent description", agent.Description, maxDescriptionRunes); err != nil {
		return Agent{}, fmt.Errorf("%w: %w", ErrInvalidAgent, err)
	}
	if agent.ModelRequirements.MinContextTokens < 1 || agent.ModelRequirements.MinContextTokens > 10_000_000 {
		return Agent{}, fmt.Errorf("agent %q min_context_tokens %d is invalid: %w", agent.ID, agent.ModelRequirements.MinContextTokens, ErrInvalidAgent)
	}
	if err := validateApplicabilityV3(agent.AppliesWhen); err != nil {
		return Agent{}, err
	}
	if err := validateContextPolicy(agent.ContextPolicy); err != nil {
		return Agent{}, err
	}
	if err := validateContextBudget(agent.ContextBudget, agent.ModelRequirements.MinContextTokens); err != nil {
		return Agent{}, err
	}
	if err := validateRAGPolicyV3(agent.RAGPolicy); err != nil {
		return Agent{}, err
	}
	if err := validateControlV3(agent.Control, agent.Output); err != nil {
		return Agent{}, err
	}
	if err := validateOutputV3(agent.Output, agent.Control); err != nil {
		return Agent{}, err
	}
	if err := validateScopeContextPolicy(agent); err != nil {
		return Agent{}, err
	}
	if err := validateFollowUpPolicy(agent); err != nil {
		return Agent{}, err
	}
	return agent, nil
}

func validateApplicabilityV3(rule ApplicabilityRule) error {
	if len(rule.Surfaces) == 0 || len(rule.InputScopes) == 0 {
		return fmt.Errorf("agent applicability must include surfaces and input scopes: %w", ErrInvalidAgent)
	}
	if err := validateDistinctSurfaces(rule.Surfaces); err != nil {
		return err
	}
	if err := validateDistinctScopesV3(rule.InputScopes); err != nil {
		return err
	}
	if rule.MinWords < 0 || rule.MaxWords < 0 || rule.MinWords > rule.MaxWords {
		return fmt.Errorf("agent word bounds are invalid: %w", ErrInvalidAgent)
	}
	return nil
}

func validateDistinctScopesV3(values []InputScope) error {
	seen := map[InputScope]struct{}{}
	for _, value := range values {
		switch value {
		case InputScopeSelection, InputScopeScene, InputScopeChapter, InputScopeChapterReview:
		default:
			return fmt.Errorf("input scope %q is unsupported: %w", value, ErrInvalidAgent)
		}
		if _, ok := seen[value]; ok {
			return fmt.Errorf("input scope %q is duplicated: %w", value, ErrInvalidAgent)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func validateContextBudget(budget ContextBudget, minContextTokens int) error {
	if budget.MaxInputEstimatedTokens < 1 || budget.MaxInputEstimatedTokens > 10_000_000 {
		return fmt.Errorf("max_input_estimated_tokens %d is invalid: %w", budget.MaxInputEstimatedTokens, ErrInvalidAgent)
	}
	if budget.ReservedOutputEstimatedTokens < 0 || budget.ReservedOutputEstimatedTokens >= budget.MaxInputEstimatedTokens {
		return fmt.Errorf("reserved_output_estimated_tokens %d is invalid: %w", budget.ReservedOutputEstimatedTokens, ErrInvalidAgent)
	}
	if budget.MaxInputEstimatedTokens < minContextTokens {
		return fmt.Errorf("max_input_estimated_tokens %d is smaller than min_context_tokens %d: %w", budget.MaxInputEstimatedTokens, minContextTokens, ErrInvalidAgent)
	}
	return nil
}

func validateRAGPolicyV3(policy RAGPolicy) error {
	switch policy.Mode {
	case RAGModeNone, RAGModeTimelineAware:
		return nil
	default:
		return fmt.Errorf("rag mode %q is unsupported: %w", policy.Mode, ErrInvalidAgent)
	}
}

func validateControlV3(control Control, output Output) error {
	switch control.OutputMode {
	case OutputModePatch:
		if !control.RequiresAcceptance || control.CanModifyCanon {
			return fmt.Errorf("patch control is unsupported: %w", ErrInvalidAgent)
		}
	case OutputModeSuggestion:
		if control.RequiresAcceptance || control.CanModifyCanon {
			return fmt.Errorf("suggestion control is unsupported: %w", ErrInvalidAgent)
		}
	default:
		return fmt.Errorf("output mode %q is unsupported: %w", control.OutputMode, ErrInvalidAgent)
	}
	_ = output
	return nil
}

func validateOutputV3(output Output, control Control) error {
	switch output.Type {
	case OutputTypeReplacementText, OutputTypeRevisedText:
		if control.OutputMode != OutputModePatch || !output.RequiresDiffPreview {
			return fmt.Errorf("output type %q is incompatible with control: %w", output.Type, ErrInvalidAgent)
		}
	case OutputTypeEditorialFindings:
		if control.OutputMode != OutputModeSuggestion || output.RequiresDiffPreview {
			return fmt.Errorf("output type %q is incompatible with control: %w", output.Type, ErrInvalidAgent)
		}
	default:
		return fmt.Errorf("output type %q is unsupported: %w", output.Type, ErrInvalidAgent)
	}
	return nil
}

func validateScopeContextPolicy(agent Agent) error {
	scopes := agent.AppliesWhen.InputScopes
	if slices.Contains(scopes, InputScopeSelection) {
		if !slices.Contains(agent.ContextPolicy.Required, ContextSelectedText) || !slices.Contains(agent.ContextPolicy.Required, ContextStyleSheet) {
			return fmt.Errorf("agent %q selection execution requires selected_text and style_sheet: %w", agent.ID, ErrInvalidAgent)
		}
		for _, forbidden := range []ContextPack{ContextGlobalCodexRAG, ContextRawImportNotes, ContextPriorChat} {
			if !slices.Contains(agent.ContextPolicy.Forbidden, forbidden) {
				return fmt.Errorf("agent %q selection execution must forbid %q: %w", agent.ID, forbidden, ErrInvalidAgent)
			}
		}
		if agent.RAGPolicy.Mode != RAGModeNone {
			return fmt.Errorf("agent %q selection execution requires rag mode none: %w", agent.ID, ErrInvalidAgent)
		}
	}
	if slices.Contains(scopes, InputScopeScene) {
		for _, required := range []ContextPack{ContextCurrentScene, ContextStyleSheet} {
			if !slices.Contains(agent.ContextPolicy.Required, required) {
				return fmt.Errorf("agent %q scene execution requires %q: %w", agent.ID, required, ErrInvalidAgent)
			}
		}
		if agent.RAGPolicy.Mode == RAGModeTimelineAware && !slices.Contains(agent.ContextPolicy.Required, ContextActiveCodex) {
			return fmt.Errorf("agent %q scene execution requires %q: %w", agent.ID, ContextActiveCodex, ErrInvalidAgent)
		}
	}
	if slices.Contains(scopes, InputScopeChapterReview) {
		for _, required := range []ContextPack{ContextCurrentChapter, ContextStyleSheet, ContextActiveCodex} {
			if !slices.Contains(agent.ContextPolicy.Required, required) {
				return fmt.Errorf("agent %q chapter review requires %q: %w", agent.ID, required, ErrInvalidAgent)
			}
		}
		if agent.Control.OutputMode != OutputModeSuggestion || agent.Output.Type != OutputTypeEditorialFindings {
			return fmt.Errorf("agent %q chapter review must return editorial findings: %w", agent.ID, ErrInvalidAgent)
		}
		if !agent.ModelRequirements.SupportsStructuredOutput {
			return fmt.Errorf("agent %q chapter review requires structured output: %w", agent.ID, ErrInvalidAgent)
		}
	}
	return nil
}

var allowedFollowUpTargets = map[string]map[FollowUpScope]string{
	"line_polish": {
		FollowUpScopeScene: "scene_rewrite",
	},
	"scene_rewrite": {
		FollowUpScopeChapterReview: "chapter_review",
	},
}

func inputScopeRank(scope InputScope) int {
	switch scope {
	case InputScopeSelection:
		return 0
	case InputScopeScene:
		return 1
	case InputScopeChapter, InputScopeChapterReview:
		return 2
	default:
		return -1
	}
}

func followUpScopeRank(scope FollowUpScope) int {
	switch scope {
	case FollowUpScopeSelection:
		return 0
	case FollowUpScopeScene:
		return 1
	case FollowUpScopeChapterReview:
		return 2
	default:
		return -1
	}
}

func (budget ContextBudget) isZero() bool {
	return budget.MaxInputEstimatedTokens == 0 && budget.ReservedOutputEstimatedTokens == 0
}

func (policy FollowUpPolicy) isZero() bool {
	return len(policy.OnAccept) == 0
}

func validateFollowUpPolicy(agent Agent) error {
	if len(agent.FollowUps.OnAccept) == 0 {
		return nil
	}
	if agent.Control.OutputMode != OutputModePatch {
		return fmt.Errorf("agent %q follow-ups are only supported for patch actions: %w", agent.ID, ErrInvalidAgent)
	}
	primaryScope := agent.AppliesWhen.InputScopes[0]
	seen := map[string]struct{}{}
	for _, rule := range agent.FollowUps.OnAccept {
		if err := validateRegistryID(rule.AgentID); err != nil {
			return fmt.Errorf("follow-up agent id: %w", err)
		}
		key := rule.AgentID + ":" + string(rule.Scope)
		if _, ok := seen[key]; ok {
			return fmt.Errorf("follow-up %q is duplicated: %w", key, ErrInvalidAgent)
		}
		seen[key] = struct{}{}
		switch rule.Relationship {
		case FollowUpRelationshipTriggered:
		case FollowUpRelationshipDependsOn:
			return fmt.Errorf("follow-up relationship %q is unsupported on accept: %w", rule.Relationship, ErrInvalidAgent)
		default:
			return fmt.Errorf("follow-up relationship %q is unsupported: %w", rule.Relationship, ErrInvalidAgent)
		}
		if rule.AgentID == agent.ID {
			return fmt.Errorf("follow-up cannot target the same agent %q: %w", agent.ID, ErrInvalidAgent)
		}
		allowedTargets, ok := allowedFollowUpTargets[agent.ID]
		if !ok {
			return fmt.Errorf("agent %q does not support follow-ups: %w", agent.ID, ErrInvalidAgent)
		}
		expectedAgentID, ok := allowedTargets[rule.Scope]
		if !ok {
			return fmt.Errorf("follow-up scope %q is not allowed for agent %q: %w", rule.Scope, agent.ID, ErrInvalidAgent)
		}
		if rule.AgentID != expectedAgentID {
			return fmt.Errorf("follow-up agent %q does not match scope %q: %w", rule.AgentID, rule.Scope, ErrInvalidAgent)
		}
		if followUpScopeRank(rule.Scope) <= inputScopeRank(primaryScope) {
			return fmt.Errorf("follow-up scope %q must broaden beyond %q: %w", rule.Scope, primaryScope, ErrInvalidAgent)
		}
	}
	return nil
}
