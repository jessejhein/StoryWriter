# 07 — Frontend and Editor

## MVP frontend choice

Use Vite + React + TypeScript.

Reasons:

- Go backend owns API and local storage.
- SPA is enough for MVP.
- Easier future Electron/Tauri packaging.
- CodeMirror 6 integrates cleanly.

## Required UI areas

### Project screen

- Create project.
- Open project.
- Show current project path/status.

### Outline screen

- Tree/list: arcs -> chapters -> scenes.
- Create arc/chapter/scene.
- Reorder chapters/scenes with keyboard-operable controls.
- Drag/drop may be added as an enhancement, not as the only reorder control.
- Open selected scene.

### Scene editor

- CodeMirror 6.
- Vim keybindings enabled by user setting, default on for this project.
- Save/reload scene.
- Selection-based AI action menu with context preview before Run.
- Scene-scoped Scene Rewrite and chapter-scoped Chapter Review actions.
- Redacted context manifest (packs used/omitted, estimates, active Codex IDs).
- Diff preview for selection and full-scene patch outputs.
- Dirty drafts disable preview, action lookup, and run controls.
- Scope-broadening confirmation when moving from selection to scene/chapter scope.
- Follow-up invitation cards that require an explicit Run (never auto-execute).
- Chapter Review findings grouped by scene ID without accept-prose controls.
- Accepting a patch updates the editor baseline from the returned canonical scene without a second scene save.

### Codex screen

- List entries.
- Create/edit entry.
- Aliases/tags.
- Progressions anchored to scene/chapter/event IDs.
- "Codex as of this scene" view later.

### Agent/style screen

- List agents.
- List styles.
- Show model/provider profile and capabilities.
- Show which agents apply to current state.

### Provider settings screen

- Reachable before or after opening a project.
- Load application-level provider profiles and public readiness.
- Add, edit, and remove non-secret profiles.
- Explain that bearer keys must come from named backend environment variables.
- Keep dirty-draft navigation protection and conflict reload handling.

### Import review screen

- Require an active project.
- Enter an absolute source directory and create an import snapshot.
- Show imports, imported files, and deterministic chunks with provenance.
- Choose a ready provider/model and run structure extraction.
- Show candidate status, provenance, and editable proposal fields.
- Save edits, merge compatible pending candidates, discard, or accept.
- Confirm before losing dirty candidate edits.

### Branches workspace (Milestone 8)

Top-level navigation; the scene editor does not own Git state.

- active branch badge with explicit `Canon` or `Experiment` label,
- deterministic managed experiment list from `GET /api/branches`,
- create, switch, compare, analyze, promote, and discard controls,
- changed-file list with `added`/`modified`/`deleted` status text and whole-file
  promotion checkboxes,
- read-only side-by-side comparison with `Canon (main)` and `Experiment` panes,
  line numbers, textual added/deleted/modified indicators, independent horizontal
  scrolling, and practical synchronized vertical scrolling for aligned rows,
- explicit ramification summary/findings grouped by severity and category with an
  advisory notice and no accept/apply controls,
- confirmation before branch switch, promotion, or discard,
- dirty browser-draft guard using the existing discard-confirmation pattern;
  confirmed switch clears only browser-local unsaved state and never asks Git to
  overwrite a dirty worktree,
- stale-response protection keyed by project, experiment, both heads,
  comparison fingerprint, and selected path,
- after successful branch change, reset editor, Codex, import-review, action
  preview/run/invitation, comparison text, and ramification findings before
  refetching outline and branch status.

Branch comparison text, goals, and findings are not persisted in browser
storage. Inputs beyond the frontend diff bound show both complete texts with an
explicit message that line highlighting is unavailable.

## Selection AI flow

1. User selects text in CodeMirror.
2. UI computes current state:
   - surface,
   - scene ID,
   - selection text,
   - word/token estimate.
3. UI calls `/api/actions/available`.
4. UI shows only applicable agents.
5. User selects agent/style.
6. UI calls `/api/actions/run`.
7. UI shows patch/diff/proposal.
8. User accepts/rejects.

The implemented Milestone 4 editor flow keeps selection state in the scene
editor, converts CodeMirror character offsets to UTF-8 byte offsets for the run
request, lists only applicable actions, exposes the matching styles, and shows
context packs plus RAG mode in the preview. When a preview opens, the named
inline preview region receives one-time programmatic focus so keyboard and
screen-reader users land on the review state immediately without turning the
workflow into a modal dialog.

## Diff/accept behavior

For MVP, a simple side-by-side or inline preview is enough.

Must support:

- accept replacement,
- reject replacement,
- copy replacement manually,
- show context packs used.

The current implementation uses an inline preview region with side-by-side
original and replacement text, whitespace-preserving `<pre>` blocks, and
keyboard-operable buttons for Copy, Accept, and Reject. Navigation confirmation
remains tied to dirty authored drafts that would lose unsaved user work, not to
a discardable clean preview. Milestone 5 adds non-secret provider identity
(profile ID, provider type, model) to that preview while continuing to hide
endpoint URLs and credential references.

The Milestone 6 import review workbench follows the same dirty-draft rule: it
keeps import/review navigation inside `web/src/imports/`, requires confirmation
before switching candidates with unsaved edits, preserves the local draft across
409 conflict responses, and renders terminal accepted/merged/discarded
candidates as visibly non-editable. Chunk text is inspectable in keyboard-
operable disclosures; kind/status filters expose visible and total counts;
accept and discard require confirmation; conflicts offer an explicit server
reload; dirty drafts install `beforeunload` protection; and stale chunk
responses are ignored after import selection changes.

## Do not build yet

- full WYSIWYG rich text,
- mobile polish,
- collaborative cursors,
- command palette unless trivial,
- Firenvim integration.
