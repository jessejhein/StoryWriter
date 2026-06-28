import { afterEach, expect, test, vi } from 'vitest'
import { APIError, saveScene } from './api'

// BDD trace:
// - Story: 2.3, Protect concurrent and external work.
// - Scenario: 2.3.1, stale revision leaves newer canon unchanged.
// - Requirements: M2-R06, M2-R14.
// - File purpose: verify the frontend transport preserves HTTP conflict semantics.

afterEach(() => vi.unstubAllGlobals())

// Test purpose: a scene-save conflict retains both its HTTP status and useful
// server message so editor behavior does not depend on message wording.
test('preserves status and message when a scene save conflicts', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
    ok: false,
    status: 409,
    json: async () => ({ error: 'canonical worktree is not clean' }),
  }))

  const request = saveScene('scn_00000000000000000001', {
    title: 'The Duel',
    frontmatter: { pov: 'Luke', status: 'draft', exclude_from_ai: false },
    markdown: 'Draft.\n',
    expected_revision: `sha256:${'a'.repeat(64)}`,
  })

  await expect(request).rejects.toEqual(new APIError(409, 'canonical worktree is not clean'))
})
