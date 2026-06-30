// BDD Scenario: 3.1.3 - Edit an entry
// Requirements: M3-R09
// Test purpose: The Codex API client sends the documented entry-update route and JSON body without a type field.
import { expect, test, vi } from 'vitest'
import { updateCodexEntry } from './api'

// Test: updating an entry uses PUT on the stable entry route and sends the canonical mutable fields plus expected_revision.
// Requirements: M3-R09
test('uses the documented entry update route and body', async () => {
  const fetchMock = vi.fn().mockResolvedValue({ ok: true, json: async () => ({ id: 'char_1' }) })
  vi.stubGlobal('fetch', fetchMock)

  await updateCodexEntry('char_1', {
    name: 'Ben',
    aliases: [],
    tags: [],
    description: '',
    metadata: {},
    expected_revision: 'sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
  })

  expect(fetchMock).toHaveBeenCalledWith('/api/codex/char_1', expect.objectContaining({ method: 'PUT' }))
  expect(fetchMock.mock.calls[0]?.[1]?.body).toBe(JSON.stringify({
    name: 'Ben',
    aliases: [],
    tags: [],
    description: '',
    metadata: {},
    expected_revision: 'sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
  }))

  vi.unstubAllGlobals()
})
