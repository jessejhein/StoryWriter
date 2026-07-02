package extract

import (
	"errors"
	"testing"
)

func TestValidateRequestRejectsUnknownModeAndOversizedPayload(t *testing.T) {
	t.Parallel()

	if err := ValidateRequest(Request{Mode: "bad", ProfileID: "p", Model: "m", Chunks: []Chunk{{ID: "chk", ImportID: "imp", SourcePath: "notes.md", StartLine: 1, EndLine: 1, Text: "Alpha"}}}); err == nil {
		t.Fatal("ValidateRequest(unknown mode) error = nil")
	}

	oversized := make([]byte, (200<<10)+1)
	if err := ValidateRequest(Request{Mode: ModeStructure, ProfileID: "p", Model: "m", Chunks: []Chunk{{ID: "chk", ImportID: "imp", SourcePath: "notes.md", StartLine: 1, EndLine: 1, Text: string(oversized)}}}); err == nil {
		t.Fatal("ValidateRequest(oversized payload) error = nil")
	}
}

func TestValidateRequestRejectsMalformedChunkMetadata(t *testing.T) {
	t.Parallel()

	valid := Request{Mode: ModeStructure, ProfileID: "p", Model: "m", Chunks: []Chunk{{
		ID: "chk_0123456789abcdef0123", ImportID: "imp_0123456789abcdef0123",
		SourcePath: "notes.md", StartLine: 1, EndLine: 1, Text: "Alpha",
	}}}
	tests := []struct {
		name   string
		mutate func(*Request)
	}{
		{name: "duplicate chunk", mutate: func(request *Request) { request.Chunks = append(request.Chunks, request.Chunks[0]) }},
		{name: "mixed imports", mutate: func(request *Request) {
			request.Chunks = append(request.Chunks, Chunk{ID: "chk_abcdef0123456789abcd", ImportID: "imp_abcdef0123456789abcd", SourcePath: "other.md", StartLine: 1, EndLine: 1, Text: "Other"})
		}},
		{name: "invalid lines", mutate: func(request *Request) { request.Chunks[0].StartLine = 0 }},
		{name: "empty text", mutate: func(request *Request) { request.Chunks[0].Text = "" }},
	}
	for _, testCase := range tests {
		request := valid
		request.Chunks = append([]Chunk(nil), valid.Chunks...)
		testCase.mutate(&request)
		if err := ValidateRequest(request); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("%s error = %v, want %v", testCase.name, err, ErrInvalidRequest)
		}
	}
}

func TestParseResponseRequiresStrictSingleObject(t *testing.T) {
	t.Parallel()

	valid := []byte(`{"candidates":[{"kind":"arc","local_id":"arc_local","title":"Act One"}]}`)
	proposals, err := ParseResponse(valid)
	if err != nil {
		t.Fatalf("ParseResponse(valid) error = %v", err)
	}
	if len(proposals) != 1 || proposals[0].Arc == nil || proposals[0].Arc.LocalID != "arc_local" {
		t.Fatalf("ParseResponse(valid) = %+v", proposals)
	}

	for _, invalid := range [][]byte{
		[]byte(""),
		[]byte("```json\n{}\n```"),
		[]byte(`{"candidates":[]}`),
		[]byte(`{"candidates":[{"kind":"arc","local_id":"dup","title":"Act One"},{"kind":"arc","local_id":"dup","title":"Act Two"}]}`),
		[]byte(`{"candidates":[{"kind":"arc","local_id":"","title":"Act One"}]}`),
		[]byte(`{"candidates":[{"kind":"arc","local_id":"arc_local","title":""}]}`),
		[]byte(`{"candidates":[{"kind":"chapter","local_id":"chapter_local","title":"Chapter","parent_local_id":"missing"}]}`),
		[]byte(`{"candidates":[{"kind":"arc","local_id":"arc_local","title":"Act One","unknown":true}]}`),
	} {
		if _, err := ParseResponse(invalid); err == nil {
			t.Fatalf("ParseResponse(%s) error = nil", string(invalid))
		}
	}
}
