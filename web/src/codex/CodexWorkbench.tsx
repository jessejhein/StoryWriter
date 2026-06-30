/**
 * CodexWorkbench.tsx
 *
 * Provides the Milestone 3 Codex workspace for managing canonical entries,
 * editing ordered progressions, and inspecting resolved active state at a
 * selected scene.
 */

import { useEffect, useRef, useState } from 'react'
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
  type Outline,
  type Project,
} from '../api'
import ConfirmDialog from '../components/ConfirmDialog'
import ActiveStateInspector from './ActiveStateInspector'
import CodexEntryForm from './CodexEntryForm'
import CodexEntryList from './CodexEntryList'
import ProgressionList from './ProgressionList'
import {
  cloneDraft,
  cloneProgressionRows,
  compareCodexEntries,
  draftToCreateRequest,
  draftToUpdateRequest,
  draftsEqual,
  emptyDraft,
  entryToDraft,
  progressionDocumentToRows,
  progressionRowsEqual,
  progressionRowsToRequest,
} from './workbenchState'
import type { EntryDraft, ProgressionRow } from './workbenchTypes'

type Props = {
  project: Project
  onDirtyChange?: (dirty: boolean) => void
}

type PendingDiscardAction =
  | { kind: 'select'; entryID: string }
  | { kind: 'new-entry' }
  | null

type LoadedProgressionDocument = Awaited<ReturnType<typeof getCodexProgressions>>

