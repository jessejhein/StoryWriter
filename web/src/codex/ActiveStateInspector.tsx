import type { CodexActiveState, OutlineScene } from '../api'

type Props = {
  scenes: OutlineScene[]
  activeSceneID: string
  activeState: CodexActiveState | null
  onChangeSceneID: (sceneID: string) => void
}

export default function ActiveStateInspector({ scenes, activeSceneID, activeState, onChangeSceneID }: Props) {
  return (
    <section>
      <header className="section-heading">
        <h4>Active state</h4>
      </header>
      <label>
        Scene selector
        <select value={activeSceneID} onChange={(event) => onChangeSceneID(event.target.value)}>
          {scenes.map((scene) => (
            <option key={scene.id} value={scene.id}>{scene.title}</option>
          ))}
        </select>
      </label>
      {activeState && (
        <div>
          <p>{activeState.entry.description}</p>
          <dl>
            {Object.entries(activeState.entry.metadata).map(([key, value]) => (
              <div key={key}>
                <dt>{key}</dt>
                <dd>{value}</dd>
              </div>
            ))}
          </dl>
          <p>Applied progressions: {activeState.applied_progression_ids.join(', ') || 'None'}</p>
        </div>
      )}
    </section>
  )
}
