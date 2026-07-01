package action_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"storywork/internal/action"
	"storywork/internal/agent"
	"storywork/internal/gitstore"
	"storywork/internal/index"
	"storywork/internal/project"
	"storywork/internal/provider"
	"storywork/internal/story"
	"storywork/internal/storyfile"
	"storywork/internal/workspace"
)

// BDD trace:
//   - Requirements: M5-R01 through M5-R14.
//   - Scenarios: 5.1.1, 5.1.2, 5.2.1, 5.3.1, 5.5.1, 5.5.2.
//   - Test purpose: cross the real app-config, HTTP provider, project file,
//     SQLite, Git, transient reject, and explicit acceptance boundaries.
func TestMilestone5RealProviderFlowWithRealAdapters(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	var outboundRequests int
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		outboundRequests++
		if request.URL.Path != "/v1/chat/completions" || request.Method != http.MethodPost {
			t.Errorf("provider request = %s %s", request.Method, request.URL.Path)
		}
		if request.Header.Get("Authorization") != "" {
			t.Error("no-auth provider received Authorization header")
		}
		var body struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
			Stream bool `json:"stream"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode provider request: %v", err)
		}
		if body.Model != "local-model" || body.Stream || len(body.Messages) != 2 || !strings.Contains(body.Messages[1].Content, "Selected text:\nAlpha beta") {
			t.Errorf("provider request body = %#v", body)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"response-metadata","choices":[{"message":{"role":"assistant","content":"Provider polished selection"},"finish_reason":"stop"}]}`))
	})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		if errors.Is(err, os.ErrPermission) || strings.Contains(err.Error(), "operation not permitted") {
			t.Skipf("local loopback listener unavailable in this environment: %v", err)
		}
		t.Fatalf("Listen() error = %v", err)
	}
	server := &http.Server{Handler: handler}
	go func() {
		_ = server.Serve(listener)
	}()
	defer func() {
		_ = server.Shutdown(context.Background())
	}()
	serverURL := "http://" + listener.Addr().String()

	configPath := filepath.Join(t.TempDir(), "app-config", "providers.yaml")
	profileStore := provider.NewStore(configPath)
	profileService := provider.NewService(profileStore, provider.EnvironmentBroker{})
	profiles := []provider.Profile{{
		ID: "local_openai", Name: "Local OpenAI", Type: provider.TypeOpenAICompatible, BaseURL: serverURL + "/v1",
		Auth:         provider.AuthConfig{Type: provider.AuthTypeNone},
		Capabilities: provider.Capabilities{Chat: true, MaxContextTokens: 8192},
	}}
	if _, revision, err := profileService.Save(ctx, profiles, nil); err != nil || revision == nil {
		t.Fatalf("Save(provider profiles) = revision %v, error %v", revision, err)
	}
	if contents := mustReadFile(t, configPath); bytes.Contains(contents, []byte("secret")) || !bytes.Contains(contents, []byte(serverURL)) {
		t.Fatalf("provider config contents are incorrect: %s", contents)
	}

	projectPath := filepath.Join(t.TempDir(), "real-provider-story")
	git := gitstore.New("git")
	idx := index.New()
	created, err := project.NewService(git, idx, func() time.Time { return time.Date(2026, time.June, 30, 12, 0, 0, 0, time.UTC) }).Create(ctx, project.CreateRequest{Name: "Provider Story", Path: projectPath})
	if err != nil {
		t.Fatalf("Create(project) error = %v", err)
	}
	session := workspace.NewSession()
	session.Set(created)
	stories := story.NewService(session, storyfile.New(), git, idx, &staticStoryIDGenerator{ids: []string{"arc_00000000000000000001", "ch_00000000000000000001", "scn_00000000000000000001"}})
	if _, err := stories.CreateArc(ctx, "Act One"); err != nil {
		t.Fatal(err)
	}
	if _, err := stories.CreateChapter(ctx, "arc_00000000000000000001", "Chapter"); err != nil {
		t.Fatal(err)
	}
	if _, err := stories.CreateScene(ctx, "ch_00000000000000000001", "Scene"); err != nil {
		t.Fatal(err)
	}
	loaded, err := stories.LoadScene(ctx, "scn_00000000000000000001")
	if err != nil {
		t.Fatal(err)
	}
	selected := "Alpha beta gamma delta echo foxtrot golf hotel india juliet kilo lima mike november oscar papa quebec romeo sierra tango."
	saved, err := stories.SaveScene(ctx, loaded.ID, story.SaveSceneRequest{Title: loaded.Title, FrontMatter: loaded.FrontMatter, Markdown: "Before " + selected + " After", ExpectedRevision: loaded.Revision})
	if err != nil {
		t.Fatal(err)
	}
	stylePath := filepath.Join(projectPath, "styles", "real_provider.yaml")
	styleYAML := "version: 2\nid: real_provider\nname: Real Provider\nprovider_profile_id: local_openai\nmodel: local-model\nparameters:\n  temperature: 0.2\nsystem_prompt: Preserve meaning.\n"
	if err := os.WriteFile(stylePath, []byte(styleYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(ctx, "git", "-C", projectPath, "add", "styles/real_provider.yaml")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, output)
	}
	command = exec.CommandContext(ctx, "git", "-C", projectPath, "commit", "-m", "Add real provider style")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, output)
	}

	actions := action.NewService(session, agent.NewLoader(), stories, stories, agent.NewDispatcher(profileService, &http.Client{}), profileService, action.NewRunStore(), &staticRunIDGenerator{ids: []string{"run_00000000000000000001", "run_00000000000000000002"}})
	available, err := actions.AvailableActions(ctx, agent.AvailabilityInput{Surface: agent.SurfaceEditor, InputScope: agent.InputScopeSelection, SceneID: saved.ID, SelectionWords: agent.WordCount(selected)})
	if err != nil || len(available) != 1 || !containsString(available[0].StyleIDs, "real_provider") {
		t.Fatalf("AvailableActions() = %#v, %v", available, err)
	}
	scenePath := filepath.Join(projectPath, "scenes", saved.ID+".md")
	indexPath := filepath.Join(projectPath, ".storywork", "index.sqlite")
	sceneBefore := mustReadFile(t, scenePath)
	indexBefore := mustReadFile(t, indexPath)
	commitsBefore := gitCommitCount(t, ctx, projectPath)
	runRequest := action.RunRequest{AgentID: "line_polish", StyleID: "real_provider", Surface: agent.SurfaceEditor, InputScope: agent.InputScopeSelection, SceneID: saved.ID, SceneRevision: saved.Revision, Selection: action.Selection{StartByte: len("Before "), EndByte: len("Before ") + len(selected), Text: selected}}

	firstRun, err := actions.Run(ctx, runRequest)
	if err != nil {
		t.Fatalf("Run(reject path) error = %v", err)
	}
	if firstRun.Provider.ProfileID != "local_openai" || firstRun.Provider.Model != "local-model" {
		t.Fatalf("provider identity = %#v", firstRun.Provider)
	}
	if !bytes.Equal(sceneBefore, mustReadFile(t, scenePath)) || !bytes.Equal(indexBefore, mustReadFile(t, indexPath)) || gitCommitCount(t, ctx, projectPath) != commitsBefore {
		t.Fatal("run mutated canonical state")
	}
	if _, err := actions.Reject(ctx, firstRun.RunID); err != nil {
		t.Fatalf("Reject() error = %v", err)
	}
	if !bytes.Equal(sceneBefore, mustReadFile(t, scenePath)) || gitCommitCount(t, ctx, projectPath) != commitsBefore {
		t.Fatal("reject mutated canonical state")
	}

	secondRun, err := actions.Run(ctx, runRequest)
	if err != nil {
		t.Fatalf("Run(accept path) error = %v", err)
	}
	_, accepted, err := actions.Accept(ctx, secondRun.RunID, saved.Revision)
	if err != nil {
		t.Fatalf("Accept() error = %v", err)
	}
	if accepted.Markdown != "Before Provider polished selection After\n" || gitCommitCount(t, ctx, projectPath) != commitsBefore+1 {
		t.Fatalf("accepted scene/commits = %q/%d", accepted.Markdown, gitCommitCount(t, ctx, projectPath))
	}
	if clean, err := git.IsClean(ctx, projectPath); err != nil || !clean {
		t.Fatalf("IsClean() = %v, %v", clean, err)
	}
	if outboundRequests != 2 {
		t.Fatalf("outbound request count = %d, want 2", outboundRequests)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
