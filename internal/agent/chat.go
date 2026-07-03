package agent

import (
	"context"
	"net/http"

	"storywork/internal/modelchat"
)

// ChatMessage is a compatibility alias for modelchat.Message.
type ChatMessage = modelchat.Message

// ChatRequest is a compatibility alias for modelchat.Request.
type ChatRequest = modelchat.Request

// ChatResponse is a compatibility alias for modelchat.Response.
type ChatResponse = modelchat.Response

// CompleteChat completes a chat request using the shared modelchat transport.
func CompleteChat(ctx context.Context, client *http.Client, request ChatRequest) (ChatResponse, error) {
	return modelchat.Complete(ctx, client, request)
}
