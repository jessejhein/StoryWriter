// Test purpose: Verify Validator accepts injected StoryReader and RegistryReader
// implementations, enabling unit tests without filesystem fixtures.

package projectcheck_test

import (
	"context"
	"errors"
	"testing"

	"storywork/internal/agent"
	"storywork/internal/codex"
	"storywork/internal/projectcheck"
	"storywork/internal/story"
)

type fakeStoryReader struct {
	outline      story.Outline
	outlineErr   error
	entries      []codex.Entry
	entriesErr   error
	progressions codex.ProgressionDocument
	progErr      error
}

func (f *fakeStoryReader) Load(_ context.Context, _ string) (story.Outline, error) {
	return f.outline, f.outlineErr
}
func (f *fakeStoryReader) LoadCodexEntries(_ context.Context, _ string) ([]codex.Entry, error) {
	return f.entries, f.entriesErr
}
func (f *fakeStoryReader) LoadProgressions(_ context.Context, _, _ string) (codex.ProgressionDocument, error) {
	return f.progressions, f.progErr
}

type fakeRegistryReader struct {
	agents    []agent.Agent
	agentsErr error
	styles    []agent.Style
	stylesErr error
}

func (f *fakeRegistryReader) LoadAgents(_ string) ([]agent.Agent, error) {
	return f.agents, f.agentsErr
}
func (f *fakeRegistryReader) LoadStyles(_ string) ([]agent.Style, error) {
	return f.styles, f.stylesErr
}

func noopMetadata(_ string) (string, string, error) { return "", "", nil }
func noopImports(_ context.Context, _ string) error { return nil }

// Test: NewWithReaders uses injected readers and returns their errors directly.
func TestNewWithReadersUsesInjectedImplementations(t *testing.T) {
	t.Parallel()

	outlineErr := errors.New("injected outline failure")
	files := &fakeStoryReader{
		outlineErr: outlineErr,
	}
	agents := &fakeRegistryReader{}

	v := projectcheck.NewWithReaders(files, agents,
		projectcheck.WithMetadataFunc(noopMetadata),
		projectcheck.WithImportsFunc(noopImports),
	)
	err := v.ValidateProject(context.Background(), t.TempDir())
	if !errors.Is(err, outlineErr) {
		t.Fatalf("ValidateProject() error = %v, want %v", err, outlineErr)
	}
}

// Test: NewWithReaders propagates agent registry errors.
func TestNewWithReadersPropagatesRegistryErrors(t *testing.T) {
	t.Parallel()

	agentsErr := errors.New("injected agent failure")
	files := &fakeStoryReader{
		outline: story.Outline{},
	}
	agents := &fakeRegistryReader{
		agentsErr: agentsErr,
	}

	v := projectcheck.NewWithReaders(files, agents,
		projectcheck.WithMetadataFunc(noopMetadata),
		projectcheck.WithImportsFunc(noopImports),
	)
	err := v.ValidateProject(context.Background(), t.TempDir())
	if !errors.Is(err, agentsErr) {
		t.Fatalf("ValidateProject() error = %v, want %v", err, agentsErr)
	}
}
