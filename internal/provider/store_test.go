package provider

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

// Test: provider configuration refuses filesystem indirection and non-files.
// Requirements: M5-R01.
func TestStoreRejectsSymlinkAndNonRegularConfig(t *testing.T) {
	t.Parallel()

	t.Run("symlink", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "target.yaml")
		if err := os.WriteFile(target, []byte("not read"), 0o600); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "providers.yaml")
		if err := os.Symlink(target, path); err != nil {
			t.Fatal(err)
		}
		if _, _, err := NewStore(path).Load(context.Background()); !errors.Is(err, ErrProfileStore) {
			t.Fatalf("Load() error = %v, want ErrProfileStore", err)
		}
	})

	t.Run("directory", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "providers.yaml")
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if _, _, err := NewStore(path).Load(context.Background()); !errors.Is(err, ErrProfileStore) {
			t.Fatalf("Load() error = %v, want ErrProfileStore", err)
		}
	})
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

// BDD trace:
//   - Requirements: M5-R01, M5-R02.
//   - Scenario: 5.1.2, 5.1.3.
//   - Test purpose: verify each atomic-save failure mode wraps ErrProfileStore,
//     preserves the previous canonical file before rename, cleans temp files
//     when possible, and leaves first-create failures without a destination.
func TestStoreSaveFailureInjection(t *testing.T) {
	t.Parallel()

	profiles := []Profile{testLocalOllamaProfile()}
	validExistingBytes := mustCanonicalProfiles(t, []Profile{testHostedOpenAIProfile()})
	existingRevision := ComputeRevision(validExistingBytes)

	testCases := []struct {
		name                   string
		existing               bool
		mutate                 func(t *testing.T, store *Store, state *tempFileState)
		wantPath               string
		wantCleanupAttempt     bool
		wantDestinationPresent bool
		wantPersistedChanged   bool
	}{
		{
			name: "mkdir failure",
			mutate: func(t *testing.T, store *Store, state *tempFileState) {
				t.Helper()
				store.mkdirAll = func(string, os.FileMode) error { return errors.New("mkdir failed") }
			},
			wantPath:               "create provider config dir",
			wantDestinationPresent: false,
		},
		{
			name: "temp create failure",
			mutate: func(t *testing.T, store *Store, state *tempFileState) {
				t.Helper()
				store.openTempFile = func(string, os.FileMode) (tempFile, error) { return nil, errors.New("open temp failed") }
			},
			wantPath:               "create temp provider config",
			wantDestinationPresent: false,
		},
		{
			name:     "partial write failure preserves existing",
			existing: true,
			mutate: func(t *testing.T, store *Store, state *tempFileState) {
				t.Helper()
				state.writeN = 7
				state.writeErr = errors.New("short write")
			},
			wantPath:               "write temp provider config",
			wantCleanupAttempt:     true,
			wantDestinationPresent: true,
		},
		{
			name:     "zero byte write failure preserves existing",
			existing: true,
			mutate: func(t *testing.T, store *Store, state *tempFileState) {
				t.Helper()
				state.writeN = 0
				state.writeErr = errors.New("write failed")
			},
			wantPath:               "write temp provider config",
			wantCleanupAttempt:     true,
			wantDestinationPresent: true,
		},
		{
			name:     "chmod failure preserves existing",
			existing: true,
			mutate: func(t *testing.T, store *Store, state *tempFileState) {
				t.Helper()
				state.chmodErr = errors.New("chmod failed")
			},
			wantPath:               "chmod temp provider config",
			wantCleanupAttempt:     true,
			wantDestinationPresent: true,
		},
		{
			name:     "sync failure preserves existing",
			existing: true,
			mutate: func(t *testing.T, store *Store, state *tempFileState) {
				t.Helper()
				state.syncErr = errors.New("sync failed")
			},
			wantPath:               "sync temp provider config",
			wantCleanupAttempt:     true,
			wantDestinationPresent: true,
		},
		{
			name:     "close failure preserves existing",
			existing: true,
			mutate: func(t *testing.T, store *Store, state *tempFileState) {
				t.Helper()
				state.closeErr = errors.New("close failed")
			},
			wantPath:               "close temp provider config",
			wantCleanupAttempt:     true,
			wantDestinationPresent: true,
		},
		{
			name:     "rename failure preserves existing",
			existing: true,
			mutate: func(t *testing.T, store *Store, state *tempFileState) {
				t.Helper()
				store.rename = func(string, string) error { return errors.New("rename failed") }
			},
			wantPath:               "rename temp provider config",
			wantCleanupAttempt:     true,
			wantDestinationPresent: true,
		},
		{
			name:     "dir sync failure leaves renamed replacement visible",
			existing: true,
			mutate: func(t *testing.T, store *Store, state *tempFileState) {
				t.Helper()
				store.openTempFile = func(path string, mode os.FileMode) (tempFile, error) {
					return os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode)
				}
				store.syncDir = func(string) error { return errors.New("dir sync failed") }
			},
			wantPath:               "sync provider config dir",
			wantDestinationPresent: true,
			wantPersistedChanged:   true,
		},
		{
			name:     "cleanup remove failure keeps primary write error",
			existing: true,
			mutate: func(t *testing.T, store *Store, state *tempFileState) {
				t.Helper()
				state.writeErr = errors.New("write failed")
				store.remove = func(string) error {
					state.cleanupAttempts++
					return errors.New("remove failed")
				}
			},
			wantPath:               "write temp provider config",
			wantCleanupAttempt:     true,
			wantDestinationPresent: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			path := filepath.Join(root, "config", "providers.yaml")
			if tc.existing {
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatalf("MkdirAll() error = %v", err)
				}
				if err := os.WriteFile(path, validExistingBytes, 0o600); err != nil {
					t.Fatalf("WriteFile(existing) error = %v", err)
				}
			}

			store := NewStore(path)
			state := &tempFileState{path: filepath.Join(filepath.Dir(path), ".providers.yaml.tmp")}
			store.remove = func(path string) error {
				state.cleanupAttempts++
				return nil
			}
			store.openTempFile = func(string, os.FileMode) (tempFile, error) {
				file := &tempFileStub{state: state}
				return file, nil
			}
			tc.mutate(t, store, state)

			var expectedRevision *string
			if tc.existing {
				expectedRevision = &existingRevision
			}
			_, _, err := store.Save(context.Background(), profiles, expectedRevision)
			if err == nil {
				t.Fatal("Save() error = nil, want failure")
			}
			if !errors.Is(err, ErrProfileStore) {
				t.Fatalf("Save() error = %v, want ErrProfileStore", err)
			}
			if !strings.Contains(err.Error(), tc.wantPath) {
				t.Fatalf("Save() error = %q, want path %q", err, tc.wantPath)
			}
			if tc.wantCleanupAttempt && state.cleanupAttempts == 0 {
				t.Fatal("cleanup remove attempts = 0, want at least one")
			}

			persistedBytes, readErr := os.ReadFile(path)
			if tc.wantDestinationPresent {
				if readErr != nil {
					t.Fatalf("ReadFile(destination) error = %v", readErr)
				}
				switch {
				case tc.wantPersistedChanged:
					wantBytes := mustCanonicalProfiles(t, profiles)
					if string(persistedBytes) != string(wantBytes) {
						t.Fatalf("persisted bytes = %q, want renamed replacement", string(persistedBytes))
					}
				case tc.existing:
					if string(persistedBytes) != string(validExistingBytes) {
						t.Fatalf("persisted bytes changed to %q", string(persistedBytes))
					}
				}
			} else if !errors.Is(readErr, os.ErrNotExist) {
				t.Fatalf("ReadFile(destination) error = %v, want not exist", readErr)
			}
		})
	}
}

