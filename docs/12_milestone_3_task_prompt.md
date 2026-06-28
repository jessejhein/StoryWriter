# Milestone 3 Task Prompt - Codex and Timeline Progressions

Implement only Milestone 3. Milestones 0 through 2 are complete and are
regression constraints. Do not add AI agents, provider adapters, import,
embeddings/RAG, or what-if branches.

This document is the durable implementation contract. When a general project
document is less specific, this document controls Milestone 3 behavior. Do not
invent alternate schemas, routes, merge rules, or checkpoint semantics during
implementation.

## Outcome

An author can create and edit canonical Codex entries, attach ordered timeline
progressions to an entry using stable scene anchors, and inspect that entry's
active state as of any scene. Reordering scenes changes chronology but does not
change the stable anchor stored in the progression.

## Lessons carried forward from Milestone 2

Milestone 2 required follow-up work because important behavior was tested too
indirectly or left for implementation to infer. Milestone 3 therefore fixes:

- exact canonical schemas and normalization rules,
- exact HTTP request, response, and error contracts,
- typed domain errors and status mapping,
- mutation ordering, shared locking, checkpoint, and rollback behavior,
- direct API-client tests rather than component mocks alone,
- automated real-adapter acceptance rather than a manual-only smoke test,
- explicit requirement/BDD/test traceability in every test file and test case,
- a requirement audit before the milestone can be called complete.

Passing a nearby test is not evidence for an untested requirement. Tests must
assert observable behavior at the lowest useful layer and at each important
boundary.

## Scope boundaries

In scope:

- character, location, lore, and custom Codex entries,
- create, list, load, and edit entry workflows,
- aliases, tags, description, and string metadata,
- one progression file per Codex entry,
- progression replacement as one explicit author save,
- stable scene anchors with `before` and `after` timing,
- active-state resolution for a requested scene,
- UI workflows for entries, progressions, and active-state inspection,
- canonical files, index rebuilds, and Git checkpoints for mutations.

Out of scope:

- deleting Codex entries,
- chapter or event anchors,
- editing active state directly,
- rich metadata value types or nested metadata,
- AI generation or extraction of entries,
- search, embeddings, RAG, and mention detection,
- progression conflict resolution across Git branches,
- autosave or background mutation,
- Milestone 4 agent/style behavior.

Chapter and event anchors remain future-compatible concepts. Milestone 3 stores
only scene anchors because scenes already provide a complete stable linear
timeline and are the anchor required by the milestone acceptance criteria.

## Requirements

| ID | Requirement |
| --- | --- |
| M3-R01 | List and load strictly validated canonical Codex entries from the active project. |
| M3-R02 | Create a Codex entry of an allowed type with a generated stable type-prefixed ID. |
| M3-R03 | Edit mutable entry fields without changing its ID, type, or canonical path. |
| M3-R04 | Normalize aliases, tags, description, and metadata exactly as specified below. |
| M3-R05 | Store and load one strictly validated ordered progression document per entry. |
| M3-R06 | Validate progression IDs, stable scene anchors, timing, changes, and duplicate anchors. |
| M3-R07 | Resolve active entry state deterministically for any scene in current outline order. |
| M3-R08 | Keep progression anchors stable when the outline is reordered while using the new order for activation. |
| M3-R09 | Expose the exact Codex, progression, and active-state HTTP APIs in this contract. |
| M3-R10 | Provide a usable UI for entry creation/editing, progression editing, and active-state inspection. |
| M3-R11 | Show explicit loading, empty, dirty, saving, saved, conflict, and actionable error states. |
| M3-R12 | Prevent dirty-form navigation from silently discarding entry or progression edits. |
| M3-R13 | Serialize all canonical mutations with the existing shared story mutation lock. |
| M3-R14 | Reject app mutations when the story-project Git worktree is dirty. |
| M3-R15 | Atomically replace canonical files, rebuild the disposable index, and create exactly one Git commit per successful mutation. |
| M3-R16 | On write, index, or checkpoint failure, restore canonical bytes, unstage app changes, and rebuild the index from restored files. |
| M3-R17 | Reject stale entry and progression revisions without overwriting newer canonical state. |
| M3-R18 | Reject malformed canonical files with contextual errors and never silently repair them. |
| M3-R19 | Preserve all Milestone 0-2 behavior and keep the full check and race suites green. |
| M3-R20 | Every Milestone 3 test file and test case traces to one BDD scenario, its requirements, and an English test purpose. |
| M3-R21 | Automate the real filesystem, real Git, real index, service, HTTP, and UI acceptance paths; manual testing is supplementary only. |

