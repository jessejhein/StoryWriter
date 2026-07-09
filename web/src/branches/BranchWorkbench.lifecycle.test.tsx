// BDD Scenario: 8.1.1 - Create from current canon
// Requirements: M8-R01, M8-R02, M8-R18
// Test purpose: verify branch lifecycle controls, badges, and dirty guards.

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

const canonStatus = {
  active_branch: 'main',
  active_kind: 'canon' as const,
  main_head: mainHead,
  experiment_head: null,
  active_experiment_id: null,
  worktree_clean: true,
}

const experimentStatus = {
  active_branch: 'branch/obi-wan-lives-0123456789abcdef0123',
  active_kind: 'experiment' as const,
  main_head: mainHead,
  experiment_head: experimentHead,
  active_experiment_id: experimentID,
  worktree_clean: true,
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(api.getProviderProfiles).mockResolvedValue({
    profiles: [{
      id: 'local_ollama',
      name: 'Local Ollama',
      type: 'ollama',
      base_url: 'http://127.0.0.1:11434',
      auth: { type: 'none', credential_env: '' },
      capabilities: { chat: true, streaming: false, structured_output: false, max_context_tokens: 8192 },
      readiness: 'ready',
    }],
    revision: null,
  })
  vi.mocked(api.getBranchStatus).mockResolvedValue(canonStatus)
  vi.mocked(api.listExperiments).mockResolvedValue({
    experiments: [{
      experiment_id: experimentID,
      branch_name: 'branch/obi-wan-lives-0123456789abcdef0123',
      head: experimentHead,
      display_name: 'obi-wan-lives',
    }],
  })
  vi.mocked(api.getBranchComparison).mockResolvedValue({
    experiment_id: experimentID,
    branch_name: 'branch/obi-wan-lives-0123456789abcdef0123',
    main_head: mainHead,
    experiment_head: experimentHead,
    base_head: 'dddddddddddddddddddddddddddddddddddddddd',
    fingerprint: `sha256:${'c'.repeat(64)}`,
    files: [{ path: 'scenes/scn_0123456789abcdef0123.md', status: 'modified' }],
  })
  vi.mocked(api.getBranchFileComparison).mockResolvedValue({
    path: 'scenes/scn_0123456789abcdef0123.md',
    status: 'modified',
    main_head: mainHead,
    experiment_head: experimentHead,
    fingerprint: `sha256:${'c'.repeat(64)}`,
    canon: { exists: true, text: 'alpha' },
    experiment: { exists: true, text: 'beta' },
  })
})

