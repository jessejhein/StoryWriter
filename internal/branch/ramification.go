package branch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"storywork/internal/modelchat"
)

// Analyzer performs bounded ramification analysis without mutating the repository.
type Analyzer interface {
	Analyze(context.Context, AnalysisPacket) (AnalysisResult, error)
}

// AnalysisPacket is the bounded provider input.
type AnalysisPacket struct {
	Goal       string
	Comparison Comparison
	DiffText   string
	ProfileID  string
	Model      string
}

// ModelchatAnalyzer adapts modelchat transport for branch analysis.
type ModelchatAnalyzer struct {
	Resolver  func(context.Context, string) (modelchat.Request, error)
	Completer modelchat.Completer
	Client    *http.Client
}

var analysisProfileIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// Analyze sends the bounded packet to the configured provider.
func (a *ModelchatAnalyzer) Analyze(ctx context.Context, packet AnalysisPacket) (AnalysisResult, error) {
	if a == nil || a.Resolver == nil || a.Completer == nil {
		return AnalysisResult{}, ErrProviderUnavailable
	}
	base, err := a.Resolver(ctx, packet.ProfileID)
	if err != nil {
		return AnalysisResult{}, mapAnalyzerError(err)
	}
	base.Model = packet.Model
	base.Messages = []modelchat.Message{
		{Role: "system", Content: "Return one strict JSON object with summary and findings only."},
		{Role: "user", Content: buildRamificationPrompt(packet)},
	}
	response, err := a.Completer.Complete(ctx, a.Client, base)
	if err != nil {
		return AnalysisResult{}, mapAnalyzerError(err)
	}
	allowed := indexComparisonPaths(packet.Comparison.Files)
	summary, findings, err := ParseRamificationOutput([]byte(response.Content), allowed)
	if err != nil {
		return AnalysisResult{}, ErrInvalidAnalysisOutput
	}
	return AnalysisResult{
		Summary:  summary,
		Findings: findings,
		Provider: ProviderIdentity{
			ProfileID: response.Provider.ProfileID,
			Type:      string(response.Provider.Type),
			Model:     response.Provider.Model,
		},
	}, nil
}

func indexComparisonPaths(files []ChangedFile) map[ProjectPath]struct{} {
	allowed := make(map[ProjectPath]struct{}, len(files))
	for _, file := range files {
		allowed[file.Path] = struct{}{}
	}
	return allowed
}

const (
	promptLabelGoal         = "Goal: "
	promptLabelChangedFiles = "\nChanged files:\n"
	promptLabelDiff         = "\nDiff:\n"
)

func buildRamificationPrompt(packet AnalysisPacket) string {
	var builder strings.Builder
	builder.WriteString(promptLabelGoal)
	builder.WriteString(packet.Goal)
	builder.WriteString(promptLabelChangedFiles)
	for _, file := range packet.Comparison.Files {
		builder.WriteString(string(file.Status))
		builder.WriteByte(' ')
		builder.WriteString(string(file.Path))
		builder.WriteByte('\n')
	}
	builder.WriteString(promptLabelDiff)
	builder.WriteString(packet.DiffText)
	return builder.String()
}

func AnalysisPromptOverhead(goal string, files []ChangedFile) int {
	overhead := len(promptLabelGoal) + len(goal) + len(promptLabelChangedFiles)
	for _, file := range files {
		overhead += len(file.Status) + 1 + len(file.Path) + 1
	}
	overhead += len(promptLabelDiff)
	return overhead
}

// BuildAnalysisPacket constructs the bounded diff packet under a read lock.
func BuildAnalysisPacket(goal string, comparison Comparison, diffText string) (AnalysisPacket, RedactedManifest, error) {
	goal = strings.TrimSpace(goal)
	if goal == "" || len(goal) > MaxGoalBytes {
		return AnalysisPacket{}, RedactedManifest{}, ErrInvalidAnalysis
	}
	if len(comparison.Files) > MaxAnalysisFiles {
		return AnalysisPacket{}, RedactedManifest{}, ErrAnalysisBudget
	}
	included := make([]ProjectPath, 0, len(comparison.Files))
	for _, file := range comparison.Files {
		included = append(included, file.Path)
	}
	packet := AnalysisPacket{
		Goal:       goal,
		Comparison: comparison,
		DiffText:   diffText,
	}
	rendered := buildRamificationPrompt(packet)
	if len(rendered) > MaxAnalysisPacket {
		return AnalysisPacket{}, RedactedManifest{}, ErrAnalysisBudget
	}
	manifest := RedactedManifest{
		MainHead:         comparison.MainHead,
		ExperimentHead:   comparison.ExperimentHead,
		Fingerprint:      comparison.Fingerprint,
		ChangedFileCount: len(comparison.Files),
		IncludedPaths:    SortProjectPaths(included),
		EstimatedBytes:   len(rendered),
	}
	return packet, manifest, nil
}

