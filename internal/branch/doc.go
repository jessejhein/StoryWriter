// Package branch owns what-if experiment policy, comparison, promotion,
// discard, and ramification orchestration for Milestone 8.
//
// Invariants:
//   - main is the fixed canon branch.
//   - Managed experiments live under the branch/ ref namespace.
//   - Comparison reads Git objects without switching branches.
//   - Branch-changing operations require a clean worktree and the shared
//     mutation coordinator write lock.
package branch
