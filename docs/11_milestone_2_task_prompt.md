# 11 — Milestone 2 Coding Agent Task Prompt

Use this file as the implementation contract for **Milestone 2 — Vim-friendly
scene editor**. Read every document listed before this one in `README.md`,
including the local Go rules, before changing code.

Milestone 1 is complete. Preserve its behavior and tests. Do not implement later
milestones except for narrow interfaces needed to keep this milestone testable.

## Outcome

An author can select an existing scene from the outline, load its canonical
Markdown into a CodeMirror 6 editor with Vim keybindings enabled by default,
edit its prose and supported metadata, explicitly save it, and reload the page
without losing the accepted changes.

Each successful explicit save is an atomic canonical mutation with exactly one
Git checkpoint. Stale browser state, dirty worktrees, invalid input, and adapter
failures must not silently overwrite or partially mutate canon.

## Scope boundaries

Implement only:

1. loading one existing scene by stable ID,
2. editing `title`, `pov`, `status`, `exclude_from_ai`, and Markdown prose,
3. explicit save with revision conflict protection,
4. one Git checkpoint per successful save,
5. CodeMirror 6 with Vim mode on by default,
6. editor loading, clean, dirty, saving, saved, conflict, and error states,
7. navigation from the outline to the editor and back.

Do not implement:

- autosave or checkpoint batching,
- creating, deleting, moving, or reordering scenes from the editor,
- editing `id` or `chapter_id`,
- arbitrary YAML front-matter fields,
- AI actions, selections, patches, providers, agents, or styles,
- Codex entries or timeline progressions,
- multiple open scene tabs,
- collaborative editing,
- filesystem watching or automatic external-change merging,
- configurable Vim keybindings beyond the default-on mode,
- a rich-text or WYSIWYG editor.

## Requirement catalog

Every BDD scenario and automated test must cite one or more requirement IDs from
this catalog. Do not refer only to “Milestone 2” in new test comments.

| ID | Requirement |
| --- | --- |
| M2-R01 | Load an existing scene from the active project by a validated stable scene ID. |
| M2-R02 | Return only the supported canonical metadata, Markdown body, and a deterministic content revision. |
| M2-R03 | Render the selected scene in CodeMirror 6 with Vim keybindings enabled by default. |
| M2-R04 | Allow editing of title, POV, status, AI-exclusion flag, and Markdown prose while keeping ID and chapter ID immutable. |
| M2-R05 | Track and visibly report clean, dirty, saving, saved, conflict, and request-error editor states. |
| M2-R06 | Save only when `expected_revision` matches the current canonical scene revision. |
| M2-R07 | Serialize scene saves with all structural mutations so reads never observe a partial canonical mutation. |
| M2-R08 | Validate all route IDs, request JSON, metadata, and body limits before writing. |
| M2-R09 | Write the complete scene file atomically and preserve the exact documented canonical schema. |
| M2-R10 | Rebuild the disposable index after the canonical write and before checkpointing. |
| M2-R11 | Create exactly one Git commit for each successful explicit save and none for failed or no-op saves. |
| M2-R12 | Reject saves when the story-project Git worktree is dirty. |
| M2-R13 | Restore canonical files, unstage app changes, and rebuild the index if index or checkpoint work fails. |
| M2-R14 | Prevent stale editor state from overwriting a newer canonical scene. |
| M2-R15 | Navigate from an outline scene to the editor and back without changing the active project. |
| M2-R16 | Preserve all Milestone 0 and Milestone 1 behavior and keep `make check` green. |
| M2-R17 | Every Milestone 2 test file and test case provides requirement, BDD scenario, and plain-English test-purpose traceability. |

## Required BDD stories

### Story 2.1 — Open a scene

As an author, I want to open a scene from my outline so that I can edit its
canonical prose and metadata.

#### Scenario 2.1.1 — Load a valid scene

Requirements: M2-R01, M2-R02, M2-R03, M2-R15.

```gherkin
Given an active project with a scene referenced by outline.yaml
When I select that scene in the outline
Then the app loads the scene by its stable ID
And shows its title, POV, status, AI-exclusion value, and Markdown body
And opens the Markdown body in CodeMirror with Vim keybindings enabled
And shows the editor as clean
```

#### Scenario 2.1.2 — Missing active project

Requirements: M2-R01, M2-R08.

