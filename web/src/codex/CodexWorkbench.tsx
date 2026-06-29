/**
 * CodexWorkbench.tsx
 *
 * Provides the Milestone 3 Codex workspace for managing canonical entries,
 * editing ordered progressions, and inspecting resolved active state at a
 * selected scene.
 */

import { ChangeEvent, useEffect, useState } from 'react'
import {
  APIError,
  createCodexEntry,
  getCodexActiveState,
  getCodexEntries,
  getCodexEntry,
  getCodexProgressions,
  getOutline,
  saveCodexProgressions,
  updateCodexEntry,
  type CodexActiveState,
  type CodexEntry,
  type CodexEntryType,
  type CodexProgression,
  type Outline,
  type Project,
} from '../api'

type Props = {
  project: Project
  onDirtyChange?: (dirty: boolean) => void
}

type MetadataRow = { key: string; value: string }
type ProgressionRow = {
  id?: string
  sceneID: string
  timing: 'before' | 'after'
  description: string
  metadata: MetadataRow[]
}

type EntryDraft = {
  id?: string
  type: CodexEntryType
  name: string
  aliases: string[]
  tags: string[]
  description: string
  metadata: MetadataRow[]
  revision?: string
}

const emptyDraft = (entryType: CodexEntryType = 'character'): EntryDraft => ({
  type: entryType,
  name: '',
  aliases: [],
  tags: [],
  description: '',
  metadata: [],
})

function normalizeDraft(draft: EntryDraft) {
  return JSON.stringify(draft)
}

