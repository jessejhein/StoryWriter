# 04 — Agent and Style System

## Core distinction

### Agent

An **Agent** defines the task/workflow.

Examples:

- Line Polish
- Local Voice Texture
- Chapter Refiner
- Outline Architect
- Codex Extractor
Agents define:

- where they apply,
- required input scope,
- required/optional/forbidden context packs,
- model capability requirements,
- output shape,
- human-control mode.

### Style

A **Style** defines voice/prompt/model preferences.

Examples:

- Dry modern fantasy
- Pulp adventure
- Brainstorming high-temperature
- Low-context pretty prose model
- Precise editor

Styles define:

- provider profile,
- model,
- system prompt or voice card,
- temperature and parameters,
- optional fallback style/model.

An agent can be run with a style if they are compatible.

## Milestone 4 implementation boundary

Milestone 4 implements a strict offline subset of this system:

- registries are read-only project-local `agents/*.yaml` and `styles/*.yaml`,
- only direct regular `*.yaml` children are loaded,
- YAML decoding is strict, single-document, and rejects unknown fields,
- built-in executable flow is `line_polish` plus the mock `precise_editor` style,
- `chapter_refiner` is loaded and listed for applicability only,
- provider profiles, credentials, capability discovery, and real model calls wait for Milestone 5.

Opening an older project with the pre-version Milestone 3 starter files still
works, but registry and availability routes now fail with an unsupported-schema
error until the author updates the registry files.

## Context packs

Agents should request context packs, not raw global dumps.

Possible packs:

```text
selected_text
surrounding_paragraphs
current_scene
current_chapter
chapter_summary
arc_summary
outline_neighborhood
active_codex_at_position
global_codex_rag
series_codex_rag
raw_import_notes
prior_chat
voice_sheet
style_sheet
continuity_events
```

## RAG modes

```yaml
rag_policy:
  mode: none
```

```yaml
rag_policy:
  mode: local
```

```yaml
rag_policy:
  mode: timeline_aware
```

```yaml
rag_policy:
  mode: global_timeline_aware
```

```yaml
rag_policy:
  mode: source_notes
```

Meaning:

- `none`: direct input only.
- `local`: current scene/chapter and neighbors.
- `timeline_aware`: active Codex state at current story position.
- `global_timeline_aware`: search whole project, resolve entries to current timeline state.
- `source_notes`: imported notes/chats, mostly during extraction.

## Human-control output modes

```text
suggestion
patch
proposal
direct_draft
```

Rules:

- Suggestions inform and do not modify files.
- Patches require diff preview and accept/reject.
- Proposals go to a review queue.
- Direct drafts are created as draft/branch content, not canon, until accepted.

## Agent applicability

The app should only offer agents that apply to the current state.

Current state includes:

```text
surface: editor | outline | codex | import_review | branch_compare
selection_words
selection_type
current_scene_id
current_chapter_id
current_arc_id
project_has_codex
project_has_outline
provider_capabilities
model_context_limit
```

Future example: if the user highlights 200 words in the editor, a later
milestone could offer Line Polish and Local Voice Texture. In Milestone 4, the
strict built-in registries expose Line Polish for editor selections and Chapter
Refiner for chapter scope only.

## Milestone 4 agent schema

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

Milestone 4 style schema:

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

The production mock adapter returns `Mock polished: ` followed by the trimmed
selection. The app rejects a byte-identical no-op before any run is stored or
accepted.

Only the exact version-1 schemas shown above and the exact loadable
`chapter_refiner` example below are accepted by the Milestone 4 registry
loader. Older unversioned examples, top-level style temperature fields, and
future-only keys are intentionally rejected by registry routes even though
project opening itself still works.

## Future example: local voice texture agent

This is a conceptual Milestone 5+ example. It is not loadable in Milestone 4
because the current strict registry loader rejects `model_requirements`,
unsupported context-pack names, and any behavior beyond the Milestone 4 subset.

```yaml
id: local_voice_texture
name: Local Voice Texture Pass
description: Use a small local model for paragraph/page-level wording and cadence.

applies_when:
  surfaces:
    - editor
  input_scopes:
    - selection
  min_words: 50
  max_words: 2500

model_requirements:
  min_context_tokens: 8000
  supports_streaming: false
  supports_structured_output: false

context_policy:
  required:
    - selected_text
    - style_sheet
  optional:
    - surrounding_paragraphs
    - short_voice_card
  forbidden:
    - global_codex_rag
    - raw_import_notes
    - full_chapter
    - full_outline

rag_policy:
  mode: none
  reason: This agent is for wording texture, not continuity reasoning.

control:
  output_mode: patch
  requires_acceptance: true
  can_modify_canon: false

output:
  type: replacement_text
  requires_diff_preview: true
```

