import '@testing-library/jest-dom/vitest'
import { cleanup, configure } from '@testing-library/react'
import { afterEach } from 'vitest'

// Full-suite jsdom workers can delay React effects beyond Testing Library's
// one-second default even when isolated tests are fast. Keep asynchronous UI
// assertions deterministic under the required complete-suite run.
configure({ asyncUtilTimeout: 3000 })

afterEach(() => {
  cleanup()
})
