# AI Story Workshop — Coding Agent Handoff

Working title: **AI Story Workshop**. Rename later; do not spend implementation time on naming.

This repository is intended to be implemented by a coding agent using a strict milestone/TDD flow.

## What this is

A local-first AI writing workshop for single authors who want to:

- import messy story notes and chat logs,
- extract a usable Codex, outline, and scene stubs,
- write and revise with specialized AI agents/styles,
- minimize context sent to models,
- run cheap/local models when useful,
- branch story decisions with Git,
- keep the human author in full control of canon.

This is **not** an autopilot novelist. Chat is allowed later, but the base product is structured author assistance: import, organize, revise, branch, compare, and selectively regenerate.

## Required reading order for coding agents

1. `AGENTS.md`
2. `DOCUMENTATION.md`
3. `docs/00_project_brief.md`
4. `docs/01_development_flow.md`
5. `docs/02_architecture.md`
6. `docs/03_storage_model.md`
7. `docs/04_agent_style_system.md`
8. `docs/05_milestones.md`
9. `docs/06_api_contract.md`
10. `docs/07_frontend_editor.md`
11. `docs/08_testing_acceptance.md`
12. `docs/09_milestone_0_task_prompt.md`
13. `docs/10_milestone_1_task_prompt.md`
14. `docs/11_milestone_2_task_prompt.md`
15. `docs/12_milestone_3_task_prompt.md`
16. `docs/13_milestone_4_task_prompt.md`
17. `docs/14_milestone_5_task_prompt.md`
18. `docs/15_milestone_6_task_prompt.md`
19. `docs/16_milestone_7_task_prompt.md`
20. `docs/17_milestone_8_task_prompt.md`

Then implement only the milestone the user assigned. If the user did not assign one,
stop after identifying the next incomplete milestone and ask before coding.

## Local Go rules

The user will add local Go coding rules. When present, read them first and obey them:

- `LOCAL_GO_RULES.md`
- `docs/local_go_rules.md`
- `.codex/local_go_rules.md`

If those rules conflict with these docs, prefer the local Go rules for coding style, but do not change product requirements unless the user says so.

## Initial tech choices

- Backend: Go.
- AI orchestration: Go interfaces, with Eino allowed behind adapters.
- Frontend: Vite + React + TypeScript for MVP.
- Editor: CodeMirror 6 with Vim keybindings.
- Storage: Git + text files as canonical project state; SQLite as rebuildable index/cache.
- Credentials: OS/browser/provider credential mechanisms where possible; never store provider secrets in project folders.
- Deployment: local dev first; Docker Compose may be added, but the app should also run directly.

## Implemented foundation

Milestone 0 creates a runnable local skeleton:

- Go API server starts.
- React frontend starts.
- A project folder can be created/opened.
- Project folder is initialized as a Git repo.
- Canonical starter files are written.
- SQLite index is created under `.storywork/` and can be rebuilt.
- Health/status endpoint works.
- Tests exist before code for the milestone behavior.

## Current implementation status

Milestones 0 through 8 are implemented and the verified Milestone 8 check set
was green on July 5, 2026. The app includes the local project foundation,
canonical outline editing, the Vim-friendly scene editor with revision
conflict protection and one Git checkpoint per explicit save, the Codex
workbench with strict progression validation and active-state resolution, the
agent/style registry and reviewable AI patch workflow, application-level
provider profiles with real OpenAI-compatible and Ollama adapters, the
Milestone 6 Markdown import and review queue, Milestone 7 timeline-aware
context with scene rewrite, chapter review suggestions, context preview, and
explicit follow-up invitations, and Milestone 8 what-if branches with live
main-versus-experiment comparison, explicit ramification analysis, and
rollback-safe whole-file promotion from fixed `main` canon.

Milestone 4 adds:

- strict project-local `agents/*.yaml` and `styles/*.yaml` loading,
- deterministic registry listing and applicability filtering,
- a provider-neutral mock Line Polish run path with minimal context,
- transient reviewable runs with explicit reject/accept,
- revision-safe patch acceptance using the existing scene lock, rollback, index rebuild, and exactly one Git commit.

Milestone 6 adds:

- tracked `imports/raw/<import_id>/` Markdown snapshots with strict path and
  content validation,
- rebuildable deterministic chunk data under `.storywork/import/`,
- provider-neutral structure extraction into durable
  `imports/review/<candidate_id>.yaml` proposals,
- explicit edit, merge, discard, and accept review operations with exactly one
  checkpoint per successful mutation,
- acceptance through story-owned mutation ports so extraction never mutates
  canon until the author accepts a candidate,
- the Import Review workbench and strict HTTP routes for the full flow.

Milestone 7 adds timeline-aware context assembly (`internal/contextpack`),
redacted context preview, scene-scoped Scene Rewrite, chapter-scoped Chapter
Review suggestions, deterministic follow-up invitations, and Git commit
trailers for accepted operation lineage. Its durable contract is
`docs/16_milestone_7_task_prompt.md`.

Milestone 8 adds managed `branch/` experiments, live canon-versus-experiment
comparison from Git objects, transient ramification analysis, and rollback-safe
whole-file promotion. Its durable contract is `docs/17_milestone_8_task_prompt.md`.
Milestone 9 is the next incomplete phase.

## Development

Requirements:

- Go 1.26 or newer
- Node.js 22 or newer
- Git

Install frontend dependencies and run all checks:

```bash
cd web && npm install && cd ..
make check
```

Run the backend and frontend in separate terminals:

```bash
make dev-backend
make dev-frontend
```

The API listens on `127.0.0.1:9090`. Vite proxies `/api` requests to it.

## Current package map

- `cmd/storywork`: process startup and graceful shutdown only.
- `internal/app`: production dependency composition.
- `internal/api`: HTTP transport and JSON validation.
- `internal/project`: project creation/opening orchestration.
- `internal/gitstore`: Git command adapter.
- `internal/index`: rebuildable SQLite index adapter.
- `internal/agent`: strict agent/style registry (v1–v3), applicability, scope-specific message builders, and provider boundaries.
- `internal/branch`: what-if experiment lifecycle, live comparison, ramification orchestration, promotion, and discard policy.
- `internal/action`: scope-aware action orchestration, context preview, invitations, lineage, and accept/reject run lifecycle.
- `internal/contextpack`: deterministic context policy, budgeting, lexical relevance, and redacted manifests.
- `internal/extract`: provider-neutral extraction request, prompt, and response validation.
- `internal/importer`: Markdown snapshot import, chunking, review queue, and acceptance orchestration.
- `internal/modelchat`: provider-neutral OpenAI-compatible and Ollama chat transport shared by agent actions, extraction, and branch analysis.
- `internal/mutation`: shared transaction coordination for story and durable review mutations.
- `internal/projectcheck`: read-only full-project canonical validation composition.
- `internal/story`: canonical outline, scene, and Codex mutation orchestration.
- `internal/codex`: pure Codex validation and active-state decisions.
- `internal/provider`: application-level provider profile storage and readiness resolution.
- `internal/storyfile`: strict canonical story-file adapters.
- `templates`: embedded canonical starter files.
- `web`: React/Vite local UI, including the Branches workspace and side-by-side comparison.

Working implementation plans belong in `.plans/`, which is ignored by Git. Product decisions and durable behavior belong in `docs/`.
