# 10 — Milestone 1 Coding Agent Task Prompt

Use this file as the implementation contract for **Milestone 1 — Story structure
files and basic outline UI**. Read all earlier required documents first.

Do not implement Milestone 2. In particular, do not add CodeMirror, scene prose
editing, AI behavior, Codex behavior, deletion, rename, or cross-parent moves.

## Outcome

An author who creates or opens a project can:

1. view its arc -> chapter -> scene hierarchy,
2. create an arc,
3. create a chapter in an arc,
4. create a scene in a chapter,
5. reorder chapters within one arc,
6. reorder scenes within one chapter,
7. see a Git checkpoint after every successful structural mutation.

Canonical state remains readable text. SQLite remains derived and rebuildable.

## Required BDD stories

### Story 1.1 — View the outline

```gherkin
Given I created or opened a valid project
When I request the outline
Then I receive arcs containing their ordered chapters and scenes
And each node includes its stable ID, title, and derived display label
```

```gherkin
Given no project is active in this backend process
When I request the outline
Then I receive 409 Conflict with a useful error
```

### Story 1.2 — Create structure

```gherkin
Given a clean active project
When I create an arc, a chapter in that arc, and a scene in that chapter
Then their canonical files are written
And outline.yaml references them in creation order
And each ID has the correct prefix and remains unchanged
And each successful request creates exactly one Git commit
```

### Story 1.3 — Reorder structure

```gherkin
Given an arc with two chapters
When I submit both chapter IDs in reverse order
Then outline.yaml stores the new order
And the chapter IDs and chapter files are unchanged
And the derived chapter display labels reflect the new order
And exactly one Git commit is created
```

Repeat this case for two scenes in one chapter.

```gherkin
Given an existing parent and children
When I submit missing, duplicate, unknown, or foreign child IDs
Then the request fails
And canonical files are unchanged
And Git history is unchanged
```

### Story 1.4 — Preserve checkpoint integrity

```gherkin
Given the active story project has uncommitted changes
When I request a structural mutation
Then the request returns 409 Conflict
And no project file or Git commit is changed
```

```gherkin
Given a clean active project
And Git checkpoint creation fails
When I request a structural mutation
Then the request fails
And canonical files are restored to their pre-request contents
And the repository is left unstaged
```

### Story 1.5 — Use the outline UI

```gherkin
Given I create or open a project in the UI
When the outline loads
Then I see nested arcs, chapters, and scenes
And I can create each node at the valid parent
And I can reorder chapters and scenes
And the updated tree is rendered without reloading the page
```

## Fixed design decisions

Do not invent alternatives to these decisions during implementation.

### Active project

The API is a local, single-author process. Add a concurrency-safe in-memory active
project session. A successful `POST /api/projects` or `POST /api/projects/open`
sets it. Backend restart clears it. Structure handlers read it; clients do not
send arbitrary project paths on each structure request.

Suggested package: `internal/workspace`.

The session stores the validated `project.Project`, protects reads/writes with
`sync.RWMutex`, and exposes only `Set(project.Project)` and
`Current() (project.Project, bool)`.

### Stable IDs

IDs are opaque and never derived from titles or display order.

- Arc: `arc_` plus 20 lowercase hexadecimal characters.
- Chapter: `ch_` plus 20 lowercase hexadecimal characters.
- Scene: `scn_` plus 20 lowercase hexadecimal characters.

Define an ID generator at the consumer boundary. Production generation reads 10
bytes from `crypto/rand` and hex-encodes them. Tests inject fixed IDs. Validate IDs
before using them in a path. Never concatenate an unchecked request ID into a
filesystem path.

Before creating an entity file, verify the generated ID is unused. On collision,
generate again up to five times and then return an error. Never overwrite an
existing entity file.

### Canonical ordering and labels

`outline.yaml` is the only ordering authority. Arc/chapter/scene files contain
identity and content metadata, not numeric order or display labels. Reordering
therefore writes only `outline.yaml`.

Derive labels at read time:

- `Arc 1`
- `Chapter 1.1` for the first chapter of the first arc
- `Scene 1.1.1` for the first scene of that chapter

Titles do not affect labels or IDs.

### Exact canonical schemas

Use the formats in `docs/03_storage_model.md`. The exact outline shape is:

```yaml
version: 1
root:
  arcs:
    - id: arc_0123456789abcdef0123
      chapters:
        - id: ch_0123456789abcdef0123
          scenes:
            - id: scn_0123456789abcdef0123
```

Use `gopkg.in/yaml.v3`. Decode with known-field checking. Write deterministic
two-space-indented YAML ending in one newline. Do not hand-parse YAML.

Load referenced entity files and validate:

- supported `version` where the schema has one,
- correctly prefixed ID,
- no duplicate IDs in the outline,
- every referenced entity file exists,
- chapter `arc_id` matches its containing arc,
- scene `chapter_id` matches its containing chapter,
- non-empty title after trimming.

Return contextual errors that name the bad file or ID. Do not silently repair
malformed canonical files in Milestone 1.

### Mutation rules

- Trim titles.
- Reject empty titles and titles longer than 200 Unicode code points.
- A new node is appended to its parent's child list.
- A chapter requires an existing arc.
- A scene requires an existing chapter.
- Reorder input must be an exact permutation of current direct children.
- Reorder does not move children between parents.
- Reject a structural mutation when the story project's Git worktree is dirty.
- Serialize structural mutations in the service so two requests cannot both read
  the same outline and overwrite each other. Reads must not observe a partial
  mutation.

### Checkpoint and failure behavior

Every successful create or reorder request produces exactly one Git commit. Use
these messages:

```text
Add arc <id>
Add chapter <id>
Add scene <id>
Reorder chapters in <arc-id>
Reorder scenes in <chapter-id>
```

Use this mutation sequence:

1. Resolve the active project.
2. Verify its Git worktree is clean.
3. Load and validate current canonical state.
4. Apply the requested change to an in-memory copy.
5. Snapshot every file that will be created or replaced.
6. Write changed files atomically using a temporary file in the destination
   directory followed by rename.
7. Rebuild the disposable index.
8. Commit all project changes with the exact message above.
9. Return the newly loaded outline view.

If steps 6 through 8 fail, restore the snapshot, remove newly created files,
unstage app-staged changes, and rebuild the index from the restored files. Return
the original error joined with any rollback error. Never report success without a
checkpoint.

The clean-worktree precondition makes `CommitAll` safe: it cannot absorb unrelated
author changes. Extend the Git boundary with explicit clean-check and unstage/reset
operations rather than running Git commands from the story service.

### Derived index

Rebuild the existing SQLite index after canonical writes and before committing.
The index database is ignored by Git. No Milestone 1-specific SQLite tables are
required; the existing file manifest must reflect newly created canonical files.

### HTTP API

Implement exactly the routes, request/response bodies, and status rules in
`docs/06_api_contract.md`.

Keep JSON decoding strict: reject unknown fields and trailing JSON values. The
current decoder rejects unknown fields but does not reject a second JSON value;
fix that and retain the 1 MiB request limit.

Do not expose filesystem paths in outline mutation requests.

### Frontend

After project create/open succeeds, replace the Milestone 0 completion card with
an outline workbench while retaining visible project path and backend status.

Required UI states:

- loading,
- empty outline with a create-arc action,
- nested outline,
- request error with retry,
- mutation in progress with duplicate submission disabled.

Each arc has an add-chapter control. Each chapter has an add-scene control. Use a
small inline form for a title and cancel/submit controls.

Support chapter and scene reorder with accessible Move up / Move down buttons.
Drag/drop may be added using a maintained sortable library, but it is not a
substitute for keyboard-operable controls. Do not add arc reorder.

Keep API types and calls in `web/src/api.ts`. Extract outline components from
`App.tsx`; do not grow the root component into the entire feature.

## Package responsibilities

Use this map unless an existing package already owns the responsibility:

```text
internal/workspace  active project session only
internal/story      pure outline model, validation, labels, reorder decisions,
                    and structural application service
internal/storyfile  strict YAML/Markdown structure persistence and atomic writes
internal/gitstore   Git clean check, commit, and rollback/reset adapter methods
internal/api        HTTP transport, JSON validation, and status mapping
internal/app        production dependency composition only
web/src/outline     outline components and interaction state
```

Do not put domain ordering rules in HTTP handlers, React components, or the Git
adapter. Keep `cmd/storywork/main.go` unchanged unless startup wiring genuinely
requires a small change.

