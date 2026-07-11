// BDD Scenario: 8.1.1 - Create from current canon
// Requirements: M8-R01
// Test purpose: HTTP handler accepts cohesive BranchStore dependency.

package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"storywork/internal/api"
	"storywork/internal/branch"
)

type branchServiceStub struct{}

func (branchServiceStub) Status(context.Context) (branch.RepositoryStatus, error) {
	return branch.RepositoryStatus{ActiveBranch: "main", IsCanon: true, IsClean: true}, nil
}
func (branchServiceStub) ListExperiments(context.Context) ([]branch.ExperimentRef, error) {
	return []branch.ExperimentRef{}, nil
}
func (branchServiceStub) CreateExperiment(context.Context, string) (branch.RepositoryStatus, error) {
	return branch.RepositoryStatus{}, nil
}
func (branchServiceStub) SwitchTarget(context.Context, string, *branch.CommitID) (branch.RepositoryStatus, error) {
	return branch.RepositoryStatus{}, nil
}
func (branchServiceStub) LoadComparison(context.Context, string) (branch.Comparison, error) {
	return branch.Comparison{}, nil
}
func (branchServiceStub) LoadFileComparison(context.Context, string, string) (branch.FileComparison, error) {
	return branch.FileComparison{}, nil
}
func (branchServiceStub) AnalyzeRamifications(context.Context, string, branch.AnalysisRequest) (branch.AnalysisResult, error) {
	return branch.AnalysisResult{}, nil
}
func (branchServiceStub) PromoteSelectedFiles(context.Context, branch.PromotionRequest) (branch.PromotionResult, error) {
	return branch.PromotionResult{MainHead: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", PromotedPaths: []branch.ProjectPath{}, ExperimentID: "brn_0123456789abcdef0123"}, nil
}
func (branchServiceStub) DiscardExperiment(context.Context, string, branch.CommitID) (branch.RepositoryStatus, error) {
	return branch.RepositoryStatus{}, nil
}

// Test: HandlerDependencies accepts BranchStore without widening other stores.
// Requirements: M8-R01.
func TestHandlerDependenciesAcceptBranchStore(t *testing.T) {
	t.Parallel()
	handler := api.NewHandler(api.HandlerDependencies{
		Projects:  &projectStoreStub{},
		Session:   &activeProjectSessionStub{},
		Stories:   &storyServiceStub{},
		Actions:   &storyServiceStub{},
		Providers: &storyServiceStub{},
		Imports:   &storyServiceStub{},
		Branches:  branchServiceStub{},
		Version:   "branch-deps",
	})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/branches/status", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}