export default function CodexWorkbench({ project, onDirtyChange }: Props) {
  const [entries, setEntries] = useState<CodexEntry[]>([])
  const [outline, setOutline] = useState<Outline | null>(null)
  const [selectedEntryID, setSelectedEntryID] = useState<string | null>(null)
  const [entryDraft, setEntryDraft] = useState<EntryDraft>(emptyDraft())
  const [savedEntryDraft, setSavedEntryDraft] = useState<EntryDraft>(emptyDraft())
  const [progressions, setProgressions] = useState<ProgressionRow[]>([])
  const [savedProgressions, setSavedProgressions] = useState<ProgressionRow[]>([])
  const [progressionRevision, setProgressionRevision] = useState<string | null>(null)
  const [activeSceneID, setActiveSceneID] = useState('')
  const [activeState, setActiveState] = useState<CodexActiveState | null>(null)
  const [activeStateRefresh, setActiveStateRefresh] = useState(0)
  const [loading, setLoading] = useState(true)
  const [entryStatus, setEntryStatus] = useState('Saved')
  const [progressionStatus, setProgressionStatus] = useState('Saved')
  const [error, setError] = useState('')
  const [savingEntry, setSavingEntry] = useState(false)
  const [savingProgressions, setSavingProgressions] = useState(false)
  const [pendingDiscardAction, setPendingDiscardAction] = useState<PendingDiscardAction>(null)
  const selectionVersion = useRef(0)
  const entryEditVersion = useRef(0)
  const progressionEditVersion = useRef(0)
  const skipSelectedEntryLoad = useRef<string | null>(null)

  const scenes = outline?.arcs.flatMap((arc) => arc.chapters.flatMap((chapter) => chapter.scenes)) ?? []
  const entryDirty = !draftsEqual(entryDraft, savedEntryDraft)
  const progressionDirty = !progressionRowsEqual(progressions, savedProgressions)
  const dirty = entryDirty || progressionDirty
  const visibleActiveState = selectedEntryID && activeSceneID ? activeState : null

  function applyLoadedEntry(entry: CodexEntry, document: Pick<LoadedProgressionDocument, 'progressions' | 'revision'>) {
    const draft = entryToDraft(entry)
    const rows = progressionDocumentToRows(document.progressions)
    setEntryDraft(draft)
    setSavedEntryDraft(cloneDraft(draft))
    setProgressions(rows)
    setSavedProgressions(cloneProgressionRows(rows))
    setProgressionRevision(document.revision)
    setEntryStatus('Saved')
    setProgressionStatus('Saved')
  }

  function updateDraft(nextDraft: EntryDraft) {
    entryEditVersion.current += 1
    setEntryDraft(nextDraft)
    setEntryStatus(draftsEqual(nextDraft, savedEntryDraft) ? 'Saved' : 'Unsaved changes')
  }

  function updateProgressionRows(nextRows: ProgressionRow[]) {
    progressionEditVersion.current += 1
    setProgressions(nextRows)
    setProgressionStatus(progressionRowsEqual(nextRows, savedProgressions) ? 'Saved' : 'Unsaved changes')
  }

  function resetToNewEntry() {
    const draft = emptyDraft()
    selectionVersion.current += 1
    setSelectedEntryID(null)
    setEntryDraft(draft)
    setSavedEntryDraft(cloneDraft(draft))
    setProgressions([])
    setSavedProgressions([])
    setProgressionRevision(null)
    setEntryStatus('Saved')
    setProgressionStatus('Saved')
    setActiveState(null)
    setError('')
  }

  function selectEntry(entryID: string) {
    setError('')
    selectionVersion.current += 1
    setActiveState(null)
    setSelectedEntryID(entryID)
  }

  function requestSelectEntry(entryID: string) {
    if (dirty && selectedEntryID !== entryID) {
      setPendingDiscardAction({ kind: 'select', entryID })
      return
    }
    selectEntry(entryID)
  }

  function requestNewEntry() {
    if (dirty) {
      setPendingDiscardAction({ kind: 'new-entry' })
      return
    }
    resetToNewEntry()
  }

  function confirmDiscardAction() {
    const action = pendingDiscardAction
    setPendingDiscardAction(null)
    if (!action) {
      return
    }
    if (action.kind === 'select') {
      selectEntry(action.entryID)
      return
    }
    resetToNewEntry()
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
        setActiveSceneID(nextOutline.arcs.flatMap((arc) => arc.chapters.flatMap((chapter) => chapter.scenes))[0]?.id ?? '')
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
    if (skipSelectedEntryLoad.current === entryID) {
      skipSelectedEntryLoad.current = null
      return
    }
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
        if (!cancelled) {
          setActiveState(nextActiveState)
        }
      })
      .catch((requestError) => {
        if (!cancelled) {
          setActiveState(null)
          setError(requestError instanceof Error ? requestError.message : 'Request failed')
        }
      })
    return () => {
      cancelled = true
    }
  }, [selectedEntryID, activeSceneID, activeStateRefresh])

  async function saveEntry() {
    const selectionAtStart = selectionVersion.current
    const editAtStart = entryEditVersion.current
    const entryIDAtStart = entryDraft.id
    setSavingEntry(true)
    setError('')
    setEntryStatus('Saving')
    try {
      const saved = entryIDAtStart
        ? await updateCodexEntry(entryIDAtStart, draftToUpdateRequest(entryDraft))
        : await createCodexEntry(draftToCreateRequest(entryDraft))
      setEntries((current) => {
        const others = current.filter((entry) => entry.id !== saved.id)
        return [...others, saved].sort(compareCodexEntries)
      })
      if (selectionVersion.current !== selectionAtStart) {
        return
      }
      const draft = entryToDraft(saved)
      if (entryEditVersion.current !== editAtStart) {
        setEntryDraft((current) => ({ ...current, id: saved.id, type: saved.type, revision: saved.revision }))
        setSavedEntryDraft(cloneDraft(draft))
        setEntryStatus('Unsaved changes')
        if (!entryIDAtStart) {
          skipSelectedEntryLoad.current = saved.id
          setSelectedEntryID(saved.id)
        }
        return
      }
      setEntryDraft(draft)
      setSavedEntryDraft(cloneDraft(draft))
      setEntryStatus('Saved')
      if (!entryIDAtStart) {
        skipSelectedEntryLoad.current = saved.id
      }
      setSelectedEntryID(saved.id)
      setActiveStateRefresh((current) => current + 1)
    } catch (requestError) {
      if (selectionVersion.current !== selectionAtStart) {
        return
      }
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
    const entryIDAtStart = selectedEntryID
    const selectionAtStart = selectionVersion.current
    const editAtStart = progressionEditVersion.current
    setSavingProgressions(true)
    setError('')
    setProgressionStatus('Saving')
    try {
      const saved = await saveCodexProgressions(entryIDAtStart, {
        progressions: progressionRowsToRequest(progressions),
        expected_revision: progressionRevision,
      })
      if (selectionVersion.current !== selectionAtStart) {
        return
      }
      const rows = progressionDocumentToRows(saved.progressions)
      if (progressionEditVersion.current !== editAtStart) {
        setSavedProgressions(cloneProgressionRows(rows))
        setProgressionRevision(saved.revision)
        setProgressionStatus('Unsaved changes')
        return
      }
      setProgressions(rows)
      setSavedProgressions(cloneProgressionRows(rows))
      setProgressionRevision(saved.revision)
      setProgressionStatus('Saved')
      setActiveStateRefresh((current) => current + 1)
    } catch (requestError) {
      if (selectionVersion.current !== selectionAtStart) {
        return
      }
      const message = requestError instanceof Error ? requestError.message : 'Request failed'
      setError(message)
      setProgressionStatus(requestError instanceof APIError && requestError.status === 409 ? 'Conflict' : 'Error')
    } finally {
      setSavingProgressions(false)
    }
  }

  if (loading) {
    return <section className="workbench"><p>Loading Codex…</p></section>
  }

  return (
    <>
      <section className="workbench codex-workbench">
        <CodexEntryList entries={entries} onNewEntry={requestNewEntry} onSelectEntry={requestSelectEntry} />

        <div className="codex-editor">
          <CodexEntryForm
            selectedEntryID={selectedEntryID}
            draft={entryDraft}
            status={entryStatus}
            saving={savingEntry}
            dirty={entryDirty}
            onChangeDraft={updateDraft}
            onSave={() => void saveEntry()}
          />

          {selectedEntryID && (
            <>
              <ProgressionList
                progressions={progressions}
                scenes={scenes}
                status={progressionStatus}
                saving={savingProgressions}
                dirty={progressionDirty}
                onChange={updateProgressionRows}
                onSave={() => void saveProgressions()}
              />
              <ActiveStateInspector
                scenes={scenes}
                activeSceneID={activeSceneID}
                activeState={visibleActiveState}
                onChangeSceneID={setActiveSceneID}
              />
            </>
          )}

          {error && <p role="alert">{error}</p>}
        </div>
      </section>

      <ConfirmDialog
        open={pendingDiscardAction !== null}
        title="Discard Codex draft?"
        message="You have unsaved Codex changes. Discard them and continue?"
        confirmLabel="Discard draft"
        onConfirm={confirmDiscardAction}
        onCancel={() => setPendingDiscardAction(null)}
      />
    </>
  )
}
