# 09 — Milestone 0 Coding Agent Task Prompt

Give this file to the coding agent when you are ready to start implementation.

## Task

Implement **Milestone 0 — Foundation and local project skeleton**.

Do not implement later milestones except where a tiny stub is needed to keep the architecture clean.

## Required behavior

1. Go backend starts.
2. React/Vite frontend starts.
3. `GET /api/health` returns ok.
4. `POST /api/projects` creates a project folder.
5. Project creation writes starter canonical files:
   - `project.yaml`
   - `outline.yaml`
   - starter directories
   - starter built-in agent/style templates if simple enough
6. Project creation initializes Git.
7. Project creation creates `.storywork/index.sqlite`.
8. Project opening validates/rebuilds index.
9. `make check` runs backend tests and frontend checks that exist.

## Do first

1. Read all project docs.
2. Read local Go rules if present.
3. Write tests first.
4. Implement the smallest working slice.

## Suggested backend packages

```text
internal/project
internal/gitstore
internal/index
internal/api
internal/app
internal/testutil
```

## Suggested initial interfaces

```go
type ProjectStore interface {
    Create(ctx context.Context, req CreateProjectRequest) (Project, error)
    Open(ctx context.Context, path string) (Project, error)
}

type GitStore interface {
    Init(ctx context.Context, path string) error
    CommitAll(ctx context.Context, path string, message string) error
    IsRepo(ctx context.Context, path string) (bool, error)
}

type IndexStore interface {
    Init(ctx context.Context, projectPath string) error
    Rebuild(ctx context.Context, projectPath string) error
    Verify(ctx context.Context, projectPath string) error
}
```

These names are suggestions. Prefer the user's local Go rules if they specify naming/layout.

## Tests to create first

- `CreateProject` writes expected files.
- `CreateProject` initializes Git.
- `CreateProject` initializes SQLite index.
- `OpenProject` succeeds for a valid project.
- `OpenProject` fails clearly for an invalid path.
- `Health` endpoint returns ok.

## Acceptance commands

```bash
go fmt ./...
go vet ./...
go test ./...
make check
```

If frontend checks are not wired yet, `make check` may run only backend commands, but include clear TODO comments in the Makefile.

