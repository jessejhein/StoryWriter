# Milestone 4 Task Prompt - Agent/Style Registry and Mock AI Actions

Implement only Milestone 4. Milestones 0 through 3 are complete and are
regression constraints. Do not add real provider HTTP adapters, credentials,
embeddings, RAG, imports, chat, or what-if branches.

This document is the durable implementation contract. When a general project
document is less specific, this document controls Milestone 4 behavior. Follow
the ordered TDD sequence in `.plans/milestone_4_implementation.md`.

## Outcome

An author can select canonical scene prose, see only applicable agents, choose
a compatible style, run a deterministic mock action, inspect the replacement
and exact context summary, then explicitly accept or reject it. Running an
action never changes canon. Acceptance uses the existing scene revision,
shared mutation lock, rollback, index rebuild, and Git checkpoint guarantees.

## Starting state and lessons carried forward

The June 29, 2026 baseline passes `make check` with the preferred Go toolchain
and a writable Go build cache. Milestone 4 must preserve all existing behavior.

Milestone 3 showed that boundary-only mocks and broad traceability claims leave
important behavior unproven. Milestone 4 therefore requires:

- pure applicability and context-policy decisions,
- strict YAML and JSON contracts,
- a provider-neutral model interface,
- real frontend HTTP-boundary tests rather than mocking `web/src/api.ts`,
- a real filesystem/Git/SQLite acceptance path for patch acceptance,
- exact proof that run and reject do not mutate canon,
- named test evidence mapped to every BDD scenario before completion.

## Scope boundaries

In scope:

- strict, read-only registries for project-local `agents/*.yaml` and
  `styles/*.yaml`,
- the built-in Line Polish selection agent and Precise Editor mock style,
- a built-in Chapter Refiner definition used to prove scope applicability,
- pure applicability filtering by surface, input scope, and word count,
- minimal context assembly for selection actions,
- a provider-neutral text-generation interface and deterministic mock adapter,
- transient in-process action runs,
- patch preview, copy, explicit accept, and explicit reject,
- revision-safe acceptance into one canonical scene.

Out of scope:

- creating, editing, or deleting agent/style files through the UI or API,
- provider profiles, capability discovery, credentials, or network calls,
- streaming, tool calls, structured model output, retries, or fallback models,
- RAG, Codex injection, token-budget trimming, chapter action execution,
- persistence or recovery of action runs across backend restarts,
- accepting patches into anything except scene Markdown,
- multiple patches in one run, patch rebasing, fuzzy matching, or autosave,
- any Milestone 5 behavior.

## Requirements

| ID | Requirement |
| --- | --- |
| M4-R01 | Strictly load and deterministically list project-local agent definitions. |
| M4-R02 | Strictly load and deterministically list project-local style definitions. |
| M4-R03 | Reject malformed, unsupported, duplicate, or unsafe registry documents without silently skipping them. |
| M4-R04 | Compute agent applicability as a pure decision using surface, input scope, and word count. |
| M4-R05 | Offer selection agents only for a non-empty canonical editor selection within configured word limits. |
| M4-R06 | Assemble only required and allowed context packs and enforce forbidden packs in pure code. |
| M4-R07 | Route generation through a provider-neutral interface and include no provider-specific shape in application or HTTP models. |
| M4-R08 | Provide a deterministic, offline mock provider for the Line Polish patch flow. |
| M4-R09 | Validate run requests against the exact canonical scene revision and UTF-8 byte selection. |
| M4-R10 | Store action runs only in transient process memory and expose no run data in story-project files or SQLite. |
| M4-R11 | Running an action returns a reviewable patch and inspectable context summary without mutating canon, index, or Git. |
| M4-R12 | Rejecting a pending run is idempotence-safe and leaves canonical state, index, and Git unchanged. |
| M4-R13 | Accepting a pending run replaces only the selected Markdown bytes and preserves scene identity and front matter. |
| M4-R14 | Patch acceptance uses the shared story mutation lock, clean-worktree check, optimistic revision, atomic write, index rebuild, rollback, and exactly one Git commit. |
| M4-R15 | A run can reach exactly one terminal state; duplicate or conflicting accept/reject attempts cannot mutate canon twice. |
| M4-R16 | Expose the exact registry, availability, run, accept, and reject HTTP contracts below. |
| M4-R17 | Add an accessible editor action menu, style choice, diff preview, context disclosure, copy, accept, reject, and explicit UI states. |
| M4-R18 | Disable actions for dirty editor drafts so a patch is never generated against text different from canonical bytes. |
| M4-R19 | Preserve all Milestone 0-3 behavior and keep check, race, and diff-validation suites green. |
| M4-R20 | Maintain scenario-to-test evidence and update durable docs and project status before declaring completion. |

