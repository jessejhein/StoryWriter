/**
 * api.ts
 *
 * Defines the frontend API contract and small fetch helpers for the local
 * Storywork backend. All exported types mirror the JSON payloads exchanged
 * between the React workbenches and the Go HTTP API.
 */

import type {
  ContextPreviewResponse,
  RunActionResponse as ScopedRunActionResponse,
} from './editor/actionTypes'

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

export type AgentDefinition = {
  id: string
  name: string
  description: string
  surfaces: Array<'editor' | 'chapter_view'>
  input_scopes: Array<'selection' | 'scene' | 'chapter' | 'chapter_review'>
  min_words: number
  max_words: number
  required_context: string[]
  optional_context: string[]
  forbidden_context: string[]
  rag_mode: 'none' | 'timeline_aware'
  output_mode: 'patch' | 'suggestion'
  requires_acceptance: boolean
}

export type StyleDefinition = {
  id: string
  version: number
  name: string
  provider_profile_id: string
  model: string
  temperature: number
  system_prompt: string
  provider_readiness: 'ready' | 'missing_profile' | 'missing_credential'
}

export type AvailableAction = {
  agent_id: string
  name: string
  description: string
  output_mode: 'patch' | 'suggestion'
  requires_acceptance: boolean
  style_ids: string[]
}

export type AvailableActionsResponse = {
  actions: AvailableAction[]
}

export type ActionSelection = {
  start_byte: number
  end_byte: number
  text: string
}

export type RunActionRequest = {
  agent_id: string
  style_id: string
  surface: 'editor' | 'chapter_view'
  input_scope: 'selection' | 'chapter'
  scene_id: string
  scene_revision: string
  selection: ActionSelection
}

export type RunActionResponse = {
  run_id: string
  status: 'pending' | 'accepting' | 'accepted' | 'rejected'
  agent_id: string
  style_id: string
  scene_id: string
  scene_revision: string
  selection: {
    start_byte: number
    end_byte: number
  }
  output_mode: 'patch'
  patch: {
    original: string
    replacement: string
  }
  context_summary: {
    packs_used: string[]
    rag_mode: 'none'
  }
  provider: {
    profile_id: string
    type: 'openai_compatible' | 'ollama'
    model: string
  }
}

export type ProviderProfile = {
  id: string
  name: string
  type: 'openai_compatible' | 'ollama'
  base_url: string
  auth: {
    type: 'none' | 'bearer_env'
    credential_env: string
  }
  capabilities: {
    chat: boolean
    streaming: boolean
    structured_output: boolean
    max_context_tokens: number
  }
  readiness?: 'ready' | 'missing_credential'
}

export type ProviderProfilesResponse = {
  profiles: ProviderProfile[]
  revision: string | null
}

export type ImportSummary = {
  id: string
  created_at: string
  file_count: number
  total_bytes: number
}

export type ImportFile = {
  path: string
  bytes: number
  sha256: string
}

export type ImportResponse = {
  import: ImportSummary
  files: ImportFile[]
}

export type ImportChunk = {
  id: string
  import_id: string
  source_path: string
  start_line: number
  end_line: number
  text: string
  sha256: string
}

export type ImportCandidateKind = 'codex' | 'arc' | 'chapter' | 'scene'
export type ImportCandidateStatus = 'pending' | 'merged' | 'discarded' | 'accepted'

export type ImportCanonicalRef = {
  kind: 'codex' | 'arc' | 'chapter' | 'scene'
  id: string
}

export type CodexCandidateProposal = {
  type: CodexEntryType
  name: string
  aliases: string[]
  tags: string[]
  description: string
}

export type ArcCandidateProposal = { title: string }
export type ChapterCandidateProposal = { title: string; parent_candidate_id: string }
export type SceneCandidateProposal = { title: string; parent_candidate_id: string }

export type ImportCandidateProposal =
  | CodexCandidateProposal
  | ArcCandidateProposal
  | ChapterCandidateProposal
  | SceneCandidateProposal

export type ImportCandidate = {
  id: string
  kind: ImportCandidateKind
  proposal_version: number
  status: ImportCandidateStatus
  revision: string
  provenance: { chunk_ids: string[] }
  proposal: ImportCandidateProposal
  replacement_candidate_id: string | null
  canonical_refs: ImportCanonicalRef[]
}

export type ImportExtractionResponse = {
  candidates: ImportCandidate[]
  provider: {
    profile_id: string
    type: 'openai_compatible' | 'ollama'
    model: string
  }
}

export type ImportMergeResponse = {
  candidate: ImportCandidate
  merged_candidate_ids: string[]
}

export type ImportAcceptResponse = {
  candidate: ImportCandidate
  canonical_refs: ImportCanonicalRef[]
}

export type ActionDecisionResponse = {
  run_id: string
  status: 'accepted' | 'rejected'
}

export type FollowUpInvitation = {
  invitation_id: string
  parent_run_id: string
  root_run_id: string
  chain_depth: number
  agent_id: string
  scope: 'selection' | 'scene' | 'chapter_review'
  scene_id?: string
  chapter_id?: string
  relationship: 'triggered' | 'depends_on'
}

