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

func TestHealth(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	response := httptest.NewRecorder()
	api.NewHandler(&projectStoreStub{}, "0.0.0-test").ServeHTTP(response, request)

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

func TestCreateProject(t *testing.T) {
	t.Parallel()

	store := &projectStoreStub{created: project.Project{
		ID: "proj_test_novel", Path: "/tmp/test-novel", GitInitialized: true, IndexInitialized: true,
	}}
	request := httptest.NewRequest(http.MethodPost, "/api/projects", strings.NewReader(`{"name":"Test Novel","path":"/tmp/test-novel"}`))
	response := httptest.NewRecorder()
	api.NewHandler(store, "test").ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusCreated, response.Body.String())
	}
	if store.createRequest.Name != "Test Novel" {
		t.Fatalf("create name = %q", store.createRequest.Name)
	}
}
