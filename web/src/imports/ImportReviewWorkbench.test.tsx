import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, expect, test, vi } from 'vitest'
import ImportReviewWorkbench from './ImportReviewWorkbench'

let responses: Array<{ status?: number; body: unknown }>
const fetchMock = vi.fn<(path: string | URL | Request, init?: RequestInit) => Promise<Response>>(async () => {
  const next = responses.shift()
  if (!next) throw new Error('unexpected fetch')
  return new Response(JSON.stringify(next.body), {
    status: next.status ?? 200,
    headers: { 'Content-Type': 'application/json' },
  })
})

beforeEach(() => {
  responses = []
  fetchMock.mockClear()
  vi.stubGlobal('fetch', fetchMock)
})

test('imports notes, extracts candidates, and accepts one candidate', async () => {
  responses.push(
    { body: { imports: [] } },
    { body: { candidates: [] } },
    { body: { profiles: [{ id: 'local_ollama', name: 'Local Ollama', type: 'ollama', base_url: 'http://127.0.0.1:11434', auth: { type: 'none', credential_env: '' }, capabilities: { chat: true, streaming: false, structured_output: false, max_context_tokens: 8192 }, readiness: 'ready' }], revision: null } },
    { body: { import: { id: 'imp_0123456789abcdef0123', created_at: '2026-06-30T12:00:00Z', file_count: 1, total_bytes: 12 }, files: [{ path: 'notes/characters.md', bytes: 12, sha256: 'a' }] } },
    { body: { imports: [{ id: 'imp_0123456789abcdef0123', created_at: '2026-06-30T12:00:00Z', file_count: 1, total_bytes: 12 }] } },
    { body: { chunks: [{ id: 'chk_0123456789abcdef0123', import_id: 'imp_0123456789abcdef0123', source_path: 'notes/characters.md', start_line: 1, end_line: 2, text: '# Characters\nMara\n', sha256: 'b' }] } },
    { body: { candidates: [{ id: 'cand_0123456789abcdef0123', kind: 'codex', proposal_version: 1, status: 'pending', revision: 'sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', provenance: { chunk_ids: ['chk_0123456789abcdef0123'] }, proposal: { type: 'character', name: 'Mara Venn', aliases: ['Mara'], tags: ['pilot'], description: 'A cautious salvage pilot.' }, replacement_candidate_id: null, canonical_refs: [] }], provider: { profile_id: 'local_ollama', type: 'ollama', model: 'qwen2.5:7b' } } },
    { body: { candidate: { id: 'cand_0123456789abcdef0123', kind: 'codex', proposal_version: 1, status: 'accepted', revision: 'sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb', provenance: { chunk_ids: ['chk_0123456789abcdef0123'] }, proposal: { type: 'character', name: 'Mara Venn', aliases: ['Mara'], tags: ['pilot'], description: 'A cautious salvage pilot.' }, replacement_candidate_id: null, canonical_refs: [{ kind: 'codex', id: 'char_0123456789abcdef0123' }] }, canonical_refs: [{ kind: 'codex', id: 'char_0123456789abcdef0123' }] } },
    { body: { candidates: [{ id: 'cand_0123456789abcdef0123', kind: 'codex', proposal_version: 1, status: 'accepted', revision: 'sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb', provenance: { chunk_ids: ['chk_0123456789abcdef0123'] }, proposal: { type: 'character', name: 'Mara Venn', aliases: ['Mara'], tags: ['pilot'], description: 'A cautious salvage pilot.' }, replacement_candidate_id: null, canonical_refs: [{ kind: 'codex', id: 'char_0123456789abcdef0123' }] }] } },
  )

  render(<ImportReviewWorkbench onDirtyChange={vi.fn()} />)

  await waitFor(() => expect(screen.getByText('No imports yet.')).toBeInTheDocument())
  fireEvent.change(screen.getByLabelText('Source directory'), { target: { value: '/tmp/notes' } })
  fireEvent.click(screen.getByRole('button', { name: 'Import' }))
  await waitFor(() => expect(screen.getByRole('button', { name: 'imp_0123456789abcdef0123' })).toBeInTheDocument())
  expect(screen.getByText('notes/characters.md · 12 bytes')).toBeInTheDocument()
  await waitFor(() => expect(screen.getByLabelText(/notes\/characters\.md lines 1-2/i)).toBeInTheDocument())
  fireEvent.click(screen.getByText('Inspect chunk'))
  expect(screen.getByText(/# Characters/)).toBeInTheDocument()
  fireEvent.click(screen.getByLabelText(/notes\/characters\.md lines 1-2/i))
  fireEvent.change(screen.getByLabelText('Model'), { target: { value: 'qwen2.5:7b' } })
  fireEvent.click(screen.getByRole('button', { name: 'Extract' }))
  await waitFor(() => expect(screen.getByText('cand_0123456789abcdef0123')).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Accept' }))
  expect(screen.getByRole('dialog', { name: 'Accept candidate into canon?' })).toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: 'Accept candidate' }))
  await waitFor(() => expect(screen.getByText(/Canonical refs: codex:char_0123456789abcdef0123/)).toBeInTheDocument())
})

