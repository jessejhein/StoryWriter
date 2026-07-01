/**
 * SceneEditor.tsx
 *
 * Hosts the Milestone 2 canonical scene editor. It loads one scene by stable
 * ID, tracks editable metadata and markdown, and performs explicit saves with
 * optimistic revision checks.
 */

import { useEffect, useRef, useState } from 'react'
import type { AvailableAction, Project, RunActionResponse, SaveSceneRequest, SceneDocument, StyleDefinition } from '../api'
import { APIError, acceptAction, getAvailableActions, getScene, getStyles, rejectAction, runAction, saveScene } from '../api'
import ConfirmDialog from '../components/ConfirmDialog'
import CodeMirrorSurface from './CodeMirrorSurface'
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

/**
 * SceneEditor
 *
 * Renders the scene metadata form, CodeMirror editor surface, and save or
 * reload actions for one canonical scene document.
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
  const [actionsLoading, setActionsLoading] = useState(false)
  const [availableActions, setAvailableActions] = useState<AvailableAction[]>([])
  const [styles, setStyles] = useState<StyleDefinition[]>([])
  const [selectedAgentID, setSelectedAgentID] = useState('')
  const [selectedStyleID, setSelectedStyleID] = useState('')
  const [runningAction, setRunningAction] = useState(false)
  const [previewRun, setPreviewRun] = useState<RunActionResponse | null>(null)
  const [acceptingAction, setAcceptingAction] = useState(false)
  const [rejectingAction, setRejectingAction] = useState(false)
  const [actionFeedback, setActionFeedback] = useState<ActionFeedback>(null)
  const sceneVersionRef = useRef(0)
  const previewRegionRef = useRef<HTMLDivElement | null>(null)
  const previewRunID = previewRun?.run_id ?? null

  const dirty = isDraftDirty(baseline, draft)
  const validationError = validateDraft(draft)
  const canSave = !loading && !saving && dirty && !validationError && baseline !== null
  const selectedWordCount = countWords(selection.text)
  const actionDisableReason = loading
    ? 'Scene is loading.'
    : saving
      ? 'Wait for the current save to finish.'
      : dirty
        ? 'Save or reload the scene before running AI actions.'
        : !baseline || !draft
          ? 'Scene is not loaded yet.'
          : !selection.text.trim()
            ? 'Select canonical scene text to discover actions.'
            : runningAction || acceptingAction || rejectingAction
              ? 'Wait for the current action request to finish.'
              : null
  const canOpenActions = actionDisableReason === null

  useEffect(() => {
    let cancelled = false
    sceneVersionRef.current += 1
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
        setAvailableActions([])
        setStyles([])
        setSelectedAgentID('')
        setSelectedStyleID('')
        setPreviewRun(null)
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
    if (previewRunID === null) {
      return
    }
    previewRegionRef.current?.focus()
  }, [previewRunID])

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
    } catch (requestError) {
      setFeedback({
        kind: 'error',
        message: requestError instanceof Error ? requestError.message : 'Scene request failed',
      })
    } finally {
      setLoading(false)
    }
  }

  async function openActions() {
    if (!baseline || !draft || !canOpenActions) {
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

  async function submitRunAction() {
    if (!baseline || !draft || !selectedAgentID || !selectedStyleID || !selection.text.trim()) {
      return
    }
    setRunningAction(true)
    setActionFeedback(null)
    const sceneVersion = sceneVersionRef.current
    try {
      const byteRange = toUTF8ByteRange(draft.markdown, selection.start, selection.end)
      const response = await runAction({
        agent_id: selectedAgentID,
        style_id: selectedStyleID,
        surface: 'editor',
        input_scope: 'selection',
        scene_id: baseline.id,
        scene_revision: baseline.revision,
        selection: {
          start_byte: byteRange.startByte,
          end_byte: byteRange.endByte,
          text: selection.text,
        },
      })
      if (sceneVersion !== sceneVersionRef.current) {
        return
      }
      setPreviewRun(response)
    } catch (requestError) {
      if (sceneVersion !== sceneVersionRef.current) {
        return
      }
      setActionFeedback({
        kind: requestError instanceof APIError && requestError.status === 409 ? 'conflict' : 'error',
        message: requestError instanceof Error ? requestError.message : 'Action run failed',
      })
    } finally {
      if (sceneVersion === sceneVersionRef.current) {
        setRunningAction(false)
      }
    }
  }

  async function submitAcceptAction() {
    if (!previewRun || !baseline) {
      return
    }
    setAcceptingAction(true)
    setActionFeedback(null)
    const sceneVersion = sceneVersionRef.current
    try {
      const response = await acceptAction(previewRun.run_id, baseline.revision)
      if (sceneVersion !== sceneVersionRef.current) {
        return
      }
      setBaseline(response.scene)
      setDraft(toDraft(response.scene))
      setPreviewRun(null)
      setActionsOpen(false)
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
    if (!previewRun) {
      return
    }
    setRejectingAction(true)
    setActionFeedback(null)
    const sceneVersion = sceneVersionRef.current
    try {
      await rejectAction(previewRun.run_id)
      if (sceneVersion !== sceneVersionRef.current) {
        return
      }
      setPreviewRun(null)
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
    if (!previewRun?.patch.replacement || !navigator.clipboard) {
      return
    }
    await navigator.clipboard.writeText(previewRun.patch.replacement)
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

  return (
    <section className="editor-shell" aria-live="polite">
      <div className="editor-meta">
        <p className="folio">Milestone 2 / Scene editor</p>
        <h2>Edit accepted canon without silent overwrites.</h2>
        <p>Load one canonical scene, revise supported metadata and Markdown, then checkpoint exactly one accepted save.</p>
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

            <section className="ai-actions-panel" aria-label="Scene AI actions">
              <div className="scene-editor-header">
                <strong>Selection Actions</strong>
                <button type="button" className="secondary" onClick={() => void openActions()} disabled={!canOpenActions}>
                  AI actions
                </button>
              </div>
              <p className="section-note">
                {actionDisableReason ?? `Selected ${selectedWordCount} words from canonical scene text.`}
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
                        <button type="button" onClick={() => void submitRunAction()} disabled={runningAction || !selectedAgentID || !selectedStyleID}>
                          {runningAction ? 'Running action...' : 'Run action'}
                        </button>
                      </div>
                    </>
                  )}
                  {previewRun && (
                    <div ref={previewRegionRef} className="ai-preview" role="region" aria-label="AI patch preview" tabIndex={-1}>
                      <div className="ai-preview-grid">
                        <div>
                          <span className="section-label">Original</span>
                          <pre>{previewRun.patch.original}</pre>
                        </div>
                        <div>
                          <span className="section-label">Replacement</span>
                          <pre>{previewRun.patch.replacement}</pre>
                        </div>
                      </div>
                      <p className="section-note">
                        Context packs: {previewRun.context_summary.packs_used.join(', ')}. RAG mode: {previewRun.context_summary.rag_mode}. Provider: {previewRun.provider.profile_id} ({previewRun.provider.type}, model {previewRun.provider.model}).
                      </p>
                      <div className="actions">
                        <button type="button" className="secondary" onClick={() => void copyReplacement()}>Copy replacement</button>
                        <button type="button" className="secondary" onClick={() => void submitRejectAction()} disabled={rejectingAction || acceptingAction}>
                          {rejectingAction ? 'Rejecting...' : 'Reject replacement'}
                        </button>
                        <button type="button" onClick={() => void submitAcceptAction()} disabled={acceptingAction || rejectingAction}>
                          {acceptingAction ? 'Accepting...' : 'Accept replacement'}
                        </button>
                      </div>
                    </div>
                  )}
                </div>
              )}
            </section>

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
    </section>
  )
}
