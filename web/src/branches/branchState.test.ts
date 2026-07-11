// BDD Scenario: 8.2.1 - List exact changed files
// Requirements: M8-R06, M8-R18
// Test purpose: verify pure branch state keys, stale protection, selection, and invalidation.

import { expect, test, vi } from 'vitest'
import type { BranchContextKey, ChangedFile, ComparisonResponse, RamificationFinding } from './branchTypes'
import {
  applyComparisonFailure,
  applyComparisonSuccess,
  applyFileComparisonFailure,
  applyFileComparisonSuccess,
  applyRamificationSuccess,
  beginComparisonRequest,
  buildBranchContextKey,
  canProceedWithBranchChange,
  contextKeysEqual,
  initialBranchWorkbenchState,
  invalidateOnBranchChange,
  prunePromotionSelection,
  shouldAcceptResponse,
  togglePromotionPath,
} from './branchState'

const projectID = 'proj_story'
const mainHead = 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'
const experimentHead = 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb'
const fingerprint = `sha256:${'c'.repeat(64)}`
const experimentID = 'brn_0123456789abcdef0123'

const comparison: ComparisonResponse = {
  experiment_id: experimentID,
  branch_name: 'branch/obi-wan-lives-0123456789abcdef0123',
  main_head: mainHead,
  experiment_head: experimentHead,
  base_head: 'dddddddddddddddddddddddddddddddddddddddd',
  fingerprint,
  files: [
    { path: 'scenes/scn_0123456789abcdef0123.md', status: 'modified' },
    { path: 'scenes/scn_deleted.md', status: 'deleted' },
    { path: 'scenes/scn_added.md', status: 'added' },
  ],
}

function context(overrides: Partial<BranchContextKey> = {}): BranchContextKey {
  return buildBranchContextKey({
    projectID,
    experimentID,
    mainHead,
    experimentHead,
    fingerprint,
    selectedPath: 'scenes/scn_0123456789abcdef0123.md',
    ...overrides,
  })
}

// Test: state keys include project, experiment, both heads, fingerprint, and selected path.
// Requirements: M8-R06, M8-R18.
test('builds branch context keys with every stale dimension', () => {
  const key = context()
  expect(key).toEqual({
    projectID,
    experimentID,
    mainHead,
    experimentHead,
    fingerprint,
    selectedPath: 'scenes/scn_0123456789abcdef0123.md',
  })
})

// Test: stale list, file, analysis, and promotion responses are ignored.
// Requirements: M8-R18.
test('ignores stale comparison file and ramification responses', () => {
  const state = applyComparisonSuccess(
    initialBranchWorkbenchState(projectID),
    comparison,
    context(),
    0,
  )

  const staleContext = context({ fingerprint: `sha256:${'e'.repeat(64)}` })
  const staleComparison = applyComparisonSuccess(state, comparison, staleContext, 1)
  expect(staleComparison.comparison?.fingerprint).toBe(fingerprint)

  const staleFile = applyFileComparisonSuccess(
    staleComparison,
    {
      path: 'scenes/scn_0123456789abcdef0123.md',
      status: 'modified',
      main_head: mainHead,
      experiment_head: experimentHead,
      fingerprint,
      canon: { exists: true, text: 'Canon line' },
      experiment: { exists: true, text: 'Experiment line' },
    },
    context({ selectedPath: 'scenes/scn_added.md' }),
    3,
  )
  expect(staleFile.fileComparison).toBeNull()

  const staleVersion = applyRamificationSuccess(
    staleComparison,
    {
      summary: 'Stale summary',
      findings: [],
      provider: { profile_id: 'local_ollama', type: 'ollama', model: 'qwen2.5:7b' },
      manifest: {
        main_head: mainHead,
        experiment_head: experimentHead,
        fingerprint,
        changed_file_count: 1,
        included_paths: ['scenes/scn_0123456789abcdef0123.md'],
        estimated_input_bytes: 120,
      },
    },
    context(),
    staleComparison.requestVersion + 99,
  )
  expect(staleVersion.ramification).toBeNull()
})

