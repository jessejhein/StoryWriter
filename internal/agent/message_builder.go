package agent

// message_builder.go formats scope-specific provider messages from context packets.

import (
	"encoding/json"
	"fmt"
	"strings"

	"storywork/internal/contextpack"
)

// MessageBuilder formats provider-neutral chat messages from typed packets.
type MessageBuilder struct{}

// NewMessageBuilder returns the production message builder.
func NewMessageBuilder() *MessageBuilder {
	return &MessageBuilder{}
}

// BuildMessages renders scope/output-specific provider messages.
func (b *MessageBuilder) BuildMessages(agentDefinition Agent, style Style, packet contextpack.Packet) ([]ChatMessage, error) {
	if packet == nil {
		return nil, fmt.Errorf("packet is required: %w", ErrInvalidAgent)
	}
	if packet.Scope() != scopeForOutput(agentDefinition) {
		return nil, fmt.Errorf("packet scope mismatch: %w", ErrInvalidAgent)
	}
	switch agentDefinition.Output.Type {
	case OutputTypeReplacementText:
		selection, ok := packet.(contextpack.SelectionPacket)
		if !ok {
			return nil, fmt.Errorf("packet type mismatch: %w", ErrInvalidAgent)
		}
		return selectionMessages(agentDefinition, style, selection), nil
	case OutputTypeRevisedText:
		scene, ok := packet.(contextpack.ScenePacket)
		if !ok {
			return nil, fmt.Errorf("packet type mismatch: %w", ErrInvalidAgent)
		}
		return sceneMessages(agentDefinition, style, scene), nil
	case OutputTypeEditorialFindings:
		chapter, ok := packet.(contextpack.ChapterReviewPacket)
		if !ok {
			return nil, fmt.Errorf("packet type mismatch: %w", ErrInvalidAgent)
		}
		return chapterReviewMessages(agentDefinition, style, chapter), nil
	default:
		return nil, fmt.Errorf("output type %q is unsupported: %w", agentDefinition.Output.Type, ErrInvalidAgent)
	}
}

func scopeForOutput(agentDefinition Agent) contextpack.Scope {
	switch agentDefinition.Output.Type {
	case OutputTypeReplacementText:
		return contextpack.ScopeSelection
	case OutputTypeRevisedText:
		return contextpack.ScopeScene
	case OutputTypeEditorialFindings:
		return contextpack.ScopeChapterReview
	default:
		return ""
	}
}

func selectionMessages(agentDefinition Agent, style Style, packet contextpack.SelectionPacket) []ChatMessage {
	return []ChatMessage{
		{Role: "system", Content: style.SystemPrompt},
		{Role: "user", Content: "Task: " + agentDefinition.Description + "\n\nRewrite only the selected text. Return replacement text only. Do not add commentary or Markdown fences.\n\nSelected text:\n" + packet.SelectedText},
	}
}

func sceneMessages(agentDefinition Agent, style Style, packet contextpack.ScenePacket) []ChatMessage {
	var builder strings.Builder
	builder.WriteString("Task: ")
	builder.WriteString(agentDefinition.Description)
	builder.WriteString("\n\nRewrite the full scene Markdown body. Return replacement prose only.\n\nScene:\n")
	builder.WriteString(packet.SceneMarkdown)
	if len(packet.ActiveCodex) > 0 {
		builder.WriteString("\n\nActive facts:\n")
		for _, entry := range packet.ActiveCodex {
			builder.WriteString("- ")
			builder.WriteString(entry.Name)
			builder.WriteString(": ")
			builder.WriteString(entry.Description)
			builder.WriteString("\n")
		}
	}
	for _, neighbor := range packet.OutlineNeighbors {
		builder.WriteString("\nNeighbor ")
		builder.WriteString(neighbor.Kind)
		builder.WriteString(" ")
		builder.WriteString(neighbor.ID)
		builder.WriteString(":\n")
		builder.WriteString(neighbor.Text)
	}
	return []ChatMessage{
		{Role: "system", Content: style.SystemPrompt},
		{Role: "user", Content: builder.String()},
	}
}

func chapterReviewMessages(agentDefinition Agent, style Style, packet contextpack.ChapterReviewPacket) []ChatMessage {
	var builder strings.Builder
	builder.WriteString("Task: ")
	builder.WriteString(agentDefinition.Description)
	builder.WriteString("\n\nReturn one strict JSON object with editorial findings only.\n\nChapter scenes:\n")
	for _, scene := range packet.ChapterScenes {
		builder.WriteString("Scene ")
		builder.WriteString(scene.SceneID)
		builder.WriteString(":\n")
		builder.WriteString(scene.Markdown)
		builder.WriteString("\n")
	}
	schema, _ := json.Marshal(map[string]any{
		"findings": []map[string]any{{
			"title": "string", "explanation": "string",
			"scene_ids": []string{"scn_..."}, "follow_up_agent_ids": []string{"scene_rewrite"},
		}},
	})
	builder.WriteString("\nRequired JSON shape:\n")
	builder.Write(schema)
	return []ChatMessage{
		{Role: "system", Content: style.SystemPrompt},
		{Role: "user", Content: builder.String()},
	}
}