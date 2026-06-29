// BDD Scenario: 3.5.1 - Edit through explicit UI states
// Requirements: M3-R10, M3-R11, M3-R21
// Test purpose: Entry saves expose saving, saved, conflict, and actionable error states through the real fetch boundary while preserving failed drafts.
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { expect, test, vi } from 'vitest'
import type { CodexEntry, Project } from '../api'
import CodexWorkbench from './CodexWorkbench'

const project: Project = {
  project_id: 'proj_test_novel',
  name: 'Test Novel',
  path: '/tmp/test-novel',
  git_initialized: true,
  index_initialized: true,
}

const savedEntry: CodexEntry = {
  id: 'char_0123456789abcdef0123',
  type: 'character',
  name: 'Ben Kenobi',
  aliases: [],
  tags: [],
  description: '',
  metadata: {},
  revision: 'sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
}

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((complete) => {
    resolve = complete
  })
  return { promise, resolve }
}

test('shows saving and saved around an explicit entry save', async () => {
  const createResponse = deferred<{ ok: boolean; status: number; json: () => Promise<CodexEntry> }>()
  vi.stubGlobal('fetch', vi.fn((input: string | URL | Request, init?: RequestInit) => {
    const path = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url
    if (path === '/api/codex' && init?.method === 'POST') {
      return createResponse.promise
    }
    if (path === '/api/codex') {
      return Promise.resolve({ ok: true, status: 200, json: async () => ({ entries: [] }) })
    }
    if (path === '/api/outline') {
      return Promise.resolve({ ok: true, status: 200, json: async () => ({ version: 1, arcs: [] }) })
    }
    if (path === `/api/codex/${savedEntry.id}/progressions`) {
      return Promise.resolve({ ok: true, status: 200, json: async () => ({ entry_id: savedEntry.id, progressions: [], revision: null }) })
    }
    return Promise.resolve({ ok: false, status: 404, json: async () => ({ error: 'not found' }) })
  }))

  // Test: an explicit save shows Saving until the server responds, then adopts the canonical response and shows Saved.
  // Requirements: M3-R10, M3-R11
  render(<CodexWorkbench project={project} />)
  await waitFor(() => expect(screen.getByText('No Codex entries yet.')).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'New entry' }))
  fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'Ben Kenobi' } })
  fireEvent.click(screen.getByRole('button', { name: 'Save entry' }))
  expect(screen.getByText('Saving')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Saving…' })).toBeDisabled()

  createResponse.resolve({ ok: true, status: 201, json: async () => savedEntry })
  await waitFor(() => expect(screen.getAllByText('Saved')).toHaveLength(2))
  expect(screen.getByLabelText('Name')).toHaveValue('Ben Kenobi')
})

test('preserves the draft and shows conflict details when a save fails', async () => {
  vi.stubGlobal('fetch', vi.fn(async (input: string | URL | Request, init?: RequestInit) => {
    const path = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url
    if (path === '/api/codex' && init?.method === 'POST') {
      return { ok: false, status: 409, json: async () => ({ error: 'worktree is dirty; commit or discard external changes' }) }
    }
    if (path === '/api/codex') {
      return { ok: true, status: 200, json: async () => ({ entries: [] }) }
    }
    if (path === '/api/outline') {
      return { ok: true, status: 200, json: async () => ({ version: 1, arcs: [] }) }
    }
    return { ok: false, status: 404, json: async () => ({ error: 'not found' }) }
  }))

  // Test: a 409 save failure keeps the author's draft and presents both the conflict state and actionable server error.
  // Requirements: M3-R11
  render(<CodexWorkbench project={project} />)
  await waitFor(() => expect(screen.getByText('No Codex entries yet.')).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'New entry' }))
  fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'Unsaved Ben' } })
  fireEvent.click(screen.getByRole('button', { name: 'Save entry' }))

  await waitFor(() => expect(screen.getByText('Conflict')).toBeInTheDocument())
  expect(screen.getByText('worktree is dirty; commit or discard external changes')).toBeInTheDocument()
  expect(screen.getByLabelText('Name')).toHaveValue('Unsaved Ben')
})
