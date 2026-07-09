// BDD Scenario: 8.5.1 - Discard the active experiment
// Requirements: M8-R17, M8-R18
// Test purpose: verify discard confirmation, dirty guards, and state invalidation.

import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
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

const mainHead = `sha256:${'a'.repeat(64)}`
const experimentHead = `sha256:${'b'.repeat(64)}`
const experimentID = 'brn_0123456789abcdef0123'
const inactiveExperimentID = 'brn_0123456789abcdef0124'
const inactiveExperimentHead = `sha256:${'e'.repeat(64)}`

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
        head: inactiveExperimentHead,
        display_name: 'yoda-lives',
      },
    ],
  })
  vi.mocked(api.getBranchComparison).mockResolvedValue({
    experiment_id: experimentID,
    branch_name: 'branch/obi-wan-lives-0123456789abcdef0123',
    main_head: mainHead,
    experiment_head: experimentHead,
    base_head: `sha256:${'d'.repeat(64)}`,
    fingerprint: `sha256:${'c'.repeat(64)}`,
    files: [],
  })
})

// Test: active discard requires confirmation and sends expected experiment head.
// Requirements: M8-R17.
test('requires confirmation and sends expected experiment head on discard', async () => {
  vi.mocked(api.discardExperiment).mockResolvedValue({ main_head: mainHead })
  const onBranchChanged = vi.fn()
  render(<BranchWorkbench project={project} appDirty={false} onDirtyChange={vi.fn()} onBranchChanged={onBranchChanged} />)
  await waitFor(() => expect(screen.getByRole('button', { name: 'Discard experiment' })).toBeInTheDocument())

  fireEvent.click(screen.getByRole('button', { name: 'Discard experiment' }))
  await waitFor(() => expect(screen.getByRole('dialog', { name: 'Discard experiment?' })).toBeInTheDocument())
  fireEvent.click(within(screen.getByRole('dialog', { name: 'Discard experiment?' })).getByRole('button', { name: 'Discard experiment' }))

  await waitFor(() => expect(api.discardExperiment).toHaveBeenCalledWith(experimentID, {
    expected_experiment_head: experimentHead,
  }))
  expect(onBranchChanged).toHaveBeenCalled()
})

