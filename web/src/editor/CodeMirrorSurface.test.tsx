import { render, screen } from '@testing-library/react'
import { expect, test, vi } from 'vitest'

const editorViewMock = vi.fn()
const updateListenerOfMock = vi.fn(() => ({ type: 'listener' }))
const markdownMock = vi.fn(() => ({ type: 'markdown' }))
const vimMock = vi.fn(() => ({ type: 'vim' }))

class EditorViewMock {
  static lineWrapping = { type: 'lineWrapping' }
  static updateListener = { of: updateListenerOfMock }
  state: { doc: { toString: () => string; length: number; sliceString: (from: number, to: number) => string }; selection: { main: { head: number; from: number; to: number } } }
  dispatch = vi.fn()
  destroy = vi.fn()

  constructor(config: { doc: string }) {
    editorViewMock(config)
    this.state = {
      doc: {
        toString: () => config.doc,
        length: config.doc.length,
        sliceString: (from, to) => config.doc.slice(from, to),
      },
      selection: { main: { head: 0, from: 0, to: 0 } },
    }
  }
}

vi.mock('@codemirror/view', () => ({
  EditorView: EditorViewMock,
}))

vi.mock('@codemirror/state', () => ({
  EditorSelection: {
    cursor: vi.fn((value) => value),
  },
}))

vi.mock('@codemirror/lang-markdown', () => ({
  markdown: markdownMock,
}))

vi.mock('codemirror', () => ({
  basicSetup: { type: 'basicSetup' },
}))

vi.mock('@replit/codemirror-vim', () => ({
  vim: vimMock,
}))

const { default: CodeMirrorSurface } = await import('./CodeMirrorSurface')

// BDD trace:
// - Requirement: M2-R03.
// - Scenario: 2.1.1 — Load a valid scene.
// - Test purpose: verify the editor initializes CodeMirror with the Markdown and
//   Vim extensions rather than falling back to a plain textarea.
test('configures codemirror markdown and vim extensions', () => {
  render(<CodeMirrorSurface value="Scene prose.\n" onChange={() => {}} />)

  expect(screen.getByTestId('codemirror-surface')).toBeInTheDocument()
  expect(markdownMock).toHaveBeenCalled()
  expect(vimMock).toHaveBeenCalled()
  expect(editorViewMock).toHaveBeenCalled()
})