function normalizeProgressionRows(rows: ProgressionRow[]) {
  return JSON.stringify(rows)
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

function entryToDraft(entry: CodexEntry): EntryDraft {
  return {
    id: entry.id,
    type: entry.type,
    name: entry.name,
    aliases: entry.aliases,
    tags: entry.tags,
    description: entry.description,
    metadata: Object.entries(entry.metadata).map(([key, value]) => ({ key, value })),
    revision: entry.revision,
  }
}

function draftToRequest(draft: EntryDraft) {
  return {
    type: draft.type,
    name: draft.name,
    aliases: draft.aliases,
    tags: draft.tags,
    description: draft.description,
    metadata: Object.fromEntries(draft.metadata.filter((row) => row.key.trim() !== '').map((row) => [row.key, row.value])),
    expected_revision: draft.revision,
  }
}

function progressionDocumentToRows(progressions: CodexProgression[]): ProgressionRow[] {
  return progressions.map((progression) => ({
    id: progression.id,
    sceneID: progression.anchor.id,
    timing: progression.anchor.timing,
    description: progression.changes.description ?? '',
    metadata: Object.entries(progression.changes.metadata ?? {}).map(([key, value]) => ({ key, value })),
  }))
}

function progressionRowsToRequest(rows: ProgressionRow[]): CodexProgression[] {
  return rows.map((progression) => {
    const metadata = Object.fromEntries(progression.metadata.filter((row) => row.key.trim() !== '').map((row) => [row.key, row.value]))
    return {
      id: progression.id,
      anchor: {
        type: 'scene',
        id: progression.sceneID,
        timing: progression.timing,
      },
      changes: {
        ...(progression.description ? { description: progression.description } : {}),
        ...(Object.keys(metadata).length > 0 ? { metadata } : {}),
      },
    }
  })
}

function compareCodexEntries(left: CodexEntry, right: CodexEntry) {
  const typeOrder = { character: 0, location: 1, lore: 2, custom: 3 }
  if (typeOrder[left.type] !== typeOrder[right.type]) {
    return typeOrder[left.type] - typeOrder[right.type]
  }
  if (left.name < right.name) {
    return -1
  }
  if (left.name > right.name) {
    return 1
  }
  if (left.id < right.id) {
    return -1
  }
  if (left.id > right.id) {
    return 1
  }
  return 0
}

/**
 * CodexWorkbench
 *
 * Renders the active project's Codex editor, progression editor, and
 * active-state inspector with explicit save and dirty-state behavior.
 */
export default function CodexWorkbench({ project, onDirtyChange }: Props) {
  const [entries, setEntries] = useState<CodexEntry[]>([])
  const [outline, setOutline] = useState<Outline | null>(null)
  const [selectedEntryID, setSelectedEntryID] = useState<string | null>(null)
  const [entryDraft, setEntryDraft] = useState<EntryDraft>(emptyDraft())
  const [savedEntrySnapshot, setSavedEntrySnapshot] = useState(normalizeDraft(emptyDraft()))
  const [progressions, setProgressions] = useState<ProgressionRow[]>([])
  const [savedProgressionsSnapshot, setSavedProgressionsSnapshot] = useState(normalizeProgressionRows([]))
  const [progressionRevision, setProgressionRevision] = useState<string | null>(null)
  const [activeSceneID, setActiveSceneID] = useState('')
  const [activeState, setActiveState] = useState<CodexActiveState | null>(null)
  const [loading, setLoading] = useState(true)
  const [entryStatus, setEntryStatus] = useState('Saved')
  const [progressionStatus, setProgressionStatus] = useState('Saved')
  const [error, setError] = useState('')
  const [savingEntry, setSavingEntry] = useState(false)
  const [savingProgressions, setSavingProgressions] = useState(false)

  const scenes = outline?.arcs.flatMap((arc) => arc.chapters.flatMap((chapter) => chapter.scenes)) ?? []
  const entryDirty = normalizeDraft(entryDraft) !== savedEntrySnapshot
  const progressionDirty = normalizeProgressionRows(progressions) !== savedProgressionsSnapshot
  const dirty = entryDirty || progressionDirty
  const visibleActiveState = selectedEntryID && activeSceneID ? activeState : null

  function applyLoadedEntry(entry: CodexEntry, document: { progressions: CodexProgression[]; revision: string | null }) {
    const draft = entryToDraft(entry)
    setEntryDraft(draft)
    setSavedEntrySnapshot(normalizeDraft(draft))
    const rows = progressionDocumentToRows(document.progressions)
    setProgressions(rows)
    setSavedProgressionsSnapshot(normalizeProgressionRows(rows))
    setProgressionRevision(document.revision)
    setEntryStatus('Saved')
    setProgressionStatus('Saved')
  }

  useEffect(() => {
    onDirtyChange?.(dirty)
  }, [dirty, onDirtyChange])

  useEffect(() => {
    let cancelled = false
    async function load() {
      setLoading(true)
      setError('')
      try {
        const [entryResponse, nextOutline] = await Promise.all([getCodexEntries(), getOutline()])
        if (cancelled) {
          return
        }
        setEntries([...entryResponse.entries].sort(compareCodexEntries))
        setOutline(nextOutline)
        const firstSceneID = nextOutline.arcs.flatMap((arc) => arc.chapters.flatMap((chapter) => chapter.scenes))[0]?.id ?? ''
        setActiveSceneID(firstSceneID)
      } catch (requestError) {
        if (!cancelled) {
          setError(requestError instanceof Error ? requestError.message : 'Request failed')
        }
      } finally {
        if (!cancelled) {
          setLoading(false)
        }
      }
    }
    void load()
    return () => {
      cancelled = true
    }
  }, [project.project_id])

  useEffect(() => {
    if (!selectedEntryID) {
      return
    }
    const entryID = selectedEntryID
    let cancelled = false
    async function loadSelectedEntry() {
      try {
        const [entry, document] = await Promise.all([getCodexEntry(entryID), getCodexProgressions(entryID)])
        if (cancelled) {
          return
        }
        applyLoadedEntry(entry, document)
      } catch (requestError) {
        if (!cancelled) {
          setError(requestError instanceof Error ? requestError.message : 'Request failed')
        }
      }
    }
    void loadSelectedEntry()
    return () => {
      cancelled = true
    }
  }, [selectedEntryID])

  useEffect(() => {
    if (!selectedEntryID || !activeSceneID) {
      return
    }
    let cancelled = false
    void getCodexActiveState(selectedEntryID, activeSceneID)
      .then((nextActiveState) => {
        if (cancelled) {
          return
        }
        setActiveState(nextActiveState)
      })
      .catch((requestError) => {
        if (cancelled) {
          return
        }
        setActiveState(null)
        setError(requestError instanceof Error ? requestError.message : 'Request failed')
      })
    return () => {
      cancelled = true
    }
  }, [selectedEntryID, activeSceneID])

  function updateProgressionRows(next: ProgressionRow[]) {
    setProgressions(next)
    setProgressionStatus(normalizeProgressionRows(next) === savedProgressionsSnapshot ? 'Saved' : 'Unsaved changes')
  }

  function selectEntry(entryID: string) {
    if (dirty && selectedEntryID !== entryID && !window.confirm('Discard the current Codex draft?')) {
      return
    }
    setError('')
    setActiveState(null)
    setSelectedEntryID(entryID)
  }

  function beginNewEntry() {
    if (dirty && !window.confirm('Discard the current Codex draft?')) {
      return
    }
    const draft = emptyDraft()
    setSelectedEntryID(null)
    setEntryDraft(draft)
    setSavedEntrySnapshot(normalizeDraft(draft))
    setProgressions([])
    setSavedProgressionsSnapshot(normalizeProgressionRows([]))
    setProgressionRevision(null)
    setEntryStatus('Unsaved changes')
    setProgressionStatus('Saved')
    setActiveState(null)
  }

  function updateDraft(nextDraft: EntryDraft) {
    setEntryDraft(nextDraft)
    setEntryStatus(normalizeDraft(nextDraft) === savedEntrySnapshot ? 'Saved' : 'Unsaved changes')
  }

  async function saveEntry() {
    setSavingEntry(true)
    setError('')
    setEntryStatus('Saving')
    try {
      const saved = entryDraft.id
        ? await updateCodexEntry(entryDraft.id, draftToRequest(entryDraft))
        : await createCodexEntry(draftToRequest(entryDraft))
      const draft = entryToDraft(saved)
      setEntryDraft(draft)
      setSavedEntrySnapshot(normalizeDraft(draft))
      setEntryStatus('Saved')
      setSelectedEntryID(saved.id)
      setEntries((current) => {
        const others = current.filter((entry) => entry.id !== saved.id)
        return [...others, saved].sort(compareCodexEntries)
      })
    } catch (requestError) {
      const message = requestError instanceof Error ? requestError.message : 'Request failed'
      setError(message)
      setEntryStatus(requestError instanceof APIError && requestError.status === 409 ? 'Conflict' : 'Error')
    } finally {
      setSavingEntry(false)
    }
  }

  async function saveProgressions() {
    if (!selectedEntryID) {
      return
    }
    setSavingProgressions(true)
    setError('')
    setProgressionStatus('Saving')
    try {
      const saved = await saveCodexProgressions(selectedEntryID, {
        progressions: progressionRowsToRequest(progressions),
        expected_revision: progressionRevision,
      })
      const rows = progressionDocumentToRows(saved.progressions)
      setProgressions(rows)
      setSavedProgressionsSnapshot(normalizeProgressionRows(rows))
      setProgressionRevision(saved.revision)
      setProgressionStatus('Saved')
    } catch (requestError) {
      const message = requestError instanceof Error ? requestError.message : 'Request failed'
      setError(message)
      setProgressionStatus(requestError instanceof APIError && requestError.status === 409 ? 'Conflict' : 'Error')
    } finally {
      setSavingProgressions(false)
    }
  }

  function updateAlias(index: number, value: string) {
    const aliases = [...entryDraft.aliases]
    aliases[index] = value
    updateDraft({ ...entryDraft, aliases })
  }

  function updateTag(index: number, value: string) {
    const tags = [...entryDraft.tags]
    tags[index] = value
    updateDraft({ ...entryDraft, tags })
  }

  function updateMetadata(index: number, key: 'key' | 'value', value: string) {
    const metadata = [...entryDraft.metadata]
    metadata[index] = { ...metadata[index], [key]: value }
    updateDraft({ ...entryDraft, metadata })
  }

  function updateProgression(index: number, field: keyof ProgressionRow, value: string) {
    const next = [...progressions]
    next[index] = { ...next[index], [field]: value }
    updateProgressionRows(next)
  }

  function updateProgressionMetadata(index: number, metadataIndex: number, field: keyof MetadataRow, value: string) {
    const next = [...progressions]
    const metadata = [...next[index].metadata]
    metadata[metadataIndex] = { ...metadata[metadataIndex], [field]: value }
    next[index] = { ...next[index], metadata }
    updateProgressionRows(next)
  }

  function moveAlias(index: number, offset: number) {
    updateDraft({ ...entryDraft, aliases: moveItem(entryDraft.aliases, index, index + offset) })
  }

  function removeAlias(index: number) {
    updateDraft({ ...entryDraft, aliases: entryDraft.aliases.filter((_, currentIndex) => currentIndex !== index) })
  }

  function moveTag(index: number, offset: number) {
    updateDraft({ ...entryDraft, tags: moveItem(entryDraft.tags, index, index + offset) })
  }

  function removeTag(index: number) {
    updateDraft({ ...entryDraft, tags: entryDraft.tags.filter((_, currentIndex) => currentIndex !== index) })
  }

  function addProgression() {
    updateProgressionRows([...progressions, { sceneID: scenes[0]?.id ?? '', timing: 'after', description: '', metadata: [] }])
  }

  function moveProgression(index: number, offset: number) {
    updateProgressionRows(moveItem(progressions, index, index + offset))
  }

  function removeProgression(index: number) {
    updateProgressionRows(progressions.filter((_, currentIndex) => currentIndex !== index))
  }

  function addProgressionMetadata(index: number) {
    const next = [...progressions]
    next[index] = { ...next[index], metadata: [...next[index].metadata, { key: '', value: '' }] }
    updateProgressionRows(next)
  }

  if (loading) {
    return <section className="workbench"><p>Loading Codex…</p></section>
  }

  const groupedEntries = entries.reduce<Record<CodexEntryType, CodexEntry[]>>((groups, entry) => {
    groups[entry.type].push(entry)
    return groups
  }, { character: [], location: [], lore: [], custom: [] })

  return (
    <section className="workbench codex-workbench">
      <aside>
        <div className="section-heading">
          <h2>Codex</h2>
          <button type="button" onClick={beginNewEntry}>New entry</button>
        </div>
        {entries.length === 0 ? (
          <p>No Codex entries yet.</p>
        ) : (
          (Object.entries(groupedEntries) as [CodexEntryType, CodexEntry[]][]).map(([entryType, grouped]) => (
            grouped.length === 0 ? null : (
              <section key={entryType}>
                <h3>{entryType}</h3>
                <ul>
                  {grouped.map((entry) => (
                    <li key={entry.id}>
                      <button type="button" onClick={() => void selectEntry(entry.id)}>{entry.name}</button>
                    </li>
                  ))}
                </ul>
              </section>
            )
          ))
        )}
      </aside>

      <div className="codex-editor">
        <header className="section-heading">
          <div>
            <p className="folio">Milestone 3 / Codex</p>
            <h3>{selectedEntryID ? entryDraft.name || 'Untitled entry' : 'New entry'}</h3>
          </div>
          <div>
            <span>{entryStatus}</span>
            <button type="button" onClick={() => void saveEntry()} disabled={savingEntry || !entryDirty}>
              {savingEntry ? 'Saving…' : 'Save entry'}
            </button>
          </div>
        </header>

        <label>
          Type
          <select value={entryDraft.type} onChange={(event) => updateDraft({ ...entryDraft, type: event.target.value as CodexEntryType })} disabled={Boolean(entryDraft.id)}>
            <option value="character">Character</option>
            <option value="location">Location</option>
            <option value="lore">Lore</option>
            <option value="custom">Custom</option>
          </select>
        </label>
        <label>
          Name
          <input value={entryDraft.name} onChange={(event) => updateDraft({ ...entryDraft, name: event.target.value })} />
        </label>
        <label>
          Description
          <textarea value={entryDraft.description} onChange={(event) => updateDraft({ ...entryDraft, description: event.target.value })} />
        </label>

        <div>
          <div className="section-heading">
            <h4>Aliases</h4>
            <button type="button" onClick={() => updateDraft({ ...entryDraft, aliases: [...entryDraft.aliases, ''] })}>Add alias</button>
          </div>
          {entryDraft.aliases.map((alias, index) => (
            <div key={`alias-${index}`} className="metadata-row">
              <label>
                Alias {index + 1}
                <input value={alias} onChange={(event) => updateAlias(index, event.target.value)} />
              </label>
              <div>
                <button type="button" onClick={() => moveAlias(index, -1)} disabled={index === 0}>Move alias {index + 1} up</button>
                <button type="button" onClick={() => moveAlias(index, 1)} disabled={index === entryDraft.aliases.length - 1}>Move alias {index + 1} down</button>
                <button type="button" onClick={() => removeAlias(index)}>Remove alias {index + 1}</button>
              </div>
            </div>
          ))}
        </div>

        <div>
          <div className="section-heading">
            <h4>Tags</h4>
            <button type="button" onClick={() => updateDraft({ ...entryDraft, tags: [...entryDraft.tags, ''] })}>Add tag</button>
          </div>
          {entryDraft.tags.map((tag, index) => (
            <div key={`tag-${index}`} className="metadata-row">
              <label>
                Tag {index + 1}
                <input value={tag} onChange={(event) => updateTag(index, event.target.value)} />
              </label>
              <div>
                <button type="button" onClick={() => moveTag(index, -1)} disabled={index === 0}>Move tag {index + 1} up</button>
                <button type="button" onClick={() => moveTag(index, 1)} disabled={index === entryDraft.tags.length - 1}>Move tag {index + 1} down</button>
                <button type="button" onClick={() => removeTag(index)}>Remove tag {index + 1}</button>
              </div>
            </div>
          ))}
        </div>

        <div>
          <div className="section-heading">
            <h4>Metadata</h4>
            <button type="button" onClick={() => updateDraft({ ...entryDraft, metadata: [...entryDraft.metadata, { key: '', value: '' }] })}>Add metadata</button>
          </div>
          {entryDraft.metadata.map((row, index) => (
            <div key={`metadata-${index}`} className="metadata-row">
              <label>
                Metadata key {index + 1}
                <input value={row.key} onChange={(event) => updateMetadata(index, 'key', event.target.value)} />
              </label>
              <label>
                Metadata value {index + 1}
                <input value={row.value} onChange={(event) => updateMetadata(index, 'value', event.target.value)} />
              </label>
            </div>
          ))}
        </div>

        {selectedEntryID && (
          <>
            <section>
              <header className="section-heading">
                <h4>Progressions</h4>
                <div>
                  <span>{progressionStatus}</span>
                  <button
                    type="button"
                    onClick={addProgression}
                  >
                    Add progression
                  </button>
                  <button type="button" onClick={() => void saveProgressions()} disabled={savingProgressions || !progressionDirty}>
                    {savingProgressions ? 'Saving…' : 'Save progressions'}
                  </button>
                </div>
              </header>
              {progressions.map((progression, index) => (
                <div key={progression.id ?? `new-${index}`} className="progression-row">
                  <div className="section-heading">
                    <h5>Progression {index + 1}</h5>
                    <div>
                      <button type="button" onClick={() => moveProgression(index, -1)} disabled={index === 0}>Move progression {index + 1} up</button>
                      <button type="button" onClick={() => moveProgression(index, 1)} disabled={index === progressions.length - 1}>Move progression {index + 1} down</button>
                      <button type="button" onClick={() => removeProgression(index)}>Remove progression {index + 1}</button>
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
                    <select value={progression.timing} onChange={(event) => updateProgression(index, 'timing', event.target.value)}>
                      <option value="before">Before</option>
                      <option value="after">After</option>
                    </select>
                  </label>
                  <label>
                    Progression description
                    <textarea value={progression.description} onChange={(event) => updateProgression(index, 'description', event.target.value)} />
                  </label>
                  <div>
                    <div className="section-heading">
                      <h5>Progression metadata</h5>
                      <button
                        type="button"
                        onClick={() => addProgressionMetadata(index)}
                      >
                        Add progression metadata
                      </button>
                    </div>
                    {progression.metadata.map((row, metadataIndex) => (
                      <div key={`progression-${index}-metadata-${metadataIndex}`} className="metadata-row">
                        <label>
                          Progression metadata key {metadataIndex + 1}
                          <input value={row.key} onChange={(event) => updateProgressionMetadata(index, metadataIndex, 'key', event.target.value)} />
                        </label>
                        <label>
                          Progression metadata value {metadataIndex + 1}
                          <input value={row.value} onChange={(event) => updateProgressionMetadata(index, metadataIndex, 'value', event.target.value)} />
                        </label>
                      </div>
                    ))}
                  </div>
                </div>
              ))}
            </section>

            <section>
              <header className="section-heading">
                <h4>Active state</h4>
              </header>
              <label>
                Scene selector
                <select value={activeSceneID} onChange={(event: ChangeEvent<HTMLSelectElement>) => setActiveSceneID(event.target.value)}>
                  {scenes.map((scene) => (
                    <option key={scene.id} value={scene.id}>{scene.title}</option>
                  ))}
                </select>
              </label>
              {visibleActiveState && (
                <div>
                  <p>{visibleActiveState.entry.description}</p>
                  <dl>
                    {Object.entries(visibleActiveState.entry.metadata).map(([key, value]) => (
                      <div key={key}>
                        <dt>{key}</dt>
                        <dd>{value}</dd>
                      </div>
                    ))}
                  </dl>
                  <p>Applied progressions: {visibleActiveState.applied_progression_ids.join(', ') || 'None'}</p>
                </div>
              )}
            </section>
          </>
        )}

        {error && <p role="alert">{error}</p>}
      </div>
    </section>
  )
}
