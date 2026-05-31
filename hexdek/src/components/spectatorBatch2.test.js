// Run with: node --test hexdek/src/components/spectatorBatch2.test.js
//
// Unit coverage for the pure helpers powering the R60 spectator UX
// batch 2 components: ManaPoolPips + StackPanel.

import { test } from 'node:test'
import assert from 'node:assert/strict'

import {
  MANA_COLOR_ORDER,
  MANA_COLORS,
  normalizeManaPool,
  totalMana,
} from './manaPoolHelpers.js'

import {
  formatStackItemLabel,
  stackItemKindGlyph,
  stackOrderTopFirst,
} from './stackPanelHelpers.js'

// -----------------------------------------------------------------------------
// ManaPoolHelpers
// -----------------------------------------------------------------------------

test('MANA_COLOR_ORDER has the canonical WUBRG-C-Any sequence', () => {
  assert.deepEqual(MANA_COLOR_ORDER, ['W', 'U', 'B', 'R', 'G', 'C', 'Any'])
})

test('MANA_COLORS has a stop for every order entry', () => {
  for (const c of MANA_COLOR_ORDER) {
    assert.equal(typeof MANA_COLORS[c], 'string', `missing color stop for ${c}`)
  }
})

test('normalizeManaPool returns empty array for empty/missing input', () => {
  assert.deepEqual(normalizeManaPool(null), [])
  assert.deepEqual(normalizeManaPool(undefined), [])
  assert.deepEqual(normalizeManaPool({}), [])
  assert.deepEqual(normalizeManaPool('not-a-map'), [])
})

test('normalizeManaPool preserves canonical WUBRG order', () => {
  // Intentionally insert in non-canonical order; helper must re-order.
  const got = normalizeManaPool({ B: 1, G: 2, W: 3, U: 4, R: 5 })
  assert.deepEqual(got.map(e => e.color), ['W', 'U', 'B', 'R', 'G'])
  assert.equal(got.find(e => e.color === 'W').count, 3)
})

test('normalizeManaPool filters zero counts defensively', () => {
  const got = normalizeManaPool({ W: 2, U: 0, B: 0, R: 1, G: 0, C: 3, Any: 0 })
  assert.deepEqual(got.map(e => e.color), ['W', 'R', 'C'])
})

test('normalizeManaPool tolerates non-numeric values', () => {
  // Defensive: a malformed payload shouldn't throw.
  const got = normalizeManaPool({ W: 'two', U: null, R: 1 })
  assert.deepEqual(got, [{ color: 'R', count: 1 }])
})

test('totalMana sums every bucket', () => {
  assert.equal(totalMana({ W: 1, U: 1, B: 1, R: 1, G: 1, C: 1, Any: 1 }), 7)
})

test('totalMana returns 0 for empty/missing input', () => {
  assert.equal(totalMana(null), 0)
  assert.equal(totalMana({}), 0)
})

// -----------------------------------------------------------------------------
// StackPanelHelpers
// -----------------------------------------------------------------------------

test('stackItemKindGlyph empty/legacy/spell → no prefix', () => {
  assert.equal(stackItemKindGlyph(''), '')
  assert.equal(stackItemKindGlyph(null), '')
  assert.equal(stackItemKindGlyph(undefined), '')
  assert.equal(stackItemKindGlyph('spell'), '')
  assert.equal(stackItemKindGlyph('legacy'), '')
})

test('stackItemKindGlyph activated → ACT', () => {
  assert.equal(stackItemKindGlyph('activated'), 'ACT')
  assert.equal(stackItemKindGlyph('ACTIVATED'), 'ACT')
})

test('stackItemKindGlyph triggered → TRG', () => {
  assert.equal(stackItemKindGlyph('triggered'), 'TRG')
})

test('stackItemKindGlyph unknown kind uppercased as fallback', () => {
  assert.equal(stackItemKindGlyph('exotic_new_kind'), 'EXOTIC_NEW_KIND')
})

test('formatStackItemLabel renders source name', () => {
  assert.equal(formatStackItemLabel({ source: 'Counterspell' }), 'Counterspell')
})

test('formatStackItemLabel marks copy flag', () => {
  const got = formatStackItemLabel({ source: 'Lightning Bolt', is_copy: true })
  assert.equal(got, 'Lightning Bolt (copy)')
})

test('formatStackItemLabel marks countered flag', () => {
  const got = formatStackItemLabel({ source: 'Lightning Bolt', countered: true })
  assert.equal(got, 'Lightning Bolt (countered)')
})

test('formatStackItemLabel combines copy + countered', () => {
  const got = formatStackItemLabel({ source: 'Mystical Tutor', is_copy: true, countered: true })
  assert.equal(got, 'Mystical Tutor (copy, countered)')
})

test('formatStackItemLabel handles missing source defensively', () => {
  assert.equal(formatStackItemLabel({}), '?')
  assert.equal(formatStackItemLabel(null), '')
})

test('stackOrderTopFirst flips engine bottom-first order', () => {
  // Engine: Stack[0] = bottom (resolves last), Stack[len-1] = top (resolves first).
  const engine = [
    { source: 'Bottom (oldest)' },
    { source: 'Middle' },
    { source: 'Top (newest, resolves first)' },
  ]
  const display = stackOrderTopFirst(engine)
  assert.equal(display[0].source, 'Top (newest, resolves first)')
  assert.equal(display[2].source, 'Bottom (oldest)')
})

test('stackOrderTopFirst returns a fresh array (caller-safe)', () => {
  const engine = [{ source: 'A' }, { source: 'B' }]
  const display = stackOrderTopFirst(engine)
  display.push({ source: 'C' })
  assert.equal(engine.length, 2, 'mutating the display result should not affect engine')
})

test('stackOrderTopFirst handles empty/missing input', () => {
  assert.deepEqual(stackOrderTopFirst(null), [])
  assert.deepEqual(stackOrderTopFirst(undefined), [])
  assert.deepEqual(stackOrderTopFirst([]), [])
  assert.deepEqual(stackOrderTopFirst('not-an-array'), [])
})

test('stackOrderTopFirst single-item passes through unchanged', () => {
  const got = stackOrderTopFirst([{ source: 'A' }])
  assert.deepEqual(got, [{ source: 'A' }])
})
