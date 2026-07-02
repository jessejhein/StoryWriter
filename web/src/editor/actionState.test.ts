import { expect, test } from 'vitest'
import {
  actionUsesBrowserStorage,
  applyInvitationRunFailure,
  applyInvitationRunSuccess,
  applyPreviewFailure,
  applyPreviewSuccess,
  applyRunFailure,
  applyRunSuccess,
  bumpActionRequestVersion,
  initialActionWorkflowState,
  invalidatePreviewForRevision,
} from './actionState'
import type { RunActionResponse } from './actionTypes'

// BDD Scenario: 7.1.1 - Preview minimal Line Polish context
// Requirements: M7-R09, M7-R17
// Test purpose: verify pure action workflow state invalidates stale previews and
// ignores stale run and invitation responses.

const previewManifest = {
  scope: 'selection' as const,
  packs_used: ['selected_text', 'style_sheet'],
  packs_omitted: [],
  estimated_input_tokens: 42,
  max_input_estimated_tokens: 8000,
  rag_mode: 'none' as const,
}

const patchResponse: RunActionResponse = {
  run_id: 'run_0123456789abcdef0123',
  status: 'pending',
  agent_id: 'line_polish',
  style_id: 'precise_editor',
  scene_id: 'scn_0123456789abcdef0123',
  scene_revision: `sha256:${'a'.repeat(64)}`,
  output_mode: 'patch',
  patch: { original: 'Alpha', replacement: 'Mock polished: Alpha' },
  context_summary: { packs_used: ['selected_text'], rag_mode: 'none' },
  provider: { profile_id: 'mock_default', type: 'openai_compatible', model: 'mock' },
}

// Test: invalidates preview when target revision changes.
// Requirements: M7-R09.
test('invalidates preview when target revision changes', () => {
  const state = applyPreviewSuccess(
    initialActionWorkflowState(),
    'selection',
    { manifest: previewManifest, target_revision: 'sha256:old' },
    1,
  )
  const next = invalidatePreviewForRevision(state, 'sha256:new')
  expect(next.preview).toBeNull()
})

// Test: ignores stale preview run and invitation responses.
// Requirements: M7-R17.
test('ignores stale preview run and invitation responses', () => {
  const bumped = bumpActionRequestVersion(initialActionWorkflowState())
  const previewState = applyPreviewSuccess(
    bumped,
    'selection',
    { manifest: previewManifest, target_revision: 'sha256:current' },
    bumped.requestVersion,
  )
  expect(applyPreviewSuccess(
    previewState,
    'selection',
    { manifest: previewManifest, target_revision: 'sha256:stale' },
    0,
  ).preview?.targetRevision).toBe('sha256:current')

  const runState = applyRunSuccess(previewState, patchResponse, previewState.requestVersion)
  expect(applyRunFailure(runState, 'stale run', 0).runError).toBeNull()
  expect(applyInvitationRunSuccess(runState, 'invite_0123456789abcdef0123', patchResponse, 0).invitationRun).toBeNull()
  expect(applyInvitationRunFailure(runState, 'stale invitation', 0).invitationError).toBeNull()
})

// Test: never writes action data to browser storage.
// Requirements: M7-R17.
test('never writes action data to browser storage', () => {
  expect(actionUsesBrowserStorage()).toBe(false)
})