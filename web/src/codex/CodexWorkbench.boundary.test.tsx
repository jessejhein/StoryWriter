// BDD Scenario: 3.5.3 - Inspect active state
// Requirements: M3-R07, M3-R10, M3-R11, M3-R21
// Test purpose: Plain-English description of the Codex workbench flow through a fetch boundary, proving progression metadata save requests and resolved active-state metadata rendering without mocking the workbench API module.
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { expect, test, vi } from 'vitest'
import type { Project } from '../api'
import CodexWorkbench from './CodexWorkbench'

const project: Project = {
  project_id: 'proj_test_novel',
  name: 'Test Novel',
  path: '/tmp/test-novel',
  git_initialized: true,
  index_initialized: true,
}

test('saves progression metadata and renders active-state metadata through fetch', async () => {
  const requests: Array<{ path: string; init?: RequestInit }> = []
  vi.stubGlobal('fetch', vi.fn(async (input: string | URL | Request, init?: RequestInit) => {
    const path = typeof input === 'string' ? input : input instanceof URL ? input.pathname + input.search : input.url
    requests.push({ path, init })

    if (path === '/api/codex') {
      return { ok: true, json: async () => ({ entries: [{ id: 'char_0123456789abcdef0123', type: 'character', name: 'Obi-Wan Kenobi', aliases: ['Ben'], tags: ['mentor'], description: 'Guide.', metadata: { status: 'alive' }, revision: 'sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa' }] }) }
    }
    if (path === '/api/outline') {
      return {
        ok: true,
        json: async () => ({
          version: 1,
          arcs: [{
            id: 'arc_1',
            title: 'Act One',
            display_label: 'Arc 1',
            chapters: [{
              id: 'ch_1',
              title: 'Arrival',
              display_label: 'Chapter 1.1',
              scenes: [
                { id: 'scn_0123456789abcdef0123', title: 'Docking', display_label: 'Scene 1.1.1' },
                { id: 'scn_0123456789abcdef0124', title: 'Debrief', display_label: 'Scene 1.1.2' },
              ],
            }],
          }],
        }),
      }
    }
    if (path === '/api/codex/char_0123456789abcdef0123') {
      return { ok: true, json: async () => ({ id: 'char_0123456789abcdef0123', type: 'character', name: 'Obi-Wan Kenobi', aliases: ['Ben'], tags: ['mentor'], description: 'Guide.', metadata: { status: 'alive' }, revision: 'sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa' }) }
    }
    if (path === '/api/codex/char_0123456789abcdef0123/progressions' && (!init || init.method === 'GET')) {
      return { ok: true, json: async () => ({ entry_id: 'char_0123456789abcdef0123', progressions: [], revision: null }) }
    }
    if (path === '/api/codex/char_0123456789abcdef0123/progressions' && init?.method === 'PUT') {
      return {
        ok: true,
        json: async () => ({
          entry_id: 'char_0123456789abcdef0123',
          progressions: [{
            id: 'prog_0123456789abcdef0123',
            anchor: { type: 'scene', id: 'scn_0123456789abcdef0123', timing: 'after' },
            changes: { description: 'Gone.', metadata: { status: 'deceased' } },
          }],
          revision: 'sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
        }),
      }
    }
    if (path === '/api/codex/char_0123456789abcdef0123/active?scene_id=scn_0123456789abcdef0123') {
      return {
        ok: true,
        json: async () => ({
          scene_id: 'scn_0123456789abcdef0123',
          entry: {
            id: 'char_0123456789abcdef0123',
            type: 'character',
            name: 'Obi-Wan Kenobi',
            aliases: ['Ben'],
            tags: ['mentor'],
            description: 'Guide.',
            metadata: { role: 'mentor', status: 'alive' },
          },
          applied_progression_ids: [],
        }),
      }
    }
    return { ok: false, status: 404, json: async () => ({ error: 'not found' }) }
  }))

  // Test: the workbench uses real fetch boundaries, sends progression metadata in the save request, and renders resolved metadata from the active-state response.
  // Requirements: M3-R21
  render(<CodexWorkbench project={project} />)

  await waitFor(() => expect(screen.getByRole('button', { name: 'Obi-Wan Kenobi' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Obi-Wan Kenobi' }))
  await waitFor(() => expect(screen.getByText('role')).toBeInTheDocument())
  expect(screen.getByText('mentor')).toBeInTheDocument()

  fireEvent.click(screen.getByRole('button', { name: 'Add progression' }))
  fireEvent.change(screen.getByLabelText('Progression description'), { target: { value: 'Gone.' } })
  fireEvent.click(screen.getByRole('button', { name: 'Add progression metadata' }))
  fireEvent.change(screen.getByLabelText('Progression metadata key 1'), { target: { value: 'status' } })
  fireEvent.change(screen.getByLabelText('Progression metadata value 1'), { target: { value: 'deceased' } })
  fireEvent.click(screen.getByRole('button', { name: 'Save progressions' }))

  await waitFor(() => expect(requests.some((request) => request.path === '/api/codex/char_0123456789abcdef0123/progressions' && request.init?.method === 'PUT')).toBe(true))
  const saveRequest = requests.find((request) => request.path === '/api/codex/char_0123456789abcdef0123/progressions' && request.init?.method === 'PUT')
  expect(saveRequest?.init?.body).toBe(JSON.stringify({
    progressions: [{
      anchor: { type: 'scene', id: 'scn_0123456789abcdef0123', timing: 'after' },
      changes: { description: 'Gone.', metadata: { status: 'deceased' } },
    }],
    expected_revision: null,
  }))

  vi.unstubAllGlobals()
})
