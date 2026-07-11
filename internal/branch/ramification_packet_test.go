// BDD Scenario: 8.3.1 - Exact analysis packet budget
// Requirements: M8-R09, M8-R20
// Test purpose: BuildAnalysisPacket EstimatedBytes equals the rendered prompt,
// and the 512 KiB boundary is exact.

package branch

import (
	"errors"
	"strings"
	"testing"
)

// Test: manifest EstimatedBytes equals len(rendered prompt) including labels.
func TestBuildAnalysisPacketEstimatedBytesMatchesRenderedPrompt(t *testing.T) {
	t.Parallel()
	goal := "Review the plot continuity"
	files := []ChangedFile{
		{Path: "scenes/scn_0123456789abcdef0123.md", Status: StatusModified},
		{Path: "outline.yaml", Status: StatusModified},
	}
	diffText := "--- a/outline.yaml\n+++ b/outline.yaml\n@@ -1,1 +1,1 @@\n-old\n+new\n"
	comparison := Comparison{
		ExperimentID:   "brn_0123456789abcdef0123",
		BranchName:     "branch/test-exp-0123456789abcdef0123",
		MainHead:       CommitID(strings.Repeat("a", 40)),
		ExperimentHead: CommitID(strings.Repeat("b", 40)),
		BaseHead:       CommitID(strings.Repeat("c", 40)),
		Fingerprint:    "sha256:" + strings.Repeat("d", 64),
		Files:          files,
	}
	packet, manifest, err := BuildAnalysisPacket(goal, comparison, diffText)
	if err != nil {
		t.Fatalf("BuildAnalysisPacket() error = %v", err)
	}
	rendered := buildRamificationPrompt(packet)
	if manifest.EstimatedBytes != len(rendered) {
		t.Fatalf("EstimatedBytes = %d, want %d (len of rendered prompt)", manifest.EstimatedBytes, len(rendered))
	}
}

// Test: exactly MaxAnalysisPacket succeeds, +1 fails.
func TestBuildAnalysisPacketExactBoundary(t *testing.T) {
	t.Parallel()
	files := []ChangedFile{
		{Path: "outline.yaml", Status: StatusModified},
	}
	comparison := Comparison{
		ExperimentID:   "brn_0123456789abcdef0123",
		BranchName:     "branch/test-exp-0123456789abcdef0123",
		MainHead:       CommitID(strings.Repeat("a", 40)),
		ExperimentHead: CommitID(strings.Repeat("b", 40)),
		BaseHead:       CommitID(strings.Repeat("c", 40)),
		Fingerprint:    "sha256:" + strings.Repeat("d", 64),
		Files:          files,
	}

	overhead := AnalysisPromptOverhead("g", files)

	// Exactly MaxAnalysisPacket should succeed.
	exactGoal := strings.Repeat("g", 1)
	exactDiff := strings.Repeat("x", MaxAnalysisPacket-overhead)
	_, _, err := BuildAnalysisPacket(exactGoal, comparison, exactDiff)
	if err != nil {
		t.Fatalf("BuildAnalysisPacket(exact) error = %v", err)
	}

	// One byte over should fail.
	overDiff := strings.Repeat("x", MaxAnalysisPacket-overhead+1)
	_, _, err = BuildAnalysisPacket(exactGoal, comparison, overDiff)
	if err == nil {
		t.Fatal("BuildAnalysisPacket(over) = nil, want error")
	}
}

// Test: invalid goals are rejected before packet construction.
func TestBuildAnalysisPacketRejectsInvalidGoals(t *testing.T) {
	t.Parallel()
	comparison := Comparison{
		ExperimentID:   "brn_0123456789abcdef0123",
		BranchName:     "branch/test-exp-0123456789abcdef0123",
		MainHead:       CommitID(strings.Repeat("a", 40)),
		ExperimentHead: CommitID(strings.Repeat("b", 40)),
		BaseHead:       CommitID(strings.Repeat("c", 40)),
		Fingerprint:    "sha256:" + strings.Repeat("d", 64),
		Files:          []ChangedFile{{Path: "outline.yaml", Status: StatusModified}},
	}
	for _, goal := range []string{
		"",
		"   ",
		strings.Repeat("x", MaxGoalBytes+1),
		string([]byte{0xff, 0xfe}),
		"review\x00goal",
	} {
		if _, _, err := BuildAnalysisPacket(goal, comparison, "diff"); err == nil {
			t.Fatalf("BuildAnalysisPacket(%q) = nil, want error", goal)
		}
	}
}

