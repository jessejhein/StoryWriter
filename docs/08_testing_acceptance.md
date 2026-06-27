# 08 — Testing and Acceptance

## Backend test conventions

Use standard Go tests.

Minimum commands:

```bash
go fmt ./...
go vet ./...
go test ./...
```

Use table-driven tests for decision logic.

Use `t.TempDir()` for project folder and Git/SQLite tests.

Use fakes for model providers.

## Frontend test conventions

Use Vitest for unit/component tests once frontend exists.

Commands:

```bash
npm run lint
npm run typecheck
npm test -- --run
```

## Tests required by milestone

### Milestone 0

Backend:

- Create project writes `project.yaml`.
- Create project initializes Git repo.
- Create project creates `.storywork/index.sqlite`.
- Opening valid project succeeds.
- Opening invalid project returns useful error.
- Index rebuild is idempotent.
- Health endpoint returns ok.

Manual:

1. Start backend.
2. Start frontend.
3. Create project from UI or API.
4. Confirm files on disk.
5. Confirm Git repo exists.
6. Confirm index exists.

### Milestone 1

- Create arc/chapter/scene.
- Reorder preserves stable IDs.
- Display labels update after reorder.
- Files serialize/deserialize correctly.
- Git checkpoint occurs after structural change.

### Milestone 2

- Load scene Markdown.
- Save scene Markdown.
- Preserve YAML front matter.
- Reload after save.
- Vim mode manual smoke test.

### Milestone 3

- Add Codex entry.
- Add progression anchored after scene.
- Active state before anchor excludes progression.
- Active state after anchor includes progression.
- Reordering scenes does not break anchor.

### Milestone 4

- Agent applicability filters by surface.
- Agent applicability filters by selection size.
- Context policy forbids global RAG for line polish.
- Mock provider patch requires acceptance.
- Reject leaves scene unchanged.
- Accept updates scene file.

### Milestone 5

- Provider interface can run fake provider.
- Local endpoint adapter maps requests correctly.
- API-key provider path does not store key in project folder.
- Capability matrix hides incompatible agents.

### Milestone 6

- Markdown folder import finds `.md` files.
- Imported notes are chunked.
- Extraction produces candidates.
- Candidates do not mutate canon until accepted.
- Merge candidate combines aliases/tags correctly.

### Milestone 7

- Context builder uses active Codex state.
- Same character has different context before/after progression.
- Paragraph action uses minimal context.
- Chapter action can use timeline-aware RAG.

### Milestone 8

- Create Git branch.
- Edit branch without changing canon branch.
- Diff branch against canon.
- Discard branch safely.
- Promote selected file manually.

### Milestone 9

- Project backup snapshot works.
- SQLite index rebuild works after deletion.
- Markdown export works.
- AI run log shows context packs/prompt summary.

## MVP acceptance script

1. Create a project.
2. Import a folder of Markdown notes.
3. Extract candidates.
4. Approve at least one Codex entry.
5. Create an outline with at least one arc, chapter, and scene.
6. Edit the scene in the Vim-friendly editor.
7. Add a timeline progression to a Codex entry.
8. Confirm active Codex differs before/after the progression.
9. Select a paragraph and run a style/agent patch.
10. Accept the patch.
11. Create a what-if branch.
12. Change a scene in the branch.
13. Compare branch to canon.
14. Promote or discard the branch.
15. Rebuild index.
16. Export Markdown.

Pass only if the author remains in control at every AI mutation point.

