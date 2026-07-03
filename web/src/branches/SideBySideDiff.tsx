/**
 * SideBySideDiff.tsx
 *
 * Accessible read-only side-by-side canon and experiment comparison panes.
 */

import { useEffect, useMemo, useRef } from 'react'
import type { ChangedFileStatus, FileSide } from './branchTypes'
import { alignLines } from './lineDiff'

type Props = {
  path: string
  status: ChangedFileStatus
  mainHead: string
  experimentHead: string
  fingerprint: string
  canon: FileSide
  experiment: FileSide
  loading: boolean
  stale: boolean
  error: string | null
}

function screenReaderLabel(kind: string): string {
  switch (kind) {
    case 'added':
      return 'Added line'
    case 'deleted':
      return 'Deleted line'
    case 'modified':
      return 'Modified line'
    default:
      return 'Unchanged line'
  }
}

export default function SideBySideDiff({
  path,
  status,
  mainHead,
  experimentHead,
  fingerprint,
  canon,
  experiment,
  loading,
  stale,
  error,
}: Props) {
  const canonScrollerRef = useRef<HTMLDivElement>(null)
  const experimentScrollerRef = useRef<HTMLDivElement>(null)
  const syncingRef = useRef(false)

  const diff = useMemo(
    () => alignLines(canon.exists ? canon.text : '', experiment.exists ? experiment.text : ''),
    [canon.exists, canon.text, experiment.exists, experiment.text],
  )

  useEffect(() => {
    const canonScroller = canonScrollerRef.current
    const experimentScroller = experimentScrollerRef.current
    if (!canonScroller || !experimentScroller || diff.mode !== 'highlighted') {
      return
    }

    function syncFromCanon() {
      if (syncingRef.current || !canonScroller || !experimentScroller) {
        return
      }
      syncingRef.current = true
      experimentScroller.scrollTop = canonScroller.scrollTop
      syncingRef.current = false
    }

    function syncFromExperiment() {
      if (syncingRef.current || !canonScroller || !experimentScroller) {
        return
      }
      syncingRef.current = true
      canonScroller.scrollTop = experimentScroller.scrollTop
      syncingRef.current = false
    }

    canonScroller.addEventListener('scroll', syncFromCanon)
    experimentScroller.addEventListener('scroll', syncFromExperiment)
    return () => {
      canonScroller.removeEventListener('scroll', syncFromCanon)
      experimentScroller.removeEventListener('scroll', syncFromExperiment)
    }
  }, [diff.mode, path])

  if (loading) {
    return <p className="branch-message">Loading comparison…</p>
  }
  if (error) {
    return <p className="error" role="alert">{error}</p>
  }
  if (stale) {
    return <p className="branch-message">Stale comparison. Select the file again to refresh.</p>
  }

  return (
    <section aria-labelledby="branch-comparison-heading" className="branch-comparison">
      <div className="branch-comparison-meta">
        <h3 id="branch-comparison-heading">{path}</h3>
        <p><span className="section-label">Status</span> <strong>{status}</strong></p>
        <p><span className="section-label">Fingerprint</span> <code>{fingerprint}</code></p>
        <p><span className="section-label">Main head</span> <code>{mainHead}</code></p>
        <p><span className="section-label">Experiment head</span> <code>{experimentHead}</code></p>
      </div>

      {diff.mode === 'fallback' ? (
        <div className="branch-diff-fallback">
          <p className="branch-message">{diff.message}</p>
          <div className="branch-diff-grid branch-diff-grid-fallback">
            <FallbackPane label="Canon (main)" exists={canon.exists} text={canon.text} missingLabel="Missing on canon" />
            <FallbackPane label="Experiment" exists={experiment.exists} text={experiment.text} missingLabel="Missing on experiment" />
          </div>
        </div>
      ) : (
        <div className="branch-diff-grid">
          <div className="branch-diff-pane">
            <div className="branch-diff-pane-header">
              <h4>Canon (main)</h4>
              {!canon.exists && <span className="branch-missing-label">Missing on canon</span>}
            </div>
            <div aria-label="Canon comparison pane" className="branch-diff-scroller" ref={canonScrollerRef}>
              {diff.rows.map((row, index) => (
                <div className={`branch-diff-row branch-diff-row-${row.kind}`} key={`canon-${index}`}>
                  <span className="branch-diff-line-number">{row.canonLine ?? ''}</span>
                  <span className="branch-diff-text">{row.canonText ?? (row.kind === 'added' ? 'Missing on canon' : '')}</span>
                  <span className="sr-only">{screenReaderLabel(row.kind)}</span>
                </div>
              ))}
            </div>
          </div>
          <div className="branch-diff-pane">
            <div className="branch-diff-pane-header">
              <h4>Experiment</h4>
              {!experiment.exists && <span className="branch-missing-label">Missing on experiment</span>}
            </div>
            <div aria-label="Experiment comparison pane" className="branch-diff-scroller" ref={experimentScrollerRef}>
              {diff.rows.map((row, index) => (
                <div className={`branch-diff-row branch-diff-row-${row.kind}`} key={`experiment-${index}`}>
                  <span className="branch-diff-line-number">{row.branchLine ?? ''}</span>
                  <span className="branch-diff-text">{row.branchText ?? (row.kind === 'deleted' ? 'Missing on experiment' : '')}</span>
                  <span className="sr-only">{screenReaderLabel(row.kind)}</span>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}
    </section>
  )
}

function FallbackPane({
  label,
  exists,
  text,
  missingLabel,
}: {
  label: string
  exists: boolean
  text: string
  missingLabel: string
}) {
  return (
    <div className="branch-diff-pane">
      <div className="branch-diff-pane-header">
        <h4>{label}</h4>
        {!exists && <span className="branch-missing-label">{missingLabel}</span>}
      </div>
      <pre className="branch-diff-fallback-text">{exists ? text : missingLabel}</pre>
    </div>
  )
}
