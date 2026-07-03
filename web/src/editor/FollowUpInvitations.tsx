/**
 * FollowUpInvitations.tsx
 *
 * Renders explicit follow-up invitations that require a separate author Run
 * request before any provider call occurs.
 */

import type { FollowUpInvitation } from './actionTypes'

type Props = {
  invitations: FollowUpInvitation[]
  loadingInvitationID?: string | null
  error?: string | null
  onRun: (invitation: FollowUpInvitation) => void
}

/**
 * FollowUpInvitations
 *
 * Shows broader-scope invitations with provenance and explicit run controls.
 */
export default function FollowUpInvitations({
  invitations,
  loadingInvitationID = null,
  error = null,
  onRun,
}: Props) {
  if (invitations.length === 0) {
    return null
  }

  return (
    <section className="follow-up-invitations" aria-label="Follow-up invitations" role="region">
      <h3>Follow-up actions</h3>
      <p className="section-note">
        These suggestions have not called a model yet. Choose Run only when you want the broader scope.
      </p>
      {error && <p className="error" role="alert">{error}</p>}
      <ul>
        {invitations.map((invitation) => (
          <li key={invitation.invitation_id} className="follow-up-card">
            <div>
              <strong>{invitation.agent_id}</strong>
              <span className="section-note"> scope: {invitation.scope}</span>
            </div>
            <p className="section-note">
              Parent run <code>{invitation.parent_run_id}</code>. Relationship: {invitation.relationship}.
            </p>
            <button
              type="button"
              onClick={() => onRun(invitation)}
              disabled={loadingInvitationID !== null}
              aria-busy={loadingInvitationID === invitation.invitation_id}
            >
              {loadingInvitationID === invitation.invitation_id ? 'Running suggested action...' : 'Run suggested action'}
            </button>
          </li>
        ))}
      </ul>
    </section>
  )
}