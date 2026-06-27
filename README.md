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
2. `docs/00_project_brief.md`
3. `docs/01_development_flow.md`
4. `docs/02_architecture.md`
5. `docs/03_storage_model.md`
6. `docs/04_agent_style_system.md`
7. `docs/05_milestones.md`
8. `docs/06_api_contract.md`
9. `docs/07_frontend_editor.md`
10. `docs/08_testing_acceptance.md`

Then implement **Milestone 0 only** unless the user explicitly asks for a later milestone.

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

## First implementation target

Milestone 0 creates a runnable local skeleton:

- Go API server starts.
- React frontend starts.
- A project folder can be created/opened.
- Project folder is initialized as a Git repo.
- Canonical starter files are written.
- SQLite index is created under `.storywork/` and can be rebuilt.
- Health/status endpoint works.
- Tests exist before code for the milestone behavior.

