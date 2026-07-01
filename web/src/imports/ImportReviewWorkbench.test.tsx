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
  await waitFor(() => expect(screen.getByLabelText(/notes\/characters\.md lines 1-2/i)).toBeInTheDocument())
  fireEvent.click(screen.getByLabelText(/notes\/characters\.md lines 1-2/i))
  fireEvent.change(screen.getByLabelText('Model'), { target: { value: 'qwen2.5:7b' } })
  fireEvent.click(screen.getByRole('button', { name: 'Extract' }))
  await waitFor(() => expect(screen.getByText('cand_0123456789abcdef0123')).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Accept' }))
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
