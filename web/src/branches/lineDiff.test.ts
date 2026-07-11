// BDD Scenario: 8.2.2 - Show side-by-side text
// Requirements: M8-R08
// Test purpose: verify bounded deterministic line alignment for side-by-side display.

import { expect, test } from 'vitest'
import { alignLines, LINE_DIFF_MAX_CELLS, LINE_DIFF_MAX_LINES, NO_NEWLINE_MARKER, splitLines } from './lineDiff'

// Test: equal, insert, delete, replace, empty side, newline, and repeated-line cases.
// Requirements: M8-R08.
test('aligns equal insert delete replace empty and repeated lines with line numbers', () => {
  const equal = alignLines('alpha\nbeta', 'alpha\nbeta')
  expect(equal).toEqual({
    mode: 'highlighted',
    rows: [
      { kind: 'equal', canonLine: 1, canonText: 'alpha', branchLine: 1, branchText: 'alpha' },
      { kind: 'equal', canonLine: 2, canonText: 'beta', branchLine: 2, branchText: 'beta' },
    ],
  })

  const inserted = alignLines('alpha', 'alpha\nbeta')
  expect(inserted).toEqual({
    mode: 'highlighted',
    rows: [
      { kind: 'equal', canonLine: 1, canonText: 'alpha', branchLine: 1, branchText: 'alpha' },
      { kind: 'added', canonLine: null, canonText: null, branchLine: 2, branchText: 'beta' },
    ],
  })

  const deleted = alignLines('alpha\nbeta', 'alpha')
  expect(deleted).toEqual({
    mode: 'highlighted',
    rows: [
      { kind: 'equal', canonLine: 1, canonText: 'alpha', branchLine: 1, branchText: 'alpha' },
      { kind: 'deleted', canonLine: 2, canonText: 'beta', branchLine: null, branchText: null },
    ],
  })

  const replaced = alignLines('alpha\nbeta', 'alpha\ngamma')
  expect(replaced).toEqual({
    mode: 'highlighted',
    rows: [
      { kind: 'equal', canonLine: 1, canonText: 'alpha', branchLine: 1, branchText: 'alpha' },
      { kind: 'modified', canonLine: 2, canonText: 'beta', branchLine: 2, branchText: 'gamma' },
    ],
  })

  const emptyCanon = alignLines('', 'only experiment')
  expect(emptyCanon).toEqual({
    mode: 'highlighted',
    rows: [
      { kind: 'added', canonLine: null, canonText: null, branchLine: 1, branchText: 'only experiment' },
    ],
  })

  const emptyExperiment = alignLines('only canon', '')
  expect(emptyExperiment).toEqual({
    mode: 'highlighted',
    rows: [
      { kind: 'deleted', canonLine: 1, canonText: 'only canon', branchLine: null, branchText: null },
    ],
  })

  const trailingNewline = alignLines('alpha\n', 'alpha\n')
  expect(trailingNewline).toEqual({
    mode: 'highlighted',
    rows: [
      { kind: 'equal', canonLine: 1, canonText: 'alpha', branchLine: 1, branchText: 'alpha' },
    ],
  })

  const missingFinalNewline = alignLines('alpha\n', 'alpha')
  expect(missingFinalNewline).toEqual({
    mode: 'highlighted',
    rows: [
      { kind: 'equal', canonLine: 1, canonText: 'alpha', branchLine: 1, branchText: 'alpha' },
      { kind: 'added', canonLine: null, canonText: null, branchLine: 2, branchText: NO_NEWLINE_MARKER },
    ],
  })

  const addedFinalNewline = alignLines('alpha', 'alpha\n')
  expect(addedFinalNewline).toEqual({
    mode: 'highlighted',
    rows: [
      { kind: 'equal', canonLine: 1, canonText: 'alpha', branchLine: 1, branchText: 'alpha' },
      { kind: 'deleted', canonLine: 2, canonText: NO_NEWLINE_MARKER, branchLine: null, branchText: null },
    ],
  })

  const repeated = alignLines('same\nsame\nbeta', 'same\nsame\ngamma')
  expect(repeated).toEqual({
    mode: 'highlighted',
    rows: [
      { kind: 'equal', canonLine: 1, canonText: 'same', branchLine: 1, branchText: 'same' },
      { kind: 'equal', canonLine: 2, canonText: 'same', branchLine: 2, branchText: 'same' },
      { kind: 'modified', canonLine: 3, canonText: 'beta', branchLine: 3, branchText: 'gamma' },
    ],
  })
})

