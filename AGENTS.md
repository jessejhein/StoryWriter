# Coding Agent Instructions

You are implementing **AI Story Workshop**. Follow these rules.

## Development discipline

This project is TDD.

Workflow:

```text
Epic -> Milestone/Sprint -> BDD Stories -> TDD Tests -> Code -> Acceptance Check
```

There is one Epic for now: **MVP**.

Each milestone must be complete and working for its current feature set. Do not build half of Milestone 3 while Milestone 0 is broken. This is how projects become archaeology.

## Before coding

1. Read this file.
2. Read `README.md`.
3. Read docs in the required order listed in `README.md`.
4. Check for local rules:
   - `LOCAL_GO_RULES.md`
   - `docs/local_go_rules.md`
   - `.codex/local_go_rules.md`
5. Identify the current milestone.
6. Write or update tests first.

## Implementation rules

- Prefer boring, idiomatic Go.
- Keep `main` small.
- Put internal application code under `internal/`.
- Use interfaces at boundaries: model providers, Git operations, file system, SQLite index, clock, ID generation.
- Do not leak provider-specific response shapes through the app.
- Separate decisions from actions. Make eligibility/context/agent-selection decisions inspectable and testable.
- Avoid premature generics, reflection, and clever framework magic.
- Never store provider tokens or API keys inside story project folders.
- Never let AI output silently mutate canon. AI output becomes a suggestion, patch, draft, or proposal until the author accepts it.
- Text files are canonical project state. SQLite is rebuildable index/cache unless a doc explicitly says otherwise.
- Git is for story state history and what-if branches. Do not use Git as a query engine.

## Background processes

Do not start long-running background servers without the user's permission unless the task explicitly requires it.

If you must start one:

1. Say what command you are running.
2. Capture logs.
3. Provide how to stop it.
4. Stop it before handing back unless the user asked it to stay running.

## Test commands

Preferred Go toolchain:

- Use `/home/linuxbrew/.linuxbrew/bin/go` first when available.
- Fall back to `go` if that path is unavailable.

Backend baseline:

```bash
go fmt ./...
go vet ./...
go test ./...
```

Frontend baseline, once frontend exists:

```bash
npm run lint
npm run typecheck
npm test -- --run
```

Full baseline, once wired:

```bash
make check
```

If a command does not exist yet, create it when appropriate rather than pretending it passed. A missing test command is not a green build; it is a todo with delusions.

## Commit behavior

The app will eventually auto-commit project changes inside user story projects. That is separate from committing this source repository.

For this source repository:

- Keep changes small.
- Update docs when implementation changes behavior.
- Do not commit secrets, generated binaries, SQLite databases, or node modules.
