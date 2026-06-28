import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, expect, test, vi } from 'vitest'
import type { Outline, OutlineMutation, Project } from '../api'
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

const emptyOutline: Outline = { version: 1, arcs: [] }

const nestedOutline: Outline = {
  version: 1,
  arcs: [{
    id: 'arc_00000000000000000001',
    title: 'Act One',
    display_label: 'Arc 1',
    chapters: [{
      id: 'ch_00000000000000000001',
      title: 'Arrival',
      display_label: 'Chapter 1.1',
      scenes: [
        { id: 'scn_00000000000000000001', title: 'The Station', display_label: 'Scene 1.1.1' },
        { id: 'scn_00000000000000000002', title: 'The Gate', display_label: 'Scene 1.1.2' },
      ],
    }, {
      id: 'ch_00000000000000000002',
      title: 'Departure',
      display_label: 'Chapter 1.2',
      scenes: [],
    }],
  }],
}

const reorderedOutline: Outline = {
  ...nestedOutline,
  arcs: [{
    ...nestedOutline.arcs[0],
    chapters: [{
      id: 'ch_00000000000000000002',
      title: 'Departure',
      display_label: 'Chapter 1.1',
      scenes: [],
    }, {
      id: 'ch_00000000000000000001',
      title: 'Arrival',
      display_label: 'Chapter 1.2',
      scenes: [
        { id: 'scn_00000000000000000001', title: 'The Station', display_label: 'Scene 1.2.1' },
        { id: 'scn_00000000000000000002', title: 'The Gate', display_label: 'Scene 1.2.2' },
      ],
    }],
  }],
}

function mutation(outline: Outline, changedID = ''): OutlineMutation {
  return { changed_id: changedID, outline }
}

beforeEach(() => {
  vi.clearAllMocks()
})

// BDD trace:
// - Requirement: Milestone 1, Story 1.5, use the outline UI.
// - Scenario: given an empty outline, when I create an arc from the UI, then
//   the inline form submits and the returned outline is rendered without a page
//   reload.
// - Test purpose: verify the empty-state create control and optimistic outline
//   replacement flow.
test('renders empty outline and creates an arc', async () => {
  vi.mocked(api.getOutline).mockResolvedValue(emptyOutline)
  vi.mocked(api.createArc).mockResolvedValue(mutation({
    version: 1,
    arcs: [{ id: 'arc_00000000000000000001', title: 'Act One', display_label: 'Arc 1', chapters: [] }],
  }, 'arc_00000000000000000001'))

  render(<OutlineWorkbench project={project} />)

  await waitFor(() => expect(screen.getByText('No arcs yet')).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Add arc' }))
  fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Act One' } })
  fireEvent.click(screen.getByRole('button', { name: 'Save' }))

  await waitFor(() => expect(api.createArc).toHaveBeenCalledWith('Act One'))
  await waitFor(() => expect(screen.getByText('Act One')).toBeInTheDocument())
})

// BDD trace:
// - Requirement: Milestone 1, Story 1.5, use the outline UI.
// - Scenario: given a nested outline, when I create a child node or move a
//   chapter up, the UI sends stable parent and child IDs and then renders the
//   updated labels from the returned outline.
// - Test purpose: verify create-child and reorder requests use stable IDs
//   rather than display labels or local positions.
test('sends stable IDs for child creation and reorder', async () => {
  vi.mocked(api.getOutline).mockResolvedValue(nestedOutline)
  vi.mocked(api.createScene).mockResolvedValue(mutation(nestedOutline, 'scn_00000000000000000003'))
  vi.mocked(api.reorderOutline).mockResolvedValue(mutation(reorderedOutline))

  render(<OutlineWorkbench project={project} />)

  await waitFor(() => expect(screen.getByText('Arrival')).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Add scene in Arrival' }))
  fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'The Bridge' } })
  fireEvent.click(screen.getByRole('button', { name: 'Save' }))

  await waitFor(() => expect(api.createScene).toHaveBeenCalledWith('ch_00000000000000000001', 'The Bridge'))

  fireEvent.click(screen.getByRole('button', { name: 'Move chapter Departure up' }))
  await waitFor(() => expect(api.reorderOutline).toHaveBeenCalledWith({
    parent_type: 'arc',
    parent_id: 'arc_00000000000000000001',
    ordered_child_ids: ['ch_00000000000000000002', 'ch_00000000000000000001'],
  }))
  await waitFor(() => expect(screen.getByText('Chapter 1.1')).toBeInTheDocument())
})

