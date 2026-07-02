/**
 * ChapterReview.tsx
 *
 * Renders chapter-level editorial findings grouped by stable scene ID. Findings
 * are suggestions only and cannot mutate canon.
 */

import type { ActionFinding } from './actionTypes'

type Props = {
  findings: ActionFinding[]
}

function groupFindingsByScene(findings: ActionFinding[]): Map<string, ActionFinding[]> {
  const grouped = new Map<string, ActionFinding[]>()
  for (const finding of findings) {
    for (const sceneID of finding.scene_ids) {
      const current = grouped.get(sceneID) ?? []
      current.push(finding)
      grouped.set(sceneID, current)
    }
  }
  return new Map([...grouped.entries()].sort(([left], [right]) => left.localeCompare(right)))
}

/**
 * ChapterReview
 *
 * Displays structured editorial findings without any prose acceptance control.
 */
export default function ChapterReview({ findings }: Props) {
  if (findings.length === 0) {
    return <p className="outline-message">Chapter review returned no findings.</p>
  }

  const grouped = groupFindingsByScene(findings)

  return (
    <section className="chapter-review" aria-label="Chapter review findings" role="region">
      {Array.from(grouped.entries()).map(([sceneID, sceneFindings]) => (
        <article key={sceneID} className="chapter-review-group">
          <h3>Scene <code>{sceneID}</code></h3>
          <ul>
            {sceneFindings.map((finding) => (
              <li key={`${sceneID}:${finding.title}`}>
                <strong>{finding.title}</strong>
                <p>{finding.explanation}</p>
                {finding.follow_up_agent_ids.length > 0 && (
                  <p className="section-note">Suggested actions: {finding.follow_up_agent_ids.join(', ')}</p>
                )}
              </li>
            ))}
          </ul>
        </article>
      ))}
    </section>
  )
}