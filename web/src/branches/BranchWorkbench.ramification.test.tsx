// BDD Scenario: 8.3.1 - Run only after explicit authorization
// Requirements: M8-R09, M8-R10
// Test purpose: verify explicit ramification analysis workflow and clearing behavior.

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
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
    base_head: `sha256:${'d'.repeat(64)}`,
    fingerprint,
    files: [{ path: 'scenes/scn_0123456789abcdef0123.md', status: 'modified' }],
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

// Test: explicit Analyze is the only provider-triggering UI action.
// Requirements: M8-R09.
test('calls ramification analysis only after explicit analyze authorization', async () => {
  render(<BranchWorkbench project={project} appDirty={false} onDirtyChange={vi.fn()} onBranchChanged={vi.fn()} />)
  await waitFor(() => expect(api.getBranchComparison).toHaveBeenCalled())
  expect(api.analyzeBranchRamifications).not.toHaveBeenCalled()

  fireEvent.change(screen.getByLabelText('Analysis goal'), { target: { value: 'Explore consequences.' } })
  fireEvent.click(screen.getByRole('button', { name: 'Analyze ramifications' }))
  await waitFor(() => expect(api.analyzeBranchRamifications).toHaveBeenCalledWith(experimentID, {
    goal: 'Explore consequences.',
    profile_id: 'local_ollama',
    model: '',
    expected_main_head: mainHead,
    expected_experiment_head: experimentHead,
    comparison_fingerprint: fingerprint,
  }))
})

// Test: goal, profile, model, and reviewed fingerprint are sent with the request.
// Requirements: M8-R09.
test('sends reviewed fingerprint and provider selection with the analysis request', async () => {
  vi.mocked(api.analyzeBranchRamifications).mockResolvedValue({
    summary: 'Summary',
    findings: [],
    provider: { profile_id: 'local_ollama', type: 'ollama', model: 'qwen2.5:7b' },
    manifest: {
      main_head: mainHead,
      experiment_head: experimentHead,
      fingerprint,
      changed_file_count: 1,
      included_paths: ['scenes/scn_0123456789abcdef0123.md'],
      estimated_input_bytes: 100,
    },
  })

  render(<BranchWorkbench project={project} appDirty={false} onDirtyChange={vi.fn()} onBranchChanged={vi.fn()} />)
  await waitFor(() => expect(screen.getByLabelText('Provider profile')).toBeInTheDocument())

  fireEvent.change(screen.getByLabelText('Analysis goal'), { target: { value: 'Goal text' } })
  fireEvent.change(screen.getByLabelText('Model'), { target: { value: 'qwen2.5:7b' } })
  await waitFor(() => expect(screen.getByRole('button', { name: 'Analyze ramifications' })).not.toBeDisabled())
  fireEvent.click(screen.getByRole('button', { name: 'Analyze ramifications' }))

  await waitFor(() => expect(api.analyzeBranchRamifications).toHaveBeenCalledWith(experimentID, expect.objectContaining({
    goal: 'Goal text',
    profile_id: 'local_ollama',
    model: 'qwen2.5:7b',
    comparison_fingerprint: fingerprint,
  })))
  await waitFor(() => expect(screen.getByText('Summary', { selector: '.branch-ramification-summary' })).toBeInTheDocument())
})

// Test: findings clear after fingerprint change and are not browser-persisted.
// Requirements: M8-R10, M8-R18.
test('clears findings after comparison fingerprint changes', async () => {
  vi.mocked(api.analyzeBranchRamifications).mockResolvedValue({
    summary: 'Visible summary',
    findings: [{
      category: 'continuity',
      severity: 'high',
      title: 'Conflict',
      explanation: 'Explanation',
      affected_paths: ['scenes/scn_0123456789abcdef0123.md'],
      recommended_action: 'Review scene.',
    }],
    provider: { profile_id: 'local_ollama', type: 'ollama', model: 'qwen2.5:7b' },
    manifest: {
      main_head: mainHead,
      experiment_head: experimentHead,
      fingerprint,
      changed_file_count: 1,
      included_paths: ['scenes/scn_0123456789abcdef0123.md'],
      estimated_input_bytes: 100,
    },
  })

  render(<BranchWorkbench project={project} appDirty={false} onDirtyChange={vi.fn()} onBranchChanged={vi.fn()} />)
  await waitFor(() => expect(screen.getByLabelText('Analysis goal')).toBeInTheDocument())
  fireEvent.change(screen.getByLabelText('Analysis goal'), { target: { value: 'Goal' } })
  await waitFor(() => expect(screen.getByRole('button', { name: 'Analyze ramifications' })).not.toBeDisabled())
  fireEvent.click(screen.getByRole('button', { name: 'Analyze ramifications' }))
  await waitFor(() => expect(screen.getByText('Visible summary', { selector: '.branch-ramification-summary' })).toBeInTheDocument())

  vi.mocked(api.getBranchComparison).mockResolvedValue({
    experiment_id: experimentID,
    branch_name: 'branch/obi-wan-lives-0123456789abcdef0123',
    main_head: mainHead,
    experiment_head: experimentHead,
    base_head: `sha256:${'d'.repeat(64)}`,
    fingerprint: `sha256:${'e'.repeat(64)}`,
    files: [{ path: 'scenes/scn_0123456789abcdef0123.md', status: 'modified' }],
  })
  cleanup()
  render(<BranchWorkbench key="refreshed" project={project} appDirty={false} onDirtyChange={vi.fn()} onBranchChanged={vi.fn()} />)
  await waitFor(() => expect(screen.queryByText('Visible summary', { selector: '.branch-ramification-summary' })).not.toBeInTheDocument())
  expect(localStorage.getItem('branch-ramification')).toBeNull()
  expect(sessionStorage.getItem('branch-ramification')).toBeNull()
})