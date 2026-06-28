export type Health = { status: string; version: string }

export type Project = {
  project_id: string
  name?: string
  path: string
  git_initialized: boolean
  index_initialized: boolean
}

export type OutlineScene = {
  id: string
  title: string
  display_label: string
}

export type Chapter = {
  id: string
  title: string
  display_label: string
  scenes: OutlineScene[]
}

export type Arc = {
  id: string
  title: string
  display_label: string
  chapters: Chapter[]
}

export type Outline = {
  version: number
  arcs: Arc[]
}

export type OutlineMutation = {
  changed_id?: string
  outline: Outline
}

export type ReorderRequest = {
  parent_type: 'arc' | 'chapter'
  parent_id: string
  ordered_child_ids: string[]
}

export type SceneFrontMatter = {
  pov: string
  status: 'draft' | 'revised' | 'final'
  exclude_from_ai: boolean
}

export type SceneDocument = {
  id: string
  chapter_id: string
  title: string
  frontmatter: SceneFrontMatter
  markdown: string
  revision: string
}

export type SaveSceneRequest = {
  title: string
  frontmatter: SceneFrontMatter
  markdown: string
  expected_revision: string
}

export type CodexEntryType = 'character' | 'location' | 'lore' | 'custom'

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

export type SaveCodexEntryRequest = {
  type?: CodexEntryType
  name: string
  aliases: string[]
  tags: string[]
  description: string
  metadata: Record<string, string>
  expected_revision?: string
}

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

export type CodexProgressionDocument = {
  entry_id: string
  progressions: CodexProgression[]
  revision: string | null
}

export type SaveCodexProgressionsRequest = {
  progressions: CodexProgression[]
  expected_revision: string | null
}

export type CodexActiveState = {
  scene_id: string
  entry: Omit<CodexEntry, 'revision'>
  applied_progression_ids: string[]
}

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

export function getHealth(): Promise<Health> {
  return request('/api/health')
}

export function createProject(name: string, path: string): Promise<Project> {
  return postJSON('/api/projects', { name, path })
}

export function openProject(path: string): Promise<Project> {
  return postJSON('/api/projects/open', { path })
}

export function getOutline(): Promise<Outline> {
  return request('/api/outline')
}

export function createArc(title: string): Promise<OutlineMutation> {
  return postJSON('/api/arcs', { title })
}

export function createChapter(arcID: string, title: string): Promise<OutlineMutation> {
  return postJSON('/api/chapters', { arc_id: arcID, title })
}

export function createScene(chapterID: string, title: string): Promise<OutlineMutation> {
  return postJSON('/api/scenes', { chapter_id: chapterID, title })
}

export function reorderOutline(requestBody: ReorderRequest): Promise<OutlineMutation> {
  return postJSON('/api/outline/reorder', requestBody)
}

export function getScene(sceneID: string): Promise<SceneDocument> {
  return request(`/api/scenes/${sceneID}`)
}

export function saveScene(sceneID: string, requestBody: SaveSceneRequest): Promise<SceneDocument> {
  return request(`/api/scenes/${sceneID}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(requestBody),
  })
}

export function getCodexEntries(): Promise<{ entries: CodexEntry[] }> {
  return request('/api/codex')
}

export function createCodexEntry(requestBody: SaveCodexEntryRequest): Promise<CodexEntry> {
  return postJSON('/api/codex', requestBody)
}

export function getCodexEntry(entryID: string): Promise<CodexEntry> {
  return request(`/api/codex/${entryID}`)
}

export function updateCodexEntry(entryID: string, requestBody: SaveCodexEntryRequest): Promise<CodexEntry> {
  return request(`/api/codex/${entryID}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(requestBody),
  })
}

export function getCodexProgressions(entryID: string): Promise<CodexProgressionDocument> {
  return request(`/api/codex/${entryID}/progressions`)
}

export function saveCodexProgressions(entryID: string, requestBody: SaveCodexProgressionsRequest): Promise<CodexProgressionDocument> {
  return request(`/api/codex/${entryID}/progressions`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(requestBody),
  })
}

export function getCodexActiveState(entryID: string, sceneID: string): Promise<CodexActiveState> {
  return request(`/api/codex/${entryID}/active?scene_id=${encodeURIComponent(sceneID)}`)
}
