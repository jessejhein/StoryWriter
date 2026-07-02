package app_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"storywork/internal/app"

	_ "modernc.org/sqlite"
)

// TestMilestone6EndToEndWithRealAdapters exercises the complete HTTP workflow
// with real filesystem, Git, SQLite, provider transport, and service adapters.
func TestMilestone6EndToEndWithRealAdapters(t *testing.T) {
	configPath := t.TempDir()
	t.Setenv("STORYWORK_CONFIG_DIR", configPath)
	providerOutput := `{"candidates":[` +
		`{"kind":"arc","local_id":"arc_one","title":"Act One"},` +
		`{"kind":"chapter","local_id":"chapter_one","title":"Arrival","parent_local_id":"arc_one"},` +
		`{"kind":"scene","local_id":"scene_one","title":"The Station","parent_local_id":"chapter_one"},` +
		`{"kind":"codex","local_id":"mara_one","type":"character","name":"Mara Venn","aliases":["Mara"],"tags":["pilot"],"description":"First source."},` +
		`{"kind":"codex","local_id":"mara_two","type":"character","name":"Mara Venn","aliases":["Captain Mara"],"tags":["salvage"],"description":"Second source."},` +
		`{"kind":"codex","local_id":"unused","type":"character","name":"Unused","aliases":[],"tags":[],"description":"Discard me."}` +
		`]}`
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/chat" || request.Method != http.MethodPost {
			t.Errorf("provider request = %s %s", request.Method, request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{"message": map[string]string{"content": providerOutput}})
	}))
	defer provider.Close()

	handler := app.NewHandler("test")
	putJSON(t, handler, http.MethodPut, "/api/provider-profiles", map[string]any{
		"profiles": []map[string]any{{
			"id": "local_test", "name": "Local Test", "type": "ollama", "base_url": provider.URL,
			"auth":         map[string]any{"type": "none", "credential_env": ""},
			"capabilities": map[string]any{"chat": true, "streaming": false, "structured_output": false, "max_context_tokens": 8192},
		}},
		"expected_revision": nil,
	}, http.StatusOK, nil)

	projectPath := filepath.Join(t.TempDir(), "novel")
	putJSON(t, handler, http.MethodPost, "/api/projects", map[string]any{"name": "Novel", "path": projectPath}, http.StatusCreated, nil)
	sourcePath := t.TempDir()
	writeIntegrationFile(t, filepath.Join(sourcePath, "z.md"), "# Later\r\nText\r\n")
	writeIntegrationFile(t, filepath.Join(sourcePath, "nested", "a.markdown"), "# First\nMara arrives.\n")

	var imported struct {
		Import struct {
			ID string `json:"id"`
		} `json:"import"`
		Files []struct {
			Path string `json:"path"`
		} `json:"files"`
	}
	putJSON(t, handler, http.MethodPost, "/api/imports", map[string]string{"source_directory": sourcePath}, http.StatusCreated, &imported)
	if len(imported.Files) != 2 || imported.Files[0].Path != "nested/a.markdown" || imported.Files[1].Path != "z.md" {
		t.Fatalf("imported files = %+v", imported.Files)
	}
	assertCommitCount(t, projectPath, 2)
	assertClean(t, projectPath)
	assertTreeDoesNotContain(t, projectPath, sourcePath)
	var reloadedImport struct {
		Files []struct {
			Path string `json:"path"`
		} `json:"files"`
	}
	getJSON(t, handler, "/api/imports/"+imported.Import.ID, http.StatusOK, &reloadedImport)
	if len(reloadedImport.Files) != 2 || reloadedImport.Files[0].Path != "nested/a.markdown" {
		t.Fatalf("reloaded import files = %+v", reloadedImport.Files)
	}
	normalized, err := os.ReadFile(filepath.Join(projectPath, "imports", "raw", imported.Import.ID, "files", "z.md"))
	if err != nil || string(normalized) != "# Later\nText\n" {
		t.Fatalf("normalized snapshot = %q, err=%v", normalized, err)
	}

	var chunkResponse struct {
		Chunks []struct {
			ID         string `json:"id"`
			SourcePath string `json:"source_path"`
			Text       string `json:"text"`
			StartLine  int    `json:"start_line"`
			EndLine    int    `json:"end_line"`
		} `json:"chunks"`
	}
	getJSON(t, handler, "/api/imports/"+imported.Import.ID+"/chunks", http.StatusOK, &chunkResponse)
	if len(chunkResponse.Chunks) != 2 || chunkResponse.Chunks[0].SourcePath != "nested/a.markdown" {
		t.Fatalf("chunks = %+v", chunkResponse.Chunks)
	}
	chunkIDs := []string{chunkResponse.Chunks[0].ID, chunkResponse.Chunks[1].ID}

	var extracted struct {
		Candidates []candidateJSON `json:"candidates"`
	}
	putJSON(t, handler, http.MethodPost, "/api/imports/"+imported.Import.ID+"/extractions", map[string]any{
		"chunk_ids": chunkIDs, "mode": "structure", "profile_id": "local_test", "model": "test-model",
	}, http.StatusCreated, &extracted)
	if len(extracted.Candidates) != 6 {
		t.Fatalf("candidate count = %d", len(extracted.Candidates))
	}
	assertCommitCount(t, projectPath, 3)
	attempts, err := filepath.Glob(filepath.Join(projectPath, ".storywork", "import", imported.Import.ID, "attempts", "*.json"))
	if err != nil || len(attempts) != 1 {
		t.Fatalf("extraction attempts = %v, err=%v", attempts, err)
	}
	attemptBytes, err := os.ReadFile(attempts[0])
	if err != nil || bytes.Contains(attemptBytes, []byte(providerOutput)) || bytes.Contains(attemptBytes, []byte(sourcePath)) {
		t.Fatalf("unsafe extraction attempt data: %s err=%v", attemptBytes, err)
	}
	for _, path := range []string{"arcs", "chapters", "scenes", "codex/characters"} {
		entries, err := os.ReadDir(filepath.Join(projectPath, path))
		canonicalEntries := 0
		for _, entry := range entries {
			if !strings.HasPrefix(entry.Name(), ".") {
				canonicalEntries++
			}
		}
		if err != nil || canonicalEntries != 0 {
			t.Fatalf("extraction changed canon %s: entries=%v err=%v", path, entries, err)
		}
	}

	arc := candidateByKind(t, extracted.Candidates, "arc", 0)
	chapter := candidateByKind(t, extracted.Candidates, "chapter", 0)
	scene := candidateByKind(t, extracted.Candidates, "scene", 0)
	codexOne := candidateByKind(t, extracted.Candidates, "codex", 0)
	codexTwo := candidateByKind(t, extracted.Candidates, "codex", 1)
	discard := candidateByKind(t, extracted.Candidates, "codex", 2)

	var edited candidateJSON
	putJSON(t, handler, http.MethodPut, "/api/import-candidates/"+codexOne.ID, map[string]any{
		"proposal":          map[string]any{"type": "character", "name": "Mara Venn", "aliases": []string{"Mara"}, "tags": []string{"pilot"}, "description": "Author edit."},
		"expected_revision": codexOne.Revision,
	}, http.StatusOK, &edited)
	assertCommitCount(t, projectPath, 4)

	var merged struct {
		Candidate candidateJSON `json:"candidate"`
	}
	putJSON(t, handler, http.MethodPost, "/api/import-candidates/"+edited.ID+"/merge", map[string]any{
		"other_candidate_id": codexTwo.ID, "expected_revision": edited.Revision, "other_expected_revision": codexTwo.Revision,
		"proposal": map[string]any{"type": "character", "name": "Mara Venn", "aliases": []string{"Mara"}, "tags": []string{"pilot", "salvage"}, "description": "Merged author text."},
	}, http.StatusCreated, &merged)
	assertCommitCount(t, projectPath, 5)
	putJSON(t, handler, http.MethodPost, "/api/import-candidates/"+discard.ID+"/discard", map[string]string{"expected_revision": discard.Revision}, http.StatusOK, nil)
	assertCommitCount(t, projectPath, 6)

	for index, candidate := range []candidateJSON{arc, chapter, scene, merged.Candidate} {
		putJSON(t, handler, http.MethodPost, "/api/import-candidates/"+candidate.ID+"/accept", map[string]string{"expected_revision": candidate.Revision}, http.StatusOK, nil)
		assertCommitCount(t, projectPath, 7+index)
		assertClean(t, projectPath)
	}

	var queue struct {
		Candidates []candidateJSON `json:"candidates"`
	}
	reloadedHandler := app.NewHandler("test")
	putJSON(t, reloadedHandler, http.MethodPost, "/api/projects/open", map[string]string{"path": projectPath}, http.StatusOK, nil)
	getJSON(t, reloadedHandler, "/api/import-candidates", http.StatusOK, &queue)
	if len(queue.Candidates) != 7 {
		t.Fatalf("reloaded queue count = %d, want 7", len(queue.Candidates))
	}
	assertIndexed(t, projectPath, "outline.yaml")
	assertTreeDoesNotContain(t, projectPath, sourcePath)
}

