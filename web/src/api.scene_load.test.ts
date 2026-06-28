import { afterEach, expect, test, vi } from 'vitest'
import { getScene, type SceneDocument } from './api'

// BDD trace:
// - Story: 2.1, Open an existing scene by stable ID.
// - Scenario: 2.1.1, load a valid scene.
// - Requirements: M2-R01, M2-R02.
// - File purpose: verify the frontend scene-load transport contract.

afterEach(() => vi.unstubAllGlobals())

// Test purpose: loading requests the stable scene route and returns every
// canonical editor field without client-side remapping or loss.
test('loads the documented scene representation by stable id', async () => {
  const response: SceneDocument = {
    id: 'scn_00000000000000000001',
    chapter_id: 'ch_00000000000000000001',
    title: 'The Duel',
    frontmatter: { pov: 'Luke', status: 'draft', exclude_from_ai: false },
    markdown: 'Scene prose.\n',
    revision: `sha256:${'a'.repeat(64)}`,
  }
  const fetchMock = vi.fn().mockResolvedValue({
    ok: true,
    status: 200,
    json: async () => response,
  })
  vi.stubGlobal('fetch', fetchMock)

  await expect(getScene(response.id)).resolves.toEqual(response)
  expect(fetchMock).toHaveBeenCalledWith(`/api/scenes/${response.id}`, undefined)
})
