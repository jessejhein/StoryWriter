package provider

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// BDD trace:
//   - Requirements: M5-R01, M5-R02.
//   - Scenarios: 5.1.1, 5.1.2, 5.1.3.
//   - Test purpose: verify strict provider profile validation, canonical
//     ordering and revision, missing-file empty state, and optimistic whole-doc
//     replacement outside story projects.
func TestValidateProfilesResolveConfigDirAndStoreRoundTrip(t *testing.T) {
	t.Parallel()

	localOpenAI := Profile{
		ID:      "local_openai",
		Name:    "Local OpenAI-compatible",
		Type:    TypeOpenAICompatible,
		BaseURL: "http://127.0.0.1:1234/v1/",
		Auth: AuthConfig{
			Type:          AuthTypeNone,
			CredentialEnv: "",
		},
		Capabilities: Capabilities{
			Chat:             true,
			Streaming:        false,
			StructuredOutput: false,
			MaxContextTokens: 8192,
		},
	}
	hosted := Profile{
		ID:      "hosted_api",
		Name:    "Hosted API",
		Type:    TypeOpenAICompatible,
		BaseURL: "https://api.example.test/v1",
		Auth: AuthConfig{
			Type:          AuthTypeBearerEnv,
			CredentialEnv: "STORYWORK_HOSTED_API_KEY",
		},
		Capabilities: Capabilities{
			Chat:             true,
			Streaming:        false,
			StructuredOutput: false,
			MaxContextTokens: 32768,
		},
	}
	ollama := Profile{
		ID:      "local_ollama",
		Name:    "Local Ollama",
		Type:    TypeOllama,
		BaseURL: "http://127.0.0.1:11434",
		Auth: AuthConfig{
			Type:          AuthTypeNone,
			CredentialEnv: "",
		},
		Capabilities: Capabilities{
			Chat:             true,
			Streaming:        false,
			StructuredOutput: false,
			MaxContextTokens: 8192,
		},
	}

	normalized, canonical, revision, err := ValidateProfiles([]Profile{localOpenAI, hosted, ollama})
	if err != nil {
		t.Fatalf("ValidateProfiles() error = %v", err)
	}
	if got := []string{normalized[0].ID, normalized[1].ID, normalized[2].ID}; strings.Join(got, ",") != "hosted_api,local_ollama,local_openai" {
		t.Fatalf("normalized ordering = %v", got)
	}
	if normalized[2].BaseURL != "http://127.0.0.1:1234/v1" {
		t.Fatalf("normalized base_url = %q, want trimmed trailing slash", normalized[2].BaseURL)
	}
	if revision != ComputeRevision(canonical) {
		t.Fatalf("revision = %q, want canonical revision", revision)
	}
	expectedCanonical := "" +
		"version: 1\n" +
		"profiles:\n" +
		"  - id: hosted_api\n" +
		"    name: Hosted API\n" +
		"    type: openai_compatible\n" +
		"    base_url: https://api.example.test/v1\n" +
		"    auth:\n" +
		"      type: bearer_env\n" +
		"      credential_env: STORYWORK_HOSTED_API_KEY\n" +
		"    capabilities:\n" +
		"      chat: true\n" +
		"      streaming: false\n" +
		"      structured_output: false\n" +
		"      max_context_tokens: 32768\n" +
		"  - id: local_ollama\n" +
		"    name: Local Ollama\n" +
		"    type: ollama\n" +
		"    base_url: http://127.0.0.1:11434\n" +
		"    auth:\n" +
		"      type: none\n" +
		"      credential_env: \"\"\n" +
		"    capabilities:\n" +
		"      chat: true\n" +
		"      streaming: false\n" +
		"      structured_output: false\n" +
		"      max_context_tokens: 8192\n" +
		"  - id: local_openai\n" +
		"    name: Local OpenAI-compatible\n" +
		"    type: openai_compatible\n" +
		"    base_url: http://127.0.0.1:1234/v1\n" +
		"    auth:\n" +
		"      type: none\n" +
		"      credential_env: \"\"\n" +
		"    capabilities:\n" +
		"      chat: true\n" +
		"      streaming: false\n" +
		"      structured_output: false\n" +
		"      max_context_tokens: 8192\n"
	if string(canonical) != expectedCanonical {
		t.Fatalf("canonical bytes = %q", string(canonical))
	}

	configDir, err := ResolveConfigDir("/tmp/storywork-config", func() (string, error) {
		t.Fatal("fallback should not be called for absolute override")
		return "", nil
	})
	if err != nil {
		t.Fatalf("ResolveConfigDir(absolute) error = %v", err)
	}
	if configDir != "/tmp/storywork-config" {
		t.Fatalf("ResolveConfigDir(absolute) = %q", configDir)
	}
	if _, err := ResolveConfigDir("relative", func() (string, error) { return "/unused", nil }); err == nil {
		t.Fatal("ResolveConfigDir(relative) error = nil, want failure")
	}
	configDir, err = ResolveConfigDir("", func() (string, error) { return "/home/tester/.config", nil })
	if err != nil {
		t.Fatalf("ResolveConfigDir(fallback) error = %v", err)
	}
	if configDir != "/home/tester/.config/storywork" {
		t.Fatalf("ResolveConfigDir(fallback) = %q", configDir)
	}

	root := t.TempDir()
	store := NewStore(filepath.Join(root, "providers.yaml"))
	loaded, loadedRevision, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load(missing) error = %v", err)
	}
	if len(loaded) != 0 || loadedRevision != nil {
		t.Fatalf("Load(missing) = (%v, %v), want empty/null", loaded, loadedRevision)
	}

	savedProfiles, savedRevision, err := store.Save(context.Background(), []Profile{localOpenAI, hosted, ollama}, nil)
	if err != nil {
		t.Fatalf("Save(create) error = %v", err)
	}
	if savedRevision == nil || *savedRevision == "" {
		t.Fatalf("Save(create) revision = %v, want value", savedRevision)
	}
	if len(savedProfiles) != 3 || savedProfiles[0].ID != "hosted_api" {
		t.Fatalf("Save(create) profiles = %#v", savedProfiles)
	}
	persistedBytes, err := os.ReadFile(filepath.Join(root, "providers.yaml"))
	if err != nil {
		t.Fatalf("ReadFile(providers.yaml) error = %v", err)
	}
	if string(persistedBytes) != expectedCanonical {
		t.Fatalf("persisted bytes = %q", string(persistedBytes))
	}

	reloaded, reloadedRevision, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load(saved) error = %v", err)
	}
	if reloadedRevision == nil || *reloadedRevision != *savedRevision {
		t.Fatalf("Load(saved) revision = %v, want %q", reloadedRevision, *savedRevision)
	}
	if len(reloaded) != 3 || reloaded[2].BaseURL != "http://127.0.0.1:1234/v1" {
		t.Fatalf("Load(saved) profiles = %#v", reloaded)
	}

	if _, _, err := store.Save(context.Background(), []Profile{localOpenAI, hosted, ollama}, savedRevision); !errors.Is(err, ErrNoProfileChanges) {
		t.Fatalf("Save(no change) error = %v, want ErrNoProfileChanges", err)
	}

	stale := "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if _, _, err := store.Save(context.Background(), []Profile{localOpenAI}, &stale); !errors.Is(err, ErrProfileRevisionConflict) {
		t.Fatalf("Save(stale) error = %v, want ErrProfileRevisionConflict", err)
	}
}