## Expected source changes

This is a navigation checklist, not permission to put unrelated behavior in a
file:

```text
internal/workspace/session.go
internal/workspace/session_test.go
internal/story/model.go
internal/story/model_test.go
internal/story/service.go
internal/story/service_test.go
internal/storyfile/store.go
internal/storyfile/store_test.go
internal/gitstore/store.go
internal/gitstore/store_test.go
internal/api/handler.go
internal/api/handler_test.go
internal/app/app.go
web/src/api.ts
web/src/App.tsx
web/src/App.test.tsx
web/src/outline/OutlineWorkbench.tsx
web/src/outline/OutlineWorkbench.test.tsx
```

Small additional files are acceptable when they keep one responsibility clear.
Do not create generic `utils`, `common`, or `manager` packages.

## Required tests, in implementation order

Write each group first, run it to observe the expected failure, then implement the
smallest passing behavior. Do not write every test and all code in one pass.

### 1. Pure story model tests

Create table-driven tests for:

- appending arcs, chapters, and scenes,
- rejecting unknown parents,
- exact-permutation reorder validation,
- preserving IDs during reorder,
- deriving labels before and after reorder,
- title and ID validation.

These tests must not touch files, Git, SQLite, or HTTP.

### 2. Story file adapter tests

Use `t.TempDir()` fixtures for:

- empty outline round trip,
- populated outline and entity round trip,
- deterministic output,
- scene front matter plus empty body,
- malformed YAML,
- unknown YAML fields,
- missing referenced file,
- duplicate ID,
- parent mismatch,
- atomic replacement failure leaving the old file intact.

### 3. Workspace tests

Test empty, set/current, replacement, and concurrent reads under `go test -race`.

### 4. Story application service tests

Use fake session, ID generator, Git, index, and persistence boundaries. Verify:

- each create writes the entity and outline then rebuilds and commits once,
- reorder writes only the outline then rebuilds and commits once,
- dirty Git state rejects before file writes,
- write/index/commit failures return errors,
- commit failure restores files and unstages,
- unsuccessful requests create no commit.

### 5. Real Git adapter tests

Use `t.TempDir()` and the Git CLI. Verify clean detection, commit count/message,
and reset/unstage behavior. Do not modify global Git configuration.

### 6. API tests

Test every route with fakes:

- success status and exact JSON shape,
- active project set by both create and open,
- 409 without an active project,
- 400 malformed/unknown/trailing JSON,
- 400 invalid reorder,
- 404 unknown well-formed parent,
- 409 dirty repository,
- 500 adapter failure.

Use typed/sentinel domain errors and `errors.Is` for status mapping. Do not map all
service failures to 400.

### 7. Frontend tests

With Vitest and Testing Library verify:

- empty and nested outline rendering,
- create controls send parent stable IDs,
- Move up / Move down sends the complete ordered stable-ID list,
- labels update from the returned outline,
- controls disable during a mutation,
- API errors are visible and retryable.

Do not assert implementation-only CSS classes.

### 8. End-to-end adapter acceptance test

Using a temp story project and real filesystem, Git, and SQLite adapters:

1. create/open the project,
2. add two arcs, two chapters, and two scenes,
3. reorder chapters and scenes,
4. reload from disk,
5. verify stable IDs and order,
6. verify canonical files exist,
7. verify Git commit count increased by one per mutation,
8. verify the Git worktree is clean,
9. verify the index manifest includes the entity files.

This can be a Go integration test; it does not require a browser or live server.

## Acceptance commands

Run from the repository root with the Homebrew Go binary available on `PATH`:

```bash
go fmt ./...
go vet ./...
go test ./...
go test -race ./...
make check
git diff --check
```

Also perform one manual UI smoke test for create and reorder. Do not leave a
backend or frontend process running after the check.

## Definition of done

Milestone 1 is done only when:

- all BDD stories above are implemented,
- all required automated tests pass,
- the UI can create and reorder a simple hierarchy,
- canonical files match the documented schemas,
- every successful mutation has one Git checkpoint,
- failed mutations do not leak canonical changes,
- the derived index reflects the current files,
- docs describe any intentional behavior change,
- no Milestone 2 feature or generated artifact is committed.
