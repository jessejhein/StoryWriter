# Milestone 6 Task Prompt - Markdown Import and Extraction Review Queue

Implement only Milestone 6. Milestones 0 through 5 are complete and are
regression constraints. Do not add embeddings, semantic search, timeline-aware
RAG, chat, what-if branches, background jobs, filesystem watching, PDF/DOCX
import, or automatic canon mutation.

This document is the durable implementation contract. When a general project
document is less specific, this document controls Milestone 6 behavior. Follow
the ordered red/green/refactor sequence in
`.plans/milestone_6_implementation.md`.

## Outcome

An author can select a local folder containing Markdown notes, import a safe
snapshot into the active story project, inspect deterministic chunks, run a
provider-neutral extraction, and review proposed Codex entries and outline
items. The author can edit, merge, discard, or explicitly accept candidates.
Extraction and review operations never change canon. Each explicit acceptance
uses the existing story mutation boundaries and creates exactly one Git
checkpoint.

The complete flow must work through the HTTP API and the Import Review UI. Tests
must run offline by using deterministic fakes; acceptance must not depend on a
public model endpoint.

## Starting state and design constraints

Milestone 5 provides:

- an active-project session and portable project folders,
- canonical text files with rebuildable SQLite indexing,
- revision-safe outline, scene, and Codex mutation services,
- one Git checkpoint per successful explicit mutation,
- application-level provider profiles and provider-neutral model execution,
- strict HTTP JSON handling and React workbench patterns.

Preserve those guarantees. Import and review are a separate bounded context;
do not add import methods to `internal/storyfile` or extraction state to
`internal/action`.

- `imports/raw/` contains canonical, project-local snapshots of author-selected
  source Markdown. Source paths outside the project are never retained.
- `.storywork/import/` contains rebuildable chunk and extraction-attempt data.
  It is not canonical and is not committed.
- `imports/review/` contains durable, versioned candidate documents and review
  state. Candidates are proposals, never Codex or outline state. Each review
  mutation is checkpointed so the ordinary clean-worktree invariant remains
  usable by every other feature.
- Canonical import/review writes go through one `internal/importer` store
  boundary. Derived chunk writes go through one separate index/cache boundary.
- Candidate acceptance delegates to narrow mutation ports owned by
  `internal/story`; it does not write `codex/`, `arcs/`, `chapters/`,
  `scenes/`, or `outline.yaml` directly.
- Decisions such as path eligibility, chunk boundaries, candidate validation,
  merging, and state transitions are pure and table-testable.
- Candidate-kind behavior is registered behind a small typed handler boundary;
  queue persistence and orchestration must not contain a growing switch that
  knows how every present or future proposal becomes canon.
- Filesystem, model generation, clock, ID generation, story mutation, index,
  and Git behavior are injected boundaries.

## Scope boundaries

In scope:

- recursive folder import of regular `.md` and `.markdown` files,
- a copy-only source policy for Milestone 6,
- deterministic source ordering, collision-free destination paths, and a
  durable import manifest,
- UTF-8 validation, line-ending normalization, limits, and symlink refusal,
- deterministic Markdown-aware chunking with source-line provenance,
- rebuildable chunk metadata under `.storywork/import/`,
- extraction of Codex-entry, arc, chapter, and scene candidates through a
  provider-neutral extraction boundary,
- strict validation of model-produced candidate data before persistence,
- durable candidate queue states and exact-byte revisions,
- author editing, compatible-candidate merging, discarding, and accepting,
- acceptance through existing Codex/outline services with rollback and one
  checkpoint,
- accessible Import Review UI states,
- named test evidence, documentation updates, and status reporting.

Out of scope:

- linked/reference imports, source watchers, incremental sync, or deletion sync,
- importing a single file, archive, URL, clipboard, PDF, DOCX, or binary file,
- following symlinks or importing hidden directories,
- parsing Markdown into a full syntax tree or preserving source formatting
  beyond normalized copied Markdown,
- embeddings, vector databases, similarity retrieval, or semantic deduplication,
- automatic merging based on fuzzy names or model judgment,
- scene-fragment, research-note, alternate-draft, or freeform-idea candidate
  kinds (the schema and dispatch design must permit them later),
