# 05 — MVP Milestones

## Epic: MVP

Build a local-first AI writing workshop that turns messy Markdown notes into a structured, timeline-aware, branchable novel project with specialized AI assistance.

Each milestone is a sprint. Each sprint must leave the app working.

## Current status

- Milestones 0 through 8: implemented; Milestone 8 verification was re-run on
  July 9, 2026 and passed.
- Milestone 9: next incomplete phase.

The durable Milestone 4 contract is `docs/13_milestone_4_task_prompt.md`.

---

## Milestone 0 — Foundation and local project skeleton

### Goal

Create a runnable local app skeleton and project-folder format.

### Stories

#### Story 0.1 — Create project folder

As an author, I want to create a new project folder so that my story has a portable home on disk.

Acceptance:

```gherkin
Given an empty directory path
When I create a project named "Test Novel"
Then the app writes project starter files
And initializes a Git repository
And creates .storywork/index.sqlite
And records a first Git commit
```

#### Story 0.2 — Open existing project

As an author, I want to open an existing project folder so that I can continue work.

Acceptance:

```gherkin
Given a valid project folder
When I open the project
Then the app loads project metadata
And verifies the Git repository
And verifies or rebuilds the SQLite index
```

#### Story 0.3 — Health check

As a developer, I want a health endpoint so that I can verify the local server is running.

Acceptance:

```gherkin
When I request /api/health
Then I receive status ok and version information
```

### TDD focus

- project creation service,
- Git init adapter with fake/temp repo tests,
- file writer interface,
- SQLite migration/index initialization,
- health endpoint.

### Done when

- `go test ./...` passes.
- `make check` exists and passes for implemented parts.
- Go server starts.
- Frontend starts and can call health endpoint.
- A project can be created through API.

---

## Milestone 1 — Story structure files and basic outline UI

### Goal

Create and reorder the story hierarchy using canonical text files.

### Stories

- As an author, I can create arcs, chapters, and scenes.
- As an author, I can view a tree of arcs -> chapters -> scenes.
- As an author, I can reorder scenes and chapters while stable IDs remain unchanged.
- As an author, every structural change creates a Git checkpoint.

The implementation contract, BDD cases, schemas, error behavior, and ordered TDD
work are defined in `docs/10_milestone_1_task_prompt.md`.

### TDD focus

- stable ID generation,
- outline ordering rules,
- file serialization/deserialization,
- Git checkpoint service,
- API handlers for structure changes.

### Done when

The user can create a simple outline in UI/API and see files written under `arcs/`, `chapters/`, `scenes/`, and `outline.yaml`.

Renaming, deleting, reparenting, and editing scene prose are not part of this
milestone.

---

## Milestone 2 — Vim-friendly scene editor

### Goal

Write and save scene prose.

### Stories

- As an author, I can open a scene in a CodeMirror editor.
- As an author, I can use Vim keybindings.
- As an author, edits save to the scene Markdown file.
- As an author, I can see unsaved/saved status.
- As an author, each explicit successful save creates exactly one lightweight Git checkpoint.

### TDD focus

- scene parse/write logic,
- save endpoint,
- frontend editor state,
- automated Vim extension regression coverage.

### Done when

The user can create a scene, edit prose, save it, reload the page, and see the saved text.

---

## Milestone 3 — Codex and timeline progressions

### Goal

Create Codex entries and resolve active story state by timeline position.

### Stories

- As an author, I can create/edit Codex entries.
- As an author, I can add aliases and tags.
- As an author, I can add a progression anchored to a stable scene/chapter/event ID.
- As an author, when I reorder chapters, progressions still point to the same story event.
- As the app, I can compute the active Codex state for a given scene.

### TDD focus

- progression activation logic,
- active Codex state merge rules,
- anchor validation,
- ID vs display-label behavior.

### Done when

The user can view "Codex as of this scene" and see different active states before/after a progression anchor.

---

## Milestone 4 — Agent/style registry and mock AI actions

### Goal

Add configurable agents/styles and prove context-minimized selection actions without relying on real providers.

### Stories

- As an author, I can view built-in agents and styles.
- As an author, when I select a paragraph, I only see agents that apply to paragraph selection.
- As an author, when I select a full chapter, I see different applicable agents.
- As a developer, model/provider calls go through interfaces.
- As an author, I can run a mock Line Polish action and review a patch before accepting.

### TDD focus

- agent applicability decisions,
- context policy assembly,
- provider interface,
- patch accept/reject flow.

### Done when

Selection -> applicable agents -> mock patch -> diff preview -> accept/reject works.

---

## Milestone 5 — Real provider adapters and credential broker v1

Status: complete June 30, 2026.

### Goal

Connect real model providers without leaking credentials into projects.

