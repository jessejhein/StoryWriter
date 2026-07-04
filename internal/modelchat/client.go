package modelchat

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"storywork/internal/provider"
)

// HTTPClient implements Completer using OpenAI-compatible and Ollama wire formats.
type HTTPClient struct{}

// NewHTTPClient returns a provider-neutral HTTP completer.
func NewHTTPClient() *HTTPClient {
	return &HTTPClient{}
}

// PrepareHTTPClient copies the supplied client and applies the safe transport
// policy used by Storywork model consumers.
func PrepareHTTPClient(client *http.Client) *http.Client {
	if client == nil {
		client = &http.Client{}
	}
	copyClient := *client
	client = &copyClient
	if client.Timeout == 0 {
		client.Timeout = 60 * time.Second
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return client
}

// Complete performs chat completion for a resolved profile.
func (c *HTTPClient) Complete(ctx context.Context, client *http.Client, request Request) (Response, error) {
	return Complete(ctx, client, request)
}

// Complete performs chat completion using the default HTTP completer.
func Complete(ctx context.Context, client *http.Client, request Request) (Response, error) {
	if client == nil {
		client = PrepareHTTPClient(nil)
	}
	switch request.Profile.Type {
	case provider.TypeOpenAICompatible:
		return completeOpenAIChat(ctx, client, request)
	case provider.TypeOllama:
		return completeOllamaChat(ctx, client, request)
	default:
		return Response{}, ErrProviderInvalid
	}
}

func completeOpenAIChat(ctx context.Context, client *http.Client, request Request) (Response, error) {
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
		return Response{}, ErrProviderInvalid
	}
	responseBody, err := doProviderChat(ctx, client, request.Profile, "/chat/completions", body)
	if err != nil {
		return Response{}, err
	}
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := decodeStrictJSON(responseBody, &response); err != nil {
		return Response{}, ErrProviderRejected
	}
	if len(response.Choices) == 0 {
		return Response{}, ErrProviderRejected
	}
	return Response{
		Content: response.Choices[0].Message.Content,
		Provider: ProviderIdentity{
			ProfileID: request.Profile.ID,
			Type:      request.Profile.Type,
			Model:     request.Model,
		},
	}, nil
}

func completeOllamaChat(ctx context.Context, client *http.Client, request Request) (Response, error) {
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
		return Response{}, ErrProviderInvalid
	}
	responseBody, err := doProviderChat(ctx, client, request.Profile, "/api/chat", body)
	if err != nil {
		return Response{}, err
	}
	var response struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := decodeStrictJSON(responseBody, &response); err != nil {
		return Response{}, ErrProviderRejected
	}
	return Response{
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

// Ensure HTTPClient satisfies Completer at compile time.
var _ Completer = (*HTTPClient)(nil)