// ParseRamificationOutput validates strict provider JSON output.
func ParseRamificationOutput(raw []byte, allowedPaths map[ProjectPath]struct{}) (string, []RamificationFinding, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || !utf8.Valid(trimmed) {
		return "", nil, ErrInvalidAnalysisOutput
	}
	if trimmed[0] != '{' {
		return "", nil, ErrInvalidAnalysisOutput
	}
	if err := rejectDuplicateJSONKeys(trimmed); err != nil {
		return "", nil, ErrInvalidAnalysisOutput
	}
	type findingWire struct {
		Category          *string   `json:"category"`
		Severity          *string   `json:"severity"`
		Title             *string   `json:"title"`
		Explanation       *string   `json:"explanation"`
		AffectedPaths     *[]string `json:"affected_paths"`
		RecommendedAction *string   `json:"recommended_action"`
	}
	var payload struct {
		Summary  *string        `json:"summary"`
		Findings *[]findingWire `json:"findings"`
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return "", nil, ErrInvalidAnalysisOutput
	}
	if err := decoder.Decode(new(any)); err == nil {
		return "", nil, ErrInvalidAnalysisOutput
	}
	if payload.Summary == nil || payload.Findings == nil || *payload.Summary == "" || len(*payload.Summary) > 4000 {
		return "", nil, ErrInvalidAnalysisOutput
	}
	if len(*payload.Findings) > 30 {
		return "", nil, ErrInvalidAnalysisOutput
	}
	findings := make([]RamificationFinding, 0, len(*payload.Findings))
	for _, item := range *payload.Findings {
		if item.Category == nil || item.Severity == nil || item.Title == nil || item.Explanation == nil || item.AffectedPaths == nil || item.RecommendedAction == nil {
			return "", nil, ErrInvalidAnalysisOutput
		}
		category, err := parseCategory(*item.Category)
		if err != nil {
			return "", nil, err
		}
		severity, err := parseSeverity(*item.Severity)
		if err != nil {
			return "", nil, err
		}
		if *item.Title == "" || utf8.RuneCountInString(*item.Title) > 200 {
			return "", nil, ErrInvalidAnalysisOutput
		}
		if *item.Explanation == "" || len(*item.Explanation) > 4000 {
			return "", nil, ErrInvalidAnalysisOutput
		}
		if *item.RecommendedAction == "" || len(*item.RecommendedAction) > 1000 {
			return "", nil, ErrInvalidAnalysisOutput
		}
		if len(*item.AffectedPaths) == 0 || len(*item.AffectedPaths) > 50 {
			return "", nil, ErrInvalidAnalysisOutput
		}
		affected := make([]ProjectPath, 0, len(*item.AffectedPaths))
		seen := make(map[ProjectPath]struct{})
		for _, rawPath := range *item.AffectedPaths {
			path, err := ValidateProjectPath(rawPath)
			if err != nil {
				return "", nil, ErrInvalidAnalysisOutput
			}
			if _, ok := allowedPaths[path]; !ok {
				return "", nil, ErrInvalidAnalysisOutput
			}
			if _, ok := seen[path]; ok {
				return "", nil, ErrInvalidAnalysisOutput
			}
			seen[path] = struct{}{}
			affected = append(affected, path)
		}
		findings = append(findings, RamificationFinding{
			Category:          category,
			Severity:          severity,
			Title:             *item.Title,
			Explanation:       *item.Explanation,
			AffectedPaths:     SortProjectPaths(affected),
			RecommendedAction: *item.RecommendedAction,
		})
	}
	return *payload.Summary, findings, nil
}

func rejectDuplicateJSONKeys(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := consumeJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return errors.New("trailing JSON")
		}
		return err
	}
	return nil
}

func consumeJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]struct{}{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("object key is not a string")
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("duplicate JSON key %q", key)
			}
			seen[key] = struct{}{}
			if err := consumeJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim('}') {
			return errors.New("unterminated JSON object")
		}
	case '[':
		for decoder.More() {
			if err := consumeJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim(']') {
			return errors.New("unterminated JSON array")
		}
	default:
		return errors.New("unexpected JSON delimiter")
	}
	return nil
}

