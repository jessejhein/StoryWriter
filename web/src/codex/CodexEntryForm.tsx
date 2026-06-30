import type { CodexEntryType } from '../api'
import type { EntryDraft } from './workbenchTypes'

type Props = {
  selectedEntryID: string | null
  draft: EntryDraft
  status: string
  saving: boolean
  dirty: boolean
  onChangeDraft: (nextDraft: EntryDraft) => void
  onSave: () => void
}

export default function CodexEntryForm({ selectedEntryID, draft, status, saving, dirty, onChangeDraft, onSave }: Props) {
  function updateAlias(index: number, value: string) {
    const aliases = [...draft.aliases]
    aliases[index] = value
    onChangeDraft({ ...draft, aliases })
  }

  function updateTag(index: number, value: string) {
    const tags = [...draft.tags]
    tags[index] = value
    onChangeDraft({ ...draft, tags })
  }

  function updateMetadata(index: number, key: 'key' | 'value', value: string) {
    const metadata = [...draft.metadata]
    metadata[index] = { ...metadata[index], [key]: value }
    onChangeDraft({ ...draft, metadata })
  }

  function moveItem<T>(items: T[], from: number, to: number): T[] {
    if (to < 0 || to >= items.length || from === to) {
      return items
    }
    const next = [...items]
    const [item] = next.splice(from, 1)
    next.splice(to, 0, item)
    return next
  }

  return (
    <>
      <header className="section-heading">
        <div>
          <p className="folio">Milestone 3 / Codex</p>
          <h3>{selectedEntryID ? draft.name || 'Untitled entry' : 'New entry'}</h3>
        </div>
        <div>
          <span>{status}</span>
          <button type="button" onClick={onSave} disabled={saving || !dirty}>
            {saving ? 'Saving…' : 'Save entry'}
          </button>
        </div>
      </header>

      <label>
        Type
        <select value={draft.type} onChange={(event) => onChangeDraft({ ...draft, type: event.target.value as CodexEntryType })} disabled={Boolean(draft.id)}>
          <option value="character">Character</option>
          <option value="location">Location</option>
          <option value="lore">Lore</option>
          <option value="custom">Custom</option>
        </select>
      </label>
      <label>
        Name
        <input value={draft.name} onChange={(event) => onChangeDraft({ ...draft, name: event.target.value })} />
      </label>
      <label>
        Description
        <textarea value={draft.description} onChange={(event) => onChangeDraft({ ...draft, description: event.target.value })} />
      </label>

      <div>
        <div className="section-heading">
          <h4>Aliases</h4>
          <button type="button" onClick={() => onChangeDraft({ ...draft, aliases: [...draft.aliases, ''] })}>Add alias</button>
        </div>
        {draft.aliases.map((alias, index) => (
          <div key={`alias-${index}`} className="metadata-row">
            <label>
              Alias {index + 1}
              <input value={alias} onChange={(event) => updateAlias(index, event.target.value)} />
            </label>
            <div>
              <button type="button" onClick={() => onChangeDraft({ ...draft, aliases: moveItem(draft.aliases, index, index - 1) })} disabled={index === 0}>Move alias {index + 1} up</button>
              <button type="button" onClick={() => onChangeDraft({ ...draft, aliases: moveItem(draft.aliases, index, index + 1) })} disabled={index === draft.aliases.length - 1}>Move alias {index + 1} down</button>
              <button type="button" onClick={() => onChangeDraft({ ...draft, aliases: draft.aliases.filter((_, currentIndex) => currentIndex !== index) })}>Remove alias {index + 1}</button>
            </div>
          </div>
        ))}
      </div>

      <div>
        <div className="section-heading">
          <h4>Tags</h4>
          <button type="button" onClick={() => onChangeDraft({ ...draft, tags: [...draft.tags, ''] })}>Add tag</button>
        </div>
        {draft.tags.map((tag, index) => (
          <div key={`tag-${index}`} className="metadata-row">
            <label>
              Tag {index + 1}
              <input value={tag} onChange={(event) => updateTag(index, event.target.value)} />
            </label>
            <div>
              <button type="button" onClick={() => onChangeDraft({ ...draft, tags: moveItem(draft.tags, index, index - 1) })} disabled={index === 0}>Move tag {index + 1} up</button>
              <button type="button" onClick={() => onChangeDraft({ ...draft, tags: moveItem(draft.tags, index, index + 1) })} disabled={index === draft.tags.length - 1}>Move tag {index + 1} down</button>
              <button type="button" onClick={() => onChangeDraft({ ...draft, tags: draft.tags.filter((_, currentIndex) => currentIndex !== index) })}>Remove tag {index + 1}</button>
            </div>
          </div>
        ))}
      </div>

      <div>
        <div className="section-heading">
          <h4>Metadata</h4>
          <button type="button" onClick={() => onChangeDraft({ ...draft, metadata: [...draft.metadata, { key: '', value: '' }] })}>Add metadata</button>
        </div>
        {draft.metadata.map((row, index) => (
          <div key={`metadata-${index}`} className="metadata-row">
            <label>
              Metadata key {index + 1}
              <input value={row.key} onChange={(event) => updateMetadata(index, 'key', event.target.value)} />
            </label>
            <label>
              Metadata value {index + 1}
              <input value={row.value} onChange={(event) => updateMetadata(index, 'value', event.target.value)} />
            </label>
            <button type="button" onClick={() => onChangeDraft({ ...draft, metadata: draft.metadata.filter((_, currentIndex) => currentIndex !== index) })}>Remove metadata {index + 1}</button>
          </div>
        ))}
      </div>
    </>
  )
}