test('confirms candidate navigation when a draft is dirty', async () => {
  responses.push(
    { body: { imports: [] } },
    { body: { candidates: [
      { id: 'cand_0123456789abcdef0123', kind: 'codex', proposal_version: 1, status: 'pending', revision: 'sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', provenance: { chunk_ids: ['chk_1'] }, proposal: { type: 'character', name: 'Mara Venn', aliases: ['Mara'], tags: ['pilot'], description: 'One.' }, replacement_candidate_id: null, canonical_refs: [] },
      { id: 'cand_0123456789abcdef0456', kind: 'codex', proposal_version: 1, status: 'pending', revision: 'sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb', provenance: { chunk_ids: ['chk_2'] }, proposal: { type: 'character', name: 'Tarin Voss', aliases: ['Tarin'], tags: ['pilot'], description: 'Two.' }, replacement_candidate_id: null, canonical_refs: [] },
    ] } },
    { body: { profiles: [], revision: null } },
  )

  render(<ImportReviewWorkbench onDirtyChange={vi.fn()} />)
  await waitFor(() => expect(screen.getByText('cand_0123456789abcdef0123')).toBeInTheDocument())
  fireEvent.change(screen.getByLabelText('Candidate description'), { target: { value: 'Changed.' } })
  fireEvent.click(screen.getAllByRole('button', { name: /codex · pending/i })[1])
  await waitFor(() => expect(screen.getByRole('dialog')).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Discard draft' }))
  await waitFor(() => expect(screen.getByDisplayValue('Tarin Voss')).toBeInTheDocument())
})

test('preserves a dirty draft on conflict and offers an explicit reload', async () => {
  const original = { id: 'cand_0123456789abcdef0123', kind: 'codex', proposal_version: 1, status: 'pending', revision: 'sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', provenance: { chunk_ids: ['chk_0123456789abcdef0123'] }, proposal: { type: 'character', name: 'Mara Venn', aliases: ['Mara'], tags: ['pilot'], description: 'One.' }, replacement_candidate_id: null, canonical_refs: [] }
  const current = { ...original, revision: 'sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb', proposal: { ...original.proposal, description: 'Changed elsewhere.' } }
  responses.push(
    { body: { imports: [] } },
    { body: { candidates: [original] } },
    { body: { profiles: [], revision: null } },
    { status: 409, body: { error: 'import operation conflicts with current project state' } },
    { body: { candidates: [current] } },
  )

  render(<ImportReviewWorkbench onDirtyChange={vi.fn()} />)
  await waitFor(() => expect(screen.getByDisplayValue('One.')).toBeInTheDocument())
  fireEvent.change(screen.getByLabelText('Candidate description'), { target: { value: 'My unsaved draft.' } })
  fireEvent.click(screen.getByRole('button', { name: 'Save' }))
  await waitFor(() => expect(screen.getByText(/Your draft is preserved/)).toBeInTheDocument())
  expect(screen.getByDisplayValue('My unsaved draft.')).toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: 'Reload server version' }))
  await waitFor(() => expect(screen.getByDisplayValue('Changed elsewhere.')).toBeInTheDocument())
})

test('ignores a stale chunk response after import selection changes', async () => {
  let resolveFirst: ((response: Response) => void) | undefined
  fetchMock.mockImplementation(async (input) => {
    const path = String(input)
    if (path === '/api/imports') return new Response(JSON.stringify({ imports: [
      { id: 'imp_0123456789abcdef0123', created_at: '2026-06-30T12:00:00Z', file_count: 1, total_bytes: 10 },
      { id: 'imp_abcdef0123456789abcd', created_at: '2026-06-30T13:00:00Z', file_count: 1, total_bytes: 20 },
    ] }))
    if (path === '/api/import-candidates') return new Response(JSON.stringify({ candidates: [] }))
    if (path === '/api/provider-profiles') return new Response(JSON.stringify({ profiles: [], revision: null }))
    if (path.includes('imp_0123456789abcdef0123')) {
      return new Promise<Response>((resolve) => { resolveFirst = resolve })
    }
    if (path.includes('imp_abcdef0123456789abcd')) {
      return new Response(JSON.stringify({ chunks: [{ id: 'chk_abcdef0123456789abcd', import_id: 'imp_abcdef0123456789abcd', source_path: 'new.md', start_line: 1, end_line: 1, text: 'New', sha256: 'b' }] }))
    }
    throw new Error(`unexpected fetch ${path}`)
  })

  render(<ImportReviewWorkbench onDirtyChange={vi.fn()} />)
  await waitFor(() => expect(screen.getByRole('button', { name: 'imp_abcdef0123456789abcd' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'imp_abcdef0123456789abcd' }))
  await waitFor(() => expect(screen.getByLabelText(/new\.md lines 1-1/i)).toBeInTheDocument())
  resolveFirst?.(new Response(JSON.stringify({ chunks: [{ id: 'chk_0123456789abcdef0123', import_id: 'imp_0123456789abcdef0123', source_path: 'stale.md', start_line: 1, end_line: 1, text: 'Stale', sha256: 'a' }] })))
  await Promise.resolve()
  expect(screen.queryByLabelText(/stale\.md/i)).not.toBeInTheDocument()
})
