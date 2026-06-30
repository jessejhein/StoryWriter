// BDD Scenario: 3.5.3 - Inspect active state
// Requirements: M3-R07, M3-R10, M3-R11
// Test purpose: Entry and progression controls preserve canonical state while refreshing the active-state inspector and handling stale responses.
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, expect, test, vi } from 'vitest'
import type { CodexActiveState, CodexEntry, CodexProgressionDocument, Outline, Project } from '../api'
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
  project_id: 'proj_test_novel',
  name: 'Test Novel',
  path: '/tmp/test-novel',
  git_initialized: true,
  index_initialized: true,
}

const outline: Outline = {
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
}

const entry: CodexEntry = {
  id: 'char_0123456789abcdef0123',
  type: 'character',
  name: 'Obi-Wan Kenobi',
  aliases: ['Ben'],
  tags: ['mentor'],
  description: 'Guide.',
  metadata: { status: 'alive' },
  revision: 'sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
}

const progressions: CodexProgressionDocument = {
  entry_id: entry.id,
  progressions: [],
  revision: null,
}

const activeState: CodexActiveState = {
  scene_id: 'scn_0123456789abcdef0123',
  entry: {
    id: entry.id,
    type: entry.type,
    name: entry.name,
    aliases: entry.aliases,
    tags: entry.tags,
    description: 'Guide.',
    metadata: { role: 'mentor', status: 'alive' },
  },
  applied_progression_ids: [],
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(api.getOutline).mockResolvedValue(outline)
  vi.mocked(api.getCodexActiveState).mockResolvedValue(activeState)
})

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