- Codex progressions, relationships, metadata inference, scene prose drafting,
- updating existing canonical Codex/outline entities from a candidate,
- accepting several candidates in one transaction,
- streaming, retries, background processing, cancellation UI, or persistent
  prompt/response logs,
- exposing raw provider responses or credentials,
- silently repairing malformed canonical import/review files.

## Requirements

| ID | Requirement |
| --- | --- |
| M6-R01 | Validate an absolute source directory and recursively discover only eligible regular Markdown files without following symlinks. |
| M6-R02 | Copy normalized source snapshots into `imports/raw/<import_id>/` with deterministic relative paths and no retained external path. |
| M6-R03 | Persist a strict, revisioned import manifest atomically and create one Git checkpoint for a successful import batch. |
| M6-R04 | Produce deterministic, bounded, UTF-8-safe chunks with source file and one-based line provenance. |
| M6-R05 | Store chunks and extraction attempts only as rebuildable data under `.storywork/import/`. |
| M6-R06 | Extract typed candidate proposals through a provider-neutral, mode-aware boundary and strictly validate all generated data. |
| M6-R07 | Persist candidates as durable review documents without changing Codex, outline, or scene canon; checkpoint each complete review mutation. |
| M6-R08 | Model candidate edit, merge, discard, accept-claim, accepted, and failed-claim behavior as explicit revision-safe transitions. |
| M6-R09 | Permit merges only for compatible pending candidates and preserve deterministic provenance without silently discarding author data. |
| M6-R10 | Accept a Codex candidate only through the existing Codex create service. |
| M6-R11 | Accept an outline candidate only through an existing or narrowly extended story mutation service with valid parent references. |
| M6-R12 | Make each successful candidate acceptance one atomic logical mutation with rollback, index rebuild, exactly one Git checkpoint, and a generic list of resulting canonical references. |
| M6-R13 | Expose strict import, chunk, extraction, queue, edit, merge, discard, and accept HTTP contracts. |
| M6-R14 | Provide an accessible Import Review UI with loading, empty, importing, extracting, review, dirty, conflict, accepted, discarded, and error states. |
| M6-R15 | Preserve Milestone 0-5 behavior and keep full check, race, and artifact-validation suites green. |
| M6-R16 | Maintain scenario-to-test evidence and complete documentation/status updates before declaring the milestone complete. |

## BDD stories

### Story 6.1 - Import Markdown safely

As an author, I want to snapshot a folder of notes into my story project so
that later extraction is reproducible and does not depend on external files.

#### Scenario 6.1.1 - Import eligible files

Requirements: M6-R01, M6-R02, M6-R03.

```gherkin
Given an active clean project and an absolute source directory
And the directory contains nested .md and .markdown files
When I import the directory
Then eligible files are sorted by normalized relative path and copied under one import ID
And copied text is valid UTF-8 with LF line endings
And a manifest records import-relative paths, byte counts, and SHA-256 digests
And no external absolute source path is persisted or returned
And exactly one Git checkpoint records the import snapshot
```

#### Scenario 6.1.2 - Refuse unsafe or invalid input

Requirements: M6-R01, M6-R02, M6-R03.

```gherkin
Given a missing, relative, project-contained, or unreadable source directory
Or an eligible file is a symlink, invalid UTF-8, oversized, or changes while read
When import is requested
Then the whole import fails without a partial manifest or copied snapshot
And index and Git history remain unchanged
```

#### Scenario 6.1.3 - Ignore non-Markdown content deterministically

Requirements: M6-R01.

```gherkin
Given a source tree contains hidden directories, binary files, and unsupported extensions
When the tree is discovered
Then those entries are not imported
And an empty eligible set returns 400 Bad Request
And ordering is independent of filesystem enumeration order
```

### Story 6.2 - Build inspectable chunks

As an author, I want imported notes split predictably so that I can understand
what material an extraction used.

#### Scenario 6.2.1 - Chunk Markdown deterministically

Requirements: M6-R04, M6-R05.

```gherkin
Given an imported Markdown snapshot
When its chunk index is built
Then chunks prefer heading and blank-line boundaries
And each chunk is at most 8,000 UTF-8 bytes except one indivisible line
And each chunk records source path plus inclusive one-based start and end lines
And the same bytes and settings always produce the same chunk IDs and order
```

#### Scenario 6.2.2 - Rebuild derived chunks

