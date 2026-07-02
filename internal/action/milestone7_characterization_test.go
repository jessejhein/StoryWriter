package action

// BDD Scenario: 7.1.1 - Preview minimal Line Polish context
// Requirements: M7-R02, M7-R10
// Test purpose: Lock existing Line Polish provider context to selected text and
// style instructions only before Milestone 7 refactors context assembly.

// BDD Scenario: 7.1.2 - Run with the previewed scope
// Requirements: M7-R02, M7-R10, M7-R15
// Test purpose: Lock existing run storage, redacted summaries, and patch
// accept/reject semantics before Milestone 7 generalizes action scopes.

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"storywork/internal/agent"
)

// Test: existing Line Polish sends only selected text and style to the provider.
// Requirements: M7-R02.
func TestM7ExistingLinePolishBuildsSelectionOnlyPacket(t *testing.T) {
	t.Parallel()

	scene := testActionScene()
	scene.Markdown = "Alpha beta gamma delta echo foxtrot.\n"
	provider := &fakeProvider{response: agent.GenerateResponse{Replacement: "Mock polished: Alpha beta"}}
	service := newActionTestService(scene, provider, &fakeAcceptor{}, NewRunStore(), &fakeRunIDGenerator{next: "run_0123456789abcdef0123"})

	request := selectionRunRequest(scene)
	request.Selection = Selection{
		StartByte: 0,
		EndByte:   len([]byte("Alpha beta")),
		Text:      "Alpha beta",
	}

	if _, err := service.Run(context.Background(), request); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if provider.calls != 1 {
		t.Fatalf("provider calls = %d, want 1", provider.calls)
	}

	packet := provider.request.Packet
	if packet.SelectedText != "Alpha beta" {
		t.Fatalf("provider packet selected text = %q, want Alpha beta", packet.SelectedText)
	}
	if packet.Style.ID != "precise_editor" {
		t.Fatalf("provider packet style = %#v, want precise_editor", packet.Style)
	}
	if strings.Contains(packet.SelectedText, "gamma") || strings.Contains(packet.SelectedText, "delta") {
		t.Fatalf("provider packet included wider scene prose: %q", packet.SelectedText)
	}

	summary := provider.request.Summary
	if len(summary.PacksUsed) != 2 {
		t.Fatalf("provider summary packs_used = %#v, want exactly two packs", summary.PacksUsed)
	}
	if summary.PacksUsed[0] != agent.ContextSelectedText || summary.PacksUsed[1] != agent.ContextStyleSheet {
		t.Fatalf("provider summary packs_used = %#v, want selected_text and style_sheet only", summary.PacksUsed)
	}
	if summary.RAGMode != agent.RAGModeNone {
		t.Fatalf("provider summary rag_mode = %q, want none", summary.RAGMode)
	}

	forbiddenPacks := []agent.ContextPack{
		agent.ContextCurrentScene,
		agent.ContextCurrentChapter,
		agent.ContextChapterSummary,
		agent.ContextOutlineNeighbor,
		agent.ContextGlobalCodexRAG,
		agent.ContextRawImportNotes,
		agent.ContextSurrounding,
	}
	for _, pack := range forbiddenPacks {
		for _, used := range summary.PacksUsed {
			if used == pack {
				t.Fatalf("provider summary included forbidden pack %q", pack)
			}
		}
	}
}

