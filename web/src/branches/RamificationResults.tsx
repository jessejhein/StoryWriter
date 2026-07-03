/**
 * RamificationResults.tsx
 *
 * Advisory ramification findings with no accept or apply authority.
 */

import type { RamificationResponse } from './branchTypes'

type Props = {
  result: RamificationResponse | null
  loading: boolean
  error: string | null
  stale: boolean
}

const severityOrder = { high: 0, medium: 1, low: 2 } as const

export default function RamificationResults({ result, loading, error, stale }: Props) {
  if (loading) {
    return <p className="branch-message">Analyzing ramifications…</p>
  }
  if (error) {
    return <p className="error" role="alert">{error}</p>
  }
  if (stale) {
    return <p className="branch-message">Stale analysis. Run analysis again after reviewing the current comparison.</p>
  }
  if (!result) {
    return <p className="branch-message">No ramification analysis yet. Review the comparison, then run analysis explicitly.</p>
  }

  const findings = [...result.findings].sort((left, right) => {
    const severityDiff = severityOrder[left.severity] - severityOrder[right.severity]
    if (severityDiff !== 0) {
      return severityDiff
    }
    return left.category.localeCompare(right.category)
  })

  return (
    <section aria-labelledby="ramification-results-heading" className="branch-ramification">
      <h3 id="ramification-results-heading">Ramification analysis</h3>
      <p className="section-note">Analysis does not edit files. Findings are advisory review notes only.</p>
      <p className="branch-ramification-summary">{result.summary}</p>
      <ul className="branch-findings-list">
        {findings.map((finding) => (
          <li className={`branch-finding branch-finding-${finding.severity}`} key={`${finding.category}-${finding.title}`}>
            <div className="branch-finding-header">
              <strong>{finding.title}</strong>
              <span>{finding.category}</span>
              <span>{finding.severity} severity</span>
            </div>
            <p>{finding.explanation}</p>
            <ul className="branch-finding-paths">
              {finding.affected_paths.map((path) => <li key={path}><code>{path}</code></li>)}
            </ul>
            <p><span className="section-label">Recommended review</span> {finding.recommended_action}</p>
          </li>
        ))}
      </ul>
    </section>
  )
}