## BDD stories

### Story 3.1 - Manage canonical Codex entries

As an author, I want to create and edit Codex entries so that story facts remain
portable, readable, and under my control.

#### Scenario 3.1.1 - List an empty Codex

Requirements: M3-R01, M3-R09, M3-R10, M3-R11.

```gherkin
Given an active project has no Codex entries
When I open the Codex workbench
Then the API returns an empty list
And the UI shows a clear empty state with a create action
```

#### Scenario 3.1.2 - Create an entry

Requirements: M3-R02, M3-R04, M3-R13, M3-R14, M3-R15.

```gherkin
Given a clean active project
When I create a character entry with a name, aliases, tags, description, and metadata
Then a strictly canonical character file is written with a new stable character ID
And the disposable index is rebuilt
And exactly one Git commit named "Create Codex entry <entry-id>" is created
And the worktree is clean
And the saved entry and revision are returned
```

Repeat this scenario for location, lore, and custom types at the domain and
storage layers. One HTTP/UI flow may use character as the representative type.

#### Scenario 3.1.3 - Edit an entry

Requirements: M3-R03, M3-R04, M3-R15, M3-R17.

```gherkin
Given I loaded an entry at its current revision
When I change its mutable fields and explicitly save
Then the same canonical file is atomically replaced
And its ID and type are unchanged
And exactly one Git commit named "Edit Codex entry <entry-id>" is created
And the response contains the normalized saved state and its new revision
```

#### Scenario 3.1.4 - Reject invalid entry data

Requirements: M3-R02, M3-R03, M3-R04, M3-R09.

```gherkin
Given an active project
When I create or edit an entry with an invalid type, name, alias, tag, description, metadata key, or metadata value
Then the request returns 400 Bad Request with a useful error
And no canonical file, index, staging state, or Git history changes
```

#### Scenario 3.1.5 - Reject missing or malformed canonical entries

Requirements: M3-R01, M3-R09, M3-R18.

```gherkin
Given a project contains a malformed Codex file
When the app lists or loads that entry
Then the request returns 500 Internal Server Error with the file path and cause
And the file is not repaired or omitted from the list

Given an entry ID is well formed but has no canonical file
When I load it
Then the request returns 404 Not Found
```

### Story 3.2 - Define timeline progressions

As an author, I want to record changes to a Codex entry at stable story points so
that the app can distinguish earlier facts from later facts.

#### Scenario 3.2.1 - Save ordered progressions

Requirements: M3-R05, M3-R06, M3-R13, M3-R14, M3-R15, M3-R17.

```gherkin
Given a clean active project with an entry and referenced scenes
When I save ordered progressions anchored before or after those scenes
Then one canonical progression document is atomically written for the entry
And exactly one Git commit named "Edit progressions <entry-id>" is created
And the response preserves progression IDs and submitted order
And the response includes a revision of the progression document
```

The first save creates the progression document. Later saves replace its full
ordered `progressions` list. This is not a patch endpoint.

#### Scenario 3.2.2 - Reject invalid progressions

Requirements: M3-R05, M3-R06, M3-R09.

```gherkin
Given an active project with an entry
When I save a progression with a malformed or duplicate progression ID,
  an unknown scene anchor, invalid timing, no effective changes,
  duplicate entry/anchor/timing, or an entry ID mismatch
Then the request returns 400 Bad Request
And no canonical file, index, staging state, or Git history changes
```

#### Scenario 3.2.3 - Report malformed canonical progressions

Requirements: M3-R05, M3-R18.

```gherkin
Given a canonical progression document is malformed, unsupported, or inconsistent
When I load progressions or resolve active state
Then the request fails with a contextual internal error
And the app does not silently repair, reorder, or skip the progression
```

### Story 3.3 - Resolve active Codex state

As an author, I want to view a Codex entry as of a selected scene so that I see
only facts active at that point in the story.

