package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"storywork/internal/api"
	"storywork/internal/project"
)

type projectStoreStub struct {
	createRequest project.CreateRequest
	created       project.Project
}

func (s *projectStoreStub) Create(_ context.Context, request project.CreateRequest) (project.Project, error) {
	s.createRequest = request
	return s.created, nil
}

func (s *projectStoreStub) Open(_ context.Context, _ string) (project.Project, error) {
	return s.created, nil
}

// BDD trace:
//   - Requirement: Milestone 0, Story 0.3, health check.
//   - Scenario: when I request /api/health, I receive status ok and version
//     information.
//   - Test purpose: verify the API health handler returns the expected status
//     and version payload.
func TestHealth(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	response := httptest.NewRecorder()
	api.NewHandler(&projectStoreStub{}, &activeProjectSessionStub{}, &storyServiceStub{}, "0.0.0-test").ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	var body map[string]string
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" || body["version"] != "0.0.0-test" {
		t.Fatalf("body = %#v", body)
	}
}

// BDD trace:
//   - Requirement: Milestone 0, Story 0.1, create project folder.
//   - Scenario: given an empty directory path, when I create a project named
//     "Test Novel", then the app writes project starter files, initializes Git,
//     creates the SQLite index, and records a first commit.
//   - Test purpose: verify the API forwards the project create request and
//     returns HTTP 201 for a successful project creation.
func TestCreateProject(t *testing.T) {
	t.Parallel()

	store := &projectStoreStub{created: project.Project{
		ID: "proj_test_novel", Path: "/tmp/test-novel", GitInitialized: true, IndexInitialized: true,
	}}
	request := httptest.NewRequest(http.MethodPost, "/api/projects", strings.NewReader(`{"name":"Test Novel","path":"/tmp/test-novel"}`))
	response := httptest.NewRecorder()
	api.NewHandler(store, &activeProjectSessionStub{}, &storyServiceStub{}, "test").ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusCreated, response.Body.String())
	}
	if store.createRequest.Name != "Test Novel" {
		t.Fatalf("create name = %q", store.createRequest.Name)
	}
}