Requirements: M6-R04, M6-R05.

```gherkin
Given derived chunk data is absent or corrupt
When chunks are listed or extraction starts
Then chunks are rebuilt from the canonical imported snapshot
And no canonical file or Git history changes
```

### Story 6.3 - Extract review candidates

As an author, I want notes converted into typed proposals so that I can curate
structure without giving the model authority over canon.

#### Scenario 6.3.1 - Extract validated candidates

Requirements: M6-R06, M6-R07.

```gherkin
Given a valid import and a ready extraction provider/model selection
When extraction runs for selected chunk IDs
Then the extractor receives only those chunks and an explicit candidate schema
And valid Codex, arc, chapter, and scene proposals enter the pending review queue
And every candidate records sorted unique chunk provenance and a revision
And raw provider output is not persisted
And Codex, outline, and scene canon do not change
And the complete candidate batch creates exactly one Git checkpoint
```

#### Scenario 6.3.2 - Reject invalid model output safely

Requirements: M6-R06, M6-R07.

```gherkin
Given the provider is unavailable or returns oversized, malformed, unknown-field,
  duplicate-ID, invalid-type, invalid-parent, or empty candidate output
When extraction runs
Then no candidate document or partial attempt is committed
And the API returns a safe error without provider body or credential disclosure
And canonical story state and Git history remain unchanged
```

### Story 6.4 - Curate the review queue

As an author, I want to edit, merge, or discard proposals so that only useful,
author-approved material remains.

#### Scenario 6.4.1 - Edit a pending candidate

Requirements: M6-R08.

```gherkin
Given a pending candidate at revision A
When I save valid edited proposal fields with revision A
Then the candidate is replaced at revision B without changing its type or provenance
And no Codex, outline, or scene file changes
And exactly one review checkpoint is created
When I retry with stale revision A
Then the API returns 409 Conflict and revision B remains unchanged
```

#### Scenario 6.4.2 - Merge compatible candidates

Requirements: M6-R08, M6-R09.

```gherkin
Given two pending Codex candidates have the same Codex type
When I merge them with both current revisions and an explicit merged payload
Then one new pending candidate contains the validated payload
And provenance is the sorted union of both inputs
And both inputs become merged with the replacement candidate ID
And no canonical story state changes
And exactly one review checkpoint is created
```

Only two Codex candidates of the same type may be merged in Milestone 6.
Outline-candidate merging and automatic field reconciliation are out of scope.

#### Scenario 6.4.3 - Discard a candidate

Requirements: M6-R08.

```gherkin
Given a pending candidate at its current revision
When I discard it
Then it becomes discarded and remains auditable in the durable queue
And repeated or stale terminal decisions return 409 Conflict
And canon remains unchanged and exactly one review checkpoint is created
```

### Story 6.5 - Accept into canon explicitly

As an author, I want an explicit final acceptance so that proposals cannot
silently become story truth.

#### Scenario 6.5.1 - Accept a Codex candidate

Requirements: M6-R08, M6-R10, M6-R12.

```gherkin
Given a valid pending Codex candidate and a clean project
When I accept it at its current revision
Then the existing Codex create boundary assigns the canonical ID
And the candidate becomes accepted with that canonical ID
And the derived index is current and exactly one Git checkpoint is created
```

#### Scenario 6.5.2 - Accept outline candidates in dependency order

Requirements: M6-R08, M6-R11, M6-R12.

```gherkin
Given a pending arc candidate
When I accept it
Then one canonical arc is created and its ID is recorded on the accepted candidate
Given a chapter or scene candidate references an accepted parent candidate
When I accept it
Then the canonical parent ID is resolved and one canonical child is created
```

A chapter must reference an accepted arc candidate. A scene must reference an
accepted chapter candidate. Free selection of existing canonical parents is not
part of Milestone 6.

#### Scenario 6.5.3 - Preserve consistency on failed or concurrent acceptance

Requirements: M6-R08, M6-R12.

```gherkin
Given two decisions race for one pending candidate
When accept, discard, or merge claims occur concurrently
Then exactly one decision claims the candidate
And a failed story write, index rebuild, checkpoint, or candidate finalization restores all pre-request bytes and state
And no orphan canonical entity, partial review state, or extra commit remains
```

