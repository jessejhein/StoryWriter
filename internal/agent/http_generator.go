package agent

import (
	"context"
	"net/http"

	"storywork/internal/provider"
)

type HTTPGenerator struct {
	client   *http.Client
	messages func(GenerateRequest) ([]ChatMessage, *float64)
}

func (g *HTTPGenerator) Generate(ctx context.Context, request GenerateRequest, resolved provider.ResolvedProfile) (GenerateResponse, error) {
	messages, temperature := g.messages(request)
	chatResponse, err := CompleteChat(ctx, g.client, ChatRequest{
		Profile:     resolved,
		Model:       request.Style.Model,
		Messages:    messages,
		Temperature: temperature,
	})
	if err != nil {
		return GenerateResponse{}, err
	}
	content, err := NormalizeGeneratedReplacement(chatResponse.Content)
	if err != nil {
		return GenerateResponse{}, ErrProviderRejected
	}
	return GenerateResponse{Replacement: content, Provider: chatResponse.Provider}, nil
}

func userPrompt(request GenerateRequest) string {
	return "Task: " + request.Agent.Description + "\n\nRewrite only the selected text. Return replacement text only. Do not add commentary or Markdown fences.\n\nSelected text:\n" + request.Packet.SelectedText
}
