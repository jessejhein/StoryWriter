// BDD Scenario: 8.1.2 - Reject unsafe branch state
// Requirements: M8-R01, M8-R03, M8-R06, M8-R07, M8-R09, M8-R12, M8-R17
// Test purpose: Every branch route rejects malformed calls with contract status
// codes before domain side effects and returns only safe JSON errors.

package api_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"storywork/internal/api"
	"storywork/internal/branch"
	"storywork/internal/gitstore"
	"storywork/internal/mutation"
)

type statusErrorBranchStore struct {
	branchServiceStub
	err error
}

type listErrorBranchStore struct {
	branchServiceStub
	err error
}

type promotionErrorBranchStore struct {
	branchServiceStub
	err error
}

type fileComparisonErrorBranchStore struct {
	branchServiceStub
	err error
}

type switchErrorBranchStore struct {
	branchServiceStub
	err error
}

type comparisonErrorBranchStore struct {
	branchServiceStub
	err error
}

type noopBranchIndex struct{}

func (noopBranchIndex) Rebuild(context.Context, string) error { return nil }

type staticBranchIDs struct {
	id branch.ExperimentID
}

func (s staticBranchIDs) NextExperimentID() (branch.ExperimentID, error) { return s.id, nil }

type spyBranchStore struct {
	branchServiceStub
	analysisCalls       int
	createCalls         int
	switchCalls         int
	fileComparisonCalls int
	promoteCalls        int
	discardCalls        int
}

func (s statusErrorBranchStore) Status(context.Context) (branch.RepositoryStatus, error) {
	return branch.RepositoryStatus{}, s.err
}

func (s listErrorBranchStore) ListExperiments(context.Context) ([]branch.ExperimentRef, error) {
	return nil, s.err
}

func handlerWithBranches(store api.BranchStore) http.Handler {
	return api.NewHandler(api.HandlerDependencies{Projects: &projectStoreStub{}, Session: &activeProjectSessionStub{}, Stories: &storyServiceStub{}, Actions: &storyServiceStub{}, Providers: &storyServiceStub{}, Imports: &storyServiceStub{}, Branches: store, Version: "test"})
}

func (s *spyBranchStore) AnalyzeRamifications(context.Context, string, branch.AnalysisRequest) (branch.AnalysisResult, error) {
	s.analysisCalls++
	return branch.AnalysisResult{}, nil
}

func (s *spyBranchStore) CreateExperiment(context.Context, string) (branch.RepositoryStatus, error) {
	s.createCalls++
	return branch.RepositoryStatus{}, nil
}

func (s *spyBranchStore) SwitchTarget(context.Context, string, *branch.CommitID) (branch.RepositoryStatus, error) {
	s.switchCalls++
	return branch.RepositoryStatus{}, nil
}

func (s *spyBranchStore) LoadFileComparison(context.Context, string, string) (branch.FileComparison, error) {
	s.fileComparisonCalls++
	return branch.FileComparison{}, nil
}

func (s *spyBranchStore) PromoteSelectedFiles(context.Context, branch.PromotionRequest) (branch.PromotionResult, error) {
	s.promoteCalls++
	return branch.PromotionResult{}, nil
}

func (s *spyBranchStore) DiscardExperiment(context.Context, string, branch.CommitID) (branch.RepositoryStatus, error) {
	s.discardCalls++
	return branch.RepositoryStatus{}, nil
}

func (s promotionErrorBranchStore) PromoteSelectedFiles(context.Context, branch.PromotionRequest) (branch.PromotionResult, error) {
	return branch.PromotionResult{}, s.err
}

func (s fileComparisonErrorBranchStore) LoadFileComparison(context.Context, string, string) (branch.FileComparison, error) {
	return branch.FileComparison{}, s.err
}

func (s switchErrorBranchStore) SwitchTarget(context.Context, string, *branch.CommitID) (branch.RepositoryStatus, error) {
	return branch.RepositoryStatus{}, s.err
}

func (s comparisonErrorBranchStore) LoadComparison(context.Context, string) (branch.Comparison, error) {
	return branch.Comparison{}, s.err
}

