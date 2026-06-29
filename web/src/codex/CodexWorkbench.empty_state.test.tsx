// BDD Scenario: 3.1.1 - List an empty Codex
// Requirements: M3-R10, M3-R11
// Test purpose: Plain-English description of the Codex workbench empty state and explicit create workflow.
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, expect, test, vi } from 'vitest'
import type { CodexActiveState, CodexEntry, Outline, Project } from '../api'
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

test('renders empty state and creates a new codex entry', async () => {
  vi.mocked(api.getCodexEntries).mockResolvedValue({ entries: [] })
  vi.mocked(api.createCodexEntry).mockResolvedValue(entry)

  // Test: an empty Codex shows a create action, tracks unsaved changes, saves explicitly, and renders the returned entry.
  // Requirements: M3-R10
  render(<CodexWorkbench project={project} />)

  await waitFor(() => expect(screen.getByText('No Codex entries yet.')).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'New entry' }))
  expect(screen.getByText('Saved')).toBeInTheDocument()
  fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'Obi-Wan Kenobi' } })
  expect(screen.getByText('Unsaved changes')).toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: 'Add alias' }))
  fireEvent.change(screen.getByLabelText('Alias 1'), { target: { value: 'Ben' } })
  fireEvent.click(screen.getByRole('button', { name: 'Save entry' }))

  await waitFor(() => expect(api.createCodexEntry).toHaveBeenCalled())
  await waitFor(() => expect(screen.getByRole('button', { name: 'Obi-Wan Kenobi' })).toBeInTheDocument())
})
