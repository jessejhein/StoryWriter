package agent

// generate_messages.go builds provider chat messages from legacy and typed packets.

import (
	"fmt"

	"storywork/internal/contextpack"
)

func generateMessages(request GenerateRequest) ([]ChatMessage, error) {
	if request.TypedPacket != nil {
		return NewMessageBuilder().BuildMessages(request.Agent, request.Style, request.TypedPacket)
	}
	return []ChatMessage{
		{Role: "system", Content: request.Style.SystemPrompt},
		{Role: "user", Content: userPrompt(request)},
	}, nil
}

func generateMessagesWithTemperature(request GenerateRequest) ([]ChatMessage, *float64, error) {
	messages, err := generateMessages(request)
	if err != nil {
		return nil, nil, err
	}
	temperature := request.Style.Temperature
	return messages, &temperature, nil
}

// GenerateRequest carries provider input for selection and Milestone 7 scopes.
type generateRequestValidator interface {
	Scope() contextpack.Scope
}

func validateGenerateRequest(request GenerateRequest) error {
	if request.TypedPacket == nil && request.Packet.SelectedText == "" {
		return fmt.Errorf("provider packet is required: %w", ErrInvalidAgent)
	}
	return nil
}
