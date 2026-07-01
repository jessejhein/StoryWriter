package importer

import (
	"slices"
	"testing"
)

func TestChunkMarkdownSplitsDeterministicallyAtHeadingAndBlankBoundaries(t *testing.T) {
	t.Parallel()

	text := "# One\nAlpha\n\n# Two\nBeta\n\n# Three\nGamma\n"
	chunks, err := ChunkMarkdown("imp_0123456789abcdef0123", "notes/example.md", text)
	if err != nil {
		t.Fatalf("ChunkMarkdown() error = %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("ChunkMarkdown() chunk count = %d, want 1", len(chunks))
	}
	if chunks[0].StartLine != 1 || chunks[0].EndLine != 8 {
		t.Fatalf("chunk lines = %d..%d", chunks[0].StartLine, chunks[0].EndLine)
	}
	if chunks[0].Text != text {
		t.Fatalf("chunk text = %q", chunks[0].Text)
	}
}

func TestChunkMarkdownHandlesOversizedLinesWithoutSplittingRunesOrLines(t *testing.T) {
	t.Parallel()

	longLine := slices.Repeat([]byte("a"), maxChunkBytes+100)
	text := string(longLine) + "\n# Next\n"
	chunks, err := ChunkMarkdown("imp_0123456789abcdef0123", "notes/long.md", text)
	if err != nil {
		t.Fatalf("ChunkMarkdown() error = %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("ChunkMarkdown() chunk count = %d, want 2", len(chunks))
	}
	if chunks[0].StartLine != 1 || chunks[0].EndLine != 1 {
		t.Fatalf("oversized line chunk lines = %d..%d", chunks[0].StartLine, chunks[0].EndLine)
	}
	if chunks[1].StartLine != 2 || chunks[1].EndLine != 2 {
		t.Fatalf("second chunk lines = %d..%d", chunks[1].StartLine, chunks[1].EndLine)
	}
}
