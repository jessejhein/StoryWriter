import { afterEach, expect, test, vi } from 'vitest'
import {
  previewActionContext,
  runAction,
  runInvitation,
  runTaggedAction,
} from './api'
import type { RunActionResponse } from './editor/actionTypes'

// BDD Scenario: 7.1.1 - Preview minimal Line Polish context
// Requirements: M7-R09, M7-R17
// Test purpose: verify Milestone 7 action transport sends exact preview, run,
// and invitation requests without widening legacy selection behavior.

afterEach(() => vi.unstubAllGlobals())

const sceneRevision = `sha256:${'a'.repeat(64)}`
const chapterFingerprint = `sha256:${'b'.repeat(64)}`

// Test: sends exact context preview requests for every scope.
// Requirements: M7-R17.
test('sends exact context preview requests for every scope', async () => {
  const requests: Array<{ path: string; body: unknown }> = []
  vi.stubGlobal('fetch', vi.fn(async (input: string, init?: RequestInit) => {
    requests.push({ path: input, body: JSON.parse(String(init?.body)) })
    return {
      ok: true,
      json: async () => ({
        manifest: {
          scope: (JSON.parse(String(init?.body)) as { scope: string }).scope,
          packs_used: ['selected_text'],
          packs_omitted: [],
          estimated_input_tokens: 12,
          max_input_estimated_tokens: 8000,
          rag_mode: 'none',
        },
        target_revision: sceneRevision,
      }),
    }
  }))

  await previewActionContext({
    agent_id: 'line_polish',
    style_id: 'precise_editor',
    scope: 'selection',
    target: {
      scene_id: 'scn_0123456789abcdef0123',
      scene_revision: sceneRevision,
      start_byte: 0,
      end_byte: 10,
      text: 'Alpha beta',
    },
  })
  await previewActionContext({
    agent_id: 'scene_rewrite',
    style_id: 'precise_editor',
    scope: 'scene',
    target: {
      scene_id: 'scn_0123456789abcdef0123',
      scene_revision: sceneRevision,
    },
  })
  await previewActionContext({
    agent_id: 'chapter_review',
    style_id: 'precise_editor',
    scope: 'chapter_review',
    target: {
      chapter_id: 'ch_0123456789abcdef0123',
      fingerprint: chapterFingerprint,
    },
  })

  expect(requests).toEqual([
    {
      path: '/api/actions/context-preview',
      body: {
        agent_id: 'line_polish',
        style_id: 'precise_editor',
        scope: 'selection',
        target: {
          scene_id: 'scn_0123456789abcdef0123',
          scene_revision: sceneRevision,
          start_byte: 0,
          end_byte: 10,
          text: 'Alpha beta',
        },
      },
    },
    {
      path: '/api/actions/context-preview',
      body: {
        agent_id: 'scene_rewrite',
        style_id: 'precise_editor',
        scope: 'scene',
        target: {
          scene_id: 'scn_0123456789abcdef0123',
          scene_revision: sceneRevision,
        },
      },
    },
    {
      path: '/api/actions/context-preview',
      body: {
        agent_id: 'chapter_review',
        style_id: 'precise_editor',
        scope: 'chapter_review',
        target: {
          chapter_id: 'ch_0123456789abcdef0123',
          fingerprint: chapterFingerprint,
        },
      },
    },
  ])
})

// Test: preserves the legacy selection action request.
// Requirements: M7-R19.
test('preserves the legacy selection action request', async () => {
  const fetchMock = vi.fn().mockResolvedValue({
    ok: true,
    json: async () => ({
      run_id: 'run_0123456789abcdef0123',
      status: 'pending',
      agent_id: 'line_polish',
      style_id: 'precise_editor',
      scene_id: 'scn_0123456789abcdef0123',
      scene_revision: sceneRevision,
      selection: { start_byte: 0, end_byte: 10 },
      output_mode: 'patch',
      patch: { original: 'Alpha beta', replacement: 'Mock polished: Alpha beta' },
      context_summary: { packs_used: ['selected_text', 'style_sheet'], rag_mode: 'none' },
      provider: { profile_id: 'mock_default', type: 'openai_compatible', model: 'mock' },
    }),
  })
  vi.stubGlobal('fetch', fetchMock)

  await runAction({
    agent_id: 'line_polish',
    style_id: 'precise_editor',
    surface: 'editor',
    input_scope: 'selection',
    scene_id: 'scn_0123456789abcdef0123',
    scene_revision: sceneRevision,
    selection: { start_byte: 0, end_byte: 10, text: 'Alpha beta' },
  })

  expect(fetchMock).toHaveBeenCalledWith('/api/actions/run', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      agent_id: 'line_polish',
      style_id: 'precise_editor',
      surface: 'editor',
      input_scope: 'selection',
      scene_id: 'scn_0123456789abcdef0123',
      scene_revision: sceneRevision,
      selection: { start_byte: 0, end_byte: 10, text: 'Alpha beta' },
    }),
  })
})

