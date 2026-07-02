package agent

import (
	"net/http"
	"strings"
)

func newOpenAICompatibleGenerator(client *http.Client) *HTTPGenerator {
	return &HTTPGenerator{client: client, messages: func(request GenerateRequest) ([]ChatMessage, *float64) {
		temperature := request.Style.Temperature
		return []ChatMessage{
			{Role: "system", Content: strings.TrimSpace(request.Style.SystemPrompt)},
			{Role: "user", Content: userPrompt(request)},
		}, &temperature
	}}
}
