/**
 * actionState.ts
 *
 * Pure Milestone 7 action workflow state transitions for previews, runs,
 * invitations, and stale-response protection.
 */

import type {
  ActionWorkflowScope,
  ContextManifest,
  ContextPreviewResponse,
  FollowUpInvitation,
  RunActionResponse,
} from './actionTypes'

export type ActionPreviewState = {
  scope: ActionWorkflowScope
  targetRevision: string
  manifest: ContextManifest
  requestVersion: number
} | null

export type ActionRunState = {
  response: RunActionResponse
  requestVersion: number
} | null

export type InvitationRunState = {
  invitationID: string
  response: RunActionResponse
  requestVersion: number
} | null

export type ActionWorkflowState = {
  preview: ActionPreviewState
  previewLoading: boolean
  previewError: string | null
  run: ActionRunState
  runLoading: boolean
  runError: string | null
  invitations: FollowUpInvitation[]
  invitationRun: InvitationRunState
  invitationLoadingID: string | null
  invitationError: string | null
  requestVersion: number
}

export const initialActionWorkflowState = (): ActionWorkflowState => ({
  preview: null,
  previewLoading: false,
  previewError: null,
  run: null,
  runLoading: false,
  runError: null,
  invitations: [],
  invitationRun: null,
  invitationLoadingID: null,
  invitationError: null,
  requestVersion: 0,
})

export function bumpActionRequestVersion(state: ActionWorkflowState): ActionWorkflowState {
  return {
    ...state,
    requestVersion: state.requestVersion + 1,
    preview: null,
    previewError: null,
    run: null,
    runError: null,
    invitationRun: null,
    invitationError: null,
    invitations: [],
  }
}

export function invalidatePreviewForRevision(
  state: ActionWorkflowState,
  targetRevision: string,
): ActionWorkflowState {
  if (!state.preview || state.preview.targetRevision === targetRevision) {
    return state
  }
  return {
    ...state,
    preview: null,
    previewError: null,
  }
}

export function beginPreview(state: ActionWorkflowState): ActionWorkflowState {
  return {
    ...state,
    previewLoading: true,
    previewError: null,
  }
}

export function applyPreviewSuccess(
  state: ActionWorkflowState,
  scope: ActionWorkflowScope,
  preview: ContextPreviewResponse,
  requestVersion: number,
): ActionWorkflowState {
  if (requestVersion !== state.requestVersion) {
    return state
  }
  return {
    ...state,
    previewLoading: false,
    previewError: null,
    preview: {
      scope,
      targetRevision: preview.target_revision,
      manifest: preview.manifest,
      requestVersion,
    },
  }
}

export function applyPreviewFailure(
  state: ActionWorkflowState,
  message: string,
  requestVersion: number,
): ActionWorkflowState {
  if (requestVersion !== state.requestVersion) {
    return state
  }
  return {
    ...state,
    previewLoading: false,
    previewError: message,
    preview: null,
  }
}

export function beginRun(state: ActionWorkflowState): ActionWorkflowState {
  return {
    ...state,
    runLoading: true,
    runError: null,
  }
}

export function applyRunSuccess(
  state: ActionWorkflowState,
  response: RunActionResponse,
  requestVersion: number,
): ActionWorkflowState {
  if (requestVersion !== state.requestVersion) {
    return state
  }
  return {
    ...state,
    runLoading: false,
    runError: null,
    run: { response, requestVersion },
    invitations: response.follow_up_invitations ?? state.invitations,
  }
}

export function applyRunFailure(
  state: ActionWorkflowState,
  message: string,
  requestVersion: number,
): ActionWorkflowState {
  if (requestVersion !== state.requestVersion) {
    return state
  }
  return {
    ...state,
    runLoading: false,
    runError: message,
  }
}

export function clearRunPreview(state: ActionWorkflowState): ActionWorkflowState {
  return {
    ...state,
    run: null,
    runError: null,
    invitationRun: null,
    invitationError: null,
  }
}

export function setFollowUpInvitations(
  state: ActionWorkflowState,
  invitations: FollowUpInvitation[],
): ActionWorkflowState {
  return {
    ...state,
    invitations,
  }
}

export function beginInvitationRun(
  state: ActionWorkflowState,
  invitationID: string,
): ActionWorkflowState {
  return {
    ...state,
    invitationLoadingID: invitationID,
    invitationError: null,
  }
}

export function applyInvitationRunSuccess(
  state: ActionWorkflowState,
  invitationID: string,
  response: RunActionResponse,
  requestVersion: number,
): ActionWorkflowState {
  if (requestVersion !== state.requestVersion) {
    return state
  }
  return {
    ...state,
    invitationLoadingID: null,
    invitationError: null,
    invitationRun: { invitationID, response, requestVersion },
    run: { response, requestVersion },
    invitations: response.follow_up_invitations ?? state.invitations,
  }
}

export function applyInvitationRunFailure(
  state: ActionWorkflowState,
  message: string,
  requestVersion: number,
): ActionWorkflowState {
  if (requestVersion !== state.requestVersion) {
    return state
  }
  return {
    ...state,
    invitationLoadingID: null,
    invitationError: message,
  }
}

export function actionUsesBrowserStorage(): boolean {
  if (typeof window === 'undefined') {
    return false
  }
  for (let index = 0; index < window.localStorage.length; index += 1) {
    const key = window.localStorage.key(index)
    if (key && (key.includes('action') || key.includes('invitation') || key.includes('preview'))) {
      return true
    }
  }
  for (let index = 0; index < window.sessionStorage.length; index += 1) {
    const key = window.sessionStorage.key(index)
    if (key && (key.includes('action') || key.includes('invitation') || key.includes('preview'))) {
      return true
    }
  }
  return false
}