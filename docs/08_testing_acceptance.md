# 08 — Testing and Acceptance

## Backend test conventions

Use standard Go tests.

Minimum commands:

```bash
go fmt ./...
go vet ./...
go test ./...
```

Use table-driven tests for decision logic.

Use `t.TempDir()` for project folder and Git/SQLite tests.

Use fakes for model providers.

## Frontend test conventions

Use Vitest for unit/component tests once frontend exists.

Commands:

```bash
npm run lint
npm run typecheck
npm test -- --run
```

## Tests required by milestone

### Milestone 0

Backend:

- Create project writes `project.yaml`.
- Create project initializes Git repo.
- Create project creates `.storywork/index.sqlite`.
- Opening valid project succeeds.
- Opening invalid project returns useful error.
- Index rebuild is idempotent.
- Health endpoint returns ok.

Manual:

1. Start backend.
2. Start frontend.
3. Create project from UI or API.
4. Confirm files on disk.
5. Confirm Git repo exists.
6. Confirm index exists.

### Milestone 1

- Create arc/chapter/scene.
- Reorder preserves stable IDs.
- Display labels update after reorder.
- Files serialize/deserialize correctly.
- Git checkpoint occurs after structural change.
- A missing active project returns 409.
- Invalid reorder permutations leave files and Git history unchanged.
- A dirty project worktree blocks structural mutation.
- A failed checkpoint restores the pre-request canonical files.
- The outline UI renders nesting and sends stable IDs when creating/reordering.

### Milestone 2

- Load scene Markdown.
- Save scene Markdown.
- Edit and preserve the supported YAML front matter fields.
- Reload after save.
- Reject stale revisions without overwriting a newer scene.
- Reject dirty worktrees without changing canonical files or Git history.
- Roll back the scene and index when checkpoint creation fails.
- Create exactly one Git checkpoint per successful explicit save.
- Show loading, dirty, saving, saved, conflict, and error states in the editor.
- Verify the real Vim extension handles a normal-mode edit command.

### Milestone 3

- Add Codex entry.
- Add progression anchored after scene.
- Active state before anchor excludes progression.
- Active state after anchor includes progression.
- Reordering scenes does not break anchor.

### Milestone 4

- Agent applicability filters by surface.
- Agent applicability filters by selection size.
- Context policy forbids global RAG for line polish.
- Mock provider patch requires acceptance.
- Reject leaves scene unchanged.
- Accept updates scene file.
- Concurrent accept/accept and accept/reject attempts allow exactly one
  terminal decision.
- A byte-identical provider replacement is rejected before any transient run is
  stored.
- Opening the preview moves focus to the named review region once.

Named automated evidence:

- `internal/agent/registry_test.go`: `TestLoaderStrictlyLoadsAndRejectsRegistryFiles`,
  `TestLoaderRejectsStyleWithoutRequiredTemperature`
- `internal/agent/model_test.go`: `TestRegistryValidationApplicabilityAndContext`
- `internal/agent/provider_test.go`:
  `TestMockProviderProducesDeterministicReplacementAndHonorsCancellation`
- `internal/action/service_test.go`: `TestServiceRunRejectAndAcceptFlow`,
  `TestServiceRejectsStaleSelectionsAndReleasesFailedAcceptClaims`,
  `TestRunStoreEvictsTerminalRunsAndRejectsLiveCapacity`,
  `TestRunStoreRejectsDuplicateRunIDs`,
  `TestConcurrentAcceptsAllowExactlyOneRunClaim`,
  `TestConcurrentAcceptAndRejectAllowExactlyOneTerminalDecision`,
  `TestRunRejectsByteIdenticalProviderOutputWithoutStoringRun`
- `internal/action/integration_test.go`:
  `TestMilestone4ActionFlowWithRealAdapters`
- `internal/story/scene_selection_test.go`:
  `TestValidateAndReplaceMarkdownSelection`
- `internal/story/scene_patch_rollback_service_test.go`:
  `TestAcceptScenePatchHandlesEveryPersistenceFailureBoundary`
