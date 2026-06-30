import type { CodexEntry, CodexEntryType } from '../api'

type Props = {
  entries: CodexEntry[]
  onNewEntry: () => void
  onSelectEntry: (entryID: string) => void
}

export default function CodexEntryList({ entries, onNewEntry, onSelectEntry }: Props) {
  const groupedEntries = entries.reduce<Record<CodexEntryType, CodexEntry[]>>((groups, entry) => {
    groups[entry.type].push(entry)
    return groups
  }, { character: [], location: [], lore: [], custom: [] })

  return (
    <aside>
      <div className="section-heading">
        <h2>Codex</h2>
        <button type="button" onClick={onNewEntry}>New entry</button>
      </div>
      {entries.length === 0 ? (
        <p>No Codex entries yet.</p>
      ) : (
        (Object.entries(groupedEntries) as [CodexEntryType, CodexEntry[]][]).map(([entryType, grouped]) => (
          grouped.length === 0 ? null : (
            <section key={entryType}>
              <h3>{entryType}</h3>
              <ul>
                {grouped.map((entry) => (
                  <li key={entry.id}>
                    <button type="button" value={entry.id} onClick={() => onSelectEntry(entry.id)}>{entry.name}</button>
                  </li>
                ))}
              </ul>
            </section>
          )
        ))
      )}
    </aside>
  )
}
