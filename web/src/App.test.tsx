import { render, screen, waitFor } from '@testing-library/react'
import { expect, test, vi } from 'vitest'
import App from './App'

// BDD trace:
// - Requirement: Milestone 0, Story 0.3, health check.
// - Scenario: when I request /api/health, I receive status ok and version
//   information.
// - Test purpose: verify the UI renders the backend health version returned by
//   the API.
test('shows backend health version', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
    ok: true,
    json: async () => ({ status: 'ok', version: '0.0.0-test' }),
  }))

  render(<App />)

  await waitFor(() => expect(screen.getByText('Online · 0.0.0-test')).toBeInTheDocument())
  vi.unstubAllGlobals()
})