## BDD stories

### Story 4.1 - Inspect agents and styles

As an author, I want to see the available workflows and voices so that I know
what the application can run.

#### Scenario 4.1.1 - List strict registries

Requirements: M4-R01, M4-R02, M4-R16.

```gherkin
Given an active project contains valid agent and style files
When I request the agent and style registries
Then all definitions are returned in deterministic name then ID order
And their task, scope, context, control, and mock model settings are visible
And reading the registries changes no project state
```

#### Scenario 4.1.2 - Reject malformed registry state

Requirements: M4-R03, M4-R16.

```gherkin
Given an agent or style file is malformed, unsupported, duplicated, or unsafe
When I list the affected registry or request available actions
Then the request fails with a contextual internal error
And the invalid definition is not skipped or repaired
```

### Story 4.2 - Offer applicable actions

As an author, I want action choices to match my current editing scope so that I
do not invoke an inappropriate workflow.

#### Scenario 4.2.1 - Offer Line Polish for a valid selection

Requirements: M4-R04, M4-R05, M4-R16.

```gherkin
Given a clean loaded scene and a 20 to 1500 word canonical selection
When I request available editor selection actions
Then Line Polish is available
And Chapter Refiner is not available
```

#### Scenario 4.2.2 - Filter by scope and size

Requirements: M4-R04, M4-R05.

```gherkin
Given an editor request has no selection, fewer than 20 words, more than 1500 words, or chapter scope
When applicability is computed
Then Line Polish is not offered
And a chapter-scoped definition is considered only for chapter scope
```

Applicability must return reasons for excluded agents in the domain result so
tests and future diagnostics can inspect the decision. The public availability
response returns only applicable actions.

### Story 4.3 - Run a minimal-context mock patch

As an author, I want to preview a deterministic AI-shaped edit so that the
workflow can be proven without credentials or network access.

#### Scenario 4.3.1 - Assemble minimal context

Requirements: M4-R06, M4-R09.

```gherkin
Given a valid Line Polish selection
When its context packet is assembled
Then it contains selected_text and style_sheet
And it contains no Codex, outline, import-note, full-scene, or RAG content
And forbidden packs cannot be added by optional-context input
```

#### Scenario 4.3.2 - Run through the mock provider

Requirements: M4-R07, M4-R08, M4-R10, M4-R11, M4-R16.

```gherkin
Given Line Polish, Precise Editor, and a valid canonical selection
When I run the action
Then the provider-neutral interface receives the assembled context
And the response contains the original text and deterministic replacement
And the context summary reports selected_text and style_sheet with RAG mode none
And no canonical file, index, staging state, or Git history changes
```

The production mock adapter returns `Mock polished: ` followed by the selected
text with leading and trailing Unicode whitespace removed. This conspicuous
output proves plumbing, not prose quality. If that result is byte-identical to
the original selection, the run fails as a no-op.

#### Scenario 4.3.3 - Reject stale or mismatched run input

Requirements: M4-R09, M4-R11.

```gherkin
Given the requested revision, byte range, or selected text does not match the canonical scene
When I run an action
Then the request fails without calling the provider
And no run or project mutation is created
```

### Story 4.4 - Keep patch decisions under author control

As an author, I want to accept or reject a preview explicitly so that AI output
never silently becomes canon.

#### Scenario 4.4.1 - Reject a patch

Requirements: M4-R10, M4-R12, M4-R15, M4-R16.

```gherkin
Given a pending patch run
When I reject it
Then the run becomes rejected
And scene bytes, index state, staging state, and Git history remain unchanged
And later accept or reject attempts return a conflict
```

#### Scenario 4.4.2 - Accept a patch

