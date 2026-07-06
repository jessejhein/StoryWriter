/**
 * App.branch_invalidation.test.tsx
 *
 * Proves branch navigation confirmation and branch-epoch invalidation at the
 * application composition boundary.
 */
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { useState } from 'react'
import { beforeEach, expect, test, vi } from 'vitest'
import App from './App'

vi.mock('./api', () => ({
  getHealth: vi.fn(),
  createProject: vi.fn(),
  openProject: vi.fn(),
}))

let branchMounts = 0

vi.mock('./outline/OutlineWorkbench', () => ({
  default: ({ onOpenScene }: { onOpenScene: (sceneID: string) => void }) => (
    <section>
      <h2>Outline workbench</h2>
      <button type="button" onClick={() => onOpenScene('scn_00000000000000000001')}>Open scene</button>
    </section>
  ),
}))

vi.mock('./editor/SceneEditor', () => ({
  default: ({ onBack, onDirtyChange }: { onBack: () => void; onDirtyChange: (dirty: boolean) => void }) => {
    const [draft, setDraft] = useState('Scene draft')
    return (
      <section>
        <h2>Scene editor</h2>
        <textarea aria-label="Scene Markdown" value={draft} onChange={(event) => { setDraft(event.target.value); onDirtyChange(true) }} />
        <button type="button" onClick={onBack}>Back to outline</button>
      </section>
    )
  },
}))

vi.mock('./codex/CodexWorkbench', () => ({
  default: ({ onDirtyChange }: { onDirtyChange: (dirty: boolean) => void }) => (
    <section>
      <h2>Codex workbench</h2>
      <button type="button" onClick={() => onDirtyChange(true)}>Dirty codex draft</button>
      <p>Codex draft</p>
    </section>
  ),
}))

vi.mock('./imports/ImportReviewWorkbench', () => ({
  default: ({ onDirtyChange }: { onDirtyChange: (dirty: boolean) => void }) => (
    <section>
      <h2>Import review</h2>
      <button type="button" onClick={() => onDirtyChange(true)}>Dirty import draft</button>
      <p>Import draft</p>
    </section>
  ),
}))

vi.mock('./providers/ProviderWorkbench', () => ({
  default: () => <section><h2>Provider settings</h2></section>,
}))

vi.mock('./branches/BranchWorkbench', () => ({
  default: ({ onBranchChanged, onDirtyChange, appDirty }: { onBranchChanged: () => void; onDirtyChange: (dirty: boolean) => void; appDirty: boolean }) => {
    const [mount] = useState(() => {
      branchMounts += 1
      return branchMounts
    })
    const [comparisonText, setComparisonText] = useState('comparison text')
    const [goal, setGoal] = useState('goal')
    const [findings, setFindings] = useState('findings')
    return (
      <section>
        <h2>Branch workbench #{mount}</h2>
        <p>{appDirty ? 'app dirty' : 'app clean'}</p>
        <button type="button" onClick={() => onDirtyChange(true)}>Dirty branch draft</button>
        <button type="button" onClick={() => setComparisonText('comparison text populated')}>Populate comparison</button>
        <button type="button" onClick={() => setGoal('goal populated')}>Populate goal</button>
        <button type="button" onClick={() => setFindings('findings populated')}>Populate findings</button>
        <button type="button" onClick={onBranchChanged}>Switch branch</button>
        <button type="button" onClick={onBranchChanged}>Promote selected files</button>
        <button type="button" onClick={onBranchChanged}>Discard experiment</button>
        <p>{comparisonText}</p>
        <p>{goal}</p>
        <p>{findings}</p>
      </section>
    )
  },
}))

const api = await import('./api')

beforeEach(() => {
  branchMounts = 0
  vi.clearAllMocks()
  vi.mocked(api.getHealth).mockResolvedValue({ status: 'ok', version: '0.0.0-test' })
  vi.mocked(api.createProject).mockResolvedValue({
    project_id: 'proj_story',
    name: 'Story',
    path: '/tmp/story',
    git_initialized: true,
    index_initialized: true,
  })
})

function createProject() {
  fireEvent.change(screen.getByPlaceholderText('The Glass Cartographer'), { target: { value: 'Story' } })
  fireEvent.change(screen.getByPlaceholderText('/home/you/Stories/glass-cartographer'), { target: { value: '/tmp/story' } })
  fireEvent.click(screen.getByRole('button', { name: 'Create project' }))
}

async function openBranchNavigationPrompt() {
  fireEvent.click(screen.getByRole('button', { name: 'Branches' }))
  await waitFor(() => expect(screen.getByRole('dialog', { name: 'Discard current draft?' })).toBeInTheDocument())
}

