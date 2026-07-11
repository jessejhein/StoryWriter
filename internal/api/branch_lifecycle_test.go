// BDD Scenario: 8.1.1 - Create from current canon
// Requirements: M8-R01, M8-R02
// Test purpose: Branch lifecycle routes return strict JSON and status codes.

package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"storywork/internal/api"
	"storywork/internal/branch"
)

func newBranchHandler() http.Handler {
	return api.NewHandler(api.HandlerDependencies{
		Projects:  &projectStoreStub{},
		Session:   &activeProjectSessionStub{},
		Stories:   &storyServiceStub{},
		Actions:   &storyServiceStub{},
		Providers: &storyServiceStub{},
		Imports:   &storyServiceStub{},
		Branches:  branchServiceStub{},
		Version:   "branch-test",
	})
}

// Test: status and list routes return 200 JSON.
// Requirements: M8-R01.
func TestBranchStatusAndListRoutes(t *testing.T) {
	t.Parallel()
	store := lifecycleBranchStoreStub{}
	handler := api.NewHandler(api.HandlerDependencies{
		Projects:  &projectStoreStub{},
		Session:   &activeProjectSessionStub{},
		Stories:   &storyServiceStub{},
		Actions:   &storyServiceStub{},
		Providers: &storyServiceStub{},
		Imports:   &storyServiceStub{},
		Branches:  store,
		Version:   "branch-test",
	})

	t.Run("status", func(t *testing.T) {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/branches/status", nil))
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
		}
		var payload map[string]any
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if len(payload) != 6 {
			t.Fatalf("status keys=%v body=%s", payload, response.Body.String())
		}
		if payload["active_branch"] != "branch/test-exp-0123456789abcdef0123" ||
			payload["active_kind"] != "experiment" ||
			payload["main_head"] != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" ||
			payload["experiment_head"] != "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" ||
			payload["active_experiment_id"] != "brn_0123456789abcdef0123" ||
			payload["worktree_clean"] != true {
			t.Fatalf("unexpected status payload: %s", response.Body.String())
		}
		for _, forbidden := range []string{"is_canon", "is_managed_experiment", "is_detached", "is_clean", "experiment_id", "base_head"} {
			if _, ok := payload[forbidden]; ok {
				t.Fatalf("status leaked %q: %s", forbidden, response.Body.String())
			}
		}
	})

	t.Run("list", func(t *testing.T) {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/branches", nil))
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
		}
		var payload struct {
			Experiments []map[string]any `json:"experiments"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if len(payload.Experiments) != 1 {
			t.Fatalf("experiments=%d body=%s", len(payload.Experiments), response.Body.String())
		}
		experiment := payload.Experiments[0]
		if len(experiment) != 4 {
			t.Fatalf("experiment keys=%v body=%s", experiment, response.Body.String())
		}
		if experiment["experiment_id"] != "brn_0123456789abcdef0123" ||
			experiment["branch_name"] != "branch/test-exp-0123456789abcdef0123" ||
			experiment["head"] != "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" ||
			experiment["display_name"] != "test-exp" {
			t.Fatalf("unexpected experiment payload: %s", response.Body.String())
		}
		for _, forbidden := range []string{"base_head"} {
			if _, ok := experiment[forbidden]; ok {
				t.Fatalf("list leaked %q: %s", forbidden, response.Body.String())
			}
		}
	})
}

// Test: create route validates JSON and returns 201.
// Requirements: M8-R02.
func TestBranchCreateRoute(t *testing.T) {
	t.Parallel()
	handler := handlerWithBranches(lifecycleBranchStoreStub{})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/branches", strings.NewReader(`{"name":"Test Exp"}`)))
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload) != 6 {
		t.Fatalf("create keys=%v body=%s", payload, response.Body.String())
	}
	if payload["active_branch"] != "branch/test-exp-0123456789abcdef0123" ||
		payload["active_kind"] != "experiment" ||
		payload["main_head"] != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" ||
		payload["experiment_head"] != "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" ||
		payload["active_experiment_id"] != "brn_0123456789abcdef0123" ||
		payload["worktree_clean"] != true {
		t.Fatalf("unexpected create payload: %s", response.Body.String())
	}
	for _, forbidden := range []string{"is_canon", "is_managed_experiment", "is_detached", "is_clean", "base_head"} {
		if _, ok := payload[forbidden]; ok {
			t.Fatalf("create leaked %q: %s", forbidden, response.Body.String())
		}
	}
}

// Test: unsupported methods return Allow header.
// Requirements: M8-R01.
func TestBranchRoutesMethodNotAllowed(t *testing.T) {
	t.Parallel()
	handler := newBranchHandler()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/api/branches", nil))
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", response.Code)
	}
	if got := response.Header().Get("Allow"); got == "" {
		t.Fatal("Allow header missing")
	}
}

type lifecycleBranchStoreStub struct{}

func (lifecycleBranchStoreStub) Status(context.Context) (branch.RepositoryStatus, error) {
	return branch.RepositoryStatus{
		ActiveBranch:   "branch/test-exp-0123456789abcdef0123",
		IsManaged:      true,
		IsClean:        true,
		MainHead:       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ExperimentID:   "brn_0123456789abcdef0123",
		ExperimentHead: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}, nil
}

func (lifecycleBranchStoreStub) ListExperiments(context.Context) ([]branch.ExperimentRef, error) {
	return []branch.ExperimentRef{{
		ID:         "brn_0123456789abcdef0123",
		BranchName: "branch/test-exp-0123456789abcdef0123",
		Head:       "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		BaseHead:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}}, nil
}

func (lifecycleBranchStoreStub) CreateExperiment(context.Context, string) (branch.RepositoryStatus, error) {
	return branch.RepositoryStatus{
		ActiveBranch:   "branch/test-exp-0123456789abcdef0123",
		IsManaged:      true,
		IsClean:        true,
		MainHead:       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ExperimentID:   "brn_0123456789abcdef0123",
		ExperimentHead: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}, nil
}

func (lifecycleBranchStoreStub) SwitchTarget(context.Context, string, *branch.CommitID) (branch.RepositoryStatus, error) {
	return branch.RepositoryStatus{}, nil
}

func (lifecycleBranchStoreStub) LoadComparison(context.Context, string) (branch.Comparison, error) {
	return branch.Comparison{}, nil
}

func (lifecycleBranchStoreStub) LoadFileComparison(context.Context, string, string) (branch.FileComparison, error) {
	return branch.FileComparison{}, nil
}

func (lifecycleBranchStoreStub) AnalyzeRamifications(context.Context, string, branch.AnalysisRequest) (branch.AnalysisResult, error) {
	return branch.AnalysisResult{}, nil
}

func (lifecycleBranchStoreStub) PromoteSelectedFiles(context.Context, branch.PromotionRequest) (branch.PromotionResult, error) {
	return branch.PromotionResult{}, nil
}

func (lifecycleBranchStoreStub) DiscardExperiment(context.Context, string, branch.CommitID) (branch.RepositoryStatus, error) {
	return branch.RepositoryStatus{}, nil
}
