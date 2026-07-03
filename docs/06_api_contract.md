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

Outline routes operate on the active project. A successful create/open project
request sets the active project for this backend process. The active project is
not persisted across backend restarts. An outline request without an active
project returns `409 Conflict`.

```http
GET /api/outline
POST /api/arcs
POST /api/chapters
POST /api/scenes
POST /api/outline/reorder
```

Create requests:

```json
{"title":"Act One"}
```

```json
{"arc_id":"arc_0123456789abcdef0123","title":"Arrival"}
```

```json
{"chapter_id":"ch_0123456789abcdef0123","title":"The Station"}
```

Each create response is `201 Created`. Reorder returns `200 OK`. Both return the
changed ID when applicable and the complete outline view:

```json
{
  "changed_id": "scn_0123456789abcdef0123",
  "outline": {
    "version": 1,
    "arcs": [
      {
        "id": "arc_0123456789abcdef0123",
        "title": "Act One",
        "display_label": "Arc 1",
        "chapters": [
          {
            "id": "ch_0123456789abcdef0123",
            "title": "Arrival",
            "display_label": "Chapter 1.1",
            "scenes": [
              {
                "id": "scn_0123456789abcdef0123",
                "title": "The Station",
                "display_label": "Scene 1.1.1"
              }
            ]
          }
        ]
      }
    ]
  }
}
```

`GET /api/outline` returns the `outline` object itself, not the mutation wrapper.

Reorder request should use stable IDs, not display numbers.

```json
{
  "parent_type": "chapter",
  "parent_id": "ch_0001",
  "ordered_child_ids": ["scn_0002", "scn_0001"]
}
```

`parent_type` is `arc` when reordering chapters and `chapter` when reordering
scenes. `ordered_child_ids` must be an exact permutation of the parent's existing
children: no missing, duplicate, foreign, or unknown IDs. Reordering arcs and
moving a child to another parent are out of scope for Milestone 1.

Milestone 1 status rules:

- `400 Bad Request`: malformed JSON, unknown fields, invalid title, invalid ID
  shape, or an invalid reorder permutation.
- `404 Not Found`: a well-formed parent ID does not exist.
- `409 Conflict`: no active project, dirty story-project Git worktree, or a
  checkpoint conflict.
- `500 Internal Server Error`: file, index, Git executable, or rollback failure.

All errors retain the existing JSON shape: `{"error":"useful message"}`.

## Scenes

Scene routes operate on the active project and use stable scene IDs. See
`docs/11_milestone_2_task_prompt.md` for the complete Milestone 2 contract.

```http
GET /api/scenes/{scene_id}
PUT /api/scenes/{scene_id}
```

