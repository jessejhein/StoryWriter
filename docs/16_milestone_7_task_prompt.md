# Milestone 7 Task Prompt - Timeline-Aware Context and Conditional Actions

Implement only Milestone 7. Milestones 0 through 6 are complete and are
regression constraints. Follow the ordered red/green/refactor sequence in
`.plans/milestone_7_implementation.md` and update
`.plans/milestone_7_status.md` after every slice.

This document is the durable implementation contract. When an older general
document conflicts with this contract, this contract controls Milestone 7.
Do not infer multi-scene rewriting, automatic follow-up generation, embeddings,
or silent canon mutation from the older phrase "chapter refinement."

## Outcome

An author can run an AI action against one explicit target and inspect exactly
which context was sent. A paragraph action remains paragraph-sized. A scene
action may use the active Codex state at that scene and a bounded outline
neighborhood. Accepting a patch may expose a conditional follow-up action, but
the follow-up performs no provider call until the author explicitly runs it.

Accepted related actions retain causal history in Git commit trailers. A later
undo feature can therefore identify dependent accepted operations without
making Git notes or SQLite the only copy of that relationship.

Milestone 7 proves timeline behavior by resolving the same Codex entry at two
scenes on opposite sides of a progression and showing different context packet
content. It does not implement undo.

## Product interpretation

Use these terms consistently:

- **Agent**: a project-local task recipe such as Line Polish, Scene Rewrite, or
  Chapter Review.
- **Available action**: an agent/style/scope combination the application may
  offer for the current author state.
- **Action run**: one explicitly authorized provider execution against one
  immutable target snapshot.
- **Patch**: generated replacement text that can change canon only after
  explicit acceptance.
- **Suggestion**: generated editorial advice that cannot mutate canon.
- **Follow-up invitation**: a deterministic, zero-provider-call offer to run a
  new action because an earlier action reached a specified state.
- **Operation dependency**: durable commit metadata saying one accepted
  operation logically relies on another accepted operation.

An action never silently runs another action. "Generate an action
conditionally" means generate a follow-up invitation from pure policy. The
author must choose Run before the provider receives any additional context or
incurs additional work.

## Scope boundaries

In scope:

1. Preserve the existing selection-scoped Line Polish behavior and its minimal
   `selected_text` plus `style_sheet` packet.
2. Add a scene-scoped Scene Rewrite patch action.
3. Add a chapter-scoped Chapter Review suggestion action that produces
   editorial findings, not a multi-scene replacement.
4. Build typed, inspectable context packets from explicit agent policy.
5. Resolve relevant Codex entries to active state at the target scene.
6. Add a bounded outline neighborhood around the target scene/chapter.
7. Apply deterministic context budgeting and report included/omitted packs.
8. Expose a context preview before a run so the author can inspect scope and
   estimated size without calling a provider.
9. Produce deterministic follow-up invitations after accepted patches or
   completed suggestions.
10. Require a second explicit request before executing any invited action.
11. Record accepted action causality and dependency using Git commit trailers.
12. Display provider-call scope, context summary, omissions, and follow-up
    provenance in the UI.

Out of scope:

- automatic provider calls after run, acceptance, save, or navigation,
- whole-chapter prose replacement or atomic multi-scene patches,
- partial acceptance of a chapter-wide result,
- semantic embeddings, vector search, embedding providers, or model-specific
  tokenizers,
- imported-note retrieval, series-level retrieval, chat history, or web search,
- persistent prompts, generated text, selection text, or suggestion bodies,
- automatic effect tracing after acceptance,
- undo, revert, cascade revert, commit rewriting, or Git notes,
- background indexing or model execution,
- changing canon from a suggestion or follow-up invitation,
- automatic agent planning or unbounded recursive action chains.

## Requirements