// Test: typed domain errors map to the exact public status classes.
// Requirements: M8-R03, M8-R07, M8-R09, M8-R12, M8-R17.
func TestBranchRouteMapsEveryContractErrorClass(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "invalid", err: branch.ErrInvalidProjectPath, status: http.StatusBadRequest},
		{name: "missing main", err: branch.ErrMainMissing, status: http.StatusBadRequest},
		{name: "conflict", err: branch.ErrStaleRef, status: http.StatusConflict},
		{name: "not found", err: branch.ErrExperimentNotFound, status: http.StatusNotFound},
		{name: "too large", err: branch.ErrFileTooLarge, status: http.StatusRequestEntityTooLarge},
		{name: "provider output", err: branch.ErrInvalidAnalysisOutput, status: http.StatusBadGateway},
		{name: "provider unavailable", err: branch.ErrProviderUnavailable, status: http.StatusServiceUnavailable},
		{name: "promotion subset", err: branch.ErrInvalidPromotionSubset, status: http.StatusConflict},
		{name: "internal", err: errors.New("/secret/project raw git command"), status: http.StatusInternalServerError},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			handlerWithBranches(statusErrorBranchStore{err: testCase.err}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/branches/status", nil))
			if response.Code != testCase.status {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if strings.Contains(response.Body.String(), "/secret/") || strings.Contains(response.Body.String(), "git command") {
				t.Fatalf("unsafe body=%s", response.Body.String())
			}
		})
	}
}

// Test: promotion subset validation failures return 409 while infrastructure
// failures remain 500 with safe error bodies.
// Requirements: M8-R14, M8-R15.
func TestBranchPromotionRouteMapsSubsetValidationAndInfrastructureFailures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		store  api.BranchStore
		status int
	}{
		{name: "invalid subset", store: promotionErrorBranchStore{err: branch.ErrInvalidPromotionSubset}, status: http.StatusConflict},
		{name: "infrastructure", store: promotionErrorBranchStore{err: branch.ErrRepositoryState}, status: http.StatusInternalServerError},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			body := `{"paths":["outline.yaml"],"expected_main_head":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","expected_experiment_head":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","comparison_fingerprint":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}`
			response := httptest.NewRecorder()
			handlerWithBranches(testCase.store).ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/branches/brn_0123456789abcdef0123/promote", strings.NewReader(body)))
			if response.Code != testCase.status {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if strings.Contains(response.Body.String(), "/") || strings.Contains(response.Body.String(), "git") {
				t.Fatalf("unsafe body=%s", response.Body.String())
			}
		})
	}
}

// Test: oversized comparison file blobs return 413 on the file comparison
// route with a safe body and no partial content fields.
// Requirements: M8-R07.
func TestBranchFileComparisonRouteMapsOversizedBlobTo413(t *testing.T) {
	t.Parallel()
	response := httptest.NewRecorder()
	handlerWithBranches(fileComparisonErrorBranchStore{err: branch.ErrFileTooLarge}).ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, "/api/branches/brn_0123456789abcdef0123/comparison/file?path=outline.yaml", nil),
	)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	if strings.Contains(body, "/") || strings.Contains(body, "git") || strings.Contains(body, "\"canon\"") || strings.Contains(body, "\"experiment\"") {
		t.Fatalf("unsafe body=%s", body)
	}
}

// Test: malformed comparison state maps to a safe 500 on the comparison route.
// Requirements: M8-R06, M8-R07, M8-R20.
func TestBranchComparisonRouteMapsMalformedRepositoryStateToSafe500(t *testing.T) {
	t.Parallel()
	response := httptest.NewRecorder()
	handlerWithBranches(comparisonErrorBranchStore{err: errors.Join(branch.ErrRepositoryState, errors.New("/tmp/project invalid comparison row"))}).ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, "/api/branches/brn_0123456789abcdef0123/comparison", nil),
	)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	if strings.Contains(body, "/tmp/project") || strings.Contains(body, "comparison row") {
		t.Fatalf("unsafe body=%s", body)
	}
}

