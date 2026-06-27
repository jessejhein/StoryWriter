# 06 — Initial API Contract

This is a v0 internal/local API. Keep it boring. Do not over-design authentication in MVP.

## Health

```http
GET /api/health
```

Response:

```json
{
  "status": "ok",
  "version": "0.0.0-dev"
}
```

## Projects

```http
POST /api/projects
```

Request:

```json
{
  "name": "Test Novel",
  "path": "/home/user/Stories/test-novel"
}
```

Response:

```json
{
  "project_id": "proj_test_novel",
  "path": "/home/user/Stories/test-novel",
  "git_initialized": true,
  "index_initialized": true
}
```

```http
POST /api/projects/open
```

Request:

```json
{
  "path": "/home/user/Stories/test-novel"
}
```

## Outline

```http
GET /api/outline
POST /api/arcs
POST /api/chapters
POST /api/scenes
POST /api/outline/reorder
```

Reorder request should use stable IDs, not display numbers.

```json
{
  "parent_type": "chapter",
  "parent_id": "ch_0001",
  "ordered_child_ids": ["scn_0002", "scn_0001"]
}
```

## Scenes

```http
GET /api/scenes/{scene_id}
PUT /api/scenes/{scene_id}
```

Save request:

```json
{
  "title": "The Duel",
  "frontmatter": {
    "pov": "Luke",
    "status": "draft",
    "exclude_from_ai": false
  },
  "markdown": "Scene prose here...",
  "checkpoint": true
}
```

## Codex

```http
GET /api/codex
POST /api/codex
GET /api/codex/{entry_id}
PUT /api/codex/{entry_id}
```

## Progressions

```http
POST /api/codex/{entry_id}/progressions
GET /api/story-state/as-of/{scene_id}
```

## Agents and styles

```http
GET /api/agents
GET /api/styles
GET /api/actions/available?surface=editor&scene_id=scn_0001&selection_words=200
```

Available actions response:

```json
{
  "actions": [
    {
      "agent_id": "local_voice_texture",
      "name": "Local Voice Texture Pass",
      "output_mode": "patch",
      "requires_acceptance": true
    }
  ]
}
```

## Run AI action

```http
POST /api/actions/run
```

Request:

```json
{
  "agent_id": "local_voice_texture",
  "style_id": "dry_modern_fantasy",
  "surface": "editor",
  "scene_id": "scn_0001",
  "selection": {
    "start": 120,
    "end": 640,
    "text": "Selected prose..."
  }
}
```

Response for patch:

```json
{
  "run_id": "run_0001",
  "output_mode": "patch",
  "patch": {
    "original": "Selected prose...",
    "replacement": "Rewritten prose..."
  },
  "context_summary": {
    "packs_used": ["selected_text", "style_sheet"],
    "rag_mode": "none"
  }
}
```

## Accept/reject patch

```http
POST /api/actions/{run_id}/accept
POST /api/actions/{run_id}/reject
```

Accepting a patch writes to canonical files only after explicit author action.

## Import

```http
POST /api/imports/markdown-folder
POST /api/imports/{import_id}/extract
GET /api/review/candidates
POST /api/review/candidates/{candidate_id}/accept
POST /api/review/candidates/{candidate_id}/reject
POST /api/review/candidates/{candidate_id}/merge
```

Extraction creates candidates. It does not directly mutate canon.

## Branches

```http
GET /api/branches
POST /api/branches
GET /api/branches/{branch_name}/diff
POST /api/branches/{branch_name}/promote
POST /api/branches/{branch_name}/discard
```

MVP branch promotion can be manual/coarse. Do not build complex merge UI first.

