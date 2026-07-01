import { beforeEach, expect, test, vi } from 'vitest'
import {
  acceptImportCandidate,
  createImport,
  discardImportCandidate,
  extractImport,
  getImportCandidate,
  getImportCandidates,
  getImportChunks,
  getImports,
  mergeImportCandidate,
  updateImportCandidate,
} from './api'

const fetchMock = vi.fn<(path: string | URL | Request, init?: RequestInit) => Promise<Response>>(async (...args) => {
  void args
  return new Response('{}', { status: 200, headers: { 'Content-Type': 'application/json' } })
})

beforeEach(() => {
  fetchMock.mockClear()
  vi.stubGlobal('fetch', fetchMock)
})

test('uses the documented import-review routes and JSON bodies', async () => {
  await createImport('/tmp/notes')
  await getImports()
  await getImportChunks('imp_0123456789abcdef0123')
  await extractImport('imp_0123456789abcdef0123', {
    chunk_ids: ['chk_0123456789abcdef0123'],
    mode: 'structure',
    profile_id: 'local_ollama',
    model: 'qwen2.5:7b',
  })
  await getImportCandidates({ status: 'pending', kind: 'codex' })
  await getImportCandidate('cand_0123456789abcdef0123')
  await updateImportCandidate('cand_0123456789abcdef0123', {
    type: 'character',
    name: 'Mara Venn',
    aliases: ['Mara'],
    tags: ['pilot'],
    description: 'Edited author text.',
  }, 'sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa')
  await mergeImportCandidate('cand_0123456789abcdef0123', {
    other_candidate_id: 'cand_abcdef0123456789abcd',
    expected_revision: 'sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
    other_expected_revision: 'sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
    proposal: {
      type: 'character',
      name: 'Mara Venn',
      aliases: ['Mara'],
      tags: ['pilot'],
      description: 'Merged author text.',
    },
  })
  await discardImportCandidate('cand_0123456789abcdef0123', 'sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa')
  await acceptImportCandidate('cand_0123456789abcdef0123', 'sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa')

  expect(fetchMock.mock.calls.map(([path]) => String(path))).toEqual([
    '/api/imports',
    '/api/imports',
    '/api/imports/imp_0123456789abcdef0123/chunks',
    '/api/imports/imp_0123456789abcdef0123/extractions',
    '/api/import-candidates?status=pending&kind=codex',
    '/api/import-candidates/cand_0123456789abcdef0123',
    '/api/import-candidates/cand_0123456789abcdef0123',
    '/api/import-candidates/cand_0123456789abcdef0123/merge',
    '/api/import-candidates/cand_0123456789abcdef0123/discard',
    '/api/import-candidates/cand_0123456789abcdef0123/accept',
  ])
})