```gherkin
Given no project is active in this backend process
When I request a scene
Then I receive 409 Conflict with a useful error
```

#### Scenario 2.1.3 — Invalid or unknown scene

Requirements: M2-R01, M2-R08.

```gherkin
Given a project is active
When I request a malformed scene ID
Then I receive 400 Bad Request

Given a project is active
When I request a well-formed scene ID that is not in the outline
Then I receive 404 Not Found
```

#### Scenario 2.1.4 — Malformed canonical scene

Requirements: M2-R01, M2-R02, M2-R09.

```gherkin
Given a referenced scene file has malformed, unsupported, or inconsistent front matter
When I request that scene
Then I receive 500 Internal Server Error with a contextual error
And the app does not silently repair the file
```

### Story 2.2 — Edit and save a scene

As an author, I want to explicitly save scene edits so that accepted prose and
metadata become durable canonical state.

#### Scenario 2.2.1 — Save valid edits

Requirements: M2-R04, M2-R06, M2-R07, M2-R09, M2-R10, M2-R11.

```gherkin
Given I loaded a scene at its current revision
And the project worktree is clean
When I edit supported metadata or Markdown and choose Save
Then the complete scene file is atomically replaced
And immutable ID and chapter ID values are preserved
And the disposable index is rebuilt
And exactly one Git commit is created
And the response contains the saved scene and its new revision
And the editor becomes clean and shows Saved
```

#### Scenario 2.2.2 — Reload saved content

Requirements: M2-R01, M2-R02, M2-R11.

```gherkin
Given I successfully saved scene changes
When I reload the scene from canonical storage
Then I receive the saved metadata and Markdown
And the returned revision matches the saved canonical bytes
And the Git worktree is clean
```

#### Scenario 2.2.3 — No-op save

Requirements: M2-R06, M2-R11.

```gherkin
Given I loaded a scene and made no changes
When I attempt to save
Then the UI does not issue a save request
And no file, index, or Git history changes
```

### Story 2.3 — Protect concurrent and external work

As an author, I want saves to detect stale or uncommitted state so that one edit
cannot silently destroy another.

#### Scenario 2.3.1 — Stale revision

Requirements: M2-R06, M2-R14.

```gherkin
Given I loaded a scene at revision A
And the canonical scene later changed to revision B
When I save using expected revision A
Then I receive 409 Conflict with a useful stale-revision error
And revision B remains unchanged
And the index and Git history remain unchanged
And the editor retains my unsaved draft and shows Conflict
```

#### Scenario 2.3.2 — Dirty project

Requirements: M2-R12.

```gherkin
Given the active story project has uncommitted changes
When I save a scene
Then I receive 409 Conflict
And no canonical file, index, staging state, or Git history is changed
And the editor retains my unsaved draft
```

#### Scenario 2.3.3 — Overlapping app mutations

Requirements: M2-R07, M2-R14.

```gherkin
Given a scene save or structural mutation is in progress
When another scene save or structural mutation begins
Then the second mutation waits for the first mutation to finish
And it validates cleanliness and revision against the resulting canonical state
And it cannot overwrite the first mutation from an older in-memory read
```

### Story 2.4 — Preserve checkpoint integrity

As an author, I want failed saves to leave canon recoverable so that a technical
failure never masquerades as an accepted edit.

#### Scenario 2.4.1 — Atomic write failure

Requirements: M2-R09, M2-R13.

```gherkin
Given a clean active project
When atomic scene replacement fails
Then the save fails
And the original scene file remains intact
And no index rebuild or Git commit occurs
```

#### Scenario 2.4.2 — Index rebuild failure

Requirements: M2-R10, M2-R13.

```gherkin
Given the scene file was replaced
When index rebuild fails
Then the original scene file is restored
And app-staged changes are removed
And the index is rebuilt from restored canonical files
And no Git commit is created
```

#### Scenario 2.4.3 — Git checkpoint failure

Requirements: M2-R11, M2-R13.

```gherkin
Given the scene file and index were updated
When Git checkpoint creation fails
Then the original scene file is restored
And app-staged changes are removed
And the index is rebuilt from restored canonical files
And the save reports failure rather than success
```

### Story 2.5 — Keep the editor understandable

As an author, I want visible save state and safe navigation so that I know what
is canonical and do not accidentally abandon a draft.

