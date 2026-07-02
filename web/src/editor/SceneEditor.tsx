/**
 * SceneEditor.tsx
 *
 * Hosts the canonical scene editor with Milestone 7 timeline-aware actions,
 * context preview, scene rewrite, chapter review, and explicit follow-ups.
 */

import { useEffect, useRef, useState } from 'react'
import type { AvailableAction, Project, SaveSceneRequest, SceneDocument, StyleDefinition } from '../api'
import {
  APIError,
  acceptAction,
  getAvailableActions,
  getScene,
  getStyles,
  previewActionContext,
  rejectAction,
  runAction,
  runInvitation,
  runTaggedAction,
  saveScene,
} from '../api'
import ConfirmDialog from '../components/ConfirmDialog'
import ChapterReview from './ChapterReview'
import ContextPreview from './ContextPreview'
import FollowUpInvitations from './FollowUpInvitations'
import CodeMirrorSurface from './CodeMirrorSurface'
import {
  applyInvitationRunFailure,
  applyInvitationRunSuccess,
  applyPreviewFailure,
  applyPreviewSuccess,
  applyRunFailure,
  applyRunSuccess,
  beginInvitationRun,
  beginPreview,
  beginRun,
  bumpActionRequestVersion,
  clearRunPreview,
  initialActionWorkflowState,
  invalidatePreviewForRevision,
  setFollowUpInvitations,
} from './actionState'
import type { ActionWorkflowScope, ActionWorkflowState, FollowUpInvitation, RunActionResponse, SuggestionRunResponse } from './actionTypes'
import { countWords, toUTF8ByteRange } from './selection'

type Props = {
  project: Project
  sceneID: string
  onBack: () => void
  onDirtyChange: (dirty: boolean) => void
}

type Draft = {
  title: string
  pov: string
  status: 'draft' | 'revised' | 'final'
  excludeFromAI: boolean
  markdown: string
}

type Feedback = { kind: 'saved' | 'conflict' | 'error'; message: string } | null
type EditorSelectionState = { start: number; end: number; text: string }
type ActionFeedback = { kind: 'error' | 'conflict'; message: string } | null

type PendingScopeAction = {
  scope: ActionWorkflowScope
  agentID: string
  styleID: string
  invitation?: FollowUpInvitation
}

function toDraft(scene: SceneDocument): Draft {
  return {
    title: scene.title,
    pov: scene.frontmatter.pov,
    status: scene.frontmatter.status,
    excludeFromAI: scene.frontmatter.exclude_from_ai,
    markdown: scene.markdown,
  }
}

function isDraftDirty(baseline: SceneDocument | null, draft: Draft | null): boolean {
  if (!baseline || !draft) {
    return false
  }
  return baseline.title !== draft.title
    || baseline.frontmatter.pov !== draft.pov
    || baseline.frontmatter.status !== draft.status
    || baseline.frontmatter.exclude_from_ai !== draft.excludeFromAI
    || baseline.markdown !== draft.markdown
}

function validateDraft(draft: Draft | null): string | null {
  if (!draft) {
    return 'Scene is not loaded yet.'
  }
  if (!draft.title.trim()) {
    return 'Title is required.'
  }
  if (Array.from(draft.title.trim()).length > 200) {
    return 'Title must be 200 characters or fewer.'
  }
  if (Array.from(draft.pov.trim()).length > 200) {
    return 'POV must be 200 characters or fewer.'
  }
  if (!['draft', 'revised', 'final'].includes(draft.status)) {
    return 'Status is invalid.'
  }
  return null
}

function isSuggestionResponse(response: RunActionResponse): response is SuggestionRunResponse {
  return response.output_mode === 'suggestion'
}

function scopeRank(scope: ActionWorkflowScope): number {
  switch (scope) {
    case 'selection':
      return 0
    case 'scene':
      return 1
    case 'chapter_review':
      return 2
    default:
      return 0
  }
}

/**
 * SceneEditor
 *
 * Renders the scene metadata form, CodeMirror editor surface, and explicit AI
 * action workflows for selection, scene, and chapter review scopes.
 */