| ID | Requirement |
| --- | --- |
| M7-R01 | Every provider run has one validated explicit scope: selection, scene, or chapter review. |
| M7-R02 | Selection Line Polish sends no scene, chapter, outline, Codex, progression, import, or global retrieval content. |
| M7-R03 | Scene Rewrite loads one canonical scene revision and may use timeline-active Codex plus a bounded outline neighborhood. |
| M7-R04 | Chapter Review returns non-mutating editorial suggestions and may use chapter scenes, timeline-active Codex, and outline neighbors. |
| M7-R05 | Active Codex context uses current outline chronology and the existing progression algorithm. |
| M7-R06 | Codex relevance selection is deterministic, lexical, inspectable, and independent of providers and SQLite. |
| M7-R07 | Context assembly obeys required, optional, and forbidden packs and fails closed when required material cannot fit. |
| M7-R08 | Context budgets use a conservative deterministic estimator and expose estimates as estimates, not exact provider token counts. |
| M7-R09 | Context preview performs no provider call, creates no run, and mutates no project or application state. |
| M7-R10 | Runs retain a redacted context manifest sufficient to inspect included and omitted packs without retaining prompt text. |
| M7-R11 | Follow-up invitations are produced by pure policy and never execute a provider automatically. |
| M7-R12 | Running a follow-up requires explicit author authorization and revalidates target revisions, applicability, provider readiness, and context. |
| M7-R13 | Accepted dependent actions write portable Git commit trailers containing stable operation IDs and causal/dependency IDs. |
| M7-R14 | Trigger causality is distinct from semantic dependency; only dependency edges affect a future cascade-undo decision. |
| M7-R15 | Existing explicit patch acceptance remains revision-safe, rollback-safe, race-safe, and exactly one checkpoint. |
| M7-R16 | Suggestions and rejected runs create no canonical file, index, staging, or Git changes. |
| M7-R17 | HTTP and frontend behavior expose scope, context manifest, cost-relevant estimates, and follow-up provenance without exposing secrets or full prompts. |
| M7-R18 | Context loading observes one coherent story snapshot under the shared mutation coordinator. |
| M7-R19 | Preserve all Milestone 0-6 behavior and keep full check and race suites green. |
| M7-R20 | Maintain exact requirement/scenario/test evidence and status throughout implementation. |

## BDD stories

### Story 7.1 - Keep paragraph work paragraph-sized

As an author, I want a paragraph rewrite to use only paragraph-level context so
that repeated wording work does not silently spend tokens on the whole scene.

#### Scenario 7.1.1 - Preview minimal Line Polish context

Requirements: M7-R01, M7-R02, M7-R07, M7-R09, M7-R17.

```gherkin
Given a clean canonical scene with a selected paragraph
When I preview Line Polish context
Then the manifest contains selected_text and style_sheet only
And it contains no scene, chapter, outline, Codex, progression, import, or RAG content
And no provider is called
And no action run or project mutation is created
```

#### Scenario 7.1.2 - Run with the previewed scope

Requirements: M7-R01, M7-R02, M7-R10, M7-R15.

```gherkin
Given I previewed a valid paragraph action
When I explicitly run it against the unchanged scene revision and selection
Then the provider receives only the selected text and style instructions
And the patch preview identifies selection scope
And acceptance replaces only the selected UTF-8 byte range
```

The preview is advisory, not a reservation. Run repeats every validation and
rebuilds context from current canonical state.

### Story 7.2 - Rewrite one scene with timeline-aware context

As an author, I want to rewrite a scene using facts active at that point in the
story so that later developments do not leak backward into earlier prose.

#### Scenario 7.2.1 - Resolve different active facts by scene

Requirements: M7-R03, M7-R05, M7-R06, M7-R18.

```gherkin
Given a character has a progression after scene A
And scene B occurs after scene A
When I build Scene Rewrite context for a scene before the progression
Then the character context contains the earlier active state
When I build it for scene B
Then the character context contains the later active state
And both packets identify the progression IDs applied
```

#### Scenario 7.2.2 - Exclude irrelevant Codex entries

Requirements: M7-R06, M7-R07, M7-R08.

```gherkin
Given many Codex entries exist
And the scene mentions a character by canonical name or alias
When timeline-aware context is assembled
Then the mentioned entry ranks ahead of entries with no lexical evidence
And every included entry is resolved as of the target scene
And deterministic trimming reports omitted entry IDs and reasons
```

#### Scenario 7.2.3 - Review and accept one scene replacement

