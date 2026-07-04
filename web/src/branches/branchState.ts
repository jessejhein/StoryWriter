/**
 * branchState.ts
 *
 * Pure Milestone 8 branch workflow state transitions for stale-response
 * protection, promotion selection, dirty guards, and invalidation.
 */

import type {
  BranchContextKey,
  ChangedFile,
  ComparisonResponse,
  FileComparisonResponse,
  RamificationResponse,
} from './branchTypes'

export type BranchWorkbenchState = {
  projectID: string
  comparison: ComparisonResponse | null
  comparisonLoading: boolean
  comparisonError: string | null
  fileComparison: FileComparisonResponse | null
  fileLoading: boolean
  fileError: string | null
  selectedPath: string | null
  selectedPaths: string[]
  ramification: RamificationResponse | null
  ramificationLoading: boolean
  ramificationError: string | null
  goal: string
  requestVersion: number
}

export const initialBranchWorkbenchState = (projectID: string): BranchWorkbenchState => ({
  projectID,
  comparison: null,
  comparisonLoading: false,
  comparisonError: null,
  fileComparison: null,
  fileLoading: false,
  fileError: null,
  selectedPath: null,
  selectedPaths: [],
  ramification: null,
  ramificationLoading: false,
  ramificationError: null,
  goal: '',
  requestVersion: 0,
})

export function buildBranchContextKey(input: BranchContextKey): BranchContextKey {
  return {
    projectID: input.projectID,
    experimentID: input.experimentID,
    mainHead: input.mainHead,
    experimentHead: input.experimentHead,
    fingerprint: input.fingerprint,
    selectedPath: input.selectedPath,
  }
}

export function contextKeysEqual(left: BranchContextKey, right: BranchContextKey): boolean {
  return left.projectID === right.projectID &&
    left.experimentID === right.experimentID &&
    left.mainHead === right.mainHead &&
    left.experimentHead === right.experimentHead &&
    left.fingerprint === right.fingerprint &&
    left.selectedPath === right.selectedPath
}

export function shouldAcceptResponse(
  expected: BranchContextKey,
  actual: BranchContextKey,
  expectedVersion: number,
  actualVersion: number,
): boolean {
  return expectedVersion === actualVersion && contextKeysEqual(expected, actual)
}

export function prunePromotionSelection(selectedPaths: string[], changedFiles: ChangedFile[]): string[] {
  const allowed = new Set(changedFiles.map((file) => file.path))
  return [...selectedPaths]
    .filter((path) => allowed.has(path))
    .sort((left, right) => left.localeCompare(right))
}

export function togglePromotionPath(
  selectedPaths: string[],
  path: string,
  changedFiles: ChangedFile[],
): string[] {
  const allowed = new Set(changedFiles.map((file) => file.path))
  if (!allowed.has(path)) {
    return selectedPaths
  }
  if (selectedPaths.includes(path)) {
    return selectedPaths.filter((selected) => selected !== path)
  }
  return prunePromotionSelection([...selectedPaths, path], changedFiles)
}

export function canProceedWithBranchChange(
  appDirty: boolean,
  worktreeClean: boolean,
): 'allowed' | 'confirm' | 'blocked' {
  if (!worktreeClean) {
    return 'blocked'
  }
  if (appDirty) {
    return 'confirm'
  }
  return 'allowed'
}

export function buildCurrentContext(state: BranchWorkbenchState): BranchContextKey {
  return buildBranchContextKey({
    projectID: state.projectID,
    experimentID: state.comparison?.experiment_id ?? null,
    mainHead: state.comparison?.main_head ?? '',
    experimentHead: state.comparison?.experiment_head ?? null,
    fingerprint: state.comparison?.fingerprint ?? '',
    selectedPath: state.selectedPath,
  })
}

export function applyComparisonSuccess(
  state: BranchWorkbenchState,
  comparison: ComparisonResponse,
  context: BranchContextKey,
  requestVersion: number,
): BranchWorkbenchState {
  if (!shouldAcceptResponse(
    {
      projectID: context.projectID,
      experimentID: comparison.experiment_id,
      mainHead: comparison.main_head,
      experimentHead: comparison.experiment_head,
      fingerprint: comparison.fingerprint,
      selectedPath: null,
    },
    {
      projectID: context.projectID,
      experimentID: context.experimentID,
      mainHead: context.mainHead,
      experimentHead: context.experimentHead,
      fingerprint: context.fingerprint,
      selectedPath: null,
    },
    requestVersion,
    state.requestVersion,
  )) {
    return state
  }

  const firstPath = comparison.files[0]?.path ?? null
  return {
    ...state,
    comparison,
    comparisonLoading: false,
    comparisonError: null,
    selectedPath: firstPath,
    selectedPaths: prunePromotionSelection(state.selectedPaths, comparison.files),
    ramification: null,
    ramificationError: null,
  }
}

export function applyFileComparisonSuccess(
  state: BranchWorkbenchState,
  fileComparison: FileComparisonResponse,
  expected: BranchContextKey,
  requestVersion: number,
): BranchWorkbenchState {
  const current = buildBranchContextKey({
    projectID: state.projectID,
    experimentID: state.comparison?.experiment_id ?? '',
    mainHead: state.comparison?.main_head ?? '',
    experimentHead: state.comparison?.experiment_head ?? '',
    fingerprint: state.comparison?.fingerprint ?? '',
    selectedPath: state.selectedPath,
  })
  if (!shouldAcceptResponse(expected, current, requestVersion, state.requestVersion)) {
    return state
  }
  if (state.selectedPath !== fileComparison.path) {
    return state
  }

  return {
    ...state,
    fileComparison,
    fileLoading: false,
    fileError: null,
  }
}

export function applyRamificationSuccess(
  state: BranchWorkbenchState,
  ramification: RamificationResponse,
  context: BranchContextKey,
  requestVersion: number,
): BranchWorkbenchState {
  const current: BranchContextKey = {
    projectID: state.projectID,
    experimentID: state.comparison?.experiment_id ?? null,
    mainHead: state.comparison?.main_head ?? '',
    experimentHead: state.comparison?.experiment_head ?? null,
    fingerprint: state.comparison?.fingerprint ?? '',
    selectedPath: state.selectedPath,
  }
  if (!shouldAcceptResponse(context, current, requestVersion, state.requestVersion)) {
    return state
  }
  if (
    ramification.manifest.main_head !== context.mainHead ||
    ramification.manifest.experiment_head !== context.experimentHead ||
    ramification.manifest.fingerprint !== context.fingerprint
  ) {
    return state
  }

  return {
    ...state,
    ramification,
    ramificationLoading: false,
    ramificationError: null,
  }
}

export function invalidateOnBranchChange(projectID: string): BranchWorkbenchState {
  const next = initialBranchWorkbenchState(projectID)
  return {
    ...next,
    requestVersion: next.requestVersion + 1,
  }
}
export function branchUsesBrowserStorage(): boolean {
  if (typeof window === 'undefined') {
    return false
  }
  const keys = [...Object.keys(localStorage), ...Object.keys(sessionStorage)]
  return keys.some((key) => /branch|comparison|ramification|experiment/i.test(key))
}