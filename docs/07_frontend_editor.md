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
- Reorder via drag/drop.
- Open selected scene.

### Scene editor

- CodeMirror 6.
- Vim keybindings enabled by user setting, default on for this project.
- Save/reload scene.
- Selection-based AI action menu.
- Diff preview for patch outputs.

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

### Import review screen

- Show imported files.
- Show extraction candidates.
- Approve/edit/merge/discard.

### Branch screen

- Show current branch.
- Create what-if branch.
- Compare branch to canon.
- Promote/discard manually.

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

## Diff/accept behavior

For MVP, a simple side-by-side or inline preview is enough.

Must support:

- accept replacement,
- reject replacement,
- copy replacement manually,
- show context packs used.

## Do not build yet

- full WYSIWYG rich text,
- mobile polish,
- collaborative cursors,
- command palette unless trivial,
- Firenvim integration.