Requirements: M7-R03, M7-R10, M7-R15.

```gherkin
Given Scene Rewrite has a valid scene-scoped context packet
When I run it
Then the provider returns one replacement for that scene Markdown body
And the UI shows the original and replacement with the context manifest
When I accept the patch
Then only that scene file changes
And exactly one checkpoint is created
```

Scene Rewrite preserves scene identity and front matter. The output replaces
the complete Markdown body only. It does not rewrite title, POV, status,
chapter membership, or other scenes.

### Story 7.3 - Review a chapter without rewriting it wholesale

As an author, I want chapter-level editorial analysis so that I can decide
which smaller changes deserve additional model calls.

#### Scenario 7.3.1 - Build bounded chapter-review context

Requirements: M7-R04, M7-R05, M7-R07, M7-R08.

```gherkin
Given a chapter contains ordered scenes
When Chapter Review context is assembled
Then it contains the ordered scene text within the configured budget
And active Codex entries are resolved separately at each scene where needed
And the outline neighborhood identifies the preceding and following chapters
And trimming is deterministic and inspectable
```

#### Scenario 7.3.2 - Return suggestions without canon mutation

Requirements: M7-R04, M7-R10, M7-R16.

```gherkin
Given valid Chapter Review context
When I explicitly run Chapter Review
Then I receive structured editorial findings naming affected stable scene IDs
And no canonical file, index, staging state, or Git history changes
And the findings cannot be accepted as a chapter-wide prose replacement
```

Chapter Review findings are transient. Each finding contains a short title,
explanation, affected scene IDs, and zero or more allowed follow-up action
templates. It contains no generated replacement prose.

### Story 7.4 - Offer conditional follow-up actions safely

As an author, I want the application to suggest a logical next action without
running it so that I retain control of cost, scope, and canon.

#### Scenario 7.4.1 - Offer a follow-up without calling a provider

Requirements: M7-R11, M7-R16, M7-R17.

```gherkin
Given I accepted a paragraph patch
And the configured follow-up policy allows a scene-level review
When acceptance completes
Then the response may include an invitation to review or rewrite the scene
And the invitation names its scope and parent run
And no follow-up provider request occurs
```

#### Scenario 7.4.2 - Explicitly run an invited action

Requirements: M7-R12, M7-R18.

```gherkin
Given an unexpired follow-up invitation from an accepted run
When I choose Run
Then the app reloads current canonical state
And revalidates the target revision, agent, style, provider, and context budget
And creates a distinct child run
```

Invitations are process-local and bounded like runs. Restart may discard them.
The child run remains independently previewable and rejectable.

#### Scenario 7.4.3 - Reject forged, stale, or recursive invitations

Requirements: M7-R11, M7-R12.

```gherkin
Given an invitation is absent, consumed, expired, targets changed canon,
  names a disallowed agent transition, or exceeds the maximum chain depth
When a client attempts to run it
Then the request fails before provider execution
And no run or mutation is created
```

Milestone 7 maximum chain depth is 3, including the root run. One invitation
may create at most one child run. A child may not target a broader scope unless
the invitation names that scope and the author explicitly confirms it.

### Story 7.5 - Preserve accepted-action dependencies in Git

As an author, I want accepted touchups to point back to changes they rely on so
that a future undo workflow can identify dependent work.

#### Scenario 7.5.1 - Record causal and dependency trailers

Requirements: M7-R13, M7-R14, M7-R15.

```gherkin
Given accepted run B was explicitly launched from accepted run A
And B semantically depends on A
When B is accepted
Then B creates one checkpoint with its normal subject
And the commit contains stable Storywork operation, trigger, dependency, and scope trailers
And A's commit can be found by operation ID in current branch ancestry
```

#### Scenario 7.5.2 - Preserve trigger-only relationships

Requirements: M7-R13, M7-R14.

```gherkin
Given run B was suggested because of A but remains valid without A
When B is accepted
Then B records Storywork-Triggered-By for A
And does not record Storywork-Depends-On for A
```

#### Scenario 7.5.3 - Refuse invalid dependency metadata

Requirements: M7-R13, M7-R15.