### Story 6.6 - Complete the flow in the UI

As an author, I want an import and review workbench so that I can complete the
workflow without editing project files manually.

#### Scenario 6.6.1 - Import and extract

Requirements: M6-R13, M6-R14.

```gherkin
Given an active project
When I open Import Review, enter an absolute source folder, and choose Import
Then visible progress states prevent duplicate requests
And imported files and chunks are inspectable
When I select chunks and a ready provider/model and choose Extract
Then pending candidates appear with type and provenance
```

#### Scenario 6.6.2 - Review without losing edits

Requirements: M6-R14.

```gherkin
Given I have edited a candidate form
When I navigate, reload, merge, discard, accept, or start another selection
Then the UI requires confirmation before losing dirty edits
And conflicts retain my draft while offering reload
And accepted, merged, and discarded candidates are visibly terminal and not editable
```

## Canonical file contracts

### Import manifest

Store one manifest at `imports/raw/<import_id>/manifest.yaml`. The import ID is
`imp_` plus 20 lowercase hexadecimal characters. Files are copied below
`imports/raw/<import_id>/files/` using normalized source-relative paths.

```yaml
version: 1
id: imp_0123456789abcdef0123
created_at: "2026-06-30T12:00:00Z"
files:
  - path: notes/characters.md
    bytes: 1240
    sha256: 64-lowercase-hex-digest
```

Rules:

- `files` is non-empty and sorted bytewise by slash-separated path.
- Paths are relative, clean, NFC-normalized UTF-8 paths with no empty, `.`,
  `..`, absolute, backslash, control-character, or hidden path component.
- Case-folded destination path collisions are rejected for portability.
- Maximums: 500 files, 5 MiB per file, and 50 MiB per import batch.
- `created_at` comes from the injected clock and is normalized to UTC RFC3339.
- Manifest and snapshots are written to a temporary sibling directory, synced,
  renamed into place, indexed, and checkpointed. Failure restores the prior
  index and Git state and removes the temporary tree.
- Import IDs are never derived from external paths or content names.

Project settings remain version 1. Copy-only is the Milestone 6 policy; adding
an import-mode setting before a reference implementation exists would create a
misleading, unsupported configuration surface.

### Chunk record

Derived chunks may use implementation-private storage under
`.storywork/import/<import_id>/`, but the domain/API shape is fixed:

```json
{
  "id": "chk_0123456789abcdef0123",
  "import_id": "imp_0123456789abcdef0123",
  "source_path": "notes/characters.md",
  "start_line": 1,
  "end_line": 18,
  "text": "# Characters\n...",
  "sha256": "64-lowercase-hex-digest"
}
```

Chunk IDs are deterministic: hash the explicit chunking algorithm version,
import ID, normalized source path, line range, and exact chunk bytes; use the
first 20 lowercase hexadecimal characters after `chk_`. A collision with
different input is an error. Chunk order is source path then start line.

Chunking algorithm version 1:

1. Split normalized text into lines while retaining newline bytes.
2. Accumulate up to 8,000 bytes.
3. Within the limit prefer the latest boundary immediately before an ATX
   heading (`#` through `######` followed by space/end); otherwise prefer the
   latest blank-line boundary; otherwise split before the next whole line.
4. Never split a UTF-8 code point or a line. One line over 8,000 bytes becomes
   one oversized chunk, capped by the 5 MiB source-file limit.
5. Omit no bytes. Empty files produce no chunks but remain in the manifest.

### Candidate document

Store each candidate at `imports/review/<candidate_id>.yaml`. The candidate ID
is `cand_` plus 20 lowercase hexadecimal characters.

```yaml
version: 1
id: cand_0123456789abcdef0123
kind: codex
proposal_version: 1
status: pending
revision: sha256:...
provenance:
  chunk_ids: [chk_0123456789abcdef0123]
proposal:
  codex:
    type: character
    name: Mara Venn
    aliases: [Mara]
    tags: [pilot]
    description: A cautious salvage pilot.
  arc: null
  chapter: null
  scene: null
decision:
  replacement_candidate_id: null
  canonical_refs: []
```

