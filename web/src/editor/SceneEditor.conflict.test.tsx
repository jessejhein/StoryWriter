import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { expect, test, vi } from 'vitest'
import { APIError, type Project, type SceneDocument } from '../api'
import SceneEditor from './SceneEditor'

vi.mock('../api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api')>()
  return {
    ...actual,
    getScene: vi.fn(),
    saveScene: vi.fn(),
    getStyles: vi.fn(),
    getAvailableActions: vi.fn(),
    runAction: vi.fn(),
    acceptAction: vi.fn(),
    rejectAction: vi.fn(),
  }
})

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
// - Requirement: M2-R05, M2-R14.
// - Scenario: 2.5.2 — Failed save retains draft.
// - Test purpose: verify a failed or conflicting save keeps the edited draft in
//   place, shows actionable feedback, and allows a retry or canonical reload.
test('retains the current draft on conflict and exposes retry actions', async () => {
  vi.mocked(api.getScene).mockResolvedValue(scene)
  vi.mocked(api.saveScene).mockRejectedValue(new APIError(409, 'canonical worktree is not clean'))

  render(<SceneEditor project={project} sceneID={scene.id} onBack={() => {}} onDirtyChange={() => {}} />)

  await waitFor(() => expect(screen.getByText('Clean')).toBeInTheDocument())
  fireEvent.change(screen.getByLabelText('Scene Markdown'), { target: { value: 'Conflicting draft.\n' } })
  fireEvent.click(screen.getByRole('button', { name: 'Save scene' }))

  await waitFor(() => expect(screen.getByText('Conflict')).toBeInTheDocument())
  expect(screen.getByLabelText('Scene Markdown')).toHaveValue('Conflicting draft.\n')
  expect(screen.getByRole('button', { name: 'Retry save' })).toBeInTheDocument()
  expect(screen.getAllByRole('button', { name: 'Reload canonical' })).toHaveLength(2)
})

// BDD trace:
// - Requirement: M2-R05, M2-R15.
// - Scenario: 2.5.3 — Navigate away with unsaved changes.
// - Test purpose: verify reloading canonical scene content asks for confirmation
//   when the current draft is dirty, preserves the draft on cancel, and reloads
//   canonical content on confirm.
test('confirms before reloading canonical content over a dirty draft', async () => {
  vi.mocked(api.getScene).mockResolvedValue(scene)

  render(<SceneEditor project={project} sceneID={scene.id} onBack={() => {}} onDirtyChange={() => {}} />)

  await waitFor(() => expect(screen.getByText('Clean')).toBeInTheDocument())
  fireEvent.change(screen.getByLabelText('Scene Markdown'), { target: { value: 'Dirty draft.\n' } })
  fireEvent.click(screen.getByRole('button', { name: 'Reload canonical' }))

  await waitFor(() => expect(screen.getByRole('dialog', { name: 'Discard scene draft?' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Keep editing' }))
  expect(screen.getByLabelText('Scene Markdown')).toHaveValue('Dirty draft.\n')

  fireEvent.click(screen.getByRole('button', { name: 'Reload canonical' }))
  await waitFor(() => expect(screen.getByRole('dialog', { name: 'Discard scene draft?' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Discard draft' }))

  await waitFor(() => expect(screen.getByLabelText('Scene Markdown')).toHaveValue('Scene prose.\n'))
})
