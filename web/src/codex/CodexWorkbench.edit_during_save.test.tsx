// BDD Scenario: 3.5.1 - Edit through explicit UI states
// Requirements: M3-R10, M3-R11
// Test purpose: Successful save responses advance the canonical baseline without discarding edits made while the request was in flight.
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, expect, test, vi } from 'vitest'
import type { CodexEntry, CodexProgressionDocument, Outline, Project } from '../api'
import CodexWorkbench from './CodexWorkbench'

vi.mock('../api', () => ({
  getCodexEntries: vi.fn(),
  createCodexEntry: vi.fn(),
  getCodexEntry: vi.fn(),
  updateCodexEntry: vi.fn(),
  getCodexProgressions: vi.fn(),
  saveCodexProgressions: vi.fn(),
  getCodexActiveState: vi.fn(),
  getOutline: vi.fn(),
}))

const api = await import('../api')

const project: Project = {
  project_id: 'proj_test_novel', name: 'Test Novel', path: '/tmp/test-novel',
  git_initialized: true, index_initialized: true,
}
const outline: Outline = { version: 1, arcs: [] }
const entry: CodexEntry = {
  id: 'char_0123456789abcdef0123', type: 'character', name: 'Ben', aliases: [], tags: [],
  description: 'Guide.', metadata: {}, revision: 'sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
}

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((complete) => { resolve = complete })
  return { promise, resolve }
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(api.getCodexEntries).mockResolvedValue({ entries: [entry] })
  vi.mocked(api.getOutline).mockResolvedValue(outline)
  vi.mocked(api.getCodexEntry).mockResolvedValue(entry)
  vi.mocked(api.getCodexProgressions).mockResolvedValue({ entry_id: entry.id, progressions: [], revision: null })
})

// Test: an update response advances the revision baseline without replacing entry edits made after submission.
// Requirements: M3-R10, M3-R11
test('preserves newer entry edits while an update is in flight', async () => {
  const save = deferred<CodexEntry>()
  vi.mocked(api.updateCodexEntry).mockReturnValue(save.promise)

  render(<CodexWorkbench project={project} />)
  await waitFor(() => expect(screen.getByRole('button', { name: 'Ben' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Ben' }))
  await waitFor(() => expect(screen.getByLabelText('Name')).toHaveValue('Ben'))
  fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'Ben Kenobi' } })
  fireEvent.click(screen.getByRole('button', { name: 'Save entry' }))
  fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'General Kenobi' } })

  save.resolve({ ...entry, name: 'Ben Kenobi', revision: 'sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb' })
  await waitFor(() => expect(screen.getByText('Unsaved changes')).toBeInTheDocument())
  await waitFor(() => expect(api.getCodexEntry).toHaveBeenCalledWith(entry.id))
  expect(screen.getByLabelText('Name')).toHaveValue('General Kenobi')
  expect(screen.getByRole('button', { name: 'Save entry' })).toBeEnabled()
})

// Test: a create response assigns canonical identity without replacing entry edits made after submission.
// Requirements: M3-R10, M3-R11
test('preserves newer entry edits while a create is in flight', async () => {
  const save = deferred<CodexEntry>()
  vi.mocked(api.getCodexEntries).mockResolvedValue({ entries: [] })
  vi.mocked(api.createCodexEntry).mockReturnValue(save.promise)

  render(<CodexWorkbench project={project} />)
  await waitFor(() => expect(screen.getByText('No Codex entries yet.')).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'New entry' }))
  fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'Ben Kenobi' } })
  fireEvent.click(screen.getByRole('button', { name: 'Save entry' }))
  fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'General Kenobi' } })

  save.resolve({ ...entry, name: 'Ben Kenobi', revision: 'sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb' })
  await waitFor(() => expect(screen.getByText('Unsaved changes')).toBeInTheDocument())
  await new Promise((resolve) => setTimeout(resolve, 0))
  expect(api.getCodexEntry).not.toHaveBeenCalled()
  expect(screen.getByLabelText('Name')).toHaveValue('General Kenobi')
  expect(screen.getByRole('button', { name: 'Save entry' })).toBeEnabled()
})

// Test: a progression response advances the revision baseline without replacing rows edited after submission.
// Requirements: M3-R10, M3-R11
test('preserves newer progression edits while a save is in flight', async () => {
  const save = deferred<CodexProgressionDocument>()
  vi.mocked(api.saveCodexProgressions).mockReturnValue(save.promise)

  render(<CodexWorkbench project={project} />)
  await waitFor(() => expect(screen.getByRole('button', { name: 'Ben' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Ben' }))
  await waitFor(() => expect(screen.getByLabelText('Name')).toHaveValue('Ben'))
  fireEvent.click(screen.getByRole('button', { name: 'Add progression' }))
  fireEvent.change(screen.getByLabelText('Progression description'), { target: { value: 'Submitted.' } })
  fireEvent.click(screen.getByRole('button', { name: 'Save progressions' }))
  fireEvent.change(screen.getByLabelText('Progression description'), { target: { value: 'Newer edit.' } })

  save.resolve({
    entry_id: entry.id,
    progressions: [{ id: 'prog_0123456789abcdef0123', anchor: { type: 'scene', id: '', timing: 'after' }, changes: { description: 'Submitted.' } }],
    revision: 'sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
  })
  await waitFor(() => expect(screen.getByText('Unsaved changes')).toBeInTheDocument())
  expect(screen.getByLabelText('Progression description')).toHaveValue('Newer edit.')
  expect(screen.getByRole('button', { name: 'Save progressions' })).toBeEnabled()
})