// Test: selecting an entry loads its canonical state, progression saves are explicit, and the active-state inspector refreshes from the saved result.
// Requirements: M3-R07, M3-R10, M3-R11
test('loads an entry, saves progressions, and refreshes active state', async () => {
  vi.mocked(api.getCodexEntries).mockResolvedValue({ entries: [entry] })
  vi.mocked(api.getCodexEntry).mockResolvedValue(entry)
  vi.mocked(api.getCodexProgressions).mockResolvedValue(progressions)
  vi.mocked(api.saveCodexProgressions).mockResolvedValue({
    entry_id: entry.id,
    progressions: [{
      id: 'prog_0123456789abcdef0123',
      anchor: { type: 'scene', id: 'scn_0123456789abcdef0123', timing: 'after' },
      changes: { description: 'Gone.', metadata: { status: 'deceased' } },
    }],
    revision: 'sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
  })
  vi.mocked(api.getCodexActiveState)
    .mockResolvedValueOnce(activeState)
    .mockResolvedValue({
      ...activeState,
      entry: { ...activeState.entry, description: 'Gone.', metadata: { role: 'mentor', status: 'deceased' } },
      applied_progression_ids: ['prog_0123456789abcdef0123'],
    })

  render(<CodexWorkbench project={project} />)

  await waitFor(() => expect(screen.getByRole('button', { name: 'Obi-Wan Kenobi' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Obi-Wan Kenobi' }))
  await waitFor(() => expect(screen.getByLabelText('Description')).toHaveValue('Guide.'))
  await waitFor(() => expect(api.getCodexActiveState).toHaveBeenCalled())
  expect(screen.getByText('role')).toBeInTheDocument()
  expect(screen.getByText('mentor')).toBeInTheDocument()

  fireEvent.click(screen.getByRole('button', { name: 'Add progression' }))
  fireEvent.change(screen.getByLabelText('Progression description'), { target: { value: 'Gone.' } })
  fireEvent.click(screen.getByRole('button', { name: 'Add progression metadata' }))
  fireEvent.change(screen.getByLabelText('Progression metadata key 1'), { target: { value: 'status' } })
  fireEvent.change(screen.getByLabelText('Progression metadata value 1'), { target: { value: 'deceased' } })
  fireEvent.click(screen.getByRole('button', { name: 'Save progressions' }))
  await waitFor(() => expect(api.saveCodexProgressions).toHaveBeenCalled())
  expect(vi.mocked(api.saveCodexProgressions).mock.calls[0]?.[1]).toEqual({
    progressions: [{
      anchor: { type: 'scene', id: 'scn_0123456789abcdef0123', timing: 'after' },
      changes: { description: 'Gone.', metadata: { status: 'deceased' } },
    }],
    expected_revision: null,
  })
  await waitFor(() => expect(api.getCodexActiveState).toHaveBeenCalledTimes(2))
  expect(screen.getByText('deceased')).toBeInTheDocument()
  expect(screen.getByText('Applied progressions: prog_0123456789abcdef0123')).toBeInTheDocument()
})

// Test: alias, tag, and progression reorder and remove controls preserve the edited order in the save payload.
// Requirements: M3-R10, M3-R11
test('supports remove and reorder controls for aliases, tags, and progressions', async () => {
  const secondEntry: CodexEntry = {
    ...entry,
    aliases: ['Ben', 'General Kenobi'],
    tags: ['mentor', 'jedi'],
  }
  vi.mocked(api.getCodexEntries).mockResolvedValue({ entries: [secondEntry] })
  vi.mocked(api.getCodexEntry).mockResolvedValue(secondEntry)
  vi.mocked(api.getCodexProgressions).mockResolvedValue({
    entry_id: entry.id,
    progressions: [{
      id: 'prog_0123456789abcdef0123',
      anchor: { type: 'scene', id: 'scn_0123456789abcdef0123', timing: 'after' },
      changes: { description: 'First.' },
    }, {
      id: 'prog_0123456789abcdef0124',
      anchor: { type: 'scene', id: 'scn_0123456789abcdef0124', timing: 'before' },
      changes: { description: 'Second.' },
    }],
    revision: 'sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc',
  })
  vi.mocked(api.updateCodexEntry).mockResolvedValue(secondEntry)
  vi.mocked(api.saveCodexProgressions).mockResolvedValue({
    entry_id: entry.id,
    progressions: [{
      id: 'prog_0123456789abcdef0124',
      anchor: { type: 'scene', id: 'scn_0123456789abcdef0124', timing: 'before' },
      changes: { description: 'Second.' },
    }],
    revision: 'sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd',
  })

  render(<CodexWorkbench project={project} />)

  await waitFor(() => expect(screen.getByRole('button', { name: 'Obi-Wan Kenobi' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Obi-Wan Kenobi' }))
  await waitFor(() => expect(screen.getByLabelText('Alias 2')).toHaveValue('General Kenobi'))

  fireEvent.click(screen.getByRole('button', { name: 'Move alias 2 up' }))
  expect(screen.getByLabelText('Alias 1')).toHaveValue('General Kenobi')
  fireEvent.click(screen.getByRole('button', { name: 'Remove alias 2' }))
  expect(screen.queryByLabelText('Alias 2')).not.toBeInTheDocument()

  fireEvent.click(screen.getByRole('button', { name: 'Move tag 2 up' }))
  expect(screen.getByLabelText('Tag 1')).toHaveValue('jedi')
  fireEvent.click(screen.getByRole('button', { name: 'Remove tag 2' }))
  expect(screen.queryByLabelText('Tag 2')).not.toBeInTheDocument()

  fireEvent.click(screen.getByRole('button', { name: 'Move progression 2 up' }))
  expect(screen.getAllByLabelText('Progression description')[0]).toHaveValue('Second.')
  fireEvent.click(screen.getByRole('button', { name: 'Remove progression 2' }))
  expect(screen.getAllByLabelText('Progression description')).toHaveLength(1)

  fireEvent.click(screen.getByRole('button', { name: 'Save progressions' }))
  await waitFor(() => expect(api.saveCodexProgressions).toHaveBeenCalled())
  expect(vi.mocked(api.saveCodexProgressions).mock.calls[0]?.[1]).toEqual({
    progressions: [{
      id: 'prog_0123456789abcdef0124',
      anchor: { type: 'scene', id: 'scn_0123456789abcdef0124', timing: 'before' },
      changes: { description: 'Second.' },
    }],
    expected_revision: 'sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc',
  })
})

// Test: entry and progression metadata rows can be removed while preserving explicit empty string description changes in the saved payloads.
// Requirements: M3-R10, M3-R11
test('supports removing metadata rows and preserving explicit empty progression descriptions', async () => {
  vi.mocked(api.getCodexEntries).mockResolvedValue({ entries: [entry] })
  vi.mocked(api.getCodexEntry).mockResolvedValue({
    ...entry,
    metadata: { role: 'mentor', status: 'alive' },
  })
  vi.mocked(api.getCodexProgressions).mockResolvedValue({
    entry_id: entry.id,
    progressions: [{
      id: 'prog_0123456789abcdef0123',
      anchor: { type: 'scene', id: 'scn_0123456789abcdef0123', timing: 'after' },
      changes: { description: 'Guide.', metadata: { status: 'alive' } },
    }],
    revision: 'sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc',
  })
  vi.mocked(api.updateCodexEntry).mockResolvedValue(entry)
  vi.mocked(api.saveCodexProgressions).mockResolvedValue({
    entry_id: entry.id,
    progressions: [{
      id: 'prog_0123456789abcdef0123',
      anchor: { type: 'scene', id: 'scn_0123456789abcdef0123', timing: 'after' },
      changes: { description: '', metadata: { role: 'force-ghost' } },
    }],
    revision: 'sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd',
  })

  render(<CodexWorkbench project={project} />)

  await waitFor(() => expect(screen.getByRole('button', { name: 'Obi-Wan Kenobi' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Obi-Wan Kenobi' }))
  await waitFor(() => expect(screen.getByLabelText('Metadata key 2')).toHaveValue('status'))

  fireEvent.click(screen.getByRole('button', { name: 'Remove metadata 2' }))
  fireEvent.click(screen.getByRole('button', { name: 'Save entry' }))
  await waitFor(() => expect(api.updateCodexEntry).toHaveBeenCalled())
  expect(vi.mocked(api.updateCodexEntry).mock.calls[0]?.[1]).toEqual({
    name: 'Obi-Wan Kenobi',
    aliases: ['Ben'],
    tags: ['mentor'],
    description: 'Guide.',
    metadata: { role: 'mentor' },
    expected_revision: 'sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
  })

  await waitFor(() => expect(screen.getByLabelText('Progression description')).toHaveValue('Guide.'))
  fireEvent.change(screen.getByLabelText('Progression description'), { target: { value: '' } })
  fireEvent.click(screen.getByRole('button', { name: 'Remove progression metadata 1' }))
  fireEvent.click(screen.getByRole('button', { name: 'Add progression metadata' }))
  fireEvent.change(screen.getByLabelText('Progression metadata key 1'), { target: { value: 'role' } })
  fireEvent.change(screen.getByLabelText('Progression metadata value 1'), { target: { value: 'force-ghost' } })
  fireEvent.click(screen.getByRole('button', { name: 'Save progressions' }))
  await waitFor(() => expect(api.saveCodexProgressions).toHaveBeenCalled())
  expect(vi.mocked(api.saveCodexProgressions).mock.calls[0]?.[1]).toEqual({
    progressions: [{
      id: 'prog_0123456789abcdef0123',
      anchor: { type: 'scene', id: 'scn_0123456789abcdef0123', timing: 'after' },
      changes: { description: '', metadata: { role: 'force-ghost' } },
    }],
    expected_revision: 'sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc',
  })
})

test('removes a progression description change without removing its metadata changes', async () => {
  vi.mocked(api.getCodexEntries).mockResolvedValue({ entries: [entry] })
  vi.mocked(api.getCodexEntry).mockResolvedValue(entry)
  vi.mocked(api.getCodexProgressions).mockResolvedValue({
    entry_id: entry.id,
    progressions: [{
      id: 'prog_0123456789abcdef0123',
      anchor: { type: 'scene', id: 'scn_0123456789abcdef0123', timing: 'after' },
      changes: { description: 'Gone.', metadata: { status: 'deceased' } },
    }],
    revision: 'sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc',
  })
  vi.mocked(api.saveCodexProgressions).mockResolvedValue({
    entry_id: entry.id,
    progressions: [{
      id: 'prog_0123456789abcdef0123',
      anchor: { type: 'scene', id: 'scn_0123456789abcdef0123', timing: 'after' },
      changes: { metadata: { status: 'deceased' } },
    }],
    revision: 'sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd',
  })

  // Test: an author can remove an optional description replacement while preserving the row's metadata change.
  // Requirements: M3-R10, M3-R11
  render(<CodexWorkbench project={project} />)

  await waitFor(() => expect(screen.getByRole('button', { name: 'Obi-Wan Kenobi' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Obi-Wan Kenobi' }))
  await waitFor(() => expect(screen.getByLabelText('Progression description')).toHaveValue('Gone.'))
  fireEvent.click(screen.getByRole('button', { name: 'Remove description change' }))
  fireEvent.click(screen.getByRole('button', { name: 'Save progressions' }))

  await waitFor(() => expect(api.saveCodexProgressions).toHaveBeenCalled())
  expect(vi.mocked(api.saveCodexProgressions).mock.calls[0]?.[1].progressions[0]?.changes).toEqual({
    metadata: { status: 'deceased' },
  })
})

// Test: stale entry, progression, and active-state responses are ignored after the author switches selection.
// Requirements: M3-R11
test('ignores stale entry and active-state responses after switching selection', async () => {
  const firstEntry = entry
  const secondEntry: CodexEntry = {
    ...entry,
    id: 'char_0123456789abcdef0999',
    name: 'Leia Organa',
    aliases: ['Leia'],
    tags: ['leader'],
    description: 'Princess.',
    metadata: { status: 'alive', role: 'leader' },
    revision: 'sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee',
  }
  const firstEntryLoad = deferred<CodexEntry>()
  const firstProgressionsLoad = deferred<CodexProgressionDocument>()
  const secondEntryLoad = deferred<CodexEntry>()
  const secondProgressionsLoad = deferred<CodexProgressionDocument>()
  const firstActiveLoad = deferred<CodexActiveState>()
  const secondActiveLoad = deferred<CodexActiveState>()

  vi.mocked(api.getCodexEntries).mockResolvedValue({ entries: [firstEntry, secondEntry] })
  vi.mocked(api.getCodexEntry).mockImplementation((entryID: string) => {
    return entryID === firstEntry.id ? firstEntryLoad.promise : secondEntryLoad.promise
  })
  vi.mocked(api.getCodexProgressions).mockImplementation((entryID: string) => {
    return entryID === firstEntry.id ? firstProgressionsLoad.promise : secondProgressionsLoad.promise
  })
  vi.mocked(api.getCodexActiveState).mockImplementation((entryID: string) => {
    return entryID === firstEntry.id ? firstActiveLoad.promise : secondActiveLoad.promise
  })

  render(<CodexWorkbench project={project} />)

  await waitFor(() => expect(screen.getByRole('button', { name: 'Obi-Wan Kenobi' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Obi-Wan Kenobi' }))
  fireEvent.click(screen.getByRole('button', { name: 'Leia Organa' }))

  secondEntryLoad.resolve(secondEntry)
  secondProgressionsLoad.resolve(progressions)
  await waitFor(() => expect(screen.getByLabelText('Name')).toHaveValue('Leia Organa'))

  secondActiveLoad.resolve({
    ...activeState,
    entry: {
      ...activeState.entry,
      id: secondEntry.id,
      name: secondEntry.name,
      aliases: secondEntry.aliases,
      tags: secondEntry.tags,
      description: secondEntry.description,
      metadata: secondEntry.metadata,
    },
  })
  await waitFor(() => expect(screen.getByText('leader')).toBeInTheDocument())

  firstEntryLoad.resolve(firstEntry)
  firstProgressionsLoad.resolve(progressions)
  firstActiveLoad.resolve(activeState)

  await waitFor(() => expect(screen.getByLabelText('Name')).toHaveValue('Leia Organa'))
  expect(screen.queryByDisplayValue('Obi-Wan Kenobi')).not.toBeInTheDocument()
  expect(screen.getByText('leader')).toBeInTheDocument()
})
