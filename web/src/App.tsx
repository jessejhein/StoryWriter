import { FormEvent, useEffect, useState } from 'react'
import { createProject, getHealth, openProject, type Project } from './api'
import OutlineWorkbench from './outline/OutlineWorkbench'
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

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const submitter = (event.nativeEvent as SubmitEvent).submitter as HTMLButtonElement | null
    const mode = submitter?.value === 'open' ? 'open' : 'create'
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

      {project && <OutlineWorkbench project={project} />}
    </main>
  )
}
