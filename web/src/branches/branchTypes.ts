/**
 * branchTypes.ts
 *
 * Transport and UI state types for Milestone 8 branch experiments,
 * comparison, ramification analysis, promotion, and discard.
 */

/** ChangedFileStatus is the comparison status for one project-relative path. */
export type ChangedFileStatus = 'added' | 'modified' | 'deleted'

/** ExperimentSummary is one managed what-if experiment in the branch list. */
export type ExperimentSummary = {
  experiment_id: string
  branch_name: string
  head: string
  display_name: string
}

/** BranchStatusResponse reports the active branch and current commit heads. */
export type BranchStatusResponse = {
  active_branch: string
  active_kind: 'canon' | 'experiment'
  main_head: string
  experiment_head: string | null
  active_experiment_id: string | null
  worktree_clean: boolean
}

/** ExperimentsListResponse lists managed experiments deterministically. */
export type ExperimentsListResponse = {
  experiments: ExperimentSummary[]
}

/** CreateExperimentRequest names a new experiment from current main. */
export type CreateExperimentRequest = {
  name: string
}

/** SwitchBranchRequest switches between main and a managed experiment. */
export type SwitchBranchRequest =
  | { target: 'main' }
  | { target: string; expected_head: string }

/** ChangedFile is one path record in a branch comparison inventory. */
export type ChangedFile = {
  path: string
  status: ChangedFileStatus
}

/** ComparisonResponse is the full tree comparison between main and an experiment. */
export type ComparisonResponse = {
  experiment_id: string
  branch_name: string
  main_head: string
  experiment_head: string
  base_head: string
  fingerprint: string
  files: ChangedFile[]
}

/** FileSide is one bounded UTF-8 text blob or an absent add/delete side. */
export type FileSide = {
  exists: boolean
  text: string
}

/** FileComparisonResponse is the side-by-side source for one changed file. */
export type FileComparisonResponse = {
  path: string
  status: ChangedFileStatus
  main_head: string
  experiment_head: string
  fingerprint: string
  canon: FileSide
  experiment: FileSide
}

/** RamificationCategory classifies one advisory finding. */
export type RamificationCategory = 'plot' | 'character' | 'continuity' | 'timeline' | 'world' | 'structure'

/** RamificationSeverity ranks one advisory finding. */
export type RamificationSeverity = 'low' | 'medium' | 'high'

/** RamificationFinding is one structured consequence in an analysis result. */
export type RamificationFinding = {
  category: RamificationCategory
  severity: RamificationSeverity
  title: string
  explanation: string
  affected_paths: string[]
  recommended_action: string
}

/** RamificationRequest triggers explicit ramification analysis. */
export type RamificationRequest = {
  goal: string
  profile_id: string
  model: string
  expected_main_head: string
  expected_experiment_head: string
  comparison_fingerprint: string
}

/** RamificationManifest is the redacted provider input summary. */
export type RamificationManifest = {
  main_head: string
  experiment_head: string
  fingerprint: string
  changed_file_count: number
  included_paths: string[]
  estimated_input_bytes: number
}

/** RamificationResponse is the strict transient analysis payload. */
export type RamificationResponse = {
  summary: string
  findings: RamificationFinding[]
  provider: {
    profile_id: string
    type: 'openai_compatible' | 'ollama'
    model: string
  }
  manifest: RamificationManifest
}

/** PromoteRequest promotes selected whole files onto main. */
export type PromoteRequest = {
  paths: string[]
  expected_main_head: string
  expected_experiment_head: string
  comparison_fingerprint: string
}

/** PromoteResponse reports the resulting main head and promoted paths. */
export type PromoteResponse = {
  main_head: string
  promoted_paths: string[]
  experiment_id: string
}

/** PromoteConflictResponse names safe conflicting paths on main. */
export type PromoteConflictResponse = {
  conflicting_paths: string[]
}

/** DiscardRequest deletes one experiment with optimistic head protection. */
export type DiscardRequest = {
  expected_experiment_head: string
}

/** DiscardResponse reports main after discard completes. */
export type DiscardResponse = {
  main_head: string
}

/** DiffRow is one aligned side-by-side comparison row for display only. */
export type DiffRow = {
  kind: 'equal' | 'added' | 'deleted' | 'modified'
  canonLine: number | null
  canonText: string | null
  branchLine: number | null
  branchText: string | null
}

/** BranchContextKey identifies the branch-sensitive UI snapshot for stale checks. */
export type BranchContextKey = {
  projectID: string
  experimentID: string | null
  mainHead: string
  experimentHead: string | null
  fingerprint: string
  selectedPath: string | null
}

/** LineDiffResult is the bounded line alignment output for one file pair. */
export type LineDiffResult =
  | { mode: 'highlighted'; rows: DiffRow[] }
  | { mode: 'fallback'; message: string }