/**
 * ProviderWorkbench.tsx
 *
 * Maintains the non-secret application-level provider profile document and
 * surfaces public credential readiness without accepting secrets in the browser.
 */

import { useEffect, useMemo, useState } from 'react'
import { APIError, getProviderProfiles, saveProviderProfiles, type ProviderProfile } from '../api'
import ConfirmDialog from '../components/ConfirmDialog'
import { normalizeProfiles } from './providerState'

type Props = {
  onDirtyChange: (dirty: boolean) => void
}

type Feedback = { kind: 'saved' | 'conflict' | 'error'; message: string } | null

function createDefaultProfile(): ProviderProfile {
  return {
    id: '',
    name: '',
    type: 'openai_compatible',
    base_url: 'http://127.0.0.1:1234/v1',
    auth: { type: 'none', credential_env: '' },
    capabilities: {
      chat: true,
      streaming: false,
      structured_output: false,
      max_context_tokens: 8192,
    },
  }
}

function cloneProfiles(profiles: ProviderProfile[]) {
  return profiles.map((profile) => ({
    ...profile,
    auth: { ...profile.auth },
    capabilities: { ...profile.capabilities },
  }))
}

function validateProfiles(profiles: ProviderProfile[]): string | null {
  const profileIDs = new Set<string>()
  for (const profile of profiles) {
    if (!profile.id.trim()) return 'Profile ID is required.'
    if (!/^[a-z][a-z0-9_]{0,63}$/.test(profile.id.trim())) return 'Profile IDs must match the documented format.'
    if (profileIDs.has(profile.id.trim())) return 'Profile IDs must be unique.'
    profileIDs.add(profile.id.trim())
    if (!profile.name.trim()) return 'Profile name is required.'
    if (!profile.base_url.trim()) return 'Base URL is required.'
    if (profile.auth.type === 'bearer_env' && !/^STORYWORK_[A-Z][A-Z0-9_]{0,127}$/.test(profile.auth.credential_env)) {
      return 'Bearer auth requires a STORYWORK_* environment variable name.'
    }
    if (profile.auth.type === 'none' && profile.auth.credential_env !== '') {
      return 'No-auth profiles must keep the credential environment field empty.'
    }
    if (profile.type === 'ollama' && profile.auth.type !== 'none') {
      return 'Ollama profiles use no auth in Milestone 5.'
    }
    if (profile.capabilities.max_context_tokens < 1 || profile.capabilities.max_context_tokens > 10_000_000) {
      return 'Context token limit must be between 1 and 10,000,000.'
    }
  }
  return null
}

