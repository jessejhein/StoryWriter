import { markdown } from '@codemirror/lang-markdown'
import { EditorView } from '@codemirror/view'
import { basicSetup } from 'codemirror'
import { vim } from '@replit/codemirror-vim'

export const codeMirrorExtensions = [
  basicSetup,
  markdown(),
  EditorView.lineWrapping,
  vim(),
]
