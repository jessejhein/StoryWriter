/** vite.config.ts configures the frontend build, tests, and local API proxy. */
/// <reference types="vitest/config" />

import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

const apiOrigin = process.env.STORYWORK_API_ORIGIN ?? 'http://127.0.0.1:9090'

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': apiOrigin,
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: './src/test/setup.ts',
  },
})
