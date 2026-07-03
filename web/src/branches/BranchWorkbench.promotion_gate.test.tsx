// BDD Scenario: 8.4.1 - Promote only from the active experiment
// Requirements: M8-R12, M8-R13
// Test purpose: promotion UI is disabled when the reviewed experiment is not
// the active branch, preventing an opaque 409.

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

const mainHead = `sha256:${'a'.repeat(64)}`
const experimentHead = `sha256:${'b'.repeat(64)}`
const fingerprint = `sha256:${'c'.repeat(64)}`
const experimentID = 'brn_0123456789abcdef0123'

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(api.getProviderProfiles).mockResolvedValue({ profiles: [], revision: null })
  vi.mocked(api.getBranchComparison).mockResolvedValue({
    experiment_id: experimentID,
    branch_name: 'branch/obi-wan-lives-0123456789abcdef0123',
    main_head: mainHead,
    experiment_head: experimentHead,
    base_head: `sha256:${'d'.repeat(64)}`,
    fingerprint,
    files: [
      { path: 'scenes/scn_0123456789abcdef0123.md', status: 'modified' },
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

// Test: when main is active and a different experiment is reviewed, the promote
// button is disabled and selecting a file does not send a promote request.
test('disables promotion when main is active and a different experiment is reviewed', async () => {
  vi.mocked(api.getBranchStatus).mockResolvedValue({
    active_branch: 'main',
    active_kind: 'canon',
    main_head: mainHead,
    experiment_head: null,
    active_experiment_id: null,
    worktree_clean: true,
  })
  vi.mocked(api.listExperiments).mockResolvedValue({
    experiments: [
      { experiment_id: experimentID, branch_name: 'branch/obi-wan-lives-0123456789abcdef0123', head: experimentHead, display_name: 'obi-wan-lives' },
    ],
  })

  render(<BranchWorkbench project={project} appDirty={false} onDirtyChange={vi.fn()} onBranchChanged={vi.fn()} />)
  await waitFor(() => expect(screen.getByLabelText('Promote scenes/scn_0123456789abcdef0123.md')).toBeInTheDocument())

  fireEvent.click(screen.getByLabelText('Promote scenes/scn_0123456789abcdef0123.md'))
  await waitFor(() => expect(screen.getByText(/whole selected files/i)).toBeInTheDocument())

  const promoteButton = screen.getByRole('button', { name: 'Promote selected files' })
  expect(promoteButton).toBeDisabled()
  fireEvent.click(promoteButton)
  expect(api.promoteExperimentFiles).not.toHaveBeenCalled()
  expect(screen.getByText(/switch to the reviewed experiment/i)).toBeInTheDocument()
})