The durable implementation contract, exact provider/profile schemas, API
behavior, security constraints, BDD cases, and ordered TDD work are defined in
`docs/14_milestone_5_task_prompt.md`.

### Stories

- As an author, I can configure a local OpenAI-compatible endpoint or Ollama-style endpoint.
- As an author, I can configure API-key provider profiles for dev use.
- As the app, credentials are stored outside project folders.
- As a developer, provider capability flags control which agents are available.

### TDD focus

- provider request mapping,
- capability filtering,
- credential lookup interface,
- fake provider tests.

### Done when

At least one local endpoint and one API-key provider path can run a simple text action through the same model interface.

---

## Milestone 6 — Markdown import and extraction review queue

Status: complete July 1, 2026. The durable implementation contract is
`docs/15_milestone_6_task_prompt.md`; the implementation sequence and evidence
live in `.plans/milestone_6_*`. Milestone 7 is the next incomplete phase.

### Goal

Import messy notes and create reviewable candidates.

### Stories

- As an author, I can import a folder of Markdown files.
- As the app, imported files are copied into tracked project-local snapshots.
- As the app, notes are deterministically chunked with source-line provenance.
- As an author, I can run provider-neutral structure extraction.
- As an author, I can review, edit, merge, discard, or explicitly accept candidates.

### TDD focus

- Markdown importer,
- chunker,
- candidate schema,
- review queue state transitions,
- no direct canon mutation from extraction.

### Done when

Drop Markdown notes -> inspect chunks -> run extraction -> review candidates
appear -> author accepts one into Codex/outline through the existing mutation
boundaries with exactly one checkpoint.

---

## Milestone 7 — Timeline-aware RAG for AI actions

Status: complete July 2, 2026. Durable contract:
`docs/16_milestone_7_task_prompt.md`. Evidence:
`.plans/milestone_7_test_evidence.md`.

### Goal

Use active Codex and story position to build context for agents.

For this milestone, `timeline_aware` means deterministic active-progression
resolution plus lexical relevance, not embeddings. Each model run has one
explicit scope. Line Polish remains paragraph-sized; Scene Rewrite may use
timeline-aware context; Chapter Review returns suggestions rather than a
multi-scene rewrite. Conditional follow-ups require a separate author-approved
run and accepted dependencies are recorded in portable Git commit trailers.

### Stories

- As an author, AI actions in a scene use the active Codex state for that scene.
- As an author, paragraph polish avoids global RAG unless the agent requires it.
- As an author, chapter refinement can use timeline-aware Codex and outline neighbors.
- As a developer, context packets are inspectable in tests/logs.

### TDD focus

- context pack builder,
- local vs global RAG policy,
- active progression integration,
- token budget trimming.

### Done when

Two scenes at different timeline positions produce different Codex context for
the same character, paragraph actions are proven not to send wider context, and
conditional follow-ups make no provider call until the author explicitly runs
them.

---

## Milestone 8 — What-if branches and ramification analysis

Status: complete July 2, 2026. Durable contract:
`docs/17_milestone_8_task_prompt.md`. Evidence:
`.plans/milestone_8_test_evidence.md`.

### Goal

Support controlled what-if experiments from fixed `main` canon with direct
comparison, explicit ramification analysis, and conservative whole-file
promotion.

### Stories

- As an author, I can create a what-if experiment from current `main` and
  continue normal story, Codex, import, and AI flows on that branch.
- As an author, I can compare the active experiment to current `main` without
  switching away from it.
- As an author, I can inspect read-only side-by-side canon and experiment text
  with accessible line-level highlighting.
- As an author, I can request explicit ramification analysis and receive strict
  advisory findings without file mutation.
- As an author, I can promote selected complete files to `main` or discard the
  experiment explicitly.

### TDD focus

- `internal/branch` lifecycle, comparison, promotion, and discard orchestration,
- `internal/modelchat` shared transport extraction,
- `internal/projectcheck` full-project validation before promotion,
- Git adapter branch/ref/blob/tree operations,
- Branches workspace UI with stale-response protection and branch-change
  invalidation.

### Done when

The user can create an experiment, mutate it while `main` stays unchanged,
compare against current canon from Git objects, run explicit analysis, promote
selected whole files with rollback-safe validation, or discard safely.

---

## Milestone 9 — MVP hardening

### Goal

Make the app daily-usable for one author.

### Stories

- As an author, I can create a project backup snapshot.
- As an author, I can export Markdown.
- As an author, I can recover from a deleted SQLite index by rebuilding it.
- As an author, I can see recent AI runs and what context/prompt was used.
- As a developer, all acceptance tests are documented and pass.

### Done when

The full MVP success criteria from `00_project_brief.md` are satisfied.
