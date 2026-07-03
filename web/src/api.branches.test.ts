// BDD Scenario: 8.2.1 - List exact changed files
// Requirements: M8-R05, M8-R06, M8-R07
// Test purpose: verify branch API helpers use the documented routes and JSON bodies.

import { beforeEach, expect, test, vi } from 'vitest'
import {
  analyzeBranchRamifications,
  createExperiment,
  discardExperiment,
  getBranchComparison,
  getBranchFileComparison,
  getBranchStatus,
  listExperiments,
  promoteExperimentFiles,
  switchBranch,
} from './api'

const fetchMock = vi.fn<(path: string | URL | Request, init?: RequestInit) => Promise<Response>>(async (...args) => {
  void args
  return new Response('{}', { status: 200, headers: { 'Content-Type': 'application/json' } })
})

const mainHead = `sha256:${'a'.repeat(64)}`
const experimentHead = `sha256:${'b'.repeat(64)}`
const fingerprint = `sha256:${'c'.repeat(64)}`

beforeEach(() => {
  fetchMock.mockClear()
  vi.stubGlobal('fetch', fetchMock)
})

// Test: uses documented branch lifecycle, comparison, analysis, promotion, and discard routes.
// Requirements: M8-R05, M8-R06, M8-R07, M8-R09, M8-R12, M8-R17.
test('uses the documented branch routes and JSON bodies', async () => {
  await getBranchStatus()
  await listExperiments()
  await createExperiment('Obi-Wan lives')
  await switchBranch({ target: 'main' })
  await switchBranch({ target: 'brn_0123456789abcdef0123', expected_head: experimentHead })
  await getBranchComparison('brn_0123456789abcdef0123')
  await getBranchFileComparison('brn_0123456789abcdef0123', 'scenes/scn_0123456789abcdef0123.md')
  await analyzeBranchRamifications('brn_0123456789abcdef0123', {
    goal: 'Explore the consequences if Obi-Wan survives.',
    profile_id: 'local_ollama',
    model: 'qwen2.5:7b',
    expected_main_head: mainHead,
    expected_experiment_head: experimentHead,
    comparison_fingerprint: fingerprint,
  })
  await promoteExperimentFiles('brn_0123456789abcdef0123', {
    paths: ['scenes/scn_0123456789abcdef0123.md'],
    expected_main_head: mainHead,
    expected_experiment_head: experimentHead,
    comparison_fingerprint: fingerprint,
  })
  await discardExperiment('brn_0123456789abcdef0123', { expected_experiment_head: experimentHead })

  expect(fetchMock.mock.calls.map(([path]) => String(path))).toEqual([
    '/api/branches/status',
    '/api/branches',
    '/api/branches',
    '/api/branches/switch',
    '/api/branches/switch',
    '/api/branches/brn_0123456789abcdef0123/comparison',
    '/api/branches/brn_0123456789abcdef0123/comparison/file?path=scenes%2Fscn_0123456789abcdef0123.md',
    '/api/branches/brn_0123456789abcdef0123/ramifications',
    '/api/branches/brn_0123456789abcdef0123/promote',
    '/api/branches/brn_0123456789abcdef0123/discard',
  ])

  const postBodies = fetchMock.mock.calls
    .filter(([, init]) => init?.method === 'POST')
    .map(([, init]) => JSON.parse(String(init?.body)))

  expect(postBodies).toEqual([
    { name: 'Obi-Wan lives' },
    { target: 'main' },
    { target: 'brn_0123456789abcdef0123', expected_head: experimentHead },
    {
      goal: 'Explore the consequences if Obi-Wan survives.',
      profile_id: 'local_ollama',
      model: 'qwen2.5:7b',
      expected_main_head: mainHead,
      expected_experiment_head: experimentHead,
      comparison_fingerprint: fingerprint,
    },
    {
      paths: ['scenes/scn_0123456789abcdef0123.md'],
      expected_main_head: mainHead,
      expected_experiment_head: experimentHead,
      comparison_fingerprint: fingerprint,
    },
    { expected_experiment_head: experimentHead },
  ])
})