The implementation must use strict Go proposal types internally. Serialized
YAML contains exactly one proposal matching `kind`. Valid Milestone 6 kinds are
`codex`, `arc`, `chapter`, and `scene`; statuses are `pending`, `merged`,
`discarded`, and `accepted`. `proposal_version` versions the kind-specific
payload independently from the queue envelope.

- Codex proposal fields reuse existing Codex validation. Metadata and
  progressions are absent in Milestone 6.
- Arc proposal contains `title`.
- Chapter proposal contains `title` and `parent_candidate_id` naming an arc.
- Scene proposal contains `title` and `parent_candidate_id` naming a chapter.
- Aliases and tags are trimmed, deduplicated, and deterministically sorted.
- Provenance has 1 to 100 sorted unique chunk IDs.
- Revision is SHA-256 over canonical serialized content with the revision field
  omitted. Load recomputes and rejects a mismatched stored revision.
- Pending candidates have null/empty decision fields. Merged candidates set
  only `replacement_candidate_id`; accepted candidates set one non-empty
  `canonical_refs` item in Milestone 6. Discarded candidates set neither.
- A canonical reference is `{kind, id}`. It is an array because a later
  proposal kind may intentionally create zero, one, or several canonical
  artifacts. Milestone 6 handlers must still enforce exactly one result.
- Candidate writes are atomic and revision-protected. Queue listing sorts
  pending first, then merged, discarded, accepted, and then by candidate ID.

Import, extraction, edit, merge, discard, and acceptance each begin from the
existing clean-worktree requirement and leave the worktree clean. Successful
extraction publishes its complete candidate batch in exactly one commit.
Successful edit, merge, and discard each create exactly one commit containing
only their review files. This keeps proposals outside story canon while making
their durable state auditable and preventing review work from blocking ordinary
scene/Codex/outline mutations. Do not weaken or special-case the global dirty
worktree guard.

## Extraction boundary

Create an `internal/extract` package containing proposal types, prompt/input
assembly, strict output decoding, validation, and an `Extractor` interface
owned by the consumer. Keep provider HTTP mapping behind existing provider
adapters; do not make `internal/importer` understand OpenAI or Ollama shapes.

```go
type Request struct {
    Chunks    []Chunk
    Mode      Mode
    ProfileID string
    Model     string
}

type Result struct {
    Proposals []Proposal
    Provider  ProviderIdentity
}

type Extractor interface {
    Extract(context.Context, Request) (Result, error)
}
```

`Mode` is a validated domain enum and is part of prompt selection. Milestone 6
supports only `structure`. Unknown modes fail before provider execution. A
future mode such as `fragments`, `continuity_questions`, or `research_notes`
must be addable by registering its prompt/output validator without changing
import snapshot, chunk, queue, or transaction code.

The concrete adapter may reuse a lower-level provider-neutral chat completion
transport extracted from Milestone 5. Do not force extraction through
`agent.GenerateRequest`, which contains action-specific agent/style/context
types. Refactor shared transport below both consumers and retain existing
action behavior.

Extraction request rules:

- 1 to 50 chunk IDs, all belonging to one import; total exact chunk text at
  most 200 KiB.
- Explicit extraction mode `structure`; the dispatcher selects a registered
  mode handler before assembling a prompt.
- Explicit `profile_id` and non-empty `model`; profile must be ready and
  chat-capable at run time.
- One deterministic system instruction and one user message containing chunk
  IDs, provenance, text, and the exact JSON candidate schema.
- Non-streaming generation, 60-second existing outbound timeout, and existing
  redirect/credential/body safety rules.
- Response is one JSON object with exactly `{"candidates":[...]}` and no
  Markdown fence or trailing value. Maximum response is 1 MiB and maximum 200
  candidates.
- Generated parent references use extraction-local proposal IDs. Resolve them
  to persisted candidate IDs before the batch is atomically published. Reject
  missing, duplicate, self, wrong-kind, or cyclic references.
- Zero valid candidates is a validation error. Never silently drop an invalid
  candidate from a mixed response.

## Review transaction and acceptance rules

Use an `internal/importer.Service` to coordinate manifests, chunks, extraction,
review state, and story mutation ports. Use a per-project mutex shared by all
import/review operations. Candidate terminal decisions use a claim/finalize
state machine so concurrent requests cannot both act.

