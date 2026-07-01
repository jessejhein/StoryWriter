import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { expect, test, vi } from 'vitest'
import App from './App'

vi.mock('./api', () => ({
  getHealth: vi.fn(),
  createProject: vi.fn(),
  openProject: vi.fn(),
  getOutline: vi.fn(),
  createArc: vi.fn(),
  createChapter: vi.fn(),
  createScene: vi.fn(),
  reorderOutline: vi.fn(),
  getScene: vi.fn(),
  saveScene: vi.fn(),
  getCodexEntries: vi.fn(),
  createCodexEntry: vi.fn(),
  getCodexEntry: vi.fn(),
  updateCodexEntry: vi.fn(),
  getCodexProgressions: vi.fn(),
  saveCodexProgressions: vi.fn(),
  getCodexActiveState: vi.fn(),
  getProviderProfiles: vi.fn(),
  saveProviderProfiles: vi.fn(),
  APIError: class APIError extends Error {
    status: number
    constructor(status: number, message: string) {
      super(message)
      this.status = status
    }
  },
}))

const api = await import('./api')

test('opens provider settings before project selection and confirms dirty navigation', async () => {
  vi.mocked(api.getHealth).mockResolvedValue({ status: 'ok', version: '0.0.0-test' })
  vi.mocked(api.getProviderProfiles).mockResolvedValue({ profiles: [], revision: null })

  render(<App />)

  await waitFor(() => expect(screen.getByText('Online · 0.0.0-test')).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Provider settings' }))
  await waitFor(() => expect(screen.getByText('No provider profiles are configured yet.')).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Add profile' }))
  fireEvent.change(screen.getByLabelText('Profile ID 1'), { target: { value: 'local_ollama' } })
  await waitFor(() => {
    const beforeUnload = new Event('beforeunload', { cancelable: true })
    window.dispatchEvent(beforeUnload)
    expect(beforeUnload.defaultPrevented).toBe(true)
  })

  fireEvent.click(screen.getByRole('button', { name: 'Project setup' }))
  expect(screen.getByLabelText('Profile ID 1')).toHaveValue('local_ollama')
  expect(screen.getByText('Configure provider profiles without putting secrets in projects.')).toBeInTheDocument()
})
