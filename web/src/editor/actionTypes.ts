/**
 * actionTypes.ts
 *
 * Milestone 7 action scope, manifest, finding, and invitation types mirrored
 * from the backend HTTP contract.
 */

export type ActionScope = 'selection' | 'scene' | 'chapter_review'

export type ActionRAGMode = 'none' | 'timeline_aware'

export type ActionOutputMode = 'patch' | 'suggestion'

export type ActionRunStatus = 'pending' | 'accepting' | 'accepted' | 'rejected' | 'completed'

export type ContextPackName = string

export type PackOmissionReason = 'budget'

export type ContextPackOmission = {
  pack: ContextPackName
  reason: PackOmissionReason
}

export type ManifestCodexRef = {
  entry_id: string
  applied_progression_ids: string[]
}

export type ContextManifest = {
  scope: ActionScope
  packs_used: ContextPackName[]
  packs_omitted: ContextPackOmission[]
  estimated_input_tokens: number
  max_input_estimated_tokens: number
  rag_mode: ActionRAGMode
  active_codex?: ManifestCodexRef[]
  outline_refs?: string[]
}

export type SelectionTarget = {
  scene_id: string
  scene_revision: string
  start_byte: number
  end_byte: number
  text: string
}

export type SceneTarget = {
  scene_id: string
  scene_revision: string
}

export type ChapterReviewTarget = {
  chapter_id: string
  fingerprint: string
}

export type TaggedActionTarget =
  | { scope: 'selection'; target: SelectionTarget }
  | { scope: 'scene'; target: SceneTarget }
  | { scope: 'chapter_review'; target: ChapterReviewTarget }

export type LegacyRunActionRequest = {
  agent_id: string
  style_id: string
  surface: 'editor' | 'chapter_view'
  input_scope: 'selection' | 'chapter'
  scene_id: string
  scene_revision: string
  selection: {
    start_byte: number
    end_byte: number
    text: string
  }
}

export type TaggedRunActionRequest = {
  agent_id: string
  style_id: string
  scope: ActionScope
  target: SelectionTarget | SceneTarget | ChapterReviewTarget
}

export type ContextPreviewResponse = {
  manifest: ContextManifest
  target_revision: string
}

export type ActionFinding = {
  title: string
  explanation: string
  scene_ids: string[]
  follow_up_agent_ids: string[]
}

export type FollowUpInvitation = {
  invitation_id: string
  parent_run_id: string
  root_run_id: string
  chain_depth: number
  agent_id: string
  scope: ActionScope
  scene_id?: string
  chapter_id?: string
  relationship: 'triggered' | 'depends_on'
}

export type ActionProviderInfo = {
  profile_id: string
  type: 'openai_compatible' | 'ollama'
  model: string
}

export type ActionContextSummary = {
  packs_used: string[]
  rag_mode: ActionRAGMode
}

export type PatchRunResponse = {
  run_id: string
  status: ActionRunStatus
  agent_id: string
  style_id: string
  scope?: ActionScope
  scene_id: string
  scene_revision: string
  parent_run_id?: string | null
  root_run_id?: string
  chain_depth?: number
  selection?: {
    start_byte: number
    end_byte: number
  }
  output_mode: 'patch'
  patch: {
    original: string
    replacement: string
  }
  context_summary: ActionContextSummary
  manifest?: ContextManifest
  provider: ActionProviderInfo
  follow_up_invitations?: FollowUpInvitation[]
}

export type SuggestionRunResponse = {
  run_id: string
  status: ActionRunStatus
  agent_id: string
  style_id: string
  scope: 'chapter_review'
  chapter_id: string
  chapter_fingerprint: string
  parent_run_id?: string | null
  root_run_id?: string
  chain_depth?: number
  output_mode: 'suggestion'
  findings: ActionFinding[]
  manifest?: ContextManifest
  provider: ActionProviderInfo
  follow_up_invitations?: FollowUpInvitation[]
}

export type RunActionResponse = PatchRunResponse | SuggestionRunResponse

export type AcceptActionResponse = {
  run_id: string
  status: 'accepted' | 'rejected'
  scene?: {
    id: string
    chapter_id: string
    title: string
    frontmatter: {
      pov: string
      status: 'draft' | 'revised' | 'final'
      exclude_from_ai: boolean
    }
    markdown: string
    revision: string
  }
  follow_up_invitations: FollowUpInvitation[]
}

export type RunInvitationRequest = {
  style_id: string
  expected_target_revision: string
}

export type ActionWorkflowScope = 'selection' | 'scene' | 'chapter_review'

export type ActionWorkflowState = {
  preview: {
    scope: ActionWorkflowScope
    targetRevision: string
    manifest: ContextManifest
    requestVersion: number
  } | null
  previewLoading: boolean
  previewError: string | null
  run: {
    response: RunActionResponse
    requestVersion: number
  } | null
  runLoading: boolean
  runError: string | null
  invitations: FollowUpInvitation[]
  invitationRun: {
    invitationID: string
    response: RunActionResponse
    requestVersion: number
  } | null
  invitationLoadingID: string | null
  invitationError: string | null
  requestVersion: number
}