Requirements: M4-R13, M4-R14, M4-R15, M4-R16.

```gherkin
Given a pending patch run and an unchanged clean canonical scene
When I explicitly accept it with the run's scene revision
Then only the selected Markdown byte range is replaced
And scene identity, title, chapter, and front matter are preserved
And the index is rebuilt
And exactly one Git commit named "Accept AI patch <run-id>" is created
And the run becomes accepted
And the saved scene with its new revision is returned
```

#### Scenario 4.4.3 - Refuse unsafe acceptance

Requirements: M4-R14, M4-R15.

```gherkin
Given a pending run and a dirty worktree, stale revision, write failure, index failure, or checkpoint failure
When I try to accept it
Then canon and Git history are unchanged or restored exactly
And the run remains pending so the author may reload or retry
And no successful response is reported
```

Concurrent accept/reject requests must be race-safe. Exactly one may claim a
pending run, and a failed accept must release that claim back to pending.

### Story 4.5 - Complete the editor workflow

As an author, I want the complete action flow in the scene editor so that I can
review changes in context before deciding.

#### Scenario 4.5.1 - Preview and inspect

Requirements: M4-R17, M4-R18.

```gherkin
Given a clean loaded scene with a valid CodeMirror selection
When I open the action menu, choose Line Polish and Precise Editor, and run it
Then I see loading and running states
And I see original and replacement text in an accessible diff preview
And I can inspect the context packs and RAG mode
And I can copy the replacement without changing canon
```

#### Scenario 4.5.2 - Accept or reject in the editor

Requirements: M4-R17, M4-R18.

```gherkin
Given a patch preview is open
When I reject it
Then the editor remains unchanged and the preview closes
When I instead accept a fresh patch
Then the returned canonical scene replaces the editor baseline and draft
And the editor reports Saved without a second save
```

Dirty drafts disable action discovery and run controls with an explanation.
Selection changes after a run starts must not alter that run's preview. Stale
responses after scene navigation must be ignored.

## Canonical registry schemas

Registry files are canonical, Git-tracked project files. Milestone 4 reads but
does not mutate them. Use YAML with `KnownFields(true)` or an equivalent strict
decoder, reject multiple YAML documents, and reject symlinks and non-regular
files. Only direct `*.yaml` children of `agents/` and `styles/` are loaded.

IDs match `^[a-z][a-z0-9_]{0,63}$`; names and descriptions are trimmed,
non-empty UTF-8 strings with maximum lengths 100 and 500 runes. Duplicate IDs
are errors even when filenames differ. Filename stems need not equal IDs.

Agent schema version 1:

```yaml
version: 1
id: line_polish
name: Line Polish
description: Rewrite selected prose for clarity, cadence, and flow while preserving meaning.
applies_when:
  surfaces: [editor]
  input_scopes: [selection]
  min_words: 20
  max_words: 1500
context_policy:
  required: [selected_text, style_sheet]
  optional: [surrounding_paragraphs]
  forbidden: [global_codex_rag, raw_import_notes]
rag_policy:
  mode: none
control:
  output_mode: patch
  requires_acceptance: true
  can_modify_canon: false
output:
  type: replacement_text
  requires_diff_preview: true
```

For Milestone 4, allowed values are:

- surfaces: `editor` and `chapter_view`,
- input scopes: `selection` and `chapter`,
- context packs: the names in `docs/04_agent_style_system.md`,
- RAG mode: `none` only,
- output mode: `patch` only,
- output type: `replacement_text` and `revised_text`.

Lists are required, non-empty where semantically required, duplicate-free, and
preserve document order in detail responses. Required, optional, and forbidden
context sets must be pairwise disjoint. Word bounds are inclusive, non-negative,
and `min_words <= max_words`. Milestone 4 executable selection agents must
require `selected_text` and `style_sheet`, forbid `global_codex_rag` and
`raw_import_notes`, require acceptance and diff preview, and set
`can_modify_canon: false`.

Style schema version 1:

```yaml
version: 1
id: precise_editor
name: Precise Editor
provider_profile_id: mock_default
model: mock
parameters:
  temperature: 0.2
system_prompt: >
  You are a careful prose editor. Preserve facts, continuity, POV, and intent.
```

