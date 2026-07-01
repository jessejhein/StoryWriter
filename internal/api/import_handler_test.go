package api_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"storywork/internal/agent"
	"storywork/internal/api"
	"storywork/internal/extract"
	"storywork/internal/importer"
	"storywork/internal/story"
)

func TestImportRoutesReturnExactJSONShapes(t *testing.T) {
	t.Parallel()

	stub := &storyServiceStub{
		importResponse: importer.ImportResponse{
			Import: importer.ImportSummary{ID: "imp_0123456789abcdef0123", CreatedAt: "2026-06-30T12:00:00Z", FileCount: 1, TotalBytes: 12},
			Files:  []importer.ImportFile{{Path: "notes/characters.md", Bytes: 12, SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}},
		},
		importList: []importer.ImportSummary{{ID: "imp_0123456789abcdef0123", CreatedAt: "2026-06-30T12:00:00Z", FileCount: 1, TotalBytes: 12}},
		importChunks: []importer.Chunk{{
			ID:         "chk_0123456789abcdef0123",
			ImportID:   "imp_0123456789abcdef0123",
			SourcePath: "notes/characters.md",
			StartLine:  1,
			EndLine:    2,
			Text:       "# Characters\nMara\n",
			SHA256:     "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		}},
		extractResponse: importer.ExtractResponse{
			Candidates: []importer.Candidate{{
				Version:         1,
				ID:              "cand_0123456789abcdef0123",
				Kind:            importer.CandidateKindCodex,
				ProposalVersion: 1,
				Status:          importer.CandidateStatusPending,
				Revision:        "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
				Provenance:      importer.Provenance{ChunkIDs: []string{"chk_0123456789abcdef0123"}},
				Proposal: importer.CandidateProposal{Codex: &importer.CodexProposal{
					Type:        "character",
					Name:        "Mara Venn",
					Aliases:     []string{"Mara"},
					Tags:        []string{"pilot"},
					Description: "A cautious salvage pilot.",
				}},
				Decision: importer.CandidateDecision{CanonicalRefs: []importer.CanonicalRef{}},
			}},
			Provider: agent.ProviderIdentity{ProfileID: "local_ollama", Type: "ollama", Model: "qwen2.5:7b"},
		},
		candidates: []importer.Candidate{{
			Version:         1,
			ID:              "cand_0123456789abcdef0123",
			Kind:            importer.CandidateKindCodex,
			ProposalVersion: 1,
			Status:          importer.CandidateStatusPending,
			Revision:        "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
			Provenance:      importer.Provenance{ChunkIDs: []string{"chk_0123456789abcdef0123"}},
			Proposal: importer.CandidateProposal{Codex: &importer.CodexProposal{
				Type:        "character",
				Name:        "Mara Venn",
				Aliases:     []string{"Mara"},
				Tags:        []string{"pilot"},
				Description: "A cautious salvage pilot.",
			}},
			Decision: importer.CandidateDecision{CanonicalRefs: []importer.CanonicalRef{}},
		}},
		candidate: importer.Candidate{
			Version:         1,
			ID:              "cand_0123456789abcdef0123",
			Kind:            importer.CandidateKindCodex,
			ProposalVersion: 1,
			Status:          importer.CandidateStatusPending,
			Revision:        "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
			Provenance:      importer.Provenance{ChunkIDs: []string{"chk_0123456789abcdef0123"}},
			Proposal: importer.CandidateProposal{Codex: &importer.CodexProposal{
				Type:        "character",
				Name:        "Mara Venn",
				Aliases:     []string{"Mara"},
				Tags:        []string{"pilot"},
				Description: "A cautious salvage pilot.",
			}},
			Decision: importer.CandidateDecision{CanonicalRefs: []importer.CanonicalRef{}},
		},
		updateCandidate: importer.Candidate{
			Version:         1,
			ID:              "cand_0123456789abcdef0123",
			Kind:            importer.CandidateKindCodex,
			ProposalVersion: 1,
			Status:          importer.CandidateStatusPending,
			Revision:        "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
			Provenance:      importer.Provenance{ChunkIDs: []string{"chk_0123456789abcdef0123"}},
			Proposal: importer.CandidateProposal{Codex: &importer.CodexProposal{
				Type:        "character",
				Name:        "Mara Venn",
				Aliases:     []string{"Mara"},
				Tags:        []string{"pilot"},
				Description: "Edited author text.",
			}},
			Decision: importer.CandidateDecision{CanonicalRefs: []importer.CanonicalRef{}},
		},
		mergeCandidate: importer.Candidate{
			Version:         1,
			ID:              "cand_0123456789abcdef0999",
			Kind:            importer.CandidateKindCodex,
			ProposalVersion: 1,
			Status:          importer.CandidateStatusPending,
			Revision:        "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
			Provenance:      importer.Provenance{ChunkIDs: []string{"chk_0123456789abcdef0123", "chk_0123456789abcdef0456"}},
			Proposal: importer.CandidateProposal{Codex: &importer.CodexProposal{
				Type:        "character",
				Name:        "Mara Venn",
				Aliases:     []string{"Mara"},
				Tags:        []string{"pilot"},
				Description: "Merged author text.",
			}},
			Decision: importer.CandidateDecision{CanonicalRefs: []importer.CanonicalRef{}},
		},
		mergeCandidateIDs: []string{"cand_0123456789abcdef0123", "cand_abcdef0123456789abcd"},
		discardCandidate: importer.Candidate{
			Version:         1,
			ID:              "cand_0123456789abcdef0123",
			Kind:            importer.CandidateKindCodex,
			ProposalVersion: 1,
			Status:          importer.CandidateStatusDiscarded,
			Revision:        "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
			Provenance:      importer.Provenance{ChunkIDs: []string{"chk_0123456789abcdef0123"}},
			Proposal: importer.CandidateProposal{Codex: &importer.CodexProposal{
				Type:        "character",
				Name:        "Mara Venn",
				Aliases:     []string{"Mara"},
				Tags:        []string{"pilot"},
				Description: "A cautious salvage pilot.",
			}},
			Decision: importer.CandidateDecision{CanonicalRefs: []importer.CanonicalRef{}},
		},
		acceptCandidate: importer.Candidate{
			Version:         1,
			ID:              "cand_0123456789abcdef0123",
			Kind:            importer.CandidateKindCodex,
			ProposalVersion: 1,
			Status:          importer.CandidateStatusAccepted,
			Revision:        "sha256:1111111111111111111111111111111111111111111111111111111111111111",
			Provenance:      importer.Provenance{ChunkIDs: []string{"chk_0123456789abcdef0123"}},
			Proposal: importer.CandidateProposal{Codex: &importer.CodexProposal{
				Type:        "character",
				Name:        "Mara Venn",
				Aliases:     []string{"Mara"},
				Tags:        []string{"pilot"},
				Description: "A cautious salvage pilot.",
			}},
			Decision: importer.CandidateDecision{CanonicalRefs: []importer.CanonicalRef{{Kind: "codex", ID: "char_0123456789abcdef0123"}}},
		},
		acceptRefs: []importer.CanonicalRef{{Kind: "codex", ID: "char_0123456789abcdef0123"}},
	}
	handler := api.NewHandler(&projectStoreStub{}, &activeProjectSessionStub{}, stub, "test")

	for _, testCase := range []struct {
		name         string
		method       string
		path         string
		body         string
		status       int
		expectedJSON string
	}{
		{
			name:         "create import",
			method:       http.MethodPost,
			path:         "/api/imports",
			body:         `{"source_directory":"/tmp/notes"}`,
			status:       http.StatusCreated,
			expectedJSON: `{"import":{"id":"imp_0123456789abcdef0123","created_at":"2026-06-30T12:00:00Z","file_count":1,"total_bytes":12},"files":[{"path":"notes/characters.md","bytes":12,"sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}]}`,
		},
		{
			name:         "list imports",
			method:       http.MethodGet,
			path:         "/api/imports",
			status:       http.StatusOK,
			expectedJSON: `{"imports":[{"id":"imp_0123456789abcdef0123","created_at":"2026-06-30T12:00:00Z","file_count":1,"total_bytes":12}]}`,
		},
		{
			name:         "list chunks",
			method:       http.MethodGet,
			path:         "/api/imports/imp_0123456789abcdef0123/chunks",
			status:       http.StatusOK,
			expectedJSON: `{"chunks":[{"id":"chk_0123456789abcdef0123","import_id":"imp_0123456789abcdef0123","source_path":"notes/characters.md","start_line":1,"end_line":2,"text":"# Characters\nMara\n","sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}]}`,
		},
		{
			name:         "extract import",
			method:       http.MethodPost,
			path:         "/api/imports/imp_0123456789abcdef0123/extractions",
			body:         `{"chunk_ids":["chk_0123456789abcdef0123"],"mode":"structure","profile_id":"local_ollama","model":"qwen2.5:7b"}`,
			status:       http.StatusCreated,
			expectedJSON: `{"candidates":[{"id":"cand_0123456789abcdef0123","kind":"codex","proposal_version":1,"status":"pending","revision":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc","provenance":{"chunk_ids":["chk_0123456789abcdef0123"]},"proposal":{"type":"character","name":"Mara Venn","aliases":["Mara"],"tags":["pilot"],"description":"A cautious salvage pilot."},"replacement_candidate_id":null,"canonical_refs":[]}],"provider":{"profile_id":"local_ollama","type":"ollama","model":"qwen2.5:7b"}}`,
		},
		{
			name:         "list candidates",
			method:       http.MethodGet,
			path:         "/api/import-candidates?status=pending&kind=codex",
			status:       http.StatusOK,
			expectedJSON: `{"candidates":[{"id":"cand_0123456789abcdef0123","kind":"codex","proposal_version":1,"status":"pending","revision":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc","provenance":{"chunk_ids":["chk_0123456789abcdef0123"]},"proposal":{"type":"character","name":"Mara Venn","aliases":["Mara"],"tags":["pilot"],"description":"A cautious salvage pilot."},"replacement_candidate_id":null,"canonical_refs":[]}]}`,
		},
		{
			name:         "get candidate",
			method:       http.MethodGet,
			path:         "/api/import-candidates/cand_0123456789abcdef0123",
			status:       http.StatusOK,
			expectedJSON: `{"id":"cand_0123456789abcdef0123","kind":"codex","proposal_version":1,"status":"pending","revision":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc","provenance":{"chunk_ids":["chk_0123456789abcdef0123"]},"proposal":{"type":"character","name":"Mara Venn","aliases":["Mara"],"tags":["pilot"],"description":"A cautious salvage pilot."},"replacement_candidate_id":null,"canonical_refs":[]}`,
		},
		{
			name:         "edit candidate",
			method:       http.MethodPut,
			path:         "/api/import-candidates/cand_0123456789abcdef0123",
			body:         `{"proposal":{"type":"character","name":"Mara Venn","aliases":["Mara"],"tags":["pilot"],"description":"Edited author text."},"expected_revision":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}`,
			status:       http.StatusOK,
			expectedJSON: `{"id":"cand_0123456789abcdef0123","kind":"codex","proposal_version":1,"status":"pending","revision":"sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd","provenance":{"chunk_ids":["chk_0123456789abcdef0123"]},"proposal":{"type":"character","name":"Mara Venn","aliases":["Mara"],"tags":["pilot"],"description":"Edited author text."},"replacement_candidate_id":null,"canonical_refs":[]}`,
		},
		{
			name:         "merge candidate",
			method:       http.MethodPost,
			path:         "/api/import-candidates/cand_0123456789abcdef0123/merge",
			body:         `{"other_candidate_id":"cand_abcdef0123456789abcd","expected_revision":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc","other_expected_revision":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","proposal":{"type":"character","name":"Mara Venn","aliases":["Mara"],"tags":["pilot"],"description":"Merged author text."}}`,
			status:       http.StatusCreated,
			expectedJSON: `{"candidate":{"id":"cand_0123456789abcdef0999","kind":"codex","proposal_version":1,"status":"pending","revision":"sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee","provenance":{"chunk_ids":["chk_0123456789abcdef0123","chk_0123456789abcdef0456"]},"proposal":{"type":"character","name":"Mara Venn","aliases":["Mara"],"tags":["pilot"],"description":"Merged author text."},"replacement_candidate_id":null,"canonical_refs":[]},"merged_candidate_ids":["cand_0123456789abcdef0123","cand_abcdef0123456789abcd"]}`,
		},
		{
			name:         "discard candidate",
			method:       http.MethodPost,
			path:         "/api/import-candidates/cand_0123456789abcdef0123/discard",
			body:         `{"expected_revision":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}`,
			status:       http.StatusOK,
			expectedJSON: `{"id":"cand_0123456789abcdef0123","kind":"codex","proposal_version":1,"status":"discarded","revision":"sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff","provenance":{"chunk_ids":["chk_0123456789abcdef0123"]},"proposal":{"type":"character","name":"Mara Venn","aliases":["Mara"],"tags":["pilot"],"description":"A cautious salvage pilot."},"replacement_candidate_id":null,"canonical_refs":[]}`,
		},
		{
			name:         "accept candidate",
			method:       http.MethodPost,
			path:         "/api/import-candidates/cand_0123456789abcdef0123/accept",
			body:         `{"expected_revision":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}`,
			status:       http.StatusOK,
			expectedJSON: `{"candidate":{"id":"cand_0123456789abcdef0123","kind":"codex","proposal_version":1,"status":"accepted","revision":"sha256:1111111111111111111111111111111111111111111111111111111111111111","provenance":{"chunk_ids":["chk_0123456789abcdef0123"]},"proposal":{"type":"character","name":"Mara Venn","aliases":["Mara"],"tags":["pilot"],"description":"A cautious salvage pilot."},"replacement_candidate_id":null,"canonical_refs":[{"kind":"codex","id":"char_0123456789abcdef0123"}]},"canonical_refs":[{"kind":"codex","id":"char_0123456789abcdef0123"}]}`,
		},
	} {
		request := httptest.NewRequest(testCase.method, testCase.path, strings.NewReader(testCase.body))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != testCase.status {
			t.Fatalf("%s status = %d, want %d body=%s", testCase.name, response.Code, testCase.status, response.Body.String())
		}
		assertJSONShape(t, response.Body.Bytes(), testCase.expectedJSON)
	}
}

func TestImportRouteStatusMapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		method string
		path   string
		body   string
		stub   storyServiceStub
		status int
	}{
		{name: "bad candidate filter", method: http.MethodGet, path: "/api/import-candidates?kind=bad", status: http.StatusBadRequest},
		{name: "invalid source directory", method: http.MethodPost, path: "/api/imports", body: `{"source_directory":"/tmp/notes"}`, stub: storyServiceStub{importErr: importer.ErrInvalidSourceDirectory}, status: http.StatusBadRequest},
		{name: "dirty worktree", method: http.MethodPost, path: "/api/imports", body: `{"source_directory":"/tmp/notes"}`, stub: storyServiceStub{importErr: story.ErrDirtyWorktree}, status: http.StatusConflict},
		{name: "provider rejected", method: http.MethodPost, path: "/api/imports/imp_0123456789abcdef0123/extractions", body: `{"chunk_ids":["chk_0123456789abcdef0123"],"mode":"structure","profile_id":"local","model":"m"}`, stub: storyServiceStub{extractErr: agent.ErrProviderRejected}, status: http.StatusBadGateway},
		{name: "invalid provider response", method: http.MethodPost, path: "/api/imports/imp_0123456789abcdef0123/extractions", body: `{"chunk_ids":["chk_0123456789abcdef0123"],"mode":"structure","profile_id":"local","model":"m"}`, stub: storyServiceStub{extractErr: extract.ErrInvalidResponse}, status: http.StatusBadGateway},
		{name: "provider offline", method: http.MethodPost, path: "/api/imports/imp_0123456789abcdef0123/extractions", body: `{"chunk_ids":["chk_0123456789abcdef0123"],"mode":"structure","profile_id":"local","model":"m"}`, stub: storyServiceStub{extractErr: agent.ErrProviderOffline}, status: http.StatusServiceUnavailable},
		{name: "candidate not found", method: http.MethodGet, path: "/api/import-candidates/cand_0123456789abcdef0123", stub: storyServiceStub{candidateErr: os.ErrNotExist}, status: http.StatusNotFound},
		{name: "stale candidate", method: http.MethodPost, path: "/api/import-candidates/cand_0123456789abcdef0123/discard", body: `{"expected_revision":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`, stub: storyServiceStub{discardCandidateErr: importer.ErrCandidateConflict}, status: http.StatusConflict},
		{name: "invalid import id", method: http.MethodGet, path: "/api/imports/not-an-id/chunks", status: http.StatusBadRequest},
		{name: "invalid candidate id", method: http.MethodGet, path: "/api/import-candidates/not-an-id", status: http.StatusBadRequest},
	}

	for _, testCase := range tests {
		handler := api.NewHandler(&projectStoreStub{}, &activeProjectSessionStub{}, &testCase.stub, "test")
		request := httptest.NewRequest(testCase.method, testCase.path, strings.NewReader(testCase.body))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != testCase.status {
			t.Fatalf("%s status = %d, want %d body=%s", testCase.name, response.Code, testCase.status, response.Body.String())
		}
	}
}

