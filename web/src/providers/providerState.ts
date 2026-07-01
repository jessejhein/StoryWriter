import type { ProviderProfile } from '../api'

export function normalizeProfiles(profiles: ProviderProfile[]) {
  return JSON.stringify(profiles
    .map((profile) => ({ ...profile, readiness: undefined }))
    .sort((left, right) => left.id.localeCompare(right.id)))
}
