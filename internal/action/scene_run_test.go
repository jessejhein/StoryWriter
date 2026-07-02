// BDD Scenario: 7.2.3 - Review and accept one scene replacement
// Requirements: M7-R03, M7-R10, M7-R15
// Test purpose: Scene rewrite runs rebuild context and store redacted manifests with full body patches.

package action

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"storywork/internal/agent"
	"storywork/internal/contextpack"
	"storywork/internal/project"
	"storywork/internal/story"
)

type sceneRunMaterialSource struct {
	result           story.ContextMaterialResult
	calls            int
	expectedRevision string
}

func (s *sceneRunMaterialSource) LoadSelectionMaterial(context.Context, story.SelectionMaterialRequest) (story.ContextMaterialResult, error) {
	return story.ContextMaterialResult{}, nil
}

func (s *sceneRunMaterialSource) LoadSceneMaterial(_ context.Context, _ string, sceneRevision string) (story.ContextMaterialResult, error) {
	s.calls++
	if s.expectedRevision != "" && sceneRevision != s.expectedRevision {
		return story.ContextMaterialResult{}, fmt.Errorf("scene revision changed: %w", story.ErrStaleRevision)
	}
	return s.result, nil
}

func (s *sceneRunMaterialSource) LoadChapterMaterial(context.Context, string) (story.ContextMaterialResult, error) {
	return story.ContextMaterialResult{}, nil
}

type fakeBodyAcceptor struct {
	request story.AcceptSceneBodyPatchRequest
	scene   story.SceneDocument
	calls   int
}

func (a *fakeBodyAcceptor) AcceptSceneBodyPatch(_ context.Context, request story.AcceptSceneBodyPatchRequest) (story.SceneDocument, error) {
	a.calls++
	a.request = request
	return a.scene, nil
}

func (a *fakeBodyAcceptor) AcceptScenePatch(context.Context, story.AcceptScenePatchRequest) (story.SceneDocument, error) {
	return story.SceneDocument{}, nil
}

// Test: scene rewrite run rebuilds and revalidates context before provider execution.
// Requirements: M7-R03.
func TestSceneRewriteRunRebuildsAndRevalidatesContext(t *testing.T) {
	t.Parallel()

	material := &sceneRunMaterialSource{result: story.ContextMaterialResult{
		Material: contextpack.Material{
			Scope: contextpack.ScopeScene, SceneMarkdown: "Ann arrives.\n",
			SceneOrder: []contextpack.SceneOrderRef{{ID: "scn_0123456789abcdef0123"}},
		},
		TargetRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}}
	provider := &fakeProvider{response: agent.GenerateResponse{Replacement: "Mock rewritten: Ann arrives."}}
	service := newSceneRunTestService(t, material, provider)

	run, err := service.RunTagged(context.Background(), TaggedRunRequest{
		AgentID: "scene_rewrite", StyleID: "precise_editor",
		Target: TaggedTarget{
			Scope: contextpack.ScopeScene,
			Scene: &SceneTarget{
				SceneID:       "scn_0123456789abcdef0123",
				SceneRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
		},
	})
	if err != nil {
		t.Fatalf("RunTagged() error = %v", err)
	}
	if material.calls != 1 {
		t.Fatalf("material calls = %d, want 1", material.calls)
	}
	if provider.calls != 1 || provider.request.TypedPacket == nil {
		t.Fatalf("provider was not called with typed packet: %#v", provider.request)
	}
	if run.Scope != contextpack.ScopeScene || run.Replacement != "Mock rewritten: Ann arrives." {
		t.Fatalf("run = %#v", run)
	}
}

// Test: scene rewrite run stores redacted manifest and full body patch fields.
// Requirements: M7-R10.
func TestSceneRewriteRunStoresRedactedManifestAndFullBodyPatch(t *testing.T) {
	t.Parallel()

	material := &sceneRunMaterialSource{result: story.ContextMaterialResult{
		Material: contextpack.Material{
			Scope: contextpack.ScopeScene, SceneMarkdown: "Ann arrives.\n",
			SceneOrder: []contextpack.SceneOrderRef{{ID: "scn_0123456789abcdef0123"}},
		},
		TargetRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}}
	provider := &fakeProvider{response: agent.GenerateResponse{Replacement: "Mock rewritten: Ann arrives."}}
	service := newSceneRunTestService(t, material, provider)

	run, err := service.RunTagged(context.Background(), TaggedRunRequest{
		AgentID: "scene_rewrite", StyleID: "precise_editor",
		Target: TaggedTarget{
			Scope: contextpack.ScopeScene,
			Scene: &SceneTarget{
				SceneID:       "scn_0123456789abcdef0123",
				SceneRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
		},
	})
	if err != nil {
		t.Fatalf("RunTagged() error = %v", err)
	}
	if run.OriginalText != "Ann arrives.\n" || run.Replacement != "Mock rewritten: Ann arrives." {
		t.Fatalf("body patch fields = %#v", run)
	}
	if run.Manifest.Scope != contextpack.ScopeScene || len(run.Manifest.PacksUsed) == 0 {
		t.Fatalf("manifest = %#v", run.Manifest)
	}
	encoded, err := json.Marshal(run)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	payload := string(encoded)
	for _, forbidden := range []string{"Ann arrives", "Mock rewritten", "original_text", "replacement"} {
		if strings.Contains(payload, forbidden) {
			t.Fatalf("serialized run leaked %q: %s", forbidden, payload)
		}
	}
}

func newSceneRunTestService(t *testing.T, material ContextMaterialSource, provider *fakeProvider) *Service {
	t.Helper()
	return NewService(
		&fakeSession{project: project.Project{Path: "/tmp/test"}, ok: true},
		&fakeLoader{registry: agent.Registry{Agents: []agent.Agent{testSceneRewriteAgent()}, Styles: []agent.Style{testPreciseEditorStyle()}}},
		&fakeSceneLoader{scene: testActionScene()},
		&fakeAcceptor{},
		provider,
		nil,
		NewRunStore(),
		&fakeRunIDGenerator{next: "run_0123456789abcdef0123"},
	).WithMaterialSource(material).WithContextBuilder(contextpack.NewBuilder()).WithBodyAcceptor(&fakeBodyAcceptor{
		scene: story.SceneDocument{ID: "scn_0123456789abcdef0123", Revision: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
	})
}
