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
│   └── processed/
└── .storywork/
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
- `imports/raw/` when the user chooses to preserve source notes

Ignored:

- `.storywork/*.sqlite`
- `.storywork/tmp/`
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
position: 10
pov: Luke
status: draft
exclude_from_ai: false
---

Scene prose starts here.
```

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
- import chunks,
- extraction candidates,
- agent run logs,
- embeddings/vector data if implemented.

SQLite must not be the only copy of canonical prose, Codex, outline, agents, or styles.

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