export type AcceptActionResponse = ActionDecisionResponse & {
  scene?: SceneDocument
  follow_up_invitations: FollowUpInvitation[]
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

/** CodexEntryFields contains the mutable fields shared by create and update requests. */
export type CodexEntryFields = {
  name: string
  aliases: string[]
  tags: string[]
  description: string
  metadata: Record<string, string>
}

/** CreateCodexEntryRequest creates an entry of one required canonical type. */
export type CreateCodexEntryRequest = CodexEntryFields & { type: CodexEntryType }

/** UpdateCodexEntryRequest updates mutable fields at one required revision. */
export type UpdateCodexEntryRequest = CodexEntryFields & { expected_revision: string }

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

export function getAgents(): Promise<{ agents: AgentDefinition[] }> {
  return request('/api/agents')
}

export function getStyles(): Promise<{ styles: StyleDefinition[] }> {
  return request('/api/styles')
}

export function getAvailableActions(params: {
  surface: 'editor' | 'chapter_view'
  input_scope: 'selection' | 'scene' | 'chapter' | 'chapter_review'
  scene_id: string
  selection_words: number
}): Promise<AvailableActionsResponse> {
  const query = new URLSearchParams({
    surface: params.surface,
    input_scope: params.input_scope,
    scene_id: params.scene_id,
    selection_words: String(params.selection_words),
  })
  return request(`/api/actions/available?${query.toString()}`)
}

export function runAction(requestBody: RunActionRequest): Promise<RunActionResponse> {
  return postJSON('/api/actions/run', requestBody)
}

export type TaggedRunActionRequest = {
  agent_id: string
  style_id: string
  scope: 'selection' | 'scene' | 'chapter_review'
  target: {
    scene_id?: string
    scene_revision?: string
    chapter_id?: string
    fingerprint?: string
    start_byte?: number
    end_byte?: number
    text?: string
  }
}

export function previewActionContext(requestBody: RunActionRequest | TaggedRunActionRequest): Promise<ContextPreviewResponse> {
  return postJSON('/api/actions/context-preview', requestBody)
}

export function runTaggedAction(requestBody: TaggedRunActionRequest): Promise<ScopedRunActionResponse> {
  return postJSON('/api/actions/run', requestBody)
}

export function runInvitation(
  invitationID: string,
  requestBody: { style_id: string; expected_target_revision: string },
): Promise<ScopedRunActionResponse> {
  return postJSON(`/api/action-invitations/${invitationID}/run`, requestBody)
}

export function acceptAction(runID: string, expectedRevision: string): Promise<AcceptActionResponse> {
  return postJSON(`/api/actions/${runID}/accept`, { expected_revision: expectedRevision })
}

export function rejectAction(runID: string): Promise<ActionDecisionResponse> {
  return request(`/api/actions/${runID}/reject`, { method: 'POST' })
}

export function getProviderProfiles(): Promise<ProviderProfilesResponse> {
  return request('/api/provider-profiles')
}

export function saveProviderProfiles(
  profiles: ProviderProfile[],
  expectedRevision: string | null,
): Promise<ProviderProfilesResponse> {
  return request('/api/provider-profiles', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ profiles, expected_revision: expectedRevision }),
  })
}

/** getCodexEntries loads the active project's full Codex list. */
export function getCodexEntries(): Promise<{ entries: CodexEntry[] }> {
  return request('/api/codex')
}

/** createCodexEntry creates one new canonical Codex entry. */
export function createCodexEntry(requestBody: CreateCodexEntryRequest): Promise<CodexEntry> {
  return postJSON('/api/codex', requestBody)
}

/** getCodexEntry loads one canonical Codex entry by stable ID. */
export function getCodexEntry(entryID: string): Promise<CodexEntry> {
  return request(`/api/codex/${entryID}`)
}

/** updateCodexEntry updates one existing canonical Codex entry. */
export function updateCodexEntry(entryID: string, requestBody: UpdateCodexEntryRequest): Promise<CodexEntry> {
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

export function createImport(sourceDirectory: string): Promise<ImportResponse> {
  return postJSON('/api/imports', { source_directory: sourceDirectory })
}

export function getImports(): Promise<{ imports: ImportSummary[] }> {
	return request('/api/imports')
}

/** getImport loads one durable import manifest and its file records. */
export function getImport(importID: string): Promise<ImportResponse> {
  return request(`/api/imports/${importID}`)
}

export function getImportChunks(importID: string): Promise<{ chunks: ImportChunk[] }> {
  return request(`/api/imports/${importID}/chunks`)
}

export function extractImport(importID: string, requestBody: {
  chunk_ids: string[]
  mode: 'structure'
  profile_id: string
  model: string
}): Promise<ImportExtractionResponse> {
  return postJSON(`/api/imports/${importID}/extractions`, requestBody)
}

export function getImportCandidates(filters?: { status?: ImportCandidateStatus; kind?: ImportCandidateKind }): Promise<{ candidates: ImportCandidate[] }> {
  const query = new URLSearchParams()
  if (filters?.status) query.set('status', filters.status)
  if (filters?.kind) query.set('kind', filters.kind)
  const suffix = query.toString() ? `?${query.toString()}` : ''
  return request(`/api/import-candidates${suffix}`)
}

export function getImportCandidate(candidateID: string): Promise<ImportCandidate> {
  return request(`/api/import-candidates/${candidateID}`)
}

export function updateImportCandidate(candidateID: string, proposal: ImportCandidateProposal, expectedRevision: string): Promise<ImportCandidate> {
  return request(`/api/import-candidates/${candidateID}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ proposal, expected_revision: expectedRevision }),
  })
}

export function mergeImportCandidate(candidateID: string, requestBody: {
  other_candidate_id: string
  expected_revision: string
  other_expected_revision: string
  proposal: ImportCandidateProposal
}): Promise<ImportMergeResponse> {
  return postJSON(`/api/import-candidates/${candidateID}/merge`, requestBody)
}

export function discardImportCandidate(candidateID: string, expectedRevision: string): Promise<ImportCandidate> {
  return postJSON(`/api/import-candidates/${candidateID}/discard`, { expected_revision: expectedRevision })
}

export function acceptImportCandidate(candidateID: string, expectedRevision: string): Promise<ImportAcceptResponse> {
  return postJSON(`/api/import-candidates/${candidateID}/accept`, { expected_revision: expectedRevision })
}
