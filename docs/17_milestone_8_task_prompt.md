# Milestone 8 Task Prompt - What-If Branches and Ramification Analysis

Implement only Milestone 8. Milestones 0 through 7 are complete and are
regression constraints. Follow the ordered red/green/refactor sequence in
`.plans/milestone_8_implementation.md` and update
`.plans/milestone_8_status.md` after every work unit.

This document is the durable implementation contract. When an older general
document conflicts with this contract, this contract controls Milestone 8.
In particular, one project uses one working directory, `main` is the fixed
canon branch, and experiment comparisons read Git objects directly. Do not add
a second checkout merely to display canon and experiment content together.

## Outcome

An author can create and enter a controlled what-if experiment from `main`,
continue using the existing story, Codex, import, and AI mutation flows on that
experiment branch, inspect every changed file against current `main`, request a
structured ramification analysis, and then either discard the experiment or
promote selected complete files to `main`.

The branch comparison screen shows current canon and experiment text in two
read-only panes with line-level additions, deletions, and modifications
highlighted. It works while the experiment branch remains checked out because
the backend reads `main:<path>` and the experiment commit tree through Git. The
author does not edit both versions simultaneously.

Promotion is deliberately conservative. It promotes whole selected files, not
arbitrary hunks, and refuses a selected path when `main` changed that path
after the experiment forked. The resulting `main` snapshot must pass strict
canonical validation before one promotion checkpoint is created.

## Approved architecture decisions

The author approved these decisions while planning Milestone 8:

1. Use one project working directory and one checked-out branch at a time.
2. Treat `main` as the fixed canon branch.
3. Put branch experiments, comparisons, promotion, discard, and ramification
   orchestration in a dedicated `internal/branch` boundary.
4. The UI must compare an active experiment against `main` directly from Git
   and render side-by-side highlighted differences.
5. Refactor shared provider chat transport from `internal/agent` into
   `internal/modelchat` before adding the third AI consumer.

These decisions are fixed for this milestone. Do not replace them with Git
worktrees, cloned directories, configurable canon refs, or a new generic
workflow framework.

## Product terminology

- **Canon branch**: the local Git branch named exactly `main`.
- **Experiment**: a branch under the reserved `branch/` namespace created from
  the current `main` commit.
- **Active branch**: the branch currently checked out in the one project
  working directory.
- **Experiment base**: the merge base of `main` and the experiment. Experiments
  must retain this base as an ancestor; rewritten or unrelated history fails
  closed.
- **Comparison**: a direct tree-to-tree comparison of current `main` and the
  experiment head. It is not a comparison only against the old merge base.
- **Ramification analysis**: a transient, structured provider result describing
  possible consequences of the reviewed branch diff. It never edits files.
- **Promotion**: copying selected complete experiment files or deletions onto
  `main`, validating the resulting project, and creating exactly one commit.
- **Discard**: deleting an experiment ref without merging it into `main`.

Text files remain the authoritative project representation for whichever
branch is checked out. In product language, only the `main` versions are canon;
checked-out experiment files are branch-local alternatives. SQLite remains a
rebuildable index of the active checked-out tree.

## Scope boundaries

In scope:

1. Detect and report the active branch, current `main` head, and experiment
   head without exposing the absolute project path.
2. Create validated experiment branches from current `main` and switch the
   one working tree to them.
3. Switch between `main` and Storywork-managed experiments only when the Git
   worktree is clean.
4. Rebuild the disposable index after every successful checkout and restore
   the prior branch if checkout/index synchronization fails.
5. List experiments deterministically from the reserved `branch/` namespace.
6. Compare current `main` and an experiment head without switching branches.
7. Return changed-file status plus bounded UTF-8 content for one selected file.
8. Render canon and experiment text side by side with accessible line-level
   highlighting and synchronized file selection.
9. Run explicit, provider-neutral ramification analysis from a bounded,
   inspectable branch-diff packet.
10. Parse all ramification findings strictly and retain no prompt or generated
    analysis after the response is delivered.
11. Promote selected complete files or deletions with optimistic ref checks,
    path-level divergence checks, canonical validation, rollback, index rebuild,
    and exactly one `main` commit.
12. Discard an experiment explicitly, switching to `main` first when it is
    active and rebuilding the index.
13. Invalidate stale editor, Codex, import-review, context-preview, action-run,
    and branch-comparison UI state after a branch switch.
14. Extract shared OpenAI-compatible/Ollama chat transport to
    `internal/modelchat` without changing existing provider behavior.

Out of scope:

- multiple worktrees, clones, or simultaneous editable canon/experiment panes,
- remote Git operations, pull, push, fetch, authentication, or hosted review,
- arbitrary user branches outside `branch/`, detached HEAD operation, tags, or
  changing the fixed canon branch,
- automatic branch creation or provider execution,
- automatic ramification-driven edits,
- hunk-level, line-level, or semantic merge promotion,
- automatic conflict resolution, rebasing, merging, cherry-picking, force
  updates, history rewriting, or deletion of `main`,