```gherkin
Given a child run claims an unknown, non-accepted, self, or cyclic dependency
When acceptance is attempted
Then acceptance fails before canonical writes
And no commit is created
```

Milestone 7 validates dependencies against the bounded in-memory run lineage
and existing branch ancestry. Full historical graph traversal is reserved for
the future undo feature.

## Fixed action scopes and outputs

Add explicit scopes rather than overloading selection fields:

| Scope | Required target | Output | Canon effect |
| --- | --- | --- | --- |
| `selection` | scene ID, scene revision, UTF-8 byte range and exact text | replacement patch | selected bytes after acceptance |
| `scene` | scene ID and scene revision | replacement patch | scene Markdown body after acceptance |
| `chapter_review` | chapter ID plus current outline revision/fingerprint | structured suggestions | none |

Do not add a generic `map[string]any` target. Use a tagged request DTO at the
HTTP boundary and explicit Go structs internally. Exactly one target matching
the scope must be present.

The chapter fingerprint is a deterministic SHA-256 over `outline.yaml` bytes
and the ordered `(scene ID, scene revision)` pairs in the target chapter. It is
an optimistic-read token, not canonical state. Run rebuilds and compares it.

## Agent schema version 3

Continue loading version-1 and version-2 definitions unchanged. Add explicit
version 3 for context budgets and follow-up policy. Do not reinterpret older
documents. Existing version-2 Line Polish files remain minimal and produce no
follow-up invitations. Update the new-project Line Polish template to version 3
with the same required/forbidden context and an explicit `scene_rewrite`
post-accept invitation; this adds no context and makes no second provider call.

```yaml
version: 3
id: scene_rewrite
name: Scene Rewrite
description: Rewrite one scene while preserving established facts and intent.
applies_when:
  surfaces: [editor]
  input_scopes: [scene]
  min_words: 1
  max_words: 12000
model_requirements:
  min_context_tokens: 4096
  supports_streaming: false
  supports_structured_output: false
context_policy:
  required: [current_scene, style_sheet, active_codex_at_position]
  optional: [outline_neighborhood]
  forbidden: [global_codex_rag, raw_import_notes, prior_chat]
context_budget:
  max_input_estimated_tokens: 12000
  reserved_output_estimated_tokens: 4000
rag_policy:
  mode: timeline_aware
follow_ups:
  on_accept:
    - agent_id: chapter_review
      scope: chapter_review
      relationship: triggered
control:
  output_mode: patch
  requires_acceptance: true
  can_modify_canon: false
output:
  type: revised_text
  requires_diff_preview: true
```

Chapter Review uses `output_mode: suggestion`, `output.type:
editorial_findings`, `requires_acceptance: false`, and `can_modify_canon:
false`. Version-3 Line Polish retains only `selected_text` and `style_sheet`,
uses `rag_policy.mode: none`, and may declare Scene Rewrite after acceptance.
Version-3 validation must reject unsupported packs, modes, outputs,
relationships, transitions, cycles, duplicate follow-ups, and budgets
inconsistent with provider requirements.

Supported Milestone 7 RAG modes are `none` and `timeline_aware`. The name
`timeline_aware` means deterministic active-state resolution plus lexical
relevance; it does not imply embeddings.

## Context architecture

Add a cohesive `internal/contextpack` package. It owns pure context selection,
ranking, budgeting, typed packets, and redacted manifests. It must not import
HTTP, Git, SQLite, provider adapters, action run storage, or filesystem code.

The action service consumes two narrow interfaces:

```go
type ContextMaterialSource interface {
    LoadContextMaterial(context.Context, ContextTarget) (ContextMaterial, error)
}

type ContextBuilder interface {
    Build(BuildRequest) (Packet, Manifest, error)
}
```

`story.Service` implements the material source because it already owns strict
outline, scene, Codex, progression, and shared read-lock behavior. It must load
one coherent snapshot while holding `mutation.Coordinator.RLock`. It returns
typed material; it does not decide relevance, budgets, prompts, or follow-ups.