// Test: detached HEAD switch failures map to 409 with a sanitized body.
// Requirements: M8-R03.
func TestBranchSwitchRouteMapsDetachedHEADTo409(t *testing.T) {
	t.Parallel()
	response := httptest.NewRecorder()
	handlerWithBranches(switchErrorBranchStore{err: branch.ErrDetachedHEAD}).ServeHTTP(
		response,
		httptest.NewRequest(http.MethodPost, "/api/branches/switch", strings.NewReader(`{"target":"main"}`)),
	)
	if response.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	if strings.Contains(body, "/") || strings.Contains(body, "git") {
		t.Fatalf("unsafe body=%s", body)
	}
}

// Test: malformed repository state returns a safe 500 for status and list
// routes without surfacing raw diagnostics.
// Requirements: M8-R01, M8-R06, M8-R20.
func TestBranchStatusAndListRoutesMapRepositoryStateErrorsSafely(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		path  string
		store api.BranchStore
	}{
		{name: "status", path: "/api/branches/status", store: statusErrorBranchStore{err: branch.ErrRepositoryState}},
		{name: "list", path: "/api/branches", store: listErrorBranchStore{err: branch.ErrRepositoryState}},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			handlerWithBranches(testCase.store).ServeHTTP(response, httptest.NewRequest(http.MethodGet, testCase.path, nil))
			if response.Code != http.StatusInternalServerError {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if strings.Contains(response.Body.String(), "/") || strings.Contains(response.Body.String(), "git") {
				t.Fatalf("unsafe body=%s", response.Body.String())
			}
		})
	}
}

// Test: a malformed managed ref in a real repository maps to safe 500s on the
// public status and list routes.
// Requirements: M8-R01, M8-R06, M8-R20.
func TestBranchStatusAndListRoutesRejectMalformedManagedRefFromRealRepository(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "outline.yaml"), []byte("version: 1\nroot:\n  arcs: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := gitstore.New("git")
	if err := store.Init(ctx, dir); err != nil {
		t.Fatal(err)
	}
	if err := store.CommitAll(ctx, dir, "init"); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.CommandContext(ctx, "git", "-C", dir, "branch", "branch/main-0123456789abcdef0123", "main").CombinedOutput(); err != nil {
		t.Fatalf("git branch: %v: %s", err, output)
	}
	if output, err := exec.CommandContext(ctx, "git", "-C", dir, "checkout", "branch/main-0123456789abcdef0123").CombinedOutput(); err != nil {
		t.Fatalf("git checkout: %v: %s", err, output)
	}
	service := branch.NewService(&branch.GitRepository{Store: store}, noopBranchIndex{}, mutation.NewCoordinator(), branch.SessionAdapter{PathFn: func() (string, bool) { return dir, true }}, nil, nil, staticBranchIDs{id: "brn_0123456789abcdef0123"})
	handler := handlerWithBranches(service)
	for _, path := range []string{"/api/branches/status", "/api/branches"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusInternalServerError {
			t.Fatalf("%s status=%d body=%s", path, response.Code, response.Body.String())
		}
		if strings.Contains(response.Body.String(), dir) || strings.Contains(response.Body.String(), "git checkout") {
			t.Fatalf("unsafe body=%s", response.Body.String())
		}
	}
}