#### Scenario 3.3.1 - Resolve before and after an anchor

Requirements: M3-R07, M3-R09, M3-R10.

```gherkin
Given an entry has a progression timed after scene A
And scene B follows scene A
When I request active state as of scene A
Then the progression is excluded
When I request active state as of scene B
Then the progression is included
```

#### Scenario 3.3.2 - Resolve a before anchor

Requirements: M3-R07.

```gherkin
Given an entry has a progression timed before scene A
When I request active state as of scene A
Then the progression is included
```

#### Scenario 3.3.3 - Apply multiple progressions deterministically

Requirements: M3-R06, M3-R07.

```gherkin
Given several active progressions target one entry
When I resolve the entry at a later scene
Then changes are applied in current outline chronology
And before changes at one anchor apply before after changes at that anchor
And the progression document order breaks ties
And later metadata values replace earlier values
And later description replacements replace earlier descriptions
And aliases and tags remain the canonical entry values
```

#### Scenario 3.3.4 - Reorder around a stable anchor

Requirements: M3-R07, M3-R08.

```gherkin
Given a progression is anchored after scene A and scene B follows scene A
When I reorder scene B before scene A
Then the stored anchor remains scene A
And active state as of scene B no longer includes the progression
And active state as of a later scene still includes it
```

#### Scenario 3.3.5 - Reject an invalid resolution target

Requirements: M3-R07, M3-R09, M3-R18.

```gherkin
Given an active project
When active state is requested for a malformed scene ID
Then the request returns 400 Bad Request
When active state is requested for a well-formed scene ID absent from the outline
Then the request returns 404 Not Found
```

### Story 3.4 - Protect canonical history

As an author, I want failed or stale mutations to preserve existing work so that
the app never presents a partial write as accepted canon.

#### Scenario 3.4.1 - Reject a dirty worktree

Requirements: M3-R14.

```gherkin
Given the active story project has uncommitted changes
When I create or edit an entry or save progressions
Then the request returns 409 Conflict
And no project file, index, staging state, or Git history changes
```

#### Scenario 3.4.2 - Reject a stale revision

Requirements: M3-R17.

```gherkin
Given I loaded an entry or progression document at revision A
And its canonical bytes later changed to revision B
When I save with expected revision A
Then the request returns 409 Conflict
And revision B remains unchanged
And the index and Git history remain unchanged
And the UI retains my unsaved form
```

#### Scenario 3.4.3 - Serialize overlapping mutations

Requirements: M3-R13, M3-R14, M3-R17.

```gherkin
Given any structure, scene, Codex, or progression mutation is in progress
When another canonical mutation begins
Then the second waits for the shared mutation lock
And validates cleanliness and revisions against the resulting canonical state
And cannot overwrite the first mutation from an older read
```

#### Scenario 3.4.4 - Roll back failures

Requirements: M3-R15, M3-R16.

```gherkin
Given a clean active project
When canonical replacement, index rebuild, or Git checkpoint creation fails
Then the request fails
And all canonical files match their pre-request bytes
And app-staged changes are removed
And the index is rebuilt from restored canonical files when necessary
And no Git commit is created
```

Test write failure, index failure, and checkpoint failure separately for both a
new file and an existing file. A new file rollback removes the file; an existing
file rollback restores its exact previous bytes.

### Story 3.5 - Use the Codex workbench safely

As an author, I want clear editing and timeline state in the UI so that I know
what is saved and do not accidentally discard a draft.

#### Scenario 3.5.1 - Edit through explicit UI states

Requirements: M3-R10, M3-R11.

```gherkin
Given the Codex workbench is loaded
When I create or edit an entry or its progressions
Then changed forms show Unsaved changes and enable Save
When I choose Save
Then duplicate submissions are disabled and the form shows Saving
When save succeeds
Then the canonical response replaces local state and the form shows Saved
When save fails
Then my draft remains visible and the error is actionable
```

#### Scenario 3.5.2 - Confirm destructive navigation

Requirements: M3-R11, M3-R12.

```gherkin
Given an entry or progression form has unsaved changes
When I select another entry, switch back to the outline, or leave the page
Then the app asks me to confirm discarding the draft
And Cancel retains the current draft
And Confirm discards it and completes navigation
```

