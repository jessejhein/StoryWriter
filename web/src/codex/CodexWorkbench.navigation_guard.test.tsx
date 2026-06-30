// BDD Scenario: 3.5.2 - Confirm destructive navigation
// Requirements: M3-R11, M3-R12
// Test purpose: Dirty Codex drafts require explicit confirmation before selecting another entry or resetting to a new draft.
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

const firstEntry: CodexEntry = {
  id: 'char_0123456789abcdef0123',
  type: 'character',
  name: 'Obi-Wan Kenobi',
  aliases: ['Ben'],
  tags: ['mentor'],
  description: 'Guide.',
  metadata: { status: 'alive' },
  revision: 'sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
}

const secondEntry: CodexEntry = {
  ...firstEntry,
  id: 'char_0123456789abcdef0999',
  name: 'Leia Organa',
  aliases: ['Leia'],
  tags: ['leader'],
  description: 'Princess.',
  metadata: { role: 'leader', status: 'alive' },
  revision: 'sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee',
}

const emptyProgressions = (entryID: string): CodexProgressionDocument => ({
  entry_id: entryID,
  progressions: [],
  revision: null,
})

const activeStateFor = (entry: CodexEntry): CodexActiveState => ({
  scene_id: 'scn_0123456789abcdef0123',
  entry: {
    id: entry.id,
    type: entry.type,
    name: entry.name,
    aliases: entry.aliases,
    tags: entry.tags,
    description: entry.description,
    metadata: entry.metadata,
  },
  applied_progression_ids: [],
})

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(api.getCodexEntries).mockResolvedValue({ entries: [firstEntry, secondEntry] })
  vi.mocked(api.getOutline).mockResolvedValue(outline)
  vi.mocked(api.getCodexEntry).mockImplementation(async (entryID) => entryID === firstEntry.id ? firstEntry : secondEntry)
  vi.mocked(api.getCodexProgressions).mockImplementation(async (entryID) => emptyProgressions(entryID))
  vi.mocked(api.getCodexActiveState).mockImplementation(async (entryID) => activeStateFor(entryID === firstEntry.id ? firstEntry : secondEntry))
})

// Test: selecting another Codex entry while the current draft is dirty opens the discard dialog and Cancel preserves the current draft.
// Requirements: M3-R11, M3-R12
test('keeps the current draft when selecting another entry and cancelling discard', async () => {
  render(<CodexWorkbench project={project} />)

  await waitFor(() => expect(screen.getByRole('button', { name: 'Obi-Wan Kenobi' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Obi-Wan Kenobi' }))
  await waitFor(() => expect(screen.getByLabelText('Name')).toHaveValue('Obi-Wan Kenobi'))

  fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'Ben Kenobi' } })
  fireEvent.click(screen.getByRole('button', { name: 'Leia Organa' }))

  await waitFor(() => expect(screen.getByRole('dialog', { name: 'Discard Codex draft?' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Keep editing' }))

  expect(screen.getByLabelText('Name')).toHaveValue('Ben Kenobi')
  expect(screen.queryByDisplayValue('Leia Organa')).not.toBeInTheDocument()
})

// Test: requesting a new entry while the current draft is dirty opens the discard dialog and Confirm resets to a fresh draft.
// Requirements: M3-R11, M3-R12
test('discards the current draft when requesting a new entry and confirming', async () => {
  render(<CodexWorkbench project={project} />)

  await waitFor(() => expect(screen.getByRole('button', { name: 'Obi-Wan Kenobi' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Obi-Wan Kenobi' }))
  await waitFor(() => expect(screen.getByLabelText('Name')).toHaveValue('Obi-Wan Kenobi'))

  fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'Ben Kenobi' } })
  fireEvent.click(screen.getByRole('button', { name: 'New entry' }))

  await waitFor(() => expect(screen.getByRole('dialog', { name: 'Discard Codex draft?' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Discard draft' }))

  await waitFor(() => expect(screen.getByLabelText('Name')).toHaveValue(''))
  expect(screen.queryByDisplayValue('Ben Kenobi')).not.toBeInTheDocument()
})
