/**
 * App.tsx
 *
 * Hosts the local-first Storywork shell. It creates or opens a project,
 * switches between the outline, scene editor, and Codex workbench views,
 * and centralizes cross-view dirty-state navigation guards.
 */

import { FormEvent, useEffect, useState } from 'react'
import { createProject, getHealth, openProject, type Project } from './api'
import ConfirmDialog from './components/ConfirmDialog'
import CodexWorkbench from './codex/CodexWorkbench'
import SceneEditor from './editor/SceneEditor'
import OutlineWorkbench from './outline/OutlineWorkbench'
import './styles.css'

type ProjectView =
  | { mode: 'outline' }
  | { mode: 'codex' }
  | { mode: 'scene'; sceneID: string }

/**
 * App
 *
 * Renders the root Storywork UI and coordinates project-level navigation.
 */
export default function App() {
  const [health, setHealth] = useState('Connecting')
  const [name, setName] = useState('')
  const [path, setPath] = useState('')
  const [project, setProject] = useState<Project | null>(null)
  const [view, setView] = useState<ProjectView>({ mode: 'outline' })
  const [dirty, setDirty] = useState(false)
  const [error, setError] = useState('')
  const [pendingView, setPendingView] = useState<ProjectView | null>(null)

  useEffect(() => {
    getHealth()
      .then(({ version }) => setHealth(`Online · ${version}`))
      .catch(() => setHealth('Backend unavailable'))
  }, [])

  useEffect(() => {
    if (!dirty) {
      return
    }
    function handleBeforeUnload(event: BeforeUnloadEvent) {
      event.preventDefault()
      event.returnValue = ''
    }
    window.addEventListener('beforeunload', handleBeforeUnload)
    return () => {
      window.removeEventListener('beforeunload', handleBeforeUnload)
    }
  }, [dirty])

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const submitter = (event.nativeEvent as SubmitEvent).submitter as HTMLButtonElement | null
    const mode = submitter?.value === 'open' ? 'open' : 'create'
    setError('')
    try {
      setProject(mode === 'create' ? await createProject(name, path) : await openProject(path))
      setView({ mode: 'outline' })
      setDirty(false)
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : 'Request failed')
    }
  }

  function navigate(nextView: ProjectView) {
    if (dirty && !sameProjectView(view, nextView)) {
      setPendingView(nextView)
      return
    }
    setView(nextView)
    setDirty(false)
  }

  function confirmNavigation() {
    if (!pendingView) {
      return
    }
    setView(pendingView)
    setPendingView(null)
    setDirty(false)
  }

  return (
    <main>
      <header>
        <p className="eyebrow">Local-first writing environment</p>
        <h1>AI Story Workshop</h1>
        <span className="status">{health}</span>
      </header>

      {!project && (
        <section className="workbench">
          <div className="intro">
            <p className="folio">Milestone 1 / Outline</p>
            <h2>Give the story a durable home, then shape it.</h2>
            <p>Create or open a portable folder backed by plain text, Git history, a disposable local index, and a structured outline workbench.</p>
          </div>

          <form onSubmit={(event) => void submit(event)}>
            <label>
              Project name
              <input value={name} onChange={(event) => setName(event.target.value)} placeholder="The Glass Cartographer" />
            </label>
            <label>
              Absolute folder path
              <input value={path} onChange={(event) => setPath(event.target.value)} placeholder="/home/you/Stories/glass-cartographer" required />
            </label>
            <div className="actions">
              <button type="submit" value="create" disabled={!name || !path}>Create project</button>
              <button className="secondary" type="submit" value="open" disabled={!path}>Open existing</button>
            </div>
            {error && <p className="error" role="alert">{error}</p>}
          </form>
        </section>
      )}

      {project && (
        <>
          <nav className="project-nav">
            <button type="button" onClick={() => navigate({ mode: 'outline' })}>Outline</button>
            <button type="button" onClick={() => navigate({ mode: 'codex' })}>Codex</button>
          </nav>
          {view.mode === 'outline' ? (
            <OutlineWorkbench project={project} onOpenScene={(sceneID) => navigate({ mode: 'scene', sceneID })} />
          ) : view.mode === 'scene' ? (
            <SceneEditor
              key={view.sceneID}
              project={project}
              sceneID={view.sceneID}
              onBack={() => navigate({ mode: 'outline' })}
              onDirtyChange={setDirty}
            />
          ) : (
            <CodexWorkbench project={project} onDirtyChange={setDirty} />
          )}
          <ConfirmDialog
            open={pendingView !== null}
            title="Discard current draft?"
            message="You have unsaved changes in the current workspace. Discard them and continue?"
            confirmLabel="Discard draft"
            onConfirm={confirmNavigation}
            onCancel={() => setPendingView(null)}
          />
        </>
      )}
    </main>
  )
}

function sameProjectView(left: ProjectView, right: ProjectView) {
  return left.mode === right.mode && (left.mode !== 'scene' || right.mode !== 'scene' || left.sceneID === right.sceneID)
}
