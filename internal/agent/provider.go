package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

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
		client = &http.Client{
			Timeout: 60 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
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
	decision := Compatibility(request.Agent, request.Style, &resolved.Profile, resolved.Readiness)
	if !decision.Compatible {
		return GenerateResponse{}, ErrProviderInvalid
	}
	if request.Agent.ModelRequirements.SupportsStreaming || request.Agent.ModelRequirements.SupportsStructuredOutput {
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

type HTTPGenerator struct {
	client  *http.Client
	mode    provider.Type
	path    string
	decoder func([]byte) (string, error)
	body    func(GenerateRequest) ([]byte, error)
}

func newOpenAICompatibleGenerator(client *http.Client) *HTTPGenerator {
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	return &HTTPGenerator{
		client: client,
		mode:   provider.TypeOpenAICompatible,
		path:   "/chat/completions",
		body: func(request GenerateRequest) ([]byte, error) {
			payload := struct {
				Model       string    `json:"model"`
				Messages    []message `json:"messages"`
				Temperature float64   `json:"temperature"`
				Stream      bool      `json:"stream"`
			}{
				Model: request.Style.Model,
				Messages: []message{
					{Role: "system", Content: strings.TrimSpace(request.Style.SystemPrompt)},
					{Role: "user", Content: userPrompt(request)},
				},
				Temperature: request.Style.Temperature,
				Stream:      false,
			}
			return json.Marshal(payload)
		},
		decoder: func(body []byte) (string, error) {
			var response struct {
				Choices []struct {
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				} `json:"choices"`
			}
			if err := decodeStrictJSON(body, &response); err != nil {
				return "", err
			}
			if len(response.Choices) == 0 {
				return "", ErrProviderRejected
			}
			return response.Choices[0].Message.Content, nil
		},
	}
}

func newOllamaGenerator(client *http.Client) *HTTPGenerator {
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	return &HTTPGenerator{
		client: client,
		mode:   provider.TypeOllama,
		path:   "/api/chat",
		body: func(request GenerateRequest) ([]byte, error) {
			payload := struct {
				Model    string    `json:"model"`
				Messages []message `json:"messages"`
				Stream   bool      `json:"stream"`
				Options  struct {
					Temperature float64 `json:"temperature"`
				} `json:"options"`
			}{
				Model: request.Style.Model,
				Messages: []message{
					{Role: "system", Content: strings.TrimSpace(request.Style.SystemPrompt)},
					{Role: "user", Content: userPrompt(request)},
				},
				Stream: false,
			}
			payload.Options.Temperature = request.Style.Temperature
			return json.Marshal(payload)
		},
		decoder: func(body []byte) (string, error) {
			var response struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}
			if err := decodeStrictJSON(body, &response); err != nil {
				return "", err
			}
			return response.Message.Content, nil
		},
	}
}

func (g *HTTPGenerator) Generate(ctx context.Context, request GenerateRequest, resolved provider.ResolvedProfile) (GenerateResponse, error) {
	body, err := g.body(request)
	if err != nil {
		return GenerateResponse{}, ErrProviderInvalid
	}
	if len(body) > 6<<20 {
		return GenerateResponse{}, ErrProviderRejected
	}
	targetURL, err := joinProviderURL(resolved.BaseURL, g.path)
	if err != nil {
		return GenerateResponse{}, ErrProviderInvalid
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return GenerateResponse{}, ErrProviderInvalid
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	if resolved.Auth.Type == provider.AuthTypeBearerEnv {
		httpRequest.Header.Set("Authorization", "Bearer "+resolved.Credential.Value)
	}
	response, err := g.client.Do(httpRequest)
	if err != nil {
		if errorsIsAny(err, context.Canceled, context.DeadlineExceeded) || isTimeoutError(err) {
			return GenerateResponse{}, ErrProviderOffline
		}
		return GenerateResponse{}, ErrProviderOffline
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusTooManyRequests || response.StatusCode == http.StatusServiceUnavailable {
		return GenerateResponse{}, ErrProviderOffline
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return GenerateResponse{}, ErrProviderRejected
	}
	contentType := strings.TrimSpace(response.Header.Get("Content-Type"))
	if contentType != "" {
		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil || mediaType != "application/json" {
			return GenerateResponse{}, ErrProviderRejected
		}
	}
	responseBody, err := readBoundedBody(response.Body, 2<<20)
	if err != nil {
		return GenerateResponse{}, ErrProviderRejected
	}
	content, err := g.decoder(responseBody)
	if err != nil {
		return GenerateResponse{}, ErrProviderRejected
	}
	content, err = normalizeReplacement(content)
	if err != nil {
		return GenerateResponse{}, ErrProviderRejected
	}
	return GenerateResponse{
		Replacement: content,
		Provider: ProviderIdentity{
			ProfileID: resolved.ID,
			Type:      resolved.Type,
			Model:     request.Style.Model,
		},
	}, nil
}

func userPrompt(request GenerateRequest) string {
	return "Task: " + request.Agent.Description + "\n\n" +
		"Rewrite only the selected text. Return replacement text only. Do not add commentary or Markdown fences.\n\n" +
		"Selected text:\n" + request.Packet.SelectedText
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
	decoder.DisallowUnknownFields()
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

func normalizeReplacement(content string) (string, error) {
	if !utf8.ValidString(content) {
		return "", ErrProviderRejected
	}
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	if len(content) > 5<<20 {
		return "", ErrProviderRejected
	}
	if strings.TrimFunc(content, unicode.IsSpace) == "" {
		return "", ErrProviderRejected
	}
	if strings.ContainsRune(content, '\x00') {
		return "", ErrProviderRejected
	}
	return content, nil
}

func errorsIsAny(err error, targets ...error) bool {
	for _, target := range targets {
		if target != nil && errors.Is(err, target) {
			return true
		}
	}
	return false
}

func isTimeoutError(err error) bool {
	var netErr net.Error
	return errorAs(err, &netErr) && netErr.Timeout()
}

func errorAs(err error, target any) bool {
	return errors.As(err, target)
}