// Test: strict bodies, tagged switch variants, exact queries, IDs, fingerprints,
// and size limits reject bad API calls.
// Requirements: M8-R01, M8-R06, M8-R07, M8-R09, M8-R12, M8-R17.
func TestBranchRoutesRejectMalformedCallsBeforeService(t *testing.T) {
	t.Parallel()
	handler := newBranchHandler()
	tests := []struct {
		name, method, path, body string
		status                   int
	}{
		{name: "create missing", method: http.MethodPost, path: "/api/branches", body: `{}`, status: 400},
		{name: "create null", method: http.MethodPost, path: "/api/branches", body: `{"name":null}`, status: 400},
		{name: "create unknown", method: http.MethodPost, path: "/api/branches", body: `{"name":"x","extra":true}`, status: 400},
		{name: "create reserved", method: http.MethodPost, path: "/api/branches", body: `{"name":"main"}`, status: 400},
		{name: "create trailing", method: http.MethodPost, path: "/api/branches", body: `{"name":"x"}{}`, status: 400},
		{name: "create duplicate key", method: http.MethodPost, path: "/api/branches", body: `{"name":"x","name":"y"}`, status: 400},
		{name: "main expected head forbidden", method: http.MethodPost, path: "/api/branches/switch", body: `{"target":"main","expected_head":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`, status: 400},
		{name: "experiment expected head required", method: http.MethodPost, path: "/api/branches/switch", body: `{"target":"brn_0123456789abcdef0123"}`, status: 400},
		{name: "malformed route id", method: http.MethodPost, path: "/api/branches/not-an-id/discard", body: `{"expected_experiment_head":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}`, status: 400},
		{name: "analysis empty profile id", method: http.MethodPost, path: "/api/branches/brn_0123456789abcdef0123/ramifications", body: `{"goal":"test","profile_id":"","model":"ok","expected_main_head":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","expected_experiment_head":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","comparison_fingerprint":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}`, status: 400},
		{name: "analysis empty model", method: http.MethodPost, path: "/api/branches/brn_0123456789abcdef0123/ramifications", body: `{"goal":"test","profile_id":"local_test","model":"","expected_main_head":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","expected_experiment_head":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","comparison_fingerprint":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}`, status: 400},
		{name: "malformed fingerprint", method: http.MethodPost, path: "/api/branches/brn_0123456789abcdef0123/promote", body: `{"paths":["outline.yaml"],"expected_main_head":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","expected_experiment_head":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","comparison_fingerprint":"sha256:short"}`, status: 400},
		{name: "duplicate promotion path", method: http.MethodPost, path: "/api/branches/brn_0123456789abcdef0123/promote", body: `{"paths":["outline.yaml","outline.yaml"],"expected_main_head":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","expected_experiment_head":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","comparison_fingerprint":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}`, status: 400},
		{name: "unknown comparison query", method: http.MethodGet, path: "/api/branches/brn_0123456789abcdef0123/comparison?extra=true", status: 400},
		{name: "duplicate file query", method: http.MethodGet, path: "/api/branches/brn_0123456789abcdef0123/comparison/file?path=outline.yaml&path=outline.yaml", status: 400},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(testCase.method, testCase.path, strings.NewReader(testCase.body)))
			if response.Code != testCase.status {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

// Test: invalid switch targets and comparison paths fail before the branch
// service is invoked.
// Requirements: M8-R01, M8-R07, M8-R20.
func TestBranchRoutesRejectInvalidSwitchTargetAndComparisonPathBeforeService(t *testing.T) {
	t.Parallel()
	store := &spyBranchStore{}
	handler := handlerWithBranches(store)
	tests := []struct {
		name   string
		method string
		path   string
		body   string
		check  func(*testing.T)
	}{
		{
			name:   "invalid switch target",
			method: http.MethodPost,
			path:   "/api/branches/switch",
			body:   `{"target":"not-a-branch","expected_head":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`,
			check: func(t *testing.T) {
				if store.switchCalls != 0 {
					t.Fatalf("switchCalls=%d", store.switchCalls)
				}
			},
		},
		{
			name:   "invalid comparison path",
			method: http.MethodGet,
			path:   "/api/branches/brn_0123456789abcdef0123/comparison/file?path=/absolute/path.md",
			check: func(t *testing.T) {
				if store.fileComparisonCalls != 0 {
					t.Fatalf("fileComparisonCalls=%d", store.fileComparisonCalls)
				}
			},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(testCase.method, testCase.path, strings.NewReader(testCase.body)))
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			testCase.check(t)
		})
	}
}

// Test: malformed branch analysis requests are rejected before the service is
// invoked.
// Requirements: M8-R09, M8-R20.
func TestBranchRoutesRejectMalformedAnalysisBeforeService(t *testing.T) {
	t.Parallel()
	store := &spyBranchStore{}
	response := httptest.NewRecorder()
	handlerWithBranches(store).ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/branches/brn_0123456789abcdef0123/ramifications", strings.NewReader(`{"goal":"test","profile_id":"","model":"ok","expected_main_head":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","expected_experiment_head":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","comparison_fingerprint":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}`)))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if store.analysisCalls != 0 {
		t.Fatalf("analysisCalls=%d", store.analysisCalls)
	}
}