#### Scenario 3.5.3 - Inspect active state

Requirements: M3-R07, M3-R10, M3-R11.

```gherkin
Given a Codex entry and at least two outline scenes
When I select a scene in the active-state inspector
Then the UI requests active state by stable entry and scene IDs
And shows the resolved description and metadata
And identifies which progression IDs were applied
And changing the scene refreshes the resolved state without mutating canon
```

## Canonical Codex contract

Canonical path by type:

```text
codex/characters/<entry-id>.yaml
codex/locations/<entry-id>.yaml
codex/lore/<entry-id>.yaml
codex/custom/<entry-id>.yaml
```

Allowed types and ID prefixes:

| Type | Prefix |
| --- | --- |
| `character` | `char_` |
| `location` | `loc_` |
| `lore` | `lore_` |
| `custom` | `custom_` |

Use the existing random-ID boundary and inject deterministic IDs in tests. The
suffix is exactly 20 lowercase hexadecimal characters, matching existing story
entity IDs.

Exact schema:

```yaml
version: 1
id: char_0123456789abcdef0123
type: character
name: Obi-Wan Kenobi
aliases:
  - Ben
  - Old Ben
tags:
  - jedi
  - mentor
description: A former Jedi acting as Luke's guide.
metadata:
  role: mentor
  status: alive
```

Rules:

- Reject unknown and duplicate YAML fields at every object level.
- `version` is exactly `1`.
- ID, type, directory, and filename must agree.
- `name` is trimmed, non-empty, and at most 200 Unicode code points.
- Each alias is trimmed, non-empty, and at most 200 Unicode code points.
- Aliases are unique by Unicode code-point exact comparison after trimming and
  may not equal the trimmed name. Preserve submitted alias order.
- Each tag is trimmed, non-empty, at most 64 Unicode code points, and matches
  `[a-z0-9][a-z0-9_-]*`. Duplicate tags are rejected. Canonical tags are sorted
  ascending by byte value.
- `description` normalizes CRLF/CR to LF, rejects NUL, is at most 64 KiB in UTF-8
  bytes, and is otherwise not trimmed.
- Metadata is a flat string-to-string map with at most 100 entries. Keys are
  trimmed, non-empty, at most 100 Unicode code points, and unique after trim.
  Values normalize line endings, reject NUL, and are at most 4 KiB each.
- Canonical metadata keys are serialized ascending by byte value.
- Empty aliases, tags, and metadata serialize as `[]`, `[]`, and `{}` rather
  than null or omitted values.
- Canonical YAML uses two-space indentation and exactly one terminal newline.
- The revision is `sha256:` plus lowercase SHA-256 of exact canonical bytes.

Create accepts no ID or revision. Update takes ID from the route, preserves ID
and type from canonical storage, and requires `expected_revision`. A byte-identical
update is a typed no-change error mapped to 400 and creates no side effects.

## Canonical progression contract

Path:

```text
progressions/<entry-id>.yaml
```

Exact schema:

```yaml
version: 1
entry_id: char_0123456789abcdef0123
progressions:
  - id: prog_0123456789abcdef0123
    anchor:
      type: scene
      id: scn_0123456789abcdef0123
      timing: after
    changes:
      description: No longer physically present, but still influential.
      metadata:
        status: deceased
```

Rules:

- Reject unknown and duplicate fields at every level.
- `version` is exactly `1`; `entry_id` must identify an existing Codex entry and
  agree with route and filename.
- Progression IDs use `prog_` plus exactly 20 lowercase hexadecimal characters.
- Existing progression IDs remain stable across edits. A missing ID on a new UI
  row is assigned by the backend; API requests represent new IDs as an omitted
  `id`. A supplied malformed or duplicate ID is rejected.
- `anchor.type` is exactly `scene`; anchor ID must be a scene currently present
  in the strict canonical outline; timing is `before` or `after`.
- Reject two progressions with the same anchor ID and timing for one entry.
- `changes.description`, when present, is a full replacement, not a textual
  diff. It follows entry description validation.
- `changes.metadata`, when present, is a non-empty flat string map following
  entry metadata validation. Values replace keys; Milestone 3 cannot delete a
  metadata key.
