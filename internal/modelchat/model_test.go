package modelchat

// BDD Scenario: 8.3.1 - Run only after explicit authorization
// Requirements: M8-R11
// Test purpose: Prove neutral modelchat types and errors before transport moves.

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"storywork/internal/provider"
)

type stubCompleter struct {
	response Response
	err      error
}

func (s stubCompleter) Complete(context.Context, *http.Client, Request) (Response, error) {
	return s.response, s.err
}

// Test: message roles and content remain explicit string fields.
// Requirements: M8-R11.
func TestM8ModelMessageRolesAndContent(t *testing.T) {
	t.Parallel()

	message := Message{Role: "system", Content: "Stay neutral."}
	if message.Role != "system" || message.Content != "Stay neutral." {
		t.Fatalf("message = %#v", message)
	}
}

// Test: request carries resolved profile, model, messages, and optional temperature.
// Requirements: M8-R11.
func TestM8ModelRequestCarriesResolvedProfileAndMessages(t *testing.T) {
	t.Parallel()

	temp := 0.2
	request := Request{
		Profile: provider.ResolvedProfile{
			Profile: provider.Profile{
				ID: "hosted", Type: provider.TypeOpenAICompatible, BaseURL: "https://api.example.test/v1",
			},
		},
		Model: "model-name",
		Messages: []Message{
			{Role: "system", Content: "System"},
			{Role: "user", Content: "User"},
		},
		Temperature: &temp,
	}
	if request.Profile.ID != "hosted" || request.Model != "model-name" || len(request.Messages) != 2 {
		t.Fatalf("request = %#v", request)
	}
	if request.Temperature == nil || *request.Temperature != 0.2 {
		t.Fatalf("temperature = %#v", request.Temperature)
	}
}

// Test: provider identity records profile, type, and model for consumers.
// Requirements: M8-R11.
func TestM8ModelProviderIdentityFields(t *testing.T) {
	t.Parallel()

	identity := ProviderIdentity{
		ProfileID: "local_ollama",
		Type:      provider.TypeOllama,
		Model:     "ollama-model",
	}
	if identity.ProfileID != "local_ollama" || identity.Type != provider.TypeOllama || identity.Model != "ollama-model" {
		t.Fatalf("identity = %#v", identity)
	}
}

// Test: neutral provider errors are distinct sentinels for consumer mapping.
// Requirements: M8-R11.
func TestM8ModelProviderErrorsAreDistinctSentinels(t *testing.T) {
	t.Parallel()

	if errors.Is(ErrProviderInvalid, ErrProviderRejected) || errors.Is(ErrProviderInvalid, ErrProviderOffline) {
		t.Fatal("invalid must not match other provider errors")
	}
	if errors.Is(ErrProviderRejected, ErrProviderOffline) {
		t.Fatal("rejected must not match offline")
	}
}

// Test: Completer is a narrow completion contract without profile resolution.
// Requirements: M8-R11.
func TestM8ModelCompleterContract(t *testing.T) {
	t.Parallel()

	var _ Completer = stubCompleter{}
	response, err := stubCompleter{
		response: Response{
			Content: "ok",
			Provider: ProviderIdentity{
				ProfileID: "hosted",
				Type:      provider.TypeOpenAICompatible,
				Model:     "model",
			},
		},
	}.Complete(context.Background(), nil, Request{})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if response.Content != "ok" || response.Provider.ProfileID != "hosted" {
		t.Fatalf("response = %#v", response)
	}
}
