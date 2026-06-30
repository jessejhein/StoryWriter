/**
 * CodeMirrorSurface.tsx
 *
 * Wraps a CodeMirror editor instance for the scene editor. It owns the editor
 * lifecycle and keeps the external React value synchronized with CodeMirror's
 * document state.
 */

import { useEffect, useEffectEvent, useRef } from 'react'
import { EditorSelection } from '@codemirror/state'
import { EditorView } from '@codemirror/view'
import { codeMirrorExtensions } from './editor_extensions'

type Props = {
  value: string
  onChange: (value: string) => void
  onSelectionChange?: (selection: { start: number; end: number; text: string }) => void
}

/**
 * CodeMirrorSurface
 *
 * Renders the imperative CodeMirror host used for canonical scene markdown.
 */
export default function CodeMirrorSurface({ value, onChange, onSelectionChange }: Props) {
  const hostRef = useRef<HTMLDivElement | null>(null)
  const viewRef = useRef<EditorView | null>(null)
  const initialValueRef = useRef(value)
  const handleChange = useEffectEvent(onChange)
  const handleSelectionChange = useEffectEvent(onSelectionChange ?? (() => {}))

  useEffect(() => {
    if (!hostRef.current) {
      return
    }

    const view = new EditorView({
      doc: initialValueRef.current,
      extensions: [
        ...codeMirrorExtensions,
        EditorView.updateListener.of((update) => {
          if (update.docChanged) {
            handleChange(update.state.doc.toString())
          }
          if (update.docChanged || update.selectionSet) {
            const selection = update.state.selection.main
            const text = update.state.doc.sliceString(selection.from, selection.to)
            handleSelectionChange({ start: selection.from, end: selection.to, text })
          }
        }),
      ],
      parent: hostRef.current,
    })
    const selection = view.state.selection.main
    handleSelectionChange({ start: selection.from, end: selection.to, text: view.state.doc.sliceString(selection.from, selection.to) })
    viewRef.current = view
    return () => {
      view.destroy()
      viewRef.current = null
    }
  }, [])

  useEffect(() => {
    const view = viewRef.current
    if (!view) {
      return
    }
    if (view.state.doc.toString() === value) {
      return
    }
    view.dispatch({
      changes: { from: 0, to: view.state.doc.length, insert: value },
      selection: EditorSelection.cursor(Math.min(view.state.selection.main.head, value.length)),
    })
  }, [value])

  return <div ref={hostRef} className="code-surface" data-testid="codemirror-surface" />
}
