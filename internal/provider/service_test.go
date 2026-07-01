package provider

import (
	"context"
	"testing"
)

type storeStub struct {
	loadProfiles []Profile
	loadRevision *string
	loadErr      error
	saveProfiles []Profile
	saveRevision *string
	saveErr      error
	saveInput    []Profile
	saveExpected *string
}

func (s *storeStub) Load(context.Context) ([]Profile, *string, error) {
	return CloneProfiles(s.loadProfiles), s.loadRevision, s.loadErr
}

func (s *storeStub) Save(_ context.Context, profiles []Profile, expectedRevision *string) ([]Profile, *string, error) {
	s.saveInput = CloneProfiles(profiles)
	s.saveExpected = expectedRevision
	return CloneProfiles(s.saveProfiles), s.saveRevision, s.saveErr
}

// BDD trace:
//   - Requirements: M5-R03, M5-R04, M5-R12.
//   - Scenarios: 5.1.1-5.1.4.
//   - Test purpose: verify the provider service exposes public readiness on
//     load/save without leaking or persisting the resolved credential value.
func TestServiceListsSavesAndResolvesProfiles(t *testing.T) {
	t.Parallel()

	revision := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	store := &storeStub{
		loadProfiles: []Profile{
			{
				ID:      "hosted_api",
				Name:    "Hosted API",
				Type:    TypeOpenAICompatible,
				BaseURL: "https://api.example.test/v1",
				Auth:    AuthConfig{Type: AuthTypeBearerEnv, CredentialEnv: "STORYWORK_HOSTED_API_KEY"},
				Capabilities: Capabilities{
					Chat:             true,
					Streaming:        false,
					StructuredOutput: false,
					MaxContextTokens: 32768,
				},
			},
			{
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
			},
		},
		loadRevision: &revision,
		saveProfiles: []Profile{
			{
				ID:      "hosted_api",
				Name:    "Hosted API",
				Type:    TypeOpenAICompatible,
				BaseURL: "https://api.example.test/v1",
				Auth:    AuthConfig{Type: AuthTypeBearerEnv, CredentialEnv: "STORYWORK_HOSTED_API_KEY"},
				Capabilities: Capabilities{
					Chat:             true,
					Streaming:        false,
					StructuredOutput: false,
					MaxContextTokens: 32768,
				},
			},
		},
		saveRevision: &revision,
	}
	service := NewService(store, EnvironmentBroker{
		LookupEnv: func(key string) (string, bool) {
			if key == "STORYWORK_HOSTED_API_KEY" {
				return "super-secret", true
			}
			return "", false
		},
	})

	listed, gotRevision, err := service.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if gotRevision == nil || *gotRevision != revision {
		t.Fatalf("List() revision = %v", gotRevision)
	}
	if listed[0].Readiness != ReadinessReady || listed[1].Readiness != ReadinessReady {
		t.Fatalf("List() readiness = %#v", listed)
	}

	saved, savedRevision, err := service.Save(context.Background(), []Profile{store.loadProfiles[0]}, nil)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if savedRevision == nil || *savedRevision != revision {
		t.Fatalf("Save() revision = %v", savedRevision)
	}
	if len(store.saveInput) != 1 || store.saveInput[0].Readiness != "" {
		t.Fatalf("Save() persisted input = %#v", store.saveInput)
	}
	if saved[0].Readiness != ReadinessReady {
		t.Fatalf("Save() readiness = %#v", saved)
	}

	resolved, found, err := service.Resolve(context.Background(), "hosted_api")
	if err != nil {
		t.Fatalf("Resolve(found) error = %v", err)
	}
	if !found || resolved.Readiness != ReadinessReady || resolved.Credential.Value != "super-secret" {
		t.Fatalf("Resolve(found) = %#v", resolved)
	}

	resolved, found, err = service.Resolve(context.Background(), "missing")
	if err != nil {
		t.Fatalf("Resolve(missing) error = %v", err)
	}
	if found || resolved.Credential.Value != "" {
		t.Fatalf("Resolve(missing) = %#v, found=%v", resolved, found)
	}
}
