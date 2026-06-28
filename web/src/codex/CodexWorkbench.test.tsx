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
    metadata: { status: 'alive' },
  },
  applied_progression_ids: [],
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(api.getOutline).mockResolvedValue(outline)
  vi.mocked(api.getCodexActiveState).mockResolvedValue(activeState)
})

// BDD Scenario: 3.1.1 - List an empty Codex
// Requirements: M3-R10, M3-R11
// Test purpose: Plain-English description of the Codex workbench empty state and explicit create workflow.
test('renders empty state and creates a new codex entry', async () => {
  vi.mocked(api.getCodexEntries).mockResolvedValue({ entries: [] })
  vi.mocked(api.createCodexEntry).mockResolvedValue(entry)

  // Test: an empty Codex shows a create action, tracks unsaved changes, saves explicitly, and renders the returned entry.
  // Requirements: M3-R10
  render(<CodexWorkbench project={project} />)

  await waitFor(() => expect(screen.getByText('No Codex entries yet.')).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'New entry' }))
  fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'Obi-Wan Kenobi' } })
  fireEvent.click(screen.getByRole('button', { name: 'Add alias' }))
  fireEvent.change(screen.getByLabelText('Alias 1'), { target: { value: 'Ben' } })
  fireEvent.click(screen.getByRole('button', { name: 'Save entry' }))

  await waitFor(() => expect(api.createCodexEntry).toHaveBeenCalled())
  await waitFor(() => expect(screen.getByRole('button', { name: 'Obi-Wan Kenobi' })).toBeInTheDocument())
})

// BDD Scenario: 3.5.3 - Inspect active state
// Requirements: M3-R07, M3-R10, M3-R11, M3-R12
// Test purpose: Plain-English description of the workbench edit, progression, active-state, and dirty-navigation guard behavior.
test('loads an entry, saves progressions, refreshes active state, and guards dirty navigation', async () => {
  vi.mocked(api.getCodexEntries).mockResolvedValue({ entries: [entry] })
  vi.mocked(api.getCodexEntry).mockResolvedValue(entry)
  vi.mocked(api.getCodexProgressions).mockResolvedValue(progressions)
  vi.mocked(api.saveCodexProgressions).mockResolvedValue({
    entry_id: entry.id,
    progressions: [{
      id: 'prog_0123456789abcdef0123',
      anchor: { type: 'scene', id: 'scn_0123456789abcdef0123', timing: 'after' },
      changes: { description: 'Gone.' },
    }],
    revision: 'sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
  })
  const confirm = vi.spyOn(window, 'confirm')

  // Test: selecting an entry loads its canonical state, progression saves are explicit, and switching entries while dirty asks for confirmation.
  // Requirements: M3-R12
  render(<CodexWorkbench project={project} />)

  await waitFor(() => expect(screen.getByRole('button', { name: 'Obi-Wan Kenobi' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Obi-Wan Kenobi' }))
  await waitFor(() => expect(screen.getByLabelText('Description')).toHaveValue('Guide.'))
  await waitFor(() => expect(api.getCodexActiveState).toHaveBeenCalled())

  fireEvent.click(screen.getByRole('button', { name: 'Add progression' }))
  fireEvent.change(screen.getByLabelText('Progression description'), { target: { value: 'Gone.' } })
  fireEvent.click(screen.getByRole('button', { name: 'Save progressions' }))
  await waitFor(() => expect(api.saveCodexProgressions).toHaveBeenCalled())

  fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'Ben Kenobi' } })
  confirm.mockReturnValueOnce(false)
  fireEvent.click(screen.getByRole('button', { name: 'New entry' }))
  expect(confirm).toHaveBeenCalled()
  expect(screen.getByLabelText('Name')).toHaveValue('Ben Kenobi')
})