`contextpack.Builder` is pure. `action.Service` orchestrates registry lookup,
scope validation, material loading, provider compatibility, run lifecycle, and
follow-up lineage. Provider adapters serialize already-built packets into
provider-neutral messages. They do not query story state.

This division is required by SOLID:

- context selection has one reason to change,
- story persistence remains behind the story-owned boundary,
- action orchestration depends on small consumer-owned interfaces,
- providers remain substitutable and receive no project-specific store,
- new packs require explicit typed additions rather than permissive maps.

## Active Codex relevance

For each target scene:

1. Strictly load all base entries and progression documents in deterministic
   canonical list order.
2. Resolve every candidate entry with the existing pure active-state algorithm
   at the target scene.
3. Build lexical evidence from the target text using canonical name, aliases,
   and tags. Matching is Unicode case-folded and requires token/phrase
   boundaries; do not use substring matches such as `Ann` inside `annual`.
4. Rank entries by: canonical-name mention, alias mention, tag mention, number
   of occurrences descending, canonical type order, name, then ID.
5. Include explicitly mentioned entries first. If the required active-Codex
   pack has no lexical matches, include no entries and report the empty pack;
   do not dump the global Codex.
6. Resolve before budgeting so no later progression state can leak into an
   earlier target.

Chapter Review performs relevance per scene, then deduplicates equal resolved
entry states while retaining the scene IDs for which each state applies.

## Context budgeting and manifest

Use an injected `Estimator` interface in pure tests. Production initially uses
a conservative UTF-8 byte estimator: one UTF-8 byte equals one estimated token.
This intentionally overestimates typical text and must be labeled
`estimated_tokens`. Do not claim exact billing or tokenizer output.

Budget order:

1. Reserve the configured output estimate.
2. Include required style/task framing.
3. Include the exact target text; target text is indivisible.
4. Include required active-Codex entries in relevance order.
5. Include optional outline neighbors nearest-first.
6. Omit optional material that does not fit.

If the target plus required framing exceeds the action or provider input
budget, fail before provider execution. Do not silently truncate selected text,
scene prose, or required active-state fields. Individual optional Codex entries
are indivisible.

The public manifest contains:

```json
{
  "scope": "scene",
  "packs_used": ["current_scene", "style_sheet", "active_codex_at_position"],
  "packs_omitted": [{"pack":"outline_neighborhood","reason":"budget"}],
  "estimated_input_tokens": 4312,
  "max_input_estimated_tokens": 12000,
  "rag_mode": "timeline_aware",
  "active_codex": [
    {"entry_id":"char_...","applied_progression_ids":["prog_..."]}
  ],
  "outline_refs": ["scn_...", "ch_..."]
}
```

It must not contain prose, descriptions, metadata values, system prompts,
provider credentials, endpoint URLs, or generated output.

## Follow-up invitations and lineage

Invitation IDs use `invite_` plus 20 lowercase hexadecimal characters. Store
invitations in a mutex-protected, process-local bounded store. Each invitation
contains only:

- invitation ID,
- parent run ID and root run ID,
- chain depth,
- target agent ID and explicit scope,
- stable scene/chapter target IDs,
- relationship: `triggered` or `depends_on`,
- creation state and one-time status.

Do not retain prose in invitations. A follow-up request contains invitation ID,
style ID, and the current target revision/fingerprint. The service claims the
invitation, revalidates everything, then creates a child run. Provider or
validation failure releases it for retry; successful run creation consumes it.

Pure follow-up policy validates allowed transitions from version-3 agent
definitions. It never chooses a style, executes a provider, or broadens scope
without an explicit configured transition and author request.

## Git causal metadata

Use commit-message trailers, not Git notes. Keep existing subject lines. An
accepted root patch uses:

```text
Accept AI patch run_aaaaaaaaaaaaaaaaaaaa

Storywork-Operation-ID: run_aaaaaaaaaaaaaaaaaaaa
Storywork-Scope: selection:scn_0123456789abcdef0123
```

A dependent child uses:

```text
Accept AI patch run_bbbbbbbbbbbbbbbbbbbb

Storywork-Operation-ID: run_bbbbbbbbbbbbbbbbbbbb
Storywork-Triggered-By: run_aaaaaaaaaaaaaaaaaaaa
Storywork-Depends-On: run_aaaaaaaaaaaaaaaaaaaa
Storywork-Scope: scene:scn_0123456789abcdef0123
```

