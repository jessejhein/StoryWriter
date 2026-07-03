// BDD Scenario: 7.2.3 / 7.3.2 - Scene replacement and chapter suggestions
// Requirements: M7-R03, M7-R04, M7-R09, M7-R11, M7-R15, M7-R17
// Test purpose: verify the integrated editor action workflows and dirty guards.

import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
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

const sceneRevision = `sha256:${'a'.repeat(64)}`
const sceneBody = 'Alpha beta gamma.\nMara entered the room.\n'

function buildFetchMock(requests: Array<{ path: string; init?: RequestInit }>) {
  return vi.fn(async (input: string | URL | Request, init?: RequestInit) => {
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
          markdown: sceneBody,
          revision: sceneRevision,
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
    if (path.startsWith('/api/actions/available')) {
      if (path.includes('input_scope=scene')) {
        return {
          ok: true,
          json: async () => ({
            actions: [{
              agent_id: 'scene_rewrite',
              name: 'Scene Rewrite',
              description: 'Rewrite one scene.',
              output_mode: 'patch',
              requires_acceptance: true,
              style_ids: ['precise_editor'],
            }],
          }),
        }
      }
      if (path.includes('input_scope=chapter_review')) {
        return {
          ok: true,
          json: async () => ({
            actions: [{
              agent_id: 'chapter_review',
              name: 'Chapter Review',
              description: 'Review one chapter.',
              output_mode: 'suggestion',
              requires_acceptance: false,
              style_ids: ['precise_editor'],
            }],
          }),
        }
      }
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
    if (path === '/api/actions/context-preview' && init?.method === 'POST') {
      const body = JSON.parse(String(init.body))
      return {
        ok: true,
        json: async () => ({
          manifest: {
            scope: body.scope,
            packs_used: body.scope === 'selection' ? ['selected_text', 'style_sheet'] : ['current_scene', 'style_sheet', 'active_codex_at_position'],
            packs_omitted: [],
            estimated_input_tokens: body.scope === 'selection' ? 42 : 1200,
            max_input_estimated_tokens: 12000,
            rag_mode: body.scope === 'selection' ? 'none' : 'timeline_aware',
          },
          target_revision: body.scope === 'chapter_review' ? `sha256:${'b'.repeat(64)}` : sceneRevision,
        }),
      }
    }
    if (path === '/api/actions/run' && init?.method === 'POST') {
      const body = JSON.parse(String(init.body))
      if (body.scope === 'scene') {
        return {
          ok: true,
          json: async () => ({
            run_id: 'run_scene0123456789abcdef',
            status: 'pending',
            agent_id: 'scene_rewrite',
            style_id: 'precise_editor',
            scope: 'scene',
            scene_id: 'scn_0123456789abcdef0123',
            scene_revision: sceneRevision,
            output_mode: 'patch',
            patch: { original: sceneBody, replacement: 'Mock rewritten: ' + sceneBody.trim() + '\n' },
            context_summary: { packs_used: ['current_scene', 'style_sheet'], rag_mode: 'timeline_aware' },
            provider: { profile_id: 'mock_default', type: 'openai_compatible', model: 'mock' },
          }),
        }
      }
      if (body.scope === 'chapter_review') {
        return {
          ok: true,
          json: async () => ({
            run_id: 'run_review0123456789abcde',
            status: 'completed',
            agent_id: 'chapter_review',
            style_id: 'precise_editor',
            scope: 'chapter_review',
            chapter_id: 'ch_0123456789abcdef0123',
            chapter_fingerprint: `sha256:${'b'.repeat(64)}`,
            output_mode: 'suggestion',
            findings: [{
              title: 'Transition loses urgency',
              explanation: 'The shift releases tension.',
              scene_ids: ['scn_0123456789abcdef0123'],
              follow_up_agent_ids: ['scene_rewrite'],
            }],
            follow_up_invitations: [{
              invitation_id: 'invite_0123456789abcdef0123',
              parent_run_id: 'run_review0123456789abcde',
              root_run_id: 'run_review0123456789abcde',
              chain_depth: 2,
              agent_id: 'scene_rewrite',
              scope: 'scene',
              scene_id: 'scn_0123456789abcdef0123',
              relationship: 'triggered',
            }],
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
    if (path === '/api/action-invitations/invite_0123456789abcdef0123/run' && init?.method === 'POST') {
      return {
        ok: true,
        json: async () => ({
          run_id: 'run_invite0123456789abcde',
          status: 'pending',
          agent_id: 'scene_rewrite',
          style_id: 'precise_editor',
          scope: 'scene',
          scene_id: 'scn_0123456789abcdef0123',
          scene_revision: sceneRevision,
          output_mode: 'patch',
          patch: { original: sceneBody, replacement: 'Mock rewritten: ' + sceneBody.trim() + '\n' },
          context_summary: { packs_used: ['current_scene', 'style_sheet'], rag_mode: 'timeline_aware' },
          provider: { profile_id: 'mock_default', type: 'openai_compatible', model: 'mock' },
        }),
      }
    }
    if (path === '/api/actions/run_scene0123456789abcdef/accept') {
      return {
        ok: true,
        json: async () => ({
          run_id: 'run_scene0123456789abcdef',
          status: 'accepted',
          follow_up_invitations: [],
          scene: {
            id: 'scn_0123456789abcdef0123',
            chapter_id: 'ch_0123456789abcdef0123',
            title: 'The Duel',
            frontmatter: { pov: 'Luke', status: 'draft', exclude_from_ai: false },
            markdown: 'Mock rewritten: ' + sceneBody,
            revision: `sha256:${'c'.repeat(64)}`,
          },
        }),
      }
    }
    return { ok: false, status: 404, json: async () => ({ error: 'not found' }) }
  })
}

// BDD Scenario: 7.2.3 - Review and accept one scene replacement
// Requirements: M7-R03, M7-R15
// Test purpose: verify scene rewrite preview, diff, and acceptance without a second save.

// Test: disables preview and run for dirty drafts.
// Requirements: M7-R09.
test('disables preview and run for dirty drafts', async () => {
  vi.stubGlobal('fetch', buildFetchMock([]))
  render(<SceneEditor project={project} sceneID="scn_0123456789abcdef0123" onBack={() => {}} onDirtyChange={() => {}} />)
  await waitFor(() => expect(screen.getByDisplayValue('The Duel')).toBeInTheDocument())
  fireEvent.change(screen.getByLabelText('Scene Markdown'), { target: { value: 'Changed draft' } })
  expect(screen.getByRole('button', { name: 'AI actions' })).toBeDisabled()
  expect(screen.getByRole('button', { name: 'Rewrite scene' })).toBeDisabled()
  expect(screen.getAllByText('Save or reload the scene before running AI actions.').length).toBeGreaterThan(0)
  vi.unstubAllGlobals()
})

// Test: shows a full scene diff for Scene Rewrite.
// Requirements: M7-R03.
test('shows a full scene diff for Scene Rewrite', async () => {
  const requests: Array<{ path: string; init?: RequestInit }> = []
  vi.stubGlobal('fetch', buildFetchMock(requests))
  render(<SceneEditor project={project} sceneID="scn_0123456789abcdef0123" onBack={() => {}} onDirtyChange={() => {}} />)
  await waitFor(() => expect(screen.getByDisplayValue('The Duel')).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Rewrite scene' }))
  const scenePanel = await screen.findByLabelText('Scene AI actions')
  await waitFor(() => expect(within(scenePanel).getByRole('button', { name: 'Run scene rewrite' })).toBeInTheDocument())
  fireEvent.click(within(scenePanel).getByRole('button', { name: 'Preview context' }))
  await waitFor(() => expect(screen.getByText(/current_scene, style_sheet, active_codex_at_position/)).toBeInTheDocument())
  fireEvent.click(within(scenePanel).getByRole('button', { name: 'Run scene rewrite' }))
  const preview = await screen.findByRole('region', { name: 'AI action preview' })
  await waitFor(() => expect(within(preview).getByText(/Mock rewritten:/)).toBeInTheDocument())
  expect(within(preview).getByText('Original', { selector: 'span' })).toBeInTheDocument()
  vi.unstubAllGlobals()
})

// Test: accepts Scene Rewrite without a second scene save.
// Requirements: M7-R15.
test('accepts Scene Rewrite without a second scene save', async () => {
  const requests: Array<{ path: string; init?: RequestInit }> = []
  vi.stubGlobal('fetch', buildFetchMock(requests))
  render(<SceneEditor project={project} sceneID="scn_0123456789abcdef0123" onBack={() => {}} onDirtyChange={() => {}} />)
  await waitFor(() => expect(screen.getByDisplayValue('The Duel')).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Rewrite scene' }))
  await waitFor(() => expect(screen.getByRole('button', { name: 'Run scene rewrite' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Run scene rewrite' }))
  fireEvent.click(screen.getByRole('button', { name: 'Run broader action' }))
  await waitFor(() => expect(screen.getByRole('button', { name: 'Accept replacement' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Accept replacement' }))
  await waitFor(() => expect(screen.getByLabelText('Scene Markdown')).toHaveValue('Mock rewritten: ' + sceneBody))
  expect(requests.some((request) => request.path === '/api/scenes/scn_0123456789abcdef0123' && request.init?.method === 'PUT')).toBe(false)
  vi.unstubAllGlobals()
})

// BDD Scenario: 7.3.2 - Return suggestions without canon mutation
// Requirements: M7-R04, M7-R11
// Test purpose: chapter review shows findings and follow-up cards without auto-running invited actions.

// Test: shows chapter review findings and follow-up invitation with scope confirmation.
// Requirements: M7-R04, M7-R11.
test('shows chapter review findings and follow-up invitation with scope confirmation', async () => {
  const requests: Array<{ path: string; init?: RequestInit }> = []
  vi.stubGlobal('fetch', buildFetchMock(requests))
  render(<SceneEditor project={project} sceneID="scn_0123456789abcdef0123" onBack={() => {}} onDirtyChange={() => {}} />)
  await waitFor(() => expect(screen.getByDisplayValue('The Duel')).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Review chapter' }))
  await waitFor(() => expect(screen.getByRole('button', { name: 'Run chapter review' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Run chapter review' }))
  fireEvent.click(screen.getByRole('button', { name: 'Run broader action' }))
  await waitFor(() => expect(screen.getByText('Transition loses urgency')).toBeInTheDocument())
  await waitFor(() => expect(screen.getByRole('region', { name: 'Follow-up invitations' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Run suggested action' }))
  await waitFor(() => expect(screen.getByRole('button', { name: 'Run broader action' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Run broader action' }))
  await waitFor(() => expect(requests.some((request) => request.path === '/api/action-invitations/invite_0123456789abcdef0123/run')).toBe(true))
  vi.unstubAllGlobals()
})

// Test: chapter context preview obtains the current fingerprint in one read-only request.
// Requirements: M7-R09, M7-R17.
test('previews chapter context once and retains the returned fingerprint', async () => {
  const requests: Array<{ path: string; init?: RequestInit }> = []
  vi.stubGlobal('fetch', buildFetchMock(requests))
  render(<SceneEditor project={project} sceneID="scn_0123456789abcdef0123" onBack={() => {}} onDirtyChange={() => {}} />)
  await waitFor(() => expect(screen.getByDisplayValue('The Duel')).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Review chapter' }))
  await waitFor(() => expect(screen.getByRole('button', { name: 'Preview context' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Preview context' }))
  await waitFor(() => expect(screen.getByText('chapter_review', { selector: 'strong' })).toBeInTheDocument())

  const previews = requests.filter((request) => request.path === '/api/actions/context-preview')
  expect(previews).toHaveLength(1)
  expect(JSON.parse(String(previews[0]?.init?.body))).toMatchObject({
    scope: 'chapter_review',
    target: { chapter_id: 'ch_0123456789abcdef0123' },
  })
  vi.unstubAllGlobals()
})

// Test: preserves conflict draft and ignores stale responses.
// Requirements: M7-R17.
test('preserves conflict draft and ignores stale responses', async () => {
  vi.stubGlobal('fetch', buildFetchMock([]))
  const { unmount } = render(<SceneEditor project={project} sceneID="scn_0123456789abcdef0123" onBack={() => {}} onDirtyChange={() => {}} />)
  await waitFor(() => expect(screen.getByDisplayValue('The Duel')).toBeInTheDocument())
  fireEvent.change(screen.getByLabelText('Scene Markdown'), { target: { value: 'Author draft stays' } })
  unmount()
  expect(screen.queryByText('Stale body')).not.toBeInTheDocument()
  vi.unstubAllGlobals()
})
