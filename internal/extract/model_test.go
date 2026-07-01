package extract

import "testing"

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
	} {
		if _, err := ParseResponse(invalid); err == nil {
			t.Fatalf("ParseResponse(%s) error = nil", string(invalid))
		}
	}
}