#### Scenario 2.5.1 — Editor state transitions

Requirements: M2-R05.

```gherkin
Given a loaded clean scene
When I modify a supported field
Then the editor shows Unsaved changes and enables Save
When I choose Save
Then duplicate submissions are disabled and the editor shows Saving
When the save succeeds
Then the editor shows Saved and disables Save until another edit
```

#### Scenario 2.5.2 — Failed save retains draft

Requirements: M2-R05, M2-R14.

```gherkin
Given I have unsaved scene edits
When saving fails or conflicts
Then my edited form and prose remain visible
And the error is actionable
And I can retry after resolving the cause
```

#### Scenario 2.5.3 — Navigate away with unsaved changes

Requirements: M2-R05, M2-R15.

```gherkin
Given the editor has unsaved changes
When I choose Back to outline or select another scene
Then the app asks me to confirm discarding the unsaved draft
And Cancel keeps the current draft open
And Confirm discards the draft and completes navigation
```

## Fixed canonical scene contract

The canonical path is `scenes/<scene-id>.md`. Validate the ID before constructing
the path. The file has strict YAML front matter followed by Markdown:

```markdown
---
id: scn_0123456789abcdef0123
title: The Duel
chapter_id: ch_0123456789abcdef0123
pov: Luke
status: draft
exclude_from_ai: false
---

Scene prose starts here.
```

Rules:

- Front matter has exactly the six fields shown above. Unknown or duplicate YAML
  fields are errors. Do not preserve unsupported fields silently.
- `id` and `chapter_id` are immutable during save and come from the current
  canonical scene, never from the request body.
- `title` is trimmed, non-empty, and at most 200 Unicode code points, reusing the
  Milestone 1 title validation.
- `pov` is trimmed and at most 200 Unicode code points. It may be empty.
- `status` is one of `draft`, `revised`, or `final`.
- `exclude_from_ai` is a Boolean.
- `markdown` is UTF-8 text and may be empty. Reject invalid UTF-8 and NUL bytes.
- Limit the Markdown body to 5 MiB in UTF-8 bytes. Keep the HTTP request limit
  slightly larger at 6 MiB to allow JSON and metadata overhead.
- Normalize line endings in accepted request text from CRLF/CR to LF.
- Canonical serialization uses the field order shown, two-space YAML indentation,
  `---` delimiters, one blank line before the Markdown body, and exactly one
  terminal newline for the complete file.
- Do not trim leading/trailing Markdown whitespace except line-ending
  normalization and terminal-newline canonicalization.
- Empty prose serializes as `---\n\n` after the closing front-matter delimiter.

The revision is `sha256:` plus the lowercase hexadecimal SHA-256 digest of the
exact canonical file bytes. Compute it from bytes loaded from disk and from the
final serialized bytes after save. It is an opaque concurrency token in the UI.

## Fixed save and concurrency decisions

An explicit Save creates exactly one commit. Do not add a `checkpoint` Boolean,
autosave, debounce, or batch policy.

Use the existing Milestone 1 mutation lock for both structural mutations and
scene saves. A separate lock in a separate service is insufficient because it
would permit a scene save and reorder to overlap. Prefer extending the existing
`story.Service` with scene load/save methods or inject one shared mutation lock
into both services. The tests must prove cross-operation serialization.

Save sequence:

1. Resolve the active project.
2. Validate route ID and request values.
3. Acquire the shared mutation lock.
4. Verify the Git worktree is clean.
5. Load and strictly validate the current outline and referenced scene.
6. Compute current revision and compare it with `expected_revision` using exact
   string equality.
7. Build the next scene in memory while preserving canonical ID and chapter ID.
8. Serialize the complete next scene and detect a no-op byte comparison.
9. Snapshot the scene file.
10. Atomically replace the scene file.
11. Rebuild the disposable index.
12. Commit all project changes with `Edit scene <scene-id>`.
13. Return the exact canonical scene representation written in step 10 with its
    new revision. Do not perform fallible work after the commit succeeds.

Check the dirty worktree before checking revision. Both map to conflict, but a
dirty worktree may include author work outside this scene and must stop all app
mutation immediately.

If steps 10 through 12 fail, use the existing snapshot rollback pattern: restore
the original scene, unstage app-staged changes, and rebuild the index from the
restored canonical files. Join rollback errors to the original error. Never
return success if checkpoint creation failed.

