import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, expect, test, vi } from 'vitest'
import App from './App'

const fetchMock = vi.fn(async (path: string | URL | Request, init?: RequestInit) => {
  const url = String(path)
  const body = url === '/api/health'
    ? { status: 'ok', version: '0.0.0-test' }
    : init?.method === 'PUT'
      ? { profiles: [{ id: 'local_ollama', name: 'Local Ollama', type: 'ollama', base_url: 'http://127.0.0.1:11434', auth: { type: 'none', credential_env: '' }, capabilities: { chat: true, streaming: false, structured_output: false, max_context_tokens: 8192 }, readiness: 'ready' }], revision: 'sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa' }
      : { profiles: [], revision: null }
  return new Response(JSON.stringify(body), { status: 200, headers: { 'Content-Type': 'application/json' } })
})

beforeEach(() => {
  fetchMock.mockClear()
  vi.stubGlobal('fetch', fetchMock)
})

test('opens provider settings before project selection and confirms dirty navigation', async () => {
  const { unmount } = render(<App />)

  await waitFor(() => expect(screen.getByText('Online · 0.0.0-test')).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Provider settings' }))
  await waitFor(() => expect(screen.getByText('No provider profiles are configured yet.')).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Add profile' }))
  fireEvent.change(screen.getByLabelText('Profile ID 1'), { target: { value: 'local_ollama' } })
  fireEvent.change(screen.getByLabelText('Profile name 1'), { target: { value: 'Local Ollama' } })
  await waitFor(() => {
    const beforeUnload = new Event('beforeunload', { cancelable: true })
    window.dispatchEvent(beforeUnload)
    expect(beforeUnload.defaultPrevented).toBe(true)
  })

  fireEvent.click(screen.getByRole('button', { name: 'Project setup' }))
  expect(screen.getByLabelText('Profile ID 1')).toHaveValue('local_ollama')
  expect(screen.getByText('Configure provider profiles without putting secrets in projects.')).toBeInTheDocument()

  fireEvent.click(screen.getByRole('button', { name: 'Save' }))
  await waitFor(() => expect(screen.getByText('Saved')).toBeInTheDocument())
  await waitFor(() => {
    const afterSaveBeforeUnload = new Event('beforeunload', { cancelable: true })
    window.dispatchEvent(afterSaveBeforeUnload)
    expect(afterSaveBeforeUnload.defaultPrevented).toBe(false)
  })

  fireEvent.change(screen.getByLabelText('Profile name 1'), { target: { value: 'Local Ollama v2' } })
  await waitFor(() => {
    const beforeUnmountBeforeUnload = new Event('beforeunload', { cancelable: true })
    window.dispatchEvent(beforeUnmountBeforeUnload)
    expect(beforeUnmountBeforeUnload.defaultPrevented).toBe(true)
  })
  unmount()

  const afterUnmountBeforeUnload = new Event('beforeunload', { cancelable: true })
  window.dispatchEvent(afterUnmountBeforeUnload)
  expect(afterUnmountBeforeUnload.defaultPrevented).toBe(false)
})
