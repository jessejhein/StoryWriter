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
- Ramification Analyzer
- Branch Compare

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

Example: if the user highlights 200 words in the editor, offer Line Polish and Local Voice Texture. Do not offer Outline Architect.

## Example: local voice texture agent

```yaml
id: local_voice_texture
name: Local Voice Texture Pass
description: Use a small local model for paragraph/page-level wording and cadence.

applies_when:
  surfaces:
    - editor
  input_scope:
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

## Example: chapter refiner agent

```yaml
id: chapter_refiner
name: Chapter Refiner
description: Refine a full chapter for flow, clarity, continuity, and scene-level cohesion.

applies_when:
  surfaces:
    - editor
    - chapter_view
  input_scope:
    - chapter
  min_words: 1000
  max_words: 12000

model_requirements:
  min_context_tokens: 32000
  supports_streaming: true

context_policy:
  required:
    - current_chapter
    - chapter_summary
    - active_codex_at_position
    - outline_neighborhood
    - voice_sheet
  optional:
    - global_codex_rag
    - arc_summary
    - continuity_events
  forbidden:
    - raw_import_notes

rag_policy:
  mode: timeline_aware
  scope: chapter_plus_neighbors
  max_entries: 20
  include_progressions: true

control:
  output_mode: patch
  requires_acceptance: true
  can_modify_canon: false

output:
  type: revised_text
  mode: whole_document_or_patch
  requires_diff_preview: true
```

## Example: style

```yaml
id: brainstorming_hot
name: Brainstorming Hot
provider_profile_id: local_or_api_default
model: configurable
temperature: 1.1
top_p: 0.95
system_prompt: >
  You are a brainstorming partner. Offer several options, call out consequences,
  and do not assume any suggestion is canon until the author accepts it.
```

## Provider capability matrix

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

