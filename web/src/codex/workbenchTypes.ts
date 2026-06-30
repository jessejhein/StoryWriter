import type { CodexEntryType } from '../api'

export type MetadataRow = { key: string; value: string }

export type ProgressionRow = {
  id?: string
  sceneID: string
  timing: 'before' | 'after'
  hasDescription: boolean
  description: string
  metadata: MetadataRow[]
}

export type EntryDraft = {
  id?: string
  type: CodexEntryType
  name: string
  aliases: string[]
  tags: string[]
  description: string
  metadata: MetadataRow[]
  revision?: string
}