type candidateJSON struct {
	ID       string         `json:"id"`
	Kind     string         `json:"kind"`
	Status   string         `json:"status"`
	Revision string         `json:"revision"`
	Proposal map[string]any `json:"proposal"`
}

func candidateByKind(t *testing.T, candidates []candidateJSON, kind string, occurrence int) candidateJSON {
	t.Helper()
	for _, candidate := range candidates {
		if candidate.Kind == kind {
			if occurrence == 0 {
				return candidate
			}
			occurrence--
		}
	}
	t.Fatalf("candidate kind %s not found", kind)
	return candidateJSON{}
}

func putJSON(t *testing.T, handler http.Handler, method, path string, body any, status int, target any) {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(encoded))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != status {
		t.Fatalf("%s %s status=%d want=%d body=%s", method, path, response.Code, status, response.Body.String())
	}
	if target != nil && response.Code < 300 {
		if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
			t.Fatal(err)
		}
	}
}

func getJSON(t *testing.T, handler http.Handler, path string, status int, target any) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != status {
		t.Fatalf("GET %s status=%d want=%d body=%s", path, response.Code, status, response.Body.String())
	}
	if target != nil && response.Code < 300 {
		if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
			t.Fatal(err)
		}
	}
}

func writeIntegrationFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertCommitCount(t *testing.T, projectPath string, expected int) {
	t.Helper()
	output, err := exec.Command("git", "-C", projectPath, "rev-list", "--count", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(output)) != strconv.Itoa(expected) {
		t.Fatalf("commit count = %s, want %d", strings.TrimSpace(string(output)), expected)
	}
}

func assertClean(t *testing.T, projectPath string) {
	t.Helper()
	output, err := exec.Command("git", "-C", projectPath, "status", "--porcelain").Output()
	if err != nil || len(bytes.TrimSpace(output)) != 0 {
		t.Fatalf("worktree not clean: %s err=%v", output, err)
	}
}

func assertTreeDoesNotContain(t *testing.T, root, forbidden string) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || strings.Contains(path, string(filepath.Separator)+".git"+string(filepath.Separator)) {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.Contains(body, []byte(forbidden)) {
			t.Fatalf("%s contains external source path", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func assertIndexed(t *testing.T, projectPath, relativePath string) {
	t.Helper()
	database, err := sql.Open("sqlite", filepath.Join(projectPath, ".storywork", "index.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	var count int
	if err := database.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM files WHERE path = ?", relativePath).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("index count for %s = %d", relativePath, count)
	}
}
