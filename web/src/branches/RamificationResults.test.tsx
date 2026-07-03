// BDD Scenario: 8.3.2 - Return strict findings only
// Requirements: M8-R10
// Test purpose: verify advisory ramification findings render without accept authority.

import { render, screen } from '@testing-library/react'
import { expect, test } from 'vitest'
import RamificationResults from './RamificationResults'
import type { RamificationResponse } from './branchTypes'

const response: RamificationResponse = {
  summary: 'The survival changes Luke mentorship and later confrontations.',
  findings: [
    {
      category: 'continuity',
      severity: 'high',
      title: 'Later death references conflict',
      explanation: 'Two later scenes still describe Obi-Wan as dead.',
      affected_paths: ['scenes/scn_0123456789abcdef0123.md'],
      recommended_action: 'Review the later scene before promotion.',
    },
    {
      category: 'character',
      severity: 'medium',
      title: 'Mentor tone shifts',
      explanation: 'Obi-Wan now guides Luke directly in the opening arc.',
      affected_paths: ['scenes/scn_0123456789abcdef0124.md'],
      recommended_action: 'Check mentor voice consistency.',
    },
  ],
  provider: { profile_id: 'local_ollama', type: 'ollama', model: 'qwen2.5:7b' },
  manifest: {
    main_head: `sha256:${'a'.repeat(64)}`,
    experiment_head: `sha256:${'b'.repeat(64)}`,
    fingerprint: `sha256:${'c'.repeat(64)}`,
    changed_file_count: 2,
    included_paths: ['scenes/scn_0123456789abcdef0123.md'],
    estimated_input_bytes: 420,
  },
}

// Test: summary and findings render severity, category, paths, and recommended action.
// Requirements: M8-R10.
test('renders summary and grouped findings with severity category paths and action', () => {
  render(
    <RamificationResults
      result={response}
      loading={false}
      error={null}
      stale={false}
    />,
  )

  expect(screen.getByText(response.summary)).toBeInTheDocument()
  expect(screen.getByText('Later death references conflict')).toBeInTheDocument()
  expect(screen.getByText(/continuity/i)).toBeInTheDocument()
  expect(screen.getByText(/high severity/i)).toBeInTheDocument()
  expect(screen.getByText('scenes/scn_0123456789abcdef0123.md')).toBeInTheDocument()
  expect(screen.getByText('Review the later scene before promotion.')).toBeInTheDocument()
  expect(screen.getByText('Mentor tone shifts')).toBeInTheDocument()
})

// Test: analysis is advisory and provides no accept or apply control.
// Requirements: M8-R10.
test('shows advisory notice without accept or apply controls', () => {
  render(
    <RamificationResults
      result={response}
      loading={false}
      error={null}
      stale={false}
    />,
  )

  expect(screen.getByText(/analysis does not edit files/i)).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: /accept/i })).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: /apply/i })).not.toBeInTheDocument()
})

// Test: loading, stale, and provider error states.
// Requirements: M8-R10.
test('shows loading stale and error states', () => {
  const { rerender } = render(
    <RamificationResults result={null} loading error={null} stale={false} />,
  )
  expect(screen.getByText(/analyzing ramifications/i)).toBeInTheDocument()

  rerender(
    <RamificationResults result={null} loading={false} error="Provider unavailable" stale={false} />,
  )
  expect(screen.getByRole('alert')).toHaveTextContent('Provider unavailable')

  rerender(
    <RamificationResults result={response} loading={false} error={null} stale />,
  )
  expect(screen.getByText(/stale analysis/i)).toBeInTheDocument()
})