// BDD trace:
//   - Requirements: M5-R01, M5-R02.
//   - Scenario: 5.1.2, 5.1.3.
//   - Test purpose: verify successful saves write a 0600 file, leave no temp
//     artifact, and serialize concurrent whole-document replacements so exactly
//     one caller wins a shared revision.
func TestStoreSavePermissionsAndConcurrentRevisionConflicts(t *testing.T) {
	t.Parallel()

	t.Run("successful save uses 0600 and no leaked temp file", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		path := filepath.Join(root, "providers.yaml")
		store := NewStore(path)
		saved, revision, err := store.Save(context.Background(), []Profile{testLocalOllamaProfile()}, nil)
		if err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		if revision == nil || len(saved) != 1 {
			t.Fatalf("Save() = (%#v, %v)", saved, revision)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat() error = %v", err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("providers.yaml mode = %#o, want 0600", got)
		}
		if _, err := os.Stat(filepath.Join(root, ".providers.yaml.tmp")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("temp artifact err = %v, want not exist", err)
		}
	})

	t.Run("concurrent same revision allows one winner", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		path := filepath.Join(root, "providers.yaml")
		store := NewStore(path)
		initialProfiles := []Profile{testLocalOllamaProfile()}
		_, revision, err := store.Save(context.Background(), initialProfiles, nil)
		if err != nil {
			t.Fatalf("Save(initial) error = %v", err)
		}

		enterRename := make(chan struct{}, 1)
		releaseRename := make(chan struct{})
		store.rename = func(oldPath, newPath string) error {
			enterRename <- struct{}{}
			<-releaseRename
			return os.Rename(oldPath, newPath)
		}

		winnerProfiles := []Profile{testHostedOpenAIProfile()}
		loserProfiles := []Profile{testLocalOllamaProfile(), testHostedOpenAIProfile()}
		type result struct {
			revision *string
			err      error
		}
		firstResult := make(chan result, 1)
		secondResult := make(chan result, 1)

		go func() {
			_, nextRevision, err := store.Save(context.Background(), winnerProfiles, revision)
			firstResult <- result{revision: nextRevision, err: err}
		}()

		<-enterRename
		go func() {
			_, nextRevision, err := store.Save(context.Background(), loserProfiles, revision)
			secondResult <- result{revision: nextRevision, err: err}
		}()

		close(releaseRename)
		first := <-firstResult
		second := <-secondResult
		if first.err != nil {
			t.Fatalf("first Save() error = %v", first.err)
		}
		if !errors.Is(second.err, ErrProfileRevisionConflict) {
			t.Fatalf("second Save() error = %v, want ErrProfileRevisionConflict", second.err)
		}

		loaded, loadedRevision, err := store.Load(context.Background())
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if loadedRevision == nil || first.revision == nil || *loadedRevision != *first.revision {
			t.Fatalf("Load() revision = %v, want %v", loadedRevision, first.revision)
		}
		if len(loaded) != 1 || loaded[0].ID != "hosted_api" {
			t.Fatalf("Load() profiles = %#v", loaded)
		}
	})

	t.Run("concurrent initial create allows one winner", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		path := filepath.Join(root, "providers.yaml")
		store := NewStore(path)
		enterRename := make(chan struct{}, 1)
		releaseRename := make(chan struct{})
		store.rename = func(oldPath, newPath string) error {
			enterRename <- struct{}{}
			<-releaseRename
			return os.Rename(oldPath, newPath)
		}

		type result struct {
			err error
		}
		firstResult := make(chan result, 1)
		secondResult := make(chan result, 1)
		go func() {
			_, _, err := store.Save(context.Background(), []Profile{testLocalOllamaProfile()}, nil)
			firstResult <- result{err: err}
		}()
		<-enterRename
		go func() {
			_, _, err := store.Save(context.Background(), []Profile{testHostedOpenAIProfile()}, nil)
			secondResult <- result{err: err}
		}()
		close(releaseRename)

		first := <-firstResult
		second := <-secondResult
		if first.err != nil {
			t.Fatalf("first Save() error = %v", first.err)
		}
		if !errors.Is(second.err, ErrProfileRevisionConflict) {
			t.Fatalf("second Save() error = %v, want ErrProfileRevisionConflict", second.err)
		}
	})
}

