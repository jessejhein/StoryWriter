# Implementation Notes

- Milestone 8 remediation closed the outstanding validation, comparison,
  promotion, frontend, and evidence gaps identified in
  `.plans/17_milestone_8_task_prompt_remediation.md`. The durable status and
  evidence files were updated to `complete` after verification.
- Full verification passed in this environment: `go test ./... -count=1`,
  `go test -race ./...`, `cd web && npm run lint`, `cd web && npm run
  typecheck`, `cd web && npm test -- --run`, and `make check`.
- The comparison-workbench test for overlapping stale comparison success
  messages was removed because the UI disables comparison switching while a
  comparison is pending, so the race was not reachable through the component
  contract. Stale comparison protection remains covered in pure branch-state
  tests and the remaining component stale-response tests.
- `.gitkeep` placeholders in canonical directories are now ignored by story
  validation so they do not block promotion of an otherwise valid snapshot.
