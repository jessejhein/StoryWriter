// BDD Scenario: 3.2.1 - Save ordered progressions
// Requirements: M3-R09
// Test purpose: The Codex API client uses the documented progression load/save routes and preserves nullable expected_revision values.
import { expect, test, vi } from 'vitest'
import { getCodexProgressions, saveCodexProgressions } from './api'

// Test: progression load and save use the exact milestone route and send the canonical ordered progressions payload.
// Requirements: M3-R09
test('uses the documented progression routes and body', async () => {
  const fetchMock = vi.fn()
    .mockResolvedValueOnce({ ok: true, json: async () => ({ entry_id: 'char_1', progressions: [], revision: null }) })
    .mockResolvedValueOnce({ ok: true, json: async () => ({ entry_id: 'char_1', progressions: [], revision: null }) })
  vi.stubGlobal('fetch', fetchMock)

  await getCodexProgressions('char_1')
  await saveCodexProgressions('char_1', { progressions: [], expected_revision: null })

  expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/codex/char_1/progressions', undefined)
  expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/codex/char_1/progressions', expect.objectContaining({ method: 'PUT' }))
  expect(fetchMock.mock.calls[1]?.[1]?.body).toBe(JSON.stringify({
    progressions: [],
    expected_revision: null,
  }))

  vi.unstubAllGlobals()
})
