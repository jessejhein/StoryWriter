package modelchat

import (
	"context"
	"fmt"
	"net/http"

	"storywork/internal/provider"
)

// Message is one chat turn sent to a provider.
type Message struct {
	Role    string
	Content string
}

// Request is a resolved-profile chat completion call. Callers must resolve
// profiles before constructing a request; this package does not resolve them.
type Request struct {
	Profile     provider.ResolvedProfile
	Model       string
	Messages    []Message
	Temperature *float64
}

// Response is a successful provider completion with stable identity metadata.
type Response struct {
	Content  string
	Provider ProviderIdentity
}

// ProviderIdentity records which profile and model produced a completion.
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

// Completer performs provider-neutral chat completion for a resolved profile.
type Completer interface {
	Complete(context.Context, *http.Client, Request) (Response, error)
}
