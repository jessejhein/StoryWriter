import { FormEvent, useEffect, useMemo, useState } from 'react'
import {
  APIError,
  acceptImportCandidate,
  createImport,
  discardImportCandidate,
  extractImport,
  getImportCandidates,
  getImportChunks,
  getImports,
  getProviderProfiles,
  mergeImportCandidate,
  type ImportCandidate,
  type ImportCandidateProposal,
  type ImportChunk,
  type ImportFile,
  type ImportSummary,
  type CodexEntryType,
  updateImportCandidate,
} from '../api'
import ConfirmDialog from '../components/ConfirmDialog'

type Props = {
  onDirtyChange?: (dirty: boolean) => void
}

export default function ImportReviewWorkbench({ onDirtyChange }: Props) {
  const [sourceDirectory, setSourceDirectory] = useState('')
  const [imports, setImports] = useState<ImportSummary[]>([])
  const [selectedImportID, setSelectedImportID] = useState('')
  const [chunks, setChunks] = useState<ImportChunk[]>([])
  const [filesByImport, setFilesByImport] = useState<Record<string, ImportFile[]>>({})
  const [selectedChunkIDs, setSelectedChunkIDs] = useState<string[]>([])
  const [candidates, setCandidates] = useState<ImportCandidate[]>([])
  const [selectedCandidateID, setSelectedCandidateID] = useState('')
  const [draft, setDraft] = useState<ImportCandidateProposal | null>(null)
  const [dirty, setDirty] = useState(false)
  const [pendingCandidateID, setPendingCandidateID] = useState<string | null>(null)
  const [profiles, setProfiles] = useState<Array<{ id: string; name: string }>>([])
  const [profileID, setProfileID] = useState('')
  const [model, setModel] = useState('')
  const [status, setStatus] = useState('Loading imports...')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState<'import' | 'extract' | 'save' | 'merge' | 'discard' | 'accept' | null>(null)
  const [mergeTargetID, setMergeTargetID] = useState('')
  const [pendingDecision, setPendingDecision] = useState<'accept' | 'discard' | null>(null)
  const [conflict, setConflict] = useState(false)
  const [kindFilter, setKindFilter] = useState('all')
  const [statusFilter, setStatusFilter] = useState('all')

  useEffect(() => {
    onDirtyChange?.(dirty)
  }, [dirty, onDirtyChange])

  useEffect(() => {
    function warnBeforeUnload(event: BeforeUnloadEvent) {
      if (!dirty) return
      event.preventDefault()
    }
    window.addEventListener('beforeunload', warnBeforeUnload)
    return () => window.removeEventListener('beforeunload', warnBeforeUnload)
  }, [dirty])

  useEffect(() => {
    void Promise.all([getImports(), getImportCandidates(), getProviderProfiles()])
      .then(([importResponse, candidateResponse, profileResponse]) => {
        setImports(importResponse.imports)
        if (importResponse.imports.length > 0) {
          setSelectedImportID(importResponse.imports[0].id)
        }
        setCandidates(candidateResponse.candidates)
        if (candidateResponse.candidates.length > 0) {
          applyCandidateSelection(candidateResponse.candidates[0].id, candidateResponse.candidates)
        }
        const ready = profileResponse.profiles.filter((profile) => profile.readiness === 'ready' && profile.capabilities.chat)
        setProfiles(ready.map((profile) => ({ id: profile.id, name: profile.name })))
        if (ready.length > 0) setProfileID(ready[0].id)
        setStatus(importResponse.imports.length === 0 ? 'No imports yet.' : 'Imports loaded.')
      })
      .catch((requestError) => {
        setError(requestError instanceof Error ? requestError.message : 'Failed to load import review state.')
      })
  }, [])

  useEffect(() => {
    if (!selectedImportID) {
      return
    }
    let current = true
    void getImportChunks(selectedImportID)
      .then((response) => {
        if (!current) return
        setChunks(response.chunks)
        setSelectedChunkIDs([])
      })
      .catch((requestError) => {
        if (!current) return
        setError(requestError instanceof Error ? requestError.message : 'Failed to load chunks.')
      })
    return () => { current = false }
  }, [selectedImportID])

  const selectedCandidate = useMemo(
    () => candidates.find((candidate) => candidate.id === selectedCandidateID) ?? null,
    [candidates, selectedCandidateID],
  )

  const mergeOptions = useMemo(() => {
    if (!selectedCandidate || selectedCandidate.kind !== 'codex') return []
    return candidates.filter((candidate) => candidate.id !== selectedCandidate.id && candidate.kind === selectedCandidate.kind && candidate.status === 'pending')
  }, [candidates, selectedCandidate])

  const visibleCandidates = useMemo(() => candidates.filter((candidate) =>
    (kindFilter === 'all' || candidate.kind === kindFilter) &&
    (statusFilter === 'all' || candidate.status === statusFilter),
  ), [candidates, kindFilter, statusFilter])

  async function refreshCandidates(nextSelectedCandidateID?: string) {
    const response = await getImportCandidates()
    setCandidates(response.candidates)
    applyCandidateSelection(nextSelectedCandidateID ?? response.candidates[0]?.id ?? '', response.candidates)
  }

  function applyCandidateSelection(candidateID: string, sourceCandidates: ImportCandidate[]) {
    setSelectedCandidateID(candidateID)
    const nextCandidate = sourceCandidates.find((candidate) => candidate.id === candidateID) ?? null
    setDraft(nextCandidate ? cloneProposal(nextCandidate.proposal) : null)
    setDirty(false)
    setConflict(false)
  }

  async function handleImport(event: FormEvent) {
    event.preventDefault()
    setBusy('import')
    setError('')
    try {
      const response = await createImport(sourceDirectory)
      const importsResponse = await getImports()
      setImports(importsResponse.imports)
      setFilesByImport((current) => ({ ...current, [response.import.id]: response.files }))
      setSelectedImportID(response.import.id)
      setChunks([])
      setSelectedChunkIDs([])
      setSourceDirectory('')
      setStatus(`Imported ${response.import.file_count} files.`)
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : 'Import failed.')
    } finally {
      setBusy(null)
    }
  }

  async function handleExtract() {
    if (!selectedImportID || !profileID || !model || selectedChunkIDs.length === 0) return
    setBusy('extract')
    setError('')
    try {
      const response = await extractImport(selectedImportID, {
        chunk_ids: selectedChunkIDs,
        mode: 'structure',
        profile_id: profileID,
        model,
      })
      setCandidates(response.candidates)
      applyCandidateSelection(response.candidates[0]?.id ?? '', response.candidates)
      setStatus(`Extracted ${response.candidates.length} candidates with ${response.provider.model}.`)
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : 'Extraction failed.')
    } finally {
      setBusy(null)
    }
  }

  async function handleSave() {
    if (!selectedCandidate || !draft) return
    setBusy('save')
    setError('')
    try {
      const updated = await updateImportCandidate(selectedCandidate.id, draft, selectedCandidate.revision)
      await refreshCandidates(updated.id)
      setStatus(`Saved candidate ${updated.id}.`)
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : 'Save failed.')
	  if (requestError instanceof APIError && requestError.status === 409) {
		setConflict(true)
	  } else {
        setDirty(false)
      }
    } finally {
      setBusy(null)
    }
  }

  async function handleMerge() {
    if (!selectedCandidate || !draft || !mergeTargetID) return
    const other = candidates.find((candidate) => candidate.id === mergeTargetID)
    if (!other) return
    setBusy('merge')
    setError('')
    try {
      const response = await mergeImportCandidate(selectedCandidate.id, {
        other_candidate_id: other.id,
        expected_revision: selectedCandidate.revision,
        other_expected_revision: other.revision,
        proposal: draft,
      })
      await refreshCandidates(response.candidate.id)
      setMergeTargetID('')
      setStatus(`Merged ${response.merged_candidate_ids.length} candidates.`)
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : 'Merge failed.')
    } finally {
      setBusy(null)
    }
  }

  async function handleDiscard() {
    if (!selectedCandidate) return
    setBusy('discard')
    setError('')
    try {
      const discarded = await discardImportCandidate(selectedCandidate.id, selectedCandidate.revision)
      await refreshCandidates(discarded.id)
      setStatus(`Discarded ${discarded.id}.`)
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : 'Discard failed.')
    } finally {
      setBusy(null)
    }
  }

  async function handleAccept() {
    if (!selectedCandidate) return
    setBusy('accept')
    setError('')
    try {
      const accepted = await acceptImportCandidate(selectedCandidate.id, selectedCandidate.revision)
      await refreshCandidates(accepted.candidate.id)
      setStatus(`Accepted ${accepted.candidate.id}.`)
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : 'Accept failed.')
    } finally {
      setBusy(null)
    }
  }

  function confirmDecision() {
    const decision = pendingDecision
    setPendingDecision(null)
    if (decision === 'accept') void handleAccept()
    if (decision === 'discard') void handleDiscard()
  }

  async function reloadConflictedCandidate() {
    await refreshCandidates(selectedCandidateID)
    setConflict(false)
    setError('')
    setStatus('Reloaded the current server version.')
  }

  function chooseCandidate(candidateID: string) {
    if (dirty && candidateID !== selectedCandidateID) {
      setPendingCandidateID(candidateID)
      return
    }
    applyCandidateSelection(candidateID, candidates)
  }

  function confirmCandidateChange() {
    if (!pendingCandidateID) return
    applyCandidateSelection(pendingCandidateID, candidates)
    setPendingCandidateID(null)
  }

  function updateDraft(transform: (current: ImportCandidateProposal) => ImportCandidateProposal) {
    if (!draft || !selectedCandidate || selectedCandidate.status !== 'pending') return
    const next = transform(draft)
    setDraft(next)
    setDirty(JSON.stringify(next) !== JSON.stringify(selectedCandidate.proposal))
  }

  return (
    <section className="workbench">
      <div className="intro">
        <p className="folio">Milestone 6 / Import Review</p>
        <h2>Import notes, extract structure, and accept only what belongs in canon.</h2>
        <p>Review durable proposals without storing source paths, credentials, or provider output in the project.</p>
      </div>

      <form onSubmit={(event) => void handleImport(event)}>
        <label>
          Source directory
          <input aria-label="Source directory" value={sourceDirectory} onChange={(event) => setSourceDirectory(event.target.value)} placeholder="/absolute/path/to/notes" />
        </label>
        <div className="actions">
          <button type="submit" disabled={busy !== null || !sourceDirectory.startsWith('/')}>Import</button>
        </div>
      </form>

      <p role="status" aria-live="polite">{status}</p>
      {error && <p className="error" role="alert">{error}</p>}

      <div className="project-nav">
        {imports.map((item) => (
          <button key={item.id} type="button" className={item.id === selectedImportID ? '' : 'secondary'} onClick={() => setSelectedImportID(item.id)}>
            {item.id}
          </button>
        ))}
      </div>

      {selectedImportID && (
        <section>
          {filesByImport[selectedImportID] && (
            <>
              <h3>Imported files</h3>
              <ul>
                {filesByImport[selectedImportID].map((file) => (
                  <li key={file.path}>{file.path} · {file.bytes} bytes</li>
                ))}
              </ul>
            </>
          )}
          <h3>Chunks</h3>
          {chunks.length === 0 ? (
            <p>No chunks available for this import.</p>
          ) : (
            <ul>
              {chunks.map((chunk) => (
                <li key={chunk.id}>
                  <label>
                    <input
                      type="checkbox"
                      checked={selectedChunkIDs.includes(chunk.id)}
                      onChange={(event) => {
                        setSelectedChunkIDs((current) => event.target.checked ? [...current, chunk.id] : current.filter((id) => id !== chunk.id))
                      }}
                    />
                    {chunk.source_path} lines {chunk.start_line}-{chunk.end_line}
                  </label>
                  <details>
                    <summary>Inspect chunk</summary>
                    <pre>{chunk.text}</pre>
                  </details>
                </li>
              ))}
            </ul>
          )}

          <label>
            Provider profile
            <select aria-label="Provider profile" value={profileID} onChange={(event) => setProfileID(event.target.value)}>
              <option value="">Select a ready profile</option>
              {profiles.map((profile) => (
                <option key={profile.id} value={profile.id}>{profile.name}</option>
              ))}
            </select>
          </label>
          <label>
            Model
            <input aria-label="Model" value={model} onChange={(event) => setModel(event.target.value)} placeholder="qwen2.5:7b" />
          </label>
          <div className="actions">
            <button type="button" onClick={() => void handleExtract()} disabled={busy !== null || selectedChunkIDs.length === 0 || !profileID || !model}>Extract</button>
          </div>
        </section>
      )}

      <section>
        <h3>Candidates</h3>
        <p>{visibleCandidates.length} of {candidates.length} candidates</p>
        <label>
          Candidate kind
          <select aria-label="Candidate kind" value={kindFilter} onChange={(event) => setKindFilter(event.target.value)}>
            <option value="all">All kinds</option>
            <option value="codex">Codex</option>
            <option value="arc">Arc</option>
            <option value="chapter">Chapter</option>
            <option value="scene">Scene</option>
          </select>
        </label>
        <label>
          Candidate status
          <select aria-label="Candidate status" value={statusFilter} onChange={(event) => setStatusFilter(event.target.value)}>
            <option value="all">All statuses</option>
            <option value="pending">Pending</option>
            <option value="merged">Merged</option>
            <option value="discarded">Discarded</option>
            <option value="accepted">Accepted</option>
          </select>
        </label>
        {candidates.length === 0 ? (
          <p>No candidates yet.</p>
        ) : (
          <div className="project-nav">
            {visibleCandidates.map((candidate) => (
              <button key={candidate.id} type="button" className={candidate.id === selectedCandidateID ? '' : 'secondary'} onClick={() => chooseCandidate(candidate.id)}>
                {candidate.kind} · {candidate.status}
              </button>
            ))}
          </div>
        )}
      </section>

      {selectedCandidate && draft && (
        <section>
          <h3>{selectedCandidate.id}</h3>
          <p>Status: {selectedCandidate.status}</p>
          <p>Provenance: {selectedCandidate.provenance.chunk_ids.join(', ')}</p>
          {conflict && (
            <div role="alert">
              The candidate changed on disk. Your draft is preserved.
              <button type="button" className="secondary" onClick={() => void reloadConflictedCandidate()}>Reload server version</button>
            </div>
          )}
          {selectedCandidate.kind === 'codex' && (
            <>
              <label>
                Name
                <input aria-label="Candidate name" value={codexDraftName(draft)} disabled={selectedCandidate.status !== 'pending'} onChange={(event) => updateDraft((current) => ({ ...(current as CodexCandidateDraft), name: event.target.value }))} />
              </label>
              <label>
                Description
                <textarea aria-label="Candidate description" value={codexDraftDescription(draft)} disabled={selectedCandidate.status !== 'pending'} onChange={(event) => updateDraft((current) => ({ ...(current as CodexCandidateDraft), description: event.target.value }))} />
              </label>
            </>
          )}
          {selectedCandidate.kind !== 'codex' && (
            <label>
              Title
              <input aria-label="Candidate title" value={structuredDraftTitle(draft)} disabled={selectedCandidate.status !== 'pending'} onChange={(event) => updateDraft((current) => ({ ...(current as StructuredCandidateDraft), title: event.target.value }))} />
            </label>
          )}
          {selectedCandidate.canonical_refs.length > 0 && (
            <p>Canonical refs: {selectedCandidate.canonical_refs.map((item) => `${item.kind}:${item.id}`).join(', ')}</p>
          )}
          {selectedCandidate.status === 'pending' && (
            <div className="actions">
              <button type="button" onClick={() => void handleSave()} disabled={busy !== null || !dirty || !isProposalValid(draft)}>Save</button>
              {selectedCandidate.kind === 'codex' && mergeOptions.length > 0 && (
                <>
                  <select aria-label="Merge target" value={mergeTargetID} onChange={(event) => setMergeTargetID(event.target.value)}>
                    <option value="">Select merge target</option>
                    {mergeOptions.map((candidate) => (
                      <option key={candidate.id} value={candidate.id}>{candidate.id}</option>
                    ))}
                  </select>
                  <button type="button" onClick={() => void handleMerge()} disabled={busy !== null || !mergeTargetID}>Merge</button>
                </>
              )}
              <button type="button" className="secondary" onClick={() => setPendingDecision('discard')} disabled={busy !== null}>Discard</button>
              <button type="button" onClick={() => setPendingDecision('accept')} disabled={busy !== null || dirty}>Accept</button>
            </div>
          )}
        </section>
      )}

      <ConfirmDialog
        open={pendingCandidateID !== null}
        title="Discard current draft?"
        message="You have unsaved candidate edits. Discard them and switch candidates?"
        confirmLabel="Discard draft"
        onConfirm={confirmCandidateChange}
        onCancel={() => setPendingCandidateID(null)}
      />
      <ConfirmDialog
        open={pendingDecision !== null}
        title={pendingDecision === 'accept' ? 'Accept candidate into canon?' : 'Discard candidate?'}
        message={pendingDecision === 'accept' ? 'This creates one canonical story artifact.' : 'The proposal remains auditable but cannot be edited.'}
        confirmLabel={pendingDecision === 'accept' ? 'Accept candidate' : 'Discard candidate'}
        onConfirm={confirmDecision}
        onCancel={() => setPendingDecision(null)}
      />
    </section>
  )
}

type CodexCandidateDraft = {
  type: CodexEntryType
  name: string
  aliases: string[]
  tags: string[]
  description: string
}

type StructuredCandidateDraft = {
  title: string
  parent_candidate_id?: string
}

function cloneProposal(proposal: ImportCandidateProposal): ImportCandidateProposal {
  return JSON.parse(JSON.stringify(proposal)) as ImportCandidateProposal
}

function codexDraftName(proposal: ImportCandidateProposal): string {
  return 'name' in proposal ? proposal.name : ''
}

function codexDraftDescription(proposal: ImportCandidateProposal): string {
  return 'description' in proposal ? proposal.description : ''
}

function structuredDraftTitle(proposal: ImportCandidateProposal): string {
  return 'title' in proposal ? proposal.title : ''
}

function isProposalValid(proposal: ImportCandidateProposal): boolean {
  if ('name' in proposal) {
    return proposal.name.trim().length > 0 && proposal.description.trim().length > 0
  }
  return 'title' in proposal && proposal.title.trim().length > 0
}
