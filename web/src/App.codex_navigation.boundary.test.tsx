// BDD Scenario: 3.5.2 - Confirm destructive navigation
// Requirements: M3-R11, M3-R12, M3-R21
// Test purpose: Confirmed navigation discards a dirty Codex draft through the application fetch boundary.
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { expect, test, vi } from 'vitest'
import App from './App'

test('leaves the codex after the author confirms discarding a dirty draft', async () => {
  vi.stubGlobal('fetch', vi.fn(async (input: string | URL | Request, init?: RequestInit) => {
    const path = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url
    if (path === '/api/health') {
      return { ok: true, status: 200, json: async () => ({ status: 'ok', version: '0.0.0-test' }) }
    }
    if (path === '/api/projects' && init?.method === 'POST') {
      return { ok: true, status: 201, json: async () => ({ project_id: 'proj_test', name: 'Test Novel', path: '/tmp/test-novel', git_initialized: true, index_initialized: true }) }
    }
    if (path === '/api/codex') {
      return { ok: true, status: 200, json: async () => ({ entries: [] }) }
    }
    if (path === '/api/outline') {
      return { ok: true, status: 200, json: async () => ({ version: 1, arcs: [] }) }
    }
    return { ok: false, status: 404, json: async () => ({ error: 'not found' }) }
  }))
  const confirm = vi.spyOn(window, 'confirm').mockReturnValue(true)

  // Test: confirming destructive navigation leaves the dirty Codex form and opens the requested workspace.
  // Requirements: M3-R12
  render(<App />)
  await waitFor(() => expect(screen.getByText('Online · 0.0.0-test')).toBeInTheDocument())
  fireEvent.change(screen.getByPlaceholderText('The Glass Cartographer'), { target: { value: 'Test Novel' } })
  fireEvent.change(screen.getByPlaceholderText('/home/you/Stories/glass-cartographer'), { target: { value: '/tmp/test-novel' } })
  fireEvent.click(screen.getByRole('button', { name: 'Create project' }))
  await waitFor(() => expect(screen.getByRole('button', { name: 'Codex' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Codex' }))
  await waitFor(() => expect(screen.getByRole('button', { name: 'New entry' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'New entry' }))
  fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'Ben' } })
  fireEvent.click(screen.getByRole('button', { name: 'Outline' }))

  expect(confirm).toHaveBeenCalled()
  await waitFor(() => expect(screen.getByText('No arcs yet')).toBeInTheDocument())
  expect(screen.queryByLabelText('Name')).not.toBeInTheDocument()
})
