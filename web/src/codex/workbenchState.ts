import type {
  CodexEntry,
  CodexEntryType,
  CodexProgression,
  CreateCodexEntryRequest,
  UpdateCodexEntryRequest,
} from '../api'
import type { EntryDraft, MetadataRow, ProgressionRow } from './workbenchTypes'

export const emptyDraft = (entryType: CodexEntryType = 'character'): EntryDraft => ({
  type: entryType,
  name: '',
  aliases: [],
  tags: [],
  description: '',
  metadata: [],
})

export function cloneMetadataRows(rows: MetadataRow[]): MetadataRow[] {
  return rows.map((row) => ({ ...row }))
}

export function cloneProgressionRows(rows: ProgressionRow[]): ProgressionRow[] {
  return rows.map((row) => ({
    ...row,
    metadata: cloneMetadataRows(row.metadata),
  }))
}

export function cloneDraft(draft: EntryDraft): EntryDraft {
  return {
    ...draft,
    aliases: [...draft.aliases],
    tags: [...draft.tags],
    metadata: cloneMetadataRows(draft.metadata),
  }
}

export function draftsEqual(left: EntryDraft, right: EntryDraft): boolean {
  return left.id === right.id &&
    left.type === right.type &&
    left.name === right.name &&
    left.description === right.description &&
    left.revision === right.revision &&
    stringArraysEqual(left.aliases, right.aliases) &&
    stringArraysEqual(left.tags, right.tags) &&
    metadataRowsEqual(left.metadata, right.metadata)
}

export function progressionRowsEqual(left: ProgressionRow[], right: ProgressionRow[]): boolean {
  if (left.length !== right.length) {
    return false
  }
  return left.every((row, index) => {
    const other = right[index]
    return row.id === other.id &&
      row.sceneID === other.sceneID &&
      row.timing === other.timing &&
      row.hasDescription === other.hasDescription &&
      row.description === other.description &&
      metadataRowsEqual(row.metadata, other.metadata)
  })
}

function metadataRowsEqual(left: MetadataRow[], right: MetadataRow[]): boolean {
  if (left.length !== right.length) {
    return false
  }
  return left.every((row, index) => row.key === right[index].key && row.value === right[index].value)
}

function stringArraysEqual(left: string[], right: string[]): boolean {
  if (left.length !== right.length) {
    return false
  }
  return left.every((value, index) => value === right[index])
}

export function moveItem<T>(items: T[], from: number, to: number): T[] {
  if (to < 0 || to >= items.length || from === to) {
    return items
  }
  const next = [...items]
  const [item] = next.splice(from, 1)
  next.splice(to, 0, item)
  return next
}

export function entryToDraft(entry: CodexEntry): EntryDraft {
  return {
    id: entry.id,
    type: entry.type,
    name: entry.name,
    aliases: [...entry.aliases],
    tags: [...entry.tags],
    description: entry.description,
    metadata: Object.entries(entry.metadata).map(([key, value]) => ({ key, value })),
    revision: entry.revision,
  }
}

export function draftToCreateRequest(draft: EntryDraft): CreateCodexEntryRequest {
  return {
    type: draft.type,
    name: draft.name,
    aliases: [...draft.aliases],
    tags: [...draft.tags],
    description: draft.description,
    metadata: Object.fromEntries(draft.metadata.filter((row) => row.key.trim() !== '').map((row) => [row.key, row.value])),
  }
}

export function draftToUpdateRequest(draft: EntryDraft): UpdateCodexEntryRequest {
  if (!draft.revision) {
    throw new Error('Cannot update a Codex entry without its canonical revision.')
  }
  return {
    name: draft.name,
    aliases: [...draft.aliases],
    tags: [...draft.tags],
    description: draft.description,
    metadata: Object.fromEntries(draft.metadata.filter((row) => row.key.trim() !== '').map((row) => [row.key, row.value])),
    expected_revision: draft.revision,
  }
}

export function progressionDocumentToRows(progressions: CodexProgression[]): ProgressionRow[] {
  return progressions.map((progression) => ({
    id: progression.id,
    sceneID: progression.anchor.id,
    timing: progression.anchor.timing,
    hasDescription: progression.changes.description !== undefined,
    description: progression.changes.description ?? '',
    metadata: Object.entries(progression.changes.metadata ?? {}).map(([key, value]) => ({ key, value })),
  }))
}

export function progressionRowsToRequest(rows: ProgressionRow[]): CodexProgression[] {
  return rows.map((progression) => {
    const metadata = Object.fromEntries(progression.metadata.filter((row) => row.key.trim() !== '').map((row) => [row.key, row.value]))
    return {
      id: progression.id,
      anchor: {
        type: 'scene',
        id: progression.sceneID,
        timing: progression.timing,
      },
      changes: {
        ...(progression.hasDescription ? { description: progression.description } : {}),
        ...(Object.keys(metadata).length > 0 ? { metadata } : {}),
      },
    }
  })
}

export function compareCodexEntries(left: CodexEntry, right: CodexEntry) {
  const typeOrder = { character: 0, location: 1, lore: 2, custom: 3 }
  if (typeOrder[left.type] !== typeOrder[right.type]) {
    return typeOrder[left.type] - typeOrder[right.type]
  }
  if (left.name < right.name) {
    return -1
  }
  if (left.name > right.name) {
    return 1
  }
  if (left.id < right.id) {
    return -1
  }
  if (left.id > right.id) {
    return 1
  }
  return 0
}
