/**
 * BranchWorkbench.tsx
 *
 * Milestone 8 what-if experiment lifecycle, comparison, ramification,
 * promotion, and discard workspace.
 */

import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  analyzeBranchRamifications,
  createExperiment,
  discardExperiment,
  getBranchComparison,
  getBranchFileComparison,
  getBranchStatus,
  getProviderProfiles,
  listExperiments,
  promoteExperimentFiles,
  switchBranch,
  type Project,
  type ProviderProfile,
} from '../api'
import ConfirmDialog from '../components/ConfirmDialog'
import type {
  BranchStatusResponse,
  ExperimentSummary,
} from './branchTypes'
import {
  applyComparisonFailure,
  applyComparisonSuccess,
  applyFileComparisonFailure,
  applyFileComparisonSuccess,
  applyRamificationSuccess,
  beginComparisonRequest,
  buildBranchContextKey,
  canProceedWithBranchChange,
  initialBranchWorkbenchState,
  invalidateOnBranchChange,
  togglePromotionPath,
  type BranchWorkbenchState,
} from './branchState'
import RamificationResults from './RamificationResults'
import SideBySideDiff from './SideBySideDiff'

type Props = {
  project: Project
  appDirty: boolean
  onDirtyChange?: (dirty: boolean) => void
  onBranchChanged: () => void
}

type PendingAction =
  | { kind: 'switch'; target: 'main' | string; expectedHead?: string }
  | { kind: 'create'; name: string }
  | { kind: 'promote' }
  | { kind: 'discard' }