// BDD trace:
//   - Requirements: M5-R01, M5-R02.
//   - Scenario: 5.1.3.
//   - Test purpose: verify invalid profile definitions and malformed canonical
//     provider state fail closed instead of being repaired or partially loaded.
func TestValidateProfilesAndStoreRejectInvalidState(t *testing.T) {
	t.Parallel()

	valid := Profile{
		ID:      "hosted_api",
		Name:    "Hosted API",
		Type:    TypeOpenAICompatible,
		BaseURL: "https://api.example.test/v1",
		Auth: AuthConfig{
			Type:          AuthTypeBearerEnv,
			CredentialEnv: "STORYWORK_HOSTED_API_KEY",
		},
		Capabilities: Capabilities{
			Chat:             true,
			Streaming:        false,
			StructuredOutput: false,
			MaxContextTokens: 32768,
		},
	}

	invalidProfiles := []struct {
		name    string
		profile Profile
	}{
		{name: "query string forbidden", profile: Profile{ID: "bad_query", Name: "Bad", Type: TypeOpenAICompatible, BaseURL: "https://api.example.test/v1?x=1", Auth: AuthConfig{Type: AuthTypeNone}, Capabilities: valid.Capabilities}},
		{name: "userinfo forbidden", profile: Profile{ID: "bad_user", Name: "Bad", Type: TypeOpenAICompatible, BaseURL: "https://user:pass@example.test/v1", Auth: AuthConfig{Type: AuthTypeNone}, Capabilities: valid.Capabilities}},
		{name: "non https bearer on non loopback forbidden", profile: Profile{ID: "bad_http", Name: "Bad", Type: TypeOpenAICompatible, BaseURL: "http://api.example.test/v1", Auth: AuthConfig{Type: AuthTypeBearerEnv, CredentialEnv: "STORYWORK_BAD_KEY"}, Capabilities: valid.Capabilities}},
		{name: "ollama bearer forbidden", profile: Profile{ID: "bad_ollama", Name: "Bad", Type: TypeOllama, BaseURL: "http://127.0.0.1:11434", Auth: AuthConfig{Type: AuthTypeBearerEnv, CredentialEnv: "STORYWORK_BAD_KEY"}, Capabilities: valid.Capabilities}},
		{name: "none auth requires empty env", profile: Profile{ID: "bad_none_env", Name: "Bad", Type: TypeOpenAICompatible, BaseURL: "http://127.0.0.1:1234", Auth: AuthConfig{Type: AuthTypeNone, CredentialEnv: "STORYWORK_BAD_KEY"}, Capabilities: valid.Capabilities}},
		{name: "invalid env name", profile: Profile{ID: "bad_env", Name: "Bad", Type: TypeOpenAICompatible, BaseURL: "https://api.example.test/v1", Auth: AuthConfig{Type: AuthTypeBearerEnv, CredentialEnv: "BAD_KEY"}, Capabilities: valid.Capabilities}},
		{name: "zero context limit", profile: Profile{ID: "bad_context", Name: "Bad", Type: TypeOpenAICompatible, BaseURL: "https://api.example.test/v1", Auth: AuthConfig{Type: AuthTypeNone}, Capabilities: Capabilities{Chat: true}}},
	}
	for _, tc := range invalidProfiles {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, _, _, err := ValidateProfiles([]Profile{tc.profile}); err == nil {
				t.Fatal("ValidateProfiles() error = nil, want failure")
			}
		})
	}
	if _, _, _, err := ValidateProfiles([]Profile{valid, valid}); err == nil {
		t.Fatal("ValidateProfiles(duplicate id) error = nil, want failure")
	}

	root := t.TempDir()
	path := filepath.Join(root, "providers.yaml")
	if err := os.WriteFile(path, []byte("version: 1\nprofiles:\n  - id: bad\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(malformed) error = %v", err)
	}
	store := NewStore(path)
	if _, _, err := store.Load(context.Background()); err == nil {
		t.Fatal("Load(malformed) error = nil, want failure")
	}
}

