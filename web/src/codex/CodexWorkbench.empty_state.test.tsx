// BDD Scenario: 3.1.1 - List an empty Codex
// Requirements: M3-R01, M3-R09, M3-R10, M3-R11
// Test purpose: The empty Codex workbench exposes the documented empty state and an explicit create action.
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, expect, test, vi } from 'vitest'
import type { Outline, Project } from '../api'
import CodexWorkbench from './CodexWorkbench'

vi.mock('../api', () => ({
  getCodexEntries: vi.fn(),
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

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(api.getOutline).mockResolvedValue(outline)
})

// Test: an empty Codex renders the documented empty state and exposes a New entry action.
// Requirements: M3-R01, M3-R09, M3-R10, M3-R11
test('renders the empty codex state with a create action', async () => {
  vi.mocked(api.getCodexEntries).mockResolvedValue({ entries: [] })
  render(<CodexWorkbench project={project} />)

  await waitFor(() => expect(screen.getByText('No Codex entries yet.')).toBeInTheDocument())
  expect(screen.getByRole('button', { name: 'New entry' })).toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: 'New entry' }))
  expect(screen.getByLabelText('Name')).toHaveValue('')
})