- promoting a path that changed on `main` since the experiment base,
- promoting an invalid partial structure such as an outline reference without
  its required canonical file,
- branch-specific SQLite databases or persistent branch metadata databases,
- durable storage of ramification prompts, diff prose, or generated findings,
- undo/cascade revert of Milestone 7 operation dependencies,
- background Git polling, filesystem watching, or background model work.

## Requirements

| ID | Requirement |
| --- | --- |
| M8-R01 | `main` is the fixed canon ref and every managed experiment is under a strictly validated `branch/` ref. |
| M8-R02 | Experiment creation always starts at the current `main` commit and atomically leaves that experiment checked out with an index matching its tree. |
| M8-R03 | Branch-changing operations require a clean worktree and execute under the shared mutation coordinator. |
| M8-R04 | A checkout/index failure restores the previous branch and a usable index, or returns a joined internal error that makes incomplete recovery explicit. |
| M8-R05 | Comparison reads `main` and experiment Git objects directly, never requires a second checkout, and compares their current trees rather than only their merge base. |
| M8-R06 | Changed-file status, ordering, paths, commit IDs, and comparison fingerprints are deterministic and parsed from machine-safe Git output. |
| M8-R07 | File comparison is limited to validated project-relative canonical text paths and bounded strict UTF-8 content. |
| M8-R08 | The frontend renders read-only side-by-side canon and experiment panes with accessible line-level highlighting, including added, deleted, and modified files. |
| M8-R09 | Ramification analysis requires a separate explicit request, sends only the bounded reviewed comparison packet, and performs no file, index, staging, ref, or history mutation. |
| M8-R10 | Ramification output is strict structured data with bounded findings, validated categories/severity/path references, and no replacement prose authority. |
| M8-R11 | Shared provider chat transport belongs to `internal/modelchat`; agent, extraction, and branch analysis depend on narrow consumer-owned abstractions. |
| M8-R12 | Promotion uses expected canon head, experiment head, and comparison fingerprint and rejects stale requests before branch or file mutation. |
| M8-R13 | Promotion copies or deletes only selected complete files, rejects selected paths changed on `main` since the experiment base, and never applies arbitrary hunks. |
| M8-R14 | The tentative promoted `main` snapshot passes strict project validation and disposable-index rebuild before exactly one promotion commit. |
| M8-R15 | Promotion failure restores `main` bytes, staging state, index, and history; success leaves `main` checked out and clean. |
| M8-R16 | A promotion commit records validated experiment, source, and base identifiers without author/model prose in commit metadata. |
| M8-R17 | Discard is explicit, never changes `main`, safely handles active and inactive experiments, and refuses stale or unknown experiment refs. |
| M8-R18 | Branch switches invalidate branch-sensitive frontend state and cannot silently discard an unsaved browser draft. |
| M8-R19 | Existing story and AI operations work unchanged on a checked-out experiment and continue to make one commit per accepted mutation on that branch. |
| M8-R20 | Preserve Milestone 0-7 behavior and maintain exact requirement/scenario/test/status evidence until full checks pass. |

## BDD stories

### Story 8.1 - Create and enter a what-if experiment

As an author, I want an experiment to start from canon so that alternate work
cannot silently modify `main`.

#### Scenario 8.1.1 - Create from current canon

Requirements: M8-R01, M8-R02, M8-R03.

```gherkin
Given main exists and the project worktree is clean
And another branch may currently be active
When I create a what-if experiment named "Obi-Wan lives"
Then a unique branch/ experiment ref is created at current main
And that experiment becomes the active branch
And the disposable index represents the experiment tree
And main's ref and tree are unchanged
```

#### Scenario 8.1.2 - Reject unsafe branch state

Requirements: M8-R01, M8-R03, M8-R04.

```gherkin
Given the worktree is dirty, HEAD is detached, main is missing, or the requested
  experiment ref is invalid
When I create or switch an experiment
Then the request fails before changing refs or files
And the index remains synchronized with the still-active tree
```

#### Scenario 8.1.3 - Continue normal work in the experiment

Requirements: M8-R03, M8-R19.

```gherkin
Given an experiment is active
When I save a scene, edit a progression, or accept an AI patch
Then the existing operation commits to the experiment branch
And main remains byte-for-byte and commit-for-commit unchanged
```

### Story 8.2 - Compare an experiment to current canon

As an author, I want to inspect both versions together so that I can understand
the experiment without switching away from it.

#### Scenario 8.2.1 - List exact changed files

Requirements: M8-R05, M8-R06, M8-R07.

```gherkin
Given an experiment has added, modified, and deleted canonical text files
When I request its comparison
Then the backend compares current main and the experiment head directly
And returns deterministic added, modified, and deleted path records
And returns main head, experiment head, base head, and a comparison fingerprint
And no branch, worktree file, index, staging state, or history changes
```

