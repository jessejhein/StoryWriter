package branch

import (
	"errors"
	"fmt"
	"regexp"
)

const (
	CanonBranchName     = "main"
	ExperimentNamespace = "branch/"
	MaxChangedPaths     = 500
	MaxFileBytes        = 5 << 20 // 5 MiB
	MaxFileLines        = 200_000
	MaxAnalysisFiles    = 100
	MaxAnalysisPacket   = 512 << 10 // 512 KiB
	MaxGoalBytes        = 2000
)

var commitIDPattern = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
var fingerprintPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// ChangedStatus is a deterministic comparison status for one project path.
type ChangedStatus string

const (
	StatusAdded    ChangedStatus = "added"
	StatusModified ChangedStatus = "modified"
	StatusDeleted  ChangedStatus = "deleted"
)

// ExperimentID is a managed experiment identifier: brn_ plus 20 lowercase hex.
type ExperimentID string

// BranchRef is a validated Git branch ref under branch/.
type BranchRef string

// CommitID is a full lowercase Git object id.
type CommitID string

// ProjectPath is a validated project-relative canonical text path.
type ProjectPath string

// PathSnapshot records one path's presence at a source commit for rollback.
type PathSnapshot struct {
	Path         ProjectPath
	Exists       bool
	SourceCommit CommitID
}

// ExperimentRef records one managed experiment branch.
type ExperimentRef struct {
	ID         ExperimentID `json:"experiment_id"`
	BranchName BranchRef    `json:"branch_name"`
	Head       CommitID     `json:"head"`
}

// ChangedFile is one changed path in a tree comparison.
type ChangedFile struct {
	Path   ProjectPath   `json:"path"`
	Status ChangedStatus `json:"status"`
}

// RepositoryStatus reports the active branch and heads without absolute paths.
type RepositoryStatus struct {
	ActiveBranch   string       `json:"active_branch"`
	IsCanon        bool         `json:"is_canon"`
	IsManaged      bool         `json:"is_managed_experiment"`
	IsDetached     bool         `json:"is_detached"`
	IsClean        bool         `json:"is_clean"`
	MainHead       CommitID     `json:"main_head"`
	ExperimentID   ExperimentID `json:"experiment_id,omitempty"`
	ExperimentHead CommitID     `json:"experiment_head,omitempty"`
}

// Comparison is a deterministic main-vs-experiment inventory.
type Comparison struct {
	ExperimentID   ExperimentID  `json:"experiment_id"`
	BranchName     BranchRef     `json:"branch_name"`
	MainHead       CommitID      `json:"main_head"`
	ExperimentHead CommitID      `json:"experiment_head"`
	BaseHead       CommitID      `json:"base_head"`
	Fingerprint    string        `json:"fingerprint"`
	Files          []ChangedFile `json:"files"`
}

// TextSide is one bounded UTF-8 text side in a file comparison.
type TextSide struct {
	Exists bool   `json:"exists"`
	Text   string `json:"text"`
}

// FileComparison is bounded side-by-side content for one changed path.
type FileComparison struct {
	Path           ProjectPath   `json:"path"`
	Status         ChangedStatus `json:"status"`
	MainHead       CommitID      `json:"main_head"`
	ExperimentHead CommitID      `json:"experiment_head"`
	Fingerprint    string        `json:"fingerprint"`
	Canon          TextSide      `json:"canon"`
	Experiment     TextSide      `json:"experiment"`
}

// PromotionRequest selects whole files for conservative promotion.
type PromotionRequest struct {
	ExperimentID           ExperimentID
	Paths                  []ProjectPath
	ExpectedMainHead       CommitID
	ExpectedExperimentHead CommitID
	ExpectedFingerprint    string
}

// PromotionResult reports the new main head after promotion.
type PromotionResult struct {
	MainHead      CommitID      `json:"main_head"`
	PromotedPaths []ProjectPath `json:"promoted_paths"`
	ExperimentID  ExperimentID  `json:"experiment_id"`
}

// PromotionCommit is validated provenance for one promotion checkpoint.
type PromotionCommit struct {
	ExperimentID ExperimentID
	SourceCommit CommitID
	BaseCommit   CommitID
	Paths        []ProjectPath
}

// AnalysisRequest is an explicit ramification analysis call.
type AnalysisRequest struct {
	Goal                   string
	ProfileID              string
	Model                  string
	ExpectedMainHead       CommitID
	ExpectedExperimentHead CommitID
	ExpectedFingerprint    string
}

// FindingCategory is a strict ramification finding category.
type FindingCategory string

