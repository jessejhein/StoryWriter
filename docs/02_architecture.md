# 02 — Architecture

## High-level shape

```text
Vite React UI
  -> Go HTTP API
      -> Project service
      -> Git service
      -> File store service
      -> SQLite index service
      -> Agent/style service
      -> AI orchestration service
          -> Provider adapters
          -> Eino-backed adapters where useful
          -> Local endpoint adapters
```

## Core architectural decisions

### Go backend only

No Python layer for MVP. Python adds another runtime and deployment surface.

### Frontend: Vite + React + TypeScript

Earlier planning mentioned Next.js as possible. For MVP, prefer Vite React SPA because:

- Go owns the backend API.
- The app is local-first.
- The SPA can later be packaged into Electron/Tauri more simply.
- CodeMirror integration is straightforward.

### Git + text files + SQLite

Use:

- **Git** for history, branches, snapshots, diffs, and what-if experiments.
- **Markdown/YAML/JSONL files** as canonical project state.
- **SQLite** as a rebuildable local index/cache/query engine.

Do not use Git as the query engine. Do not store the whole project as one giant JSON blob. That way lies sadness and merge conflicts.

### Interfaces at boundaries

Create interfaces for:

- model providers,
- embeddings providers,
- Git operations,
- project filesystem,
- SQLite index,
- ID generation,
- clock/time,
- credential provider.

Provider-specific request/response formats must be converted at the boundary.

### Credentials outside project folders

Never store provider credentials in story projects.

Preferred order:

1. OS credential store / keychain where available.
2. Provider OAuth/device flow where officially supported.
3. Environment variables for local/dev use.
4. Encrypted app-level credential file only as fallback.

The browser UI should not directly own long-lived provider tokens. In local web mode, the Go backend is the credential broker.

Milestone 5 implements this boundary with application-level `providers.yaml`,
an environment credential broker, and one provider-neutral text-generation
interface. Profile storage and readiness belong to `internal/provider`; the
action lifecycle depends only on the generation and profile-resolution
boundaries. OpenAI-compatible and Ollama HTTP shapes are adapter details and do
not enter action runs or API request models. Credentials are resolved for each
availability/run decision and are passed only to the outbound adapter.

Milestone 6 adds a second provider-neutral AI consumer beside scene actions:
`internal/extract`. Import snapshotting, chunk persistence, review queue state,
and candidate acceptance live in `internal/importer`; prompt assembly and model
output validation live in `internal/extract`. This keeps import/review rules
separate from scene-patch orchestration and preserves a narrow seam for future
extraction modes or candidate kinds.

Milestone 7 adds timeline-aware action context without widening provider access
to story storage:

- `internal/contextpack` owns typed targets, material packets, lexical relevance,
  deterministic budgeting, and redacted manifests.
- `internal/story/context_material.go` loads one coherent canonical snapshot per
  target under the shared `internal/mutation.Coordinator` read lock.
- `internal/action` orchestrates preview (zero provider), tagged runs for
  selection/scene/chapter review, invitation policy/store, and accepted-operation
  lineage metadata threaded into Git commit trailers.
- `internal/agent` keeps registry validation and scope/output-specific provider
  message builders; providers remain transport adapters with no story access.
- `internal/gitstore/commit_message.go` formats validated commit subjects and
  trailers at the Git consumer boundary.

## Suggested source repository layout

```text
.
├── AGENTS.md
├── README.md
├── Makefile
├── go.mod
├── cmd/
│   └── storywork/
│       └── main.go
├── internal/
│   ├── app/
│   ├── action/
│   ├── agent/
│   ├── api/
│   ├── codex/
│   ├── contextpack/
│   ├── extract/
│   ├── gitstore/
│   ├── importer/
│   ├── index/
│   ├── mutation/
│   ├── provider/
│   ├── project/
│   ├── story/
│   ├── storyfile/
│   └── workspace/
├── web/
│   ├── package.json
│   ├── vite.config.ts
│   └── src/
├── docs/
├── templates/
└── testdata/
```

## Important boundaries

### Domain layer

Pure rules. No filesystem, HTTP, Git, or network calls.

### Application/services layer

Coordinates domain decisions and adapters.

Milestone 6 uses two bounded service paths:

- `internal/importer.Service` owns import manifests, chunk rebuild triggers,
  durable review state, and revision-safe review transactions.
- `internal/story.Service` remains the only owner of canonical story writes.
  Import candidate acceptance delegates through narrow story mutation ports so a
  successful acceptance still lands as one logical checkpointed mutation.
- `internal/mutation.Coordinator` is the application-scoped read/write boundary
  shared by both services. It prevents import/review writes, canonical story
  writes, index rebuilds, checkpoints, and rollback from interleaving across one
  logical mutation. Import acceptance calls the story-owned no-checkpoint port
  only while this coordinator is already held.

### Adapter layer

Talks to Git, SQLite, model providers, filesystem, HTTP.

## Future Electron path

The architecture should allow Electron/Tauri later:

- frontend builds as static assets,
- Go backend can run as a local process,
- project folders live in user-selected directories,
- credentials can use OS store.

Do not build Electron in MVP.
