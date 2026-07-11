// BDD Scenario: 8.5.1 - Discard the active experiment
// Requirements: M8-R18
// Test purpose: Application composition invalidates branch-sensitive UI state
// and confirms branch changes before leaving dirty drafts.
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

function MockOutlineWorkbench({ onOpenScene }: { onOpenScene: (sceneID: string) => void }) {
  return (
    <section>
      <h2>Outline workbench</h2>
      <button type="button" onClick={() => onOpenScene('scn_00000000000000000001')}>Open scene</button>
    </section>
  )
}

function MockSceneEditor({ onBack, onDirtyChange }: { onBack: () => void; onDirtyChange: (dirty: boolean) => void }) {
  const [draft, setDraft] = useState('Scene draft')
  return (
    <section>
      <h2>Scene editor</h2>
      <textarea aria-label="Scene Markdown" value={draft} onChange={(event) => { setDraft(event.target.value); onDirtyChange(true) }} />
      <button type="button" onClick={onBack}>Back to outline</button>
    </section>
  )
}

function MockCodexWorkbench({ onDirtyChange }: { onDirtyChange: (dirty: boolean) => void }) {
  const [entry, setEntry] = useState('Codex draft')
  return (
    <section>
      <h2>Codex workbench</h2>
      <button type="button" onClick={() => onDirtyChange(true)}>Dirty codex draft</button>
      <button type="button" onClick={() => setEntry('Codex entry populated')}>Populate codex</button>
      <p>{entry}</p>
    </section>
  )
}

function MockImportReviewWorkbench({ onDirtyChange }: { onDirtyChange: (dirty: boolean) => void }) {
  const [draft, setDraft] = useState('Import draft')
  return (
    <section>
      <h2>Import review</h2>
      <button type="button" onClick={() => onDirtyChange(true)}>Dirty import draft</button>
      <button type="button" onClick={() => setDraft('Import draft populated')}>Populate import</button>
      <p>{draft}</p>
    </section>
  )
}

function MockProviderWorkbench() {
  return (
    <section>
      <h2>Provider settings</h2>
    </section>
  )
}

function MockBranchWorkbench({ onBranchChanged, onDirtyChange, appDirty }: { onBranchChanged: () => void; onDirtyChange: (dirty: boolean) => void; appDirty: boolean }) {
  const [mount] = useState(() => {
    branchMounts += 1
    return branchMounts
  })
  const [comparisonText, setComparisonText] = useState('comparison text')
  const [goal, setGoal] = useState('goal')
  const [findings, setFindings] = useState('findings')
  const [sceneRevision, setSceneRevision] = useState('scene revision')
  const [actionPreview, setActionPreview] = useState('action preview')
  const [actionRun, setActionRun] = useState('action run')
  const [invitations, setInvitations] = useState('invitations')
  const [codexForm, setCodexForm] = useState('codex form')
  const [importDraft, setImportDraft] = useState('import draft')
  return (
    <section>
      <h2>Branch workbench #{mount}</h2>
      <p>{appDirty ? 'app dirty' : 'app clean'}</p>
      <button type="button" onClick={() => onDirtyChange(true)}>Dirty branch draft</button>
      <button type="button" onClick={() => setComparisonText('comparison text populated')}>Populate comparison</button>
      <button type="button" onClick={() => setGoal('goal populated')}>Populate goal</button>
      <button type="button" onClick={() => setFindings('findings populated')}>Populate findings</button>
      <button type="button" onClick={() => setSceneRevision('scene revision populated')}>Populate scene revision</button>
      <button type="button" onClick={() => setActionPreview('action preview populated')}>Populate preview</button>
      <button type="button" onClick={() => setActionRun('action run populated')}>Populate run</button>
      <button type="button" onClick={() => setInvitations('invitations populated')}>Populate invitations</button>
      <button type="button" onClick={() => setCodexForm('codex form populated')}>Populate codex form</button>
      <button type="button" onClick={() => setImportDraft('import draft populated')}>Populate branch import draft</button>
      <button type="button" onClick={onBranchChanged}>Switch branch</button>
      <button type="button" onClick={onBranchChanged}>Promote selected files</button>
      <button type="button" onClick={onBranchChanged}>Discard experiment</button>
      <p>{comparisonText}</p>
      <p>{goal}</p>
      <p>{findings}</p>
      <p>{sceneRevision}</p>
      <p>{actionPreview}</p>
      <p>{actionRun}</p>
      <p>{invitations}</p>
      <p>{codexForm}</p>
      <p>{importDraft}</p>
    </section>
  )
}

