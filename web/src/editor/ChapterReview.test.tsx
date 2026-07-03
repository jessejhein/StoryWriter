// BDD Scenario: 7.3.2 - Return suggestions without canon mutation
// Requirements: M7-R04, M7-R16
// Test purpose: verify findings are grouped by scene without accept controls.

import { render, screen } from '@testing-library/react'
import { expect, test } from 'vitest'
import ChapterReview from './ChapterReview'

// BDD Scenario: 7.3.2 - Return suggestions without canon mutation
// Requirements: M7-R04, M7-R16
// Test purpose: verify chapter review findings render grouped by scene without accept controls.

const findings = [
  {
    title: 'Transition loses urgency',
    explanation: 'The shift releases tension.',
    scene_ids: ['scn_0123456789abcdef0123', 'scn_aaaaaaaaaaaaaaaaaaaa'],
    follow_up_agent_ids: ['scene_rewrite'],
  },
  {
    title: 'POV drift',
    explanation: 'The second scene widens perspective unexpectedly.',
    scene_ids: ['scn_aaaaaaaaaaaaaaaaaaaa'],
    follow_up_agent_ids: [],
  },
]

// Test: renders findings grouped by stable scene ID.
// Requirements: M7-R04.
test('renders findings grouped by stable scene ID', () => {
  render(<ChapterReview findings={findings} />)
  expect(screen.getByText('scn_0123456789abcdef0123')).toBeInTheDocument()
  expect(screen.getByText('scn_aaaaaaaaaaaaaaaaaaaa')).toBeInTheDocument()
  expect(screen.getAllByText('Transition loses urgency').length).toBeGreaterThan(0)
  expect(screen.getByText('POV drift')).toBeInTheDocument()
})

// Test: does not render an accept prose control for findings.
// Requirements: M7-R16.
test('does not render an accept prose control for findings', () => {
  render(<ChapterReview findings={findings} />)
  expect(screen.queryByRole('button', { name: /accept/i })).not.toBeInTheDocument()
})