- `internal/api/action_handler_test.go`: `TestActionRoutesReturnExactJSONShapes`,
  `TestActionRouteStatusMapping`,
  `TestActionRoutesRejectMalformedContractInputs`,
  `TestActionRegistryLoadFailuresReturnInternalServerError`
- `web/src/editor/selection.test.ts`:
  `counts words and converts UTF-8 byte ranges for multibyte selections`
- `web/src/editor/SceneEditor.actions.boundary.test.tsx`:
  `runs, rejects, and accepts scene actions through the fetch boundary`,
  `disables actions for dirty drafts`

This evidence covers strict registry schema rejection, exact HTTP contracts,
service-level terminal-decision races, run-level no-op rejection, UTF-8 byte
selection handling, rollback at write/index/Git boundaries, inline preview
focus placement, and the no-second-save editor acceptance flow.

### Milestone 5

- `internal/provider/store_test.go`: canonical bytes/revisions, strict YAML,
  config path, symlink and invalid-state rejection, optimistic saves,
  mkdir/temp/write/chmod/sync/close/rename/dir-sync fault injection,
  concurrent-save conflicts, permissions, credential readiness, and
  punctuation-safe scalar round trips.
- `internal/provider/service_test.go`: public readiness and credential
  non-disclosure across list, save, and resolve boundaries.
- `internal/agent/model_test.go`: version-2 schema and ordered pure
  compatibility reason codes.
- `internal/agent/provider_test.go`: mock/OpenAI/Ollama routing, exact prompts
  and headers, output normalization, provider metadata tolerance, trailing JSON
  rejection, incompatible-profile rejection, request/response size limits,
  response-body closure, and safe failure classification.
- `internal/action/service_test.go`: capability-aware filtering, run-time
  readiness revalidation, provider identity, style ordering by style name then
  ID, invalid/no-op output rejection, and unchanged accept/reject transaction
  behavior.
- `internal/action/milestone5_integration_test.go`: real provider profile save,
  no-auth OpenAI outbound execution, transient reject, explicit accept, Git
  checkpoint, and persisted scene/provider reload through real adapters.
- `internal/api/provider_handler_test.go`: exact GET/PUT JSON, null revisions,
  strict nested required fields, malformed requests, 1 MiB body-limit
  boundary, conflicts, methods, and safe status mapping without an active
  project.
- `web/src/providers/ProviderWorkbench.test.tsx` and
  `web/src/App.providers_navigation.test.tsx`: empty/edit/save/conflict states,
  invalid-save prevention, saved-to-dirty transitions, dirty navigation, and
  beforeunload listener removal after save/unmount.
- `web/src/editor/SceneEditor.actions.boundary.test.tsx`: non-secret provider
  identity in the existing preview with unchanged reject/accept behavior.

### Milestone 6

- Markdown folder import finds `.md` files.
- Imported notes are chunked.
- Extraction produces candidates.
- Candidates do not mutate canon until accepted.
- Merge candidate combines aliases/tags correctly.

### Milestone 7

- Context builder uses active Codex state.
- Same character has different context before/after progression.
- Paragraph action uses minimal context.
- Chapter action can use timeline-aware RAG.

### Milestone 8

- Create Git branch.
- Edit branch without changing canon branch.
- Diff branch against canon.
- Discard branch safely.
- Promote selected file manually.

### Milestone 9

- Project backup snapshot works.
- SQLite index rebuild works after deletion.
- Markdown export works.
- AI run log shows context packs/prompt summary.

## MVP acceptance script

1. Create a project.
2. Import a folder of Markdown notes.
3. Extract candidates.
4. Approve at least one Codex entry.
5. Create an outline with at least one arc, chapter, and scene.
6. Edit the scene in the Vim-friendly editor.
7. Add a timeline progression to a Codex entry.
8. Confirm active Codex differs before/after the progression.
9. Select a paragraph and run a style/agent patch.
10. Accept the patch.
11. Create a what-if branch.
12. Change a scene in the branch.
13. Compare branch to canon.
14. Promote or discard the branch.
15. Rebuild index.
16. Export Markdown.

Pass only if the author remains in control at every AI mutation point.
