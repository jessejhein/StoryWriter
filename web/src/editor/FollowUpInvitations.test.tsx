// BDD Scenario: 7.4.1 - Offer a follow-up without calling a provider
// Requirements: M7-R11, M7-R17
// Test purpose: verify invitation provenance and explicit execution controls.

import { fireEvent, render, screen } from '@testing-library/react'
import { expect, test, vi } from 'vitest'
import FollowUpInvitations from './FollowUpInvitations'

// BDD Scenario: 7.4.1 - Offer a follow-up without calling a provider
// Requirements: M7-R11, M7-R17
// Test purpose: verify invitations state that no model has run and only execute on explicit Run.

const invitations = [{
  invitation_id: 'invite_0123456789abcdef0123',
  parent_run_id: 'run_aaaaaaaaaaaaaaaaaaaa',
  root_run_id: 'run_aaaaaaaaaaaaaaaaaaaa',
  chain_depth: 2,
  agent_id: 'scene_rewrite',
  scope: 'scene' as const,
  scene_id: 'scn_0123456789abcdef0123',
  relationship: 'triggered' as const,
}]

// Test: states that an invitation has not called a model.
// Requirements: M7-R11.
test('states that an invitation has not called a model', () => {
  render(<FollowUpInvitations invitations={invitations} onRun={() => {}} />)
  expect(screen.getByText(/have not called a model yet/i)).toBeInTheDocument()
})

// Test: does not call run while merely displaying invitation.
// Requirements: M7-R11.
test('does not call run while merely displaying invitation', () => {
  const onRun = vi.fn()
  render(<FollowUpInvitations invitations={invitations} onRun={onRun} />)
  expect(onRun).not.toHaveBeenCalled()
})

// Test: runs suggested action only after explicit confirmation.
// Requirements: M7-R12.
test('runs suggested action only after explicit confirmation', () => {
  const onRun = vi.fn()
  render(<FollowUpInvitations invitations={invitations} onRun={onRun} />)
  fireEvent.click(screen.getByRole('button', { name: 'Run suggested action' }))
  expect(onRun).toHaveBeenCalledWith(invitations[0])
})

// Test: renders consumed conflict and retry states accessibly.
// Requirements: M7-R17.
test('renders consumed conflict and retry states accessibly', () => {
  render(<FollowUpInvitations invitations={invitations} error="Invitation already consumed." onRun={() => {}} />)
  expect(screen.getByRole('alert')).toHaveTextContent('Invitation already consumed.')
})
