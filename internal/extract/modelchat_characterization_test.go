package extract

// BDD Scenario: 8.3.1 - Run only after explicit authorization
// Requirements: M8-R11
// Test purpose: Lock extraction prompt and provider result behavior through the
// current shared chat transport before it moves to internal/modelchat.

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"storywork/internal/modelchat"
	"storywork/internal/provider"
)

// Test: structure extraction still builds provider-neutral prompts and parses results.
// Requirements: M8-R11.
func TestM8ExtractionCharacterizationThroughCurrentChat(t *testing.T) {
	t.Parallel()

	var capturedPath string
	var capturedAuth string
	var capturedBody string
	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
			capturedPath = request.URL.Path
			capturedAuth = request.Header.Get("Authorization")
			body, err := io.ReadAll(request.Body)
			if err != nil {
				t.Fatalf("ReadAll(body) error = %v", err)
			}
			capturedBody = string(body)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(
					`{"choices":[{"message":{"content":"{\"candidates\":[{\"kind\":\"codex\",\"local_id\":\"char_alpha\",\"type\":\"character\",\"name\":\"Alpha\",\"aliases\":[],\"tags\":[],\"description\":\"Lead character\"}]}"}}]}`,
				)),
			}, nil
		}),
	}

	extractor := NewRemoteExtractor(resolvedProfileStub{
		profile: provider.ResolvedProfile{
			Profile: provider.Profile{
				ID: "hosted", Type: provider.TypeOpenAICompatible, BaseURL: "https://api.example.test/v1",
				Auth:         provider.AuthConfig{Type: provider.AuthTypeBearerEnv, CredentialEnv: "KEY"},
				Capabilities: provider.Capabilities{Chat: true, MaxContextTokens: 8192},
				Readiness:    provider.ReadinessReady,
			},
			Credential: provider.Credential{Value: "extract-key"},
		},
		found: true,
	}, client)

	result, err := extractor.Extract(context.Background(), Request{
		Mode:      ModeStructure,
		ProfileID: "hosted",
		Model:     "structure-model",
		Chunks: []Chunk{{
			ID: "chk_0123456789abcdef0123", ImportID: "imp_0123456789abcdef0123",
			SourcePath: "notes/alpha.md", StartLine: 1, EndLine: 3, Text: "Alpha note text",
		}},
	})
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if len(result.Proposals) != 1 || result.Proposals[0].Kind != "codex" || result.Proposals[0].Codex == nil {
		t.Fatalf("proposals = %+v", result.Proposals)
	}
	if result.Proposals[0].Codex.LocalID != "char_alpha" || result.Proposals[0].Codex.Name != "Alpha" {
		t.Fatalf("codex proposal = %+v", result.Proposals[0].Codex)
	}
	if result.Provider.ProfileID != "hosted" || result.Provider.Type != provider.TypeOpenAICompatible || result.Provider.Model != "structure-model" {
		t.Fatalf("provider identity = %#v", result.Provider)
	}
	if !strings.HasSuffix(capturedPath, "/chat/completions") {
		t.Fatalf("request path = %q, want suffix /chat/completions", capturedPath)
	}
	if capturedAuth != "Bearer extract-key" {
		t.Fatalf("authorization = %q", capturedAuth)
	}

	var payload struct {
		Model    string `json:"model"`
		Stream   bool   `json:"stream"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal([]byte(capturedBody), &payload); err != nil {
		t.Fatalf("Unmarshal(body) error = %v, body = %s", err, capturedBody)
	}
	if payload.Model != "structure-model" || payload.Stream != false {
		t.Fatalf("payload = %+v", payload)
	}
	if len(payload.Messages) != 2 || payload.Messages[0].Role != "system" || payload.Messages[1].Role != "user" {
		t.Fatalf("messages = %+v", payload.Messages)
	}
	systemPrompt := payload.Messages[0].Content
	userPrompt := payload.Messages[1].Content
	if systemPrompt != "Extract structured story candidates from imported markdown notes." {
		t.Fatalf("system prompt = %q", systemPrompt)
	}
	if !strings.Contains(userPrompt, `Return exactly one JSON object with shape {"candidates":[...]}`) {
		t.Fatalf("user prompt missing extraction envelope instruction: %s", userPrompt)
	}
	if !strings.Contains(userPrompt, "Alpha note text") {
		t.Fatalf("user prompt missing chunk text: %s", userPrompt)
	}
	if strings.Contains(userPrompt, "Rewrite only the selected text") {
		t.Fatalf("extraction reused action selection prompt: %s", userPrompt)
	}
}

// Test: extraction readiness failures still reject before chat transport runs.
// Requirements: M8-R11.
func TestM8ExtractionCharacterizationRejectsBeforeChat(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("unexpected outbound chat request")
		return nil, nil
	})}
	extractor := NewRemoteExtractor(resolvedProfileStub{
		profile: provider.ResolvedProfile{
			Profile: provider.Profile{
				ID: "hosted", Type: provider.TypeOpenAICompatible, BaseURL: "https://api.example.test/v1",
				Capabilities: provider.Capabilities{Chat: false, MaxContextTokens: 8192},
				Readiness:    provider.ReadinessReady,
			},
		},
		found: true,
	}, client)

	_, err := extractor.Extract(context.Background(), Request{
		Mode: ModeStructure, ProfileID: "hosted", Model: "structure-model",
		Chunks: []Chunk{{
			ID: "chk_0123456789abcdef0123", ImportID: "imp_0123456789abcdef0123",
			SourcePath: "notes/alpha.md", StartLine: 1, EndLine: 3, Text: "Alpha note text",
		}},
	})
	if err == nil {
		t.Fatal("Extract() error = nil, want provider invalid")
	}
	if !strings.Contains(err.Error(), modelchat.ErrProviderInvalid.Error()) {
		t.Fatalf("Extract() error = %v, want provider invalid", err)
	}
}
