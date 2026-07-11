/**
 * lineDiff.ts
 *
 * Bounded deterministic line alignment for read-only side-by-side display.
 */

import type { DiffRow, LineDiffResult } from './branchTypes'

export const LINE_DIFF_MAX_LINES = 2000
export const LINE_DIFF_MAX_CELLS = 2_000_000
const FALLBACK_MESSAGE = 'Line highlighting is unavailable for this file. Both complete texts are shown without row alignment.'
export const NO_NEWLINE_MARKER = 'No newline at end of file'

export function splitLines(text: string): string[] {
  if (text.length === 0) {
    return []
  }
  const lines = text.split('\n')
  if (text.endsWith('\n')) {
    lines.pop()
  }
  return lines
}

export function alignLines(canonText: string, branchText: string): LineDiffResult {
  const canonLines = splitLines(canonText)
  const branchLines = splitLines(branchText)

  if (
    canonLines.length > LINE_DIFF_MAX_LINES ||
    branchLines.length > LINE_DIFF_MAX_LINES ||
    canonLines.length * branchLines.length > LINE_DIFF_MAX_CELLS
  ) {
    return { mode: 'fallback', message: FALLBACK_MESSAGE }
  }

  const rawRows = buildRawRows(canonLines, branchLines)
  const rows = pairModifiedRows(rawRows)
  if (canonText.endsWith('\n') !== branchText.endsWith('\n') && (canonText.length > 0 || branchText.length > 0)) {
    if (canonText.endsWith('\n')) {
      rows.push({ kind: 'added', canonLine: null, canonText: null, branchLine: branchLines.length + 1, branchText: NO_NEWLINE_MARKER })
    } else {
      rows.push({ kind: 'deleted', canonLine: canonLines.length + 1, canonText: NO_NEWLINE_MARKER, branchLine: null, branchText: null })
    }
  }
  return { mode: 'highlighted', rows }
}

export function isNoNewlineMarker(text: string | null): boolean {
  return text === NO_NEWLINE_MARKER
}

function buildRawRows(canonLines: string[], branchLines: string[]): DiffRow[] {
  const lcs = longestCommonSubsequence(canonLines, branchLines)
  const rows: DiffRow[] = []
  let canonIndex = 0
  let branchIndex = 0
  let canonLine = 1
  let branchLine = 1

  for (const entry of lcs) {
    while (canonIndex < entry.canonIndex) {
      rows.push({
        kind: 'deleted',
        canonLine,
        canonText: canonLines[canonIndex],
        branchLine: null,
        branchText: null,
      })
      canonIndex += 1
      canonLine += 1
    }
    while (branchIndex < entry.branchIndex) {
      rows.push({
        kind: 'added',
        canonLine: null,
        canonText: null,
        branchLine,
        branchText: branchLines[branchIndex],
      })
      branchIndex += 1
      branchLine += 1
    }
    rows.push({
      kind: 'equal',
      canonLine,
      canonText: canonLines[canonIndex],
      branchLine,
      branchText: branchLines[branchIndex],
    })
    canonIndex += 1
    branchIndex += 1
    canonLine += 1
    branchLine += 1
  }

  while (canonIndex < canonLines.length) {
    rows.push({
      kind: 'deleted',
      canonLine,
      canonText: canonLines[canonIndex],
      branchLine: null,
      branchText: null,
    })
    canonIndex += 1
    canonLine += 1
  }
  while (branchIndex < branchLines.length) {
    rows.push({
      kind: 'added',
      canonLine: null,
      canonText: null,
      branchLine,
      branchText: branchLines[branchIndex],
    })
    branchIndex += 1
    branchLine += 1
  }

  return rows
}

type LcsEntry = { canonIndex: number; branchIndex: number }

function longestCommonSubsequence(left: string[], right: string[]): LcsEntry[] {
  const width = right.length + 1
  const table = new Array((left.length + 1) * width).fill(0)

  for (let leftIndex = 1; leftIndex <= left.length; leftIndex += 1) {
    for (let rightIndex = 1; rightIndex <= right.length; rightIndex += 1) {
      const current = leftIndex * width + rightIndex
      if (left[leftIndex - 1] === right[rightIndex - 1]) {
        table[current] = table[(leftIndex - 1) * width + (rightIndex - 1)] + 1
      } else {
        const up = table[(leftIndex - 1) * width + rightIndex]
        const leftValue = table[leftIndex * width + (rightIndex - 1)]
        table[current] = Math.max(up, leftValue)
      }
    }
  }

  const entries: LcsEntry[] = []
  let leftIndex = left.length
  let rightIndex = right.length
  while (leftIndex > 0 && rightIndex > 0) {
    if (left[leftIndex - 1] === right[rightIndex - 1]) {
      entries.unshift({ canonIndex: leftIndex - 1, branchIndex: rightIndex - 1 })
      leftIndex -= 1
      rightIndex -= 1
      continue
    }
    const up = table[(leftIndex - 1) * width + rightIndex]
    const leftValue = table[leftIndex * width + (rightIndex - 1)]
    if (up >= leftValue) {
      leftIndex -= 1
    } else {
      rightIndex -= 1
    }
  }
  return entries
}

function pairModifiedRows(rows: DiffRow[]): DiffRow[] {
  const paired: DiffRow[] = []
  let index = 0

  while (index < rows.length) {
    const current = rows[index]
    if (current.kind !== 'deleted') {
      paired.push(current)
      index += 1
      continue
    }

    const deletedBlock: DiffRow[] = []
    while (index < rows.length && rows[index].kind === 'deleted') {
      deletedBlock.push(rows[index])
      index += 1
    }

    const addedBlock: DiffRow[] = []
    while (index < rows.length && rows[index].kind === 'added') {
      addedBlock.push(rows[index])
      index += 1
    }

    const pairCount = Math.min(deletedBlock.length, addedBlock.length)
    for (let pairIndex = 0; pairIndex < pairCount; pairIndex += 1) {
      paired.push({
        kind: 'modified',
        canonLine: deletedBlock[pairIndex].canonLine,
        canonText: deletedBlock[pairIndex].canonText,
        branchLine: addedBlock[pairIndex].branchLine,
        branchText: addedBlock[pairIndex].branchText,
      })
    }
    for (let extraIndex = pairCount; extraIndex < deletedBlock.length; extraIndex += 1) {
      paired.push(deletedBlock[extraIndex])
    }
    for (let extraIndex = pairCount; extraIndex < addedBlock.length; extraIndex += 1) {
      paired.push(addedBlock[extraIndex])
    }
  }

  return paired
}