// Test: malformed analysis goals are rejected before the branch service is invoked.
// Requirements: M8-R09, M8-R20.
func TestBranchRoutesRejectAnalysisGoalsWithNULBeforeService(t *testing.T) {
	t.Parallel()
	store := &spyBranchStore{}
	response := httptest.NewRecorder()
	handlerWithBranches(store).ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/branches/brn_0123456789abcdef0123/ramifications", strings.NewReader(`{"goal":"review\u0000goal","profile_id":"local_test","model":"ok","expected_main_head":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","expected_experiment_head":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","comparison_fingerprint":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}`)))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if store.analysisCalls != 0 {
		t.Fatalf("analysisCalls=%d", store.analysisCalls)
	}
}

// Test: POST branch routes reject unexpected query parameters before touching
// the branch service.
// Requirements: M8-R20.
func TestBranchPostRoutesRejectUnexpectedQueryBeforeService(t *testing.T) {
	t.Parallel()
	store := &spyBranchStore{}
	handler := handlerWithBranches(store)
	tests := []struct {
		path string
		body string
	}{
		{path: "/api/branches?extra=1", body: `{"name":"test"}`},
		{path: "/api/branches/switch?extra=1", body: `{"target":"main"}`},
		{path: "/api/branches/brn_0123456789abcdef0123/ramifications?extra=1", body: `{"goal":"test","profile_id":"local_test","model":"ok","expected_main_head":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","expected_experiment_head":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","comparison_fingerprint":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}`},
		{path: "/api/branches/brn_0123456789abcdef0123/promote?extra=1", body: `{"paths":["outline.yaml"],"expected_main_head":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","expected_experiment_head":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","comparison_fingerprint":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}`},
		{path: "/api/branches/brn_0123456789abcdef0123/discard?extra=1", body: `{"expected_experiment_head":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}`},
	}
	for _, testCase := range tests {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, testCase.path, strings.NewReader(testCase.body)))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s status=%d body=%s", testCase.path, response.Code, response.Body.String())
		}
	}
	if store.createCalls != 0 || store.switchCalls != 0 || store.analysisCalls != 0 || store.promoteCalls != 0 || store.discardCalls != 0 {
		t.Fatalf("service calls: create=%d switch=%d analysis=%d promote=%d discard=%d", store.createCalls, store.switchCalls, store.analysisCalls, store.promoteCalls, store.discardCalls)
	}
}

// Test: branch mutation bodies enforce the documented 1 MiB transport limit.
// Requirements: M8-R07, M8-R09, M8-R12, M8-R17.
func TestBranchRoutesRejectOversizedBodies(t *testing.T) {
	t.Parallel()
	response := httptest.NewRecorder()
	body := `{"name":"` + strings.Repeat("x", (1<<20)+1) + `"}`
	newBranchHandler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/branches", strings.NewReader(body)))
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

// Test: every branch route returns method-specific Allow metadata.
// Requirements: M8-R20.
func TestEveryBranchRouteReturnsMethodSpecificAllow(t *testing.T) {
	t.Parallel()
	tests := []struct{ path, allow string }{
		{path: "/api/branches", allow: "GET, POST"},
		{path: "/api/branches/status", allow: "GET"},
		{path: "/api/branches/switch", allow: "POST"},
		{path: "/api/branches/brn_0123456789abcdef0123/comparison", allow: "GET"},
		{path: "/api/branches/brn_0123456789abcdef0123/comparison/file", allow: "GET"},
		{path: "/api/branches/brn_0123456789abcdef0123/ramifications", allow: "POST"},
		{path: "/api/branches/brn_0123456789abcdef0123/promote", allow: "POST"},
		{path: "/api/branches/brn_0123456789abcdef0123/discard", allow: "POST"},
	}
	for _, testCase := range tests {
		response := httptest.NewRecorder()
		newBranchHandler().ServeHTTP(response, httptest.NewRequest(http.MethodPatch, testCase.path, nil))
		if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != testCase.allow {
			t.Fatalf("%s status=%d Allow=%q", testCase.path, response.Code, response.Header().Get("Allow"))
		}
	}
}
