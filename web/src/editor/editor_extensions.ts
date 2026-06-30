/**
 * editor_extensions.ts
 *
 * Defines the shared CodeMirror extension set used by the scene editor,
 * including Markdown support, line wrapping, and Vim keybindings.
 */

import { markdown } from '@codemirror/lang-markdown'
import { EditorView } from '@codemirror/view'
import { basicSetup } from 'codemirror'
import { vim } from '@replit/codemirror-vim'

/** codeMirrorExtensions is the shared extension bundle for scene editing. */
export const codeMirrorExtensions = [
  basicSetup,
  markdown(),
  EditorView.lineWrapping,
  vim(),
]
