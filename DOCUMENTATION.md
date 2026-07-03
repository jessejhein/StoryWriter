# Storywork Documentation Standards

**Last Updated:** July 2026
**Version:** Milestone 8

This document defines the documentation and code commenting standards for **Storywork** — a local-first creative writing application with a Go backend and a Vite + React + TypeScript frontend.

---

## 1. Project Overview

Storywork helps writers organize long-form stories through:

- Hierarchical story structure (Arcs → Chapters → Scenes)
- Powerful Codex (world-building entities with relationships and progressions)
- Markdown-based scene editor with frontmatter
- Local-first design (files on disk, Git-friendly)

**Architecture:**
- **Backend**: Go (HTTP JSON API)
- **Frontend**: Vite + React + TypeScript + project CSS

---

## 2. General Principles

- **Clarity over cleverness**
- **Documentation is part of the code**, not an afterthought
- Write for **newcomers** — assume the reader knows the language but not the project
- Prefer **present tense** and **complete sentences**
- Keep documentation close to the code it describes

---

## 3. Go Backend Documentation Standards

### 3.1 Package Documentation

Every package must have a clear package-level comment, ideally in a `doc.go` file:

```go
// Package api provides the local HTTP API for the Storywork application.
//
// It implements a thin JSON HTTP layer that delegates business logic to
// internal services (project, story, codex). All routes follow RESTful
// conventions where appropriate.
package api
```

### 3.2 File-Level Comments

Add a short comment at the top of each `.go` file (after the package comment):

```go
// handler.go implements the Storywork HTTP handlers and routing policy.
```

### 3.3 Exported Identifiers

All exported types, functions, methods, and constants **must** have documentation:

```go
// StoryStore serves and mutates the active project's outline and codex.
//
// It is the primary domain service used by the HTTP layer and frontend.
type StoryStore interface {
	// Outline returns the full hierarchical structure of the story.
	Outline(ctx context.Context) (story.Outline, error)

	// CreateArc adds a new top-level arc to the story.
	CreateArc(ctx context.Context, title string) (story.MutationResult, error)
}
```

### 3.4 Functions & Methods

```go
// NewHandler creates and returns a fully configured http.Handler containing
// all API routes for the current milestone.
//
// It wires together the project, session, and story services.
func NewHandler(projects ProjectStore, session ActiveProjectSession, stories StoryStore, version string) http.Handler
```

### 3.5 Internal / Unexported Code

Still document complex logic:

```go
// writeStoryError maps domain errors to appropriate HTTP status codes.
//
// This centralizes our error handling policy.
func writeStoryError(writer http.ResponseWriter, err error) {
```

**Use `//` comments** (never block comments for documentation).

---

## 4. Frontend (Vite + React + TypeScript) Documentation Standards

### 4.1 File-Level Comments

Every major file should start with a comment block:

```tsx
/**
 * SceneEditor.tsx
 *
 * Primary editor for individual story scenes. Handles markdown editing,
 * frontmatter management, explicit saves, and revision conflicts.
 *
 * Uses the shared CodeMirror surface for Vim-friendly Markdown editing.
 */
```

### 4.2 Components

```tsx
/**
 * CodexEntryCard
 *
 * Displays a single codex entry with name, aliases, tags, and description.
 * Supports drag-and-drop for reordering and clicking to open the detail view.
 */
export function CodexEntryCard({ entry, onEdit }: CodexEntryCardProps) {
```

### 4.3 Hooks & Utilities

```ts
/**
 * useActiveCodexState
 *
 * Custom hook that resolves the current "active" state of a codex entry
 * relative to the current scene (e.g. character status, relationship state).
 */
export function useActiveCodexState(entryId: string, sceneId: string) {
```

### 4.4 Types & Interfaces

```ts
/**
 * SceneDocument
 *
 * Represents a complete scene file on disk, including metadata and content.
 */
export interface SceneDocument {
  id: string;
  title: string;
  frontmatter: SceneFrontMatter;
  markdown: string;
  revision: string;
  lastModified: string;
}
```

### 4.5 Comments Inside Components

Use `//` for inline explanations of complex logic:

```tsx
// Ignore an older response after the author navigates to a different entry.
if (selectionVersion.current !== selectionAtStart) {
  return;
}
```

---

## 5. Documentation Style Guide

| Element              | Style                              | Example                              |
|----------------------|------------------------------------|--------------------------------------|
| Package / File       | Present tense, purpose-focused     | "Provides the HTTP API..."           |
| Functions            | What it returns / does             | "Creates a new arc..."               |
| Parameters           | Describe constraints               | "title must be non-empty"            |
| Complex logic        | Explain *why*                      | "// Avoid race condition when..."    |
| TODOs                | Clear action + milestone           | `// TODO: Add auth (M1)`             |