Candidate behavior uses a compile-time registry of small typed handlers keyed
by `(kind, proposal_version)`. A handler owns payload decoding, validation,
editable-field normalization, merge eligibility, dependency validation, and
acceptance mapping. The queue store owns only the common envelope, revision,
provenance, and decision state. Do not use `map[string]any` in application
logic; a handler may use `json.RawMessage` or `yaml.Node` only at the dispatch
boundary. Missing handlers and unsupported proposal versions fail loudly.

Acceptance spans candidate state and existing canonical story writes. Before
mutation, snapshot every affected review and story file plus index state. The
story layer must expose an acceptance-oriented transaction method that creates
the entity without independently committing early. The coordinator then:

1. validates and claims the current pending candidate revision,
2. resolves and validates any accepted parent candidate,
3. writes the canonical entity through story-owned adapters,
4. writes accepted candidate state with the canonical references,
5. rebuilds the derived index,
6. creates exactly one commit named `Accept import candidate <candidate_id>`,
7. finalizes the claim and returns the candidate plus canonical references.

Any failure restores exact pre-request bytes, rebuilds the old index, restores
Git staging/worktree state, and releases the claim. A rollback failure is a
500 error that includes both safe primary and rollback context. Do not compose
two public service calls that each own independent checkpoint behavior.

Merge atomically changes both source candidates and creates the replacement.
It requires both revisions and explicit author-supplied merged fields. Editing,
merging, and discarding do not invoke a model. Each successful operation
rebuilds the index and creates one checkpoint; every failure restores exact
pre-request files, index, staging state, and commit count.

## HTTP contract

All routes require an active project. Known routes return JSON errors and `405
Method Not Allowed` with `Allow`. Request bodies use strict recursive JSON,
reject missing/null/unknown/wrong/trailing fields, and are limited to 1 MiB
unless noted. IDs are path-unescaped once and strictly validated.

```http
POST /api/imports
GET  /api/imports
GET  /api/imports/{import_id}/chunks
POST /api/imports/{import_id}/extractions
GET  /api/import-candidates?status=pending&kind=codex
GET  /api/import-candidates/{candidate_id}
PUT  /api/import-candidates/{candidate_id}
POST /api/import-candidates/{candidate_id}/merge
POST /api/import-candidates/{candidate_id}/discard
POST /api/import-candidates/{candidate_id}/accept
```

### Import

Request:

```json
{"source_directory":"/absolute/path/to/notes"}
```

Success is `201 Created`:

```json
{
  "import":{"id":"imp_0123456789abcdef0123","created_at":"2026-06-30T12:00:00Z","file_count":2,"total_bytes":2048},
  "files":[{"path":"notes/characters.md","bytes":1240,"sha256":"..."}]
}
```

`GET /api/imports` returns `{"imports":[...]}` without external paths.

### Chunks and extraction

`GET /api/imports/{import_id}/chunks` returns `{"chunks":[...]}` using the
fixed chunk shape. Text is included because the review UI must be inspectable.

Extraction request:

```json
{
  "chunk_ids":["chk_0123456789abcdef0123"],
  "mode":"structure",
  "profile_id":"local_ollama",
  "model":"qwen2.5:7b"
}
```

Success is `201 Created`:

```json
{
  "candidates":[/* complete candidate response objects */],
  "provider":{"profile_id":"local_ollama","type":"ollama","model":"qwen2.5:7b"}
}
```

### Queue and edit

Candidate responses include `id`, `kind`, `proposal_version`, `status`,
`revision`, `provenance`, the kind-specific `proposal`, nullable
`replacement_candidate_id`, and `canonical_refs` (always an array). Lists
always use arrays, never null. Query filters are optional and
repeat/unknown/empty values are rejected.

Edit request replaces editable proposal fields only:

```json
{
  "proposal":{"type":"character","name":"Mara Venn","aliases":["Mara"],"tags":["pilot"],"description":"A cautious salvage pilot."},
  "expected_revision":"sha256:..."
}
```

Success is `200 OK` with the complete updated candidate.

### Merge, discard, and accept

Merge is invoked on the first candidate:

```json
{
  "other_candidate_id":"cand_abcdef0123456789abcd",
  "expected_revision":"sha256:...",
  "other_expected_revision":"sha256:...",
  "proposal":{"type":"character","name":"Mara Venn","aliases":["Mara"],"tags":["pilot"],"description":"Merged author text."}
}
```