func TestImportMutationRoutesRejectMalformedBodiesAndOversizeWithContractStatuses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		path   string
		body   string
		status int
	}{
		{name: "missing import field", path: "/api/imports", body: `{}`, status: http.StatusBadRequest},
		{name: "null import field", path: "/api/imports", body: `{"source_directory":null}`, status: http.StatusBadRequest},
		{name: "unknown import field", path: "/api/imports", body: `{"source_directory":"/tmp/notes","extra":true}`, status: http.StatusBadRequest},
		{name: "trailing import value", path: "/api/imports", body: `{"source_directory":"/tmp/notes"}{}`, status: http.StatusBadRequest},
		{name: "null extraction chunks", path: "/api/imports/imp_0123456789abcdef0123/extractions", body: `{"chunk_ids":null,"mode":"structure","profile_id":"local","model":"m"}`, status: http.StatusBadRequest},
		{name: "malformed decision revision", path: "/api/import-candidates/cand_0123456789abcdef0123/discard", body: `{"expected_revision":"bad"}`, status: http.StatusBadRequest},
		{name: "oversized import", path: "/api/imports", body: `{"source_directory":"/` + strings.Repeat("x", (1<<20)+1) + `"}`, status: http.StatusRequestEntityTooLarge},
	}
	for _, testCase := range tests {
		handler := api.NewHandler(&projectStoreStub{}, &activeProjectSessionStub{}, &storyServiceStub{}, "test")
		request := httptest.NewRequest(http.MethodPost, testCase.path, strings.NewReader(testCase.body))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != testCase.status {
			t.Errorf("%s status = %d, want %d body=%s", testCase.name, response.Code, testCase.status, response.Body.String())
		}
	}
}