Rules:

- Trailer keys and order are exact as shown.
- Values are validated stable IDs; never interpolate author or model text.
- `Storywork-Triggered-By` records causality.
- `Storywork-Depends-On` is present only for semantic dependency.
- A dependent operation also records `Triggered-By`.
- Subjects remain compatible with current history and tests.
- SQLite may index trailers, but Git commit text is authoritative.
- Do not persist full prompts, context, or generated prose in trailers.

Extend the Git/checkpoint boundary with a typed commit message containing a
subject and validated trailers. Do not concatenate trailers throughout action
or story code. Existing callers may use subject-only commits. The story patch
acceptance request carries already-validated operation metadata; the story
service still owns the single atomic checkpoint.

## Provider output contracts

Selection and scene patch output remains replacement text only. Scene output
has the existing generated-text UTF-8, NUL, newline, empty, and size safety
rules and must fit the canonical 5 MiB scene limit.

Chapter Review requires one strict JSON object:

```json
{
  "findings": [
    {
      "title": "Transition loses urgency",
      "explanation": "The shift between the two scenes releases tension.",
      "scene_ids": ["scn_0123456789abcdef0123"],
      "follow_up_agent_ids": ["scene_rewrite"]
    }
  ]
}
```

Maximum 20 findings; title 200 runes; explanation 4000 UTF-8 bytes; 1 to 20
unique canonical scene IDs per finding; follow-up IDs must be allowed by the
Chapter Review definition. Reject unknown, missing, null, wrongly typed,
trailing, fenced, oversized, or partially invalid output as a whole. Zero
findings is valid and produces no invitations.

Run states remain output-specific:

- patch runs use `pending`, `accepting`, `accepted`, and `rejected` exactly as
  before;
- suggestion runs enter `completed` only after strict output validation and
  have no accept operation;
- closing or dismissing a suggestion in the UI does not create a server-side
  mutation or Git checkpoint.

The production mock adapter remains deterministic. Selection output remains
`Mock polished: ` plus the trimmed selection. Scene output uses the prefix
`Mock rewritten: ` followed by the trimmed current scene Markdown. Chapter
Review returns one fixed valid finding referencing the first target scene, or
an empty findings array for an empty chapter. Mock output proves plumbing only
and follows the same transient/acceptance rules as real-provider output.

Follow-ups from a completed suggestion retain process-local parent/root run
lineage, but they do not write a Git causal trailer because the suggestion has
no accepted commit to reference. Git `Triggered-By` and `Depends-On` trailers
are written only when the parent names an accepted operation present in branch
history.

## HTTP API

Preserve all existing routes. Add:

```http
POST /api/actions/context-preview
POST /api/action-invitations/{invitation_id}/run
```

Extend `POST /api/actions/run` with a strict tagged target. During migration,
continue accepting the exact Milestone 4-6 selection body and normalize it to
the selection target internally. New clients use:

```json
{
  "agent_id":"scene_rewrite",
  "style_id":"precise_editor",
  "scope":"scene",
  "target":{"scene_id":"scn_...","scene_revision":"sha256:..."}
}
```

Context preview uses the same body and returns:

```json
{"manifest":{/* redacted manifest */},"target_revision":"sha256:..."}
```

It never returns packet content. Run responses add `scope`, `parent_run_id`
(nullable), `root_run_id`, `chain_depth`, and the manifest. Patch responses
retain the existing patch fields. Suggestion responses contain `findings` and
no patch.

Patch acceptance responses add `follow_up_invitations`, always an array. A
suggestion run returns its allowed invitations with the findings because there
is no canon acceptance step. Merely receiving an invitation performs no model
call.

Invitation run request:

```json
{
  "style_id":"precise_editor",
  "expected_target_revision":"sha256:..."
}
```

Chapter invitations use the documented chapter fingerprint in that field.

Status mapping additions:

