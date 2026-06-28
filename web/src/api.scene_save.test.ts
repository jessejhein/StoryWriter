import { afterEach, expect, test, vi } from 'vitest'
import { saveScene, type SceneDocument } from './api'

// BDD trace:
// - Story: 2.2, Save an explicit scene edit.
// - Scenario: 2.2.1, save valid edits.
// - Requirements: M2-R05, M2-R07.
// - File purpose: verify the frontend scene-save transport contract.

afterEach(() => vi.unstubAllGlobals())

// Test purpose: saving sends PUT to the stable scene route with the exact
// snake_case body and returns the canonical response unchanged.
test('sends and returns the documented scene save representation', async () => {
  const response: SceneDocument = {
    id: 'scn_00000000000000000001',
    chapter_id: 'ch_00000000000000000001',
    title: 'The Duel',
    frontmatter: { pov: 'Luke', status: 'revised', exclude_from_ai: false },
    markdown: 'Revised.\n',
    revision: `sha256:${'b'.repeat(64)}`,
  }
  const fetchMock = vi.fn().mockResolvedValue({
    ok: true,
    status: 200,
    json: async () => response,
  })
  vi.stubGlobal('fetch', fetchMock)
  const requestBody = {
    title: 'The Duel',
    frontmatter: { pov: 'Luke', status: 'revised' as const, exclude_from_ai: false },
    markdown: 'Revised.\n',
    expected_revision: `sha256:${'a'.repeat(64)}`,
  }

  await expect(saveScene(response.id, requestBody)).resolves.toEqual(response)
  expect(fetchMock).toHaveBeenCalledWith(`/api/scenes/${response.id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(requestBody),
  })
})
