import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { expect, test, vi } from 'vitest'
import type { Outline, Project } from '../api'
import OutlineWorkbench from './OutlineWorkbench'

vi.mock('../api', () => ({
  getOutline: vi.fn(),
  createArc: vi.fn(),
  createChapter: vi.fn(),
  createScene: vi.fn(),
  reorderOutline: vi.fn(),
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

// BDD trace:
// - Requirement: M2-R01, M2-R15.
// - Scenario: 2.1.1 — Load a valid scene.
// - Test purpose: verify the outline exposes stable-ID open-scene actions that
//   hand navigation off without changing the active project.
test('emits the stable scene ID when the outline opens a scene', async () => {
  const onOpenScene = vi.fn()
  vi.mocked(api.getOutline).mockResolvedValue(outline)

  render(<OutlineWorkbench project={project} onOpenScene={onOpenScene} />)

  await waitFor(() => expect(screen.getByText('The Duel')).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Open scene The Duel' }))

  expect(onOpenScene).toHaveBeenCalledWith('scn_00000000000000000001')
})