// Test: runs an invitation only after an explicit call.
// Requirements: M7-R12.
test('runs an invitation only after an explicit call', async () => {
  const fetchMock = vi.fn().mockResolvedValue({
    ok: true,
    json: async () => ({
      run_id: 'run_bbbbbbbbbbbbbbbbbbbb',
      status: 'pending',
      agent_id: 'scene_rewrite',
      style_id: 'precise_editor',
      scope: 'scene',
      scene_id: 'scn_0123456789abcdef0123',
      scene_revision: sceneRevision,
      parent_run_id: 'run_aaaaaaaaaaaaaaaaaaaa',
      root_run_id: 'run_aaaaaaaaaaaaaaaaaaaa',
      chain_depth: 2,
      output_mode: 'patch',
      patch: { original: 'Scene.\n', replacement: 'Mock rewritten: Scene.\n' },
      context_summary: { packs_used: ['current_scene'], rag_mode: 'timeline_aware' },
      provider: { profile_id: 'mock_default', type: 'openai_compatible', model: 'mock' },
    }),
  })
  vi.stubGlobal('fetch', fetchMock)

  await runInvitation('invite_0123456789abcdef0123', {
    style_id: 'precise_editor',
    expected_target_revision: sceneRevision,
  })

  expect(fetchMock).toHaveBeenCalledTimes(1)
  expect(fetchMock).toHaveBeenCalledWith('/api/action-invitations/invite_0123456789abcdef0123/run', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      style_id: 'precise_editor',
      expected_target_revision: sceneRevision,
    }),
  })
})

// Test: maps manifests findings lineage and invitations.
// Requirements: M7-R17.
test('maps manifests findings lineage and invitations', async () => {
  const response: RunActionResponse = {
    run_id: 'run_cccccccccccccccccccc',
    status: 'completed',
    agent_id: 'chapter_review',
    style_id: 'precise_editor',
    scope: 'chapter_review',
    chapter_id: 'ch_0123456789abcdef0123',
    chapter_fingerprint: chapterFingerprint,
    parent_run_id: null,
    root_run_id: 'run_aaaaaaaaaaaaaaaaaaaa',
    chain_depth: 2,
    output_mode: 'suggestion',
    findings: [{
      title: 'Transition loses urgency',
      explanation: 'The shift releases tension.',
      scene_ids: ['scn_0123456789abcdef0123'],
      follow_up_agent_ids: ['scene_rewrite'],
    }],
    manifest: {
      scope: 'chapter_review',
      packs_used: ['current_chapter', 'style_sheet'],
      packs_omitted: [{ pack: 'outline_neighborhood', reason: 'budget' }],
      estimated_input_tokens: 1200,
      max_input_estimated_tokens: 12000,
      rag_mode: 'timeline_aware',
      active_codex: [{ entry_id: 'char_0123456789abcdef0123', applied_progression_ids: ['prog_0123456789abcdef0123'] }],
      outline_refs: ['ch_0123456789abcdef0123'],
    },
    follow_up_invitations: [{
      invitation_id: 'invite_dddddddddddddddddddd',
      parent_run_id: 'run_cccccccccccccccccccc',
      root_run_id: 'run_aaaaaaaaaaaaaaaaaaaa',
      chain_depth: 3,
      agent_id: 'scene_rewrite',
      scope: 'scene',
      scene_id: 'scn_0123456789abcdef0123',
      relationship: 'triggered',
    }],
    provider: { profile_id: 'mock_default', type: 'openai_compatible', model: 'mock' },
  }
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: async () => response }))

  await expect(runTaggedAction({
    agent_id: 'chapter_review',
    style_id: 'precise_editor',
    scope: 'chapter_review',
    target: {
      chapter_id: 'ch_0123456789abcdef0123',
      fingerprint: chapterFingerprint,
    },
  })).resolves.toEqual(response)
})