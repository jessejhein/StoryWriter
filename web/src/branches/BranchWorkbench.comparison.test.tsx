// BDD Scenario: 8.2.2 - Show side-by-side text
// Requirements: M8-R05, M8-R08
// Test purpose: verify changed-file list, file fetch, and no automatic analysis.

import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, expect, test, vi } from 'vitest'
import type { Project } from '../api'
import BranchWorkbench from './BranchWorkbench'

vi.mock('../api', () => ({
  getBranchStatus: vi.fn(),
  listExperiments: vi.fn(),
  createExperiment: vi.fn(),
  switchBranch: vi.fn(),
  getBranchComparison: vi.fn(),
  getBranchFileComparison: vi.fn(),
  analyzeBranchRamifications: vi.fn(),
  promoteExperimentFiles: vi.fn(),
  discardExperiment: vi.fn(),
  getProviderProfiles: vi.fn(),
  APIError: class APIError extends Error {
    status: number
    constructor(status: number, message: string) {
      super(message)
      this.status = status
    }
  },
}))

const api = await import('../api')

const project: Project = {
  project_id: 'proj_story',
  name: 'Story',
  path: '/tmp/story',
  git_initialized: true,
  index_initialized: true,
}

const mainHead = 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'
const experimentHead = 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb'
const experimentID = 'brn_0123456789abcdef0123'
const inactiveExperimentID = 'brn_0123456789abcdef0124'

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(api.getProviderProfiles).mockResolvedValue({ profiles: [], revision: null })
  vi.mocked(api.getBranchStatus).mockResolvedValue({
    active_branch: 'branch/obi-wan-lives-0123456789abcdef0123',
    active_kind: 'experiment',
    main_head: mainHead,
    experiment_head: experimentHead,
    active_experiment_id: experimentID,
    worktree_clean: true,
  })
  vi.mocked(api.listExperiments).mockResolvedValue({
    experiments: [
      {
        experiment_id: experimentID,
        branch_name: 'branch/obi-wan-lives-0123456789abcdef0123',
        head: experimentHead,
        display_name: 'obi-wan-lives',
      },
      {
        experiment_id: inactiveExperimentID,
        branch_name: 'branch/yoda-lives-0123456789abcdef0124',
        head: 'eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee',
        display_name: 'yoda-lives',
      },
    ],
  })
  vi.mocked(api.getBranchComparison).mockResolvedValue({
    experiment_id: experimentID,
    branch_name: 'branch/obi-wan-lives-0123456789abcdef0123',
    main_head: mainHead,
    experiment_head: experimentHead,
    base_head: 'dddddddddddddddddddddddddddddddddddddddd',
    fingerprint: `sha256:${'c'.repeat(64)}`,
    files: [
      { path: 'scenes/scn_0123456789abcdef0123.md', status: 'modified' },
      { path: 'scenes/scn_added.md', status: 'added' },
    ],
  })
  vi.mocked(api.getBranchFileComparison).mockResolvedValue({
    path: 'scenes/scn_0123456789abcdef0123.md',
    status: 'modified',
    main_head: mainHead,
    experiment_head: experimentHead,
    fingerprint: `sha256:${'c'.repeat(64)}`,
    canon: { exists: true, text: 'alpha\nbeta' },
    experiment: { exists: true, text: 'alpha\ngamma' },
  })
})

// Test: selecting an inactive experiment for review loads comparison without a checkout.
// Requirements: M8-R05, M8-R17.
test('reviews an inactive experiment without switching branches', async () => {
  vi.mocked(api.getBranchComparison)
    .mockResolvedValueOnce({
      experiment_id: experimentID,
      branch_name: 'branch/obi-wan-lives-0123456789abcdef0123',
      main_head: mainHead,
      experiment_head: experimentHead,
      base_head: 'dddddddddddddddddddddddddddddddddddddddd',
      fingerprint: `sha256:${'c'.repeat(64)}`,
      files: [],
    })
    .mockResolvedValueOnce({
      experiment_id: inactiveExperimentID,
      branch_name: 'branch/yoda-lives-0123456789abcdef0124',
      main_head: mainHead,
      experiment_head: 'eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee',
      base_head: 'dddddddddddddddddddddddddddddddddddddddd',
      fingerprint: `sha256:${'f'.repeat(64)}`,
      files: [],
    })

  render(<BranchWorkbench project={project} appDirty={false} onDirtyChange={vi.fn()} onBranchChanged={vi.fn()} />)
  await waitFor(() => expect(api.getBranchComparison).toHaveBeenCalledWith(experimentID))

  fireEvent.click(screen.getByRole('button', { name: 'yoda-lives' }))
  await waitFor(() => expect(api.getBranchComparison).toHaveBeenCalledWith(inactiveExperimentID))
  expect(api.switchBranch).not.toHaveBeenCalled()
})