No-op service requests are rejected as a typed invalid/no-change error and do
not rebuild or commit. The frontend should prevent the normal no-op path by
disabling Save while clean.

## Exact HTTP API

Routes:

```http
GET /api/scenes/{scene_id}
PUT /api/scenes/{scene_id}
```

Load response and successful save response (`200 OK`):

```json
{
  "id": "scn_0123456789abcdef0123",
  "chapter_id": "ch_0123456789abcdef0123",
  "title": "The Duel",
  "frontmatter": {
    "pov": "Luke",
    "status": "draft",
    "exclude_from_ai": false
  },
  "markdown": "Scene prose starts here.\n",
  "revision": "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
}
```

Save request:

```json
{
  "title": "The Duel",
  "frontmatter": {
    "pov": "Luke",
    "status": "revised",
    "exclude_from_ai": false
  },
  "markdown": "Revised scene prose.\n",
  "expected_revision": "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
}
```

JSON rules:

- Reject unknown fields at every object level.
- Reject missing required fields, including an omitted `frontmatter` object.
- Reject trailing JSON values.
- Apply a 6 MiB HTTP body limit to scene saves; retain the existing smaller
  limit for other routes by making the decoder limit configurable.
- The route ID is authoritative. Save JSON contains no `id` or `chapter_id`.

Status mapping:

- `400 Bad Request`: malformed/trailing/unknown JSON, malformed scene ID,
  invalid metadata, invalid revision shape, invalid UTF-8/NUL, oversized prose,
  or byte-identical no-op request.
- `404 Not Found`: well-formed scene ID is not referenced by the current outline.
- `409 Conflict`: no active project, dirty Git worktree, or stale revision.
- `413 Content Too Large`: HTTP request exceeds 6 MiB.
- `500 Internal Server Error`: malformed canonical storage, filesystem/index/Git
  adapter failure, or rollback failure.

All errors use `{"error":"useful message"}`. Do not expose absolute project
paths or raw scene contents in errors. Use typed/sentinel errors and `errors.Is`
for status mapping.

## Frontend contract

Add CodeMirror 6 packages and the maintained Vim keymap package. Do not simulate
an editor with a textarea. Keep API types and calls in `web/src/api.ts`.

Suggested components:

```text
web/src/editor/SceneEditor.tsx
web/src/editor/SceneEditor.test.tsx
web/src/editor/editor_state.ts       optional pure state helpers
web/src/editor/editor_state.test.ts  only if helpers are extracted
```

The outline scene row gets an `Open scene` action identified by stable scene ID.
The root app may own only navigation state (`outline` or selected scene ID); it
must not absorb editor implementation details.

Editor requirements:

- Show project path, immutable scene ID, and immutable chapter ID.
- Use ordinary accessible controls for title, POV, status, and AI exclusion.
- `status` is a select with Draft, Revised, and Final.
- CodeMirror holds Markdown prose and starts with Vim mode enabled.
- Show a visible “Vim mode” indicator. No toggle is required in this milestone.
- Save is disabled while clean, invalid, or saving.
- Any supported field change marks the editor dirty.
- A successful response replaces the baseline and revision, then marks clean.
- A failed response never replaces current form/editor values.
- Conflict UI states that canonical content changed and offers `Reload canonical`.
  Reload requires discard confirmation because it replaces the local draft.
- `Retry save` retries the current draft only after the underlying condition has
  been resolved; it still sends the existing expected revision.
- Back/scene-switch navigation uses a confirmation dialog when dirty.
- Browser reload/close while dirty registers `beforeunload`; remove the listener
  while clean and on unmount.
- The UI must remain keyboard operable. CodeMirror Vim behavior does not replace
  accessible buttons and form labels.

Do not re-test the whole CodeMirror Vim package. Automated tests should verify
that the editor is configured with the Vim extension and that one narrow
real-extension regression still accepts a normal-mode Vim command.

## Package responsibilities

Prefer extending existing packages rather than duplicating Milestone 1 logic:

```text
internal/story      scene DTO/validation, revision decision, load/save service,
                    shared mutation serialization and rollback orchestration
internal/storyfile  strict scene parse/serialize, revision bytes, atomic writes
internal/gitstore   existing clean/commit/unstage boundary; no scene policy
internal/index      existing rebuild behavior; no canonical scene authority
internal/api        route parsing, strict JSON, body limits, status mapping
internal/app        production wiring only
web/src/api.ts      scene request/response types and transport
web/src/editor      CodeMirror editor and editor interaction state
web/src/outline     open-scene action only; no editor logic
```

