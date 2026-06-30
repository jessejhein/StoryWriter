// BDD Scenario: 3.5.3 - Inspect active state
// Requirements: M3-R07, M3-R10, M3-R11, M3-R21
// Test purpose: The workbench saves ordered progressions and renders resolved active state through the real fetch boundary.
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

// Test: progression saves use the real fetch boundary, send canonical bodies for first-save and replacement-save flows, and render resolved active-state details.
// Requirements: M3-R07, M3-R10, M3-R11, M3-R21
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
      return {
        ok: true,
        json: async () => ({
          entry_id: 'char_0123456789abcdef0123',
          progressions: [{
            id: 'prog_0123456789abcdef0001',
            anchor: { type: 'scene', id: 'scn_0123456789abcdef0124', timing: 'before' },
            changes: { description: 'Watching.', metadata: { role: 'strategist' } },
          }],
          revision: 'sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaab',
        }),
      }
    }
    if (path === '/api/codex/char_0123456789abcdef0123/progressions' && init?.method === 'PUT') {
      return {
        ok: true,
        json: async () => ({
          entry_id: 'char_0123456789abcdef0123',
          progressions: init?.body === JSON.stringify({
            progressions: [{
              id: 'prog_0123456789abcdef0001',
              anchor: { type: 'scene', id: 'scn_0123456789abcdef0124', timing: 'before' },
              changes: { description: 'Watching.', metadata: { role: 'strategist' } },
            }, {
              anchor: { type: 'scene', id: 'scn_0123456789abcdef0123', timing: 'after' },
              changes: { description: 'Gone.', metadata: { status: 'deceased' } },
            }],
            expected_revision: 'sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaab',
          }) ? [{
            id: 'prog_0123456789abcdef0001',
            anchor: { type: 'scene', id: 'scn_0123456789abcdef0124', timing: 'before' },
            changes: { description: 'Watching.', metadata: { role: 'strategist' } },
          }, {
            id: 'prog_0123456789abcdef0123',
            anchor: { type: 'scene', id: 'scn_0123456789abcdef0123', timing: 'after' },
            changes: { description: 'Gone.', metadata: { status: 'deceased' } },
          }] : [{
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
          applied_progression_ids: ['prog_0123456789abcdef0001', 'prog_0123456789abcdef0123'],
        }),
      }
    }
    return { ok: false, status: 404, json: async () => ({ error: 'not found' }) }
  }))

  render(<CodexWorkbench project={project} />)

  await waitFor(() => expect(screen.getByRole('button', { name: 'Obi-Wan Kenobi' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Obi-Wan Kenobi' }))
  await waitFor(() => expect(screen.getByText('role')).toBeInTheDocument())
  expect(screen.getByText('mentor')).toBeInTheDocument()

  fireEvent.click(screen.getByRole('button', { name: 'Add progression' }))
  fireEvent.change(screen.getAllByLabelText('Progression description')[1]!, { target: { value: 'Gone.' } })
  fireEvent.click(screen.getAllByRole('button', { name: 'Add progression metadata' })[1]!)
  fireEvent.change(screen.getAllByLabelText('Progression metadata key 1')[1]!, { target: { value: 'status' } })
  fireEvent.change(screen.getAllByLabelText('Progression metadata value 1')[1]!, { target: { value: 'deceased' } })
  fireEvent.click(screen.getByRole('button', { name: 'Move progression 2 up' }))
  fireEvent.click(screen.getByRole('button', { name: 'Move progression 1 down' }))
  fireEvent.click(screen.getByRole('button', { name: 'Save progressions' }))

  await waitFor(() => expect(requests.some((request) => request.path === '/api/codex/char_0123456789abcdef0123/progressions' && request.init?.method === 'PUT')).toBe(true))
  const saveRequest = requests.find((request) => request.path === '/api/codex/char_0123456789abcdef0123/progressions' && request.init?.method === 'PUT')
  expect(saveRequest?.init?.body).toBe(JSON.stringify({
    progressions: [{
      id: 'prog_0123456789abcdef0001',
      anchor: { type: 'scene', id: 'scn_0123456789abcdef0124', timing: 'before' },
      changes: { description: 'Watching.', metadata: { role: 'strategist' } },
    }, {
      anchor: { type: 'scene', id: 'scn_0123456789abcdef0123', timing: 'after' },
      changes: { description: 'Gone.', metadata: { status: 'deceased' } },
    }],
    expected_revision: 'sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaab',
  }))
  await waitFor(() => expect(screen.getByText('Applied progressions: prog_0123456789abcdef0001, prog_0123456789abcdef0123')).toBeInTheDocument())

  vi.unstubAllGlobals()
})
