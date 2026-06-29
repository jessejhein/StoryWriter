// BDD Scenario: 3.5.2 - Confirm destructive navigation
// Requirements: M3-R11, M3-R12
// Test purpose: Plain-English description of the project-level navigation guard when leaving the Codex workbench with unsaved changes.
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

const api = await import('./api')

const project = {
  project_id: 'proj_test_novel',
  name: 'Test Novel',
  path: '/tmp/test-novel',
  git_initialized: true,
  index_initialized: true,
}

test('confirms project-level navigation when codex draft is dirty', async () => {
  vi.mocked(api.getHealth).mockResolvedValue({ status: 'ok', version: '0.0.0-test' })
  vi.mocked(api.createProject).mockResolvedValue(project)
  vi.mocked(api.getCodexEntries).mockResolvedValue({ entries: [] })
  vi.mocked(api.getOutline).mockResolvedValue({ version: 1, arcs: [] })
  const confirm = vi.spyOn(window, 'confirm')

  // Test: leaving the Codex workbench for the outline asks before discarding an unsaved draft and preserves the draft on cancel.
  // Requirements: M3-R12
  render(<App />)

  await waitFor(() => expect(screen.getByText('Online · 0.0.0-test')).toBeInTheDocument())
  fireEvent.change(screen.getByPlaceholderText('The Glass Cartographer'), { target: { value: 'Test Novel' } })
  fireEvent.change(screen.getByPlaceholderText('/home/you/Stories/glass-cartographer'), { target: { value: '/tmp/test-novel' } })
  fireEvent.click(screen.getByRole('button', { name: 'Create project' }))

  await waitFor(() => expect(screen.getByRole('button', { name: 'Codex' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Codex' }))
  await waitFor(() => expect(screen.getByRole('button', { name: 'New entry' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'New entry' }))
  fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'Ben' } })

  confirm.mockReturnValueOnce(false)
  fireEvent.click(screen.getByRole('button', { name: 'Outline' }))
  expect(confirm).toHaveBeenCalled()
  expect(screen.getByLabelText('Name')).toHaveValue('Ben')

  const beforeUnload = new Event('beforeunload', { cancelable: true })
  window.dispatchEvent(beforeUnload)
  expect(beforeUnload.defaultPrevented).toBe(true)
})
