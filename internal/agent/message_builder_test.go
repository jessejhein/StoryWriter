// BDD Scenario: 7.2.3 - Review and accept one scene replacement
// Requirements: M7-R03
// Test purpose: Provider messages include only manifest-approved packet content.

package agent

import (
	"strings"
	"testing"

	"storywork/internal/contextpack"
)

// Test: selection messages format replacement output exactly.
// Requirements: M7-R02.
func TestMessageBuilderFormatsSelectionReplacementExactly(t *testing.T) {
	t.Parallel()

	builder := NewMessageBuilder()
	messages, err := builder.BuildMessages(linePolishAgent(), preciseEditorStyle(), contextpack.SelectionPacket{
		SelectedText: "Alpha beta",
		Style:        contextpack.StyleSheet{ID: "precise_editor", SystemPrompt: "Prompt"},
	})
	if err != nil {
		t.Fatalf("BuildMessages() error = %v", err)
	}
	if len(messages) != 2 || !strings.Contains(messages[1].Content, "Alpha beta") {
		t.Fatalf("messages = %#v", messages)
	}
}

// Test: scene messages format revised text exactly.
// Requirements: M7-R03.
func TestMessageBuilderFormatsSceneReplacementExactly(t *testing.T) {
	t.Parallel()

	builder := NewMessageBuilder()
	messages, err := builder.BuildMessages(sceneRewriteAgent(), preciseEditorStyle(), contextpack.ScenePacket{
		SceneMarkdown: "Ann arrives.\n",
		Style:         contextpack.StyleSheet{ID: "precise_editor", SystemPrompt: "Prompt"},
		ActiveCodex: []contextpack.CodexEntryState{{
			EntryID: "char_0123456789abcdef0123", Name: "Ann", Description: "Pilot.",
		}},
	})
	if err != nil {
		t.Fatalf("BuildMessages() error = %v", err)
	}
	if !strings.Contains(messages[1].Content, "Ann arrives.") || !strings.Contains(messages[1].Content, "Pilot.") {
		t.Fatalf("messages = %#v", messages)
	}
}

// Test: chapter review messages request strict findings JSON.
// Requirements: M7-R04.
func TestMessageBuilderFormatsChapterReviewExactly(t *testing.T) {
	t.Parallel()

	builder := NewMessageBuilder()
	messages, err := builder.BuildMessages(chapterReviewAgent(), preciseEditorStyle(), contextpack.ChapterReviewPacket{
		ChapterID:     "ch_0123456789abcdef0123",
		ChapterScenes: []contextpack.ChapterSceneText{{SceneID: "scn_0123456789abcdef0123", Markdown: "Scene.\n"}},
		Style:         contextpack.StyleSheet{ID: "precise_editor", SystemPrompt: "Prompt"},
	})
	if err != nil {
		t.Fatalf("BuildMessages() error = %v", err)
	}
	if !strings.Contains(messages[1].Content, "findings") || !strings.Contains(messages[1].Content, "Scene.\n") {
		t.Fatalf("messages = %#v", messages)
	}
}

// Test: messages include only packet fields present in the built packet.
// Requirements: M7-R10.
func TestMessageBuilderIncludesOnlyManifestApprovedPacks(t *testing.T) {
	t.Parallel()

	builder := NewMessageBuilder()
	messages, err := builder.BuildMessages(linePolishAgent(), preciseEditorStyle(), contextpack.SelectionPacket{
		SelectedText: "Alpha beta", Style: contextpack.StyleSheet{SystemPrompt: "Prompt"},
	})
	if err != nil {
		t.Fatalf("BuildMessages() error = %v", err)
	}
	content := messages[1].Content
	if strings.Contains(content, "codex") || strings.Contains(content, "outline") {
		t.Fatalf("selection message leaked wider context: %q", content)
	}
}

// Test: message builder rejects packet/output mismatches.
// Requirements: M7-R03.
func TestMessageBuilderRejectsPacketOutputMismatch(t *testing.T) {
	t.Parallel()

	builder := NewMessageBuilder()
	if _, err := builder.BuildMessages(linePolishAgent(), preciseEditorStyle(), contextpack.ScenePacket{SceneMarkdown: "x"}); err == nil {
		t.Fatal("packet mismatch error = nil, want failure")
	}
}

func linePolishAgent() Agent {
	return Agent{
		Version: 1, ID: "line_polish", Description: "Rewrite selected prose.",
		Control: Control{OutputMode: OutputModePatch},
		Output:  Output{Type: OutputTypeReplacementText},
	}
}

func sceneRewriteAgent() Agent {
	return Agent{
		Version: 3, ID: "scene_rewrite", Description: "Rewrite one scene.",
		Control: Control{OutputMode: OutputModePatch},
		Output:  Output{Type: OutputTypeRevisedText},
	}
}

func chapterReviewAgent() Agent {
	return Agent{
		Version: 3, ID: "chapter_review", Description: "Review one chapter.",
		Control: Control{OutputMode: OutputModeSuggestion},
		Output:  Output{Type: OutputTypeEditorialFindings},
	}
}

func preciseEditorStyle() Style {
	return Style{ID: "precise_editor", SystemPrompt: "Prompt"}
}