**Prohibited:**
- Vague comments ("do the thing")
- Outdated comments
- Commented-out code (use `git` instead)

---

## 6. Generating & Viewing Documentation

**Backend:**
```bash
go install golang.org/x/pkgsite/cmd/pkgsite@latest
pkgsite -open .
```

**Frontend:**
- Use JSDoc + TypeScript types
- Consider Storybook for component documentation (future)

---

## 7. API Documentation

All HTTP endpoints are self-documenting via the handler code. When adding a new route:

1. Document the handler function
2. Document request/response types
3. Update this `DOCUMENTATION.md` under the API section (if major)

Implemented through Milestone 8:

```text
GET  /api/codex
POST /api/codex
GET  /api/codex/{entry_id}
PUT  /api/codex/{entry_id}
GET  /api/codex/{entry_id}/progressions
PUT  /api/codex/{entry_id}/progressions
GET  /api/codex/{entry_id}/active?scene_id={scene_id}
GET  /api/agents
GET  /api/styles
GET  /api/actions/available?surface={surface}&input_scope={scope}&scene_id={scene_id}&selection_words={count}
POST /api/actions/context-preview
POST /api/actions/run
POST /api/actions/{run_id}/accept
POST /api/actions/{run_id}/reject
POST /api/action-invitations/{invitation_id}/run
GET  /api/provider-profiles
PUT  /api/provider-profiles
POST /api/imports
GET  /api/imports
GET  /api/imports/{import_id}
GET  /api/imports/{import_id}/chunks
POST /api/imports/{import_id}/extractions
GET  /api/import-candidates
GET  /api/import-candidates/{candidate_id}
PUT  /api/import-candidates/{candidate_id}
POST /api/import-candidates/{candidate_id}/merge
POST /api/import-candidates/{candidate_id}/discard
POST /api/import-candidates/{candidate_id}/accept
GET  /api/branches/status
GET  /api/branches
POST /api/branches
POST /api/branches/switch
GET  /api/branches/{experiment_id}/comparison
GET  /api/branches/{experiment_id}/comparison/file?path=<project-relative-path>
POST /api/branches/{experiment_id}/ramifications
POST /api/branches/{experiment_id}/promote
POST /api/branches/{experiment_id}/discard
```

Mutation requests use strict JSON: unknown, missing, null, trailing, and
wrongly typed fields are rejected unless a field is explicitly documented as
nullable. Entry and progression updates use exact-byte revision tokens.

Milestone 4 action routes use strict UTF-8 byte selections into canonical scene
Markdown, transient in-memory run records, explicit accept/reject decisions,
and the existing scene mutation rollback/checkpoint guarantees. The durable
contract remains `docs/13_milestone_4_task_prompt.md`.

Milestone 5 adds strict application-level provider profile replacement,
credential-readiness reporting, provider-aware style availability, and
provider identity on transient action previews. Its durable contract is
`docs/14_milestone_5_task_prompt.md`; no route accepts or returns credentials.

Milestone 6 adds tracked Markdown import snapshots, rebuildable import chunks,
provider-neutral structure extraction, a durable candidate review queue, and
explicit accept/edit/merge/discard routes. Canon still changes only through the
story-owned mutation services after explicit acceptance. Its durable contract is
`docs/15_milestone_6_task_prompt.md`.

Milestone 7 adds redacted context preview, tagged selection/scene/chapter-review
runs, deterministic follow-up invitations, and Git commit trailers for accepted
operation lineage. Context assembly lives in `internal/contextpack`; providers
receive scope-specific messages only. Its durable contract is
`docs/16_milestone_7_task_prompt.md`.

Milestone 8 adds managed `branch/` experiments from fixed `main` canon,
read-only comparison against current `main` while an experiment stays checked
out, explicit transient ramification analysis, and conservative whole-file
promotion with `internal/projectcheck` validation. Shared provider chat
transport lives in `internal/modelchat`. Its durable contract is
`docs/17_milestone_8_task_prompt.md`.

Example:

```ts
// POST /api/scenes
// Creates a new scene inside a chapter
interface CreateSceneRequest {
  chapter_id: string;
  title: string;
}
```

---

## 8. Contribution Workflow

1. Write code **and** documentation together
2. Run `go fmt ./...` and linting
3. Ensure all exported symbols are documented
4. Update this file when introducing major patterns
5. Commit with a concise message that describes the completed behavior

---

## 9. Vision

Our goal is that **any developer** (including future contributors or yourself in 6 months) can understand the system by reading the code and this documentation — without needing to ask questions.

**"Good documentation makes the code obvious. Great documentation makes the intent obvious."**

---

**Status:** Living Document — update as standards evolve