export default function BranchWorkbench({ project, appDirty, onDirtyChange, onBranchChanged }: Props) {
  const [status, setStatus] = useState<BranchStatusResponse | null>(null)
  const [experiments, setExperiments] = useState<ExperimentSummary[]>([])
  const [workbench, setWorkbench] = useState<BranchWorkbenchState>(() => initialBranchWorkbenchState(project.project_id))
  const [selectedExperimentID, setSelectedExperimentID] = useState<string | null>(null)
  const [experimentName, setExperimentName] = useState('')
  const [profiles, setProfiles] = useState<ProviderProfile[]>([])
  const [profileID, setProfileID] = useState('')
  const [model, setModel] = useState('')
  const [statusMessage, setStatusMessage] = useState('Loading branch status…')
  const [error, setError] = useState('')
  const [promotionMessage, setPromotionMessage] = useState('')
  const [pendingAction, setPendingAction] = useState<PendingAction | null>(null)
  const [busy, setBusy] = useState<'create' | 'switch' | 'comparison' | 'file' | 'analysis' | 'promote' | 'discard' | null>(null)
  const [dirtyDraftAcknowledged, setDirtyDraftAcknowledged] = useState(false)
  const requestVersionRef = useRef(0)
  const liveRegionRef = useRef<HTMLParagraphElement>(null)

  useEffect(() => {
    onDirtyChange?.(false)
  }, [onDirtyChange])

  useEffect(() => {
    if (!appDirty) {
      setDirtyDraftAcknowledged(false)
    }
  }, [appDirty])

  const refreshStatus = useCallback(async () => {
    const [statusResponse, experimentResponse] = await Promise.all([
      getBranchStatus(),
      listExperiments(),
    ])
    setStatus(statusResponse)
    setExperiments(experimentResponse.experiments)
    setSelectedExperimentID((current) => {
      if (current && experimentResponse.experiments.some((experiment) => experiment.experiment_id === current)) {
        return current
      }
      if (statusResponse.active_experiment_id) {
        return statusResponse.active_experiment_id
      }
      return experimentResponse.experiments[0]?.experiment_id ?? null
    })
    return statusResponse
  }, [])

  useEffect(() => {
    void Promise.all([refreshStatus(), getProviderProfiles()])
      .then(([, profileResponse]) => {
        const ready = profileResponse.profiles.filter((profile) => profile.readiness === 'ready' && profile.capabilities.chat)
        setProfiles(ready)
        if (ready.length > 0) {
          setProfileID(ready[0].id)
        }
        setStatusMessage('Branch status loaded.')
      })
      .catch((requestError) => {
        setError(requestError instanceof Error ? requestError.message : 'Failed to load branch status.')
      })
  }, [refreshStatus])

  const comparisonContext = useMemo(() => {
    if (!selectedExperimentID || !workbench.comparison) {
      return null
    }
    return buildBranchContextKey({
      projectID: project.project_id,
      experimentID: selectedExperimentID,
      mainHead: workbench.comparison.main_head,
      experimentHead: workbench.comparison.experiment_head,
      fingerprint: workbench.comparison.fingerprint,
      selectedPath: workbench.selectedPath,
    })
  }, [project.project_id, selectedExperimentID, workbench.comparison, workbench.selectedPath])

  const loadComparison = useCallback(async (experimentID: string, requestVersion: number) => {
    setBusy('comparison')
    setWorkbench((state) => beginComparisonRequest(state, experimentID, requestVersion))
    try {
      const comparison = await getBranchComparison(experimentID)
      setWorkbench((state) => applyComparisonSuccess(
        state,
        comparison,
        buildBranchContextKey({
          projectID: project.project_id,
          experimentID,
          mainHead: comparison.main_head,
          experimentHead: comparison.experiment_head,
          fingerprint: comparison.fingerprint,
          selectedPath: null,
        }),
        requestVersion,
      ))
      if (requestVersionRef.current === requestVersion) {
        setStatusMessage(`Loaded comparison for ${comparison.branch_name}.`)
      }
    } catch (requestError) {
      const message = requestError instanceof Error ? requestError.message : 'Failed to load comparison.'
      setWorkbench((state) => applyComparisonFailure(state, experimentID, message, requestVersion))
      if (requestVersionRef.current === requestVersion) {
        setError(message)
      }
    } finally {
      if (requestVersionRef.current === requestVersion) {
        setBusy(null)
      }
    }
  }, [project.project_id])

  useEffect(() => {
    if (!selectedExperimentID) {
      return
    }
    requestVersionRef.current += 1
    const requestVersion = requestVersionRef.current
    void loadComparison(selectedExperimentID, requestVersion)
  }, [selectedExperimentID, loadComparison])

  useEffect(() => {
    if (!selectedExperimentID || !workbench.selectedPath || !workbench.comparison) {
      return
    }

    const requestVersion = requestVersionRef.current
    const path = workbench.selectedPath
    const context = buildBranchContextKey({
      projectID: project.project_id,
      experimentID: selectedExperimentID,
      mainHead: workbench.comparison.main_head,
      experimentHead: workbench.comparison.experiment_head,
      fingerprint: workbench.comparison.fingerprint,
      selectedPath: path,
    })

    let current = true
    setBusy('file')
    setWorkbench((state) => ({ ...state, fileLoading: true, fileError: null }))
    void getBranchFileComparison(selectedExperimentID, path)
      .then((fileComparison) => {
        if (!current) {
          return
        }
        setWorkbench((state) => applyFileComparisonSuccess(state, fileComparison, context, requestVersion))
      })
      .catch((requestError) => {
        if (!current) {
          return
        }
        const message = requestError instanceof Error ? requestError.message : 'Failed to load file comparison.'
        setWorkbench((state) => applyFileComparisonFailure(state, context, requestVersion, message))
      })
      .finally(() => {
        if (current) {
          setBusy(null)
        }
      })

    return () => { current = false }
  }, [
    project.project_id,
    selectedExperimentID,
    workbench.selectedPath,
    workbench.comparison,
    workbench.requestVersion,
  ])

  function resetBranchSensitiveState() {
    requestVersionRef.current += 1
    setDirtyDraftAcknowledged(false)
    setWorkbench((state) => ({
      ...invalidateOnBranchChange(project.project_id),
      requestVersion: state.requestVersion + 1,
    }))
    setPromotionMessage('')
    setError('')
  }

  async function afterBranchMutation(message: string, nextExperimentID: string | null) {
    resetBranchSensitiveState()
    onBranchChanged()
    await refreshStatus()
    if (nextExperimentID) {
      setSelectedExperimentID(nextExperimentID)
      void loadComparison(nextExperimentID, requestVersionRef.current)
    }
    setStatusMessage(message)
    liveRegionRef.current?.focus()
  }

  function requestBranchChange(action: PendingAction) {
    if (!status) {
      return
    }
    const guard = canProceedWithBranchChange(appDirty, status.worktree_clean)
    if (guard === 'blocked') {
      setError('Git worktree is dirty. Save or revert working-tree changes before switching branches.')
      return
    }
    if (guard === 'confirm') {
      setPendingAction(action)
      return
    }
    setPendingAction(action)
  }

  async function executeBranchChange(action: PendingAction) {
    if (!status) {
      return
    }
    setDirtyDraftAcknowledged(false)
    setPendingAction(null)
    setError('')
    try {
      if (action.kind === 'switch') {
        setBusy('switch')
        const nextStatus = action.target === 'main'
          ? await switchBranch({ target: 'main' })
          : await switchBranch({ target: action.target, expected_head: action.expectedHead ?? '' })
        setStatus(nextStatus)
        await afterBranchMutation(`Switched to ${nextStatus.active_branch}.`, nextStatus.active_experiment_id)
      } else if (action.kind === 'promote') {
        await runPromotion()
      } else if (action.kind === 'discard') {
        await runDiscard()
      }
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : 'Branch action failed.')
    } finally {
      setBusy(null)
    }
  }

  async function runCreateExperiment(name: string) {
    setBusy('create')
    setError('')
    try {
      const nextStatus = await createExperiment(name)
      setExperimentName('')
      setStatus(nextStatus)
      setSelectedExperimentID(nextStatus.active_experiment_id)
      await afterBranchMutation(`Created experiment ${nextStatus.active_branch}.`, nextStatus.active_experiment_id)
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : 'Failed to create experiment.')
    } finally {
      setBusy(null)
    }
  }

  async function handleCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const name = experimentName.trim()
    if (!name || !status?.worktree_clean) {
      return
    }
    if (canProceedWithBranchChange(appDirty, status.worktree_clean) === 'confirm') {
      setPendingAction({ kind: 'create', name })
      return
    }
    await runCreateExperiment(name)
  }

  async function runPromotion() {
    if (!selectedExperimentID || !workbench.comparison || workbench.selectedPaths.length === 0) {
      return
    }
    setBusy('promote')
    setError('')
    try {
      const response = await promoteExperimentFiles(selectedExperimentID, {
        paths: workbench.selectedPaths,
        expected_main_head: workbench.comparison.main_head,
        expected_experiment_head: workbench.comparison.experiment_head,
        comparison_fingerprint: workbench.comparison.fingerprint,
      })
      await afterBranchMutation('Promotion complete. Main is now active.', selectedExperimentID)
      setPromotionMessage(`Promoted ${response.promoted_paths.length} whole file${response.promoted_paths.length === 1 ? '' : 's'} to main.`)
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : 'Promotion failed.')
    } finally {
      setBusy(null)
    }
  }

  async function runDiscard() {
    if (!selectedExperimentID) {
      return
    }
    const selectedExperiment = experiments.find((experiment) => experiment.experiment_id === selectedExperimentID)
    if (!selectedExperiment) {
      return
    }
    setBusy('discard')
    setError('')
    try {
      await discardExperiment(selectedExperimentID, { expected_experiment_head: selectedExperiment.head })
      setSelectedExperimentID(null)
      await afterBranchMutation('Experiment discarded.', null)
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : 'Discard failed.')
    } finally {
      setBusy(null)
    }
  }

  async function runAnalysis() {
    if (!selectedExperimentID || !profileID) {
      return
    }

    const comparison = workbench.comparison
    const goal = workbench.goal.trim()
    if (!comparison || !goal) {
      return
    }

    requestVersionRef.current += 1
    const requestVersion = requestVersionRef.current
    const context = buildBranchContextKey({
      projectID: project.project_id,
      experimentID: selectedExperimentID,
      mainHead: comparison.main_head,
      experimentHead: comparison.experiment_head,
      fingerprint: comparison.fingerprint,
      selectedPath: workbench.selectedPath,
    })
    setBusy('analysis')
    setWorkbench((state) => ({
      ...state,
      ramificationLoading: true,
      ramificationError: null,
      requestVersion,
    }))
    try {
      const response = await analyzeBranchRamifications(selectedExperimentID, {
        goal,
        profile_id: profileID,
        model: model.trim(),
        expected_main_head: comparison.main_head,
        expected_experiment_head: comparison.experiment_head,
        comparison_fingerprint: comparison.fingerprint,
      })
      setWorkbench((state) => applyRamificationSuccess(state, response, context, requestVersion))
      if (requestVersionRef.current === requestVersion) {
        setStatusMessage('Ramification analysis complete.')
      }
    } catch (requestError) {
      if (requestVersionRef.current === requestVersion) {
        setWorkbench((state) => ({
          ...state,
          ramificationLoading: false,
          ramificationError: requestError instanceof Error ? requestError.message : 'Ramification analysis failed.',
        }))
        setError(requestError instanceof Error ? requestError.message : 'Ramification analysis failed.')
      }
    } finally {
      if (requestVersionRef.current === requestVersion) {
        setBusy(null)
      }
    }
  }

  const activeBadge = status?.active_kind === 'experiment' ? 'Experiment' : 'Canon'
  const changedFiles = workbench.comparison?.files ?? []
  const effectiveAppDirty = appDirty && !dirtyDraftAcknowledged
  const branchChangeDisposition = canProceedWithBranchChange(effectiveAppDirty, status?.worktree_clean ?? false)
  const needsDiscardConfirmation = pendingAction !== null && branchChangeDisposition === 'confirm'
  const pending = busy !== null
  const shouldRenderFileComparison = workbench.comparisonLoading || workbench.fileLoading || workbench.fileError !== null || workbench.selectedPath !== null

  return (
    <section className="branch-shell">
      <aside className="branch-meta">
        <p className="folio">Milestone 8 / Branches</p>
        <h2>What-if experiments against canon.</h2>
        <p>Create controlled experiments from main, compare every changed file against current canon, analyze ramifications explicitly, and promote or discard whole files conservatively.</p>
        <p className="branch-badge" aria-live="polite">{activeBadge}</p>
        {status && <p><span className="section-label">Active branch</span> <code>{status.active_branch}</code></p>}
        <p className="branch-message" ref={liveRegionRef} tabIndex={-1}>{statusMessage}</p>
        {promotionMessage && <p className="branch-success">{promotionMessage}</p>}
        {error && <p className="error" role="alert">{error}</p>}
      </aside>

      <div className="branch-panel">
        <div className="branch-toolbar">
          <div>
            <span className="section-label">Project</span>
            <strong>{project.name ?? project.project_id}</strong>
          </div>
          <div className="branch-toolbar-actions">
            {status?.active_kind === 'experiment' && (
              <button disabled={pending || !status.worktree_clean} onClick={() => requestBranchChange({ kind: 'switch', target: 'main' })} type="button">
                Switch to main
              </button>
            )}
          </div>
        </div>

        <form className="inline-form" onSubmit={(event) => void handleCreate(event)}>
          <label>
            Experiment name
            <input disabled={pending || !status?.worktree_clean} onChange={(event) => setExperimentName(event.target.value)} value={experimentName} />
          </label>
          <button disabled={busy === 'create' || !experimentName.trim() || !status?.worktree_clean} type="submit">Create experiment</button>
        </form>

        <section aria-labelledby="branch-experiments-heading">
          <h3 id="branch-experiments-heading">Experiments</h3>
          {experiments.length === 0 ? (
            <p className="branch-message">No experiments yet.</p>
          ) : (
            <>
              <ul className="branch-experiment-list">
                {experiments.map((experiment) => (
                  <li key={experiment.experiment_id}>
                    <button
                      className={selectedExperimentID === experiment.experiment_id ? '' : 'secondary'}
                      disabled={pending}
                      onClick={() => setSelectedExperimentID(experiment.experiment_id)}
                      type="button"
                    >
                      {experiment.display_name}
                    </button>
                  </li>
                ))}
              </ul>
              {selectedExperimentID && status?.active_experiment_id !== selectedExperimentID && (
                <button
                  className="secondary"
                  disabled={pending}
                  onClick={() => {
                    const selectedExperiment = experiments.find((experiment) => experiment.experiment_id === selectedExperimentID)
                    if (!selectedExperiment) {
                      return
                    }
                    requestBranchChange({
                      kind: 'switch',
                      target: selectedExperiment.experiment_id,
                      expectedHead: selectedExperiment.head,
                    })
                  }}
                  type="button"
                >
                  Switch to reviewed experiment
                </button>
              )}
            </>
          )}
        </section>

        {selectedExperimentID && (
          <>
            <section aria-labelledby="branch-changed-files-heading">
              <h3 id="branch-changed-files-heading">Changed files</h3>
              {changedFiles.length === 0 ? (
                <p className="branch-message">No changed files in this comparison.</p>
              ) : (
                <ul className="branch-file-list">
                  {changedFiles.map((file) => (
                    <li key={file.path}>
                      <button
                        className={workbench.selectedPath === file.path ? '' : 'secondary'}
                        disabled={pending}
                        onClick={() => setWorkbench((state) => ({ ...state, selectedPath: file.path }))}
                        type="button"
                      >
                        {file.path} <span className="branch-file-status">{file.status}</span>
                      </button>
                      <label className="checkbox-field">
                        <input
                          aria-label={`Promote ${file.path}`}
                          checked={workbench.selectedPaths.includes(file.path)}
                          disabled={pending}
                          onChange={() => setWorkbench((state) => ({
                            ...state,
                            selectedPaths: togglePromotionPath(state.selectedPaths, file.path, changedFiles),
                          }))}
                          type="checkbox"
                        />
                        Promote whole file
                      </label>
                    </li>
                  ))}
                </ul>
              )}
            </section>

            {workbench.selectedPaths.length > 0 && (
              <section className="branch-promotion-summary">
                <h3>Promotion summary</h3>
                <p>Promotion copies whole selected files onto main. Partial-file promotion is not available.</p>
                <ul>
                  {workbench.selectedPaths.map((path) => <li key={path}><code>{path}</code></li>)}
                </ul>
                {status?.active_experiment_id !== selectedExperimentID && (
                  <p className="section-note">Switch to the reviewed experiment before promoting files.</p>
                )}
                <button disabled={pending || !status?.worktree_clean || status?.active_experiment_id !== selectedExperimentID} onClick={() => requestBranchChange({ kind: 'promote' })} type="button">
                  Promote selected files
                </button>
              </section>
            )}

            {shouldRenderFileComparison ? (
              workbench.selectedPath ? (
                <SideBySideDiff
                  canon={workbench.fileComparison?.canon ?? { exists: false, text: '' }}
                  error={workbench.fileError}
                  experiment={workbench.fileComparison?.experiment ?? { exists: false, text: '' }}
                  experimentHead={workbench.comparison?.experiment_head ?? ''}
                  fingerprint={workbench.comparison?.fingerprint ?? ''}
                  loading={workbench.fileLoading || workbench.comparisonLoading}
                  mainHead={workbench.comparison?.main_head ?? ''}
                  path={workbench.selectedPath}
                  stale={Boolean(workbench.fileComparison && comparisonContext && workbench.fileComparison.path !== workbench.selectedPath)}
                  status={workbench.fileComparison?.status ?? changedFiles.find((file) => file.path === workbench.selectedPath)?.status ?? 'modified'}
                />
              ) : (
                <p className="branch-message">Select a changed file to compare.</p>
              )
            ) : null}

            <section className="branch-analysis-form">
              <h3>Ramification analysis</h3>
              <p className="section-note">Run analysis only after reviewing the current comparison. Analysis does not edit files.</p>
              <label>
                Analysis goal
                <textarea disabled={pending} onChange={(event) => setWorkbench((state) => ({ ...state, goal: event.target.value }))} rows={3} value={workbench.goal} />
              </label>
              <label>
                Provider profile
                <select disabled={pending || profiles.length === 0} onChange={(event) => setProfileID(event.target.value)} value={profileID}>
                  {profiles.map((profile) => <option key={profile.id} value={profile.id}>{profile.name}</option>)}
                </select>
              </label>
              <label>
                Model
                <input disabled={pending} onChange={(event) => setModel(event.target.value)} value={model} />
              </label>
              <button disabled={busy === 'analysis' || !workbench.goal.trim() || !profileID} onClick={() => void runAnalysis()} type="button">
                Analyze ramifications
              </button>
            </section>

            <RamificationResults
              error={workbench.ramificationError}
              loading={workbench.ramificationLoading}
              result={workbench.ramification}
              stale={Boolean(workbench.ramification && workbench.comparison && workbench.ramification.manifest.fingerprint !== workbench.comparison.fingerprint)}
            />

            <div className="branch-danger-actions">
              <button
                disabled={pending || !status?.worktree_clean}
                onClick={() => requestBranchChange({ kind: 'discard' })}
                type="button"
              >
                Discard experiment
              </button>
            </div>
          </>
        )}
      </div>

        <ConfirmDialog
          cancelLabel="Keep editing"
          confirmLabel="Discard draft"
          message="You have unsaved changes in the current workspace. Discard them and continue?"
          onCancel={() => {
            setDirtyDraftAcknowledged(false)
            setPendingAction(null)
          }}
          onConfirm={() => {
            if (pendingAction?.kind === 'create') {
              setPendingAction(null)
              onDirtyChange?.(false)
              void runCreateExperiment(pendingAction.name)
              return
            }
            if (pendingAction?.kind === 'switch') {
              onDirtyChange?.(false)
              void executeBranchChange(pendingAction)
              return
            }
            setDirtyDraftAcknowledged(true)
            onDirtyChange?.(false)
          }}
          open={needsDiscardConfirmation}
          title="Discard current draft?"
        />

        <ConfirmDialog
          cancelLabel="Stay on branch"
          confirmLabel="Switch branch"
          message="Switch branches now? Branch-sensitive loaded state will be cleared and reloaded from the selected tree."
          onCancel={() => {
            setDirtyDraftAcknowledged(false)
            setPendingAction(null)
          }}
          onConfirm={() => { if (pendingAction?.kind === 'switch') void executeBranchChange(pendingAction) }}
          open={pendingAction?.kind === 'switch' && !needsDiscardConfirmation && branchChangeDisposition === 'allowed'}
          title="Switch branches?"
        />

        <ConfirmDialog
          cancelLabel="Keep reviewing"
          confirmLabel="Promote to main"
          message="Promote the selected whole files onto main? This creates one promotion commit and leaves main active."
          onCancel={() => {
            setDirtyDraftAcknowledged(false)
            setPendingAction(null)
          }}
          onConfirm={() => void executeBranchChange({ kind: 'promote' })}
          open={pendingAction?.kind === 'promote' && !needsDiscardConfirmation}
          title="Promote selected files?"
        />

        <ConfirmDialog
          cancelLabel="Keep experiment"
          confirmLabel="Discard experiment"
          message="Discard this experiment permanently? Main history will remain unchanged."
          onCancel={() => {
            setDirtyDraftAcknowledged(false)
            setPendingAction(null)
          }}
          onConfirm={() => void executeBranchChange({ kind: 'discard' })}
          open={pendingAction?.kind === 'discard' && !needsDiscardConfirmation}
          title="Discard experiment?"
        />
    </section>
  )
}
