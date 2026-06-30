// BDD Scenario: 3.1.2 - Create an entry
// Requirements: M3-R09
// Test purpose: The Codex API client uses the documented create and list routes and sends the canonical create JSON body.
import { expect, test, vi } from 'vitest'
import { createCodexEntry, getCodexEntries } from './api'

// Test: loading the Codex list and creating an entry call the exact documented routes and create JSON body.
// Requirements: M3-R09
test('uses the documented list and create routes', async () => {
  const fetchMock = vi.fn()
    .mockResolvedValueOnce({ ok: true, json: async () => ({ entries: [] }) })
    .mockResolvedValueOnce({ ok: true, json: async () => ({ id: 'char_1' }) })
  vi.stubGlobal('fetch', fetchMock)

  await getCodexEntries()
  await createCodexEntry({ type: 'character', name: 'Ben', aliases: [], tags: [], description: '', metadata: {} })

  expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/codex', undefined)
  expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/codex', expect.objectContaining({ method: 'POST' }))
  expect(fetchMock.mock.calls[1]?.[1]?.body).toBe(JSON.stringify({
    type: 'character',
    name: 'Ben',
    aliases: [],
    tags: [],
    description: '',
    metadata: {},
  }))

  vi.unstubAllGlobals()
})