const (
	CategoryPlot       FindingCategory = "plot"
	CategoryCharacter  FindingCategory = "character"
	CategoryContinuity FindingCategory = "continuity"
	CategoryTimeline   FindingCategory = "timeline"
	CategoryWorld      FindingCategory = "world"
	CategoryStructure  FindingCategory = "structure"
)

// FindingSeverity is a strict ramification severity.
type FindingSeverity string

const (
	SeverityLow    FindingSeverity = "low"
	SeverityMedium FindingSeverity = "medium"
	SeverityHigh   FindingSeverity = "high"
)

// RamificationFinding is one advisory structured consequence.
type RamificationFinding struct {
	Category          FindingCategory `json:"category"`
	Severity          FindingSeverity `json:"severity"`
	Title             string          `json:"title"`
	Explanation       string          `json:"explanation"`
	AffectedPaths     []ProjectPath   `json:"affected_paths"`
	RecommendedAction string          `json:"recommended_action"`
}

// RedactedManifest summarizes bounded analysis input without prompts.
type RedactedManifest struct {
	MainHead         CommitID      `json:"main_head"`
	ExperimentHead   CommitID      `json:"experiment_head"`
	Fingerprint      string        `json:"fingerprint"`
	ChangedFileCount int           `json:"changed_file_count"`
	IncludedPaths    []ProjectPath `json:"included_paths"`
	EstimatedBytes   int           `json:"estimated_input_bytes"`
}

// AnalysisResult is strict transient ramification output.
type AnalysisResult struct {
	Summary  string                `json:"summary"`
	Findings []RamificationFinding `json:"findings"`
	Provider ProviderIdentity      `json:"provider"`
	Manifest RedactedManifest      `json:"manifest"`
}

// ProviderIdentity records which provider produced analysis.
type ProviderIdentity struct {
	ProfileID string `json:"profile_id"`
	Type      string `json:"type"`
	Model     string `json:"model"`
}

var (
	ErrInvalidExperimentID   = errors.New("invalid experiment id")
	ErrInvalidBranchRef      = errors.New("invalid branch ref")
	ErrInvalidCommitID       = errors.New("invalid commit id")
	ErrInvalidProjectPath    = errors.New("invalid project path")
	ErrInvalidExperimentName = errors.New("invalid experiment name")
	ErrInvalidChangedStatus  = errors.New("invalid changed status")
	ErrTooManyChangedPaths   = errors.New("too many changed paths")
	ErrFileTooLarge          = errors.New("file exceeds comparison limit")
	ErrInvalidUTF8           = errors.New("content is not strict UTF-8")
	ErrPathNotInComparison   = errors.New("path is not in comparison")
	ErrDirtyWorktree         = errors.New("worktree is not clean")
	ErrNoActiveProject       = errors.New("no active project")
	ErrStaleRef              = errors.New("stale ref")
	ErrStaleFingerprint      = errors.New("stale comparison fingerprint")
	ErrInvalidFingerprint    = errors.New("invalid comparison fingerprint")
	ErrDetachedHEAD          = errors.New("detached HEAD")
	ErrUnmanagedBranch       = errors.New("unmanaged active branch")
	ErrMainMissing           = errors.New("main branch is missing")
	ErrExperimentNotFound    = errors.New("experiment not found")
	ErrPromotionConflict     = errors.New("promotion path conflict")
	ErrInvalidPromotion      = errors.New("invalid promotion request")
	ErrInvalidAnalysis       = errors.New("invalid analysis request")
	ErrAnalysisBudget        = errors.New("analysis packet exceeds budget")
	ErrInvalidAnalysisOutput = errors.New("invalid analysis output")
	ErrProviderRejected      = errors.New("provider rejected")
	ErrProviderUnavailable   = errors.New("provider unavailable")
	ErrRepositoryState       = errors.New("invalid repository state")
)

// ValidateCommitID requires a full lowercase object id.
func ValidateCommitID(value string) (CommitID, error) {
	if !commitIDPattern.MatchString(value) {
		return "", fmt.Errorf("commit id %q: %w", value, ErrInvalidCommitID)
	}
	return CommitID(value), nil
}

// ValidateFingerprint requires sha256: plus 64 lowercase hexadecimal digits.
func ValidateFingerprint(value string) error {
	if !fingerprintPattern.MatchString(value) {
		return fmt.Errorf("fingerprint is malformed: %w", ErrInvalidFingerprint)
	}
	return nil
}

// ParseChangedStatus maps Git name-status codes to product statuses.
func ParseChangedStatus(code byte) (ChangedStatus, error) {
	switch code {
	case 'A':
		return StatusAdded, nil
	case 'M':
		return StatusModified, nil
	case 'D':
		return StatusDeleted, nil
	default:
		return "", fmt.Errorf("status code %q: %w", string(code), ErrInvalidChangedStatus)
	}
}
