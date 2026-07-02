import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, expect, test, vi } from 'vitest'
import App from './App'

const fetchMock = vi.fn(async (path: string | URL | Request) => {
  const url = String(path)
  if (url === '/api/health') {
    return new Response(JSON.stringify({ status: 'ok', version: '0.0.0-test' }), { status: 200, headers: { 'Content-Type': 'application/json' } })
  }
  if (url === '/api/provider-profiles') {
    return new Response(JSON.stringify({ profiles: [], revision: null }), { status: 200, headers: { 'Content-Type': 'application/json' } })
  }
  if (url === '/api/imports') {
    return new Response(JSON.stringify({ imports: [] }), { status: 200, headers: { 'Content-Type': 'application/json' } })
  }
  if (url === '/api/import-candidates') {
    return new Response(JSON.stringify({ candidates: [] }), { status: 200, headers: { 'Content-Type': 'application/json' } })
  }
  return new Response(JSON.stringify({ project_id: 'proj_story', path: '/tmp/story', git_initialized: true, index_initialized: true }), { status: 200, headers: { 'Content-Type': 'application/json' } })
})

beforeEach(() => {
  fetchMock.mockClear()
  vi.stubGlobal('fetch', fetchMock)
})

test('opens import review after project creation', async () => {
  render(<App />)
  await waitFor(() => expect(screen.getByText('Online · 0.0.0-test')).toBeInTheDocument())
  fireEvent.change(screen.getByPlaceholderText('/home/you/Stories/glass-cartographer'), { target: { value: '/tmp/story' } })
  fireEvent.change(screen.getByPlaceholderText('The Glass Cartographer'), { target: { value: 'Story' } })
  fireEvent.click(screen.getByRole('button', { name: 'Create project' }))
  await waitFor(() => expect(screen.getByRole('button', { name: 'Import Review' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Import Review' }))
  await waitFor(() => expect(screen.getByText('No imports yet.')).toBeInTheDocument())
})
