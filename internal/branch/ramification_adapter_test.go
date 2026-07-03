// BDD Scenario: 8.3.1 - Run only after explicit authorization
// Requirements: M8-R09, M8-R11
// Test purpose: Branch analyzer uses modelchat and rejects invalid provider output.

package branch_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"storywork/internal/branch"
	"storywork/internal/modelchat"
	"storywork/internal/provider"
)

type fakeCompleter struct{ content string }

func (f *fakeCompleter) Complete(context.Context, *http.Client, modelchat.Request) (modelchat.Response, error) {
	return modelchat.Response{
		Content:  f.content,
		Provider: modelchat.ProviderIdentity{ProfileID: "local", Type: provider.TypeOllama, Model: "qwen"},
	}, nil
}

// Test: analyzer parses strict JSON through modelchat.
// Requirements: M8-R11.
func TestModelchatAnalyzerParsesStrictFindings(t *testing.T) {
	t.Parallel()
	analyzer := &branch.ModelchatAnalyzer{
		Resolver: func(context.Context, string) (modelchat.Request, error) {
			return modelchat.Request{Profile: provider.ResolvedProfile{Profile: provider.Profile{ID: "local", Type: provider.TypeOllama}}}, nil
		},
		Completer: &fakeCompleter{content: `{"summary":"Review later scenes.","findings":[]}`},
	}
	result, err := analyzer.Analyze(context.Background(), branch.AnalysisPacket{
		Goal:       "test",
		Comparison: branch.Comparison{Files: []branch.ChangedFile{{Path: "outline.yaml", Status: branch.StatusModified}}},
		ProfileID:  "local",
		Model:      "qwen",
	})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if result.Summary == "" {
		t.Fatal("summary is empty")
	}
}

// Test: malformed output is rejected.
// Requirements: M8-R10.
func TestModelchatAnalyzerRejectsMalformedOutput(t *testing.T) {
	t.Parallel()
	analyzer := &branch.ModelchatAnalyzer{
		Resolver: func(context.Context, string) (modelchat.Request, error) {
			return modelchat.Request{Profile: provider.ResolvedProfile{Profile: provider.Profile{ID: "local", Type: provider.TypeOllama}}}, nil
		},
		Completer: &fakeCompleter{content: `not json`},
	}
	_, err := analyzer.Analyze(context.Background(), branch.AnalysisPacket{Goal: "test", ProfileID: "local", Model: "qwen"})
	if !errors.Is(err, branch.ErrInvalidAnalysisOutput) {
		t.Fatalf("err = %v", err)
	}
}
