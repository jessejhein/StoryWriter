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
const severityLabels = {
  high: 'High severity',
  medium: 'Medium severity',
  low: 'Low severity',
} as const
const categoryOrder = ['plot', 'character', 'continuity', 'timeline', 'world', 'structure'] as const

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

  const sortedFindings = [...result.findings].sort((left, right) => {
    const severityDiff = severityOrder[left.severity] - severityOrder[right.severity]
    if (severityDiff !== 0) {
      return severityDiff
    }
    const leftCategory = categoryOrder.indexOf(left.category)
    const rightCategory = categoryOrder.indexOf(right.category)
    if (leftCategory !== rightCategory) {
      return leftCategory - rightCategory
    }
    return left.title.localeCompare(right.title)
  })
  const severityGroups = (['high', 'medium', 'low'] as const).map((severity) => ({
    severity,
    categories: categoryOrder
      .map((category) => ({
        category,
        items: sortedFindings.filter((finding) => finding.category === category && finding.severity === severity),
      }))
      .filter((group) => group.items.length > 0),
  })).filter((group) => group.categories.length > 0)

  return (
    <section aria-labelledby="ramification-results-heading" className="branch-ramification">
      <h3 id="ramification-results-heading">Ramification analysis</h3>
      <p className="section-note">Analysis does not edit files. Findings are advisory review notes only.</p>
      <p className="branch-ramification-summary">{result.summary}</p>
      {sortedFindings.length === 0 ? (
        <p className="branch-message">No findings were returned.</p>
      ) : (
        <div className="branch-findings-groups">
          {severityGroups.map((group) => (
            <section className={`branch-findings-group branch-findings-group-${group.severity}`} key={group.severity}>
              <h4>{severityLabels[group.severity]}</h4>
              {group.categories.map((categoryGroup) => (
                <section className="branch-findings-category" key={`${group.severity}-${categoryGroup.category}`}>
                  <h5>{categoryGroup.category}</h5>
                  <ul className="branch-findings-list">
                    {categoryGroup.items.map((finding) => (
                      <li className={`branch-finding branch-finding-${group.severity}`} key={`${group.severity}-${categoryGroup.category}-${finding.title}`}>
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
              ))}
            </section>
          ))}
        </div>
      )}
    </section>
  )
}