`provider_profile_id` must be `mock_default` and `model` must be `mock` in
Milestone 4. Temperature is required and within 0 through 2 inclusive. The
trimmed system prompt is required and limited to 4000 runes. Unknown parameter
keys are rejected.

Update the embedded starter templates to these schemas and add a strict
`chapter_refiner.yaml` definition with `chapter_view`/`chapter`, 1000-12000
words, and no executable Milestone 4 run path. Existing projects using the
pre-version template are not silently migrated: opening still succeeds, but
registry endpoints report a contextual unsupported-schema error until the
author updates the file. Document this intentional compatibility boundary.

## Applicability and selection rules

Availability input contains:

- `surface`: `editor` or `chapter_view`,
- `input_scope`: `selection` or `chapter`,
- `scene_id`: required for editor selection,
- `selection_words`: an integer greater than or equal to zero.

Word count is computed by splitting trimmed text on Unicode whitespace. The
frontend may estimate for display, but the run endpoint recomputes it from the
canonical selected bytes and is authoritative.

Selection offsets are zero-based half-open UTF-8 byte offsets into the scene's
Markdown body, not the whole scene file and not JavaScript UTF-16 indices. The
frontend converts CodeMirror positions by UTF-8 encoding the Markdown prefix.
Offsets must be on UTF-8 rune boundaries and `start_byte < end_byte`.

## Context and provider boundary

Represent context as typed packs, not a raw prompt string or `map[string]any`.
The Line Polish packet contains only:

- the exact selected canonical text,
- the complete selected style definition needed by the provider.

`surrounding_paragraphs` is optional in the registry but deliberately not
requested or assembled in Milestone 4. The context summary lists packs actually
used, not all packs allowed by the agent.

Define the model boundary where consumed. It accepts a provider-neutral request
containing agent instructions, style instructions/parameters, and typed context,
and returns replacement text. It must accept `context.Context`. Do not expose
mock-specific request or response fields outside the adapter.

## Transient run model

Run IDs have the same opaque 20-lowercase-hex suffix convention as other IDs:
`run_[0-9a-f]{20}`. Inject ID generation in tests.

A run stores the run ID, agent/style IDs, scene ID, scene revision, byte range,
original text, replacement text, context summary, and state. States are
`pending`, `accepting`, `accepted`, and `rejected`. The store is an in-memory,
mutex-protected boundary owned by the application process. It is bounded to
1000 terminal runs by evicting the oldest terminal run before insertion; never
evict a pending or accepting run. If capacity contains only live runs, reject a
new run with a service-unavailable error.

Do not write prompts, selections, replacements, or run metadata into the story
project, SQLite index, browser storage, or logs. API responses necessarily
return review data to the current browser session. Restarting the backend loses
all runs and a later decision returns not found.

## Mutation transaction contract

Run and reject perform no project mutation. Accept must delegate to the story
service rather than writing a scene from the action package.

Acceptance order:

1. validate route ID and strict request syntax,
2. atomically claim the pending run as accepting,
3. resolve the active project and acquire the existing shared story mutation lock,
4. verify the story-project Git worktree is clean,
5. strictly reload the outline and canonical scene,
6. compare both the run revision and request `expected_revision`,
7. verify the stored byte range still selects the stored original text,
8. construct the complete next scene in memory by replacing only that range,
9. reject a byte-identical no-op,
10. snapshot and atomically replace the scene file,
11. rebuild the disposable index,
12. commit all project changes as `Accept AI patch <run-id>`,
13. mark the run accepted and return the exact saved scene representation.

Do no fallible project work after a successful commit. If write, index, or Git
fails, restore exact scene bytes, unstage app-staged changes, rebuild the index,
join rollback errors to the initiating error, and release the run back to
pending. A failure to mark accepted after commit would violate the no-fallible-
work rule; design the in-memory state transition so finalization cannot fail.

## Exact HTTP API

All routes require an active project. Registry and availability routes are
read-only. JSON mutation bodies use a 1 MiB limit, strict unknown-field and
trailing-value rejection, and the existing `{"error":"useful message"}` errors.

```http
GET /api/agents
GET /api/styles
```

Responses:

