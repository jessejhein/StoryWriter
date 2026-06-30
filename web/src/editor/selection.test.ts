import { expect, test } from 'vitest'
import { countWords, toUTF8ByteRange } from './selection'

// BDD trace:
// - Requirements: M4-R05, M4-R09, M4-R17.
// - Scenario: 4.2.1, 4.5.1.
// - Test purpose: verify frontend selection utilities estimate words and convert
//   CodeMirror character positions into UTF-8 byte offsets for multibyte prose.
test('counts words and converts UTF-8 byte ranges for multibyte selections', () => {
  expect(countWords('  one\tTwo\nthree  ')).toBe(3)
  expect(countWords('')).toBe(0)

  const markdown = 'Alpha\nLuz ágil\nOmega'
  const start = markdown.indexOf('Luz')
  const end = start + 'Luz ágil'.length
  expect(toUTF8ByteRange(markdown, start, end)).toEqual({
    startByte: 6,
    endByte: 15,
  })
})
