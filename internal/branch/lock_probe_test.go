// BDD Scenario: 8.3.1 - Lock released before provider execution
// Requirements: M8-R09
// Test purpose: The coordinator read lock is released before the provider call.

package branch_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"storywork/internal/branch"
)

type probeCoordinator struct {
	mu       sync.RWMutex
	rlocked  bool
	probe    chan struct{}
	probeErr error
}

func (c *probeCoordinator) Lock()    { c.mu.Lock() }
func (c *probeCoordinator) Unlock()  { c.mu.Unlock() }
func (c *probeCoordinator) RLock()   { c.mu.RLock(); c.rlocked = true }
func (c *probeCoordinator) RUnlock() { c.mu.RUnlock() }

type lockProbeAnalyzer struct {
	probe func() bool
}

func (a *lockProbeAnalyzer) Analyze(_ context.Context, _ branch.AnalysisPacket) (branch.AnalysisResult, error) {
	if a.probe != nil && !a.probe() {
		return branch.AnalysisResult{}, errors.New("read lock still held during provider call")
	}
	return branch.AnalysisResult{Summary: "ok"}, nil
}

// Test: the coordinator read lock is released before the provider executes on
// success.
func TestAnalyzeReleasesReadLockBeforeProviderCall(t *testing.T) {
	t.Parallel()
	mainHead := branch.CommitID("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	experimentHead := branch.CommitID("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	files := []branch.ChangedFile{{Path: "outline.yaml", Status: branch.StatusModified}}
	fingerprint, err := branch.ComputeFingerprint(mainHead, experimentHead, "cccccccccccccccccccccccccccccccccccccccc", files)
	if err != nil {
		t.Fatal(err)
	}
	coord := &probeCoordinator{}
	repo := &analysisRepo{
		fakeRepo: &fakeRepo{
			experiments:  []branch.ExperimentRef{{ID: "brn_0123456789abcdef0123", BranchName: "branch/test-exp-0123456789abcdef0123", Head: experimentHead}},
			mainHead:     mainHead,
			compareFiles: files,
		},
		diffText: "--- a/outline.yaml\n+++ b/outline.yaml\n@@ -1,1 +1,1 @@\n-old\n+new\n",
	}
	analyzer := &lockProbeAnalyzer{
		probe: func() bool {
			locked := make(chan struct{})
			go func() {
				coord.mu.Lock()
				close(locked)
			}()
			select {
			case <-locked:
				coord.mu.Unlock()
				return true
			case <-time.After(100 * time.Millisecond):
				return false
			}
		},
	}
	service := branch.NewService(repo, &fakeIndex{}, coord, branch.SessionAdapter{PathFn: func() (string, bool) { return "/tmp/project", true }}, nil, analyzer, &staticIDs{id: "brn_0123456789abcdef0123"})
	_, err = service.AnalyzeRamifications(context.Background(), "brn_0123456789abcdef0123", branch.AnalysisRequest{
		Goal: "Review", ProfileID: "local", Model: "model",
		ExpectedMainHead: mainHead, ExpectedExperimentHead: experimentHead, ExpectedFingerprint: fingerprint,
	})
	if err != nil {
		t.Fatalf("AnalyzeRamifications() error = %v", err)
	}
}
