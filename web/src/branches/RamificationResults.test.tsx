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
      category: 'character',
      severity: 'high',
      title: 'Mentor tone shifts',
      explanation: 'Obi-Wan now guides Luke directly in the opening arc.',
      affected_paths: ['scenes/scn_0123456789abcdef0124.md'],
      recommended_action: 'Check mentor voice consistency.',
    },
    {
      category: 'continuity',
      severity: 'high',
      title: 'Later death references conflict',
      explanation: 'Two later scenes still describe Obi-Wan as dead.',
      affected_paths: ['scenes/scn_0123456789abcdef0123.md'],
      recommended_action: 'Review the later scene before promotion.',
    },
    {
      category: 'timeline',
      severity: 'medium',
      title: 'Timeline ripple',
      explanation: 'The survival changes the sequence of later confrontations.',
      affected_paths: ['scenes/scn_0123456789abcdef0125.md'],
      recommended_action: 'Review the downstream timeline.',
    },
    {
      category: 'plot',
      severity: 'low',
      title: 'Plot beat expands',
      explanation: 'One beat now has an extra exchange before the escape.',
      affected_paths: ['scenes/scn_0123456789abcdef0126.md'],
      recommended_action: 'Check pacing around the beat.',
    },
  ],
  provider: { profile_id: 'local_ollama', type: 'ollama', model: 'qwen2.5:7b' },
  manifest: {
    main_head: 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
    experiment_head: 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
    fingerprint: `sha256:${'c'.repeat(64)}`,
    changed_file_count: 2,
    included_paths: ['scenes/scn_0123456789abcdef0123.md'],
    estimated_input_bytes: 420,
  },
}

function isBefore(left: Element, right: Element): boolean {
  return Boolean(left.compareDocumentPosition(right) & Node.DOCUMENT_POSITION_FOLLOWING)
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
  expect(screen.getByText('High severity')).toBeInTheDocument()
  expect(screen.getByText('Medium severity')).toBeInTheDocument()
  expect(screen.getByText('Low severity')).toBeInTheDocument()
  expect(screen.getByText('Later death references conflict')).toBeInTheDocument()
  expect(screen.getAllByText(/continuity/i).length).toBeGreaterThan(0)
  expect(screen.getAllByText(/high severity/i).length).toBeGreaterThan(0)
  expect(screen.getByText('scenes/scn_0123456789abcdef0123.md')).toBeInTheDocument()
  expect(screen.getByText('Review the later scene before promotion.')).toBeInTheDocument()
  expect(screen.getByText('Mentor tone shifts')).toBeInTheDocument()
  const highSection = screen.getByText('High severity').closest('section')
  if (!highSection) {
    throw new Error('missing high severity section')
  }
  const categories = Array.from(highSection.querySelectorAll('h5'))
  expect(categories.map((heading) => heading.textContent)).toEqual(['character', 'continuity'])
  expect(isBefore(screen.getByText('High severity'), screen.getByText('Medium severity'))).toBe(true)
  expect(isBefore(screen.getByText('Medium severity'), screen.getByText('Low severity'))).toBe(true)
})

// Test: empty findings render a bounded empty-state message.
// Requirements: M8-R10.
test('renders empty findings state', () => {
  render(
    <RamificationResults
      result={{ ...response, findings: [] }}
      loading={false}
      error={null}
      stale={false}
    />,
  )

  expect(screen.getByText(/no findings were returned/i)).toBeInTheDocument()
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