// Test: stale comparison and file failures cannot overwrite current state or
// clear newer loading contexts.
// Requirements: M8-R18.
test('ignores stale comparison and file failures', () => {
  const state = applyComparisonSuccess(
    { ...initialBranchWorkbenchState(projectID), requestVersion: 2 },
    comparison,
    context(),
    2,
  )

  const staleComparisonFailure = applyComparisonFailure(state, experimentID, 'stale comparison', 1)
  expect(staleComparisonFailure.comparisonError).toBeNull()

  const staleFileFailure = applyFileComparisonFailure(
    state,
    context({ selectedPath: 'scenes/scn_added.md' }),
    2,
    'stale file',
  )
  expect(staleFileFailure.fileError).toBeNull()
})

// Test: beginning a new comparison clears stale comparison/file/analysis state
// and records the explicitly requested experiment id.
// Requirements: M8-R18.
test('clears stale state and tracks the requested experiment when loading a comparison', () => {
  const populated = {
    ...initialBranchWorkbenchState(projectID),
    comparison,
    fileComparison: {
      path: 'scenes/scn_0123456789abcdef0123.md',
      status: 'modified' as const,
      main_head: mainHead,
      experiment_head: experimentHead,
      fingerprint,
      canon: { exists: true, text: 'canon' },
      experiment: { exists: true, text: 'experiment' },
    },
    ramification: {
      summary: 'Summary',
      findings: [],
      provider: { profile_id: 'local', type: 'ollama' as const, model: 'model' },
      manifest: {
        main_head: mainHead,
        experiment_head: experimentHead,
        fingerprint,
        changed_file_count: 1,
        included_paths: ['scenes/scn_0123456789abcdef0123.md'],
        estimated_input_bytes: 100,
      },
    },
  }
  const next = beginComparisonRequest(populated, 'brn_other', 7)
  expect(next.requestedExperimentID).toBe('brn_other')
  expect(next.comparison).toBeNull()
  expect(next.fileComparison).toBeNull()
  expect(next.ramification).toBeNull()
  expect(next.selectedPath).toBeNull()
  expect(next.requestVersion).toBe(7)
})

// Test: promotion selection contains only current changed promotable paths.
// Requirements: M8-R13, M8-R18.
test('prunes promotion selection to current changed paths', () => {
  const changedFiles: ChangedFile[] = comparison.files
  const selected = prunePromotionSelection(
    ['scenes/scn_0123456789abcdef0123.md', 'outline.yaml', 'scenes/scn_old.md'],
    changedFiles,
  )
  expect(selected).toEqual(['scenes/scn_0123456789abcdef0123.md'])

  const toggled = togglePromotionPath([], 'scenes/scn_added.md', changedFiles)
  expect(toggled).toEqual(['scenes/scn_added.md'])
  expect(togglePromotionPath(toggled, 'outline.yaml', changedFiles)).toEqual(toggled)

  const byteSorted = togglePromotionPath(['scenes/scn_added.md'], 'scenes/scn_0123456789abcdef0123.md', changedFiles)
  expect(byteSorted).toEqual(['scenes/scn_0123456789abcdef0123.md', 'scenes/scn_added.md'])
})

