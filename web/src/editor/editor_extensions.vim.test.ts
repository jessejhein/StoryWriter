import { EditorView } from '@codemirror/view'
import { getCM, Vim } from '@replit/codemirror-vim'
import { afterEach, expect, test, vi } from 'vitest'
import { codeMirrorExtensions } from './editor_extensions'

class ResizeObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}

vi.stubGlobal('ResizeObserver', ResizeObserverStub)

if (!Range.prototype.getClientRects) {
  Range.prototype.getClientRects = () => ({
    length: 0,
    item: () => null,
    [Symbol.iterator]: function* () {},
  }) as DOMRectList
}

if (!Range.prototype.getBoundingClientRect) {
  Range.prototype.getBoundingClientRect = () => new DOMRect(0, 0, 0, 0)
}

afterEach(() => {
  Vim.resetVimGlobalState_()
  document.body.innerHTML = ''
})

// BDD trace:
// - Requirement: M2-R03.
// - Scenario: 2.1.1 — Load a valid scene.
// - Test purpose: verify the real CodeMirror Vim extension starts active and
//   handles a normal-mode edit command without falling back to plain text behavior.
test('vim extension handles normal-mode delete commands', () => {
  const host = document.createElement('div')
  document.body.appendChild(host)

  const view = new EditorView({
    doc: 'Scene prose.\n',
    extensions: codeMirrorExtensions,
    parent: host,
  })
  const cm = getCM(view)
  if (!cm) {
    throw new Error('expected CodeMirror Vim bridge')
  }

  Vim.enterVimMode(cm)
  Vim.handleKey(cm, 'x', 'user')

  expect(view.state.doc.toString()).toBe('cene prose.\n')

  view.destroy()
})
