/**
 * main.tsx
 *
 * Boots the Storywork React application into the Vite root element.
 */

import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import App from './App'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
