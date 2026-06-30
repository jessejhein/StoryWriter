/**
 * SceneEditor.tsx
 *
 * Hosts the Milestone 2 canonical scene editor. It loads one scene by stable
 * ID, tracks editable metadata and markdown, and performs explicit saves with
 * optimistic revision checks.
 */

import { useEffect, useState } from 'react'
import type { Project, SaveSceneRequest, SceneDocument } from '../api'
import { APIError, getScene, saveScene } from '../api'
import ConfirmDialog from '../components/ConfirmDialog'
import CodeMirrorSurface from './CodeMirrorSurface'

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

  const dirty = isDraftDirty(baseline, draft)
  const validationError = validateDraft(draft)
  const canSave = !loading && !saving && dirty && !validationError && baseline !== null

  useEffect(() => {
    let cancelled = false
    void (async () => {
      try {
        const scene = await getScene(sceneID)
        if (cancelled) {
          return
        }
        setBaseline(scene)
        setDraft(toDraft(scene))
        setFeedback(null)
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
              <CodeMirrorSurface value={draft.markdown} onChange={(markdown) => patchDraft({ markdown })} />
            </div>

            {validationError && <p className="error" role="alert">{validationError}</p>}

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