// BDD trace:
//   - Requirements: M5-R03, M5-R04.
//   - Scenario: 5.1.4.
//   - Test purpose: verify credential lookup reports readiness without trimming
//     credential bytes and without embedding the secret in public profile data.
func TestEnvironmentBrokerReadiness(t *testing.T) {
	t.Parallel()

	broker := EnvironmentBroker{
		LookupEnv: func(key string) (string, bool) {
			switch key {
			case "STORYWORK_PRESENT_KEY":
				return "  secret-with-spaces  ", true
			case "STORYWORK_EMPTY_KEY":
				return "", true
			default:
				return "", false
			}
		},
	}

	credential, readiness, err := broker.Resolve(context.Background(), "STORYWORK_PRESENT_KEY")
	if err != nil {
		t.Fatalf("Resolve(present) error = %v", err)
	}
	if readiness != ReadinessReady || credential.Value != "  secret-with-spaces  " {
		t.Fatalf("Resolve(present) = (%q, %q)", credential.Value, readiness)
	}

	credential, readiness, err = broker.Resolve(context.Background(), "STORYWORK_EMPTY_KEY")
	if err != nil {
		t.Fatalf("Resolve(empty) error = %v", err)
	}
	if readiness != ReadinessMissingCredential || credential.Value != "" {
		t.Fatalf("Resolve(empty) = (%q, %q)", credential.Value, readiness)
	}

	credential, readiness, err = broker.Resolve(context.Background(), "STORYWORK_MISSING_KEY")
	if err != nil {
		t.Fatalf("Resolve(missing) error = %v", err)
	}
	if readiness != ReadinessMissingCredential || credential.Value != "" {
		t.Fatalf("Resolve(missing) = (%q, %q)", credential.Value, readiness)
	}
}

// Test: canonical YAML safely round-trips every punctuation form accepted in a
// profile name rather than emitting an invalid plain YAML scalar.
// Requirements: M5-R01, M5-R02.
func TestStoreCanonicalNamesRoundTrip(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"A: B", "Name # variant", `Quoted "name"`, `Backslash \\ name`, "[Draft]"} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "providers.yaml")
			store := NewStore(path)
			profiles := []Profile{{
				ID: "local", Name: name, Type: TypeOllama, BaseURL: "http://127.0.0.1:11434",
				Auth:         AuthConfig{Type: AuthTypeNone},
				Capabilities: Capabilities{Chat: true, MaxContextTokens: 8192},
			}}
			if _, _, err := store.Save(context.Background(), profiles, nil); err != nil {
				t.Fatalf("Save() error = %v", err)
			}
			loaded, _, err := store.Load(context.Background())
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if len(loaded) != 1 || loaded[0].Name != name {
				t.Fatalf("Load() name = %q, want %q", loaded[0].Name, name)
			}
		})
	}
}