// BDD trace:
// - Requirement: Milestone 1, Story 1.5, use the outline UI.
// - Scenario: each valid parent exposes creation controls, and scene reorder
//   sends the complete stable-ID permutation for exactly that chapter.
// - Test purpose: verify the chapter-create and scene-reorder paths not covered
//   by the sibling arc-create/chapter-reorder test.
test('creates chapters and reorders scenes with complete stable-ID lists', async () => {
  vi.mocked(api.getOutline).mockResolvedValue(nestedOutline)
  vi.mocked(api.createChapter).mockResolvedValue(mutation(nestedOutline, 'ch_00000000000000000003'))
  vi.mocked(api.reorderOutline).mockResolvedValue(mutation({
    ...nestedOutline,
    arcs: [{
      ...nestedOutline.arcs[0],
      chapters: [{
        ...nestedOutline.arcs[0].chapters[0],
        scenes: [
          { id: 'scn_00000000000000000002', title: 'The Gate', display_label: 'Scene 1.1.1' },
          { id: 'scn_00000000000000000001', title: 'The Station', display_label: 'Scene 1.1.2' },
        ],
      }, nestedOutline.arcs[0].chapters[1]],
    }],
  }))

  render(<OutlineWorkbench project={project} />)
  await waitFor(() => expect(screen.getByText('Arrival')).toBeInTheDocument())

  fireEvent.click(screen.getByRole('button', { name: 'Add chapter in Act One' }))
  fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Interlude' } })
  fireEvent.click(screen.getByRole('button', { name: 'Save' }))
  await waitFor(() => expect(api.createChapter).toHaveBeenCalledWith('arc_00000000000000000001', 'Interlude'))

  fireEvent.click(screen.getByRole('button', { name: 'Move scene The Gate up' }))
  await waitFor(() => expect(api.reorderOutline).toHaveBeenCalledWith({
    parent_type: 'chapter',
    parent_id: 'ch_00000000000000000001',
    ordered_child_ids: ['scn_00000000000000000002', 'scn_00000000000000000001'],
  }))
  await waitFor(() => expect(screen.getByText('Scene 1.1.1').closest('li')).toHaveTextContent('The Gate'))
})

// BDD trace:
// - Requirement: Milestone 1, Story 1.5, use the outline UI.
// - Scenario: while a mutation is in progress, duplicate controls are disabled;
//   when an API request fails, the error is visible and the user can retry the
//   outline load.
// - Test purpose: verify mutation lockout and retryable error handling.
test('disables controls during mutation and shows retryable errors', async () => {
  let resolveCreate: ((value: OutlineMutation) => void) | undefined
  vi.mocked(api.getOutline)
    .mockRejectedValueOnce(new Error('outline unavailable'))
    .mockResolvedValueOnce(nestedOutline)
  vi.mocked(api.createChapter).mockImplementation(() => new Promise((resolve) => {
    resolveCreate = resolve
  }))

  render(<OutlineWorkbench project={project} />)

  await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('outline unavailable'))
  fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
  await waitFor(() => expect(screen.getByText('Departure')).toBeInTheDocument())

  fireEvent.click(screen.getByRole('button', { name: 'Add chapter in Act One' }))
  fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Interlude' } })
  fireEvent.click(screen.getByRole('button', { name: 'Save' }))

  expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled()
  expect(screen.getByRole('button', { name: 'Move chapter Departure up' })).toBeDisabled()

  resolveCreate?.(mutation(nestedOutline, 'ch_00000000000000000003'))
  await waitFor(() => expect(screen.getByRole('button', { name: 'Move chapter Departure up' })).not.toBeDisabled())
})