// Test: existing runs retain redacted context summaries without prompt prose.
// Requirements: M7-R10.
func TestM7ExistingLinePolishRunStoresRedactedExistingSummary(t *testing.T) {
	t.Parallel()

	scene := testActionScene()
	provider := &fakeProvider{response: agent.GenerateResponse{Replacement: "Mock polished: Alpha beta"}}
	service := newActionTestService(scene, provider, &fakeAcceptor{}, NewRunStore(), &fakeRunIDGenerator{next: "run_0123456789abcdef0123"})

	run, err := service.Run(context.Background(), selectionRunRequest(scene))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	summary := run.ContextSummary
	if len(summary.PacksUsed) != 2 || summary.PacksUsed[0] != agent.ContextSelectedText || summary.PacksUsed[1] != agent.ContextStyleSheet {
		t.Fatalf("run context summary packs_used = %#v", summary.PacksUsed)
	}
	if summary.RAGMode != agent.RAGModeNone {
		t.Fatalf("run context summary rag_mode = %q, want none", summary.RAGMode)
	}

	encoded, err := json.Marshal(run)
	if err != nil {
		t.Fatalf("json.Marshal(run) error = %v", err)
	}
	payload := string(encoded)
	for _, forbidden := range []string{
		"Alpha beta",
		"Mock polished",
		"Prompt",
		"original_text",
		"replacement",
		"system_prompt",
	} {
		if strings.Contains(payload, forbidden) {
			t.Fatalf("serialized run leaked %q: %s", forbidden, payload)
		}
	}
	if !strings.Contains(payload, `"packs_used":["selected_text","style_sheet"]`) {
		t.Fatalf("serialized run omitted redacted packs_used: %s", payload)
	}
	if !strings.Contains(payload, `"rag_mode":"none"`) {
		t.Fatalf("serialized run omitted rag_mode: %s", payload)
	}
}

// Test: existing patch accept and reject semantics remain stable.
// Requirements: M7-R15.
func TestM7ExistingPatchAcceptAndRejectSemanticsRemainStable(t *testing.T) {
	t.Parallel()

	scene := testActionScene()
	replacement := "Mock polished: Alpha beta"
	acceptor := &fakeAcceptor{scene: acceptedActionScene(scene, replacement)}
	provider := &fakeProvider{response: agent.GenerateResponse{Replacement: replacement}}
	runStore := NewRunStore()
	service := newActionTestService(scene, provider, acceptor, runStore, &fakeRunIDGenerator{next: "run_0123456789abcdef0123"})

	run, err := service.Run(context.Background(), selectionRunRequest(scene))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if run.Status != RunPending {
		t.Fatalf("run status = %q, want pending", run.Status)
	}

	rejected, err := service.Reject(context.Background(), run.RunID)
	if err != nil {
		t.Fatalf("Reject() error = %v", err)
	}
	if rejected.Status != RunRejected {
		t.Fatalf("rejected status = %q, want rejected", rejected.Status)
	}
	if acceptorCalls(acceptor) != 0 {
		t.Fatalf("reject invoked acceptor %d times, want 0", acceptorCalls(acceptor))
	}
	if _, err := service.Reject(context.Background(), run.RunID); !errors.Is(err, ErrRunConflict) {
		t.Fatalf("Reject(again) error = %v, want ErrRunConflict", err)
	}

	runStore = NewRunStore()
	acceptor = &fakeAcceptor{scene: acceptedActionScene(scene, replacement)}
	service = newActionTestService(scene, provider, acceptor, runStore, &fakeRunIDGenerator{next: "run_ffffffffffffffffffff"})
	run, err = service.Run(context.Background(), selectionRunRequest(scene))
	if err != nil {
		t.Fatalf("Run(second) error = %v", err)
	}

	accepted, saved, err := service.Accept(context.Background(), run.RunID, scene.Revision)
	if err != nil {
		t.Fatalf("Accept() error = %v", err)
	}
	if accepted.Status != RunAccepted {
		t.Fatalf("accepted status = %q, want accepted", accepted.Status)
	}
	if acceptor.request.StartByte != 0 || acceptor.request.EndByte != len([]byte("Alpha beta")) {
		t.Fatalf("accept patch range = [%d,%d), want [0,%d)", acceptor.request.StartByte, acceptor.request.EndByte, len([]byte("Alpha beta")))
	}
	if acceptor.request.OriginalText != "Alpha beta" || acceptor.request.ReplacementText != replacement {
		t.Fatalf("accept patch text = %#v", acceptor.request)
	}
	if saved.Revision != acceptor.scene.Revision {
		t.Fatalf("saved scene revision = %q, want %q", saved.Revision, acceptor.scene.Revision)
	}
	if _, _, err := service.Accept(context.Background(), run.RunID, scene.Revision); !errors.Is(err, ErrRunConflict) {
		t.Fatalf("Accept(again) error = %v, want ErrRunConflict", err)
	}
}
