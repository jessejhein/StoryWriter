package agent

import (
	"net/http"
	"strings"
)

func newOpenAICompatibleGenerator(client *http.Client) *HTTPGenerator {
	return &HTTPGenerator{client: client, messages: func(request GenerateRequest) ([]ChatMessage, *float64) {
		messages, temperature, err := generateMessagesWithTemperature(request)
		if err != nil {
			return []ChatMessage{{Role: "user", Content: userPrompt(request)}}, &request.Style.Temperature
		}
		if len(messages) > 0 && messages[0].Role == "system" {
			messages[0].Content = strings.TrimSpace(messages[0].Content)
		}
		return messages, temperature
	}}
}