```json
{"agents":[{"id":"line_polish","name":"Line Polish","description":"Rewrite selected prose.","surfaces":["editor"],"input_scopes":["selection"],"min_words":20,"max_words":1500,"required_context":["selected_text","style_sheet"],"optional_context":["surrounding_paragraphs"],"forbidden_context":["global_codex_rag","raw_import_notes"],"rag_mode":"none","output_mode":"patch","requires_acceptance":true}]}
```

```json
{"styles":[{"id":"precise_editor","name":"Precise Editor","provider_profile_id":"mock_default","model":"mock","temperature":0.2,"system_prompt":"You are a careful prose editor."}]}
```

```http
GET /api/actions/available?surface=editor&input_scope=selection&scene_id=scn_0123456789abcdef0123&selection_words=200
```

```json
{"actions":[{"agent_id":"line_polish","name":"Line Polish","description":"Rewrite selected prose.","output_mode":"patch","requires_acceptance":true,"style_ids":["precise_editor"]}]}
```

`style_ids` contains all strictly loaded Milestone 4 mock styles, sorted by
style name then ID. Actions are sorted by agent name then ID.

```http
POST /api/actions/run
```

```json
{"agent_id":"line_polish","style_id":"precise_editor","surface":"editor","input_scope":"selection","scene_id":"scn_0123456789abcdef0123","scene_revision":"sha256:...","selection":{"start_byte":120,"end_byte":640,"text":"Selected prose..."}}
```

Successful response is `201 Created`:

```json
{"run_id":"run_0123456789abcdef0123","status":"pending","agent_id":"line_polish","style_id":"precise_editor","scene_id":"scn_0123456789abcdef0123","scene_revision":"sha256:...","selection":{"start_byte":120,"end_byte":640},"output_mode":"patch","patch":{"original":"Selected prose...","replacement":"Mock polished: Selected prose..."},"context_summary":{"packs_used":["selected_text","style_sheet"],"rag_mode":"none"}}
```

```http
POST /api/actions/{run_id}/accept
```

```json
{"expected_revision":"sha256:..."}
```

Success is `200 OK`:

```json
{"run_id":"run_0123456789abcdef0123","status":"accepted","scene":{"id":"scn_0123456789abcdef0123","chapter_id":"ch_0123456789abcdef0123","title":"The Duel","frontmatter":{"pov":"Luke","status":"draft","exclude_from_ai":false},"markdown":"Updated prose...","revision":"sha256:..."}}
```

```http
POST /api/actions/{run_id}/reject
```

Reject has no body. Success is `200 OK`:

```json
{"run_id":"run_0123456789abcdef0123","status":"rejected"}
```

Status mapping:

- `400 Bad Request`: malformed query, ID, JSON, revision, byte range, UTF-8
  boundary, selected-text mismatch, inapplicable agent, incompatible style,
  invalid registry authoring data detected during a run request, or no-op output.
- `404 Not Found`: well-formed scene, agent, style, or run ID is absent.
- `409 Conflict`: no active project, dirty worktree, stale scene revision, or a
  run is accepting/accepted/rejected when the requested decision requires pending.
- `503 Service Unavailable`: transient run capacity is exhausted or the mock
  provider is unavailable/canceled before producing output.
- `500 Internal Server Error`: malformed canonical registry/scene state,
  filesystem/index/Git failure, or rollback failure.
- `405 Method Not Allowed`: known route with unsupported method and `Allow` set.

Malformed canonical registry files map to 500 when discovered by list or
availability. A syntactically valid but non-executable registry definition maps
to 400 only when explicitly selected for a run.

## UI contract

Extend the existing scene editor without replacing CodeMirror or breaking Vim
mode. Minimum UI:

- action button disabled while loading, saving, dirty, empty-selection, or run,
- applicable agent list and compatible style selector,
- clear reason when the current selection has no applicable action,
- running, preview, accepting, rejecting, accepted, conflict, and error states,
- inline or side-by-side original/replacement preview with whitespace preserved,
- visible context packs and RAG mode,
- Copy replacement, Accept replacement, and Reject replacement controls,
- semantic dialog/region naming, focus placement, and keyboard-operable controls,
- confirmation before navigation discards an open preview only if user work
  would otherwise be lost; rejecting a mock preview itself needs no confirmation.