Success is `201 Created` with `candidate` plus `merged_candidate_ids`.

Discard and accept requests are:

```json
{"expected_revision":"sha256:..."}
```

Discard returns `200 OK` with the terminal candidate. Accept returns `200 OK`:

```json
{
  "candidate":{/* accepted candidate */},
  "canonical_refs":[{"kind":"codex","id":"char_0123456789abcdef0123"}]
}
```

## Future compatibility boundaries

Milestone 6 does not implement fragments, archival, deletion, or canonical
lifecycle controls. It must avoid making those features require a queue/storage
rewrite later.

### Unplaced or disposable material

An unplaced scene fragment is not a `scene` with a fake chapter and is not a
discarded extraction candidate. A future `fragment` candidate can have its own
versioned payload and acceptance handler, with no outline parent requirement.
It may remain a proposal, become a tracked draft artifact, or produce multiple
canonical references. The generic candidate envelope, extraction-mode registry,
and canonical-reference array are the reserved seams; Milestone 6 must not add
a placeholder fragment format.

### Canon that becomes outmoded

Review status describes the proposal workflow only. It must never be reused as
the lifecycle status of accepted story entities. Likewise, Codex progressions
describe facts changing at a position inside the story timeline; they do not
mean that an author has editorially superseded old canon.

A future editorial-lifecycle feature should be able to add a versioned sidecar
document keyed by stable canonical references, with states such as active,
superseded, or retired and explicit replacement references. Keeping lifecycle
as a sidecar avoids adding optional lifecycle fields independently to every
strict arc/chapter/scene/Codex schema. Existing canonical IDs and files must not
be silently deleted or reused. Milestone 6 acceptance records stable canonical
references so such a lifecycle layer can trace provenance later, but it does
not create that sidecar.

### Schema evolution

Strict decoding remains required. Extensibility comes from explicit versions
and registered handlers, not ignored unknown fields. A new kind, mode, proposal
version, or canonical lifecycle schema requires tests and a deliberate version
addition; old supported documents must continue to load or fail with a clear
unsupported-version error. Queue ordering and common state transitions must
operate on the envelope without understanding kind-specific payloads.

## HTTP status mapping

- `400 Bad Request`: malformed contract input, invalid path/ID/filter, empty
  import, limits, invalid merge, invalid generated data, or invalid parent.
- `404 Not Found`: valid import, chunk, candidate, or referenced parent ID does
  not exist.
- `409 Conflict`: no active project, stale revision, non-pending candidate,
  concurrent claim, dirty-worktree conflict, or unaccepted parent candidate.
- `413 Request Entity Too Large`: HTTP body exceeds the documented limit.
- `502 Bad Gateway`: provider rejects or returns an invalid response.
- `503 Service Unavailable`: provider/credential is unavailable or times out.
- `500 Internal Server Error`: canonical corruption or filesystem, index, Git,
  rollback, configuration, or unexpected adapter failure.

Errors must not expose external paths, copied note contents, provider response
bodies, endpoint URLs, credential references, or credentials.

## Frontend contract

Add `web/src/imports/` and keep only top-level navigation in `App.tsx`.

The Import Review workbench must provide:

- an active-project requirement and clear empty state,
- a labeled absolute source-directory input and Import button,
- imported batches/files with byte counts but no external source path,
- file/chunk inspection with keyboard-operable chunk selection,
- ready provider/profile and model controls without credential fields,
- disabled duplicate actions during import/extraction/decision requests,
- candidate filters, counts, kind/status labels, and provenance links,
- kind-specific edit forms with client validation,
- explicit merge selection plus an author-edited merged form,
- confirmation for discard and accept,
- dirty-edit navigation and `beforeunload` protection,
- stale-response suppression when selection/project changes,
- conflict recovery that preserves the author's draft,
- terminal accepted/merged/discarded rendering and canonical-reference display,
- focus placement and an ARIA live status for operation results.

Do not use browser directory-upload APIs: the local backend needs a filesystem
path, matching the existing create/open project workflow. Do not put note text,
candidate drafts, provider values, or paths in local/session storage.

## Automated acceptance

Add one real-adapter integration test using temporary source/app/project
directories, real filesystem stores, SQLite, Git, HTTP handlers, and a local
`httptest.Server` provider. It must prove:

