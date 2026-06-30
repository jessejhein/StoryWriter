import { render, screen, waitFor } from '@testing-library/react'
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

const scene: SceneDocument = {
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

// BDD trace:
// - Requirement: M2-R01, M2-R02, M2-R03, M2-R15.
// - Scenario: 2.1.1 — Load a valid scene.
// - Test purpose: verify selecting a scene loads the supported metadata and
//   Markdown, exposes immutable IDs, and shows the clean editor state with Vim
//   mode visibly enabled.
test('loads a scene into the editor with clean state and vim indicator', async () => {
  vi.mocked(api.getScene).mockResolvedValue(scene)

  render(<SceneEditor project={project} sceneID={scene.id} onBack={() => {}} onDirtyChange={() => {}} />)

  await waitFor(() => expect(api.getScene).toHaveBeenCalledWith(scene.id))
  expect(screen.getByDisplayValue('The Duel')).toBeInTheDocument()
  expect(screen.getByDisplayValue('Luke')).toBeInTheDocument()
  expect(screen.getByRole('combobox')).toHaveValue('draft')
  expect(screen.getByLabelText('Scene Markdown')).toHaveValue('Scene prose.\n')
  expect(screen.getByText('Vim mode')).toBeInTheDocument()
  expect(screen.getByText('Clean')).toBeInTheDocument()
  expect(screen.getByText(scene.id)).toBeInTheDocument()
  expect(screen.getByText(scene.chapter_id)).toBeInTheDocument()
})
