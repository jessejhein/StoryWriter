// BDD Scenario: 3.3.1 - Resolve before and after an anchor
// Requirements: M3-R09
// Test purpose: The Codex API client calls the documented active-state route and surfaces HTTP failures as API errors.
import { expect, test, vi } from 'vitest'
import { APIError, getCodexActiveState } from './api'

// Test: active-state requests use the documented route query and preserve non-OK responses as APIError failures.
// Requirements: M3-R09
test('uses the documented active-state route and surfaces failures', async () => {
  const fetchMock = vi.fn().mockResolvedValue({ ok: false, json: async () => ({ error: 'conflict' }), status: 409 })
  vi.stubGlobal('fetch', fetchMock)

  await expect(getCodexActiveState('char_1', 'scn_1')).rejects.toBeInstanceOf(APIError)
  expect(fetchMock).toHaveBeenCalledWith('/api/codex/char_1/active?scene_id=scn_1', undefined)

  vi.unstubAllGlobals()
})
