package agent

import (
	"encoding/json"
	"net/http"
	"strings"
)

func newOllamaGenerator(client *http.Client) *HTTPGenerator {
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	return &HTTPGenerator{client: client, path: "/api/chat", body: func(request GenerateRequest) ([]byte, error) {
		payload := struct {
			Model    string    `json:"model"`
			Messages []message `json:"messages"`
			Stream   bool      `json:"stream"`
			Options  struct {
				Temperature float64 `json:"temperature"`
			} `json:"options"`
		}{
			Model: request.Style.Model, Messages: []message{{Role: "system", Content: strings.TrimSpace(request.Style.SystemPrompt)}, {Role: "user", Content: userPrompt(request)}},
		}
		payload.Options.Temperature = request.Style.Temperature
		return json.Marshal(payload)
	}, decoder: func(body []byte) (string, error) {
		var response struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}
		if err := decodeStrictJSON(body, &response); err != nil {
			return "", err
		}
		return response.Message.Content, nil
	}}
}
