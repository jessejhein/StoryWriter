package importer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	chunkAlgorithmVersion = 1
	maxChunkBytes         = 8000
)

type Chunk struct {
	ID         string `json:"id"`
	ImportID   string `json:"import_id"`
	SourcePath string `json:"source_path"`
	StartLine  int    `json:"start_line"`
	EndLine    int    `json:"end_line"`
	Text       string `json:"text"`
	SHA256     string `json:"sha256"`
}

func ChunkMarkdown(importID, sourcePath, text string) ([]Chunk, error) {
	if err := ValidateImportID(importID); err != nil {
		return nil, err
	}
	normalizedPath, err := NormalizePortableRelativePath(sourcePath)
	if err != nil {
		return nil, err
	}
	if text == "" {
		return []Chunk{}, nil
	}
	lines := splitMarkdownLines(text)
	if len(lines) == 0 {
		return []Chunk{}, nil
	}

	chunks := make([]Chunk, 0, len(lines))
	for start := 0; start < len(lines); {
		end := start
		size := 0
		for end < len(lines) && size+len(lines[end].bytes) <= maxChunkBytes {
			size += len(lines[end].bytes)
			end++
		}
		if end == len(lines) {
			chunk, err := buildChunk(importID, normalizedPath, lines[start:end])
			if err != nil {
				return nil, err
			}
			chunks = append(chunks, chunk)
			break
		}
		split := preferredChunkSplit(lines, start, end)
		if split == start {
			split = start + 1
		}
		chunk, err := buildChunk(importID, normalizedPath, lines[start:split])
		if err != nil {
			return nil, err
		}
		chunks = append(chunks, chunk)
		start = split
	}
	return chunks, nil
}

type markdownLine struct {
	lineNumber int
	bytes      []byte
	text       string
}

func splitMarkdownLines(text string) []markdownLine {
	raw := []byte(text)
	lines := make([]markdownLine, 0, strings.Count(text, "\n")+1)
	start := 0
	lineNumber := 1
	for start < len(raw) {
		end := start
		for end < len(raw) && raw[end] != '\n' {
			end++
		}
		if end < len(raw) && raw[end] == '\n' {
			end++
		}
		lineBytes := append([]byte(nil), raw[start:end]...)
		lines = append(lines, markdownLine{
			lineNumber: lineNumber,
			bytes:      lineBytes,
			text:       string(lineBytes),
		})
		lineNumber++
		start = end
	}
	return lines
}

func preferredChunkSplit(lines []markdownLine, start, end int) int {
	for index := end - 1; index > start; index-- {
		if isATXHeading(lines[index].text) {
			return index
		}
	}
	for index := end - 1; index >= start; index-- {
		if isBlankLine(lines[index].text) {
			return index + 1
		}
	}
	return end
}

func isATXHeading(line string) bool {
	trimmed := strings.TrimSuffix(line, "\n")
	hashes := 0
	for hashes < len(trimmed) && hashes < 6 && trimmed[hashes] == '#' {
		hashes++
	}
	if hashes == 0 {
		return false
	}
	return hashes == len(trimmed) || trimmed[hashes] == ' '
}

func isBlankLine(line string) bool {
	return strings.TrimSpace(line) == ""
}

func buildChunk(importID, sourcePath string, lines []markdownLine) (Chunk, error) {
	if len(lines) == 0 {
		return Chunk{}, fmt.Errorf("chunk requires at least one line")
	}
	var builder strings.Builder
	byteCount := 0
	for _, line := range lines {
		builder.Write(line.bytes)
		byteCount += len(line.bytes)
	}
	text := builder.String()
	startLine := lines[0].lineNumber
	endLine := lines[len(lines)-1].lineNumber
	digestSource := fmt.Sprintf("v%d\n%s\n%s\n%d\n%d\n%s", chunkAlgorithmVersion, importID, sourcePath, startLine, endLine, text)
	digest := sha256.Sum256([]byte(digestSource))
	contentDigest := sha256.Sum256([]byte(text))
	return Chunk{
		ID:         "chk_" + hex.EncodeToString(digest[:10]),
		ImportID:   importID,
		SourcePath: sourcePath,
		StartLine:  startLine,
		EndLine:    endLine,
		Text:       text,
		SHA256:     hex.EncodeToString(contentDigest[:]),
	}, nil
}
