// BDD Scenario: 8.4.1 - Promote selected files to main
// Requirements: M8-R12, M8-R13, M8-R18
// Test purpose: verify whole-file promotion summary, confirmation, and conflicts.

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
const fingerprint = `sha256:${'c'.repeat(64)}`
const experimentID = 'brn_0123456789abcdef0123'

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
    fingerprint,
    files: [
      { path: 'scenes/scn_0123456789abcdef0123.md', status: 'modified' },
      { path: 'scenes/scn_other.md', status: 'modified' },
    ],
  })
  vi.mocked(api.getBranchFileComparison).mockResolvedValue({
    path: 'scenes/scn_0123456789abcdef0123.md',
    status: 'modified',
    main_head: mainHead,
    experiment_head: experimentHead,
    fingerprint,
    canon: { exists: true, text: 'alpha' },
    experiment: { exists: true, text: 'beta' },
  })
})

// Test: whole-file selection summary and no hunk controls.
// Requirements: M8-R13.
test('shows whole-file promotion summary without hunk controls', async () => {
  render(<BranchWorkbench project={project} appDirty={false} onDirtyChange={vi.fn()} onBranchChanged={vi.fn()} />)
  await waitFor(() => expect(screen.getByLabelText('Promote scenes/scn_0123456789abcdef0123.md')).toBeInTheDocument())

  fireEvent.click(screen.getByLabelText('Promote scenes/scn_0123456789abcdef0123.md'))
  await waitFor(() => expect(screen.getByText(/whole selected files/i)).toBeInTheDocument())
  expect(screen.getAllByText('scenes/scn_0123456789abcdef0123.md').length).toBeGreaterThan(0)
  expect(screen.queryByText(/hunk/i)).not.toBeInTheDocument()
})