vi.mock('./outline/OutlineWorkbench', () => ({ default: MockOutlineWorkbench }))
vi.mock('./editor/SceneEditor', () => ({ default: MockSceneEditor }))
vi.mock('./codex/CodexWorkbench', () => ({ default: MockCodexWorkbench }))
vi.mock('./imports/ImportReviewWorkbench', () => ({ default: MockImportReviewWorkbench }))
vi.mock('./providers/ProviderWorkbench', () => ({ default: MockProviderWorkbench }))
vi.mock('./branches/BranchWorkbench', () => ({ default: MockBranchWorkbench }))

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
  fireEvent.click(screen.getByRole('button', { name: 'Populate scene revision' }))
  fireEvent.click(screen.getByRole('button', { name: 'Populate preview' }))
  fireEvent.click(screen.getByRole('button', { name: 'Populate run' }))
  fireEvent.click(screen.getByRole('button', { name: 'Populate invitations' }))
  fireEvent.click(screen.getByRole('button', { name: 'Populate codex form' }))
  fireEvent.click(screen.getByRole('button', { name: 'Populate branch import draft' }))
  expect(screen.getByText('comparison text populated')).toBeInTheDocument()
  expect(screen.getByText('goal populated')).toBeInTheDocument()
  expect(screen.getByText('findings populated')).toBeInTheDocument()
  expect(screen.getByText('scene revision populated')).toBeInTheDocument()
  expect(screen.getByText('action preview populated')).toBeInTheDocument()
  expect(screen.getByText('action run populated')).toBeInTheDocument()
  expect(screen.getByText('invitations populated')).toBeInTheDocument()
  expect(screen.getByText('codex form populated')).toBeInTheDocument()
  expect(screen.getByText('import draft populated')).toBeInTheDocument()

  fireEvent.click(screen.getByRole('button', { name: 'Switch branch' }))
  await waitFor(() => expect(screen.getByText('Outline workbench')).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Branches' }))
  await waitFor(() => expect(screen.getByText(/Branch workbench #/)).toBeInTheDocument())
  expect(screen.queryByText('comparison text populated')).not.toBeInTheDocument()
  expect(screen.queryByText('goal populated')).not.toBeInTheDocument()
  expect(screen.queryByText('findings populated')).not.toBeInTheDocument()
  expect(screen.queryByText('scene revision populated')).not.toBeInTheDocument()
  expect(screen.queryByText('action preview populated')).not.toBeInTheDocument()
  expect(screen.queryByText('action run populated')).not.toBeInTheDocument()
  expect(screen.queryByText('invitations populated')).not.toBeInTheDocument()
  expect(screen.queryByText('codex form populated')).not.toBeInTheDocument()
  expect(screen.queryByText('import draft populated')).not.toBeInTheDocument()

  fireEvent.click(screen.getByRole('button', { name: 'Promote selected files' }))
  await waitFor(() => expect(screen.getByText('Outline workbench')).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Branches' }))
  await waitFor(() => expect(screen.getByText(/Branch workbench #/)).toBeInTheDocument())

  fireEvent.click(screen.getByRole('button', { name: 'Discard experiment' }))
  await waitFor(() => expect(screen.getByText('Outline workbench')).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Branches' }))
  await waitFor(() => expect(screen.getByText(/Branch workbench #/)).toBeInTheDocument())
})
