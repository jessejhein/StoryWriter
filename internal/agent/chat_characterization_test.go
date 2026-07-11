package agent

// BDD Scenario: 8.3.1 - Run only after explicit authorization
// Requirements: M8-R11
// Test purpose: Lock exact OpenAI-compatible and Ollama chat transport behavior
// before shared wire code moves to internal/modelchat.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"storywork/internal/provider"
)

// Test: OpenAI-compatible chat uses exact path, headers, body, and identity mapping.
// Requirements: M8-R11.
func TestM8ChatCharacterizationOpenAICompatibleRequest(t *testing.T) {
	t.Parallel()

	var capturedMethod string
	var capturedURL string
	var capturedAuth string
	var capturedContentType string
	var capturedBody []byte

	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			capturedMethod = request.Method
			capturedURL = request.URL.String()
			capturedAuth = request.Header.Get("Authorization")
			capturedContentType = request.Header.Get("Content-Type")
			var err error
			capturedBody, err = io.ReadAll(request.Body)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"assistant reply"}}]}`)),
			}, err
		}),
	}

	temp := 0.35
	response, err := CompleteChat(context.Background(), client, ChatRequest{
		Profile: provider.ResolvedProfile{
			Profile: provider.Profile{
				ID: "openai_hosted", Type: provider.TypeOpenAICompatible,
				BaseURL: "https://api.example.test/v1",
				Auth:    provider.AuthConfig{Type: provider.AuthTypeBearerEnv, CredentialEnv: "KEY"},
			},
			Credential: provider.Credential{Value: "secret-token-42"},
		},
		Model: "gpt-test-model",
		Messages: []ChatMessage{
			{Role: "system", Content: "System guidance"},
			{Role: "user", Content: "User payload"},
		},
		Temperature: &temp,
	})
	if err != nil {
		t.Fatalf("CompleteChat() error = %v", err)
	}
	if response.Content != "assistant reply" {
		t.Fatalf("content = %q, want assistant reply", response.Content)
	}
	if response.Provider.ProfileID != "openai_hosted" || response.Provider.Type != provider.TypeOpenAICompatible || response.Provider.Model != "gpt-test-model" {
		t.Fatalf("provider identity = %#v", response.Provider)
	}
	if capturedMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", capturedMethod)
	}
	if capturedURL != "https://api.example.test/v1/chat/completions" {
		t.Fatalf("url = %q", capturedURL)
	}
	if capturedAuth != "Bearer secret-token-42" {
		t.Fatalf("authorization = %q", capturedAuth)
	}
	if capturedContentType != "application/json" {
		t.Fatalf("content-type = %q", capturedContentType)
	}

	var payload map[string]any
	if err := json.Unmarshal(capturedBody, &payload); err != nil {
		t.Fatalf("Unmarshal(body) error = %v, body = %s", err, capturedBody)
	}
	if payload["model"] != "gpt-test-model" {
		t.Fatalf("model = %#v", payload["model"])
	}
	if payload["stream"] != false {
		t.Fatalf("stream = %#v, want false", payload["stream"])
	}
	if payload["temperature"] != 0.35 {
		t.Fatalf("temperature = %#v, want 0.35", payload["temperature"])
	}
	messages, ok := payload["messages"].([]any)
	if !ok || len(messages) != 2 {
		t.Fatalf("messages = %#v", payload["messages"])
	}
	first, ok := messages[0].(map[string]any)
	if !ok || first["role"] != "system" || first["content"] != "System guidance" {
		t.Fatalf("first message = %#v", messages[0])
	}
	second, ok := messages[1].(map[string]any)
	if !ok || second["role"] != "user" || second["content"] != "User payload" {
		t.Fatalf("second message = %#v", messages[1])
	}
}

// Test: Ollama chat uses exact path, options temperature, stream flag, and identity.
// Requirements: M8-R11.
func TestM8ChatCharacterizationOllamaRequest(t *testing.T) {
	t.Parallel()

	var capturedURL string
	var capturedBody []byte

	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			capturedURL = request.URL.String()
			if auth := request.Header.Get("Authorization"); auth != "" {
				t.Fatalf("ollama authorization = %q, want empty", auth)
			}
			var err error
			capturedBody, err = io.ReadAll(request.Body)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"message":{"content":"ollama reply"}}`)),
			}, err
		}),
	}

	temp := 0.55
	response, err := CompleteChat(context.Background(), client, ChatRequest{
		Profile: provider.ResolvedProfile{
			Profile: provider.Profile{
				ID: "local_ollama", Type: provider.TypeOllama, BaseURL: "http://127.0.0.1:11434",
				Auth: provider.AuthConfig{Type: provider.AuthTypeNone},
			},
		},
		Model: "ollama-model",
		Messages: []ChatMessage{
			{Role: "system", Content: "Ollama system"},
			{Role: "user", Content: "Ollama user"},
		},
		Temperature: &temp,
	})
	if err != nil {
		t.Fatalf("CompleteChat() error = %v", err)
	}
	if response.Content != "ollama reply" {
		t.Fatalf("content = %q", response.Content)
	}
	if response.Provider.ProfileID != "local_ollama" || response.Provider.Type != provider.TypeOllama || response.Provider.Model != "ollama-model" {
		t.Fatalf("provider identity = %#v", response.Provider)
	}
	if capturedURL != "http://127.0.0.1:11434/api/chat" {
		t.Fatalf("url = %q", capturedURL)
	}

	var payload struct {
		Model    string `json:"model"`
		Stream   bool   `json:"stream"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		Options struct {
			Temperature *float64 `json:"temperature,omitempty"`
		} `json:"options"`
	}
	if err := json.Unmarshal(capturedBody, &payload); err != nil {
		t.Fatalf("Unmarshal(body) error = %v, body = %s", err, capturedBody)
	}
	if payload.Model != "ollama-model" || payload.Stream != false {
		t.Fatalf("payload = %+v", payload)
	}
	if payload.Options.Temperature == nil || *payload.Options.Temperature != 0.55 {
		t.Fatalf("options.temperature = %#v, want 0.55", payload.Options.Temperature)
	}
	if len(payload.Messages) != 2 || payload.Messages[0].Role != "system" || payload.Messages[1].Content != "Ollama user" {
		t.Fatalf("messages = %+v", payload.Messages)
	}
	if bytes.Contains(capturedBody, []byte(`"temperature":0.55`)) && !bytes.Contains(capturedBody, []byte(`"options"`)) {
		t.Fatalf("temperature must live under options for ollama: %s", capturedBody)
	}
}

// Test: chat transport maps redirects, statuses, content types, limits, and malformed JSON.
// Requirements: M8-R11.
func TestM8ChatCharacterizationErrorMapping(t *testing.T) {
	t.Parallel()

	profile := provider.ResolvedProfile{
		Profile: provider.Profile{
			ID: "hosted", Type: provider.TypeOpenAICompatible, BaseURL: "https://api.example.test/v1",
			Auth: provider.AuthConfig{Type: provider.AuthTypeBearerEnv, CredentialEnv: "KEY"},
		},
		Credential: provider.Credential{Value: "secret-token"},
	}
	baseRequest := ChatRequest{
		Profile:  profile,
		Model:    "model",
		Messages: []ChatMessage{{Role: "user", Content: "hello"}},
	}

	tests := []struct {
		name         string
		profile      provider.ResolvedProfile
		request      ChatRequest
		statusCode   int
		contentType  string
		body         string
		roundTripErr error
		header       http.Header
		client       *http.Client
		want         error
	}{
		{name: "unsupported provider type", profile: provider.ResolvedProfile{Profile: provider.Profile{Type: provider.Type("unknown")}}, request: baseRequest, want: ErrProviderInvalid},
		{
			name: "redirect rejected", statusCode: http.StatusFound,
			header: http.Header{"Location": []string{"https://redirected.example/secret"}},
			want:   ErrProviderRejected,
		},
		{name: "429 unavailable", statusCode: http.StatusTooManyRequests, want: ErrProviderOffline},
		{name: "503 unavailable", statusCode: http.StatusServiceUnavailable, want: ErrProviderOffline},
		{name: "4xx rejected", statusCode: http.StatusBadRequest, body: `{"error":"detail"}`, want: ErrProviderRejected},
		{name: "malformed json", statusCode: http.StatusOK, contentType: "application/json", body: `{"choices":[`, want: ErrProviderRejected},
		{name: "empty choices", statusCode: http.StatusOK, contentType: "application/json", body: `{"choices":[]}`, want: ErrProviderRejected},
		{name: "non-json content type", statusCode: http.StatusOK, contentType: "text/plain", body: "plain", want: ErrProviderRejected},
		{name: "network offline", roundTripErr: errors.New("connection refused"), want: ErrProviderOffline},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := tc.request
			if request.Profile.ID == "" && tc.profile.Profile.ID != "" {
				request.Profile = tc.profile
			}
			if request.Profile.Type == "" && tc.profile.Profile.Type != "" {
				request.Profile = tc.profile
			}
			if tc.name == "unsupported provider type" {
				request = ChatRequest{Profile: tc.profile, Model: "model", Messages: baseRequest.Messages}
			} else if request.Profile.ID == "" {
				request = baseRequest
			}

			client := tc.client
			if client == nil {
				client = &http.Client{
					CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
					Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
						if tc.roundTripErr != nil {
							return nil, tc.roundTripErr
						}
						header := tc.header
						if header == nil {
							header = http.Header{}
						}
						if tc.contentType != "" {
							header.Set("Content-Type", tc.contentType)
						}
						return &http.Response{
							StatusCode: tc.statusCode,
							Header:     header,
							Body:       io.NopCloser(strings.NewReader(tc.body)),
						}, nil
					}),
				}
			}

			_, err := CompleteChat(context.Background(), client, request)
			if !errors.Is(err, tc.want) {
				t.Fatalf("CompleteChat() error = %v, want %v", err, tc.want)
			}
		})
	}

	t.Run("context cancellation maps offline", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		client := &http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return nil, ctx.Err()
			}),
		}
		_, err := CompleteChat(ctx, client, baseRequest)
		if !errors.Is(err, ErrProviderOffline) {
			t.Fatalf("CompleteChat() error = %v, want ErrProviderOffline", err)
		}
	})

	t.Run("oversize request rejected before network", func(t *testing.T) {
		t.Parallel()
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Fatal("unexpected outbound request for oversize payload")
			return nil, nil
		})}
		huge := strings.Repeat("x", 6<<20+1)
		_, err := CompleteChat(context.Background(), client, ChatRequest{
			Profile:  profile,
			Model:    "model",
			Messages: []ChatMessage{{Role: "user", Content: huge}},
		})
		if !errors.Is(err, ErrProviderRejected) {
			t.Fatalf("CompleteChat() error = %v, want ErrProviderRejected", err)
		}
	})

	t.Run("oversize response rejected", func(t *testing.T) {
		t.Parallel()
		oversized := `{"choices":[{"message":{"content":"` + strings.Repeat("a", 2<<20) + `"}}]}`
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(oversized)),
			}, nil
		})}
		_, err := CompleteChat(context.Background(), client, baseRequest)
		if !errors.Is(err, ErrProviderRejected) {
			t.Fatalf("CompleteChat() error = %v, want ErrProviderRejected", err)
		}
	})
}

// Test: provider errors never echo bearer credentials or redirect secrets.
// Requirements: M8-R11.
func TestM8ChatCharacterizationSecretsAbsentFromErrors(t *testing.T) {
	t.Parallel()

	const sentinel = "sentinel-secret-99"
	profile := provider.ResolvedProfile{
		Profile: provider.Profile{
			ID: "hosted", Type: provider.TypeOpenAICompatible, BaseURL: "https://origin.example/v1",
			Auth: provider.AuthConfig{Type: provider.AuthTypeBearerEnv, CredentialEnv: "KEY"},
		},
		Credential: provider.Credential{Value: sentinel},
	}
	request := ChatRequest{
		Profile:  profile,
		Model:    "model",
		Messages: []ChatMessage{{Role: "user", Content: "hello"}},
	}

	for _, tc := range []struct {
		name       string
		statusCode int
		header     http.Header
		body       string
	}{
		{name: "redirect body", statusCode: http.StatusFound, header: http.Header{"Location": []string{"https://redirected.example/" + sentinel}}, body: sentinel},
		{name: "upstream error body", statusCode: http.StatusInternalServerError, body: `{"error":"` + sentinel + `"}`},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := &http.Client{
				CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: tc.statusCode,
						Header:     tc.header,
						Body:       io.NopCloser(strings.NewReader(tc.body)),
					}, nil
				}),
			}
			_, err := CompleteChat(context.Background(), client, request)
			if err == nil {
				t.Fatal("CompleteChat() error = nil, want failure")
			}
			if strings.Contains(err.Error(), sentinel) {
				t.Fatalf("error leaked sentinel credential: %v", err)
			}
		})
	}
}

// Test: transport deadline failures surface as provider offline without leaking details.
// Requirements: M8-R11.
func TestM8ChatCharacterizationTimeoutMapsOffline(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, context.DeadlineExceeded
		}),
	}
	_, err := CompleteChat(context.Background(), client, ChatRequest{
		Profile: provider.ResolvedProfile{
			Profile: provider.Profile{
				ID: "hosted", Type: provider.TypeOpenAICompatible, BaseURL: "https://api.example.test/v1",
			},
		},
		Model:    "model",
		Messages: []ChatMessage{{Role: "user", Content: "hello"}},
	})
	if !errors.Is(err, ErrProviderOffline) {
		t.Fatalf("CompleteChat() error = %v, want ErrProviderOffline", err)
	}
	if strings.Contains(err.Error(), "deadline") {
		t.Fatalf("error leaked transport detail: %v", err)
	}
}