// Test: adjacent delete/add blocks pair into modified display rows.
// Requirements: M8-R08.
test('pairs adjacent delete and add blocks as modified rows', () => {
  const paired = alignLines('alpha\nbeta\ndelta', 'alpha\ngamma\ndelta')
  expect(paired).toEqual({
    mode: 'highlighted',
    rows: [
      { kind: 'equal', canonLine: 1, canonText: 'alpha', branchLine: 1, branchText: 'alpha' },
      { kind: 'modified', canonLine: 2, canonText: 'beta', branchLine: 2, branchText: 'gamma' },
      { kind: 'equal', canonLine: 3, canonText: 'delta', branchLine: 3, branchText: 'delta' },
    ],
  })

  const uneven = alignLines('one\ntwo\nthree', 'one\nfour\nfive\nsix')
  expect(uneven).toEqual({
    mode: 'highlighted',
    rows: [
      { kind: 'equal', canonLine: 1, canonText: 'one', branchLine: 1, branchText: 'one' },
      { kind: 'modified', canonLine: 2, canonText: 'two', branchLine: 2, branchText: 'four' },
      { kind: 'modified', canonLine: 3, canonText: 'three', branchLine: 3, branchText: 'five' },
      { kind: 'added', canonLine: null, canonText: null, branchLine: 4, branchText: 'six' },
    ],
  })
})

// Test: complexity guard returns unhighlighted fallback before quadratic work.
// Requirements: M8-R08.
test('returns fallback when line diff exceeds complexity bounds', () => {
  const longCanon = Array.from({ length: LINE_DIFF_MAX_LINES + 1 }, (_, index) => `canon-${index}`).join('\n')
  const longBranch = Array.from({ length: LINE_DIFF_MAX_LINES + 1 }, (_, index) => `branch-${index}`).join('\n')
  const tooManyLines = alignLines(longCanon, longBranch)
  expect(tooManyLines.mode).toBe('fallback')
  if (tooManyLines.mode === 'fallback') {
    expect(tooManyLines.message).toMatch(/line highlighting is unavailable/i)
  }

  const left = Array.from({ length: 1500 }, () => 'same').join('\n')
  const right = Array.from({ length: 1500 }, () => 'other').join('\n')
  const tooManyCells = alignLines(left, right)
  expect(tooManyCells.mode).toBe('fallback')
  expect(1500 * 1500).toBeGreaterThan(LINE_DIFF_MAX_CELLS)
})

// Test: inputs remain immutable and text is never interpreted as HTML.
// Requirements: M8-R08.
test('keeps inputs immutable and treats text literally', () => {
  const canon = '<b>alpha</b>\n&copy;'
  const branch = '<b>alpha</b>\n&copy; changed'
  const beforeCanon = canon
  const beforeBranch = branch
  const result = alignLines(canon, branch)
  expect(canon).toBe(beforeCanon)
  expect(branch).toBe(beforeBranch)
  expect(splitLines(canon)).toEqual(['<b>alpha</b>', '&copy;'])
  if (result.mode === 'highlighted') {
    expect(result.rows[1]).toEqual({
      kind: 'modified',
      canonLine: 2,
      canonText: '&copy;',
      branchLine: 2,
      branchText: '&copy; changed',
    })
  }

  expect(splitLines('alpha\r\nbeta\r\n')).toEqual(['alpha\r', 'beta\r'])
  expect(splitLines('')).toEqual([])
})
