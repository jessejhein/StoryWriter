// BDD Scenario: 8.3.2 - Stale ramification responses must not pollute UI
// Requirements: M8-R18
// Test purpose: Deferred analysis promises from a previous project or comparison
// context do not install findings, errors, or status messages.

import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, expect, test, vi } from 'vitest'
import type { Project, RamificationResponse } from '../api'
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

const projectA: Project = {
  project_id: 'proj_a',
  name: 'Project A',
  path: '/tmp/a',
  git_initialized: true,
  index_initialized: true,
}

const projectB: Project = {
  project_id: 'proj_b',
  name: 'Project B',
  path: '/tmp/b',
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
    branch_name: 'branch/test-exp-0123456789abcdef0123',
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

// Test: analysis started for project A does not display findings after
// rerender with project B.
test('stale analysis response from another project is ignored', async () => {
  let resolveAnalysis: (value: RamificationResponse) => void
  const analysisPromise = new Promise<RamificationResponse>((resolve) => { resolveAnalysis = resolve })
  vi.mocked(api.analyzeBranchRamifications).mockReturnValue(analysisPromise)

  vi.mocked(api.getBranchStatus).mockResolvedValue({
    active_branch: 'branch/test-exp-0123456789abcdef0123',
    active_kind: 'experiment',
    main_head: mainHead,
    experiment_head: experimentHead,
    active_experiment_id: experimentID,
    worktree_clean: true,
  })
  vi.mocked(api.listExperiments).mockResolvedValue({
    experiments: [
      { experiment_id: experimentID, branch_name: 'branch/test-exp-0123456789abcdef0123', head: experimentHead, display_name: 'test-exp' },
    ],
  })

  const { rerender } = render(
    <BranchWorkbench project={projectA} appDirty={false} onDirtyChange={vi.fn()} onBranchChanged={vi.fn()} />,
  )
  await waitFor(() => expect(screen.getByText('Ramification analysis')).toBeInTheDocument())

  fireEvent.change(screen.getByLabelText('Analysis goal'), { target: { value: 'Review' } })
  fireEvent.click(screen.getByRole('button', { name: 'Analyze ramifications' }))

  // Rerender with project B - this remounts the component due to key change
  rerender(
    <BranchWorkbench project={projectB} appDirty={false} onDirtyChange={vi.fn()} onBranchChanged={vi.fn()} />,
  )

  // Resolve project A's analysis
  resolveAnalysis!({
    summary: 'Stale summary from project A',
    findings: [],
    provider: { profile_id: 'local', type: 'ollama', model: 'qwen' },
    manifest: {
      main_head: mainHead,
      experiment_head: experimentHead,
      fingerprint,
      changed_file_count: 1,
      included_paths: ['scenes/scn_0123456789abcdef0123.md'],
      estimated_input_bytes: 120,
    },
  })

  // Project A's findings must not appear
  await waitFor(() => {
    expect(screen.queryByText('Stale summary from project A')).not.toBeInTheDocument()
  })
})

// Test: analysis error from an obsolete request does not set error state.
test('stale analysis error from obsolete request is ignored', async () => {
  let rejectAnalysis: (reason: unknown) => void
  const analysisPromise = new Promise<RamificationResponse>((_, reject) => { rejectAnalysis = reject })
  vi.mocked(api.analyzeBranchRamifications).mockReturnValue(analysisPromise)

  vi.mocked(api.getBranchStatus).mockResolvedValue({
    active_branch: 'branch/test-exp-0123456789abcdef0123',
    active_kind: 'experiment',
    main_head: mainHead,
    experiment_head: experimentHead,
    active_experiment_id: experimentID,
    worktree_clean: true,
  })
  vi.mocked(api.listExperiments).mockResolvedValue({
    experiments: [
      { experiment_id: experimentID, branch_name: 'branch/test-exp-0123456789abcdef0123', head: experimentHead, display_name: 'test-exp' },
    ],
  })

  render(
    <BranchWorkbench project={projectA} appDirty={false} onDirtyChange={vi.fn()} onBranchChanged={vi.fn()} />,
  )
  await waitFor(() => expect(screen.getByText('Ramification analysis')).toBeInTheDocument())

  fireEvent.change(screen.getByLabelText('Analysis goal'), { target: { value: 'Review' } })
  fireEvent.click(screen.getByRole('button', { name: 'Analyze ramifications' }))

  // Bump the request version by changing the goal
  fireEvent.change(screen.getByLabelText('Analysis goal'), { target: { value: 'Review the change' } })

  // Reject the first analysis request (catch to prevent unhandled rejection)
  rejectAnalysis!(new Error('Stale analysis error'))
  await analysisPromise.catch(() => {})

  // The stale error must not appear
  await waitFor(() => {
    expect(screen.queryByText('Stale analysis error')).not.toBeInTheDocument()
  })
})