async function confirmBranchNavigation() {
  await waitFor(() => expect(screen.getByRole('dialog', { name: 'Discard current draft?' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Discard draft' }))
}

test('confirms branch navigation before leaving dirty scene, codex, and import drafts', async () => {
  render(<App />)
  await waitFor(() => expect(screen.getByText('Online · 0.0.0-test')).toBeInTheDocument())
  createProject()
  await waitFor(() => expect(screen.getByRole('button', { name: 'Outline' })).toBeInTheDocument())

  fireEvent.click(screen.getByRole('button', { name: 'Open scene' }))
  await waitFor(() => expect(screen.getByRole('button', { name: 'Back to outline' })).toBeInTheDocument())
  fireEvent.change(screen.getByLabelText('Scene Markdown'), { target: { value: 'Scene draft dirty' } })
  await openBranchNavigationPrompt()
  expect(screen.queryByRole('button', { name: 'Switch branch' })).not.toBeInTheDocument()
  await confirmBranchNavigation()
  await waitFor(() => expect(screen.getByText(/Branch workbench #/)).toBeInTheDocument())
  expect(screen.queryByText('Scene editor')).not.toBeInTheDocument()

  fireEvent.click(screen.getByRole('button', { name: 'Outline' }))
  fireEvent.click(screen.getByRole('button', { name: 'Codex' }))
  await waitFor(() => expect(screen.getByRole('button', { name: 'Dirty codex draft' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Dirty codex draft' }))
  await openBranchNavigationPrompt()
  expect(screen.queryByRole('button', { name: 'Switch branch' })).not.toBeInTheDocument()
  await confirmBranchNavigation()
  await waitFor(() => expect(screen.getByText(/Branch workbench #/)).toBeInTheDocument())
  expect(screen.queryByText('Codex draft')).not.toBeInTheDocument()

  fireEvent.click(screen.getByRole('button', { name: 'Outline' }))
  fireEvent.click(screen.getByRole('button', { name: 'Import Review' }))
  await waitFor(() => expect(screen.getByRole('button', { name: 'Dirty import draft' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Dirty import draft' }))
  await openBranchNavigationPrompt()
  expect(screen.queryByRole('button', { name: 'Switch branch' })).not.toBeInTheDocument()
  await confirmBranchNavigation()
  await waitFor(() => expect(screen.getByText(/Branch workbench #/)).toBeInTheDocument())
  expect(screen.queryByText('Import draft')).not.toBeInTheDocument()
})

test('remounts branch-sensitive workspaces after switch, promotion, and discard actions', async () => {
  render(<App />)
  await waitFor(() => expect(screen.getByText('Online · 0.0.0-test')).toBeInTheDocument())
  createProject()
  await waitFor(() => expect(screen.getByRole('button', { name: 'Branches' })).toBeInTheDocument())

  fireEvent.click(screen.getByRole('button', { name: 'Branches' }))
  await waitFor(() => expect(screen.getByText(/Branch workbench #/)).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Populate comparison' }))
  fireEvent.click(screen.getByRole('button', { name: 'Populate goal' }))
  fireEvent.click(screen.getByRole('button', { name: 'Populate findings' }))
  expect(screen.getByText('comparison text populated')).toBeInTheDocument()
  expect(screen.getByText('goal populated')).toBeInTheDocument()
  expect(screen.getByText('findings populated')).toBeInTheDocument()

  fireEvent.click(screen.getByRole('button', { name: 'Switch branch' }))
  await waitFor(() => expect(screen.getByText('Outline workbench')).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Branches' }))
  await waitFor(() => expect(screen.getByText(/Branch workbench #/)).toBeInTheDocument())
  expect(screen.queryByText('comparison text populated')).not.toBeInTheDocument()
  expect(screen.queryByText('goal populated')).not.toBeInTheDocument()
  expect(screen.queryByText('findings populated')).not.toBeInTheDocument()

  fireEvent.click(screen.getByRole('button', { name: 'Promote selected files' }))
  await waitFor(() => expect(screen.getByText('Outline workbench')).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Branches' }))
  await waitFor(() => expect(screen.getByText(/Branch workbench #/)).toBeInTheDocument())

  fireEvent.click(screen.getByRole('button', { name: 'Discard experiment' }))
  await waitFor(() => expect(screen.getByText('Outline workbench')).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Branches' }))
  await waitFor(() => expect(screen.getByText(/Branch workbench #/)).toBeInTheDocument())
})