- `400 Bad Request`: malformed scope/target, unsupported transition, budget
  failure caused by request/configuration, invalid invitation ID, or invalid
  generated suggestion structure.
- `404 Not Found`: valid absent target, run, agent, style, or invitation.
- `409 Conflict`: stale target, consumed/claimed invitation, non-accepted parent
  for an acceptance-triggered invitation, dirty worktree on patch acceptance,
  or lineage conflict.
- `413 Request Entity Too Large`: body exceeds the existing applicable limit.
- `502 Bad Gateway`: provider rejects or returns invalid output.
- `503 Service Unavailable`: unavailable provider, timeout/cancellation, or
  bounded live run/invitation capacity.
- `500 Internal Server Error`: malformed canonical state, invalid committed
  operation metadata, storage/index/Git/rollback failure.

Keep recursive strict JSON, method/Allow behavior, safe errors, and existing
body limits. No response exposes full prompts or credentials.

## Frontend behavior

Keep selection actions in the existing scene editor. Add explicit controls:

- `Preview context` before Run,
- scope label and estimated input size,
- included/omitted pack list,
- active Codex entry IDs and applied progression IDs,
- `Rewrite scene` as a separate scene-scoped action,
- follow-up invitation cards that state the broader scope before Run,
- a confirmation when moving from selection to scene or chapter scope,
- Chapter Review findings grouped by stable scene ID,
- `Run suggested action` buttons, never automatic execution.

The UI must not fetch or assemble canonical context itself. It sends stable
target IDs/revisions and renders the server manifest. Dirty drafts disable
preview and run because server context must correspond to canonical bytes.
Navigation and stale-response protections from prior milestones remain.

Never label estimated tokens as price or exact billed tokens. Never suggest
that displaying an invitation means a provider has already run.

## SOLID review and required touchups

The current implementation is sound for one selection action but clashes with
Milestone 7 in specific places:

1. `agent.ContextPacket` contains only selected text and style. Replace it with
   explicit typed packet variants owned by `internal/contextpack`; do not add a
   growing list of nullable fields to the current struct.
2. `agent.BuildContext` both validates agent definitions and constructs the
   only packet shape. Retain registry validation in `internal/agent`; move
   context selection/budgeting to `internal/contextpack`.
3. `agent.ValidateAgent` accepts only RAG mode `none`. Add version-specific
   validation for version 3 rather than loosening old versions.
4. Provider prompt code assumes "rewrite selected text." Introduce
   output/scope-specific message builders behind a small interface; keep HTTP
   transport provider-neutral.
5. `action.Service.Run` owns too many sequential decisions for multiple scopes.
   Extract pure target validation/context/follow-up decisions, but keep one
   orchestration service. Do not create a framework or event bus.
6. The API `StoryStore` and application `compositeStore` are broad. Split API
   constructor dependencies into cohesive project, story, action, provider,
   and import-review interfaces while touching the action routes. Preserve one
   production composition root.
7. Shared chat transport currently lives under `internal/agent` and is also
   imported by extraction. Moving it during Milestone 7 is optional only if
   tests show no harmful cycle or action-specific leakage. If moved, create a
   narrow `internal/modelchat` adapter package and perform the refactor before
   context behavior. Do not mix it into `contextpack`.
8. Commit subjects are currently unstructured strings. Add one typed commit
   metadata value at the Git consumer boundary and preserve subject-only calls.

These are targeted refactors that directly enable Milestone 7. Do not rewrite
working story, import, provider-profile, or mutation transaction code merely to
make package diagrams symmetric.

## Required test architecture

Write tests before production code for each slice. Every Milestone 7 test file
must begin with:

```text
BDD Scenario: 7.x.y - scenario title
Requirements: M7-Rxx, M7-Ryy
Test purpose: Plain-English observable behavior proved by this file.
```

Every test case needs adjacent `Test:` and `Requirements:` comments. One file
should cover one scenario where practical. Required layers:

- pure tests for lexical matching, timeline resolution inputs, ranking,
  budgeting, manifests, follow-up transitions, depth, and commit trailers,
- story material-source tests proving coherent locked snapshots,
- registry tests for version-3 strict YAML and old-version compatibility,
- action service tests for preview zero-side-effects, all scopes, revalidation,
  invitation claims, lineage, and run capacity,
