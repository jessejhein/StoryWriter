/**
 * App.scene_navigation.test.tsx
 *
 * Verifies project-level navigation between the outline and scene editor.
 */
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
}))

vi.mock('./editor/CodeMirrorSurface', () => ({
  default: ({ value, onChange }: { value: string; onChange: (value: string) => void }) => (
    <textarea aria-label="Scene Markdown" value={value} onChange={(event) => onChange(event.target.value)} />
  ),
}))

const api = await import('./api')

const project = {
  project_id: 'proj_test_novel',
  name: 'Test Novel',
  path: '/tmp/test-novel',
  git_initialized: true,
  index_initialized: true,
}

const outline = {
  version: 1,
  arcs: [{
    id: 'arc_00000000000000000001',
    title: 'Act One',
    display_label: 'Arc 1',
    chapters: [{
      id: 'ch_00000000000000000001',
      title: 'Arrival',
      display_label: 'Chapter 1.1',
      scenes: [{ id: 'scn_00000000000000000001', title: 'The Duel', display_label: 'Scene 1.1.1' }],
    }],
  }],
}

const scene = {
  id: 'scn_00000000000000000001',
  chapter_id: 'ch_00000000000000000001',
  title: 'The Duel',
  frontmatter: {
    pov: 'Luke',
    status: 'draft' as const,
    exclude_from_ai: false,
  },
  markdown: 'Scene prose.\n',
  revision: 'sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
}

// BDD trace:
// - Requirement: M2-R05, M2-R15.
// - Scenario: 2.5.3 — Navigate away with unsaved changes.
// - Test purpose: verify back-navigation asks for confirmation when the editor
//   is dirty, preserves the draft on cancel, and discards it on confirm.
test('confirms dirty navigation before leaving the editor', async () => {
  vi.mocked(api.getHealth).mockResolvedValue({ status: 'ok', version: '0.0.0-test' })
  vi.mocked(api.createProject).mockResolvedValue(project)
  vi.mocked(api.getOutline).mockResolvedValue(outline)
  vi.mocked(api.getScene).mockResolvedValue(scene)
  const confirm = vi.spyOn(window, 'confirm')

  render(<App />)

  await waitFor(() => expect(screen.getByText('Online · 0.0.0-test')).toBeInTheDocument())
  fireEvent.change(screen.getByPlaceholderText('The Glass Cartographer'), { target: { value: 'Test Novel' } })
  fireEvent.change(screen.getByPlaceholderText('/home/you/Stories/glass-cartographer'), { target: { value: '/tmp/test-novel' } })
  fireEvent.click(screen.getByRole('button', { name: 'Create project' }))

  await waitFor(() => expect(screen.getByRole('button', { name: 'Open scene The Duel' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Open scene The Duel' }))
  await waitFor(() => expect(screen.getByRole('button', { name: 'Back to outline' })).toBeInTheDocument())
  expect(screen.queryByRole('button', { name: 'Open scene The Duel' })).not.toBeInTheDocument()

  fireEvent.change(screen.getByLabelText('Scene Markdown'), { target: { value: 'Dirty draft.\n' } })

  confirm.mockReturnValueOnce(false)
  fireEvent.click(screen.getByRole('button', { name: 'Back to outline' }))
  expect(confirm).toHaveBeenCalled()
  expect(screen.getByLabelText('Scene Markdown')).toHaveValue('Dirty draft.\n')

  confirm.mockReturnValueOnce(true)
  fireEvent.click(screen.getByRole('button', { name: 'Back to outline' }))
  await waitFor(() => expect(screen.queryByLabelText('Scene Markdown')).not.toBeInTheDocument())
})