type tempFileState struct {
	path            string
	writeN          int
	writeErr        error
	chmodErr        error
	syncErr         error
	closeErr        error
	cleanupAttempts int
	mu              sync.Mutex
	writes          []byte
}

type tempFileStub struct {
	state *tempFileState
}

func (f *tempFileStub) Write(contents []byte) (int, error) {
	f.state.mu.Lock()
	defer f.state.mu.Unlock()

	n := len(contents)
	if f.state.writeErr != nil {
		n = f.state.writeN
		if n > len(contents) {
			n = len(contents)
		}
	}
	f.state.writes = append(f.state.writes, contents[:n]...)
	return n, f.state.writeErr
}

func (f *tempFileStub) Chmod(os.FileMode) error { return f.state.chmodErr }

func (f *tempFileStub) Sync() error { return f.state.syncErr }

func (f *tempFileStub) Close() error { return f.state.closeErr }

func (f *tempFileStub) Name() string { return f.state.path }

func testLocalOllamaProfile() Profile {
	return Profile{
		ID:      "local_ollama",
		Name:    "Local Ollama",
		Type:    TypeOllama,
		BaseURL: "http://127.0.0.1:11434",
		Auth:    AuthConfig{Type: AuthTypeNone},
		Capabilities: Capabilities{
			Chat:             true,
			Streaming:        false,
			StructuredOutput: false,
			MaxContextTokens: 8192,
		},
	}
}

func testHostedOpenAIProfile() Profile {
	return Profile{
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
}

func mustCanonicalProfiles(t *testing.T, profiles []Profile) []byte {
	t.Helper()

	_, canonical, _, err := ValidateProfiles(profiles)
	if err != nil {
		t.Fatalf("ValidateProfiles() error = %v", err)
	}
	return canonical
}