// Test: branch change clears all branch-sensitive state.
// Requirements: M8-R18.
test('clears branch-sensitive state after branch change', () => {
  const populated = applyRamificationSuccess(
    applyComparisonSuccess(initialBranchWorkbenchState(projectID), comparison, context(), 1),
    {
      summary: 'Summary',
      findings: [],
      provider: { profile_id: 'local_ollama', type: 'ollama', model: 'qwen2.5:7b' },
      manifest: {
        main_head: mainHead,
        experiment_head: experimentHead,
        fingerprint,
        changed_file_count: 1,
        included_paths: ['scenes/scn_0123456789abcdef0123.md'],
        estimated_input_bytes: 120,
      },
    },
    context(),
    1,
  )

  const reset = invalidateOnBranchChange(projectID)
  expect(reset.comparison).toBeNull()
  expect(reset.fileComparison).toBeNull()
  expect(reset.ramification).toBeNull()
  expect(reset.selectedPaths).toEqual([])
  expect(reset.goal).toBe('')
  expect(reset.requestVersion).toBe(populated.requestVersion + 1)
})

// Test: dirty browser drafts require confirmation before branch-changing actions.
// Requirements: M8-R18.
test('requires confirmation when browser drafts are dirty', () => {
  expect(canProceedWithBranchChange(false, true)).toBe('allowed')
  expect(canProceedWithBranchChange(true, true)).toBe('confirm')
  expect(canProceedWithBranchChange(true, false)).toBe('blocked')
})

// Test: branch comparison transitions never write to browser storage.
// Requirements: M8-R18.
test('never writes branch comparison goals or findings to browser storage', () => {
  const setItemSpy = vi.spyOn(Storage.prototype, 'setItem')
  const state = applyComparisonSuccess(initialBranchWorkbenchState(projectID), comparison, context(), 0)
  const reset = invalidateOnBranchChange(projectID)
  expect(reset.comparison).toBeNull()
  expect(state.comparison?.fingerprint).toBe(fingerprint)
  expect(setItemSpy).not.toHaveBeenCalled()
  setItemSpy.mockRestore()
})

// Test: shouldAcceptResponse compares every context dimension.
// Requirements: M8-R18.
test('matches context keys across every stale dimension', () => {
  const left = context()
  const right = context({ mainHead: 'ffffffffffffffffffffffffffffffffffffffff' })
  expect(contextKeysEqual(left, left)).toBe(true)
  expect(contextKeysEqual(left, right)).toBe(false)
  expect(shouldAcceptResponse(left, right, 2, 2)).toBe(false)
  expect(shouldAcceptResponse(left, left, 1, 2)).toBe(false)
  expect(shouldAcceptResponse(left, left, 2, 2)).toBe(true)
})

// Test: shouldAcceptResponse rejects independently for each mismatched field.
// Requirements: M8-R18.
test('rejects stale responses per individual context field', () => {
  const base = context()
  const mismatches: Array<[string, Partial<BranchContextKey>]> = [
    ['projectID', { projectID: 'other' }],
    ['experimentID', { experimentID: 'brn_other' }],
    ['mainHead', { mainHead: 'ffffffffffffffffffffffffffffffffffffffff' }],
    ['experimentHead', { experimentHead: 'ffffffffffffffffffffffffffffffffffffffff' }],
    ['fingerprint', { fingerprint: `sha256:${'f'.repeat(64)}` }],
    ['selectedPath', { selectedPath: 'scenes/other.md' }],
  ]
  for (const [field, override] of mismatches) {
    const stale = context(override)
    expect(shouldAcceptResponse(base, stale, 2, 2), `field ${field}`).toBe(false)
  }
  expect(shouldAcceptResponse(base, base, 1, 2), 'requestVersion').toBe(false)
})

