import { render, screen } from '@testing-library/react'
import { expect, test } from 'vitest'
import ContextPreview from './ContextPreview'

// BDD Scenario: 7.1.1 - Preview minimal Line Polish context
// Requirements: M7-R08, M7-R17
// Test purpose: verify context preview shows packs, estimates, and omissions.

const selectionManifest = {
  scope: 'selection' as const,
  packs_used: ['selected_text', 'style_sheet'],
  packs_omitted: [],
  estimated_input_tokens: 42,
  max_input_estimated_tokens: 8000,
  rag_mode: 'none' as const,
}

// Test: shows included omitted packs and estimated tokens.
// Requirements: M7-R08.
test('shows included omitted packs and estimated tokens', () => {
  render(<ContextPreview manifest={{
    ...selectionManifest,
    packs_omitted: [{ pack: 'outline_neighborhood', reason: 'budget' }],
    estimated_input_tokens: 4312,
    max_input_estimated_tokens: 12000,
    active_codex: [{ entry_id: 'char_0123456789abcdef0123', applied_progression_ids: ['prog_0123456789abcdef0123'] }],
  }} />)

  expect(screen.getByText('selection')).toBeInTheDocument()
  expect(screen.getByText('4312')).toBeInTheDocument()
  expect(screen.getByText(/12000 estimated tokens/)).toBeInTheDocument()
  expect(screen.getByText(/selected_text, style_sheet/)).toBeInTheDocument()
  expect(screen.getByText(/outline_neighborhood \(budget\)/)).toBeInTheDocument()
  expect(screen.getByText('char_0123456789abcdef0123')).toBeInTheDocument()
})

// Test: shows paragraph preview with no wider context.
// Requirements: M7-R02.
test('shows paragraph preview with no wider context', () => {
  render(<ContextPreview manifest={selectionManifest} />)
  expect(screen.getByText(/selected_text, style_sheet/)).toBeInTheDocument()
  expect(screen.queryByText(/current_scene/)).not.toBeInTheDocument()
  expect(screen.queryByText(/active_codex_at_position/)).not.toBeInTheDocument()
})