func TestImportErrorsDoNotDiscloseSensitiveAdapterDetails(t *testing.T) {
	t.Parallel()

	sensitive := "/home/alice/private-notes token-secret provider-body"
	handler := api.NewHandler(&projectStoreStub{}, &activeProjectSessionStub{}, &storyServiceStub{
		importErr: errors.New(sensitive),
	}, "test")
	request := httptest.NewRequest(http.MethodPost, "/api/imports", strings.NewReader(`{"source_directory":"/tmp/notes"}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", response.Code)
	}
	if strings.Contains(response.Body.String(), sensitive) || strings.Contains(response.Body.String(), "private-notes") {
		t.Fatalf("response disclosed adapter detail: %s", response.Body.String())
	}
}

func TestImportRoutesRejectUnsupportedMethodsWithAllowHeader(t *testing.T) {
	t.Parallel()

	tests := []struct{ method, path, allow string }{
		{http.MethodPatch, "/api/imports", "GET, POST"},
		{http.MethodPost, "/api/imports/imp_0123456789abcdef0123/chunks", "GET"},
		{http.MethodGet, "/api/imports/imp_0123456789abcdef0123/extractions", "POST"},
		{http.MethodDelete, "/api/import-candidates", "GET"},
		{http.MethodPost, "/api/import-candidates/cand_0123456789abcdef0123", "GET, PUT"},
		{http.MethodGet, "/api/import-candidates/cand_0123456789abcdef0123/merge", "POST"},
		{http.MethodGet, "/api/import-candidates/cand_0123456789abcdef0123/discard", "POST"},
		{http.MethodGet, "/api/import-candidates/cand_0123456789abcdef0123/accept", "POST"},
	}
	handler := api.NewHandler(&projectStoreStub{}, &activeProjectSessionStub{}, &storyServiceStub{}, "test")
	for _, testCase := range tests {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(testCase.method, testCase.path, nil))
		if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != testCase.allow {
			t.Errorf("%s %s status=%d allow=%q, want 405 %q", testCase.method, testCase.path, response.Code, response.Header().Get("Allow"), testCase.allow)
		}
	}
}
