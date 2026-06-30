// BDD Scenario: 3.5.1 - Edit through explicit UI states
// Requirements: M3-R11
// Test purpose: Obsolete save outcomes cannot replace or mark failed a different entry selected while the request was in flight.
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
const first: CodexEntry = {
  id: 'char_0123456789abcdef0123', type: 'character', name: 'Ben', aliases: [], tags: [],
  description: 'Guide.', metadata: {}, revision: 'sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
}
const second: CodexEntry = {
  ...first,
  id: 'char_0123456789abcdef0124',
  name: 'Leia',
  description: 'Leader.',
  revision: 'sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
}
const emptyProgressions = (entryID: string): CodexProgressionDocument => ({ entry_id: entryID, progressions: [], revision: null })

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason: unknown) => void
  const promise = new Promise<T>((complete, fail) => {
    resolve = complete
    reject = fail
  })
  return { promise, resolve, reject }
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(api.getCodexEntries).mockResolvedValue({ entries: [first, second] })
  vi.mocked(api.getOutline).mockResolvedValue(outline)
  vi.mocked(api.getCodexEntry).mockImplementation(async (entryID) => entryID === first.id ? first : second)
  vi.mocked(api.getCodexProgressions).mockImplementation(async (entryID) => emptyProgressions(entryID))
})

// Test: navigating away during an entry save keeps the newly selected entry visible when the older save succeeds.
// Requirements: M3-R11
test('ignores an entry save response after selecting another entry', async () => {
  const save = deferred<CodexEntry>()
  vi.mocked(api.updateCodexEntry).mockReturnValue(save.promise)

  render(<CodexWorkbench project={project} />)
  await waitFor(() => expect(screen.getByRole('button', { name: 'Ben' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Ben' }))
  await waitFor(() => expect(screen.getByLabelText('Name')).toHaveValue('Ben'))
  fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'Ben Kenobi' } })
  fireEvent.click(screen.getByRole('button', { name: 'Save entry' }))
  fireEvent.click(screen.getByRole('button', { name: 'Leia' }))
  await waitFor(() => expect(screen.getByRole('dialog')).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Discard draft' }))
  await waitFor(() => expect(screen.getByLabelText('Name')).toHaveValue('Leia'))

  save.resolve({ ...first, name: 'Ben Kenobi', revision: 'sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc' })
  await waitFor(() => expect(screen.queryByRole('button', { name: 'Saving…' })).not.toBeInTheDocument())
  expect(screen.getByLabelText('Name')).toHaveValue('Leia')
})

// Test: navigating away during a progression save keeps the newly selected entry visible when the older save succeeds.
// Requirements: M3-R11
test('ignores a progression save response after selecting another entry', async () => {
  const save = deferred<CodexProgressionDocument>()
  vi.mocked(api.saveCodexProgressions).mockReturnValue(save.promise)

  render(<CodexWorkbench project={project} />)
  await waitFor(() => expect(screen.getByRole('button', { name: 'Ben' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Ben' }))
  await waitFor(() => expect(screen.getByLabelText('Name')).toHaveValue('Ben'))
  fireEvent.click(screen.getByRole('button', { name: 'Add progression' }))
  fireEvent.click(screen.getByRole('button', { name: 'Save progressions' }))
  fireEvent.click(screen.getByRole('button', { name: 'Leia' }))
  await waitFor(() => expect(screen.getByRole('dialog')).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Discard draft' }))
  await waitFor(() => expect(screen.getByLabelText('Name')).toHaveValue('Leia'))

  save.resolve({
    entry_id: first.id,
    progressions: [{ id: 'prog_0123456789abcdef0123', anchor: { type: 'scene', id: '', timing: 'after' }, changes: { description: 'Saved.' } }],
    revision: 'sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc',
  })
  await waitFor(() => expect(screen.queryByRole('button', { name: 'Saving…' })).not.toBeInTheDocument())
  expect(screen.getByLabelText('Name')).toHaveValue('Leia')
  expect(screen.queryByText('Progression 1')).not.toBeInTheDocument()
})

// Test: navigating away during an entry save keeps the newly selected entry clean when the older save fails.
// Requirements: M3-R11
test('ignores an entry save failure after selecting another entry', async () => {
  const save = deferred<CodexEntry>()
  vi.mocked(api.updateCodexEntry).mockReturnValue(save.promise)

  render(<CodexWorkbench project={project} />)
  await waitFor(() => expect(screen.getByRole('button', { name: 'Ben' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Ben' }))
  await waitFor(() => expect(screen.getByLabelText('Name')).toHaveValue('Ben'))
  fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'Ben Kenobi' } })
  fireEvent.click(screen.getByRole('button', { name: 'Save entry' }))
  fireEvent.click(screen.getByRole('button', { name: 'Leia' }))
  await waitFor(() => expect(screen.getByRole('dialog')).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Discard draft' }))
  await waitFor(() => expect(screen.getByLabelText('Name')).toHaveValue('Leia'))

  save.reject(new Error('obsolete entry failure'))
  await waitFor(() => expect(screen.queryByRole('button', { name: 'Saving…' })).not.toBeInTheDocument())
  expect(screen.queryByText('obsolete entry failure')).not.toBeInTheDocument()
  expect(screen.getAllByText('Saved')).toHaveLength(2)
})

// Test: navigating away during a progression save keeps the newly selected entry clean when the older save fails.
// Requirements: M3-R11
test('ignores a progression save failure after selecting another entry', async () => {
  const save = deferred<CodexProgressionDocument>()
  vi.mocked(api.saveCodexProgressions).mockReturnValue(save.promise)

  render(<CodexWorkbench project={project} />)
  await waitFor(() => expect(screen.getByRole('button', { name: 'Ben' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Ben' }))
  await waitFor(() => expect(screen.getByLabelText('Name')).toHaveValue('Ben'))
  fireEvent.click(screen.getByRole('button', { name: 'Add progression' }))
  fireEvent.click(screen.getByRole('button', { name: 'Save progressions' }))
  fireEvent.click(screen.getByRole('button', { name: 'Leia' }))
  await waitFor(() => expect(screen.getByRole('dialog')).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Discard draft' }))
  await waitFor(() => expect(screen.getByLabelText('Name')).toHaveValue('Leia'))

  save.reject(new Error('obsolete progression failure'))
  await waitFor(() => expect(screen.queryByRole('button', { name: 'Saving…' })).not.toBeInTheDocument())
  expect(screen.queryByText('obsolete progression failure')).not.toBeInTheDocument()
  expect(screen.getAllByText('Saved')).toHaveLength(2)
})