Acceptance updates the editor from the returned scene and must not issue a
second `PUT /api/scenes/{id}`. Rejection does not alter the editor draft.

## Required test architecture and acceptance

Tests must be written before production code for each behavior. Use one focused
scenario per test file where practical. Every Milestone 4 test file must state
its BDD scenario, requirement IDs, and plain-English purpose at the top; every
test name must describe observable behavior. Record actual evidence, not planned
filenames, in `.plans/milestone_4_test_evidence.md` during implementation.

Required layers:

- pure domain tests for registry validation, applicability reasons, word count,
  UTF-8 ranges, context allow/forbid rules, and run state transitions,
- strict YAML adapter tests including unknown fields, duplicate IDs, symlinks,
  multiple documents, ordering, and old unversioned templates,
- provider-boundary tests proving cancellation and provider-neutral mapping,
- action-service tests for no mutation on run/reject and capacity behavior,
- story-service tests for acceptance locking, revision checks, exact replacement,
  terminal-state races, checkpoints, and rollback at every failure boundary,
- API tests for exact JSON, query parsing, body limits, methods, and every status,
- frontend API-client and component flows using `fetch` interception or a fake
  server boundary, never `vi.mock('../api')` for Milestone 4 behavior,
- real-adapter acceptance with temporary filesystem, Git, and SQLite.

The real-adapter acceptance test must:

1. create/open a project and a scene containing ASCII and multibyte UTF-8 text,
2. load strict starter registries and prove deterministic listing,
3. request applicable actions for a valid selection,
4. run Line Polish and prove exact context and zero file/index/Git mutation,
5. reject one run and prove zero mutation,
6. run again and accept it,
7. prove only selected Markdown bytes changed and metadata bytes are canonical,
8. prove exactly one new commit, a clean worktree, rebuilt index, and new revision,
9. prove duplicate decision, stale revision, and dirty worktree do not mutate,
10. reload the scene through a fresh service instance and verify accepted text.

## Documentation and status completion

Documentation is part of acceptance. Before marking Milestone 4 complete:

1. update `docs/06_api_contract.md` from planned summaries to the implemented
   exact routes and link back to this contract,
2. update `docs/04_agent_style_system.md` with the final supported schemas and
   explicitly distinguish Milestone 4 mock behavior from Milestone 5 providers,
3. update `docs/07_frontend_editor.md` with the implemented patch-review flow,
4. update `docs/08_testing_acceptance.md` with named automated acceptance evidence,
5. update `DOCUMENTATION.md` to Version Milestone 4 and list implemented routes,
6. update `README.md` and `docs/05_milestones.md` to mark Milestone 4 complete
   and Milestone 5 as the next incomplete phase,
7. finish `.plans/milestone_4_test_evidence.md` and
   `.plans/milestone_4_status.md` with actual results and remaining risks,
8. ensure comments and docs describe implemented behavior, not planned behavior.

## Acceptance commands

Use the preferred toolchain and a writable cache when required by the execution
environment:

```bash
PATH=/home/linuxbrew/.linuxbrew/bin:$PATH GOCACHE=/tmp/storywriter-go-cache make check
PATH=/home/linuxbrew/.linuxbrew/bin:$PATH GOCACHE=/tmp/storywriter-go-cache go test -race ./...
git diff --check
git status --short
```

The build may retain the existing dependency-driven large-chunk warning, but do
not increase Vite's warning threshold. Generated `web/dist`, databases, test
projects, caches, and `node_modules` must remain untracked.

## Definition of done

Milestone 4 is complete only when:

- every M4 requirement maps to implemented BDD scenarios and named passing tests,
- malformed registries fail strictly and valid registries list deterministically,
- applicability and minimal context are proven in pure tests,
- the model boundary is provider-neutral and the production adapter is offline,
- run and reject have zero canonical/index/Git side effects,
- accept is explicit, revision-safe, race-safe, rollback-safe, and exactly one commit,
- UI acceptance uses the HTTP boundary and preserves dirty/canonical distinctions,
- full Milestone 0-3 regression and race suites pass,
- all documentation and status steps above are complete,
- no Milestone 5 behavior or generated artifact is committed.