// Test: expected refs, fingerprint, and confirmation are required.
// Requirements: M8-R12.
test('requires confirmation and sends expected refs and fingerprint on promote', async () => {
  vi.mocked(api.promoteExperimentFiles).mockResolvedValue({
    main_head: 'ffffffffffffffffffffffffffffffffffffffff',
    promoted_paths: ['scenes/scn_0123456789abcdef0123.md'],
    experiment_id: experimentID,
  })
  const onBranchChanged = vi.fn()
  render(<BranchWorkbench project={project} appDirty={false} onDirtyChange={vi.fn()} onBranchChanged={onBranchChanged} />)
  await waitFor(() => expect(screen.getByLabelText('Promote scenes/scn_0123456789abcdef0123.md')).toBeInTheDocument())

  fireEvent.click(screen.getByLabelText('Promote scenes/scn_0123456789abcdef0123.md'))
  fireEvent.click(screen.getByRole('button', { name: 'Promote selected files' }))
  await waitFor(() => expect(screen.getByRole('dialog', { name: 'Promote selected files?' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Promote to main' }))

  await waitFor(() => expect(api.promoteExperimentFiles).toHaveBeenCalledWith(experimentID, {
    paths: ['scenes/scn_0123456789abcdef0123.md'],
    expected_main_head: mainHead,
    expected_experiment_head: experimentHead,
    comparison_fingerprint: fingerprint,
  }))
  expect(onBranchChanged).toHaveBeenCalled()
})

// Test: dirty browser drafts require explicit discard confirmation before any
// promote API call.
// Requirements: M8-R18.
test('requires discard confirmation before promoting when the browser draft is dirty', async () => {
  vi.mocked(api.promoteExperimentFiles).mockResolvedValue({
    main_head: 'ffffffffffffffffffffffffffffffffffffffff',
    promoted_paths: ['scenes/scn_0123456789abcdef0123.md'],
    experiment_id: experimentID,
  })
  const onDirtyChange = vi.fn()
  const { rerender } = render(<BranchWorkbench project={project} appDirty onDirtyChange={onDirtyChange} onBranchChanged={vi.fn()} />)
  await waitFor(() => expect(screen.getByLabelText('Promote scenes/scn_0123456789abcdef0123.md')).toBeInTheDocument())

  fireEvent.click(screen.getByLabelText('Promote scenes/scn_0123456789abcdef0123.md'))
  fireEvent.click(screen.getByRole('button', { name: 'Promote selected files' }))

  await waitFor(() => expect(screen.getByRole('dialog', { name: 'Discard current draft?' })).toBeInTheDocument())
  expect(api.promoteExperimentFiles).not.toHaveBeenCalled()

  fireEvent.click(screen.getByRole('button', { name: 'Discard draft' }))
  expect(onDirtyChange).toHaveBeenCalledWith(false)
  rerender(<BranchWorkbench project={project} appDirty={false} onDirtyChange={onDirtyChange} onBranchChanged={vi.fn()} />)

  await waitFor(() => expect(screen.getByRole('dialog', { name: 'Promote selected files?' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Promote to main' }))

  await waitFor(() => expect(api.promoteExperimentFiles).toHaveBeenCalled())
})

// Test: changed-on-main conflict paths surface safe project-relative paths.
// Requirements: M8-R13.
test('shows conflict paths returned from promotion failures', async () => {
  vi.mocked(api.promoteExperimentFiles).mockRejectedValue(new api.APIError(409, 'main changed scenes/scn_0123456789abcdef0123.md since experiment base'))
  render(<BranchWorkbench project={project} appDirty={false} onDirtyChange={vi.fn()} onBranchChanged={vi.fn()} />)
  await waitFor(() => expect(screen.getByLabelText('Promote scenes/scn_0123456789abcdef0123.md')).toBeInTheDocument())

  fireEvent.click(screen.getByLabelText('Promote scenes/scn_0123456789abcdef0123.md'))
  fireEvent.click(screen.getByRole('button', { name: 'Promote selected files' }))
  fireEvent.click(screen.getByRole('button', { name: 'Promote to main' }))

  await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('scenes/scn_0123456789abcdef0123.md'))
})

// Test: successful promotion leaves main active while experiment remains listed.
// Requirements: M8-R14, M8-R16.
test('reports successful promotion with main active and experiment retained', async () => {
  const newMainHead = 'ffffffffffffffffffffffffffffffffffffffff'
  vi.mocked(api.getBranchStatus).mockResolvedValue({
    active_branch: 'branch/obi-wan-lives-0123456789abcdef0123',
    active_kind: 'experiment',
    main_head: mainHead,
    experiment_head: experimentHead,
    active_experiment_id: experimentID,
    worktree_clean: true,
  })
  vi.mocked(api.promoteExperimentFiles).mockImplementation(async () => {
    vi.mocked(api.getBranchStatus).mockResolvedValue({
      active_branch: 'main',
      active_kind: 'canon',
      main_head: newMainHead,
      experiment_head: null,
      active_experiment_id: null,
      worktree_clean: true,
    })
    return {
      main_head: newMainHead,
      promoted_paths: ['scenes/scn_0123456789abcdef0123.md'],
      experiment_id: experimentID,
    }
  })

  render(<BranchWorkbench project={project} appDirty={false} onDirtyChange={vi.fn()} onBranchChanged={vi.fn()} />)
  await waitFor(() => expect(screen.getByLabelText('Promote scenes/scn_0123456789abcdef0123.md')).toBeInTheDocument())
  fireEvent.click(screen.getByLabelText('Promote scenes/scn_0123456789abcdef0123.md'))
  fireEvent.click(screen.getByRole('button', { name: 'Promote selected files' }))
  fireEvent.click(screen.getByRole('button', { name: 'Promote to main' }))

  await waitFor(() => expect(screen.getByText('Canon', { selector: '.branch-badge' })).toBeInTheDocument())
  expect(screen.getByText('obi-wan-lives')).toBeInTheDocument()
  expect(screen.getByText(/promoted 1 whole file/i)).toBeInTheDocument()
})