Use endpoint tree comparison equivalent to `git diff --no-renames main <head>`.
Do not use triple-dot diff as the product comparison: triple-dot describes
changes since the fork and would hide newer canon-only changes.

#### Scenario 8.2.2 - Show side-by-side text

Requirements: M8-R05, M8-R07, M8-R08.

```gherkin
Given a changed UTF-8 project file is selected
When I open its comparison
Then the left pane shows current main content
And the right pane shows experiment content
And corresponding added, deleted, and changed lines are highlighted
And a missing side is labeled for an added or deleted file
And neither pane is editable
```

#### Scenario 8.2.3 - Keep comparisons bounded and safe

Requirements: M8-R06, M8-R07.

```gherkin
Given a path is absolute, traverses, is not in the comparison, is not a
  supported project text path, is non-UTF-8, or exceeds the display limit
When file comparison is requested
Then the request fails without returning partial content
And no host path or raw Git diagnostic is exposed
```

### Story 8.3 - Analyze ramifications without changing either branch

As an author, I want structured consequences of an experiment so that I can
decide what needs attention before promotion.

#### Scenario 8.3.1 - Run only after explicit authorization

Requirements: M8-R09, M8-R11.

```gherkin
Given I have reviewed a current experiment comparison
When I explicitly request ramification analysis with a ready provider profile
Then the provider receives one bounded packet containing the experiment goal,
  changed paths, and reviewed diff text
And no provider was called merely by creating, switching, or comparing
```

The request carries expected main head, experiment head, and comparison
fingerprint. Rebuild and compare them immediately before provider execution.

#### Scenario 8.3.2 - Return strict findings only

Requirements: M8-R09, M8-R10.

```gherkin
Given the provider returns valid ramification JSON
When the response is parsed
Then I receive a summary and bounded findings with category, severity,
  explanation, affected paths, and recommended review action
And I receive provider identity and a redacted input manifest
And no finding can be accepted as a file patch
```

#### Scenario 8.3.3 - Reject stale, oversized, or malformed analysis

Requirements: M8-R09, M8-R10.

```gherkin
Given the comparison changed, the packet exceeds budget, or provider output is
  malformed or references an unreviewed path
When analysis is attempted
Then it fails before publishing findings
And files, index, staging, refs, and Git history remain unchanged
```

### Story 8.4 - Promote selected complete files conservatively

As an author, I want to promote only reviewed files so that unrelated
experiment work remains outside canon.

#### Scenario 8.4.1 - Promote selected files to main

Requirements: M8-R12, M8-R13, M8-R14, M8-R16.

```gherkin
Given the matching experiment is active and clean
And expected main and experiment heads match
And selected paths did not change on main since the experiment base
When I promote selected complete files
Then the service switches to main under the mutation lock
And applies exactly the selected experiment blobs and deletions
And validates the complete resulting canonical project
And rebuilds the index
And creates exactly one promotion commit with validated provenance
And leaves main active and clean
```

Promotion always ends on `main`. The experiment ref remains available for
further comparison or explicit discard. The author may switch back later.

#### Scenario 8.4.2 - Reject a path changed on canon

Requirements: M8-R12, M8-R13.

```gherkin
Given main changed one selected path after the experiment forked
When I attempt promotion
Then promotion returns a conflict naming that project-relative path
And no checkout, file write, index rebuild, staging change, or commit occurs
```

Do not auto-merge. The author must create a fresh experiment or manually
reconcile through later supported work.

#### Scenario 8.4.3 - Reject an invalid selected subset

Requirements: M8-R13, M8-R14, M8-R15.

```gherkin
Given selected files alone would leave an outline reference missing, a parent
  mismatch, malformed YAML, invalid scene front matter, or another invalid
  canonical relationship
When promotion validates the tentative main snapshot
Then promotion fails
And main bytes, index, staging, refs, and history are restored
```

#### Scenario 8.4.4 - Roll back adapter failures

Requirements: M8-R04, M8-R15.

```gherkin
Given checkout, file application, validation, index rebuild, staging, or commit
  fails during promotion
When recovery runs
Then every touched main path is restored exactly
And app-created staging is removed
And the index is rebuilt from restored main
And no partial promotion commit remains
```

### Story 8.5 - Discard an experiment explicitly

As an author, I want to discard an unwanted branch so that canon remains clear
without retaining accidental alternatives.

#### Scenario 8.5.1 - Discard the active experiment

Requirements: M8-R03, M8-R17, M8-R18.

```gherkin
Given an experiment is active and both browser draft state and Git worktree are clean
When I confirm discard
Then the app switches to main and rebuilds the index
And deletes only that experiment ref
And main's tree and history remain unchanged
And branch-sensitive UI state is cleared
```

#### Scenario 8.5.2 - Refuse unsafe discard

Requirements: M8-R03, M8-R17.

```gherkin
Given the worktree is dirty, the expected experiment head is stale, the ref is
  not managed by Storywork, or the requested ref is main
When discard is requested
Then no branch is switched or deleted
And no worktree or index state changes
```