1. project creation/open and a clean baseline,
2. recursive import ordering and exact copied normalized bytes,
3. no external source path in project files or API responses,
4. one import commit and a clean worktree immediately after import,
5. deterministic chunks and provenance,
6. extraction creates pending candidate files but no Codex/outline changes,
7. edit, merge, and discard change only review files with one commit each,
8. acceptance of an arc, dependent chapter, dependent scene, and Codex entry,
9. exactly one commit per acceptance and current SQLite hashes,
10. stale/concurrent decisions permit one winner,
11. an injected acceptance failure restores exact bytes and commit count,
12. no credential, provider response body, or external path leaks,
13. the complete queue reloads after service reconstruction.

## Required test matrix

At minimum, add named tests for:

- discovery traversal, hidden directories, symlinks, path normalization,
  case-fold collisions, mutation-during-read detection, limits, and ordering,
- import staging, fsync/rename/index/Git failures, cleanup, and byte-exact rollback,
- chunk boundary preferences, Unicode, long lines, empty files, deterministic
  IDs, rebuild, and collision detection,
- strict candidate YAML/JSON, enum and sum-type validation, revisions,
  deterministic ordering, malformed-state refusal, and atomic store failures,
- extraction prompt/input exactness, strict response parsing, parent graph
  resolution, limits, cancellation, provider-safe errors, and zero partial save,
- every review transition, stale revisions, incompatible merge, provenance
  union, terminal-state refusal, and concurrent claims under `-race`,
- Codex and outline acceptance, accepted-parent resolution, dirty worktree,
  rollback at every persistence boundary, and exactly one checkpoint,
- exact API methods, bodies, responses, filters, limits, statuses, `Allow`, and
  non-disclosure assertions,
- frontend transport, import/chunk/extract flow, edit/merge/discard/accept,
  dirty/conflict/loading/error/terminal states, focus, and navigation races,
- all named Milestone 0-5 regressions affected by shared transport, story
  transactions, Git policy, index rebuild, and `App.tsx` navigation.

## Documentation and status updates

Before marking Milestone 6 complete:

1. Update `docs/03_storage_model.md` with import manifests, derived chunks,
   durable candidates, revisions, and rebuild rules.
2. Update `docs/05_milestones.md` with actual Milestone 6 behavior and completion
   date; identify Milestone 7 as next.
3. Add the final routes and exact shapes to `docs/06_api_contract.md`.
4. Update `docs/07_frontend_editor.md` with implemented Import Review behavior.
5. Add exact named Milestone 6 evidence to `docs/08_testing_acceptance.md`.
6. Update `docs/02_architecture.md` and its package map for `internal/importer`
   and `internal/extract`, including the shared lower-level provider transport
   if the implementation introduces it.
7. Update `DOCUMENTATION.md` API inventory, version, and any new comment rules.
8. Update `README.md` current status/package map and name Milestone 7 next.
9. Create `.plans/milestone_6_test_evidence.md` mapping every requirement and
   BDD scenario to exact test names and observable assertions.
10. Maintain `.plans/milestone_6_status.md` from baseline through completion;
    record phase, completed slices, exact verification commands/results,
    residual limitations, artifacts, and next step.
11. If review finds defects, create/update a scoped remediation plan and keep
    status `in progress` until evidence and checks pass.
12. Re-run all checks after the final documentation edit. Documentation written
    before behavior exists must remain future-tense and must not claim success.

## Final verification

Use `/home/linuxbrew/.linuxbrew/bin/go` first when available. Run:

```bash
go fmt ./...
go vet ./...
go test ./... -count=1
go test -race ./...
cd web && npm run lint
cd web && npm run typecheck
cd web && npm test -- --run
make check
git diff --check
git status --short
```

Also inspect the story-project test fixtures and repository for unexpected
databases, `.storywork/import` caches, imported note contents, provider output,
credentials, build output, live servers, and other generated artifacts. Do not
read or print the developer's environment while checking for leaks.

Milestone 6 is complete only when the full import-to-accept UI flow works, every
requirement has direct named evidence, the documentation/status checklist is
complete, and all verification commands pass. Otherwise record the concrete
next failing step in `.plans/milestone_6_status.md` and leave the phase in
progress.