## Milestone 4 loadable chapter refiner agent

```yaml
version: 1
id: chapter_refiner
name: Chapter Refiner
description: Refine a full chapter for flow, clarity, continuity, and scene-level cohesion.

applies_when:
  surfaces:
    - chapter_view
  input_scopes:
    - chapter
  min_words: 1000
  max_words: 12000

context_policy:
  required:
    - current_chapter
    - chapter_summary
    - style_sheet
  optional:
    - arc_summary
  forbidden:
    - raw_import_notes

rag_policy:
  mode: none

control:
  output_mode: patch
  requires_acceptance: true
  can_modify_canon: false

output:
  type: revised_text
  requires_diff_preview: true
```

## Future example: style

This is a conceptual Milestone 5+ style. It keeps the current
`parameters.temperature` nesting to avoid implying the old top-level shape is
still valid, but Milestone 4 does not load configurable models or extra
sampling parameters.

```yaml
version: 1
id: brainstorming_hot
name: Brainstorming Hot
provider_profile_id: local_or_api_default
model: configurable
parameters:
  temperature: 1.1
  top_p: 0.95
system_prompt: >
  You are a brainstorming partner. Offer several options, call out consequences,
  and do not assume any suggestion is canon until the author accepts it.
```

## Future provider capability matrix

This is Milestone 5+ conceptual configuration. Milestone 4 has no provider
profile loading or capability discovery and would reject these documents.

Each provider/model profile must declare capabilities:

```yaml
id: local_13b_8k
name: Local 13B 8K
provider: ollama
model: some-13b-model
capabilities:
  chat: true
  streaming: true
  structured_output: false
  tool_calling: false
  embeddings: false
  max_context_tokens: 8192
```

Do not offer incompatible agents for a selected provider/model.

## Milestone 5 provider-backed styles

Milestone 5 retains version-1 mock styles unchanged and adds version-2 agents
with explicit `model_requirements` plus version-2 styles that name an
application provider profile and model. Registry loading validates project
syntax only; a missing global profile makes a style unavailable rather than
making the project invalid.

Compatibility is a pure ordered decision: mock, profile not found, missing
credential, chat unsupported, context limit too small, streaming unsupported,
structured-output unsupported, or compatible. Availability exposes only ready,
compatible style IDs and omits agents with no usable styles. The dispatcher
repeats the decision immediately before generation so profile and environment
changes fail closed without creating a run.

Real-provider prompts contain only the agent task, style system prompt, and
exact selected text. OpenAI-compatible and Ollama results enter the same
transient patch workflow as mock output. Provider identity is retained for the
preview, but canon changes only through explicit acceptance.

## Milestone 7 version-3 agents

Milestone 7 adds strict `version: 3` agents with explicit scopes, context
policy, conservative budgets, timeline-aware RAG mode, follow-up transitions,
and output contracts. Versions 1 and 2 remain unchanged.

Starter agents:

- `line_polish` — selection scope; `rag_mode: none`; minimal
  `selected_text` + `style_sheet` packet.
- `scene_rewrite` — scene scope; `rag_mode: timeline_aware`; may include
  current scene, style, active Codex at target position, and bounded outline
  neighbors.
- `chapter_review` — `chapter_review` scope; returns strict JSON findings, not
  prose patches; `output_mode: suggestion`.

Follow-up invitations are declared in agent YAML (`follow_ups.on_accept` and
completed suggestion paths) but never auto-run. Maximum chain depth is 3.
Process-local invitations expire after 30 minutes, are one-time claims, and are
published atomically when one finding offers actions for multiple scenes.
Context budgets use a conservative byte estimator; UI labels estimates as
estimates, not billed tokens.

## Milestone 8 branch analysis versus editor actions

Ramification analysis is **not** a new editor action scope. Milestone 7 actions
remain limited to selection, scene rewrite, and chapter review. Branch
consequences are a separate branch-owned use case:

- the Branches workspace requests analysis only through
  `POST /api/branches/{experiment_id}/ramifications`,
- creating, switching, listing, or comparing experiments never calls a provider,
- the branch analyzer depends on `internal/modelchat`, not `internal/agent`
  registry or action-run orchestration,
- provider output is strict structured findings with category, severity, affected
  paths, and advisory recommended actions only; findings cannot be accepted as
  file patches,
- comparison packets include only the author goal, sorted changed paths, and
  bounded unified diff text for allowed project files.

Shared provider chat transport now lives in `internal/modelchat`. Agent actions
and import extraction consume the same neutral completer contract through thin
adapters; provider wire shapes remain adapter details and do not enter action
runs, extraction candidates, or branch comparison responses.