## Experiment identity and branch naming

Experiment IDs use `brn_` plus 20 lowercase hexadecimal characters. Inject the
ID generator in tests. A normalized display slug contains lowercase ASCII
letters, digits, and single hyphens, is 1 to 48 bytes, and is derived from the
trimmed author name. The Git branch is:

```text
branch/<slug>-<20 lowercase hexadecimal characters>
```

The terminal hexadecimal suffix is the experiment ID without `brn_`, allowing
managed refs to be listed without a second metadata store. Names must be unique
as full refs. Reject control characters, ref metacharacters, empty slugs,
reserved names, double dots, repeated slash, `.lock`, and every input Git would
interpret ambiguously. Never pass an unvalidated ref or path to Git.

The API returns both `experiment_id` and `branch_name`. The frontend displays
the author-derived slug or branch name, never treats it as HTML, and does not
infer authority from display text.

## Branch and comparison model

`internal/branch` owns domain models and orchestration. Its service consumes
small interfaces defined by the consumer:

```go
type Repository interface {
    Status(context.Context, string) (RepositoryStatus, error)
    ListExperiments(context.Context, string) ([]ExperimentRef, error)
    CreateAndSwitch(context.Context, string, ExperimentRef, CommitID) error
    Switch(context.Context, string, BranchRef) error
    DeleteExperiment(context.Context, string, ExperimentRef, CommitID) error
    CompareTrees(context.Context, string, CommitID, CommitID) ([]ChangedFile, error)
    ReadTextBlob(context.Context, string, CommitID, ProjectPath) (Blob, error)
    MergeBase(context.Context, string, CommitID, CommitID) (CommitID, error)
    PathsChanged(context.Context, string, CommitID, CommitID) ([]ProjectPath, error)
    ApplyPaths(context.Context, string, CommitID, []ProjectPath) error
    CommitPromotion(context.Context, string, PromotionCommit) error
    UnstagePaths(context.Context, string, []ProjectPath) error
}

type Index interface {
    Rebuild(context.Context, string) error
}

type CanonicalValidator interface {
    ValidateProject(context.Context, string) error
}

type Analyzer interface {
    Analyze(context.Context, AnalysisRequest) (AnalysisResult, error)
}
```

Exact method grouping may be split into even smaller interfaces when tests make
that clearer. Do not widen existing story/import/action Git interfaces. The
production Git adapter may expose additional concrete methods while each
consumer retains only what it uses.

All branch-changing service operations acquire the application-scoped
`mutation.Coordinator` write lock. Read-only status/comparison snapshot loading
uses its read lock. Ramification service code captures and fingerprints its
packet under the read lock, releases the lock, then calls the provider. It must
not hold the project mutation lock during network work.

## Git command and parsing rules

- Use `git symbolic-ref --quiet --short HEAD` for active branch detection.
- Resolve commits with `git rev-parse --verify <validated-ref>^{commit}` and
  require full lowercase object IDs in application models.
- Use NUL-delimited machine output for changed paths and ref listing wherever
  Git supports it. Never parse localized human status text.
- Disable rename detection for MVP comparison and promotion. A rename is an
  added path plus a deleted path and both must be selected to promote it.
- Put `--` before pathspecs. Pass arguments directly to `exec.CommandContext`;
  never invoke a shell or build a command string.
- Comparison is current `main` tree versus experiment head tree.
- Conflict detection is merge base versus current `main`, intersected with the
  selected paths.
- Refuse experiment history when `main` and the experiment have no valid merge
  base or the stored experiment side no longer descends from that base.
- Do not use Git as a query engine for story semantics. It supplies refs, blobs,
  and diffs; canonical validation remains a project-format concern.

## Comparison fingerprint and bounds

Compute `sha256:` plus lowercase SHA-256 over a versioned byte stream containing
current `main` commit ID, experiment commit ID, merge-base commit ID, and sorted
`status NUL path NUL` records. Do not hash absolute paths or file content into
the public fingerprint.

Changed statuses are exactly `added`, `modified`, and `deleted`. Reject Git
submodules, symlinks, type changes, unmerged entries, or unexpected status
codes. Sort by path byte order after parsing.

Promotable/displayable project paths are regular files under:

```text
outline.yaml
arcs/*.yaml
chapters/*.yaml
scenes/*.md
codex/{characters,locations,lore,custom}/*.yaml
progressions/*.yaml
agents/*.yaml
styles/*.yaml
imports/raw/**.md
imports/raw/**.yaml
imports/review/*.yaml
```

`project.yaml`, `.gitignore`, `.storywork/`, credentials, databases, generated
output, arbitrary root files, and `.gitkeep` are not promotable in Milestone 8.
Path validation must reject absolute paths, backslashes, empty/dot segments,
`..`, NUL, control characters, non-canonical separators, and paths outside the
allowlist.

