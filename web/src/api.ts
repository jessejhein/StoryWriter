/**
 * api.ts
 *
 * Defines the frontend API contract and small fetch helpers for the local
 * Storywork backend. All exported types mirror the JSON payloads exchanged
 * between the React workbenches and the Go HTTP API.
 */

/** Health is the backend status payload returned by `/api/health`. */
export type Health = { status: string; version: string }

/** Project is the active-project summary returned by create and open routes. */
export type Project = {
  project_id: string
  name?: string
  path: string
  git_initialized: boolean
  index_initialized: boolean
}

/** OutlineScene is one stable scene row in the nested outline response. */
export type OutlineScene = {
  id: string
  title: string
  display_label: string
}

/** Chapter is one ordered chapter node in the outline response. */
export type Chapter = {
  id: string
  title: string
  display_label: string
  scenes: OutlineScene[]
}

/** Arc is one ordered top-level outline node in the outline response. */
export type Arc = {
  id: string
  title: string
  display_label: string
  chapters: Chapter[]
}

/** Outline is the full hierarchical story structure returned by the backend. */
export type Outline = {
  version: number
  arcs: Arc[]
}

/** OutlineMutation wraps the changed outline after one structural mutation. */
export type OutlineMutation = {
  changed_id?: string
  outline: Outline
}

/** ReorderRequest reorders direct children under one stable parent. */
export type ReorderRequest = {
  parent_type: 'arc' | 'chapter'
  parent_id: string
  ordered_child_ids: string[]
}

/** SceneFrontMatter contains the editable canonical scene metadata. */
export type SceneFrontMatter = {
  pov: string
  status: 'draft' | 'revised' | 'final'
  exclude_from_ai: boolean
}

/** SceneDocument is the editor-facing canonical scene payload. */
export type SceneDocument = {
  id: string
  chapter_id: string
  title: string
  frontmatter: SceneFrontMatter
  markdown: string
  revision: string
}

/** SaveSceneRequest is the explicit scene-save payload sent to the backend. */
export type SaveSceneRequest = {
  title: string
  frontmatter: SceneFrontMatter
  markdown: string
  expected_revision: string
}

/** CodexEntryType enumerates the supported Codex entry categories. */
export type CodexEntryType = 'character' | 'location' | 'lore' | 'custom'

/** CodexEntry is one canonical Codex entry plus its optimistic-lock revision. */
export type CodexEntry = {
  id: string
  type: CodexEntryType
  name: string
  aliases: string[]
  tags: string[]
  description: string
  metadata: Record<string, string>
  revision: string
}

/** SaveCodexEntryRequest is the create or update payload for one Codex entry. */
export type SaveCodexEntryRequest = {
  type?: CodexEntryType
  name: string
  aliases: string[]
  tags: string[]
  description: string
  metadata: Record<string, string>
  expected_revision?: string
}

/** CodexProgression is one timeline change applied to a stable scene anchor. */
export type CodexProgression = {
  id?: string
  anchor: {
    type: 'scene'
    id: string
    timing: 'before' | 'after'
  }
  changes: {
    description?: string
    metadata?: Record<string, string>
  }
}

/** CodexProgressionDocument stores one entry's ordered progression list. */
export type CodexProgressionDocument = {
  entry_id: string
  progressions: CodexProgression[]
  revision: string | null
}

/** SaveCodexProgressionsRequest replaces one entry's full progression document. */
export type SaveCodexProgressionsRequest = {
  progressions: CodexProgression[]
  expected_revision: string | null
}

/** CodexActiveState is one Codex entry resolved as of a target scene. */
export type CodexActiveState = {
  scene_id: string
  entry: Omit<CodexEntry, 'revision'>
  applied_progression_ids: string[]
}

/** APIError exposes the HTTP status for failed backend requests. */
export class APIError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'APIError'
    this.status = status
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, init)
  const body = await response.json()
  if (!response.ok) {
    throw new APIError(response.status, body.error ?? `Request failed with status ${response.status}`)
  }
  return body as T
}

function postJSON<T>(path: string, body: unknown): Promise<T> {
  return request(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
}

/** getHealth loads the backend health payload and version string. */
export function getHealth(): Promise<Health> {
  return request('/api/health')
}

/** createProject creates a new portable project folder on disk. */
export function createProject(name: string, path: string): Promise<Project> {
  return postJSON('/api/projects', { name, path })
}

/** openProject opens an existing project folder by absolute path. */
export function openProject(path: string): Promise<Project> {
  return postJSON('/api/projects/open', { path })
}

/** getOutline loads the active project's canonical outline. */
export function getOutline(): Promise<Outline> {
  return request('/api/outline')
}

/** createArc appends one top-level arc to the active outline. */
export function createArc(title: string): Promise<OutlineMutation> {
  return postJSON('/api/arcs', { title })
}

/** createChapter appends one chapter under the supplied arc. */
export function createChapter(arcID: string, title: string): Promise<OutlineMutation> {
  return postJSON('/api/chapters', { arc_id: arcID, title })
}

/** createScene appends one scene under the supplied chapter. */
export function createScene(chapterID: string, title: string): Promise<OutlineMutation> {
  return postJSON('/api/scenes', { chapter_id: chapterID, title })
}

/** reorderOutline reorders chapters or scenes under one stable parent. */
export function reorderOutline(requestBody: ReorderRequest): Promise<OutlineMutation> {
  return postJSON('/api/outline/reorder', requestBody)
}

/** getScene loads one canonical scene document by stable scene ID. */
export function getScene(sceneID: string): Promise<SceneDocument> {
  return request(`/api/scenes/${sceneID}`)
}

/** saveScene validates and persists one explicit scene edit. */
export function saveScene(sceneID: string, requestBody: SaveSceneRequest): Promise<SceneDocument> {
  return request(`/api/scenes/${sceneID}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(requestBody),
  })
}

/** getCodexEntries loads the active project's full Codex list. */
export function getCodexEntries(): Promise<{ entries: CodexEntry[] }> {
  return request('/api/codex')
}

/** createCodexEntry creates one new canonical Codex entry. */
export function createCodexEntry(requestBody: SaveCodexEntryRequest): Promise<CodexEntry> {
  return postJSON('/api/codex', requestBody)
}

/** getCodexEntry loads one canonical Codex entry by stable ID. */
export function getCodexEntry(entryID: string): Promise<CodexEntry> {
  return request(`/api/codex/${entryID}`)
}

/** updateCodexEntry updates one existing canonical Codex entry. */
export function updateCodexEntry(entryID: string, requestBody: SaveCodexEntryRequest): Promise<CodexEntry> {
  return request(`/api/codex/${entryID}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(requestBody),
  })
}

/** getCodexProgressions loads one entry's canonical progression document. */
export function getCodexProgressions(entryID: string): Promise<CodexProgressionDocument> {
  return request(`/api/codex/${entryID}/progressions`)
}

/** saveCodexProgressions replaces one entry's ordered progression document. */
export function saveCodexProgressions(entryID: string, requestBody: SaveCodexProgressionsRequest): Promise<CodexProgressionDocument> {
  return request(`/api/codex/${entryID}/progressions`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(requestBody),
  })
}

/** getCodexActiveState resolves one Codex entry as of the supplied scene ID. */
export function getCodexActiveState(entryID: string, sceneID: string): Promise<CodexActiveState> {
  return request(`/api/codex/${entryID}/active?scene_id=${encodeURIComponent(sceneID)}`)
}