export default function ProviderWorkbench({ onDirtyChange }: Props) {
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [profiles, setProfiles] = useState<ProviderProfile[]>([])
  const [baselineProfiles, setBaselineProfiles] = useState<ProviderProfile[]>([])
  const [revision, setRevision] = useState<string | null>(null)
  const [baselineRevision, setBaselineRevision] = useState<string | null>(null)
  const [feedback, setFeedback] = useState<Feedback>(null)
  const [confirmReload, setConfirmReload] = useState(false)

  const dirty = useMemo(
    () => revision !== baselineRevision || normalizeProfiles(profiles) !== normalizeProfiles(baselineProfiles),
    [baselineProfiles, baselineRevision, profiles, revision],
  )
  const validationError = validateProfiles(profiles)

  useEffect(() => {
    onDirtyChange(dirty)
  }, [dirty, onDirtyChange])

  useEffect(() => {
    let cancelled = false
    void (async () => {
      try {
        const response = await getProviderProfiles()
        if (cancelled) return
        setProfiles(cloneProfiles(response.profiles))
        setBaselineProfiles(cloneProfiles(response.profiles))
        setRevision(response.revision)
        setBaselineRevision(response.revision)
        setFeedback(null)
      } catch (error) {
        if (cancelled) return
        setFeedback({ kind: 'error', message: error instanceof Error ? error.message : 'Provider settings request failed' })
      } finally {
        if (!cancelled) setLoading(false)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [])

  function patchProfile(index: number, patch: Partial<ProviderProfile>) {
    setProfiles((current) => current.map((profile, profileIndex) => {
      if (profileIndex !== index) return profile
      return { ...profile, ...patch }
    }))
    setFeedback(null)
  }

  function patchAuth(index: number, patch: Partial<ProviderProfile['auth']>) {
    setProfiles((current) => current.map((profile, profileIndex) => {
      if (profileIndex !== index) return profile
      return { ...profile, auth: { ...profile.auth, ...patch } }
    }))
    setFeedback(null)
  }

  function patchCapabilities(index: number, patch: Partial<ProviderProfile['capabilities']>) {
    setProfiles((current) => current.map((profile, profileIndex) => {
      if (profileIndex !== index) return profile
      return { ...profile, capabilities: { ...profile.capabilities, ...patch } }
    }))
    setFeedback(null)
  }

  function addProfile() {
    setProfiles((current) => [...current, createDefaultProfile()])
    setFeedback(null)
  }

  function removeProfile(index: number) {
    setProfiles((current) => current.filter((_, profileIndex) => profileIndex !== index))
    setFeedback(null)
  }

  async function save() {
    if (saving || validationError) return
    setSaving(true)
    try {
      const response = await saveProviderProfiles(cloneProfiles(profiles), baselineRevision)
      setProfiles(cloneProfiles(response.profiles))
      setBaselineProfiles(cloneProfiles(response.profiles))
      setRevision(response.revision)
      setBaselineRevision(response.revision)
      setFeedback({ kind: 'saved', message: 'Saved' })
    } catch (error) {
      if (error instanceof APIError && error.status === 409) {
        setFeedback({ kind: 'conflict', message: error.message })
      } else {
        setFeedback({ kind: 'error', message: error instanceof Error ? error.message : 'Save failed' })
      }
    } finally {
      setSaving(false)
    }
  }

  async function reloadLatest() {
    setConfirmReload(false)
    setLoading(true)
    try {
      const response = await getProviderProfiles()
      setProfiles(cloneProfiles(response.profiles))
      setBaselineProfiles(cloneProfiles(response.profiles))
      setRevision(response.revision)
      setBaselineRevision(response.revision)
      setFeedback(null)
    } catch (error) {
      setFeedback({ kind: 'error', message: error instanceof Error ? error.message : 'Reload failed' })
    } finally {
      setLoading(false)
    }
  }

  return (
    <section className="outline-shell">
      <div className="outline-meta">
        <p className="folio">Milestone 5 / Providers</p>
        <h2>Configure provider profiles without putting secrets in projects.</h2>
        <p>Keys stay in backend environment variables. The browser only edits non-secret endpoint, model-capability, and credential-reference metadata.</p>
      </div>
      <div className="outline-panel">
        <div className="outline-toolbar">
          <div>
            <span className="section-label">Provider settings</span>
            <strong>{profiles.length === 0 ? 'No profiles configured' : `${profiles.length} profile${profiles.length === 1 ? '' : 's'}`}</strong>
            <code>Revision: {baselineRevision ?? 'none'}</code>
          </div>
          <div className="actions">
            <button type="button" className="secondary" onClick={addProfile}>Add profile</button>
            <button type="button" onClick={() => void save()} disabled={loading || saving || !dirty || Boolean(validationError)}>
              {saving ? 'Saving…' : 'Save'}
            </button>
          </div>
        </div>

        {loading && <p className="outline-message">Loading provider settings...</p>}
        {!loading && profiles.length === 0 && (
          <div className="empty-outline">
            <p>No provider profiles are configured yet.</p>
            <p className="section-note">Add a local OpenAI-compatible, hosted OpenAI-compatible, or Ollama profile. API keys are never entered here; set them in the named backend environment variable.</p>
          </div>
        )}

        {feedback && (
          <div className={`editor-banner ${feedback.kind === 'saved' ? 'editor-banner-saved' : 'editor-banner-error'}`}>
            <span>{feedback.message}</span>
            {feedback.kind === 'conflict' && (
              <button type="button" className="secondary" onClick={() => dirty ? setConfirmReload(true) : void reloadLatest()}>
                Reload latest
              </button>
            )}
          </div>
        )}
        {validationError && <p className="error" role="alert">{validationError}</p>}

        <div className="outline-tree">
          {profiles.map((profile, index) => (
            <article key={`${profile.id || 'new'}-${index}`} className="outline-card provider-card">
              <div className="provider-fields">
                <label>
                  Profile ID
                  <input aria-label={`Profile ID ${index + 1}`} value={profile.id} onChange={(event) => patchProfile(index, { id: event.target.value })} />
                </label>
                <label>
                  Name
                  <input aria-label={`Profile name ${index + 1}`} value={profile.name} onChange={(event) => patchProfile(index, { name: event.target.value })} />
                </label>
                <label>
                  Type
                  <select aria-label={`Provider type ${index + 1}`} value={profile.type} onChange={(event) => {
                    const type = event.target.value as ProviderProfile['type']
                    patchProfile(index, { type, base_url: type === 'ollama' ? 'http://127.0.0.1:11434' : profile.base_url })
                    if (type === 'ollama') patchAuth(index, { type: 'none', credential_env: '' })
                  }}>
                    <option value="openai_compatible">OpenAI-compatible</option>
                    <option value="ollama">Ollama</option>
                  </select>
                </label>
                <label>
                  Base URL
                  <input aria-label={`Base URL ${index + 1}`} value={profile.base_url} onChange={(event) => patchProfile(index, { base_url: event.target.value })} />
                </label>
                <label>
                  Auth
                  <select aria-label={`Auth type ${index + 1}`} value={profile.auth.type} onChange={(event) => {
                    const authType = event.target.value as ProviderProfile['auth']['type']
                    patchAuth(index, { type: authType, credential_env: authType === 'none' ? '' : profile.auth.credential_env })
                  }} disabled={profile.type === 'ollama'}>
                    <option value="none">None</option>
                    <option value="bearer_env">Bearer env</option>
                  </select>
                </label>
                <label>
                  Credential environment variable
                  <input
                    aria-label={`Credential environment variable ${index + 1}`}
                    value={profile.auth.credential_env}
                    disabled={profile.auth.type === 'none'}
                    onChange={(event) => patchAuth(index, { credential_env: event.target.value })}
                    placeholder="STORYWORK_HOSTED_API_KEY"
                  />
                </label>
                <label>
                  Max context tokens
                  <input
                    aria-label={`Max context tokens ${index + 1}`}
                    type="number"
                    min={1}
                    value={profile.capabilities.max_context_tokens}
                    onChange={(event) => patchCapabilities(index, { max_context_tokens: Number(event.target.value) || 0 })}
                  />
                </label>
                <label className="checkbox-field">
                  <input
                    type="checkbox"
                    checked={profile.capabilities.chat}
                    onChange={(event) => patchCapabilities(index, { chat: event.target.checked })}
                  />
                  Chat capability
                </label>
                <label className="checkbox-field">
                  <input
                    type="checkbox"
                    checked={profile.capabilities.streaming}
                    onChange={(event) => patchCapabilities(index, { streaming: event.target.checked })}
                  />
                  Declares streaming
                </label>
                <label className="checkbox-field">
                  <input
                    type="checkbox"
                    checked={profile.capabilities.structured_output}
                    onChange={(event) => patchCapabilities(index, { structured_output: event.target.checked })}
                  />
                  Declares structured output
                </label>
                <p className="section-note">Readiness: {profile.readiness ?? 'not saved yet'}. API keys are supplied by the backend environment variable named above and are never entered in this browser form.</p>
              </div>
              <div className="scene-actions">
                <button type="button" className="secondary" onClick={() => removeProfile(index)}>Remove</button>
              </div>
            </article>
          ))}
        </div>

        <ConfirmDialog
          open={confirmReload}
          title="Discard local provider draft?"
          message="Reloading the latest provider settings will discard unsaved local edits."
          confirmLabel="Discard draft"
          onConfirm={() => void reloadLatest()}
          onCancel={() => setConfirmReload(false)}
        />
      </div>
    </section>
  )
}
