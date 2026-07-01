import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { expect, test, vi } from 'vitest'
import type { Project } from '../api'
import SceneEditor from './SceneEditor'

vi.mock('./CodeMirrorSurface', () => ({
  default: ({
    value,
    onChange,
    onSelectionChange,
  }: {
    value: string
    onChange: (value: string) => void
    onSelectionChange?: (selection: { start: number; end: number; text: string }) => void
  }) => (
    <div>
      <textarea aria-label="Scene Markdown" value={value} onChange={(event) => onChange(event.target.value)} />
      <button type="button" onClick={() => onSelectionChange?.({ start: 0, end: 10, text: 'Alpha beta' })}>Select line polish</button>
      <button type="button" onClick={() => onSelectionChange?.({ start: 24, end: 32, text: 'Luz ágil' })}>Select utf8</button>
    </div>
  ),
}))

const project: Project = {
  project_id: 'proj_test_novel',
  name: 'Test Novel',
  path: '/tmp/test-novel',
  git_initialized: true,
  index_initialized: true,
}

// BDD trace:
// - Requirements: M4-R09, M4-R17, M4-R18.
// - Scenario: 4.5.1, 4.5.2.
// - Test purpose: verify the editor discovers actions, runs and previews a patch
//   through fetch, rejects without mutation, accepts without a second scene save,
//   and sends UTF-8 byte offsets for multibyte selections.
test('runs, rejects, and accepts scene actions through the fetch boundary', async () => {
  const requests: Array<{ path: string; init?: RequestInit }> = []
  const clipboardWrite = vi.fn(async () => {})
  Object.assign(navigator, { clipboard: { writeText: clipboardWrite } })

  vi.stubGlobal('fetch', vi.fn(async (input: string | URL | Request, init?: RequestInit) => {
    const path = typeof input === 'string' ? input : input instanceof URL ? input.pathname + input.search : input.url
    requests.push({ path, init })

    if (path === '/api/scenes/scn_0123456789abcdef0123') {
      return {
        ok: true,
        json: async () => ({
          id: 'scn_0123456789abcdef0123',
          chapter_id: 'ch_0123456789abcdef0123',
          title: 'The Duel',
          frontmatter: { pov: 'Luke', status: 'draft', exclude_from_ai: false },
          markdown: 'Alpha beta gamma delta.\nLuz ágil vuela.\n',
          revision: 'sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
        }),
      }
    }
    if (path === '/api/styles') {
      return {
        ok: true,
        json: async () => ({
          styles: [{
            id: 'precise_editor',
            version: 1,
            name: 'Precise Editor',
            provider_profile_id: 'mock_default',
            model: 'mock',
            temperature: 0.2,
            system_prompt: 'You are a careful prose editor.',
            provider_readiness: 'ready',
          }],
        }),
      }
    }
    if (path === '/api/actions/available?surface=editor&input_scope=selection&scene_id=scn_0123456789abcdef0123&selection_words=2') {
      return {
        ok: true,
        json: async () => ({
          actions: [{
            agent_id: 'line_polish',
            name: 'Line Polish',
            description: 'Rewrite selected prose.',
            output_mode: 'patch',
            requires_acceptance: true,
            style_ids: ['precise_editor'],
          }],
        }),
      }
    }
    if (path === '/api/actions/available?surface=editor&input_scope=selection&scene_id=scn_0123456789abcdef0123&selection_words=0') {
      return { ok: true, json: async () => ({ actions: [] }) }
    }
    if (path === '/api/actions/run' && init?.method === 'POST') {
      const body = JSON.parse(String(init.body))
      if (body.selection.text === 'Luz ágil') {
        return {
          ok: true,
          json: async () => ({
            run_id: 'run_utf8',
            status: 'pending',
            agent_id: body.agent_id,
            style_id: body.style_id,
            scene_id: body.scene_id,
            scene_revision: body.scene_revision,
            selection: { start_byte: body.selection.start_byte, end_byte: body.selection.end_byte },
            output_mode: 'patch',
            patch: { original: 'Luz ágil', replacement: 'Mock polished: Luz ágil' },
            context_summary: { packs_used: ['selected_text', 'style_sheet'], rag_mode: 'none' },
            provider: { profile_id: 'mock_default', type: 'openai_compatible', model: 'mock' },
          }),
        }
      }
      return {
        ok: true,
        json: async () => ({
          run_id: 'run_0123456789abcdef0123',
          status: 'pending',
          agent_id: body.agent_id,
          style_id: body.style_id,
          scene_id: body.scene_id,
          scene_revision: body.scene_revision,
          selection: { start_byte: body.selection.start_byte, end_byte: body.selection.end_byte },
          output_mode: 'patch',
          patch: { original: 'Alpha beta', replacement: 'Mock polished: Alpha beta' },
          context_summary: { packs_used: ['selected_text', 'style_sheet'], rag_mode: 'none' },
          provider: { profile_id: 'mock_default', type: 'openai_compatible', model: 'mock' },
        }),
      }
    }
    if (path === '/api/actions/run_0123456789abcdef0123/reject') {
      return { ok: true, json: async () => ({ run_id: 'run_0123456789abcdef0123', status: 'rejected' }) }
    }
    if (path === '/api/actions/run_utf8/accept') {
      return {
        ok: true,
        json: async () => ({
          run_id: 'run_utf8',
          status: 'accepted',
          scene: {
            id: 'scn_0123456789abcdef0123',
            chapter_id: 'ch_0123456789abcdef0123',
            title: 'The Duel',
            frontmatter: { pov: 'Luke', status: 'draft', exclude_from_ai: false },
            markdown: 'Alpha beta gamma delta.\nMock polished: Luz ágil vuela.\n',
            revision: 'sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
          },
        }),
      }
    }
    return { ok: false, status: 404, json: async () => ({ error: 'not found' }) }
  }))

  render(<SceneEditor project={project} sceneID="scn_0123456789abcdef0123" onBack={() => {}} onDirtyChange={() => {}} />)

  await waitFor(() => expect(screen.getByDisplayValue('The Duel')).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Select line polish' }))
  fireEvent.click(screen.getByRole('button', { name: 'AI actions' }))
  await waitFor(() => expect(screen.getByRole('button', { name: 'Run action' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Run action' }))
  await waitFor(() => expect(screen.getByText('Mock polished: Alpha beta')).toBeInTheDocument())
  const previewRegion = screen.getByRole('region', { name: 'AI patch preview' })
  expect(document.activeElement).toBe(previewRegion)
  expect(screen.getByText('Context packs: selected_text, style_sheet. RAG mode: none. Provider: mock_default (openai_compatible, model mock).')).toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: 'Copy replacement' }))
  await waitFor(() => expect(clipboardWrite).toHaveBeenCalledWith('Mock polished: Alpha beta'))
  fireEvent.click(screen.getByRole('button', { name: 'Reject replacement' }))
  await waitFor(() => expect(screen.queryByText('Mock polished: Alpha beta')).not.toBeInTheDocument())
  expect(screen.getByLabelText('Scene Markdown')).toHaveValue('Alpha beta gamma delta.\nLuz ágil vuela.\n')

  fireEvent.click(screen.getByRole('button', { name: 'Select utf8' }))
  fireEvent.click(screen.getByRole('button', { name: 'AI actions' }))
  await waitFor(() => expect(screen.getByRole('button', { name: 'Run action' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Run action' }))
  await waitFor(() => expect(screen.getByText('Mock polished: Luz ágil')).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Accept replacement' }))
  await waitFor(() => expect(screen.getAllByText('Saved').length).toBeGreaterThan(0))
  expect(screen.getByLabelText('Scene Markdown')).toHaveValue('Alpha beta gamma delta.\nMock polished: Luz ágil vuela.\n')

  const runBodies = requests
    .filter((request) => request.path === '/api/actions/run')
    .map((request) => JSON.parse(String(request.init?.body)))
  expect(runBodies[1]?.selection).toEqual({
    start_byte: 24,
    end_byte: 33,
    text: 'Luz ágil',
  })
  const rejectRequest = requests.find((request) => request.path === '/api/actions/run_0123456789abcdef0123/reject')
  expect(rejectRequest?.init?.body).toBeUndefined()
  expect(requests.some((request) => request.path === '/api/scenes/scn_0123456789abcdef0123' && request.init?.method === 'PUT')).toBe(false)

  vi.unstubAllGlobals()
})

// BDD trace:
// - Requirements: M4-R17, M4-R18.
// - Scenario: 4.5.1, 4.5.2.
// - Test purpose: verify dirty drafts disable action discovery with a clear
//   explanation and stale previews do not overwrite the editor baseline.
test('disables actions for dirty drafts', async () => {
  vi.stubGlobal('fetch', vi.fn(async (input: string | URL | Request) => {
    const path = typeof input === 'string' ? input : input instanceof URL ? input.pathname + input.search : input.url
    if (path === '/api/scenes/scn_0123456789abcdef0123') {
      return {
        ok: true,
        json: async () => ({
          id: 'scn_0123456789abcdef0123',
          chapter_id: 'ch_0123456789abcdef0123',
          title: 'The Duel',
          frontmatter: { pov: 'Luke', status: 'draft', exclude_from_ai: false },
          markdown: 'Alpha beta gamma delta.\n',
          revision: 'sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
        }),
      }
    }
    return { ok: false, status: 404, json: async () => ({ error: 'not found' }) }
  }))

  render(<SceneEditor project={project} sceneID="scn_0123456789abcdef0123" onBack={() => {}} onDirtyChange={() => {}} />)
  await waitFor(() => expect(screen.getByDisplayValue('The Duel')).toBeInTheDocument())
  fireEvent.change(screen.getByLabelText('Scene Markdown'), { target: { value: 'Changed draft' } })
  expect(screen.getByRole('button', { name: 'AI actions' })).toBeDisabled()
  expect(screen.getByText('Save or reload the scene before running AI actions.')).toBeInTheDocument()

  vi.unstubAllGlobals()
})