export default function SceneEditor({ project, sceneID, onBack, onDirtyChange }: Props) {
  const [baseline, setBaseline] = useState<SceneDocument | null>(null)
  const [draft, setDraft] = useState<Draft | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [feedback, setFeedback] = useState<Feedback>(null)
  const [confirmReload, setConfirmReload] = useState(false)
  const [selection, setSelection] = useState<EditorSelectionState>({ start: 0, end: 0, text: '' })
  const [actionsOpen, setActionsOpen] = useState(false)
  const [sceneActionsOpen, setSceneActionsOpen] = useState(false)
  const [chapterReviewOpen, setChapterReviewOpen] = useState(false)
  const [actionsLoading, setActionsLoading] = useState(false)
  const [sceneActionsLoading, setSceneActionsLoading] = useState(false)
  const [availableActions, setAvailableActions] = useState<AvailableAction[]>([])
  const [sceneAvailableActions, setSceneAvailableActions] = useState<AvailableAction[]>([])
  const [chapterReviewAction, setChapterReviewAction] = useState<AvailableAction | null>(null)
  const [styles, setStyles] = useState<StyleDefinition[]>([])
  const [selectedAgentID, setSelectedAgentID] = useState('')
  const [selectedStyleID, setSelectedStyleID] = useState('')
  const [selectedSceneAgentID, setSelectedSceneAgentID] = useState('')
  const [selectedSceneStyleID, setSelectedSceneStyleID] = useState('')
  const [selectedChapterStyleID, setSelectedChapterStyleID] = useState('')
  const [runningAction, setRunningAction] = useState(false)
  const [acceptingAction, setAcceptingAction] = useState(false)
  const [rejectingAction, setRejectingAction] = useState(false)
  const [actionFeedback, setActionFeedback] = useState<ActionFeedback>(null)
  const [actionWorkflow, setActionWorkflow] = useState<ActionWorkflowState>(initialActionWorkflowState)
  const [activePreviewScope, setActivePreviewScope] = useState<ActionWorkflowScope | null>(null)
  const [pendingScopeAction, setPendingScopeAction] = useState<PendingScopeAction | null>(null)
  const sceneVersionRef = useRef(0)
  const previewRegionRef = useRef<HTMLDivElement | null>(null)
  const activeRun = actionWorkflow.run?.response ?? actionWorkflow.invitationRun?.response ?? null
  const activeRunID = activeRun?.run_id ?? null

  const dirty = isDraftDirty(baseline, draft)
  const validationError = validateDraft(draft)
  const canSave = !loading && !saving && dirty && !validationError && baseline !== null
  const selectedWordCount = countWords(selection.text)
  const sceneWordCount = countWords(draft?.markdown ?? '')
  const actionDisableReason = loading
    ? 'Scene is loading.'
    : saving
      ? 'Wait for the current save to finish.'
      : dirty
        ? 'Save or reload the scene before running AI actions.'
        : !baseline || !draft
          ? 'Scene is not loaded yet.'
          : runningAction || acceptingAction || rejectingAction || actionWorkflow.previewLoading || actionWorkflow.runLoading
            ? 'Wait for the current action request to finish.'
            : null
  const selectionActionDisableReason = actionDisableReason ?? (
    !selection.text.trim() ? 'Select canonical scene text to discover actions.' : null
  )
  const canOpenSelectionActions = selectionActionDisableReason === null
  const canOpenSceneActions = actionDisableReason === null
  const canOpenChapterReview = actionDisableReason === null

  useEffect(() => {
    let cancelled = false
    sceneVersionRef.current += 1
    setActionWorkflow(bumpActionRequestVersion(initialActionWorkflowState()))
    void (async () => {
      try {
        const scene = await getScene(sceneID)
        if (cancelled) {
          return
        }
        setBaseline(scene)
        setDraft(toDraft(scene))
        setFeedback(null)
        setSelection({ start: 0, end: 0, text: '' })
        setActionsOpen(false)
        setSceneActionsOpen(false)
        setChapterReviewOpen(false)
        setAvailableActions([])
        setSceneAvailableActions([])
        setChapterReviewAction(null)
        setStyles([])
        setSelectedAgentID('')
        setSelectedStyleID('')
        setSelectedSceneAgentID('')
        setSelectedSceneStyleID('')
        setSelectedChapterStyleID('')
        setActionFeedback(null)
      } catch (requestError) {
        if (cancelled) {
          return
        }
        setFeedback({
          kind: 'error',
          message: requestError instanceof Error ? requestError.message : 'Scene request failed',
        })
        setBaseline(null)
        setDraft(null)
      } finally {
        if (!cancelled) {
          setLoading(false)
        }
      }
    })()
    return () => {
      cancelled = true
    }
  }, [sceneID])

  useEffect(() => {
    onDirtyChange(dirty)
  }, [dirty, onDirtyChange])

  useEffect(() => {
    if (!dirty) {
      return
    }
    const handleBeforeUnload = (event: BeforeUnloadEvent) => {
      event.preventDefault()
      event.returnValue = ''
    }
    window.addEventListener('beforeunload', handleBeforeUnload)
    return () => window.removeEventListener('beforeunload', handleBeforeUnload)
  }, [dirty])

  useEffect(() => {
    if (activeRunID === null) {
      return
    }
    previewRegionRef.current?.focus()
  }, [activeRunID])

  useEffect(() => {
    if (!baseline) {
      return
    }
    setActionWorkflow((current) => invalidatePreviewForRevision(current, baseline.revision))
  }, [baseline?.revision])

  function patchDraft(patch: Partial<Draft>) {
    setDraft((current) => {
      if (!current) {
        return current
      }
      const next = { ...current, ...patch }
      if (feedback?.kind === 'error') {
        setFeedback(null)
      }
      return next
    })
  }

  function pickDefaultStyle(actions: AvailableAction[], availableStyles: StyleDefinition[]) {
    if (actions.length === 0) {
      return { agentID: '', styleID: '' }
    }
    const firstAction = actions[0]!
    const matchingStyle = availableStyles.find((style) => firstAction.style_ids.includes(style.id))
    return {
      agentID: firstAction.agent_id,
      styleID: matchingStyle?.id ?? firstAction.style_ids[0] ?? '',
    }
  }

  async function submitSave() {
    if (!baseline || !draft || !canSave) {
      return
    }
    setSaving(true)
    try {
      const requestBody: SaveSceneRequest = {
        title: draft.title,
        frontmatter: {
          pov: draft.pov,
          status: draft.status,
          exclude_from_ai: draft.excludeFromAI,
        },
        markdown: draft.markdown,
        expected_revision: baseline.revision,
      }
      const saved = await saveScene(sceneID, requestBody)
      setBaseline(saved)
      setDraft(toDraft(saved))
      setFeedback({ kind: 'saved', message: 'Saved' })
    } catch (requestError) {
      const message = requestError instanceof Error ? requestError.message : 'Save failed'
      setFeedback({
        kind: requestError instanceof APIError && requestError.status === 409 ? 'conflict' : 'error',
        message,
      })
    } finally {
      setSaving(false)
    }
  }

  async function reloadCanonical() {
    setLoading(true)
    try {
      const scene = await getScene(sceneID)
      setBaseline(scene)
      setDraft(toDraft(scene))
      setFeedback(null)
      setActionWorkflow((current) => clearRunPreview(current))
    } catch (requestError) {
      setFeedback({
        kind: 'error',
        message: requestError instanceof Error ? requestError.message : 'Scene request failed',
      })
    } finally {
      setLoading(false)
    }
  }

  async function openSelectionActions() {
    if (!baseline || !draft || !canOpenSelectionActions) {
      return
    }
    setActionsOpen(true)
    setActionsLoading(true)
    setActionFeedback(null)
    const sceneVersion = sceneVersionRef.current
    try {
      const [stylesResponse, actionsResponse] = await Promise.all([
        getStyles(),
        getAvailableActions({
          surface: 'editor',
          input_scope: 'selection',
          scene_id: baseline.id,
          selection_words: selectedWordCount,
        }),
      ])
      if (sceneVersion !== sceneVersionRef.current) {
        return
      }
      setStyles(stylesResponse.styles)
      setAvailableActions(actionsResponse.actions)
      const defaults = pickDefaultStyle(actionsResponse.actions, stylesResponse.styles)
      setSelectedAgentID(defaults.agentID)
      setSelectedStyleID(defaults.styleID)
    } catch (requestError) {
      if (sceneVersion !== sceneVersionRef.current) {
        return
      }
      setActionFeedback({
        kind: requestError instanceof APIError && requestError.status === 409 ? 'conflict' : 'error',
        message: requestError instanceof Error ? requestError.message : 'Action lookup failed',
      })
      setAvailableActions([])
      setStyles([])
      setSelectedAgentID('')
      setSelectedStyleID('')
    } finally {
      if (sceneVersion === sceneVersionRef.current) {
        setActionsLoading(false)
      }
    }
  }

  async function openSceneActions() {
    if (!baseline || !canOpenSceneActions) {
      return
    }
    setSceneActionsOpen(true)
    setSceneActionsLoading(true)
    setActionFeedback(null)
    const sceneVersion = sceneVersionRef.current
    try {
      const [stylesResponse, actionsResponse] = await Promise.all([
        getStyles(),
        getAvailableActions({
          surface: 'editor',
          input_scope: 'scene',
          scene_id: baseline.id,
          selection_words: sceneWordCount,
        }),
      ])
      if (sceneVersion !== sceneVersionRef.current) {
        return
      }
      setStyles(stylesResponse.styles)
      setSceneAvailableActions(actionsResponse.actions)
      const defaults = pickDefaultStyle(actionsResponse.actions, stylesResponse.styles)
      setSelectedSceneAgentID(defaults.agentID)
      setSelectedSceneStyleID(defaults.styleID)
    } catch (requestError) {
      if (sceneVersion !== sceneVersionRef.current) {
        return
      }
      setActionFeedback({
        kind: requestError instanceof APIError && requestError.status === 409 ? 'conflict' : 'error',
        message: requestError instanceof Error ? requestError.message : 'Scene action lookup failed',
      })
      setSceneAvailableActions([])
      setSelectedSceneAgentID('')
      setSelectedSceneStyleID('')
    } finally {
      if (sceneVersion === sceneVersionRef.current) {
        setSceneActionsLoading(false)
      }
    }
  }

  async function openChapterReview() {
    if (!baseline || !canOpenChapterReview) {
      return
    }
    setChapterReviewOpen(true)
    setActionFeedback(null)
    const sceneVersion = sceneVersionRef.current
    try {
      const [stylesResponse, actionsResponse] = await Promise.all([
        getStyles(),
        getAvailableActions({
          surface: 'chapter_view',
          input_scope: 'chapter_review',
          scene_id: baseline.id,
          selection_words: sceneWordCount,
        }),
      ])
      if (sceneVersion !== sceneVersionRef.current) {
        return
      }
      setStyles(stylesResponse.styles)
      const reviewAction = actionsResponse.actions.find((item) => item.agent_id === 'chapter_review') ?? actionsResponse.actions[0] ?? null
      setChapterReviewAction(reviewAction)
      const matchingStyle = reviewAction
        ? stylesResponse.styles.find((style) => reviewAction.style_ids.includes(style.id))
        : undefined
      setSelectedChapterStyleID(matchingStyle?.id ?? reviewAction?.style_ids[0] ?? '')
    } catch (requestError) {
      if (sceneVersion !== sceneVersionRef.current) {
        return
      }
      setActionFeedback({
        kind: requestError instanceof APIError && requestError.status === 409 ? 'conflict' : 'error',
        message: requestError instanceof Error ? requestError.message : 'Chapter review lookup failed',
      })
      setChapterReviewAction(null)
      setSelectedChapterStyleID('')
    }
  }

  function buildPreviewRequest(scope: ActionWorkflowScope, agentID: string, styleID: string) {
    if (!baseline || !draft) {
      return null
    }
    if (scope === 'selection') {
      const byteRange = toUTF8ByteRange(draft.markdown, selection.start, selection.end)
      return {
        agent_id: agentID,
        style_id: styleID,
        scope: 'selection' as const,
        target: {
          scene_id: baseline.id,
          scene_revision: baseline.revision,
          start_byte: byteRange.startByte,
          end_byte: byteRange.endByte,
          text: selection.text,
        },
      }
    }
    if (scope === 'scene') {
      return {
        agent_id: agentID,
        style_id: styleID,
        scope: 'scene' as const,
        target: {
          scene_id: baseline.id,
          scene_revision: baseline.revision,
        },
      }
    }
    return {
      agent_id: agentID,
      style_id: styleID,
      scope: 'chapter_review' as const,
      target: {
        chapter_id: baseline.chapter_id,
        fingerprint: actionWorkflow.preview?.scope === 'chapter_review'
          ? actionWorkflow.preview.targetRevision
          : `sha256:${'0'.repeat(64)}`,
      },
    }
  }

  async function submitPreviewContext(scope: ActionWorkflowScope, agentID: string, styleID: string) {
    if (!baseline || !draft || !agentID || !styleID) {
      return
    }
    if (scope === 'chapter_review' && !chapterReviewOpen) {
      await openChapterReview()
    }
    const requestBody = buildPreviewRequest(scope, agentID, styleID)
    if (!requestBody) {
      return
    }
    setActivePreviewScope(scope)
    setActionWorkflow((current) => beginPreview(current))
    const requestVersion = actionWorkflow.requestVersion
    const sceneVersion = sceneVersionRef.current
    try {
      let preview = await previewActionContext(requestBody)
      if (scope === 'chapter_review') {
        preview = await previewActionContext({
          ...requestBody,
          target: {
            chapter_id: baseline.chapter_id,
            fingerprint: preview.target_revision,
          },
        })
      }
      if (sceneVersion !== sceneVersionRef.current || requestVersion !== actionWorkflow.requestVersion) {
        return
      }
      setActionWorkflow((current) => applyPreviewSuccess(current, scope, preview, requestVersion))
    } catch (requestError) {
      if (sceneVersion !== sceneVersionRef.current || requestVersion !== actionWorkflow.requestVersion) {
        return
      }
      setActionWorkflow((current) => applyPreviewFailure(
        current,
        requestError instanceof Error ? requestError.message : 'Context preview failed',
        requestVersion,
      ))
    }
  }

  function requestScopedRun(scope: ActionWorkflowScope, agentID: string, styleID: string, invitation?: FollowUpInvitation) {
    const currentScope = activePreviewScope ?? 'selection'
    if (!invitation && scopeRank(scope) > scopeRank(currentScope)) {
      setPendingScopeAction({ scope, agentID, styleID, invitation })
      return
    }
    void submitScopedRun(scope, agentID, styleID, invitation)
  }

  async function submitScopedRun(scope: ActionWorkflowScope, agentID: string, styleID: string, invitation?: FollowUpInvitation) {
    if (!baseline || !draft) {
      return
    }
    setRunningAction(true)
    setActionFeedback(null)
    setActionWorkflow((current) => beginRun(current))
    const requestVersion = actionWorkflow.requestVersion
    const sceneVersion = sceneVersionRef.current
    try {
      let response: RunActionResponse
      if (invitation) {
        const expectedRevision = invitation.scope === 'chapter_review'
          ? actionWorkflow.preview?.targetRevision ?? baseline.revision
          : baseline.revision
        const invited = await runInvitation(invitation.invitation_id, {
          style_id: styleID,
          expected_target_revision: expectedRevision,
        })
        response = invited
      } else if (scope === 'selection') {
        const byteRange = toUTF8ByteRange(draft.markdown, selection.start, selection.end)
        response = await runAction({
          agent_id: agentID,
          style_id: styleID,
          surface: 'editor',
          input_scope: 'selection',
          scene_id: baseline.id,
          scene_revision: baseline.revision,
          selection: {
            start_byte: byteRange.startByte,
            end_byte: byteRange.endByte,
            text: selection.text,
          },
        }) as RunActionResponse
      } else if (scope === 'scene') {
        response = await runTaggedAction({
          agent_id: agentID,
          style_id: styleID,
          scope: 'scene',
          target: {
            scene_id: baseline.id,
            scene_revision: baseline.revision,
          },
        })
      } else {
        const fingerprint = actionWorkflow.preview?.scope === 'chapter_review'
          ? actionWorkflow.preview.targetRevision
          : (await previewActionContext({
            agent_id: agentID,
            style_id: styleID,
            scope: 'chapter_review',
            target: { chapter_id: baseline.chapter_id, fingerprint: `sha256:${'0'.repeat(64)}` },
          })).target_revision
        response = await runTaggedAction({
          agent_id: agentID,
          style_id: styleID,
          scope: 'chapter_review',
          target: {
            chapter_id: baseline.chapter_id,
            fingerprint,
          },
        })
      }
      if (sceneVersion !== sceneVersionRef.current || requestVersion !== actionWorkflow.requestVersion) {
        return
      }
      setActionWorkflow((current) => {
        const next = applyRunSuccess(current, response, requestVersion)
        return response.follow_up_invitations
          ? setFollowUpInvitations(next, response.follow_up_invitations)
          : next
      })
    } catch (requestError) {
      if (sceneVersion !== sceneVersionRef.current || requestVersion !== actionWorkflow.requestVersion) {
        return
      }
      setActionFeedback({
        kind: requestError instanceof APIError && requestError.status === 409 ? 'conflict' : 'error',
        message: requestError instanceof Error ? requestError.message : 'Action run failed',
      })
      setActionWorkflow((current) => applyRunFailure(
        current,
        requestError instanceof Error ? requestError.message : 'Action run failed',
        requestVersion,
      ))
    } finally {
      if (sceneVersion === sceneVersionRef.current) {
        setRunningAction(false)
      }
    }
  }

  async function submitInvitationRun(invitation: FollowUpInvitation) {
    const styleID = selectedSceneStyleID || selectedStyleID || selectedChapterStyleID || styles[0]?.id
    if (!styleID) {
      return
    }
    setActionWorkflow((current) => beginInvitationRun(current, invitation.invitation_id))
    const requestVersion = actionWorkflow.requestVersion
    const sceneVersion = sceneVersionRef.current
    try {
      const expectedRevision = invitation.scope === 'chapter_review'
        ? actionWorkflow.preview?.targetRevision ?? baseline?.revision ?? ''
        : baseline?.revision ?? ''
      const response = await runInvitation(invitation.invitation_id, {
        style_id: styleID,
        expected_target_revision: expectedRevision,
      })
      if (sceneVersion !== sceneVersionRef.current || requestVersion !== actionWorkflow.requestVersion) {
        return
      }
      setActionWorkflow((current) => applyInvitationRunSuccess(current, invitation.invitation_id, response, requestVersion))
    } catch (requestError) {
      if (sceneVersion !== sceneVersionRef.current || requestVersion !== actionWorkflow.requestVersion) {
        return
      }
      setActionWorkflow((current) => applyInvitationRunFailure(
        current,
        requestError instanceof Error ? requestError.message : 'Invitation run failed',
        requestVersion,
      ))
    }
  }

  async function submitAcceptAction() {
    if (!activeRun || !baseline || isSuggestionResponse(activeRun)) {
      return
    }
    setAcceptingAction(true)
    setActionFeedback(null)
    const sceneVersion = sceneVersionRef.current
    try {
      const response = await acceptAction(activeRun.run_id, baseline.revision)
      if (sceneVersion !== sceneVersionRef.current) {
        return
      }
      if (response.scene) {
        setBaseline(response.scene)
        setDraft(toDraft(response.scene))
      }
      setActionWorkflow((current) => setFollowUpInvitations(clearRunPreview(current), response.follow_up_invitations ?? []))
      setActionsOpen(false)
      setSceneActionsOpen(false)
      setSelection({ start: 0, end: 0, text: '' })
      setFeedback({ kind: 'saved', message: 'Saved' })
    } catch (requestError) {
      if (sceneVersion !== sceneVersionRef.current) {
        return
      }
      setActionFeedback({
        kind: requestError instanceof APIError && requestError.status === 409 ? 'conflict' : 'error',
        message: requestError instanceof Error ? requestError.message : 'Accept failed',
      })
    } finally {
      if (sceneVersion === sceneVersionRef.current) {
        setAcceptingAction(false)
      }
    }
  }

  async function submitRejectAction() {
    if (!activeRun || isSuggestionResponse(activeRun)) {
      return
    }
    setRejectingAction(true)
    setActionFeedback(null)
    const sceneVersion = sceneVersionRef.current
    try {
      await rejectAction(activeRun.run_id)
      if (sceneVersion !== sceneVersionRef.current) {
        return
      }
      setActionWorkflow((current) => clearRunPreview(current))
    } catch (requestError) {
      if (sceneVersion !== sceneVersionRef.current) {
        return
      }
      setActionFeedback({
        kind: requestError instanceof APIError && requestError.status === 409 ? 'conflict' : 'error',
        message: requestError instanceof Error ? requestError.message : 'Reject failed',
      })
    } finally {
      if (sceneVersion === sceneVersionRef.current) {
        setRejectingAction(false)
      }
    }
  }

  async function copyReplacement() {
    if (!activeRun || isSuggestionResponse(activeRun) || !activeRun.patch.replacement || !navigator.clipboard) {
      return
    }
    await navigator.clipboard.writeText(activeRun.patch.replacement)
  }

  function requestReloadCanonical() {
    if (dirty) {
      setConfirmReload(true)
      return
    }
    void reloadCanonical()
  }

  const statusText = loading
    ? 'Loading canonical scene'
    : saving
      ? 'Saving'
      : feedback?.kind === 'conflict'
        ? 'Conflict'
        : feedback?.kind === 'error'
          ? 'Request error'
          : feedback?.kind === 'saved' && !dirty
            ? 'Saved'
            : dirty
              ? 'Unsaved changes'
              : 'Clean'

  const previewManifest = actionWorkflow.preview?.manifest ?? null

  return (
    <section className="editor-shell" aria-live="polite">
      <div className="editor-meta">
        <p className="folio">Milestone 7 / Scene editor</p>
        <h2>Edit accepted canon without silent overwrites.</h2>
        <p>Inspect context, run explicit actions, and accept patches or review suggestions on your terms.</p>
      </div>

      <div className="editor-panel">
        <div className="editor-toolbar">
          <div>
            <span className="section-label">Active project</span>
            <strong>{project.name ?? project.project_id}</strong>
            <code>{project.path}</code>
          </div>
          <div className="editor-toolbar-actions">
            <span className="editor-state">{statusText}</span>
            <span className="vim-indicator">Vim mode</span>
            <button type="button" className="secondary" onClick={onBack}>Back to outline</button>
          </div>
        </div>

        {feedback && (
          <div className={`editor-banner editor-banner-${feedback.kind}`} role={feedback.kind === 'error' || feedback.kind === 'conflict' ? 'alert' : 'status'}>
            <span>{feedback.message}</span>
            {(feedback.kind === 'error' || feedback.kind === 'conflict') && (
              <div className="scene-banner-actions">
                <button type="button" className="secondary" onClick={requestReloadCanonical} disabled={loading || saving}>
                  Reload canonical
                </button>
                {dirty && (
                  <button type="button" onClick={() => void submitSave()} disabled={!canSave}>
                    Retry save
                  </button>
                )}
              </div>
            )}
          </div>
        )}

        {loading && <p className="outline-message">Loading scene...</p>}

        {!loading && draft && baseline && (
          <div className="scene-form">
            <div className="scene-identity">
              <div>
                <span className="section-label">Scene ID</span>
                <code>{baseline.id}</code>
              </div>
              <div>
                <span className="section-label">Chapter ID</span>
                <code>{baseline.chapter_id}</code>
              </div>
              <div>
                <span className="section-label">Revision</span>
                <code>{baseline.revision}</code>
              </div>
            </div>

            <div className="scene-grid">
              <label>
                Title
                <input value={draft.title} onChange={(event) => patchDraft({ title: event.target.value })} />
              </label>
              <label>
                POV
                <input value={draft.pov} onChange={(event) => patchDraft({ pov: event.target.value })} />
              </label>
              <label>
                Status
                <select value={draft.status} onChange={(event) => patchDraft({ status: event.target.value as Draft['status'] })}>
                  <option value="draft">Draft</option>
                  <option value="revised">Revised</option>
                  <option value="final">Final</option>
                </select>
              </label>
              <label className="checkbox-field">
                <input
                  type="checkbox"
                  checked={draft.excludeFromAI}
                  onChange={(event) => patchDraft({ excludeFromAI: event.target.checked })}
                />
                Exclude from AI actions
              </label>
            </div>

            <div className="scene-editor-frame">
              <div className="scene-editor-header">
                <strong>Scene Markdown</strong>
              </div>
              <CodeMirrorSurface
                value={draft.markdown}
                onChange={(markdown) => patchDraft({ markdown })}
                onSelectionChange={setSelection}
              />
            </div>

            {validationError && <p className="error" role="alert">{validationError}</p>}

            <section className="ai-actions-panel" aria-label="Selection AI actions">
              <div className="scene-editor-header">
                <strong>Selection Actions</strong>
                <button type="button" className="secondary" onClick={() => void openSelectionActions()} disabled={!canOpenSelectionActions}>
                  AI actions
                </button>
              </div>
              <p className="section-note">
                {selectionActionDisableReason ?? `Selected ${selectedWordCount} words from canonical scene text.`}
              </p>
              {actionFeedback && <p className="error" role="alert">{actionFeedback.message}</p>}
              {actionsOpen && (
                <div className="ai-actions-workflow">
                  {actionsLoading && <p className="outline-message">Loading actions...</p>}
                  {!actionsLoading && availableActions.length === 0 && (
                    <p className="outline-message">No applicable actions for this selection.</p>
                  )}
                  {!actionsLoading && availableActions.length > 0 && (
                    <>
                      <label>
                        Agent
                        <select value={selectedAgentID} onChange={(event) => setSelectedAgentID(event.target.value)} disabled={runningAction || acceptingAction || rejectingAction}>
                          {availableActions.map((item) => (
                            <option key={item.agent_id} value={item.agent_id}>{item.name}</option>
                          ))}
                        </select>
                      </label>
                      <label>
                        Style
                        <select value={selectedStyleID} onChange={(event) => setSelectedStyleID(event.target.value)} disabled={runningAction || acceptingAction || rejectingAction}>
                          {styles.filter((style) => {
                            const selectedAction = availableActions.find((item) => item.agent_id === selectedAgentID) ?? availableActions[0]
                            return selectedAction?.style_ids.includes(style.id)
                          }).map((style) => (
                            <option key={style.id} value={style.id}>{style.name}</option>
                          ))}
                        </select>
                      </label>
                      <div className="actions">
                        <button type="button" className="secondary" onClick={() => void submitPreviewContext('selection', selectedAgentID, selectedStyleID)} disabled={actionWorkflow.previewLoading || !selectedAgentID || !selectedStyleID}>
                          {actionWorkflow.previewLoading && activePreviewScope === 'selection' ? 'Previewing context...' : 'Preview context'}
                        </button>
                        <button type="button" onClick={() => requestScopedRun('selection', selectedAgentID, selectedStyleID)} disabled={runningAction || !selectedAgentID || !selectedStyleID}>
                          {runningAction ? 'Running action...' : 'Run action'}
                        </button>
                      </div>
                    </>
                  )}
                </div>
              )}
            </section>

            <section className="ai-actions-panel" aria-label="Scene AI actions">
              <div className="scene-editor-header">
                <strong>Scene Actions</strong>
                <button type="button" className="secondary" onClick={() => void openSceneActions()} disabled={!canOpenSceneActions}>
                  Rewrite scene
                </button>
              </div>
              <p className="section-note">
                {actionDisableReason ?? `Scene contains ${sceneWordCount} canonical words.`}
              </p>
              {sceneActionsOpen && (
                <div className="ai-actions-workflow">
                  {sceneActionsLoading && <p className="outline-message">Loading scene actions...</p>}
                  {!sceneActionsLoading && sceneAvailableActions.length === 0 && (
                    <p className="outline-message">No applicable scene actions.</p>
                  )}
                  {!sceneActionsLoading && sceneAvailableActions.length > 0 && (
                    <>
                      <label>
                        Agent
                        <select value={selectedSceneAgentID} onChange={(event) => setSelectedSceneAgentID(event.target.value)}>
                          {sceneAvailableActions.map((item) => (
                            <option key={item.agent_id} value={item.agent_id}>{item.name}</option>
                          ))}
                        </select>
                      </label>
                      <label>
                        Style
                        <select value={selectedSceneStyleID} onChange={(event) => setSelectedSceneStyleID(event.target.value)}>
                          {styles.filter((style) => {
                            const selectedAction = sceneAvailableActions.find((item) => item.agent_id === selectedSceneAgentID) ?? sceneAvailableActions[0]
                            return selectedAction?.style_ids.includes(style.id)
                          }).map((style) => (
                            <option key={style.id} value={style.id}>{style.name}</option>
                          ))}
                        </select>
                      </label>
                      <div className="actions">
                        <button type="button" className="secondary" onClick={() => void submitPreviewContext('scene', selectedSceneAgentID, selectedSceneStyleID)} disabled={actionWorkflow.previewLoading || !selectedSceneAgentID || !selectedSceneStyleID}>
                          {actionWorkflow.previewLoading && activePreviewScope === 'scene' ? 'Previewing context...' : 'Preview context'}
                        </button>
                        <button type="button" onClick={() => requestScopedRun('scene', selectedSceneAgentID, selectedSceneStyleID)} disabled={runningAction || !selectedSceneAgentID || !selectedSceneStyleID}>
                          {runningAction ? 'Rewriting scene...' : 'Run scene rewrite'}
                        </button>
                      </div>
                    </>
                  )}
                </div>
              )}
            </section>

            <section className="ai-actions-panel" aria-label="Chapter review actions">
              <div className="scene-editor-header">
                <strong>Chapter Review</strong>
                <button type="button" className="secondary" onClick={() => void openChapterReview()} disabled={!canOpenChapterReview}>
                  Review chapter
                </button>
              </div>
              {chapterReviewOpen && (
                <div className="ai-actions-workflow">
                  {!chapterReviewAction && <p className="outline-message">No chapter review action is available.</p>}
                  {chapterReviewAction && (
                    <>
                      <label>
                        Style
                        <select value={selectedChapterStyleID} onChange={(event) => setSelectedChapterStyleID(event.target.value)}>
                          {styles.filter((style) => chapterReviewAction.style_ids.includes(style.id)).map((style) => (
                            <option key={style.id} value={style.id}>{style.name}</option>
                          ))}
                        </select>
                      </label>
                      <div className="actions">
                        <button type="button" className="secondary" onClick={() => void submitPreviewContext('chapter_review', chapterReviewAction.agent_id, selectedChapterStyleID)} disabled={actionWorkflow.previewLoading || !selectedChapterStyleID}>
                          {actionWorkflow.previewLoading && activePreviewScope === 'chapter_review' ? 'Previewing context...' : 'Preview context'}
                        </button>
                        <button type="button" onClick={() => requestScopedRun('chapter_review', chapterReviewAction.agent_id, selectedChapterStyleID)} disabled={runningAction || !selectedChapterStyleID}>
                          {runningAction ? 'Reviewing chapter...' : 'Run chapter review'}
                        </button>
                      </div>
                    </>
                  )}
                </div>
              )}
            </section>

            {previewManifest && (
              <ContextPreview
                manifest={previewManifest}
                loading={actionWorkflow.previewLoading}
                error={actionWorkflow.previewError}
              />
            )}

            {activeRun && (
              <div ref={previewRegionRef} className="ai-preview" role="region" aria-label="AI action preview" tabIndex={-1}>
                {isSuggestionResponse(activeRun) ? (
                  <ChapterReview findings={activeRun.findings} />
                ) : (
                  <div className="ai-preview-grid">
                    <div>
                      <span className="section-label">Original</span>
                      <pre>{activeRun.patch.original}</pre>
                    </div>
                    <div>
                      <span className="section-label">Replacement</span>
                      <pre>{activeRun.patch.replacement}</pre>
                    </div>
                  </div>
                )}
                {!isSuggestionResponse(activeRun) && (
                  <p className="section-note">
                    Context packs: {activeRun.context_summary.packs_used.join(', ')}. RAG mode: {activeRun.context_summary.rag_mode}. Provider: {activeRun.provider.profile_id} ({activeRun.provider.type}, model {activeRun.provider.model}).
                  </p>
                )}
                {!isSuggestionResponse(activeRun) && (
                  <div className="actions">
                    <button type="button" className="secondary" onClick={() => void copyReplacement()}>Copy replacement</button>
                    <button type="button" className="secondary" onClick={() => void submitRejectAction()} disabled={rejectingAction || acceptingAction}>
                      {rejectingAction ? 'Rejecting...' : 'Reject replacement'}
                    </button>
                    <button type="button" onClick={() => void submitAcceptAction()} disabled={acceptingAction || rejectingAction}>
                      {acceptingAction ? 'Accepting...' : 'Accept replacement'}
                    </button>
                  </div>
                )}
              </div>
            )}

            <FollowUpInvitations
              invitations={actionWorkflow.invitations}
              loadingInvitationID={actionWorkflow.invitationLoadingID}
              error={actionWorkflow.invitationError}
              onRun={(invitation) => {
                const styleID = selectedSceneStyleID || selectedStyleID || selectedChapterStyleID
                if (scopeRank(invitation.scope) > scopeRank(activePreviewScope ?? 'selection')) {
                  setPendingScopeAction({
                    scope: invitation.scope,
                    agentID: invitation.agent_id,
                    styleID,
                    invitation,
                  })
                  return
                }
                void submitInvitationRun(invitation)
              }}
            />

            <div className="actions">
              <button type="button" onClick={() => void submitSave()} disabled={!canSave}>
                Save scene
              </button>
              <button type="button" className="secondary" onClick={requestReloadCanonical} disabled={loading || saving}>
                Reload canonical
              </button>
            </div>
          </div>
        )}
      </div>
      <ConfirmDialog
        open={confirmReload}
        title="Discard scene draft?"
        message="You have unsaved scene changes. Discard them and reload canonical content?"
        confirmLabel="Discard draft"
        onCancel={() => setConfirmReload(false)}
        onConfirm={() => {
          setConfirmReload(false)
          void reloadCanonical()
        }}
      />
      <ConfirmDialog
        open={pendingScopeAction !== null}
        title="Broaden action scope?"
        message={pendingScopeAction
          ? `Run ${pendingScopeAction.agentID} at ${pendingScopeAction.scope} scope? This sends broader canonical context than paragraph work and requires an explicit provider call.`
          : ''}
        confirmLabel="Run broader action"
        onCancel={() => setPendingScopeAction(null)}
        onConfirm={() => {
          const pending = pendingScopeAction
          setPendingScopeAction(null)
          if (!pending) {
            return
          }
          if (pending.invitation) {
            void submitInvitationRun(pending.invitation)
            return
          }
          void submitScopedRun(pending.scope, pending.agentID, pending.styleID, pending.invitation)
        }}
      />
    </section>
  )
}