- provider message/output tests for selection, scene, and strict findings,
- story/Git tests for subject/trailer bytes, acceptance, rollback, and dependency
  validation before writes,
- API tests for tagged targets, old selection compatibility, exact manifests,
  strict bodies, statuses, safe errors, and methods,
- frontend transport and component tests through intercepted `fetch`,
- one real-adapter acceptance test with temporary project, Git, SQLite, and
  local `httptest.Server` provider.

The real-adapter test must prove:

1. existing Line Polish still sends minimal context;
2. context preview calls no provider and creates no run;
3. one character resolves differently before/after a progression;
4. Scene Rewrite sends only its documented packet and changes one scene only
   after acceptance;
5. accepted root commit contains operation/scope trailers;
6. acceptance creates an invitation without a provider request;
7. explicit invitation execution creates a distinct child run;
8. accepted dependent child commit contains trigger and dependency trailers;
9. Chapter Review creates strict findings and zero canon/Git mutation;
10. stale scope, budget overflow, invalid output, and forged invitation produce
    no partial state;
11. all successful patch accepts create one commit and leave a clean worktree;
12. a fresh index rebuild can recover operation metadata from Git trailers if
    indexing is implemented in this milestone.

## Documentation and status updates

Before marking Milestone 7 complete:

1. Update `docs/02_architecture.md` with `internal/contextpack`, coherent story
   snapshot loading, action lineage, and any actual `modelchat` extraction.
2. Update `docs/03_storage_model.md` with authoritative commit trailers and any
   rebuildable operation-dependency index.
3. Update `docs/04_agent_style_system.md` with version-3 agents, scopes,
   timeline-aware semantics, budgets, suggestions, and follow-ups.
4. Mark Milestone 7 complete in `docs/05_milestones.md` only after acceptance
   passes, then identify Milestone 8 as next.
5. Update `docs/06_api_contract.md` with exact implemented preview, run,
   invitation, manifest, and response shapes.
6. Update `docs/07_frontend_editor.md` with context inspection, scene rewrite,
   Chapter Review, and explicit follow-up UI behavior.
7. Add exact named evidence to `docs/08_testing_acceptance.md`.
8. Update `DOCUMENTATION.md` version, API inventory, and package/comment rules
   only for behavior actually implemented.
9. Update `README.md` status and package map only after implementation passes.
10. Maintain `.plans/milestone_7_test_evidence.md` with exact test names and
    observable assertions; planned tests are not evidence.
11. Update `.plans/milestone_7_status.md` after every slice with phase,
    completed work, commands/results, design decisions, risks, and next command.
12. If review finds defects, create a scoped remediation plan and keep status
    `in progress` until fixes and rerun evidence pass.
13. Re-run all checks after the final documentation edit. Future-tense planning
    text must not be rewritten as a completion claim before behavior exists.

## Verification

Use `/home/linuxbrew/.linuxbrew/bin/go` first when available:

```bash
go fmt ./...
go vet ./...
go test ./... -count=1
go test -race ./...
cd web && npm run lint
cd web && npm run typecheck
cd web && npm test -- --run
make check
git diff --check
git status --short
```

Inspect for generated databases, context/prompt dumps, provider output, note
contents, credentials, build output, caches, live servers, and test projects.
Do not read or print developer environment values during leak checks.

## Definition of done

Milestone 7 is complete only when:

- paragraph actions are proven minimal rather than merely documented minimal,
- scene context differs correctly across timeline progression boundaries,
- context previews and manifests are inspectable and redacted,
- budget failures occur before provider execution,
- Chapter Review is suggestion-only and cannot rewrite multiple scenes,
- every follow-up requires a separate author-authorized provider request,
- accepted dependent operations contain valid portable Git trailers,
- patch acceptance retains all prior atomicity and author-control guarantees,
- every requirement and BDD scenario has exact named automated evidence,
- full regression, race, frontend, build, diff, and artifact checks pass,
- documentation and status records describe actual behavior,
- no Milestone 8 branch workflow or future undo behavior is implemented.
