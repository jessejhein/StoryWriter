// BDD Scenario: 8.2.1 - List exact changed files
// Requirements: M8-R06, M8-R18
// Test purpose: verify pure branch state keys, stale protection, selection, and invalidation.

import { expect, test } from 'vitest'
import type { BranchContextKey, ChangedFile, ComparisonResponse } from './branchTypes'
import {
  applyComparisonSuccess,
  applyFileComparisonSuccess,
  applyRamificationSuccess,
  branchUsesBrowserStorage,
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
const mainHead = `sha256:${'a'.repeat(64)}`
const experimentHead = `sha256:${'b'.repeat(64)}`
const fingerprint = `sha256:${'c'.repeat(64)}`
const experimentID = 'brn_0123456789abcdef0123'

const comparison: ComparisonResponse = {
  experiment_id: experimentID,
  branch_name: 'branch/obi-wan-lives-0123456789abcdef0123',
  main_head: mainHead,
  experiment_head: experimentHead,
  base_head: `sha256:${'d'.repeat(64)}`,
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

// Test: branch comparison text, goals, and findings never enter browser storage.
// Requirements: M8-R18.
test('never writes branch comparison goals or findings to browser storage', () => {
  expect(branchUsesBrowserStorage()).toBe(false)
})

// Test: shouldAcceptResponse compares every context dimension.
// Requirements: M8-R18.
test('matches context keys across every stale dimension', () => {
  const left = context()
  const right = context({ mainHead: `sha256:${'f'.repeat(64)}` })
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
    ['mainHead', { mainHead: `sha256:${'f'.repeat(64)}` }],
    ['experimentHead', { experimentHead: `sha256:${'f'.repeat(64)}` }],
    ['fingerprint', { fingerprint: `sha256:${'f'.repeat(64)}` }],
    ['selectedPath', { selectedPath: 'scenes/other.md' }],
  ]
  for (const [field, override] of mismatches) {
    const stale = context(override)
    expect(shouldAcceptResponse(base, stale, 2, 2), `field ${field}`).toBe(false)
  }
  expect(shouldAcceptResponse(base, base, 1, 2), 'requestVersion').toBe(false)
})