Comparison list limit: 500 changed paths. File comparison limit: 5 MiB per
side and 200,000 lines per side. Both blobs must be strict UTF-8 without NUL.
An absent side for add/delete is represented by `exists: false` and empty text,
not by a Git error. Responses never contain absolute paths.

## Side-by-side diff behavior

The backend returns exact bounded text for each side and immutable ref IDs. A
pure frontend module computes display rows from lines:

```ts
type DiffRow = {
  kind: "equal" | "added" | "deleted" | "modified";
  canonLine: number | null;
  canonText: string | null;
  branchLine: number | null;
  branchText: string | null;
};
```

Use a deterministic line diff with explicit complexity bounds. Pair adjacent
delete/add blocks as `modified` rows for display only; this does not create a
merge or promotion hunk. For inputs beyond the frontend diff bound, show the
two complete scrollable texts with an explicit message that line highlighting
is unavailable. Never freeze the UI attempting an unbounded quadratic diff.

The component must provide:

- file status and path outside color alone,
- line numbers on both sides,
- labels `Canon (main)` and `Experiment`,
- textual added/deleted/modified indicators for assistive technology,
- independent horizontal scrolling and practical synchronized vertical
  scrolling for aligned rows,
- loading, empty, stale, unavailable, and error states,
- no editable control inside either comparison pane.

## Ramification analysis contract

Ramification analysis is a branch-owned use case, not a new editor action
scope. It consumes a dedicated `branch.Analyzer` interface. The production
adapter depends on `internal/modelchat`, not `internal/agent`.

Request fields:

```json
{
  "goal":"Explore the consequences if Obi-Wan survives.",
  "profile_id":"local_ollama",
  "model":"qwen2.5:7b",
  "expected_main_head":"<full object id>",
  "expected_experiment_head":"<full object id>",
  "comparison_fingerprint":"sha256:..."
}
```

`goal` is required, trimmed, strict UTF-8, no NUL, and at most 2,000 bytes.
The comparison packet includes only the goal, sorted changed paths/statuses,
and bounded unified diff text for allowed project files. It excludes Git
configuration, commit messages, credentials, `.storywork`, raw provider
settings, unrelated unchanged story files, and absolute paths.

Maximum packet size is 512 KiB using the conservative one-UTF-8-byte/one-token
estimate. Maximum 100 changed files may enter analysis. If the exact reviewed
comparison cannot fit, fail before provider execution; do not silently analyze
an undisclosed subset.

Valid provider output is one strict JSON object:

```json
{
  "summary":"The survival changes Luke's mentorship and later confrontations.",
  "findings":[
    {
      "category":"continuity",
      "severity":"high",
      "title":"Later death references conflict",
      "explanation":"Two later scenes still describe Obi-Wan as dead.",
      "affected_paths":["scenes/scn_0123456789abcdef0123.md"],
      "recommended_action":"Review the later scene before promotion."
    }
  ]
}
```

Rules:

- `summary`: required, 1 to 4,000 UTF-8 bytes.
- `findings`: required array, 0 to 30 entries.
- categories: `plot`, `character`, `continuity`, `timeline`, `world`, or
  `structure`.
- severities: `low`, `medium`, or `high`.
- title: 1 to 200 runes.
- explanation: 1 to 4,000 UTF-8 bytes.
- affected paths: 1 to 50 unique paths, each present in the reviewed changed
  path set.
- recommended action: 1 to 1,000 UTF-8 bytes and advisory only.
- reject unknown, missing, duplicate, null, wrongly typed, fenced, trailing,
  oversized, invalid-enum, invalid-path, or partially valid output as a whole.

The response includes provider identity and a redacted manifest containing ref
IDs, fingerprint, changed-file count, included paths, and estimated input size.
It contains no prompt. Findings are returned to the requesting browser and are
not written to Git, SQLite, project files, action run stores, or local storage.

## Shared modelchat refactor

`internal/agent/chat.go` currently owns transport used by both agent actions
and extraction. Before adding branch analysis:

1. Characterize exact OpenAI-compatible and Ollama request/response/error,
   timeout, redirect, body-limit, credential, and provider-identity behavior.
2. Create `internal/modelchat` with neutral `Message`, `Request`, `Response`,
   `ProviderIdentity`, and `Completer` concepts.
3. Move only shared chat transport and strict wire parsing. Do not move agent
   registry, prompt policy, extraction parsing, branch packet construction, or
   provider profile storage.
4. Let `internal/agent` retain compatibility aliases or thin mapping adapters
   where that avoids a broad API break; new consumers use `modelchat` directly.
5. Migrate extraction and action dispatch behind tests before adding branch
   analyzer code.

This is a targeted SRP/DIP refactor, not permission to redesign all providers.

## Promotion transaction

Promotion accepts a non-empty, unique, byte-sorted list of paths plus the exact
main head, experiment head, and comparison fingerprint last reviewed.

Before checkout:

1. acquire the shared mutation write lock,
2. require the matching experiment to be active and the worktree clean,
3. resolve current `main`, experiment, and merge-base IDs,
4. rebuild comparison status/fingerprint and match all expected values,
5. ensure every selected path is currently changed and promotable,
6. intersect selected paths with paths changed from base to current `main`,
7. return `409 Conflict` with safe relative conflicting paths if non-empty,
8. snapshot the exact current `main` blobs/modes for selected paths from Git.

Then:

9. switch to `main`,
10. apply selected experiment blobs and deletions without staging unrelated
    paths,
11. run strict full-project canonical validation,
12. rebuild the disposable index,
13. stage exactly selected paths,
14. create one promotion commit,
15. verify `main` is clean and return the new main head.

Commit text is:

```text
Promote what-if brn_0123456789abcdef0123

Storywork-Experiment-ID: brn_0123456789abcdef0123
Storywork-Source-Commit: <full experiment object id>
Storywork-Base-Commit: <full merge-base object id>
```

Validate all values before checkout. Do not include experiment display names,
goals, paths, prompt text, or model output in commit metadata. Git's commit tree
is the authoritative promoted-path record.

On failure after switching to `main`, restore selected paths to the captured
main blobs/deletions and modes, unstage app-created changes, rebuild the index,
and preserve the original `main` head. Join rollback failures with the original
error. Do not use reset/force operations as ordinary application control flow.
If commit succeeds, do not attempt to switch back automatically.

## Canonical validation touchup

The current index rebuild hashes recognized files but does not prove structural
or semantic validity. Promotion therefore must not treat a successful index
rebuild as validation.

Add one focused `internal/projectcheck` composition. It must reuse read-only
validators exposed by the packages that own each format; it must not duplicate
their YAML schemas. Verify at least:

- project metadata and outline parse strictly,
- every referenced arc/chapter/scene file exists and matches its ID/parent,
- no duplicate IDs or outline references exist,
- scene front matter and size are valid,
- all Codex entries and progression documents parse strictly,
- progression entry and scene anchors exist,
- agent/style registry files parse under supported versions,
- import review/raw manifests remain valid where existing readers support them.

Do not make `internal/branch` understand YAML schemas. It depends on a narrow
`CanonicalValidator`. `internal/projectcheck` may compose `storyfile`, `agent`,
and importer-owned read-only validators because it sits above those packages;
those lower-level packages must not import `projectcheck`. If an existing owner
cannot validate an already-supported canonical family without mutation, add the
smallest read-only adapter in that owner. Do not redesign import or story
mutation services.

## HTTP API

Preserve all existing routes. Add:

```http
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

Create body:

```json
{"name":"Obi-Wan lives"}
```

Switch body:

```json
{"target":"main"}
```

or:

```json
{"target":"brn_0123456789abcdef0123","expected_head":"<full object id>"}
```

Comparison response:

```json
{
  "experiment_id":"brn_0123456789abcdef0123",
  "branch_name":"branch/obi-wan-lives-0123456789abcdef0123",
  "main_head":"<full object id>",
  "experiment_head":"<full object id>",
  "base_head":"<full object id>",
  "fingerprint":"sha256:...",
  "files":[{"path":"scenes/scn_....md","status":"modified"}]
}
```

File response:

```json
{
  "path":"scenes/scn_....md",
  "status":"modified",
  "main_head":"<full object id>",
  "experiment_head":"<full object id>",
  "fingerprint":"sha256:...",
  "canon":{"exists":true,"text":"..."},
  "experiment":{"exists":true,"text":"..."}
}
```

Promotion body:

```json
{
  "paths":["scenes/scn_....md"],
  "expected_main_head":"<full object id>",
  "expected_experiment_head":"<full object id>",
  "comparison_fingerprint":"sha256:..."
}
```

Discard body:

```json
{"expected_experiment_head":"<full object id>"}
```

Status mapping:

- `400 Bad Request`: malformed strict JSON/query, invalid name/ref/ID/path,
  duplicate/empty path selection, unsupported Git state, or analysis budget.
- `404 Not Found`: valid absent experiment or comparison path.
- `409 Conflict`: no active project, dirty worktree, stale ref/fingerprint,
  detached/unmanaged active branch, selected path changed on `main`, invalid
  promotion subset, or branch deletion/switch conflict.
- `413 Request Entity Too Large`: HTTP body or comparison blob exceeds its
  documented limit.
- `502 Bad Gateway`: provider rejects or returns invalid output.
- `503 Service Unavailable`: provider unavailable or timeout/cancellation.
- `500 Internal Server Error`: malformed repository/canonical state, index,
  Git, filesystem, commit, validation, or rollback failure.

Use recursive strict JSON, exact required fields, bounded bodies, exact query
keys, method-specific `Allow`, safe errors, and arrays rather than `null`. Never
return absolute project paths, raw command lines, credentials, or prompt text.

## Frontend behavior

Add a top-level Branches workspace without making the scene editor own Git
state. It includes:

- active branch and explicit `Canon`/`Experiment` badge,
- deterministic experiment list,
- create, switch, compare, analyze, promote, and discard controls,
- changed-file list with status text and selection checkboxes,
- read-only side-by-side file comparison,
- explicit promotion summary listing whole selected files,
- conflict messages naming only safe project-relative paths,
- ramification summary/findings grouped by severity/category,
- confirmation before branch switch, promotion, or discard,
- clear notice that analysis does not edit files,
- stale-response protection keyed by project, experiment, heads, fingerprint,
  and selected path.

If any editor/Codex/import form is dirty, branch-changing controls require the
existing discard confirmation pattern or remain disabled. A confirmed switch
discards only browser-local unsaved state; it never asks Git to overwrite a
dirty worktree. After successful branch change, reset selected scene, loaded
revisions, action previews/runs/invitations, Codex forms, import candidate
drafts, comparison text, and ramification findings before refetching outline and
branch status.

Do not persist branch comparison text, goals, or findings in browser storage.

## Design versus implementation review

The deliberate pre-Milestone 8 review found these clashes and resolutions:

1. **Old design implied Git branches but no canon identity.** Project creation
   already initializes `main`, so the design is adjusted to make existing
   implementation behavior the fixed canon rule rather than adding config.
2. **Old branch-screen wording could imply two editable copies.** The existing
   single active-project session is the better fit. Comparison reads Git blobs
   directly; only one tree is editable.
3. **Git adapter only supports init/checkpoint/cleanliness/ancestry.** Extend the
   concrete adapter with branch/ref/blob/diff/path-limited commit operations,
   while defining narrow consumer interfaces in `internal/branch` rather than
   widening every existing Git interface.
4. **SQLite represents the checked-out tree, not every branch.** Keep one index
   and rebuild it after checkout. Do not add per-branch databases.
5. **Existing story/action mutations assume the active working tree.** This is
   useful: after a synchronized branch switch they work unchanged and commit to
   the active experiment. Add characterization tests instead of branch flags to
   every service.
6. **Existing index rebuild is not canonical validation.** Add a read-only
   `internal/projectcheck` composition for promotion; do not pretend hashing
   files proves integrity or make `storyfile` own every project format.
7. **Shared chat transport is owned by `internal/agent` but extraction already
   consumes it.** With branch analysis as a third consumer, move transport to
   `internal/modelchat` under compatibility tests.
8. **Milestone 7 action scope is intentionally selection/scene/chapter-review.**
   Ramification analysis is a separate branch use case; do not overload tagged
   action targets or action run acceptance.
9. **Commit formatting only understands accepted action lineage.** Add a
   separate typed promotion commit value rather than a permissive arbitrary
   trailer map.
10. **Frontend navigation has feature-local dirty/stale state.** Introduce one
    explicit branch-change invalidation event/state transition rather than
    relying on component remount accidents.

## SOLID evaluation and required refactoring

### Single Responsibility Principle

- `internal/branch` owns branch policy and orchestration; `internal/gitstore`
  executes Git; canonical readers validate story state; frontend diff code only
  aligns lines for presentation.
- Move generic chat wire transport out of `internal/agent`; keeping a third AI
  consumer there would give the agent package multiple unrelated reasons to
  change.
- Keep format validation in each owning package and compose it in
  `internal/projectcheck`; do not turn `story.Service` or `storyfile.Store` into
  an owner of agent/import formats.

### Open/Closed Principle

- Add typed Git operations and promotion metadata without replacing existing
  `CommitAll`/accepted-action behavior.
- Use a branch-owned `Analyzer` interface so a mock or future analyzer can be
  substituted without modifying branch lifecycle policy.
- Do not create generic arbitrary refs, arbitrary trailers, or `map[string]any`
  extension points. Explicit variants keep invalid states closed.

### Liskov Substitution Principle

- Repository fakes and the real Git adapter must agree on clean-tree checks,
  missing blobs, add/delete semantics, expected-head conflicts, and atomic
  failure behavior.
- `modelchat.Completer` implementations must preserve the same provider-neutral
  success/error contract for OpenAI-compatible and Ollama profiles.
- Canonical validator fakes must not silently accept states production rejects;
  integration tests use real readers to prove substitutability.

### Interface Segregation Principle

- `internal/branch` defines focused repository/status/comparison/promotion,
  index, validator, analyzer, clock/ID boundaries as needed.
- Do not widen `project.GitStore`, `story.GitStore`, or `importer.GitStore` with
  branch operations they do not consume.
- Add a cohesive `BranchStore` HTTP dependency rather than putting branch
  methods on `StoryStore` or `ActionStore`.

### Dependency Inversion Principle

- Branch orchestration depends on repository, index, validation, analyzer, ID,
  and coordinator abstractions, not `exec.Cmd`, SQLite, HTTP clients, or YAML.
- AI consumers depend on neutral `modelchat` transport; provider wire shapes
  remain adapters.
- The UI depends on typed API results and pure diff/state functions, not Git
  commands or filesystem access.

Required refactors are limited to the shared modelchat extraction, the
read-only canonical validator seam, cohesive API dependency addition, and
branch-change frontend invalidation. Do not refactor working mutation services,
introduce repositories for every type, or generalize beyond Milestone 8.

## Required test architecture

Write tests before production code for every slice. Every new Milestone 8 test
file begins with:

```text
BDD Scenario: 8.x.y - scenario title
Requirements: M8-Rxx, M8-Ryy
Test purpose: Plain-English observable behavior proved by this file.
```

Every test case has adjacent `Test:` and `Requirements:` comments. Required
layers:

- pure tests for experiment IDs/names/refs, path allowlist, status parsing,
  deterministic ordering/fingerprints, conflict intersections, ramification
  schema, promotion metadata, and frontend line alignment,
- modelchat characterization and adapter tests before and after extraction,
- real Git adapter tests in temporary repositories for create/switch/list,
  direct blob reads, exact tree diff, merge base, selected path application,
  deletion, ref races, and path-limited promotion commits,
- branch service tests with fakes for locking, clean checks, stale refs,
  checkout/index rollback, zero-side-effect comparison/analysis, promotion
  ordering/rollback, and discard,
- canonical validator tests with malformed cross-file fixtures,
- API tests for exact routes, strict bodies/queries, statuses, safe errors,
  method/Allow behavior, and no absolute path leakage,
- frontend transport/state/component tests for dirty guards, stale responses,
  line highlighting, selection, conflicts, confirmation, and invalidation,
- real-adapter acceptance using temporary Git/SQLite projects and local
  `httptest.Server` providers.

The real-adapter acceptance must prove:

1. experiment creation starts exactly at current `main` and changes no main ref;
2. existing scene/Codex/action mutations commit only on the experiment;
3. comparison while the experiment is active reads current main and experiment
   blobs and performs zero checkout/mutation;
4. added, modified, and deleted files render with correct side data;
5. merely opening comparison calls no provider;
6. explicit analysis sends exactly the reviewed bounded packet and returns
   strict findings with zero repository mutation;
7. stale refs/fingerprint stop before provider or checkout;
8. changed-on-main selected paths conflict before checkout;
9. valid selected whole files promote in exactly one main commit;
10. invalid subsets and injected failures restore main/index/staging/history;
11. successful promotion leaves main active, indexed, clean, and experiment ref
    intact;
12. active discard switches/indexes main then deletes only the experiment;
13. shared modelchat behavior remains identical for existing action/extraction;
14. all previous milestone suites remain green under race detection.

## Documentation and status updates

Before marking Milestone 8 complete:

1. Update `docs/02_architecture.md` with `internal/branch`, `internal/modelchat`,
   single-worktree switching, comparison, validation, and promotion boundaries.
2. Update `docs/03_storage_model.md` with fixed `main`, managed experiment refs,
   active-tree index semantics, promotion commits, and transient analysis.
3. Update `docs/04_agent_style_system.md` to distinguish branch analysis from
   editor agent actions and describe neutral chat transport.
4. Mark Milestone 8 complete in `docs/05_milestones.md` only after acceptance
   passes and identify Milestone 9 as next.
5. Update `docs/06_api_contract.md` with exact implemented branch routes.
6. Update `docs/07_frontend_editor.md` with the actual Branches workspace and
   side-by-side accessibility behavior.
7. Add exact named evidence to `docs/08_testing_acceptance.md`.
8. Update `DOCUMENTATION.md` version/API inventory and `README.md` status/package
   map only after implementation passes.
9. Maintain `.plans/milestone_8_status.md` and
   `.plans/milestone_8_test_evidence.md`; planned tests are not evidence.
10. Record every design adjustment, replacement test, warning, migration, and
    residual limitation before final verification.

## Verification

Use `/home/linuxbrew/.linuxbrew/bin/go` first when available:

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

Inspect for generated repositories, extra worktrees, leaked comparison/prompt
text, provider output, credentials, databases, caches, build output, live
servers, stale lock files, and test projects. Do not print environment secret
values during leak checks.

## Definition of done

Milestone 8 is complete only when:

- `main` remains the fixed, protected canon branch,
- experiments start from current main and normal editing works on them,
- the active experiment can be compared against current main without checkout,
- side-by-side UI shows accessible line-level differences,
- comparison and branch navigation never call a provider,
- explicit ramification analysis is bounded, strict, transient, and non-mutating,
- selected whole-file promotion detects canon divergence and invalid subsets,
- promotion is rollback-safe and creates exactly one provenance commit,
- discard cannot modify main and safely handles the active experiment,
- shared chat transport has neutral ownership without provider regressions,
- every requirement and scenario has exact passing automated evidence,
- full regression, race, frontend, build, diff, and artifact checks pass,
- documentation and status records describe actual behavior,
- no Milestone 9 hardening, remote Git, multi-worktree, or automatic merge work
  is implemented.