func parseCategory(value string) (FindingCategory, error) {
	switch FindingCategory(value) {
	case CategoryPlot, CategoryCharacter, CategoryContinuity, CategoryTimeline, CategoryWorld, CategoryStructure:
		return FindingCategory(value), nil
	default:
		return "", ErrInvalidAnalysisOutput
	}
}

func parseSeverity(value string) (FindingSeverity, error) {
	switch FindingSeverity(value) {
	case SeverityLow, SeverityMedium, SeverityHigh:
		return FindingSeverity(value), nil
	default:
		return "", ErrInvalidAnalysisOutput
	}
}

// RamificationService orchestrates zero-mutation analysis.
type RamificationService struct {
	Service *Service
}

func (r *RamificationService) Run(ctx context.Context, experimentID string, request AnalysisRequest) (AnalysisResult, error) {
	goal := strings.TrimSpace(request.Goal)
	if goal == "" || len(goal) > MaxGoalBytes || !utf8.ValidString(goal) || strings.ContainsRune(goal, 0) {
		return AnalysisResult{}, ErrInvalidAnalysis
	}
	request.ProfileID = strings.TrimSpace(request.ProfileID)
	request.Model = strings.TrimSpace(request.Model)
	if !analysisProfileIDPattern.MatchString(request.ProfileID) || !utf8.ValidString(request.Model) || strings.ContainsRune(request.Model, 0) || request.Model == "" || len(request.Model) > 200 {
		return AnalysisResult{}, ErrInvalidAnalysis
	}
	id, err := ValidateExperimentID(experimentID)
	if err != nil {
		return AnalysisResult{}, err
	}
	path, err := r.Service.session.ProjectPath()
	if err != nil {
		return AnalysisResult{}, err
	}
	packet, manifest, err := r.buildAnalysisSnapshot(ctx, path, id, goal, request)
	if err != nil {
		return AnalysisResult{}, err
	}
	packet.ProfileID = request.ProfileID
	packet.Model = request.Model

	if r.Service.analyzer == nil {
		return AnalysisResult{}, ErrProviderUnavailable
	}
	result, err := r.Service.analyzer.Analyze(ctx, packet)
	if err != nil {
		return AnalysisResult{}, mapAnalyzerError(err)
	}
	result.Manifest = manifest
	return result, nil
}

func (r *RamificationService) buildAnalysisSnapshot(ctx context.Context, path string, id ExperimentID, goal string, request AnalysisRequest) (AnalysisPacket, RedactedManifest, error) {
	r.Service.coordinator.RLock()
	defer r.Service.coordinator.RUnlock()

	comparison, err := r.Service.buildComparison(ctx, path, id)
	if err != nil {
		return AnalysisPacket{}, RedactedManifest{}, err
	}
	if comparison.MainHead != request.ExpectedMainHead || comparison.ExperimentHead != request.ExpectedExperimentHead {
		return AnalysisPacket{}, RedactedManifest{}, ErrStaleRef
	}
	if err := ValidateFingerprintMatch(request.ExpectedFingerprint, comparison.Fingerprint); err != nil {
		return AnalysisPacket{}, RedactedManifest{}, err
	}
	if len(comparison.Files) > MaxAnalysisFiles {
		return AnalysisPacket{}, RedactedManifest{}, ErrAnalysisBudget
	}
	included := make([]ProjectPath, 0, len(comparison.Files))
	for _, file := range comparison.Files {
		included = append(included, file.Path)
	}
	overhead := AnalysisPromptOverhead(goal, comparison.Files)
	diffBudget := MaxAnalysisPacket - overhead
	if diffBudget <= 0 {
		return AnalysisPacket{}, RedactedManifest{}, ErrAnalysisBudget
	}
	diffText, err := r.Service.repo.UnifiedDiff(ctx, path, comparison.MainHead, comparison.ExperimentHead, included, diffBudget)
	if err != nil {
		return AnalysisPacket{}, RedactedManifest{}, mapRepositoryError(err)
	}
	return BuildAnalysisPacket(goal, comparison, diffText)
}

func mapAnalyzerError(err error) error {
	switch {
	case errors.Is(err, modelchat.ErrProviderRejected):
		return ErrProviderRejected
	case errors.Is(err, modelchat.ErrProviderOffline), errors.Is(err, modelchat.ErrProviderInvalid):
		return ErrProviderUnavailable
	default:
		return err
	}
}
