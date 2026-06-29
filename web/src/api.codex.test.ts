// BDD Scenario: 3.1.2 - Create an entry
// Requirements: M3-R09
// Test purpose: Plain-English description of the frontend Codex API client request shapes and error mapping.
import { expect, test, vi } from 'vitest'
import { APIError, createCodexEntry, getCodexActiveState, getCodexEntries, getCodexProgressions, saveCodexProgressions, updateCodexEntry } from './api'

test('codex api functions use the documented routes and JSON bodies', async () => {
  const fetchMock = vi.fn()
    .mockResolvedValueOnce({ ok: true, json: async () => ({ entries: [] }) })
    .mockResolvedValueOnce({ ok: true, json: async () => ({ id: 'char_1' }) })
    .mockResolvedValueOnce({ ok: true, json: async () => ({ id: 'char_1' }) })
    .mockResolvedValueOnce({ ok: true, json: async () => ({ entry_id: 'char_1', progressions: [], revision: null }) })
    .mockResolvedValueOnce({ ok: true, json: async () => ({ entry_id: 'char_1', progressions: [], revision: null }) })
    .mockResolvedValueOnce({ ok: false, json: async () => ({ error: 'conflict' }), status: 409 })
  vi.stubGlobal('fetch', fetchMock)

  // Test: each Codex client helper calls the exact milestone routes with JSON request bodies when required.
  // Requirements: M3-R09
  await getCodexEntries()
  await createCodexEntry({ type: 'character', name: 'Ben', aliases: [], tags: [], description: '', metadata: {} })
  await updateCodexEntry('char_1', { name: 'Ben', aliases: [], tags: [], description: '', metadata: {}, expected_revision: 'sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa' })
  await getCodexProgressions('char_1')
  await saveCodexProgressions('char_1', { progressions: [], expected_revision: null })
  await expect(getCodexActiveState('char_1', 'scn_1')).rejects.toBeInstanceOf(APIError)

  expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/codex', undefined)
  expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/codex', expect.objectContaining({ method: 'POST' }))
  expect(fetchMock).toHaveBeenNthCalledWith(3, '/api/codex/char_1', expect.objectContaining({ method: 'PUT' }))
  expect(fetchMock).toHaveBeenNthCalledWith(4, '/api/codex/char_1/progressions', undefined)
  expect(fetchMock).toHaveBeenNthCalledWith(5, '/api/codex/char_1/progressions', expect.objectContaining({ method: 'PUT' }))
  expect(fetchMock).toHaveBeenNthCalledWith(6, '/api/codex/char_1/active?scene_id=scn_1', undefined)

  vi.unstubAllGlobals()
})
