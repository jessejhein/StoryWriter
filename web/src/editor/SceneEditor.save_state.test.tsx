import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { expect, test, vi } from 'vitest'
import type { Project, SceneDocument } from '../api'
import SceneEditor from './SceneEditor'

vi.mock('../api', () => ({
  getScene: vi.fn(),
  saveScene: vi.fn(),
  getStyles: vi.fn(),
  getAvailableActions: vi.fn(),
  runAction: vi.fn(),
  acceptAction: vi.fn(),
  rejectAction: vi.fn(),
}))

vi.mock('./CodeMirrorSurface', () => ({
  default: ({ value, onChange }: { value: string; onChange: (value: string) => void }) => (
    <textarea aria-label="Scene Markdown" value={value} onChange={(event) => onChange(event.target.value)} />
  ),
}))

const api = await import('../api')

const project: Project = {
  project_id: 'proj_test_novel',
  name: 'Test Novel',
  path: '/tmp/test-novel',
  git_initialized: true,
  index_initialized: true,
}

const loadedScene: SceneDocument = {
  id: 'scn_00000000000000000001',
  chapter_id: 'ch_00000000000000000001',
  title: 'The Duel',
  frontmatter: {
    pov: 'Luke',
    status: 'draft',
    exclude_from_ai: false,
  },
  markdown: 'Scene prose.\n',
  revision: 'sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
}

const savedScene: SceneDocument = {
  ...loadedScene,
  title: 'The Duel Revised',
  frontmatter: {
    ...loadedScene.frontmatter,
    status: 'revised',
  },
  markdown: 'Revised prose.\n',
  revision: 'sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
}

// BDD trace:
// - Requirement: M2-R05.
// - Scenario: 2.5.1 — Editor state transitions.
// - Test purpose: verify field edits mark the scene dirty, saving disables
//   duplicate submits, and a successful save returns the editor to a clean saved
//   state with the new baseline revision.
test('transitions through dirty, saving, and saved states', async () => {
  let resolveSave: ((value: SceneDocument) => void) | undefined
  vi.mocked(api.getScene).mockResolvedValue(loadedScene)
  vi.mocked(api.saveScene).mockImplementation(() => new Promise((resolve) => {
    resolveSave = resolve
  }))

  render(<SceneEditor project={project} sceneID={loadedScene.id} onBack={() => {}} onDirtyChange={() => {}} />)

  await waitFor(() => expect(screen.getByText('Clean')).toBeInTheDocument())
  expect(screen.getByRole('button', { name: 'Save scene' })).toBeDisabled()

  fireEvent.change(screen.getByDisplayValue('The Duel'), { target: { value: 'The Duel Revised' } })
  fireEvent.change(screen.getByLabelText('Scene Markdown'), { target: { value: 'Revised prose.\n' } })
  fireEvent.change(screen.getByRole('combobox'), { target: { value: 'revised' } })

  expect(screen.getByText('Unsaved changes')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Save scene' })).not.toBeDisabled()

  fireEvent.click(screen.getByRole('button', { name: 'Save scene' }))

  await waitFor(() => expect(screen.getByText('Saving')).toBeInTheDocument())
  expect(screen.getByRole('button', { name: 'Save scene' })).toBeDisabled()

  resolveSave?.(savedScene)

  await waitFor(() => expect(api.saveScene).toHaveBeenCalledWith(loadedScene.id, {
    title: 'The Duel Revised',
    frontmatter: {
      pov: 'Luke',
      status: 'revised',
      exclude_from_ai: false,
    },
    markdown: 'Revised prose.\n',
    expected_revision: loadedScene.revision,
  }))
  await waitFor(() => expect(screen.getAllByText('Saved')).toHaveLength(2))
  expect(screen.getByRole('button', { name: 'Save scene' })).toBeDisabled()
})
