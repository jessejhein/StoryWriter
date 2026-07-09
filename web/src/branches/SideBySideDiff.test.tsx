// BDD Scenario: 8.2.2 - Show side-by-side text
// Requirements: M8-R08
// Test purpose: verify accessible read-only side-by-side comparison rendering.

import { fireEvent, render, screen } from '@testing-library/react'
import { expect, test } from 'vitest'
import SideBySideDiff from './SideBySideDiff'

const mainHead = 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'
const experimentHead = 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb'
const fingerprint = `sha256:${'c'.repeat(64)}`

// Test: Canon/main and Experiment labels, path, status, and commit identifiers.
// Requirements: M8-R08.
test('shows canon and experiment labels with path status and commit ids', () => {
  render(
    <SideBySideDiff
      path="scenes/scn_0123456789abcdef0123.md"
      status="modified"
      mainHead={mainHead}
      experimentHead={experimentHead}
      fingerprint={fingerprint}
      canon={{ exists: true, text: 'alpha\nbeta' }}
      experiment={{ exists: true, text: 'alpha\ngamma' }}
      loading={false}
      stale={false}
      error={null}
    />,
  )

  expect(screen.getByText('Canon (main)')).toBeInTheDocument()
  expect(screen.getByText('Experiment')).toBeInTheDocument()
  expect(screen.getByText('scenes/scn_0123456789abcdef0123.md')).toBeInTheDocument()
  expect(screen.getAllByText(/modified/i).length).toBeGreaterThan(0)
  expect(screen.getAllByText(mainHead).length).toBeGreaterThan(0)
  expect(screen.getAllByText(experimentHead).length).toBeGreaterThan(0)
})

// Test: added/deleted/modified/equal rows, line numbers, missing-side labels, and screen-reader status.
// Requirements: M8-R08.
test('renders aligned rows with accessible change indicators and missing-side labels', () => {
  render(
    <SideBySideDiff
      path="scenes/scn_added.md"
      status="added"
      mainHead={mainHead}
      experimentHead={experimentHead}
      fingerprint={fingerprint}
      canon={{ exists: false, text: '' }}
      experiment={{ exists: true, text: 'new scene line' }}
      loading={false}
      stale={false}
      error={null}
    />,
  )

  expect(screen.getAllByText('Missing on canon').length).toBeGreaterThan(0)
  expect(screen.getByText('new scene line')).toBeInTheDocument()
  expect(screen.getAllByText('1').length).toBeGreaterThan(0)
  expect(screen.getAllByText(/added line/i).length).toBeGreaterThan(0)
})

// Test: both panes are read-only and prose is rendered as text.
// Requirements: M8-R08.
test('renders prose as text without editable controls', () => {
  render(
    <SideBySideDiff
      path="scenes/scn_0123456789abcdef0123.md"
      status="modified"
      mainHead={mainHead}
      experimentHead={experimentHead}
      fingerprint={fingerprint}
      canon={{ exists: true, text: '<script>alert(1)</script>' }}
      experiment={{ exists: true, text: '<script>alert(2)</script>' }}
      loading={false}
      stale={false}
      error={null}
    />,
  )

  expect(screen.getByText('<script>alert(1)</script>')).toBeInTheDocument()
  expect(screen.getByText('<script>alert(2)</script>')).toBeInTheDocument()
  expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
})

// Test: loading, error, stale, and fallback states.
// Requirements: M8-R08.
test('shows loading error stale and fallback states', () => {
  const { rerender } = render(
    <SideBySideDiff
      path="scenes/scn_0123456789abcdef0123.md"
      status="modified"
      mainHead={mainHead}
      experimentHead={experimentHead}
      fingerprint={fingerprint}
      canon={{ exists: true, text: 'alpha' }}
      experiment={{ exists: true, text: 'beta' }}
      loading
      stale={false}
      error={null}
    />,
  )
  expect(screen.getByText(/loading comparison/i)).toBeInTheDocument()

  rerender(
    <SideBySideDiff
      path="scenes/scn_0123456789abcdef0123.md"
      status="modified"
      mainHead={mainHead}
      experimentHead={experimentHead}
      fingerprint={fingerprint}
      canon={{ exists: true, text: 'alpha' }}
      experiment={{ exists: true, text: 'beta' }}
      loading={false}
      stale={false}
      error="Comparison failed"
    />,
  )
  expect(screen.getByRole('alert')).toHaveTextContent('Comparison failed')

  rerender(
    <SideBySideDiff
      path="scenes/scn_0123456789abcdef0123.md"
      status="modified"
      mainHead={mainHead}
      experimentHead={experimentHead}
      fingerprint={fingerprint}
      canon={{ exists: true, text: 'alpha' }}
      experiment={{ exists: true, text: 'beta' }}
      loading={false}
      stale
      error={null}
    />,
  )
  expect(screen.getByText(/stale comparison/i)).toBeInTheDocument()

  const huge = `${'line\n'.repeat(2500)}end`
  rerender(
    <SideBySideDiff
      path="scenes/scn_0123456789abcdef0123.md"
      status="modified"
      mainHead={mainHead}
      experimentHead={experimentHead}
      fingerprint={fingerprint}
      canon={{ exists: true, text: huge }}
      experiment={{ exists: true, text: huge }}
      loading={false}
      stale={false}
      error={null}
    />,
  )
  expect(screen.getByText(/line highlighting is unavailable/i)).toBeInTheDocument()
})

// Test: fallback labels identify the side that is actually absent.
// Requirements: M8-R08.
test('labels an absent canon side correctly in fallback mode', () => {
  const huge = `${'line\n'.repeat(2500)}end`
  render(
    <SideBySideDiff
      path="scenes/scn_0123456789abcdef0123.md"
      status="added"
      mainHead={mainHead}
      experimentHead={experimentHead}
      fingerprint={fingerprint}
      canon={{ exists: false, text: '' }}
      experiment={{ exists: true, text: huge }}
      loading={false}
      stale={false}
      error={null}
    />,
  )
  expect(screen.getAllByText('Missing on canon').length).toBeGreaterThan(0)
  expect(screen.queryByText('Missing on experiment')).not.toBeInTheDocument()
})

// Test: aligned rows support practical synchronized scrolling.
// Requirements: M8-R08.
test('synchronizes vertical scrolling between aligned panes', () => {
  render(
    <SideBySideDiff
      path="scenes/scn_0123456789abcdef0123.md"
      status="modified"
      mainHead={mainHead}
      experimentHead={experimentHead}
      fingerprint={fingerprint}
      canon={{ exists: true, text: 'alpha\nbeta\ngamma' }}
      experiment={{ exists: true, text: 'alpha\nchanged\ngamma' }}
      loading={false}
      stale={false}
      error={null}
    />,
  )

  const canonScroller = screen.getByLabelText('Canon comparison pane')
  const experimentScroller = screen.getByLabelText('Experiment comparison pane')
  Object.defineProperty(canonScroller, 'scrollTop', { value: 0, writable: true })
  Object.defineProperty(experimentScroller, 'scrollTop', { value: 0, writable: true })
  fireEvent.scroll(canonScroller, { target: { scrollTop: 48 } })
  expect(experimentScroller.scrollTop).toBe(48)
})