Do not create `utils`, `common`, `manager`, or an HTTP-specific scene domain.
Provider/model interfaces do not belong in this milestone.

## Mandatory test traceability format

M2-R17 is part of definition of done, not optional documentation cleanup.

Each Milestone 2 test file must begin, after its package/import section where
language syntax requires, with a file-level trace block naming exactly one
primary BDD scenario. Tests in that file may cover multiple requirements only
when they all support that same scenario. Split files when scenarios differ.

Go example:

```go
// BDD trace:
//   - Story: 2.3, Protect concurrent and external work.
//   - Scenario: 2.3.1, stale revision leaves newer canon unchanged.
//   - Requirements: M2-R06, M2-R14.
//   - File purpose: exercise revision-conflict decisions at the service boundary.
```

Immediately above every test function, add:

```go
// Test purpose: given revision A and canonical revision B, saving A returns the
// conflict sentinel without calling write, index, or Git adapters.
func TestSaveSceneRejectsStaleRevision(t *testing.T) { ... }
```

TypeScript example:

```ts
// BDD trace:
// - Story: 2.5, Keep the editor understandable.
// - Scenario: 2.5.1, editor state transitions.
// - Requirements: M2-R05.
// - File purpose: verify visible dirty/saving/saved UI behavior.

// Test purpose: editing prose enables Save; an in-flight request disables it;
// the successful response establishes the new clean baseline.
test('moves through dirty, saving, and saved states', async () => { ... })
```

Table-driven subtests must state the English behavior in each case name and may
include a `purpose` field if the name alone is not sufficient. A generic test
name such as `errors` or `validation` is not traceable.

Do not place tests for multiple BDD scenarios in one file merely because they
exercise the same production function. Traceability takes priority over reducing
the test-file count.

## TDD implementation sequence

Follow red, green, refactor for each numbered group. Commit-ready code must not
contain skipped tests, `TODO` assertions, or tests that passed before the behavior
was introduced. Do not write all production code and backfill tests afterward.

### 1. Scene parsing and serialization

Write adapter tests first for Scenario 2.1.4 and Scenario 2.2.1:

- parse the exact existing canonical scene,
- preserve Markdown whitespace and normalize line endings as specified,
- deterministic serialization and terminal newline,
- empty Markdown body,
- strict unknown and duplicate front-matter fields,
- missing delimiter and malformed YAML,
- invalid ID/chapter ID/title/POV/status,
- invalid UTF-8 and NUL body,
- body size boundary at 5 MiB,
- exact revision digest over canonical bytes,
- atomic replacement failure leaves old bytes intact.

### 2. Pure scene validation and revision decisions

Write pure tests for Scenarios 2.2.1, 2.2.3, and 2.3.1:

- supported metadata boundaries,
- route ID validation,
- expected revision shape,
- matching and stale revision decisions,
- immutable ID/chapter ID preservation,
- byte-identical no-op detection.

These tests must not use files, Git, SQLite, HTTP, or React.

### 3. Scene load service

Write service tests for Scenarios 2.1.1 through 2.1.4:

- no active project,
- valid scene load and revision,
- malformed and unknown IDs,
- scene must be referenced by the loaded outline,
- malformed canonical storage remains an internal error.

### 4. Scene save orchestration

Use fake session, file, Git, and index boundaries. Write separate scenario files
for:

- successful save call order: clean, load, write, rebuild, commit,
- exact commit message and exactly one commit,
- stale revision calls no write/index/Git mutation,
- dirty worktree stops before canonical loading/writing,
- no-op calls no write/index/Git mutation,
- write failure does not rebuild or commit,
- index failure restores, unstages, and rebuilds restored state,
- commit failure restores, unstages, and rebuilds restored state,
- rollback error is joined and remains inspectable.

### 5. Cross-operation concurrency

Write a deterministic channel-controlled test for Scenario 2.3.3. Prove a scene
save blocks both another save and a Milestone 1 reorder/create operation until
the first mutation releases the shared lock. Do not rely on timing alone except
for a short assertion timeout protecting the test from deadlock. Run under the
race detector.

