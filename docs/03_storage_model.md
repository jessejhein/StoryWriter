# 03 — Storage Model

## Core rule

Canonical project state lives in human-readable files tracked by Git.

SQLite is a rebuildable index/cache. If `.storywork/index.sqlite` is deleted, the app should rebuild it from project files.

## Project folder layout

```text
my-novel/
├── .git/
├── project.yaml
├── outline.yaml
├── arcs/
│   └── arc_0001.yaml
├── chapters/
│   └── ch_0001.yaml
├── scenes/
│   └── scn_0001.md
├── codex/
│   ├── characters/
│   │   └── char_obi_wan.yaml
│   ├── locations/
│   ├── lore/
│   └── custom/
├── progressions/
│   └── char_obi_wan.yaml
├── agents/
│   └── line_polish.yaml
├── styles/
│   └── dry_modern_fantasy.yaml
├── imports/
│   ├── raw/
│   └── review/
└── .storywork/
    ├── import/
    ├── index.sqlite
    ├── embeddings.sqlite
    └── tmp/
```

## Git rules

Tracked:

- `project.yaml`
- `outline.yaml`
- `arcs/`
- `chapters/`
- `scenes/`
- `codex/`
- `progressions/`
- `agents/`
- `styles/`
- `imports/raw/` for tracked copy-only Markdown snapshots and manifests
- `imports/review/` for durable extraction proposals and author decisions

Ignored:

- `.storywork/*.sqlite`
- `.storywork/tmp/`
- `.storywork/import/`
- credentials
- local app settings
- generated exports unless user chooses to track them

## Scene file format

Use Markdown with YAML front matter.

```markdown
---
id: scn_0001
title: The Duel
chapter_id: ch_0001
pov: Luke
status: draft
exclude_from_ai: false
---

Scene prose starts here.
```

`outline.yaml` is the only ordering authority. Do not copy a mutable numeric
position into arc, chapter, or scene files. Display labels such as "Chapter 2"
are derived from the current outline order and are not canonical state.

## Milestone 1 structure file formats

Arc files use this exact schema:

```yaml
version: 1
id: arc_00000000000000000000
title: The First Crossing
```

Chapter files use this exact schema:

```yaml
version: 1
id: ch_00000000000000000000
arc_id: arc_00000000000000000000
title: The Map Room
```

New scene files contain YAML front matter and an empty Markdown body:

```markdown
---
id: scn_00000000000000000000
title: The Locked Cabinet
chapter_id: ch_00000000000000000000
pov: ""
status: draft
exclude_from_ai: false
---

```

The IDs above demonstrate shape only. Production IDs are random and stable;
tests inject deterministic IDs. Structure readers must reject duplicate IDs,
missing referenced files, parent mismatches, unsupported versions, and malformed
YAML with a useful error. Reordering changes only `outline.yaml`.

## Outline file

`outline.yaml` stores ordered references, not full content blobs.

```yaml
version: 1
root:
  arcs:
    - id: arc_0001
      chapters:
        - id: ch_0001
          scenes:
            - id: scn_0001
            - id: scn_0002
```

## Codex entry format

```yaml
id: char_obi_wan
type: character
name: Obi-Wan Kenobi
aliases:
  - Ben
  - Old Ben
tags:
  - mentor
  - jedi
description: >
  A former Jedi acting as Luke's guide and moral anchor.
metadata:
  status: alive
  role: mentor
```

## Progression format

Progressions must anchor to stable IDs, not display chapter numbers.

```yaml
entry_id: char_obi_wan
progressions:
  - id: prog_0001
    anchor:
      type: scene
      id: scn_0007
      timing: after
    changes:
      metadata:
        status: deceased
      description_patch: >
        After the duel, Obi-Wan is no longer physically present, but his
        influence continues through memory, teaching, and possible spiritual guidance.
```

Display text may say "After Chapter 3", but storage must reference `scn_0007` or another stable ID.

## Branch storage

Use Git branches for what-if branches.

Example branch names:

```text
branch/obiwan-dies
branch/obiwan-lives
branch/yumina-politics-heavier
```

Branch actions:

- create branch from current canon,
- generate/edit files inside branch,
- compare branch to canon with Git diff,
- manually promote selected changes to canon.

MVP does not need automatic complex merge/cherry-pick UI. Manual promotion is acceptable.

## SQLite index responsibilities

SQLite stores derived/query state:

- file manifest,
- parsed scene metadata,
- outline order cache,
- Codex search index,
- mention index,
- active progression cache,
- import chunks and transient extraction-attempt data,
- agent run logs,
- embeddings/vector data if implemented.

SQLite must not be the only copy of canonical prose, Codex, outline, agents, or
styles.
It must also not be the only copy of a review candidate or author review
decision. Milestone 6 stores those durable proposals under `imports/review/`;
only reproducible chunks and transient attempt data belong in `.storywork/`.

## Milestone 6 import storage

Milestone 6 uses three distinct storage areas:

- `imports/raw/<import_id>/manifest.yaml` plus `files/` contain the canonical
  imported Markdown snapshot. The stored paths are normalized import-relative
  paths only; no external source directory is retained.
- `.storywork/import/<import_id>/` contains rebuildable derived chunk data and
  successful extraction-attempt metadata. Attempt records contain only mode,
  selected chunk IDs, resulting candidate IDs, and non-secret provider identity;
  raw provider output is never retained. Deleting this tree is safe because it
  is ignored by Git and does not contain canonical author work.
- `imports/review/<candidate_id>.yaml` contains durable review candidates,
  revisions, provenance, and terminal decisions. These are tracked because they
  are author work product, but they are still proposals and not canon.

## Application provider configuration

Provider profiles are application configuration, not story-project state. The
single canonical document is `<config-dir>/providers.yaml`, where the config
directory is `STORYWORK_CONFIG_DIR` or the platform user-config directory plus
`storywork`. It is atomically replaced with optimistic SHA-256 revisions and
conservative directory/file permissions.

The document contains endpoint URLs, capability declarations, and environment
variable names only. Credential values never enter this file, a story folder,
Git, SQLite, browser storage, or transient action runs. Deleting a story index
or switching projects does not alter provider configuration.

## Minimal initial tables

Milestone 0 can start with:

```sql
CREATE TABLE schema_version (
  version INTEGER NOT NULL
);

CREATE TABLE project_manifest (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

CREATE TABLE files (
  path TEXT PRIMARY KEY,
  kind TEXT NOT NULL,
  content_hash TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
```

Later milestones add story, Codex, progressions, imports, and agent run logs.

## Accepted action operation trailers (Milestone 7)

Accepted AI patch operations may attach portable Git commit trailers with stable
operation IDs. Authoritative trailer text lives in Git commit bodies; SQLite may
index trailers for lookup but must not be the only copy of causal relationships.

Implemented trailers:

- `Storywork-Operation-ID`
- `Storywork-Scope`
- `Storywork-Triggered-By` (causal parent)
- `Storywork-Depends-On` (semantic dependency; omitted for trigger-only children)

Trailers are validated before write. Invalid IDs, injection, cycles, or
self-dependencies are rejected before canonical bytes change. Git notes are not
used for operation lineage in Milestone 7.
