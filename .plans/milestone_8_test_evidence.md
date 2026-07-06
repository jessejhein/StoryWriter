# Milestone 8 Test Evidence

Status: complete as of July 5, 2026.

## Baseline (M8-00)

| Work unit | Test | Assertion | Status |
| --- | --- | --- | --- |
| M8-00.3 | `go test ./... -count=1` | all packages green | recorded |
| M8-00.4 | `go test -race ./...` | all packages green | recorded |
| M8-00.5 | frontend lint/typecheck/test | 50 files, 127 tests green | recorded |
| M8-00.6 | `make check` | full check green (vite chunk warning pre-existing) | recorded |

## Requirements

| Requirement | Test file(s) | Test name(s) | Status |
| --- | --- | --- | --- |
| M8-R01 | `internal/branch/identity_test.go`; `internal/gitstore/branch_status_test.go`; `internal/api/branch_negative_contract_test.go` | `TestValidateBranchRefRejectsUnsafeRefsAndAcceptsMain`; `TestParseManagedExperimentRefRejectsReservedSlug`; `TestStatusReportsActiveBranchAndCleanliness`; `TestBranchStatusAndListRoutesMapRepositoryStateErrorsSafely`; `TestBranchStatusAndListRoutesRejectMalformedManagedRefFromRealRepository` | recorded |
| M8-R02 | `internal/gitstore/branch_switch_test.go`; `internal/branch/service_lifecycle_test.go`; `internal/app/milestone8_integration_test.go` | `TestCreateAndSwitchFromMain`; `TestCreateAndSwitchFromAnotherManagedExperiment`; `TestCreateExperimentRebuildsIndex`; `TestCreateExperimentFromActiveManagedBranchRecordsMainBase`; `TestMilestone8AcceptanceM833HappyPath` | recorded |
| M8-R03 | `internal/gitstore/branch_switch_test.go`; `internal/branch/service_lifecycle_test.go`; `internal/branch/discard_test.go` | `TestCreateAndSwitchRejectsDirtyWorktree`; `TestCreateExperimentSerializesUnderCoordinator`; `TestDiscardExperimentRejectsDirtyWorktree` | recorded |
| M8-R04 | `internal/branch/service_lifecycle_test.go`; `internal/app/milestone8_integration_test.go`; `internal/app/milestone8_history_guard_integration_test.go` | `TestCreateExperimentRecoversOnIndexFailure`; `invalid_promotion_subset_rolls_back_main`; `TestMilestone8RewrittenExperimentHistoryFailsClosed` | recorded |
| M8-R05 | `internal/gitstore/tree_comparison_test.go`; `internal/gitstore/blob_test.go`; `internal/gitstore/snapshot_test.go`; `internal/api/branch_comparison_test.go`; `internal/app/milestone8_integration_test.go`; `internal/branch/experiment_history_guard_test.go`; `internal/app/milestone8_history_guard_integration_test.go` | `TestCompareTreesReportsAddedModifiedDeleted`; `TestReadTextBlobWithoutCheckout`; `TestSnapshotPathsDistinguishesExistenceAndInspectionErrors`; `TestSnapshotPathsFailsClosedOnInvalidTreeState`; `TestBranchComparisonRoute`; `TestMilestone8AcceptanceM833HappyPath`; `TestLoadComparisonRejectsRewrittenExperimentHistory`; `TestMissingExperimentBaseFailsClosedBeforeBranchMutation`; `TestSwitchAndDiscardRejectRelatedRewrittenExperimentHistory`; `TestMilestone8RewrittenExperimentHistoryFailsClosed` | recorded |
| M8-R06 | `internal/branch/comparison_test.go`; `internal/branch/fingerprint_test.go` | `TestValidateChangedFilesSortsAndDedupes`; `TestComputeFingerprintMatchesFixture` | recorded |
| M8-R07 | `internal/branch/path_test.go`; `internal/branch/file_comparison_test.go`; `internal/api/branch_comparison_test.go` | `TestValidateProjectPathRejectsUnsafeSegments`; `TestIndexChangedFilesRejectsUnknownPath`; `TestBranchFileComparisonRouteRequiresPath` | recorded |
| M8-R08 | `web/src/branches/lineDiff.test.ts`; `web/src/branches/SideBySideDiff.test.tsx`; `web/src/branches/BranchWorkbench.comparison.test.tsx` | aligned rows and modified pairing; labels and accessible indicators; changed-file list and side-by-side load | recorded |
| M8-R09 | `internal/branch/ramification_service_test.go`; `internal/api/branch_operations_test.go`; `web/src/branches/BranchWorkbench.ramification.test.tsx`; `internal/app/milestone8_integration_test.go` | `TestAnalyzeRamificationsRejectsStaleFingerprint`; `TestBranchRamificationRouteRequiresStrictBody`; explicit Analyze only; happy-path zero repo mutation | recorded |
| M8-R10 | `internal/branch/ramification_test.go`; `internal/branch/ramification_adapter_test.go`; `web/src/branches/RamificationResults.test.tsx` | `TestParseRamificationOutputRejectsUnknownFields`; `TestModelchatAnalyzerParsesStrictFindings`; advisory notice without accept controls | recorded |
| M8-R11 | `internal/modelchat/client_test.go`; `internal/agent/modelchat_migration_test.go`; `internal/extract/modelchat_migration_test.go`; `internal/branch/ramification_adapter_test.go` | `TestM8ModelchatOpenAICompatibleRequest`; `TestM8AgentCompleteChatDelegatesToModelchat`; `TestM8ExtractNoLongerImportsAgentChatTransport`; `TestModelchatAnalyzerParsesStrictFindings` | recorded |
| M8-R12 | `internal/branch/promotion_policy_test.go`; `internal/branch/promotion_service_test.go`; `internal/branch/experiment_history_guard_test.go`; `internal/app/milestone8_integration_test.go`; `internal/app/milestone8_history_guard_integration_test.go` | `TestValidatePromotionPreflightRejectsStaleRefs`; `TestPromoteSelectedFilesRejectsConflictBeforeCheckout`; `TestPromoteSelectedFilesRejectsRewrittenExperimentHistoryBeforeCheckout`; `stale_fingerprint_and_refs_rejected_before_provider_or_checkout`; `TestMilestone8RewrittenExperimentHistoryFailsClosed` | recorded |
| M8-R13 | `internal/branch/promotion_policy_test.go`; `internal/gitstore/promotion_test.go`; `web/src/branches/BranchWorkbench.promotion.test.tsx` | `TestPromotionConflictsDetectsMainDivergence`; `TestApplyPathsAndCommitPromotion`; whole-file summary without hunk controls | recorded |
| M8-R14 | `internal/projectcheck/validator_test.go`; `internal/gitstore/promotion_test.go`; `internal/app/milestone8_integration_test.go` | `TestValidateProjectRejectsMalformedOutline`; `TestApplyPathsAndCommitPromotion`; happy-path one promotion commit | recorded |
| M8-R15 | `internal/app/milestone8_integration_test.go` | `invalid_promotion_subset_rolls_back_main` | recorded |
| M8-R16 | `internal/gitstore/promotion_message_test.go`; `internal/app/milestone8_integration_test.go` | `TestFormatPromotionMessageExactBytes`; happy-path promotion trailers | recorded |
| M8-R17 | `internal/branch/discard_test.go`; `internal/gitstore/branch_delete_test.go`; `internal/branch/sentinel_mapping_test.go`; `web/src/branches/BranchWorkbench.discard.test.tsx`; `internal/app/milestone8_integration_test.go`; `internal/app/milestone8_history_guard_integration_test.go` | `TestDiscardExperimentRejectsDirtyWorktree`; `TestDeleteExperimentLeavesMainUntouched`; `TestMapRepositoryErrorNoMergeBaseSentinel`; discard confirmation and dirty guard; happy-path discard; `TestMilestone8RewrittenExperimentHistoryFailsClosed` | recorded |
| M8-R18 | `web/src/branches/branchState.test.ts`; `web/src/branches/BranchWorkbench.lifecycle.test.tsx`; `web/src/branches/BranchWorkbench.discard.test.tsx` | stale keys and branch-change invalidation; dirty switch confirmation; discard dirty guard | recorded |
| M8-R19 | `internal/story/milestone8_branch_characterization_test.go`; `internal/action/milestone8_branch_characterization_test.go`; `internal/importer/milestone8_branch_characterization_test.go`; `internal/app/milestone8_integration_test.go` | scene/Codex/AI/import characterization tests; happy-path experiment mutations | recorded |
| M8-R20 | this file and `.plans/milestone_8_status.md` | evidence and status synchronized through M8-35 | recorded |

