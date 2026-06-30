package agent

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

type GenerateRequest struct {
	Agent   Agent
	Style   Style
	Packet  ContextPacket
	Summary ContextSummary
}

type GenerateResponse struct {
	Replacement string
}

type TextGenerator interface {
	Generate(context.Context, GenerateRequest) (GenerateResponse, error)
}

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
	return GenerateResponse{Replacement: "Mock polished: " + strings.TrimSpace(request.Packet.SelectedText)}, nil
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
