# 01 — Development Flow

## Planning model

Use this structure:

```text
Epic: MVP
  Milestone/Sprint
    Story in BDD format
      TDD tests
        implementation
          acceptance check
```

Only one Epic exists now: **MVP**.

Each milestone must be a working vertical slice.

## BDD story format

Use:

```text
As a <role>
I want <capability>
So that <reason>
```

Add acceptance criteria:

```gherkin
Given <state>
When <action>
Then <observable result>
```

## TDD workflow

For each story:

1. Write failing tests for the core behavior.
2. Run tests and confirm failure is meaningful.
3. Implement the smallest code that passes.
4. Refactor only after tests pass.
5. Update docs if behavior changed.
6. Run milestone check commands.

## Testing levels

### Domain tests

Pure Go tests for decisions and state transitions.

Examples:

- agent applicability decisions,
- context policy selection,
- progression activation by story position,
- branch ramification scopes,
- import candidate merge decisions.

These should not require HTTP, SQLite, Git, or model calls.

### Adapter tests

Test boundaries with fakes or temp directories.

Examples:

- Git repository initialization,
- SQLite index rebuild,
- filesystem project writes,
- provider adapter request mapping.

### API tests

Test HTTP handlers with in-memory/temp test fixtures.

### Frontend tests

Test components and user flows where practical:

- outline tree rendering,
- agent availability menu,
- selection action request shape,
- accept/reject patch behavior.

## Check commands

Backend:

```bash
go fmt ./...
go vet ./...
go test ./...
```

Frontend:

```bash
npm run lint
npm run typecheck
npm test -- --run
```

All:

```bash
make check
```

Create `make check` during Milestone 0.

## Definition of done for every milestone

A milestone is done only when:

1. stories have acceptance criteria,
2. tests exist for the milestone behavior,
3. the app runs locally,
4. the feature is usable through API or UI as specified,
5. docs are updated,
6. `make check` passes,
7. no secrets or generated junk are committed.

