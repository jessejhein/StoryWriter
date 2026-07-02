/**
 * ContextPreview.tsx
 *
 * Renders a redacted Milestone 7 context manifest for author inspection before
 * an explicit provider run.
 */

import type { ContextManifest } from './actionTypes'

type Props = {
  manifest: ContextManifest
  loading?: boolean
  error?: string | null
}

/**
 * ContextPreview
 *
 * Shows scope, estimated input size, included/omitted packs, and active Codex
 * references without exposing prose or prompts.
 */
export default function ContextPreview({ manifest, loading = false, error = null }: Props) {
  if (loading) {
    return <p className="outline-message" role="status">Loading context preview...</p>
  }
  if (error) {
    return <p className="error" role="alert">{error}</p>
  }

  return (
    <section className="context-preview" aria-label="Context preview" role="region">
      <div className="context-preview-grid">
        <div>
          <span className="section-label">Scope</span>
          <strong>{manifest.scope}</strong>
        </div>
        <div>
          <span className="section-label">Estimated input</span>
          <strong>{manifest.estimated_input_tokens}</strong>
          <span className="section-note"> / {manifest.max_input_estimated_tokens} estimated tokens</span>
        </div>
        <div>
          <span className="section-label">RAG mode</span>
          <strong>{manifest.rag_mode}</strong>
        </div>
      </div>
      <p className="section-note">Estimated tokens are conservative estimates, not billed provider counts.</p>
      <div>
        <span className="section-label">Included packs</span>
        <p>{manifest.packs_used.length > 0 ? manifest.packs_used.join(', ') : 'None'}</p>
      </div>
      {manifest.packs_omitted.length > 0 && (
        <div>
          <span className="section-label">Omitted packs</span>
          <ul>
            {manifest.packs_omitted.map((item) => (
              <li key={item.pack}>{item.pack} ({item.reason})</li>
            ))}
          </ul>
        </div>
      )}
      {manifest.active_codex && manifest.active_codex.length > 0 && (
        <div>
          <span className="section-label">Active Codex entries</span>
          <ul>
            {manifest.active_codex.map((entry) => (
              <li key={entry.entry_id}>
                <code>{entry.entry_id}</code>
                {entry.applied_progression_ids.length > 0 && (
                  <span> progressions: {entry.applied_progression_ids.join(', ')}</span>
                )}
              </li>
            ))}
          </ul>
        </div>
      )}
      {manifest.outline_refs && manifest.outline_refs.length > 0 && (
        <div>
          <span className="section-label">Outline references</span>
          <p>{manifest.outline_refs.join(', ')}</p>
        </div>
      )}
    </section>
  )
}