# Milestone 8 Test Evidence

Status: complete as of July 5, 2026.

## Baseline (M8-00)

| Work unit | Test | Assertion | Status |
| --- | --- | --- | --- |
| M8-00.3 | `go test ./... -count=1` | all packages green | recorded |
| M8-00.4 | `go test -race ./...` | all packages green | recorded |
| M8-00.5 | frontend lint/typecheck/test | 50 files, 127 tests green | recorded |
| M8-00.6 | `make check` | full check green (vite chunk warning pre-existing) | recorded |

## Milestone 8

Milestone 8 evidence is the verified suite and command set below. It reflects
the remediated live merge-base guard, promotion rollback/verification, bounded
comparison parsing, strict project validation, and branch workspace
invalidation behavior.

Backend:

- `internal/branch/experiment_history_guard_test.go`:
  `TestLoadComparisonRejectsUnrelatedCanonHistory`,
  `TestLoadComparisonUsesLiveMergeBase`
- `internal/branch/promotion_service_test.go`:
  `TestPromoteSelectedFilesUsesLiveMergeBase`,
  `TestPromoteSelectedFilesOrdersTransactionAndReturnsResult`,
  `TestPromoteSelectedFilesRollsBackEveryFailureBoundary`
- `internal/api/branch_negative_contract_test.go`:
  `TestBranchPromotionRouteMapsSubsetValidationAndInfrastructureFailures`,
  `TestBranchRouteMapsEveryContractErrorClass`,
  `TestBranchRoutesRejectOversizedBodies`,
  `TestEveryBranchRouteReturnsMethodSpecificAllow`
- `internal/projectcheck/validator_test.go`:
  `TestValidateProjectClassifiesInvalidCanonicalState`,
  `TestValidateProjectLeavesInfrastructureFailuresUnclassified`,
  `TestValidateProjectRejectsMalformedOutline`,
  `TestValidateProjectAcceptsValidFixture`
- `internal/app/milestone8_integration_test.go`:
  `TestMilestone8AcceptanceM833HappyPath`,
  `TestMilestone8AcceptanceM834Adversarial` subtests
  `stale_fingerprint_and_refs_rejected_before_provider_or_checkout`,
  `changed_on_main_path_conflicts_before_promotion_checkout`,
  `invalid_promotion_subset_rolls_back_main`
- `internal/app/milestone8_history_guard_integration_test.go`:
  `TestMilestone8RewrittenExperimentHistoryFailsClosed`
- `internal/gitstore/tree_comparison_test.go`:
  `TestCompareTreesReportsAddedModifiedDeleted`,
  `TestCompareTreesRejectsSymlinkChanges`
- `internal/gitstore/blob_test.go`:
  `TestReadTextBlobWithoutCheckout`,
  `TestReadTextBlobRejectsNonRegularEntriesAndDoesNotHideGitErrors`
- `internal/gitstore/promotion_test.go`:
  `TestApplyPathsAndCommitPromotion`,
  `TestCommitPromotionRejectsUnexpectedStagedPaths`
- `internal/gitstore/promotion_message_test.go`:
  `TestFormatPromotionMessageExactBytes`
- `internal/gitstore/branch_switch_test.go`:
  `TestCreateAndSwitchFromMain`,
  `TestCreateAndSwitchRejectsDirtyWorktree`,
  `TestSwitchExperimentRejectsStaleExpectedHead`
- `internal/gitstore/branch_delete_test.go`:
  `TestDeleteExperimentLeavesMainUntouched`
- `internal/modelchat/client_test.go`:
  `TestM8ModelchatOpenAICompatibleRequest`,
  `TestM8ModelchatOllamaRequest`,
  `TestM8ModelchatErrorMapping`
- `internal/modelchat/model_test.go`:
  `TestM8ModelMessageRolesAndContent`,
  `TestM8ModelCompleterContract`
- `internal/agent/modelchat_migration_test.go`:
  `TestM8AgentCompleteChatDelegatesToModelchat`
- `internal/extract/modelchat_migration_test.go`:
  `TestM8ExtractNoLongerImportsAgentChatTransport`
- `internal/branch/ramification_adapter_test.go`:
  `TestModelchatAnalyzerParsesStrictFindings`,
  `TestModelchatAnalyzerRejectsMalformedOutput`
- `internal/branch/ramification_service_test.go`:
  `TestAnalyzeRamificationsRejectsStaleFingerprint`,
  `TestAnalyzeRamificationsBuildsReviewedUnifiedDiffPacket`

Frontend:

- `web/src/App.branch_invalidation.test.tsx`:
  `confirms branch navigation before leaving dirty scene, codex, and import drafts`,
  `remounts branch-sensitive workspaces after switch, promotion, and discard actions`
- `web/src/branches/branchState.test.ts`:
  stale keys, selection pruning, branch-change invalidation, requested-comparison
  tracking, dirty guard, and zero browser persistence
- `web/src/branches/lineDiff.test.ts`:
  aligned rows, final-newline markers, modified pairing, complexity fallback
- `web/src/branches/SideBySideDiff.test.tsx`:
  labels, accessible indicators, read-only panes
- `web/src/branches/BranchWorkbench.lifecycle.test.tsx`:
  create/switch badges, dirty guard, state invalidation
- `web/src/branches/BranchWorkbench.comparison.test.tsx`:
  changed-file list, inactive review without checkout, stale file responses ignored
- `web/src/branches/BranchWorkbench.ramification.test.tsx`:
  explicit Analyze only, reviewed fingerprint request, findings cleared on change
- `web/src/branches/RamificationResults.test.tsx`:
  grouped findings and advisory notice
- `web/src/branches/BranchWorkbench.promotion.test.tsx`:
  whole-file summary, confirmation, conflict paths, success leaves experiment listed
- `web/src/branches/BranchWorkbench.discard.test.tsx`:
  active and inactive discard, dirty guard, stale conflicts, pending disabled state

Verification commands and results:

| Command | Result |
| --- | --- |
| `go fmt ./...` | PASS |
| `go vet ./...` | PASS |
| `go test ./... -count=1` | PASS |
| `go test -race ./...` | PASS |
| `cd web && npm run lint` | PASS |
| `cd web && npm run typecheck` | PASS |
| `cd web && npm test -- --run` | PASS, 50 files / 127 tests |
| `make check` | PASS, with the pre-existing Vite chunk warning |
| `git diff --check` | PASS |
| `git worktree list --porcelain` | inspected for extra worktrees; existing review worktree remained unchanged |