// Test: malformed comparison metadata is rejected before packet construction.
func TestBuildAnalysisPacketRejectsInvalidComparisonMetadata(t *testing.T) {
	t.Parallel()
	valid := Comparison{
		ExperimentID:   "brn_0123456789abcdef0123",
		BranchName:     "branch/test-exp-0123456789abcdef0123",
		MainHead:       CommitID(strings.Repeat("a", 40)),
		ExperimentHead: CommitID(strings.Repeat("b", 40)),
		BaseHead:       CommitID(strings.Repeat("c", 40)),
		Fingerprint:    "sha256:" + strings.Repeat("d", 64),
		Files:          []ChangedFile{{Path: "outline.yaml", Status: StatusModified}},
	}
	tests := []struct {
		name       string
		comparison Comparison
		wantErr    error
	}{
		{name: "bad experiment id", comparison: func() Comparison { next := valid; next.ExperimentID = "bad"; return next }(), wantErr: ErrInvalidExperimentID},
		{name: "bad branch ref", comparison: func() Comparison { next := valid; next.BranchName = "branch/main-0123456789abcdef0123"; return next }(), wantErr: ErrInvalidBranchRef},
		{name: "bad main head", comparison: func() Comparison { next := valid; next.MainHead = "bad"; return next }(), wantErr: ErrInvalidCommitID},
		{name: "bad experiment head", comparison: func() Comparison { next := valid; next.ExperimentHead = "bad"; return next }(), wantErr: ErrInvalidCommitID},
		{name: "bad base head", comparison: func() Comparison { next := valid; next.BaseHead = "bad"; return next }(), wantErr: ErrInvalidCommitID},
		{name: "bad fingerprint", comparison: func() Comparison { next := valid; next.Fingerprint = "sha256:bad"; return next }(), wantErr: ErrInvalidFingerprint},
		{name: "bad path", comparison: func() Comparison {
			next := valid
			next.Files = []ChangedFile{{Path: "/abs", Status: StatusModified}}
			return next
		}(), wantErr: ErrInvalidProjectPath},
		{name: "bad status", comparison: func() Comparison {
			next := valid
			next.Files = []ChangedFile{{Path: "outline.yaml", Status: ChangedStatus("renamed")}}
			return next
		}(), wantErr: ErrInvalidChangedStatus},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			_, _, err := BuildAnalysisPacket("goal", testCase.comparison, "diff")
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("BuildAnalysisPacket() err = %v, want errors.Is(%v)", err, testCase.wantErr)
			}
		})
	}
}

// Test: changed files are normalized to deterministic byte order.
func TestBuildAnalysisPacketSortsChangedFilesDeterministically(t *testing.T) {
	t.Parallel()
	comparison := Comparison{
		ExperimentID:   "brn_0123456789abcdef0123",
		BranchName:     "branch/test-exp-0123456789abcdef0123",
		MainHead:       CommitID(strings.Repeat("a", 40)),
		ExperimentHead: CommitID(strings.Repeat("b", 40)),
		BaseHead:       CommitID(strings.Repeat("c", 40)),
		Fingerprint:    "sha256:" + strings.Repeat("d", 64),
		Files: []ChangedFile{
			{Path: "scenes/scn_z.md", Status: StatusAdded},
			{Path: "outline.yaml", Status: StatusModified},
			{Path: "scenes/scn_a.md", Status: StatusDeleted},
		},
	}
	packet, manifest, err := BuildAnalysisPacket("goal", comparison, "diff")
	if err != nil {
		t.Fatalf("BuildAnalysisPacket() error = %v", err)
	}
	if got, want := packet.Comparison.Files[0].Path, ProjectPath("outline.yaml"); got != want {
		t.Fatalf("first sorted path = %q, want %q", got, want)
	}
	if got := manifest.IncludedPaths; len(got) != 3 || got[0] != "outline.yaml" || got[1] != "scenes/scn_a.md" || got[2] != "scenes/scn_z.md" {
		t.Fatalf("IncludedPaths = %#v", got)
	}
}

// Test: malformed diff text is rejected before rendering.
func TestBuildAnalysisPacketRejectsInvalidDiffText(t *testing.T) {
	t.Parallel()
	comparison := Comparison{
		ExperimentID:   "brn_0123456789abcdef0123",
		BranchName:     "branch/test-exp-0123456789abcdef0123",
		MainHead:       CommitID(strings.Repeat("a", 40)),
		ExperimentHead: CommitID(strings.Repeat("b", 40)),
		BaseHead:       CommitID(strings.Repeat("c", 40)),
		Fingerprint:    "sha256:" + strings.Repeat("d", 64),
		Files:          []ChangedFile{{Path: "outline.yaml", Status: StatusModified}},
	}
	for _, diffText := range []string{
		string([]byte{0xff, 0xfe}),
		"diff\x00text",
	} {
		_, _, err := BuildAnalysisPacket("goal", comparison, diffText)
		if !errors.Is(err, ErrInvalidAnalysis) {
			t.Fatalf("BuildAnalysisPacket(%q) err = %v, want ErrInvalidAnalysis", diffText, err)
		}
	}
}

// Test: analysisPromptOverhead matches the labels in buildRamificationPrompt.
func TestAnalysisPromptOverheadMatchesLabels(t *testing.T) {
	t.Parallel()
	goal := "test goal"
	files := []ChangedFile{
		{Path: "scenes/scn_001.md", Status: StatusAdded},
		{Path: "outline.yaml", Status: StatusModified},
	}
	overhead := AnalysisPromptOverhead(goal, files)
	// Construct what buildRamificationPrompt would produce without the diff.
	var builder strings.Builder
	builder.WriteString(promptLabelGoal)
	builder.WriteString(goal)
	builder.WriteString(promptLabelChangedFiles)
	for _, file := range files {
		builder.WriteString(string(file.Status))
		builder.WriteByte(' ')
		builder.WriteString(string(file.Path))
		builder.WriteByte('\n')
	}
	builder.WriteString(promptLabelDiff)
	expected := builder.Len()
	if overhead != expected {
		t.Fatalf("analysisPromptOverhead = %d, want %d", overhead, expected)
	}
}
