package agent

import (
	"encoding/json"
	"net/http"
	"strings"
)

func newOpenAICompatibleGenerator(client *http.Client) *HTTPGenerator {
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	return &HTTPGenerator{client: client, path: "/chat/completions", body: func(request GenerateRequest) ([]byte, error) {
		return json.Marshal(struct {
			Model       string    `json:"model"`
			Messages    []message `json:"messages"`
			Temperature float64   `json:"temperature"`
			Stream      bool      `json:"stream"`
		}{
			Model: request.Style.Model, Messages: []message{{Role: "system", Content: strings.TrimSpace(request.Style.SystemPrompt)}, {Role: "user", Content: userPrompt(request)}}, Temperature: request.Style.Temperature,
		})
	}, decoder: func(body []byte) (string, error) {
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
	}}
}
