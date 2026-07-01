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
│   ├── agent/
│   ├── api/
│   ├── codex/
│   ├── config/
│   ├── gitstore/
│   ├── index/
│   ├── llm/
│   ├── project/
│   ├── story/
│   └── testutil/
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

### Adapter layer

Talks to Git, SQLite, model providers, filesystem, HTTP.

## Future Electron path

The architecture should allow Electron/Tauri later:

- frontend builds as static assets,
- Go backend can run as a local process,
- project folders live in user-selected directories,
- credentials can use OS store.

Do not build Electron in MVP.
