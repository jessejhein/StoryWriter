package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"storywork/internal/agent"
)

// BDD trace:
//   - Requirements: M5-R01, M5-R09, M5-R10.
//   - Scenario: invalid application config paths fail closed for every provider consumer.
//   - Test purpose: prevent action dispatch from bypassing config-path errors.
func TestProviderDependenciesRejectInvalidConfigPath(t *testing.T) {
	t.Parallel()

	providers := newProviderDependencies("relative/config", func() (string, error) {
		return t.TempDir(), nil
	})

	if _, _, err := providers.List(context.Background()); err == nil {
		t.Fatal("List() error = nil, want config path failure")
	}
	if _, found, err := providers.Resolve(context.Background(), "working_directory_profile"); err == nil || found {
		t.Fatalf("Resolve() = (_, %t, %v), want false and config path failure", found, err)
	}
	dispatcher := agent.NewDispatcher(providers, &http.Client{Transport: failingRoundTripper{t: t}})
	_, dispatchErr := dispatcher.Generate(context.Background(), agent.GenerateRequest{Style: agent.Style{
		Version: 2, ProviderProfileID: "working_directory_profile", Model: "model",
	}})
	if !errors.Is(dispatchErr, errInvalidProviderConfigPath) {
		t.Fatalf("dispatcher.Generate() error = %v, want config path failure", dispatchErr)
	}
	if !errors.Is(providers.configErr, errInvalidProviderConfigPath) {
		t.Fatalf("configErr = %v, want errInvalidProviderConfigPath", providers.configErr)
	}
}

type failingRoundTripper struct{ t *testing.T }

func (f failingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	f.t.Fatal("invalid config path reached outbound HTTP")
	return nil, errors.New("unexpected outbound HTTP")
}

// Test: production composition supplies every handler dependency boundary.
// Requirements: M7-R19.
func TestProductionCompositionSuppliesAllHandlerStores(t *testing.T) {
	t.Parallel()

	handler := NewHandler("composition-test")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("health status = %d, want 200 body=%s", response.Code, response.Body.String())
	}
}
