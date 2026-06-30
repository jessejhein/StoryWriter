import type { OutlineScene } from '../api'
import type { ProgressionRow } from './workbenchTypes'

type Props = {
  progressions: ProgressionRow[]
  scenes: OutlineScene[]
  status: string
  saving: boolean
  dirty: boolean
  onChange: (nextRows: ProgressionRow[]) => void
  onSave: () => void
}

function moveItem<T>(items: T[], from: number, to: number): T[] {
  if (to < 0 || to >= items.length || from === to) {
    return items
  }
  const next = [...items]
  const [item] = next.splice(from, 1)
  next.splice(to, 0, item)
  return next
}

export default function ProgressionList({ progressions, scenes, status, saving, dirty, onChange, onSave }: Props) {
  function updateProgression(index: number, field: 'sceneID' | 'timing', value: string) {
    const next = [...progressions]
    next[index] = { ...next[index], [field]: value }
    onChange(next)
  }

  function updateDescription(index: number, value: string) {
    const next = [...progressions]
    next[index] = { ...next[index], hasDescription: true, description: value }
    onChange(next)
  }

  function removeDescription(index: number) {
    const next = [...progressions]
    next[index] = { ...next[index], hasDescription: false, description: '' }
    onChange(next)
  }

  function updateMetadata(index: number, metadataIndex: number, field: 'key' | 'value', value: string) {
    const next = [...progressions]
    const metadata = [...next[index].metadata]
    metadata[metadataIndex] = { ...metadata[metadataIndex], [field]: value }
    next[index] = { ...next[index], metadata }
    onChange(next)
  }

  function addProgressionMetadata(index: number) {
    const next = [...progressions]
    next[index] = { ...next[index], metadata: [...next[index].metadata, { key: '', value: '' }] }
    onChange(next)
  }

  function removeProgressionMetadata(index: number, metadataIndex: number) {
    const next = [...progressions]
    next[index] = { ...next[index], metadata: next[index].metadata.filter((_, currentIndex) => currentIndex !== metadataIndex) }
    onChange(next)
  }

  return (
    <section>
      <header className="section-heading">
        <h4>Progressions</h4>
        <div>
          <span>{status}</span>
          <button type="button" onClick={() => onChange([...progressions, { sceneID: scenes[0]?.id ?? '', timing: 'after', hasDescription: false, description: '', metadata: [] }])}>
            Add progression
          </button>
          <button type="button" onClick={onSave} disabled={saving || !dirty}>
            {saving ? 'Saving…' : 'Save progressions'}
          </button>
        </div>
      </header>
      {progressions.map((progression, index) => (
        <div key={progression.id ?? `new-${index}`} className="progression-row">
          <div className="section-heading">
            <h5>Progression {index + 1}</h5>
            <div>
              <button type="button" onClick={() => onChange(moveItem(progressions, index, index - 1))} disabled={index === 0}>Move progression {index + 1} up</button>
              <button type="button" onClick={() => onChange(moveItem(progressions, index, index + 1))} disabled={index === progressions.length - 1}>Move progression {index + 1} down</button>
              <button type="button" onClick={() => onChange(progressions.filter((_, currentIndex) => currentIndex !== index))}>Remove progression {index + 1}</button>
            </div>
          </div>
          <label>
            Scene
            <select value={progression.sceneID} onChange={(event) => updateProgression(index, 'sceneID', event.target.value)}>
              {scenes.map((scene) => (
                <option key={scene.id} value={scene.id}>{scene.title}</option>
              ))}
            </select>
          </label>
          <label>
            Timing
            <select value={progression.timing} onChange={(event) => updateProgression(index, 'timing', event.target.value as 'before' | 'after')}>
              <option value="before">Before</option>
              <option value="after">After</option>
            </select>
          </label>
          <label>
            Progression description
            <textarea value={progression.description} onChange={(event) => updateDescription(index, event.target.value)} />
          </label>
          {progression.hasDescription && (
            <button type="button" onClick={() => removeDescription(index)}>Remove description change</button>
          )}
          <div>
            <div className="section-heading">
              <h5>Progression metadata</h5>
              <button type="button" onClick={() => addProgressionMetadata(index)}>Add progression metadata</button>
            </div>
            {progression.metadata.map((row, metadataIndex) => (
              <div key={`progression-${index}-metadata-${metadataIndex}`} className="metadata-row">
                <label>
                  Progression metadata key {metadataIndex + 1}
                  <input value={row.key} onChange={(event) => updateMetadata(index, metadataIndex, 'key', event.target.value)} />
                </label>
                <label>
                  Progression metadata value {metadataIndex + 1}
                  <input value={row.value} onChange={(event) => updateMetadata(index, metadataIndex, 'value', event.target.value)} />
                </label>
                <button type="button" onClick={() => removeProgressionMetadata(index, metadataIndex)}>
                  Remove progression metadata {metadataIndex + 1}
                </button>
              </div>
            ))}
          </div>
        </div>
      ))}
    </section>
  )
}
