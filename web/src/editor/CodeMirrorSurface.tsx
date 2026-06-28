import { useEffect, useEffectEvent, useRef } from 'react'
import { EditorSelection } from '@codemirror/state'
import { EditorView } from '@codemirror/view'
import { codeMirrorExtensions } from './editor_extensions'

type Props = {
  value: string
  onChange: (value: string) => void
}

export default function CodeMirrorSurface({ value, onChange }: Props) {
  const hostRef = useRef<HTMLDivElement | null>(null)
  const viewRef = useRef<EditorView | null>(null)
  const initialValueRef = useRef(value)
  const handleChange = useEffectEvent(onChange)

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
        }),
      ],
      parent: hostRef.current,
    })
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