// Test: dirty browser drafts require explicit confirmation before discard.
// Requirements: M8-R18.
test('requires draft confirmation before discard while browser drafts are dirty', async () => {
  vi.mocked(api.discardExperiment).mockResolvedValue({ main_head: mainHead })
  const onDirtyChange = vi.fn()
  render(<BranchWorkbench project={project} appDirty onDirtyChange={onDirtyChange} onBranchChanged={vi.fn()} />)
  await waitFor(() => expect(screen.getByRole('button', { name: 'Discard experiment' })).toBeEnabled())

  fireEvent.click(screen.getByRole('button', { name: 'Discard experiment' }))
  await waitFor(() => expect(screen.getByRole('dialog', { name: 'Discard current draft?' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Keep editing' }))
  expect(api.discardExperiment).not.toHaveBeenCalled()

  fireEvent.click(screen.getByRole('button', { name: 'Discard experiment' }))
  await waitFor(() => expect(screen.getByRole('dialog', { name: 'Discard current draft?' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Discard draft' }))
  expect(onDirtyChange).toHaveBeenCalledWith(false)
  await waitFor(() => expect(screen.getByRole('dialog', { name: 'Discard experiment?' })).toBeInTheDocument())
  fireEvent.click(within(screen.getByRole('dialog', { name: 'Discard experiment?' })).getByRole('button', { name: 'Discard experiment' }))

  await waitFor(() => expect(api.discardExperiment).toHaveBeenCalledWith(experimentID, {
    expected_experiment_head: experimentHead,
  }))
})

// Test: dirty Git worktrees still block discard.
// Requirements: M8-R17, M8-R18.
test('blocks discard while the git worktree is dirty', async () => {
  vi.mocked(api.getBranchStatus).mockResolvedValue({
    active_branch: 'branch/obi-wan-lives-0123456789abcdef0123',
    active_kind: 'experiment',
    main_head: mainHead,
    experiment_head: experimentHead,
    active_experiment_id: experimentID,
    worktree_clean: false,
  })

  render(<BranchWorkbench project={project} appDirty={false} onDirtyChange={vi.fn()} onBranchChanged={vi.fn()} />)
  await waitFor(() => expect(screen.getByRole('button', { name: 'Discard experiment' })).toBeDisabled())
})

// Test: stale discard conflicts surface without mutating local state.
// Requirements: M8-R17.
test('shows stale discard conflicts without clearing the experiment list prematurely', async () => {
  vi.mocked(api.discardExperiment).mockRejectedValue(new api.APIError(409, 'experiment head is stale'))
  render(<BranchWorkbench project={project} appDirty={false} onDirtyChange={vi.fn()} onBranchChanged={vi.fn()} />)
  await waitFor(() => expect(screen.getByText('obi-wan-lives')).toBeInTheDocument())

  fireEvent.click(screen.getByRole('button', { name: 'Discard experiment' }))
  await waitFor(() => expect(screen.getByRole('dialog', { name: 'Discard experiment?' })).toBeInTheDocument())
  fireEvent.click(within(screen.getByRole('dialog', { name: 'Discard experiment?' })).getByRole('button', { name: 'Discard experiment' }))
  await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent(/stale/i))
  expect(screen.getByText('obi-wan-lives')).toBeInTheDocument()
})

// Test: pending controls disable during discard.
// Requirements: M8-R18.
test('disables pending controls while discard is in flight', async () => {
  let resolveDiscard: (value: unknown) => void
  const discardPromise = new Promise((resolve) => { resolveDiscard = resolve })
  vi.mocked(api.discardExperiment).mockImplementation(() => discardPromise as Promise<never>)

  render(<BranchWorkbench project={project} appDirty={false} onDirtyChange={vi.fn()} onBranchChanged={vi.fn()} />)
  await waitFor(() => expect(screen.getByRole('button', { name: 'Discard experiment' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Discard experiment' }))
  await waitFor(() => expect(screen.getByRole('dialog', { name: 'Discard experiment?' })).toBeInTheDocument())
  fireEvent.click(within(screen.getByRole('dialog', { name: 'Discard experiment?' })).getByRole('button', { name: 'Discard experiment' }))

  expect(screen.getByRole('button', { name: 'Create experiment' })).toBeDisabled()
  expect(screen.getByRole('button', { name: 'Analyze ramifications' })).toBeDisabled()

  resolveDiscard!({ main_head: mainHead })
  await waitFor(() => expect(screen.getByRole('button', { name: 'Discard experiment' })).not.toBeDisabled())
})

// Test: discarding a reviewed inactive experiment uses the selected experiment
// head and performs no branch switch.
// Requirements: M8-R17, M8-R18.
test('discards an inactive reviewed experiment without switching branches', async () => {
  vi.mocked(api.discardExperiment).mockResolvedValue({ main_head: mainHead })
  vi.mocked(api.getBranchComparison)
    .mockResolvedValueOnce({
      experiment_id: experimentID,
      branch_name: 'branch/obi-wan-lives-0123456789abcdef0123',
      main_head: mainHead,
      experiment_head: experimentHead,
      base_head: `sha256:${'d'.repeat(64)}`,
      fingerprint: `sha256:${'c'.repeat(64)}`,
      files: [],
    })
    .mockResolvedValueOnce({
      experiment_id: inactiveExperimentID,
      branch_name: 'branch/yoda-lives-0123456789abcdef0124',
      main_head: mainHead,
      experiment_head: inactiveExperimentHead,
      base_head: `sha256:${'d'.repeat(64)}`,
      fingerprint: `sha256:${'f'.repeat(64)}`,
      files: [],
    })

  render(<BranchWorkbench project={project} appDirty={false} onDirtyChange={vi.fn()} onBranchChanged={vi.fn()} />)
  await waitFor(() => expect(screen.getByText('yoda-lives')).toBeInTheDocument())

  fireEvent.click(screen.getByRole('button', { name: 'yoda-lives' }))
  await waitFor(() => expect(api.getBranchComparison).toHaveBeenLastCalledWith(inactiveExperimentID))
  fireEvent.click(screen.getByRole('button', { name: 'Discard experiment' }))
  await waitFor(() => expect(screen.getByRole('dialog', { name: 'Discard experiment?' })).toBeInTheDocument())
  fireEvent.click(within(screen.getByRole('dialog', { name: 'Discard experiment?' })).getByRole('button', { name: 'Discard experiment' }))

  await waitFor(() => expect(api.discardExperiment).toHaveBeenCalledWith(inactiveExperimentID, {
    expected_experiment_head: inactiveExperimentHead,
  }))
  expect(api.switchBranch).not.toHaveBeenCalled()
})