### 6. HTTP API

Use handler fakes and test exact JSON shapes and status mapping:

- load and save success,
- stable route ID reaches the service,
- unknown/trailing/missing nested JSON fields,
- normal route body limit remains 1 MiB,
- scene-save body limit is 6 MiB and overflow returns 413,
- all 400/404/409/500 mappings listed above,
- response does not expose project filesystem paths.

### 7. Frontend API client

Test request method, route, exact snake_case body, response mapping, and non-2xx
error propagation. A stale conflict must be distinguishable by HTTP status, so
extend the API error type rather than reducing every error to a message string.

### 8. Editor component and navigation

Use Vitest and Testing Library for Scenarios 2.1.1 and 2.5.1 through 2.5.3:

- outline open action passes the scene stable ID,
- loading and load-error/retry states,
- metadata and Markdown rendering,
- CodeMirror configured with Vim extension,
- all supported changes mark dirty,
- no-op Save disabled,
- exact current draft sent with expected revision,
- duplicate submission disabled while saving,
- success installs new revision and clean baseline,
- failure/conflict retains the current draft,
- reload canonical confirmation,
- back and scene-switch discard confirmation,
- `beforeunload` only while dirty.

Do not assert CodeMirror internals, generated DOM structure, or implementation-only
CSS classes.

### 9. Real-adapter acceptance test

Using a temporary story project and real filesystem, Git, and SQLite adapters:

1. create/open a project,
2. create an arc, chapter, and scene using Milestone 1 services,
3. load the empty scene and retain revision A,
4. save title, metadata, and multiline Markdown,
5. verify exactly one new commit named `Edit scene <scene-id>`,
6. reload and compare every field and revision B,
7. verify revision B differs from A,
8. verify canonical scene bytes match the documented format,
9. verify the Git worktree is clean,
10. verify the index manifest hash matches the saved scene,
11. attempt a save with revision A and verify no state changes.

## Expected source changes

This is a navigation checklist, not a mandate to put unrelated code in one file:

```text
docs/11_milestone_2_task_prompt.md
internal/story/scene.go
internal/story/scene_*_test.go
internal/story/service.go
internal/storyfile/store.go
internal/storyfile/scene_*_test.go
internal/api/handler.go
internal/api/scene_handler_test.go
internal/app/app.go
web/package.json
web/package-lock.json
web/src/api.ts
web/src/App.tsx
web/src/App.test.tsx
web/src/outline/OutlineWorkbench.tsx
web/src/editor/SceneEditor.tsx
web/src/editor/SceneEditor.test.tsx
web/src/styles.css
```

Small additional files are expected when needed to keep each test file bound to
one BDD scenario.

## Acceptance commands

Run from the repository root with Homebrew Go available on `PATH`:

```bash
go fmt ./...
go vet ./...
go test ./...
go test -race ./...
make check
git diff --check
```

Manual acceptance:

1. Start the backend on port `9090` because port `8080` is occupied in the
   current development environment; configure Vite’s proxy consistently.
2. Start the frontend and create a temporary project.
3. Create an arc, chapter, and scene.
4. Open the scene, enter CodeMirror insert mode with `i`, type multiline prose,
   press Escape, and use at least one normal-mode motion.
5. Edit every supported metadata field and save.
6. Reload the browser and verify all saved values.
7. Confirm one `Edit scene <scene-id>` commit exists for that save.
8. Make another edit and verify unsaved-navigation confirmation.
9. Stop both processes and remove only the temporary story project created by
   the acceptance run.

Do not leave background processes running.

## Definition of done

Milestone 2 is done only when:

- every requirement M2-R01 through M2-R17 is implemented or directly verified,
- every BDD scenario above has automated coverage, including a narrow real Vim
  command regression test for the editor extension,
- all new test files and test cases follow the traceability format,
- scene saves preserve immutable identity and never silently overwrite stale or
  dirty canonical state,
- every successful non-no-op save creates exactly one checkpoint,
- every failed save leaves canonical files, index, staging, and Git history in
  the documented state,
- the editor retains drafts across request failures and clearly reports state,
- all Milestone 0 and Milestone 1 tests still pass,
- all acceptance commands pass,
- no Milestone 3 behavior, generated build output, credentials, database, test
  project, or `node_modules` is committed.