## BDD Scenarios

| Scenario | Test file(s) | Test name(s) | Status |
| --- | --- | --- | --- |
| 8.1.1 | `internal/branch/identity_test.go`; `internal/branch/service_lifecycle_test.go`; `internal/api/branch_lifecycle_test.go`; `internal/app/milestone8_integration_test.go`; `web/src/branches/BranchWorkbench.lifecycle.test.tsx` | `TestBuildAndParseManagedBranchRefRoundTrip`; `TestCreateExperimentRebuildsIndex`; `TestBranchCreateRoute`; `TestMilestone8AcceptanceM833HappyPath`; create/switch badges | recorded |
| 8.1.2 | `internal/gitstore/branch_switch_test.go`; `internal/branch/service_lifecycle_test.go`; `internal/app/milestone8_history_guard_integration_test.go` | `TestCreateAndSwitchRejectsDirtyWorktree`; `TestCreateExperimentRecoversOnIndexFailure`; `TestMilestone8RewrittenExperimentHistoryFailsClosed` | recorded |
| 8.1.3 | `internal/story/milestone8_branch_characterization_test.go`; `internal/action/milestone8_branch_characterization_test.go`; `internal/importer/milestone8_branch_characterization_test.go` | `TestMilestone8SceneSaveCommitsToCheckedOutExperimentBranch`; `TestMilestone8AcceptedAIPatchCommitsToActiveBranchOnly`; `TestMilestone8ImportReviewMutationCommitsToActiveBranchOnly` | recorded |
| 8.2.1 | `internal/gitstore/tree_comparison_test.go`; `internal/branch/fingerprint_test.go`; `internal/api/branch_comparison_test.go`; `internal/branch/experiment_history_guard_test.go`; `internal/app/milestone8_history_guard_integration_test.go`; `web/src/api.branches.test.ts` | `TestCompareTreesReportsAddedModifiedDeleted`; `TestComputeFingerprintMatchesFixture`; `TestBranchComparisonRoute`; `TestLoadComparisonRejectsRewrittenExperimentHistory`; `TestMilestone8RewrittenExperimentHistoryFailsClosed`; documented routes | recorded |
| 8.2.2 | `internal/gitstore/blob_test.go`; `internal/branch/file_comparison_test.go`; `web/src/branches/lineDiff.test.ts`; `web/src/branches/SideBySideDiff.test.tsx` | `TestReadTextBlobWithoutCheckout`; `TestIndexChangedFilesRejectsUnknownPath`; aligned rows; labels and read-only panes | recorded |
| 8.2.3 | `internal/branch/path_test.go` | `TestValidateProjectPathRejectsUnsafeSegments`; `TestValidateStrictUTF8EnforcesBounds` | recorded |
| 8.3.1 | `internal/branch/ramification_adapter_test.go`; `internal/branch/ramification_service_test.go`; `web/src/branches/BranchWorkbench.ramification.test.tsx`; `internal/app/milestone8_integration_test.go` | `TestModelchatAnalyzerParsesStrictFindings`; `TestAnalyzeRamificationsRejectsStaleFingerprint`; explicit Analyze only; happy-path explicit provider call | recorded |
| 8.3.2 | `internal/branch/ramification_test.go`; `web/src/branches/RamificationResults.test.tsx` | `TestParseRamificationOutputAcceptsZeroFindings`; grouped findings and advisory notice | recorded |
| 8.3.3 | `internal/app/milestone8_integration_test.go` | `stale_fingerprint_and_refs_rejected_before_provider_or_checkout` | recorded |
| 8.4.1 | `internal/gitstore/promotion_test.go`; `internal/gitstore/promotion_message_test.go`; `internal/branch/experiment_history_guard_test.go`; `internal/app/milestone8_integration_test.go`; `internal/app/milestone8_history_guard_integration_test.go`; `web/src/branches/BranchWorkbench.promotion.test.tsx` | `TestApplyPathsAndCommitPromotion`; `TestFormatPromotionMessageExactBytes`; `TestPromoteSelectedFilesRejectsRewrittenExperimentHistoryBeforeCheckout`; happy-path promote; `TestMilestone8RewrittenExperimentHistoryFailsClosed`; confirmation and success state | recorded |
| 8.4.2 | `internal/branch/promotion_policy_test.go`; `internal/app/milestone8_integration_test.go` | `TestPromotionConflictsDetectsMainDivergence`; `changed_on_main_path_conflicts_before_promotion_checkout` | recorded |
| 8.4.3 | `internal/projectcheck/validator_test.go`; `internal/app/milestone8_integration_test.go` | `TestValidateProjectRejectsMalformedOutline`; `invalid_promotion_subset_rolls_back_main` | recorded |
| 8.4.4 | `internal/branch/service_lifecycle_test.go`; `internal/app/milestone8_integration_test.go` | `TestCreateExperimentRecoversOnIndexFailure`; `invalid_promotion_subset_rolls_back_main` | recorded |
| 8.5.1 | `internal/gitstore/branch_delete_test.go`; `internal/app/milestone8_integration_test.go`; `web/src/branches/BranchWorkbench.discard.test.tsx` | `TestDeleteExperimentLeavesMainUntouched`; happy-path discard; confirmation flow | recorded |
| 8.5.2 | `internal/branch/discard_test.go`; `internal/api/branch_operations_test.go`; `web/src/branches/BranchWorkbench.discard.test.tsx` | `TestDiscardExperimentRejectsDirtyWorktree`; `TestBranchDiscardRoute`; stale conflicts and dirty guard | recorded |

## Acceptance commands

| Command | Result | Status |
| --- | --- | --- |
| `gofmt -w` on touched Milestone 8 remediation files | PASS | recorded |
| `go vet ./...` | PASS | recorded |
| `go test ./... -count=1` | PASS all packages | recorded |
| `go test -race ./...` | PASS all packages | recorded |
| `cd web && npm run lint` | PASS | recorded |
| `cd web && npm run typecheck` | PASS | recorded |
| `cd web && npm test -- --run` | PASS 50 files, 127 tests | recorded |
| `make check` | PASS (vite chunk warning pre-existing) | recorded |
| `git diff --check` | PASS | recorded |
| artifact/leak/process inspection | no servers, credentials, comparison prompts, or test projects left in repo | recorded |
