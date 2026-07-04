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
	"strings"
	"testing"

	"storywork/internal/api"
	"storywork/internal/branch"
)

type statusErrorBranchStore struct {
	branchServiceStub
	err error
}

type spyBranchStore struct {
	branchServiceStub
	analysisCalls int
}

func (s statusErrorBranchStore) Status(context.Context) (branch.RepositoryStatus, error) {
	return branch.RepositoryStatus{}, s.err
}

func handlerWithBranches(store api.BranchStore) http.Handler {
	return api.NewHandler(api.HandlerDependencies{Projects: &projectStoreStub{}, Session: &activeProjectSessionStub{}, Stories: &storyServiceStub{}, Actions: &storyServiceStub{}, Providers: &storyServiceStub{}, Imports: &storyServiceStub{}, Branches: store, Version: "test"})
}

func (s *spyBranchStore) AnalyzeRamifications(context.Context, string, branch.AnalysisRequest) (branch.AnalysisResult, error) {
	s.analysisCalls++
	return branch.AnalysisResult{}, nil
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
		{name: "conflict", err: branch.ErrStaleRef, status: http.StatusConflict},
		{name: "not found", err: branch.ErrExperimentNotFound, status: http.StatusNotFound},
		{name: "too large", err: branch.ErrFileTooLarge, status: http.StatusRequestEntityTooLarge},
		{name: "provider output", err: branch.ErrInvalidAnalysisOutput, status: http.StatusBadGateway},
		{name: "provider unavailable", err: branch.ErrProviderUnavailable, status: http.StatusServiceUnavailable},
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
