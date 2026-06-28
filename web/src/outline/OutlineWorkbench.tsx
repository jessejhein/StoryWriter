import { FormEvent, useEffect, useState } from 'react'
import {
  createArc,
  createChapter,
  createScene,
  getOutline,
  reorderOutline,
  type Arc,
  type Chapter,
  type Outline,
  type Project,
} from '../api'

type FormState =
  | { type: 'arc' }
  | { type: 'chapter'; arcID: string; arcTitle: string }
  | { type: 'scene'; chapterID: string; chapterTitle: string }

type Props = {
  project: Project
  onOpenScene?: (sceneID: string) => void
}

export default function OutlineWorkbench({ project, onOpenScene }: Props) {
  const [outline, setOutline] = useState<Outline | null>(null)
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [form, setForm] = useState<FormState | null>(null)
  const [title, setTitle] = useState('')

  useEffect(() => {
    void loadOutline()
  }, [project.path])

  async function loadOutline() {
    setLoading(true)
    setError('')
    try {
      setOutline(await getOutline())
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : 'Outline request failed')
    } finally {
      setLoading(false)
    }
  }

  async function submit(event: FormEvent) {
    event.preventDefault()
    if (!form || busy) {
      return
    }

    setBusy(true)
    setError('')
    try {
      const result = form.type === 'arc'
        ? await createArc(title)
        : form.type === 'chapter'
          ? await createChapter(form.arcID, title)
          : await createScene(form.chapterID, title)
      setOutline(result.outline)
      setForm(null)
      setTitle('')
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : 'Mutation failed')
    } finally {
      setBusy(false)
    }
  }

  async function moveChapter(arc: Arc, chapterIndex: number, direction: -1 | 1) {
    const targetIndex = chapterIndex + direction
    if (busy || targetIndex < 0 || targetIndex >= arc.chapters.length) {
      return
    }
    const orderedChildIDs = arc.chapters.map((chapter) => chapter.id)
    ;[orderedChildIDs[chapterIndex], orderedChildIDs[targetIndex]] = [orderedChildIDs[targetIndex], orderedChildIDs[chapterIndex]]
    await runReorder({ parent_type: 'arc', parent_id: arc.id, ordered_child_ids: orderedChildIDs })
  }

  async function moveScene(chapter: Chapter, sceneIndex: number, direction: -1 | 1) {
    const targetIndex = sceneIndex + direction
    if (busy || targetIndex < 0 || targetIndex >= chapter.scenes.length) {
      return
    }
    const orderedChildIDs = chapter.scenes.map((scene) => scene.id)
    ;[orderedChildIDs[sceneIndex], orderedChildIDs[targetIndex]] = [orderedChildIDs[targetIndex], orderedChildIDs[sceneIndex]]
    await runReorder({ parent_type: 'chapter', parent_id: chapter.id, ordered_child_ids: orderedChildIDs })
  }

  async function runReorder(requestBody: Parameters<typeof reorderOutline>[0]) {
    setBusy(true)
    setError('')
    try {
      const result = await reorderOutline(requestBody)
      setOutline(result.outline)
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : 'Reorder failed')
    } finally {
      setBusy(false)
    }
  }

  function openForm(nextForm: FormState) {
    setForm(nextForm)
    setTitle('')
    setError('')
  }

  return (
    <section className="outline-shell" aria-live="polite">
      <div className="outline-meta">
        <p className="folio">Milestone 1 / Outline</p>
        <h2>Shape the story hierarchy.</h2>
        <p>Canonical arcs, chapters, and scenes stay in plain files while every structural mutation creates a checkpoint.</p>
      </div>

      <div className="outline-panel">
        <div className="outline-toolbar">
          <div>
            <span className="section-label">Active project</span>
            <strong>{project.name ?? project.project_id}</strong>
            <code>{project.path}</code>
          </div>
          <button type="button" className="secondary" onClick={() => void loadOutline()} disabled={loading || busy}>
            Refresh
          </button>
        </div>

        {loading && <p className="outline-message">Loading outline...</p>}

        {!loading && error && (
          <div className="outline-error" role="alert">
            <span>{error}</span>
            <button type="button" className="secondary" onClick={() => void loadOutline()} disabled={busy}>
              Retry
            </button>
          </div>
        )}

        {!loading && !error && outline && outline.arcs.length === 0 && (
          <div className="empty-outline">
            <p>No arcs yet</p>
            <p>Start with the first major movement of the novel, then nest chapters and scenes under it.</p>
            <button type="button" onClick={() => openForm({ type: 'arc' })} disabled={busy}>
              Add arc
            </button>
            {form?.type === 'arc' && renderForm(submit, title, setTitle, busy, () => setForm(null))}
          </div>
        )}

        {!loading && !error && outline && outline.arcs.length > 0 && (
          <>
            <div className="outline-header-row">
              <h3>Outline</h3>
              <button type="button" onClick={() => openForm({ type: 'arc' })} disabled={busy}>
                Add arc
              </button>
            </div>
            {form?.type === 'arc' && renderForm(submit, title, setTitle, busy, () => setForm(null))}
            <ol className="outline-tree">
              {outline.arcs.map((arc) => (
                <li key={arc.id} className="outline-arc">
                  <div className="outline-card">
                    <div>
                      <span className="section-label">{arc.display_label}</span>
                      <strong>{arc.title}</strong>
                    </div>
                    <button type="button" className="secondary" onClick={() => openForm({ type: 'chapter', arcID: arc.id, arcTitle: arc.title })} disabled={busy}>
                      Add chapter in {arc.title}
                    </button>
                  </div>
                  {form?.type === 'chapter' && form.arcID === arc.id && renderForm(submit, title, setTitle, busy, () => setForm(null))}
                  <ol className="chapter-list">
                    {arc.chapters.map((chapter, chapterIndex) => (
                      <li key={chapter.id} className="outline-chapter">
                        <div className="outline-card chapter-card">
                          <div>
                            <span className="section-label">{chapter.display_label}</span>
                            <strong>{chapter.title}</strong>
                          </div>
                          <div className="chapter-actions">
                            <button type="button" className="secondary" onClick={() => void moveChapter(arc, chapterIndex, -1)} disabled={busy || chapterIndex === 0}>
                              Move chapter {chapter.title} up
                            </button>
                            <button type="button" className="secondary" onClick={() => void moveChapter(arc, chapterIndex, 1)} disabled={busy || chapterIndex === arc.chapters.length - 1}>
                              Move chapter {chapter.title} down
                            </button>
                            <button type="button" className="secondary" onClick={() => openForm({ type: 'scene', chapterID: chapter.id, chapterTitle: chapter.title })} disabled={busy}>
                              Add scene in {chapter.title}
                            </button>
                          </div>
                        </div>
                        {form?.type === 'scene' && form.chapterID === chapter.id && renderForm(submit, title, setTitle, busy, () => setForm(null))}
                        {chapter.scenes.length > 0 && (
                          <ol className="scene-list">
                            {chapter.scenes.map((scene, sceneIndex) => (
                              <li key={scene.id} className="scene-row">
                                <div>
                                  <span className="section-label">{scene.display_label}</span>
                                  <strong>{scene.title}</strong>
                                </div>
                                <div className="scene-actions">
                                  <button type="button" className="secondary" onClick={() => onOpenScene?.(scene.id)} disabled={busy}>
                                    Open scene {scene.title}
                                  </button>
                                  <button type="button" className="secondary" onClick={() => void moveScene(chapter, sceneIndex, -1)} disabled={busy || sceneIndex === 0}>
                                    Move scene {scene.title} up
                                  </button>
                                  <button type="button" className="secondary" onClick={() => void moveScene(chapter, sceneIndex, 1)} disabled={busy || sceneIndex === chapter.scenes.length - 1}>
                                    Move scene {scene.title} down
                                  </button>
                                </div>
                              </li>
                            ))}
                          </ol>
                        )}
                      </li>
                    ))}
                  </ol>
                </li>
              ))}
            </ol>
          </>
        )}
      </div>
    </section>
  )
}

function renderForm(
  submit: (event: FormEvent) => Promise<void>,
  title: string,
  setTitle: (value: string) => void,
  busy: boolean,
  cancel: () => void,
) {
  return (
    <form className="inline-form" onSubmit={(event) => void submit(event)}>
      <label>
        Title
        <input value={title} onChange={(event) => setTitle(event.target.value)} placeholder="Enter title" />
      </label>
      <div className="actions">
        <button type="submit" disabled={busy || !title.trim()}>
          Save
        </button>
        <button type="button" className="secondary" onClick={cancel} disabled={busy}>
          Cancel
        </button>
      </div>
    </form>
  )
}
