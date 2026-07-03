package modelchat

// BDD Scenario: 8.3.1 - Run only after explicit authorization
// Requirements: M8-R11
// Test purpose: Prove modelchat transport preserves exact OpenAI/Ollama wire behavior.

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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

// Test: OpenAI-compatible completion uses exact wire mapping through modelchat.
// Requirements: M8-R11.
func TestM8ModelchatOpenAICompatibleRequest(t *testing.T) {
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
	response, err := Complete(context.Background(), client, Request{
		Profile: provider.ResolvedProfile{
			Profile: provider.Profile{
				ID: "openai_hosted", Type: provider.TypeOpenAICompatible,
				BaseURL: "https://api.example.test/v1",
				Auth:    provider.AuthConfig{Type: provider.AuthTypeBearerEnv, CredentialEnv: "KEY"},
			},
			Credential: provider.Credential{Value: "secret-token-42"},
		},
		Model: "gpt-test-model",
		Messages: []Message{
			{Role: "system", Content: "System guidance"},
			{Role: "user", Content: "User payload"},
		},
		Temperature: &temp,
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if response.Content != "assistant reply" {
		t.Fatalf("content = %q", response.Content)
	}
	if response.Provider.ProfileID != "openai_hosted" || response.Provider.Type != provider.TypeOpenAICompatible || response.Provider.Model != "gpt-test-model" {
		t.Fatalf("provider identity = %#v", response.Provider)
	}
	if capturedMethod != http.MethodPost || capturedURL != "https://api.example.test/v1/chat/completions" {
		t.Fatalf("request = %s %s", capturedMethod, capturedURL)
	}
	if capturedAuth != "Bearer secret-token-42" || capturedContentType != "application/json" {
		t.Fatalf("headers auth=%q content-type=%q", capturedAuth, capturedContentType)
	}

	var payload map[string]any
	if err := json.Unmarshal(capturedBody, &payload); err != nil {
		t.Fatalf("Unmarshal(body) error = %v", err)
	}
	if payload["model"] != "gpt-test-model" || payload["stream"] != false || payload["temperature"] != 0.35 {
		t.Fatalf("payload = %#v", payload)
	}
}

// Test: Ollama completion uses options temperature and /api/chat path.
// Requirements: M8-R11.
func TestM8ModelchatOllamaRequest(t *testing.T) {
	t.Parallel()

	var capturedURL string
	var capturedBody []byte

	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			capturedURL = request.URL.String()
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
	response, err := Complete(context.Background(), client, Request{
		Profile: provider.ResolvedProfile{
			Profile: provider.Profile{
				ID: "local_ollama", Type: provider.TypeOllama, BaseURL: "http://127.0.0.1:11434",
			},
		},
		Model: "ollama-model",
		Messages: []Message{
			{Role: "system", Content: "Ollama system"},
			{Role: "user", Content: "Ollama user"},
		},
		Temperature: &temp,
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if response.Content != "ollama reply" {
		t.Fatalf("content = %q", response.Content)
	}
	if capturedURL != "http://127.0.0.1:11434/api/chat" {
		t.Fatalf("url = %q", capturedURL)
	}
	if !bytes.Contains(capturedBody, []byte(`"options"`)) || !bytes.Contains(capturedBody, []byte(`"temperature":0.55`)) {
		t.Fatalf("body = %s", capturedBody)
	}
}

// Test: modelchat maps transport failures to neutral provider errors.
// Requirements: M8-R11.
func TestM8ModelchatErrorMapping(t *testing.T) {
	t.Parallel()

	profile := provider.ResolvedProfile{
		Profile: provider.Profile{
			ID: "hosted", Type: provider.TypeOpenAICompatible, BaseURL: "https://api.example.test/v1",
		},
	}
	request := Request{Profile: profile, Model: "model", Messages: []Message{{Role: "user", Content: "hello"}}}

	for _, tc := range []struct {
		name         string
		statusCode   int
		contentType  string
		body         string
		roundTripErr error
		want         error
	}{
		{name: "redirect", statusCode: http.StatusFound, want: ErrProviderRejected},
		{name: "429", statusCode: http.StatusTooManyRequests, want: ErrProviderOffline},
		{name: "malformed", statusCode: http.StatusOK, contentType: "application/json", body: `{"choices":[`, want: ErrProviderRejected},
		{name: "offline", roundTripErr: errors.New("connection refused"), want: ErrProviderOffline},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := &http.Client{
				CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					if tc.roundTripErr != nil {
						return nil, tc.roundTripErr
					}
					header := http.Header{}
					if tc.contentType != "" {
						header.Set("Content-Type", tc.contentType)
					}
					return &http.Response{StatusCode: tc.statusCode, Header: header, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
				}),
			}
			_, err := Complete(context.Background(), client, request)
			if !errors.Is(err, tc.want) {
				t.Fatalf("Complete() error = %v, want %v", err, tc.want)
			}
		})
	}
}

// Test: HTTPClient type satisfies Completer and delegates to Complete.
// Requirements: M8-R11.
func TestM8ModelchatHTTPClientCompleter(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"ok"}}]}`)),
		}, nil
	})}
	response, err := NewHTTPClient().Complete(context.Background(), client, Request{
		Profile: provider.ResolvedProfile{
			Profile: provider.Profile{ID: "hosted", Type: provider.TypeOpenAICompatible, BaseURL: "https://api.example.test/v1"},
		},
		Model:    "model",
		Messages: []Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if response.Content != "ok" {
		t.Fatalf("content = %q", response.Content)
	}
}