// Test: applyRamificationSuccess rejects independently for each mismatched
// context field including project, both heads, and selected path.
// Requirements: M8-R18.
test('applyRamificationSuccess rejects stale responses per context field', () => {
  const initialState = { ...initialBranchWorkbenchState(projectID), requestVersion: 1 }
  const state = applyComparisonSuccess(
    initialState,
    comparison,
    context(),
    1,
  )
  const ramification = {
    summary: 'Summary',
    findings: [] as RamificationFinding[],
    provider: { profile_id: 'local', type: 'ollama' as const, model: 'qwen2.5:7b' },
    manifest: {
      main_head: mainHead,
      experiment_head: experimentHead,
      fingerprint,
      changed_file_count: 1,
      included_paths: ['scenes/scn_0123456789abcdef0123.md'],
      estimated_input_bytes: 120,
    },
  }
  const result = applyRamificationSuccess(state, ramification, context(), 1)
  expect(result.ramification).not.toBeNull()

  const mismatches: Array<[string, Partial<BranchContextKey>]> = [
    ['projectID', { projectID: 'other' }],
    ['experimentID', { experimentID: 'brn_other' }],
    ['mainHead', { mainHead: 'ffffffffffffffffffffffffffffffffffffffff' }],
    ['experimentHead', { experimentHead: 'ffffffffffffffffffffffffffffffffffffffff' }],
    ['fingerprint', { fingerprint: `sha256:${'f'.repeat(64)}` }],
    ['selectedPath', { selectedPath: 'scenes/other.md' }],
  ]
  for (const [field, override] of mismatches) {
    const stale = context(override)
    const rejected = applyRamificationSuccess(state, ramification, stale, 1)
    expect(rejected.ramification, `field ${field}`).toBeNull()
  }
  const wrongVersion = applyRamificationSuccess(state, ramification, context(), 99)
  expect(wrongVersion.ramification, 'requestVersion').toBeNull()
})

// Test: applyRamificationSuccess rejects when manifest identity fields
// disagree with the captured request context.
// Requirements: M8-R18.
test('applyRamificationSuccess rejects mismatched manifest identity', () => {
  const initialState = { ...initialBranchWorkbenchState(projectID), requestVersion: 1 }
  const state = applyComparisonSuccess(
    initialState,
    comparison,
    context(),
    1,
  )
  const baseManifest = {
    main_head: mainHead,
    experiment_head: experimentHead,
    fingerprint,
    changed_file_count: 1,
    included_paths: ['scenes/scn_0123456789abcdef0123.md'],
    estimated_input_bytes: 120,
  }
  const base = {
    summary: 'Summary',
    findings: [] as RamificationFinding[],
    provider: { profile_id: 'local', type: 'ollama' as const, model: 'qwen2.5:7b' },
    manifest: baseManifest,
  }

  for (const [field, override] of [
    ['main_head', { main_head: 'ffffffffffffffffffffffffffffffffffffffff' }],
    ['experiment_head', { experiment_head: 'ffffffffffffffffffffffffffffffffffffffff' }],
    ['fingerprint', { fingerprint: `sha256:${'e'.repeat(64)}` }],
  ] as const) {
    const mismatched = {
      ...base,
      manifest: { ...baseManifest, ...override },
    }
    const rejected = applyRamificationSuccess(state, mismatched, context(), 1)
    expect(rejected.ramification, `manifest ${field}`).toBeNull()
  }
})

// Test: applyRamificationSuccess accepts a matching response and clears
// loading/error state.
// Requirements: M8-R18.
test('applyRamificationSuccess accepts matching response', () => {
  const initialState = { ...initialBranchWorkbenchState(projectID), requestVersion: 1 }
  const state = applyComparisonSuccess(
    initialState,
    comparison,
    context(),
    1,
  )
  const ramification = {
    summary: 'Summary',
    findings: [] as RamificationFinding[],
    provider: { profile_id: 'local', type: 'ollama' as const, model: 'qwen2.5:7b' },
    manifest: {
      main_head: mainHead,
      experiment_head: experimentHead,
      fingerprint,
      changed_file_count: 1,
      included_paths: ['scenes/scn_0123456789abcdef0123.md'],
      estimated_input_bytes: 120,
    },
  }
  const loading = { ...state, ramificationLoading: true, ramificationError: 'old error' }
  const result = applyRamificationSuccess(loading, ramification, context(), 1)
  expect(result.ramification).toBe(ramification)
  expect(result.ramificationLoading).toBe(false)
  expect(result.ramificationError).toBeNull()
})
