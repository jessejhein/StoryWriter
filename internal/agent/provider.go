package agent

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"storywork/internal/provider"
)

type GenerateRequest struct {
	Agent   Agent
	Style   Style
	Packet  ContextPacket
	Summary ContextSummary
}

type GenerateResponse struct {
	Replacement string
	Provider    ProviderIdentity
}

type TextGenerator interface {
	Generate(context.Context, GenerateRequest) (GenerateResponse, error)
}

type ProviderIdentity struct {
	ProfileID string        `json:"profile_id"`
	Type      provider.Type `json:"type"`
	Model     string        `json:"model"`
}

var (
	ErrProviderInvalid  = fmt.Errorf("provider invalid")
	ErrProviderRejected = fmt.Errorf("provider rejected")
	ErrProviderOffline  = fmt.Errorf("provider unavailable")
)

type MockProvider struct{}

func NewMockProvider() *MockProvider {
	return &MockProvider{}
}

func (p *MockProvider) Generate(ctx context.Context, request GenerateRequest) (GenerateResponse, error) {
	select {
	case <-ctx.Done():
		return GenerateResponse{}, ctx.Err()
	default:
	}
	return GenerateResponse{
		Replacement: "Mock polished: " + strings.TrimSpace(request.Packet.SelectedText),
		Provider: ProviderIdentity{
			ProfileID: request.Style.ProviderProfileID,
			Type:      provider.TypeOpenAICompatible,
			Model:     request.Style.Model,
		},
	}, nil
}

func ValidateExecutableSelectionAgent(agent Agent) error {
	if !slices.Contains(agent.AppliesWhen.Surfaces, SurfaceEditor) || !slices.Contains(agent.AppliesWhen.InputScopes, InputScopeSelection) {
		return fmt.Errorf("agent %q is not executable for Milestone 4 editor selections: %w", agent.ID, ErrInvalidAgent)
	}
	if agent.RAGPolicy.Mode != RAGModeNone {
		return fmt.Errorf("agent %q rag mode is unsupported: %w", agent.ID, ErrInvalidAgent)
	}
	return nil
}

type profileResolver interface {
	Resolve(ctx context.Context, profileID string) (provider.ResolvedProfile, bool, error)
}

type Dispatcher struct {
	resolver profileResolver
	mock     TextGenerator
	openAI   *HTTPGenerator
	ollama   *HTTPGenerator
}

func NewDispatcher(resolver profileResolver, client *http.Client) *Dispatcher {
	if client == nil {
		client = &http.Client{}
	} else {
		clientCopy := *client
		client = &clientCopy
	}
	if client.Timeout == 0 {
		client.Timeout = 60 * time.Second
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &Dispatcher{
		resolver: resolver,
		mock:     NewMockProvider(),
		openAI:   newOpenAICompatibleGenerator(client),
		ollama:   newOllamaGenerator(client),
	}
}

func (d *Dispatcher) Generate(ctx context.Context, request GenerateRequest) (GenerateResponse, error) {
	if request.Style.Version == 1 && request.Style.ProviderProfileID == "mock_default" && request.Style.Model == "mock" {
		return d.mock.Generate(ctx, request)
	}
	if d.resolver == nil {
		return GenerateResponse{}, ErrProviderInvalid
	}
	resolved, found, err := d.resolver.Resolve(ctx, request.Style.ProviderProfileID)
	if err != nil {
		return GenerateResponse{}, err
	}
	if !found {
		return GenerateResponse{}, ErrProviderInvalid
	}
	decision := ExecutableCompatibility(request.Agent, request.Style, &resolved.Profile, resolved.Readiness)
	if !decision.Compatible {
		return GenerateResponse{}, ErrProviderInvalid
	}
	switch resolved.Type {
	case provider.TypeOpenAICompatible:
		return d.openAI.Generate(ctx, request, resolved)
	case provider.TypeOllama:
		return d.ollama.Generate(ctx, request, resolved)
	default:
		return GenerateResponse{}, ErrProviderInvalid
	}
}