- At least one of description or metadata must be present and effective.
- Preserve progression list order and serialize deterministically with two-space
  indentation and exactly one terminal newline.
- An entry with no progressions has no progression file until first save. GET
  returns an empty list and revision `null`. Saving an empty list when no file
  exists is a no-op error. Saving an empty list over an existing file writes a
  canonical empty document; it does not delete the file.
- Non-null revision uses the same exact-byte SHA-256 format as entries.

## Active-state algorithm

1. Strictly load the outline and flatten scenes in arc, chapter, then scene
   order. Every scene appears once.
2. Strictly load the base entry and its progression document, if present.
3. Reject canonical progressions whose scene anchors are not in the outline.
4. For each progression, derive an ordering key of scene index, timing rank
   (`before` = 0, `after` = 1), then progression document index.
5. A `before` progression is active when anchor index is less than or equal to
   target index. An `after` progression is active only when anchor index is less
   than target index.
6. Apply active progressions by the ordering key. Description replacement and
   metadata values use last-active-change-wins semantics.
7. Name, aliases, tags, ID, and type always come from the base entry.
8. Return applied progression IDs in application order. Resolution is read-only
   and never writes files, rebuilds the index, or creates a commit.

Put this algorithm in a pure domain function. Its tests must not require the
filesystem, Git, SQLite, HTTP, or React.

## Mutation transaction contract

All Codex and progression writes use the existing story-wide mutation lock also
used by outline and scene writes. Do not create an independent lock.

Mutation order:

1. Resolve the active project and validate route/request syntax.
2. Acquire the shared mutation lock.
3. verify the Git worktree is clean,
4. strictly reload all canonical state needed for the decision,
5. compare expected revision when updating,
6. validate and normalize the complete next document in memory,
7. detect byte-identical no-op updates,
8. snapshot an existing target or record that the target did not exist,
9. atomically replace the target,
10. rebuild the disposable index,
11. commit all project changes with the exact message specified by the story,
12. return the representation of the exact bytes written.

Dirty-worktree validation happens before revision validation. Do no fallible
work after a successful commit. If steps 9-11 fail, restore or remove the target,
unstage app-staged changes, and rebuild the index from restored files. Join
rollback errors to the initiating error. Never report success after a failed
checkpoint.

## Exact HTTP API

Routes:

```http
GET  /api/codex
POST /api/codex
GET  /api/codex/{entry_id}
PUT  /api/codex/{entry_id}
GET  /api/codex/{entry_id}/progressions
PUT  /api/codex/{entry_id}/progressions
GET  /api/codex/{entry_id}/active?scene_id={scene_id}
```

List response, sorted by type order `character`, `location`, `lore`, `custom`,
then case-sensitive name, then ID:

```json
{"entries":[{"id":"char_0123456789abcdef0123","type":"character","name":"Obi-Wan Kenobi","aliases":["Ben"],"tags":["mentor"],"description":"Guide.","metadata":{"status":"alive"},"revision":"sha256:..."}]}
```

Create request (`POST`) omits ID and revision. Update request (`PUT`) has the
same fields plus `expected_revision`; route ID is authoritative. Type is
required on create and omitted on update.

```json
{"type":"character","name":"Obi-Wan Kenobi","aliases":["Ben"],"tags":["mentor"],"description":"Guide.","metadata":{"status":"alive"}}
```

Single-entry create response is `201 Created`; load/update response is `200 OK`
and uses the entry object shape shown in the list.

Progression response:

```json
{"entry_id":"char_0123456789abcdef0123","progressions":[{"id":"prog_0123456789abcdef0123","anchor":{"type":"scene","id":"scn_0123456789abcdef0123","timing":"after"},"changes":{"description":"Gone, but influential.","metadata":{"status":"deceased"}}}],"revision":"sha256:..."}
```

For no progression file, `progressions` is `[]` and `revision` is `null`. PUT
uses the same body but omits `entry_id`; it adds `expected_revision`, which is a
string for an existing document and null for first creation. New progression
items omit `id`; returned items always have IDs.

Active-state response:

```json
{"scene_id":"scn_0123456789abcdef0123","entry":{"id":"char_0123456789abcdef0123","type":"character","name":"Obi-Wan Kenobi","aliases":["Ben"],"tags":["mentor"],"description":"Gone, but influential.","metadata":{"status":"deceased"}},"applied_progression_ids":["prog_0123456789abcdef0123"]}
```

