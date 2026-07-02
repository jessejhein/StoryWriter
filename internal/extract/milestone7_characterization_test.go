package extract

// BDD Scenario: characterization guard for extraction chat transport
// Requirements: M7-R19
// Test purpose: Prove import extraction still uses provider-neutral chat prompts
// distinct from action selection rewrite messages during Milestone 7 refactors.

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"storywork/internal/provider"
)

type resolvedProfileStub struct {
	profile provider.ResolvedProfile
	found   bool
	err     error
}

func (s resolvedProfileStub) Resolve(context.Context, string) (provider.ResolvedProfile, bool, error) {
	if s.err != nil {
		return provider.ResolvedProfile{}, false, s.err
	}
	return s.profile, s.found, nil
}

// Test: extraction still uses provider-neutral chat prompts, not action rewrite prompts.
// Requirements: M7-R19.
func TestM7ExtractionStillUsesProviderNeutralChat(t *testing.T) {
	t.Parallel()

	var capturedPath string
	var capturedBody string
	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
			capturedPath = request.URL.Path
			body, err := io.ReadAll(request.Body)
			if err != nil {
				t.Fatalf("ReadAll(body) error = %v", err)
			}
			capturedBody = string(body)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(
					`{"choices":[{"message":{"content":"{\"candidates\":[{\"kind\":\"arc\",\"local_id\":\"arc_local\",\"title\":\"Act One\"}]}"}}]}`,
				)),
			}, nil
		}),
	}

	extractor := NewRemoteExtractor(resolvedProfileStub{
		profile: provider.ResolvedProfile{
			Profile: provider.Profile{
				ID: "hosted", Type: provider.TypeOpenAICompatible, BaseURL: "https://api.example.test/v1",
				Auth: provider.AuthConfig{Type: provider.AuthTypeBearerEnv, CredentialEnv: "KEY"},
				Capabilities: provider.Capabilities{Chat: true, MaxContextTokens: 8192},
				Readiness: provider.ReadinessReady,
			},
			Credential: provider.Credential{Value: "test-key"},
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
	if len(result.Proposals) != 1 || result.Proposals[0].Arc == nil {
		t.Fatalf("result proposals = %+v", result.Proposals)
	}
	if !strings.HasSuffix(capturedPath, "/chat/completions") {
		t.Fatalf("request path = %q, want suffix /chat/completions", capturedPath)
	}
	if !strings.Contains(capturedBody, `"messages"`) {
		t.Fatalf("request body missing messages: %s", capturedBody)
	}
	if !strings.Contains(capturedBody, `Return exactly one JSON object with shape {\"candidates\"`) {
		t.Fatalf("request body missing extraction system prompt: %s", capturedBody)
	}
	if !strings.Contains(capturedBody, "Alpha note text") {
		t.Fatalf("request body missing chunk text: %s", capturedBody)
	}
	if strings.Contains(capturedBody, "Rewrite only the selected text") {
		t.Fatalf("extraction request reused action selection prompt: %s", capturedBody)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}