Load response:

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
  "markdown": "Scene prose here...",
  "revision": "sha256:7d6b9b5f..."
}
```

Save request uses the revision returned by the most recent load or save:

```json
{
  "title": "The Duel",
  "frontmatter": {
    "pov": "Luke",
    "status": "draft",
    "exclude_from_ai": false
  },
  "markdown": "Scene prose here...",
  "expected_revision": "sha256:7d6b9b5f..."
}
```

A successful save returns the same shape as the load response with the new
revision. Every successful explicit save creates exactly one Git commit. A stale
`expected_revision` returns `409 Conflict` without changing files, the index, or
Git history.

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

## Provider profiles

Provider settings do not require an active project.

```http
GET /api/provider-profiles
PUT /api/provider-profiles
```

Other methods return `405 Method Not Allowed` with `Allow: GET, PUT`. A PUT
body larger than 1 MiB returns `413 Request Entity Too Large`.

Configured response:

```json
{
  "profiles": [
    {
      "id": "hosted_api",
      "name": "Hosted API",
      "type": "openai_compatible",
      "base_url": "https://api.example.test/v1",
      "auth": {"type": "bearer_env", "credential_env": "STORYWORK_HOSTED_API_KEY"},
      "capabilities": {
        "chat": true,
        "streaming": false,
        "structured_output": false,
        "max_context_tokens": 32768
      },
      "readiness": "ready"
    }
  ],
  "revision": "sha256:..."
}
```

Missing configuration returns:

```json
{"profiles":[],"revision":null}
```

## Import review

Milestone 6 adds a strict active-project import/review API. All routes return
JSON errors with the existing `{"error":"..."}` shape. Other methods return
`405 Method Not Allowed` with `Allow`.

```http
POST /api/imports
GET  /api/imports
GET  /api/imports/{import_id}
GET  /api/imports/{import_id}/chunks
POST /api/imports/{import_id}/extractions
GET  /api/import-candidates?status=pending&kind=codex
GET  /api/import-candidates/{candidate_id}
PUT  /api/import-candidates/{candidate_id}
POST /api/import-candidates/{candidate_id}/merge
POST /api/import-candidates/{candidate_id}/discard
POST /api/import-candidates/{candidate_id}/accept
```

Create import request:

```json
{"source_directory":"/absolute/path/to/notes"}
```

Create import response:

```json
{
  "import": {
    "id": "imp_0123456789abcdef0123",
    "created_at": "2026-06-30T12:00:00Z",
    "file_count": 1,
    "total_bytes": 12
  },
  "files": [
    {
      "path": "notes/characters.md",
      "bytes": 12,
      "sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    }
  ]
}
```

List imports returns `{"imports":[...]}`. No response includes an external
source path.

Chunk response:

```json
{
  "chunks": [
    {
      "id": "chk_0123456789abcdef0123",
      "import_id": "imp_0123456789abcdef0123",
      "source_path": "notes/characters.md",
      "start_line": 1,
      "end_line": 2,
      "text": "# Characters\nMara\n",
      "sha256": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    }
  ]
}
```

Extraction request:

```json
{
  "chunk_ids": ["chk_0123456789abcdef0123"],
  "mode": "structure",
  "profile_id": "local_ollama",
  "model": "qwen2.5:7b"
}
```

Extraction response:

```json
{
  "candidates": [
    {
      "id": "cand_0123456789abcdef0123",
      "kind": "codex",
      "proposal_version": 1,
      "status": "pending",
      "revision": "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
      "provenance": {"chunk_ids": ["chk_0123456789abcdef0123"]},
      "proposal": {
        "type": "character",
        "name": "Mara Venn",
        "aliases": ["Mara"],
        "tags": ["pilot"],
        "description": "A cautious salvage pilot."
      },
      "replacement_candidate_id": null,
      "canonical_refs": []
    }
  ],
  "provider": {
    "profile_id": "local_ollama",
    "type": "ollama",
    "model": "qwen2.5:7b"
  }
}
```

Candidate list and load responses use the same candidate object shape. Lists
always use arrays, never `null`.

Candidate edit request:

```json
{
  "proposal": {
    "type": "character",
    "name": "Mara Venn",
    "aliases": ["Mara"],
    "tags": ["pilot"],
    "description": "Edited author text."
  },
  "expected_revision": "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
}
```

Merge response:

```json
{
  "candidate": {/* merged candidate */},
  "merged_candidate_ids": [
    "cand_0123456789abcdef0123",
    "cand_abcdef0123456789abcd"
  ]
}
```

Accept response:

```json
{
  "candidate": {/* accepted candidate */},
  "canonical_refs": [{"kind": "codex", "id": "char_0123456789abcdef0123"}]
}
```

Import-review status rules:

- `400 Bad Request`: malformed JSON, invalid path/ID/filter, empty import,
  invalid chunk selection, invalid merge request, or invalid generated data.
- `404 Not Found`: import or candidate ID is well formed but absent.
- `409 Conflict`: no active project, dirty worktree, stale revision,
  non-pending candidate, duplicate claim, or unaccepted parent candidate.
- `413 Request Entity Too Large`: request body exceeds 1 MiB.
- `502 Bad Gateway`: provider rejects or returns invalid output.
- `503 Service Unavailable`: provider profile is unavailable or invalid at run
  time.
- `500 Internal Server Error`: filesystem, index, Git, rollback, or unexpected
  adapter failure.

## Agents and styles

Milestone 5 extends the Milestone 4 registry and action contract. Agent, style,
and action routes still require an active project.

```http
GET /api/agents
GET /api/styles
GET /api/actions/available?surface=editor&input_scope=selection&scene_id=scn_0001&selection_words=200
```

Agent list response:

```json
{
  "agents": [
    {
      "id": "line_polish",
      "name": "Line Polish",
      "description": "Rewrite selected prose for clarity, cadence, and flow while preserving meaning.",
      "surfaces": ["editor"],
      "input_scopes": ["selection"],
      "min_words": 20,
      "max_words": 1500,
      "required_context": ["selected_text", "style_sheet"],
      "optional_context": ["surrounding_paragraphs"],
      "forbidden_context": ["global_codex_rag", "raw_import_notes"],
      "rag_mode": "none",
      "output_mode": "patch",
      "requires_acceptance": true
    }
  ]
}
```

Style list response:

```json
{
  "styles": [
    {
      "id": "precise_editor",
      "version": 1,
      "name": "Precise Editor",
      "provider_profile_id": "mock_default",
      "model": "mock",
      "temperature": 0.2,
      "system_prompt": "You are a careful prose editor.",
      "provider_readiness": "ready"
    }
  ]
}
```

Available actions response:

```json
{
  "actions": [
    {
      "agent_id": "line_polish",
      "name": "Line Polish",
      "description": "Rewrite selected prose for clarity, cadence, and flow while preserving meaning.",
      "output_mode": "patch",
      "requires_acceptance": true,
      "style_ids": ["precise_editor"]
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
  "agent_id": "line_polish",
  "style_id": "precise_editor",
  "surface": "editor",
  "input_scope": "selection",
  "scene_id": "scn_0001",
  "scene_revision": "sha256:...",
  "selection": {
    "start_byte": 120,
    "end_byte": 640,
    "text": "Selected prose..."
  }
}
```

Response for patch:

```json
{
  "run_id": "run_0123456789abcdef0123",
  "status": "pending",
  "agent_id": "line_polish",
  "style_id": "precise_editor",
  "scene_id": "scn_0001",
  "scene_revision": "sha256:...",
  "selection": {
    "start_byte": 120,
    "end_byte": 640
  },
  "output_mode": "patch",
  "patch": {
    "original": "Selected prose...",
    "replacement": "Mock polished: Selected prose..."
  },
  "context_summary": {
    "packs_used": ["selected_text", "style_sheet"],
    "rag_mode": "none"
  },
  "provider": {
    "profile_id": "mock_default",
    "type": "openai_compatible",
    "model": "mock"
  }
}
```

## Accept/reject patch

```http
POST /api/actions/{run_id}/accept
POST /api/actions/{run_id}/reject
```

Accept request:

```json
{"expected_revision":"sha256:..."}
```

Reject requests have no body. Unknown or duplicate availability query keys,
missing required selection fields, malformed IDs/revisions, and any reject body
are rejected with `400 Bad Request` before the action service is called.

Accepting a patch writes to canonical files only after explicit author action.

Milestone 4 status rules:

- `400 Bad Request`: malformed query or JSON, invalid registry authoring on an explicit run, invalid byte range, selected-text mismatch, inapplicable agent, incompatible style, or no-op replacement.
- `404 Not Found`: missing agent, style, scene, or run.
- `409 Conflict`: no active project, dirty worktree, stale revision, or a non-pending run decision.
- `503 Service Unavailable`: transient run capacity exhaustion or provider cancellation/unavailability.
- `500 Internal Server Error`: malformed canonical registry state discovered during list/availability, filesystem/index/Git failure, or rollback failure.

## Milestone 7 context preview and tagged runs

```http
POST /api/actions/context-preview
POST /api/action-invitations/{invitation_id}/run
```

`POST /api/actions/run` accepts the Milestone 4 selection body and a strict
tagged body:

```json
{
  "agent_id": "scene_rewrite",
  "style_id": "precise_editor",
  "scope": "scene",
  "target": {"scene_id": "scn_...", "scene_revision": "sha256:..."}
}
```

Context preview uses the same request shape and returns:

```json
{
  "manifest": {
    "scope": "scene",
    "packs_used": ["current_scene", "style_sheet", "active_codex_at_position"],
    "packs_omitted": [],
    "estimated_input_tokens": 1200,
    "max_input_estimated_tokens": 12000,
    "rag_mode": "timeline_aware",
    "active_codex": [{"entry_id": "char_...", "applied_progression_ids": ["prog_..."]}]
  },
  "target_revision": "sha256:..."
}
```

For `chapter_review`, preview treats the supplied well-formed fingerprint as an
advisory token and returns the current fingerprint in `target_revision`. This
lets the UI obtain a coherent read token without a provider call. The subsequent
run or invitation run must send that returned fingerprint and fails on mismatch.

Preview performs no provider call and returns no packet prose.

Tagged run responses add `scope`, `parent_run_id`, `root_run_id`, `chain_depth`,
and `manifest`. Scene patch runs include full-scene `patch.original` and
`patch.replacement`. Chapter review runs use `output_mode: "suggestion"` with
`findings` and no patch.

Patch accept responses add `follow_up_invitations` (always an array). Invitation
run request:

```json
{"style_id": "precise_editor", "expected_target_revision": "sha256:..."}
```

Milestone 7 additions to status rules:

- `400 Bad Request`: malformed tagged target, context budget overflow,
  invalid invitation ID, invalid suggestion JSON.
- `409 Conflict`: stale target revision/fingerprint, consumed invitation,
  lineage conflict.
- `502 Bad Gateway`: provider returns invalid structured suggestion output.

Invitations expire 30 minutes after creation. Expired and already claimed or
consumed invitation runs return `409 Conflict` without a provider call.

`GET /api/actions/available` accepts `input_scope` values `selection`, `scene`,
`chapter`, and `chapter_review`.

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
