package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

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

func joinProviderURL(baseURL, suffix string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	parsed.Path = path.Clean(strings.TrimSuffix(parsed.Path, "/") + suffix)
	return parsed.String(), nil
}

func decodeStrictJSON(body []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		if err == nil {
			return io.ErrUnexpectedEOF
		}
		return err
	}
	return nil
}

func readBoundedBody(reader io.Reader, limit int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, ErrProviderRejected
	}
	return body, nil
}