// Test: changed-file status list, selection, and file fetch.
// Requirements: M8-R05, M8-R08.
test('lists changed files and loads side-by-side comparison for the selected path', async () => {
  render(<BranchWorkbench project={project} appDirty={false} onDirtyChange={vi.fn()} onBranchChanged={vi.fn()} />)

  await waitFor(() => expect(screen.getAllByText('scenes/scn_0123456789abcdef0123.md').length).toBeGreaterThan(0))
  expect(screen.getAllByText(/modified/i).length).toBeGreaterThan(0)
  await waitFor(() => expect(api.getBranchFileComparison).toHaveBeenCalledWith(
    experimentID,
    'scenes/scn_0123456789abcdef0123.md',
  ))
  expect(screen.getByText('Canon (main)')).toBeInTheDocument()
  expect(screen.getByText('beta')).toBeInTheDocument()
  expect(screen.getByText('gamma')).toBeInTheDocument()

  fireEvent.click(screen.getByRole('button', { name: /scenes\/scn_added\.md/i }))
  await waitFor(() => expect(api.getBranchFileComparison).toHaveBeenCalledWith(experimentID, 'scenes/scn_added.md'))
})

// Test: an empty comparison renders only the empty state and skips file loading.
// Requirements: M8-R08.
test('renders a true empty state when a comparison has no changed files', async () => {
  vi.mocked(api.getBranchComparison).mockResolvedValue({
    experiment_id: experimentID,
    branch_name: 'branch/obi-wan-lives-0123456789abcdef0123',
    main_head: mainHead,
    experiment_head: experimentHead,
    base_head: 'dddddddddddddddddddddddddddddddddddddddd',
    fingerprint: `sha256:${'c'.repeat(64)}`,
    files: [],
  })

  render(<BranchWorkbench project={project} appDirty={false} onDirtyChange={vi.fn()} onBranchChanged={vi.fn()} />)

  await waitFor(() => expect(screen.getByText('No changed files in this comparison.')).toBeInTheDocument())
  expect(api.getBranchFileComparison).not.toHaveBeenCalled()
  expect(screen.queryByRole('heading', { name: 'Canon (main)' })).not.toBeInTheDocument()
  expect(screen.queryByRole('heading', { name: 'Experiment' })).not.toBeInTheDocument()
  expect(screen.queryByText(mainHead)).not.toBeInTheDocument()
  expect(screen.queryByText(experimentHead)).not.toBeInTheDocument()
})

// Test: merely opening comparison never calls analysis.
// Requirements: M8-R09.
test('does not call ramification analysis when listing or comparing files', async () => {
  render(<BranchWorkbench project={project} appDirty={false} onDirtyChange={vi.fn()} onBranchChanged={vi.fn()} />)
  await waitFor(() => expect(api.getBranchComparison).toHaveBeenCalled())
  await waitFor(() => expect(api.getBranchFileComparison).toHaveBeenCalled())
  expect(api.analyzeBranchRamifications).not.toHaveBeenCalled()
})

// Test: stale responses after navigation are ignored.
// Requirements: M8-R18.
test('ignores stale file comparison responses after path changes', async () => {
  let resolveSecond: (value: unknown) => void
  const secondPromise = new Promise((resolve) => { resolveSecond = resolve })
  vi.mocked(api.getBranchFileComparison)
    .mockResolvedValueOnce({
      path: 'scenes/scn_0123456789abcdef0123.md',
      status: 'modified',
      main_head: mainHead,
      experiment_head: experimentHead,
      fingerprint: `sha256:${'c'.repeat(64)}`,
      canon: { exists: true, text: 'stale canon' },
      experiment: { exists: true, text: 'stale experiment' },
    })
    .mockImplementationOnce(() => secondPromise as Promise<never>)

  render(<BranchWorkbench project={project} appDirty={false} onDirtyChange={vi.fn()} onBranchChanged={vi.fn()} />)
  await waitFor(() => expect(screen.getByText('stale canon')).toBeInTheDocument())

  fireEvent.click(screen.getByRole('button', { name: /scenes\/scn_added\.md/i }))
  resolveSecond!({
    path: 'scenes/scn_added.md',
    status: 'added',
    main_head: mainHead,
    experiment_head: experimentHead,
    fingerprint: `sha256:${'c'.repeat(64)}`,
    canon: { exists: false, text: '' },
    experiment: { exists: true, text: 'fresh added line' },
  })

  await waitFor(() => expect(screen.queryByText('stale canon')).not.toBeInTheDocument())
  await waitFor(() => expect(screen.getByText('fresh added line')).toBeInTheDocument())
})

// Test: a late file-comparison failure after unmount cannot publish an
// obsolete error.
// Requirements: M8-R18.
test('ignores stale file comparison failures after unmount', async () => {
  const pending = deferred<never>()
  vi.mocked(api.getBranchFileComparison).mockImplementationOnce(() => pending.promise)

  const { unmount } = render(<BranchWorkbench project={project} appDirty={false} onDirtyChange={vi.fn()} onBranchChanged={vi.fn()} />)
  await waitFor(() => expect(api.getBranchFileComparison).toHaveBeenCalledWith(
    experimentID,
    'scenes/scn_0123456789abcdef0123.md',
  ))

  unmount()
  pending.reject(new Error('stale failure'))
  await Promise.resolve()
  expect(screen.queryByText('stale failure')).not.toBeInTheDocument()
})
