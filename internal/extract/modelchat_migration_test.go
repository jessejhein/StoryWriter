package extract

// BDD Scenario: 8.3.1 - Run only after explicit authorization
// Requirements: M8-R11
// Test purpose: Prove extraction consumes modelchat directly without agent chat transport.

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// Test: extraction package no longer imports internal/agent for chat transport.
// Requirements: M8-R11.
func TestM8ExtractNoLongerImportsAgentChatTransport(t *testing.T) {
	t.Parallel()

	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "model.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("ParseFile(model.go) error = %v", err)
	}
	for _, importSpec := range file.Imports {
		path := strings.Trim(importSpec.Path.Value, `"`)
		if path == "storywork/internal/agent" {
			t.Fatalf("extract/model.go still imports %s", path)
		}
	}
}