JSON rules:

- Reject unknown fields at every object level, omitted required fields, trailing
  JSON values, and wrong JSON types.
- Use a 1 MiB HTTP body limit for Codex and progression mutations.
- Return JSON errors in the existing `{"error":"useful message"}` shape.
- Do not expose filesystem paths outside the active project root.

Status mapping:

- `400 Bad Request`: malformed ID or JSON, validation failure, invalid revision
  shape, no-op update, missing/duplicate/foreign anchor, or ineffective change.
- `404 Not Found`: well-formed entry or target scene ID absent from canonical
  state.
- `409 Conflict`: no active project, dirty worktree, or stale revision.
- `500 Internal Server Error`: malformed canonical state, filesystem/index/Git
  failure, or rollback failure.
- `405 Method Not Allowed`: known route with unsupported method; include `Allow`.

## UI contract

Add a top-level Codex workbench reachable from the existing project view without
closing the active project. Preserve the existing outline and editor flows.

Minimum UI:

- entry list grouped or filterable by type,
- explicit New entry action and type selection,
- form controls for every mutable entry field,
- add/remove/reorder controls for aliases and tags,
- key/value controls for metadata,
- progression rows with stable scene selection, before/after timing, optional
  description replacement, and metadata changes,
- active-state scene selector and read-only resolved result,
- applied progression IDs visible in the inspector,
- loading, empty, dirty, saving, saved, conflict, and error states,
- save disabled while clean or saving,
- confirmation before any navigation that discards dirty forms.

Use semantic labels and controls queryable by accessible name. Do not encode
domain behavior only in components; validation and active-state decisions belong
in Go. Frontend validation may improve feedback but the API remains authoritative.

## Bundle-size decision

On 2026-06-28, `npm run build -- --sourcemap` produced one 929.94 kB minified
JavaScript chunk (308.45 kB gzip). Source-map source content was approximately
1.2% first-party code and 98.8% dependencies. The largest sources were React DOM,
CodeMirror, and the Vim extension.

Therefore Milestone 3 does not include a story to split first-party code: the
condition for that story is not met. Do not silence the warning by increasing
Vite's threshold. A later performance milestone may lazy-load the scene editor
or manually separate dependency chunks, with automated chunk assertions. New
Milestone 3 code should still use a separate Codex workbench module and avoid
adding another large dependency.

## Required test architecture and traceability

Use the three TDD rules for every behavior:

1. write only enough failing test to describe the next behavior,
2. write only enough production code to pass it,
3. refactor only while all tests are green.

Every Milestone 3 test file must begin with a comment in its language's normal
comment syntax containing exactly these labels:

```text
BDD Scenario: 3.x.y - scenario title
Requirements: M3-Rxx, M3-Ryy
Test purpose: Plain-English description of the behavior this file proves.
```

One file covers exactly one BDD scenario. If a scenario needs several layers,
use several files and keep the same scenario label. Every test case must have an
adjacent comment with:

```text
Test: Plain-English description of this individual test.
Requirements: M3-Rxx
```

Do not use one broad header to claim unrelated coverage. Table-driven subtests
are allowed only when all rows test the same scenario and requirements; each row
name must state the behavior in English.

Required layers:

- pure domain tests for validation, ordering, activation, and merge rules,
- story-file tests using temporary directories and exact-byte assertions,
- service tests with injected failures at write, index, and Git boundaries,
- race-safe shared-lock tests across old and new mutation types,
- API handler tests for exact JSON and every status class,
- frontend API-client tests exercising real `fetch` response handling,
- frontend component tests for each user-visible state and navigation guard,
- real-adapter integration tests using temporary filesystem, Git repository,
  and SQLite index.

Mocks alone cannot prove canonical serialization, Git cleanliness, rollback, or
frontend HTTP error handling. Snapshot tests alone are insufficient.

## Required automated acceptance test

Create a Go integration test using real filesystem, Git command adapter, and
SQLite index in a temporary directory. It must:

1. create/open a project and create at least three ordered scenes,
2. create a Codex entry and verify exact canonical bytes and one commit,
3. edit the entry and verify stable ID, new revision, and one commit,
4. save before/after progressions and verify exact bytes and one commit,
5. resolve active state at scenes before and after the anchor,
6. reorder scenes and prove the stored anchor is unchanged while activation
   follows new chronology,
7. reload all state from disk through a fresh service instance,
8. verify index manifest coverage, clean worktree, and exact commit count,
9. prove a stale revision and dirty worktree leave files and history unchanged.

Add frontend tests that drive the complete workbench flow through a fake server
boundary or Mock Service Worker-style boundary, not by mocking the workbench's
own API module. A live manual browser check is optional and cannot substitute for
the automated acceptance paths.

## Suggested implementation boundaries

Prefer extending cohesive packages over creating generic helpers:

```text
internal/codex/       pure entry, progression, and active-state rules
internal/storyfile/   strict Codex/progression YAML adapters
internal/story/       mutation orchestration using shared lock and rollback
internal/api/         transport and typed-error mapping
web/src/codex/        workbench components and scenario-focused tests
web/src/api.ts        typed transport functions, or a small cohesive split
```

Do not place filesystem, Git, SQLite, or HTTP concerns in `internal/codex`.
Avoid a generic repository abstraction unless two real implementations require
it. Reuse the existing atomic writer, snapshot rollback, index, Git, workspace,
ID, and mutation-lock boundaries.

## Requirement-to-scenario coverage

| Requirement | BDD scenarios |
| --- | --- |
| M3-R01 | 3.1.1, 3.1.5 |
| M3-R02 | 3.1.2, 3.1.4 |
| M3-R03 | 3.1.3, 3.1.4 |
| M3-R04 | 3.1.2, 3.1.3, 3.1.4 |
| M3-R05 | 3.2.1, 3.2.2, 3.2.3 |
| M3-R06 | 3.2.1, 3.2.2, 3.3.3 |
| M3-R07 | 3.3.1, 3.3.2, 3.3.3, 3.3.4, 3.5.3 |
| M3-R08 | 3.3.4 |
| M3-R09 | 3.1.1, 3.1.4, 3.1.5, 3.2.2, 3.3.1, 3.3.5 |
| M3-R10 | 3.1.1, 3.3.1, 3.5.1, 3.5.3 |
| M3-R11 | 3.1.1, 3.5.1, 3.5.2, 3.5.3 |
| M3-R12 | 3.5.2 |
| M3-R13 | 3.1.2, 3.2.1, 3.4.3 |
| M3-R14 | 3.1.2, 3.2.1, 3.4.1, 3.4.3 |
| M3-R15 | 3.1.2, 3.1.3, 3.2.1, 3.4.4 |
| M3-R16 | 3.4.4 |
| M3-R17 | 3.1.3, 3.2.1, 3.4.2, 3.4.3 |
| M3-R18 | 3.1.5, 3.2.3, 3.3.5 |
| M3-R19 | all scenarios plus regression suite |
| M3-R20 | all Milestone 3 test files and cases |
| M3-R21 | required automated acceptance test and frontend flow tests |

Before declaring completion, add a test-evidence table to the implementation
PR/commit notes or a temporary `.plans/` audit mapping every scenario to actual
test file and test names. Do not commit a stale aspirational test-file list.

## Acceptance commands

Run from repository root:

```bash
go fmt ./...
go vet ./...
go test ./...
go test -race ./...
make check
git diff --check
```

`make check` must include the production frontend build. Generated `web/dist`,
coverage output, databases, story-project fixtures, and `node_modules` must not
be committed.

## Definition of done

Milestone 3 is complete only when:

- every requirement maps to implemented BDD scenarios and passing named tests,
- every test has the required file-level and case-level trace comments,
- canonical schemas, normalization, revisions, and active-state merge rules are
  proven with exact-byte and pure-domain tests,
- all mutation failure points are automatically proven to preserve canon,
- real-adapter acceptance and frontend boundary tests pass without manual steps,
- every successful explicit mutation creates exactly one commit and leaves a
  clean story-project worktree,
- stale and dirty mutations have zero canonical/index/Git side effects,
- the complete Milestone 0-2 regression suite and race suite pass,
- docs reflect any intentional contract adjustment,
- no Milestone 4 feature or generated artifact is committed.
