import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, expect, test, vi } from 'vitest'
import ProviderWorkbench from './ProviderWorkbench'
import { normalizeProfiles } from './providerState'

let responses: Array<{ status?: number; body: unknown }>
const fetchMock = vi.fn<(path: string | URL | Request, init?: RequestInit) => Promise<Response>>(async () => {
  const next = responses.shift()
  if (!next) throw new Error('unexpected fetch')
  return new Response(JSON.stringify(next.body), {
    status: next.status ?? 200,
    headers: { 'Content-Type': 'application/json' },
  })
})

beforeEach(() => {
  responses = []
  fetchMock.mockClear()
  vi.stubGlobal('fetch', fetchMock)
})

test('normalizes profile ordering for dirty-state comparisons', () => {
  const alpha = { ...profileFixture, id: 'alpha', name: 'Alpha' }
  const zeta = { ...profileFixture, id: 'zeta', name: 'Zeta' }
  expect(normalizeProfiles([zeta, alpha])).toBe(normalizeProfiles([alpha, zeta]))
})

const profileFixture = {
  id: 'profile',
  name: 'Profile',
  type: 'ollama' as const,
  base_url: 'http://127.0.0.1:11434',
  auth: { type: 'none' as const, credential_env: '' },
  capabilities: { chat: true, streaming: false, structured_output: false, max_context_tokens: 8192 },
}

test('loads provider settings, edits a profile, and saves a clean baseline', async () => {
  const onDirtyChange = vi.fn()
  responses.push({ body: {
    profiles: [],
    revision: null,
  } })
  responses.push({ body: {
    profiles: [{
      id: 'local_ollama',
      name: 'Local Ollama',
      type: 'ollama',
      base_url: 'http://127.0.0.1:11434',
      auth: { type: 'none', credential_env: '' },
      capabilities: { chat: true, streaming: false, structured_output: false, max_context_tokens: 8192 },
      readiness: 'ready',
    }],
    revision: 'sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
  } })

  render(<ProviderWorkbench onDirtyChange={onDirtyChange} />)

  await waitFor(() => expect(screen.getByText('No provider profiles are configured yet.')).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Add profile' }))
  fireEvent.change(screen.getByLabelText('Profile ID 1'), { target: { value: 'local_ollama' } })
  fireEvent.change(screen.getByLabelText('Profile name 1'), { target: { value: 'Local Ollama' } })
  fireEvent.change(screen.getByLabelText('Provider type 1'), { target: { value: 'ollama' } })
  expect(screen.getByRole('button', { name: 'Save' })).toBeEnabled()
  fireEvent.click(screen.getByRole('button', { name: 'Save' }))

  await waitFor(() => expect(screen.getByText('Saved')).toBeInTheDocument())
  expect(fetchMock).toHaveBeenLastCalledWith('/api/provider-profiles', expect.objectContaining({ method: 'PUT' }))
  const request = fetchMock.mock.calls[1][1] as RequestInit
  expect(JSON.parse(String(request.body))).toEqual(expect.objectContaining({
    profiles: [expect.objectContaining({ id: 'local_ollama', name: 'Local Ollama', type: 'ollama' })],
    expected_revision: null,
  }))
  expect(onDirtyChange).toHaveBeenCalled()
})

test('shows conflict feedback and reload confirmation for a dirty draft', async () => {
  const onDirtyChange = vi.fn()
  responses.push({ body: {
      profiles: [{
        id: 'hosted_api',
        name: 'Hosted API',
        type: 'openai_compatible',
        base_url: 'https://api.example.test/v1',
        auth: { type: 'bearer_env', credential_env: 'STORYWORK_HOSTED_API_KEY' },
        capabilities: { chat: true, streaming: false, structured_output: false, max_context_tokens: 32768 },
        readiness: 'missing_credential',
      }],
      revision: 'sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
  } })
  responses.push({ status: 409, body: { error: 'stale expected revision' } })
  responses.push({ body: {
      profiles: [],
      revision: null,
  } })

  render(<ProviderWorkbench onDirtyChange={onDirtyChange} />)

  await waitFor(() => expect(screen.getByLabelText('Profile name 1')).toHaveValue('Hosted API'))
  fireEvent.change(screen.getByLabelText('Profile name 1'), { target: { value: 'Hosted API v2' } })
  fireEvent.click(screen.getByRole('button', { name: 'Save' }))
  await waitFor(() => expect(screen.getByText('stale expected revision')).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Reload latest' }))
  await waitFor(() => expect(screen.getByRole('dialog')).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Discard draft' }))
  await waitFor(() => expect(screen.getByText('No provider profiles are configured yet.')).toBeInTheDocument())
})

test('clears saved feedback on the next edit and disables save for invalid profiles', async () => {
  responses.push({ body: { profiles: [], revision: null } })
  responses.push({ body: {
    profiles: [{
      id: 'local_ollama',
      name: 'Local Ollama',
      type: 'ollama',
      base_url: 'http://127.0.0.1:11434',
      auth: { type: 'none', credential_env: '' },
      capabilities: { chat: true, streaming: false, structured_output: false, max_context_tokens: 8192 },
      readiness: 'ready',
    }],
    revision: 'sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
  } })

  render(<ProviderWorkbench onDirtyChange={vi.fn()} />)
  await waitFor(() => expect(screen.getByText('No provider profiles are configured yet.')).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Add profile' }))
  fireEvent.change(screen.getByLabelText('Profile ID 1'), { target: { value: 'local_ollama' } })
  fireEvent.change(screen.getByLabelText('Profile name 1'), { target: { value: 'Local Ollama' } })
  fireEvent.change(screen.getByLabelText('Provider type 1'), { target: { value: 'ollama' } })
  fireEvent.click(screen.getByRole('button', { name: 'Save' }))
  await waitFor(() => expect(screen.getByText('Saved')).toBeInTheDocument())

  fireEvent.change(screen.getByLabelText('Profile name 1'), { target: { value: 'Changed' } })
  expect(screen.queryByText('Saved')).not.toBeInTheDocument()

  fireEvent.change(screen.getByLabelText('Max context tokens 1'), { target: { value: '10000001' } })
  expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled()
  expect(screen.getByRole('alert')).toHaveTextContent('between 1 and 10,000,000')

  fireEvent.change(screen.getByLabelText('Max context tokens 1'), { target: { value: '8192' } })
  fireEvent.click(screen.getByRole('button', { name: 'Add profile' }))
  fireEvent.change(screen.getByLabelText('Profile ID 2'), { target: { value: 'local_ollama' } })
  fireEvent.change(screen.getByLabelText('Profile name 2'), { target: { value: 'Duplicate' } })
  expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled()
  expect(screen.getByRole('alert')).toHaveTextContent('unique')
})
