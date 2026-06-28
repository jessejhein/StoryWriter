import { FormEvent, useEffect, useState } from 'react'
import { createProject, getHealth, openProject, type Project } from './api'
import './styles.css'

export default function App() {
  const [health, setHealth] = useState('Connecting')
  const [name, setName] = useState('')
  const [path, setPath] = useState('')
  const [project, setProject] = useState<Project | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    getHealth()
      .then(({ version }) => setHealth(`Online · ${version}`))
      .catch(() => setHealth('Backend unavailable'))
  }, [])

  async function submit(event: FormEvent, mode: 'create' | 'open') {
    event.preventDefault()
    setError('')
    try {
      setProject(mode === 'create' ? await createProject(name, path) : await openProject(path))
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : 'Request failed')
    }
  }

  return (
    <main>
      <header>
        <p className="eyebrow">Local-first writing environment</p>
        <h1>AI Story Workshop</h1>
        <span className="status">{health}</span>
      </header>

      <section className="workbench">
        <div className="intro">
          <p className="folio">Milestone 0 / Foundation</p>
          <h2>Give the story a durable home.</h2>
          <p>Create a portable folder backed by plain text, Git history, and a disposable local index.</p>
        </div>

        <form>
          <label>
            Project name
            <input value={name} onChange={(event) => setName(event.target.value)} placeholder="The Glass Cartographer" />
          </label>
          <label>
            Absolute folder path
            <input value={path} onChange={(event) => setPath(event.target.value)} placeholder="/home/you/Stories/glass-cartographer" required />
          </label>
          <div className="actions">
            <button type="submit" onClick={(event) => void submit(event, 'create')} disabled={!name || !path}>Create project</button>
            <button className="secondary" type="submit" onClick={(event) => void submit(event, 'open')} disabled={!path}>Open existing</button>
          </div>
          {error && <p className="error" role="alert">{error}</p>}
        </form>
      </section>

      {project && (
        <aside aria-live="polite">
          <span>Project ready</span>
          <strong>{project.name ?? project.project_id}</strong>
          <code>{project.path}</code>
        </aside>
      )}
    </main>
  )
}