// Test: create, switch, list, status, and Canon/Experiment badges.
// Requirements: M8-R01, M8-R02.
test('shows branch status badges and supports create and switch actions', async () => {
  vi.mocked(api.createExperiment).mockResolvedValue(experimentStatus)
  vi.mocked(api.getBranchStatus)
    .mockResolvedValueOnce(canonStatus)
    .mockResolvedValue(experimentStatus)
  render(<BranchWorkbench project={project} appDirty={false} onDirtyChange={vi.fn()} onBranchChanged={vi.fn()} />)

  await waitFor(() => expect(screen.getByText('Canon')).toBeInTheDocument())
  expect(screen.getByText('obi-wan-lives')).toBeInTheDocument()

  fireEvent.change(screen.getByLabelText('Experiment name'), { target: { value: 'Obi-Wan lives' } })
  await waitFor(() => expect(screen.getByRole('button', { name: 'Create experiment' })).not.toBeDisabled())
  fireEvent.click(screen.getByRole('button', { name: 'Create experiment' }))
  await waitFor(() => expect(api.createExperiment).toHaveBeenCalledWith('Obi-Wan lives'))

  vi.mocked(api.switchBranch).mockResolvedValue(canonStatus)
  await waitFor(() => expect(screen.getByRole('button', { name: 'Switch to main' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Switch to main' }))
  await waitFor(() => expect(screen.getByRole('dialog', { name: 'Switch branches?' })).toBeInTheDocument())
  expect(api.switchBranch).not.toHaveBeenCalled()
  fireEvent.click(screen.getByRole('button', { name: 'Switch branch' }))
  await waitFor(() => expect(api.switchBranch).toHaveBeenCalledWith({ target: 'main' }))
})

// Test: dirty browser draft guard and explicit confirmation before branch switch.
// Requirements: M8-R18.
test('requires confirmation before switching branches when the app draft is dirty', async () => {
  const onBranchChanged = vi.fn()
  render(<BranchWorkbench project={project} appDirty onDirtyChange={vi.fn()} onBranchChanged={onBranchChanged} />)
  await waitFor(() => expect(screen.getByRole('button', { name: 'Switch to reviewed experiment' })).toBeInTheDocument())

  fireEvent.click(screen.getByRole('button', { name: 'Switch to reviewed experiment' }))
  await waitFor(() => expect(screen.getByRole('dialog', { name: 'Discard current draft?' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Keep editing' }))
  expect(api.switchBranch).not.toHaveBeenCalled()

  fireEvent.click(screen.getByRole('button', { name: 'Switch to reviewed experiment' }))
  await waitFor(() => expect(screen.getByRole('dialog', { name: 'Discard current draft?' })).toBeInTheDocument())
  vi.mocked(api.switchBranch).mockResolvedValue(experimentStatus)
  fireEvent.click(screen.getByRole('button', { name: 'Discard draft' }))
  await waitFor(() => expect(api.switchBranch).toHaveBeenCalled())
  expect(onBranchChanged).toHaveBeenCalled()
})

// Test: dirty browser draft guard also covers experiment creation and only
// creates after explicit discard confirmation.
// Requirements: M8-R02, M8-R18.
test('requires confirmation before creating an experiment when the app draft is dirty', async () => {
  const onBranchChanged = vi.fn()
  const onDirtyChange = vi.fn()
  vi.mocked(api.createExperiment).mockResolvedValue(experimentStatus)
  vi.mocked(api.getBranchStatus)
    .mockResolvedValueOnce(canonStatus)
    .mockResolvedValue(experimentStatus)

  render(<BranchWorkbench project={project} appDirty onDirtyChange={onDirtyChange} onBranchChanged={onBranchChanged} />)

  await waitFor(() => expect(screen.getByText('Canon')).toBeInTheDocument())
  fireEvent.change(screen.getByLabelText('Experiment name'), { target: { value: 'Obi-Wan lives' } })
  await waitFor(() => expect(screen.getByRole('button', { name: 'Create experiment' })).not.toBeDisabled())
  fireEvent.click(screen.getByRole('button', { name: 'Create experiment' }))

  await waitFor(() => expect(screen.getByRole('dialog', { name: 'Discard current draft?' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Keep editing' }))
  expect(api.createExperiment).not.toHaveBeenCalled()

  fireEvent.click(screen.getByRole('button', { name: 'Create experiment' }))
  await waitFor(() => expect(screen.getByRole('dialog', { name: 'Discard current draft?' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Discard draft' }))

  await waitFor(() => expect(api.createExperiment).toHaveBeenCalledWith('Obi-Wan lives'))
  expect(onDirtyChange).toHaveBeenCalledWith(false)
  expect(onBranchChanged).toHaveBeenCalled()
})

// Test: the branch workbench does not clear a dirty parent draft merely by
// mounting.
// Requirements: M8-R18.
test('does not clear parent dirty state on mount', async () => {
  const onDirtyChange = vi.fn()
  render(<BranchWorkbench project={project} appDirty onDirtyChange={onDirtyChange} onBranchChanged={vi.fn()} />)

  await waitFor(() => expect(screen.getByText('Canon')).toBeInTheDocument())
  expect(onDirtyChange).not.toHaveBeenCalledWith(false)
})

// Test: successful switch to the reviewed experiment clears branch-sensitive
// state and refetches the reviewed comparison under the new active branch.
// Requirements: M8-R18.
test('clears comparison state and refetches after switching to the reviewed experiment', async () => {
  const onBranchChanged = vi.fn()
  render(<BranchWorkbench project={project} appDirty={false} onDirtyChange={vi.fn()} onBranchChanged={onBranchChanged} />)

  await waitFor(() => expect(api.getBranchComparison).toHaveBeenCalled())

  vi.mocked(api.getBranchComparison).mockClear()
  vi.mocked(api.switchBranch).mockResolvedValue(experimentStatus)
  vi.mocked(api.getBranchStatus).mockResolvedValue(experimentStatus)
  await waitFor(() => expect(screen.getByRole('button', { name: 'Switch to reviewed experiment' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Switch to reviewed experiment' }))
  await waitFor(() => expect(screen.getByRole('dialog', { name: 'Switch branches?' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Switch branch' }))
  await waitFor(() => expect(onBranchChanged).toHaveBeenCalled())
  await waitFor(() => expect(api.getBranchComparison).toHaveBeenCalled())
})
