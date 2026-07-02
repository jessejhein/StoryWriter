package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"mime"
	"net/http"
	"strings"

	"storywork/internal/provider"
)

type ChatMessage struct {
	Role    string
	Content string
}

type ChatRequest struct {
	Profile     provider.ResolvedProfile
	Model       string
	Messages    []ChatMessage
	Temperature *float64
}

type ChatResponse struct {
	Content  string
	Provider ProviderIdentity
}

func CompleteChat(ctx context.Context, client *http.Client, request ChatRequest) (ChatResponse, error) {
	if client == nil {
		client = &http.Client{}
	}
	switch request.Profile.Type {
	case provider.TypeOpenAICompatible:
		return completeOpenAIChat(ctx, client, request)
	case provider.TypeOllama:
		return completeOllamaChat(ctx, client, request)
	default:
		return ChatResponse{}, ErrProviderInvalid
	}
}

func completeOpenAIChat(ctx context.Context, client *http.Client, request ChatRequest) (ChatResponse, error) {
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	payload := struct {
		Model       string    `json:"model"`
		Messages    []message `json:"messages"`
		Temperature *float64  `json:"temperature,omitempty"`
		Stream      bool      `json:"stream"`
	}{
		Model:       request.Model,
		Messages:    make([]message, 0, len(request.Messages)),
		Temperature: request.Temperature,
		Stream:      false,
	}
	for _, entry := range request.Messages {
		payload.Messages = append(payload.Messages, message{Role: entry.Role, Content: entry.Content})
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return ChatResponse{}, ErrProviderInvalid
	}
	responseBody, err := doProviderChat(ctx, client, request.Profile, "/chat/completions", body)
	if err != nil {
		return ChatResponse{}, err
	}
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := decodeStrictJSON(responseBody, &response); err != nil {
		return ChatResponse{}, ErrProviderRejected
	}
	if len(response.Choices) == 0 {
		return ChatResponse{}, ErrProviderRejected
	}
	return ChatResponse{
		Content: response.Choices[0].Message.Content,
		Provider: ProviderIdentity{
			ProfileID: request.Profile.ID,
			Type:      request.Profile.Type,
			Model:     request.Model,
		},
	}, nil
}

func completeOllamaChat(ctx context.Context, client *http.Client, request ChatRequest) (ChatResponse, error) {
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	payload := struct {
		Model    string    `json:"model"`
		Messages []message `json:"messages"`
		Stream   bool      `json:"stream"`
		Options  struct {
			Temperature *float64 `json:"temperature,omitempty"`
		} `json:"options"`
	}{
		Model:    request.Model,
		Messages: make([]message, 0, len(request.Messages)),
		Stream:   false,
	}
	payload.Options.Temperature = request.Temperature
	for _, entry := range request.Messages {
		payload.Messages = append(payload.Messages, message{Role: entry.Role, Content: entry.Content})
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return ChatResponse{}, ErrProviderInvalid
	}
	responseBody, err := doProviderChat(ctx, client, request.Profile, "/api/chat", body)
	if err != nil {
		return ChatResponse{}, err
	}
	var response struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := decodeStrictJSON(responseBody, &response); err != nil {
		return ChatResponse{}, ErrProviderRejected
	}
	return ChatResponse{
		Content: response.Message.Content,
		Provider: ProviderIdentity{
			ProfileID: request.Profile.ID,
			Type:      request.Profile.Type,
			Model:     request.Model,
		},
	}, nil
}

func doProviderChat(ctx context.Context, client *http.Client, profile provider.ResolvedProfile, suffix string, body []byte) ([]byte, error) {
	if len(body) > 6<<20 {
		return nil, ErrProviderRejected
	}
	targetURL, err := joinProviderURL(profile.BaseURL, suffix)
	if err != nil {
		return nil, ErrProviderInvalid
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return nil, ErrProviderInvalid
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	if profile.Auth.Type == provider.AuthTypeBearerEnv {
		httpRequest.Header.Set("Authorization", "Bearer "+profile.Credential.Value)
	}
	response, err := client.Do(httpRequest)
	if err != nil {
		return nil, ErrProviderOffline
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusTooManyRequests || response.StatusCode == http.StatusServiceUnavailable {
		return nil, ErrProviderOffline
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, ErrProviderRejected
	}
	contentType := strings.TrimSpace(response.Header.Get("Content-Type"))
	if contentType != "" {
		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil || mediaType != "application/json" {
			return nil, ErrProviderRejected
		}
	}
	responseBody, err := readBoundedBody(response.Body, 2<<20)
	if err != nil {
		return nil, ErrProviderRejected
	}
	return responseBody, nil
}
