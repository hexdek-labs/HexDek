// Unit tests for the DECK BUDGET frontend helpers
// (lib/deckBudget.js). The React panel composes these with a single
// /api/decks/{id}/budget fetch; these cases pin the formatter +
// summary derivation so a future backend payload tweak can't
// silently shift what users see in the headline.
//
// Run: npm run test:unit
// Or:  node --test tests/unit/deckBudget.test.js

import test from 'node:test'
import assert from 'node:assert/strict'

import {
  budgetTierKey,
  formatUSD,
  formatUSDShort,
  summarize,
} from '../../src/lib/deckBudget.js'

// ── formatUSD ─────────────────────────────────────────────────────────────

test('formatUSD — basic dollar amounts', () => {
  assert.equal(formatUSD(0),       '$0.00')
  assert.equal(formatUSD(1.5),     '$1.50')
  assert.equal(formatUSD(89.99),   '$89.99')
  assert.equal(formatUSD(148.32),  '$148.32')
})

test('formatUSD — thousands-separator inserted at 4+ digits', () => {
  assert.equal(formatUSD(1234.56), '$1,234.56')
  assert.equal(formatUSD(1234567.89), '$1,234,567.89')
})

test('formatUSD — withCents=false rounds to whole dollars', () => {
  assert.equal(formatUSD(89.99, { withCents: false }), '$90')
  assert.equal(formatUSD(0.49,  { withCents: false }), '$0')
})

test('formatUSD — null / non-numeric returns em-dash placeholder', () => {
  assert.equal(formatUSD(null),       '—')
  assert.equal(formatUSD(undefined),  '—')
  assert.equal(formatUSD('not a number'), '—')
  assert.equal(formatUSD(NaN),        '—')
})

test('formatUSD — negative numbers stay signed', () => {
  assert.equal(formatUSD(-12.34), '-$12.34')
})

// ── formatUSDShort ────────────────────────────────────────────────────────

test('formatUSDShort — values under $1000 drop cents at $10+, keep them under $10', () => {
  // Compact form trades cents for chrome economy: a $89.99 chip
  // reads "$90" at-a-glance, but $9.99 keeps the cents because the
  // dollar precision matters more at low denominations.
  assert.equal(formatUSDShort(9.99),  '$9.99')
  assert.equal(formatUSDShort(89.99), '$90')
  assert.equal(formatUSDShort(999),   '$999')
})

test('formatUSDShort — $1000+ compacts to K notation', () => {
  assert.equal(formatUSDShort(1289.99), '$1.29K')
  assert.equal(formatUSDShort(15000),   '$15.0K')
})

test('formatUSDShort — small values render without cents above $10', () => {
  // $9.99 -> "$9.99" (with cents), $50 -> "$50" (no cents)
  assert.equal(formatUSDShort(50), '$50')
})

test('formatUSDShort — null / NaN tolerated', () => {
  assert.equal(formatUSDShort(null), '—')
  assert.equal(formatUSDShort(NaN),  '—')
})

// ── budgetTierKey ─────────────────────────────────────────────────────────
// Asserts the catalog KEY, not a word: the lib returns a key so it stays
// importable here without the JSON catalogs.

test('budgetTierKey — bracket boundaries', () => {
  assert.equal(budgetTierKey(0),    'dka.bt_unpriced')
  assert.equal(budgetTierKey(49),   'dka.bt_ultra')
  assert.equal(budgetTierKey(50),   'dka.bt_budget')
  assert.equal(budgetTierKey(149),  'dka.bt_budget')
  assert.equal(budgetTierKey(150),  'dka.bt_moderate')
  assert.equal(budgetTierKey(399),  'dka.bt_moderate')
  assert.equal(budgetTierKey(400),  'dka.bt_premium')
  assert.equal(budgetTierKey(999),  'dka.bt_premium')
  assert.equal(budgetTierKey(1000), 'dka.bt_chase')
  assert.equal(budgetTierKey(9999), 'dka.bt_chase')
})

test('budgetTierKey — non-numeric / null fall back to the unpriced key', () => {
  assert.equal(budgetTierKey(null),      'dka.bt_unpriced')
  assert.equal(budgetTierKey(undefined), 'dka.bt_unpriced')
  assert.equal(budgetTierKey('nope'),    'dka.bt_unpriced')
})

// ── summarize ─────────────────────────────────────────────────────────────

const PAYLOAD = {
  currency: 'USD',
  total: 148.32,
  card_count: 99,
  unique_count: 82,
  priced_count: 79,
  missing_count: 3,
  cards: [
    { name: 'Mana Crypt', qty: 1, unit: 89.99, line: 89.99, priced: true },
    { name: 'Sol Ring',   qty: 1, unit: 1.50,  line: 1.50,  priced: true },
    { name: 'Forest',     qty: 30, unit: 0,    line: 0,     priced: true },
    { name: 'Mystery Card', qty: 1, unit: 0, line: 0, priced: false },
  ],
  top_expensive: [
    { name: 'Mana Crypt', qty: 1, unit: 89.99, line: 89.99, priced: true },
    { name: 'Sol Ring',   qty: 1, unit: 1.50,  line: 1.50,  priced: true },
  ],
}

test('summarize — exposes the core display metrics', () => {
  const s = summarize(PAYLOAD)
  assert.equal(s.total, 148.32)
  assert.equal(s.cardCount, 99)
  assert.equal(s.uniqueCount, 82)
  assert.equal(s.pricedCount, 79)
  assert.equal(s.missingCount, 3)
  assert.equal(s.currency, 'USD')
})

test('summarize — coverage = priced / unique', () => {
  const s = summarize(PAYLOAD)
  assert.ok(Math.abs(s.coverage - (79 / 82)) < 1e-9)
})

test('summarize — avgPerCard = total / card_count', () => {
  const s = summarize(PAYLOAD)
  assert.ok(Math.abs(s.avgPerCard - (148.32 / 99)) < 1e-9)
})

test('summarize — tier key set from total', () => {
  assert.equal(summarize({ total: 148 }).tierKey, 'dka.bt_budget')
  assert.equal(summarize({ total: 500 }).tierKey, 'dka.bt_premium')
})

test('summarize — uses backend top_expensive when present and large enough', () => {
  const s = summarize(PAYLOAD, { topN: 2 })
  assert.equal(s.topExpensive.length, 2)
  assert.equal(s.topExpensive[0].name, 'Mana Crypt')
})

test('summarize — re-derives top-N when backend list is short', () => {
  // Server returned only one row in top_expensive, but caller asks for 3.
  // summarize should fall back to deriving from `cards` (priced rows only,
  // sorted by line desc).
  const short = { ...PAYLOAD, top_expensive: [PAYLOAD.top_expensive[0]] }
  const s = summarize(short, { topN: 3 })
  // Cards has 2 priced non-zero rows (Mana Crypt, Sol Ring); Forest is
  // priced but $0; Mystery Card is unpriced. Both filtered out.
  assert.equal(s.topExpensive.length, 2)
  assert.equal(s.topExpensive[0].name, 'Mana Crypt')
  assert.equal(s.topExpensive[1].name, 'Sol Ring')
})

test('summarize — derives top-N when backend top_expensive missing entirely', () => {
  const noTop = { ...PAYLOAD, top_expensive: undefined }
  const s = summarize(noTop)
  // Falls back to client-side derivation; Forest + Mystery excluded.
  assert.equal(s.topExpensive[0].name, 'Mana Crypt')
  assert.equal(s.topExpensive.length, 2)
})

test('summarize — empty / null payload returns zero shape (panel still renders)', () => {
  const s = summarize(null)
  assert.equal(s.total, 0)
  assert.equal(s.cardCount, 0)
  assert.equal(s.uniqueCount, 0)
  assert.equal(s.coverage, 0)
  assert.equal(s.avgPerCard, 0)
  assert.equal(s.tierKey, 'dka.bt_unpriced')
  assert.deepEqual(s.topExpensive, [])
  assert.equal(s.currency, 'USD')
})

test('summarize — zero unique_count avoids divide-by-zero on coverage', () => {
  const s = summarize({ total: 0, card_count: 0, unique_count: 0 })
  assert.equal(s.coverage, 0)
  assert.equal(s.avgPerCard, 0)
})

test('summarize — partial response (missing top_expensive, has cards) still works', () => {
  const s = summarize({
    currency: 'USD',
    total: 5,
    card_count: 2,
    unique_count: 2,
    priced_count: 2,
    missing_count: 0,
    cards: [
      { name: 'A', qty: 1, unit: 3, line: 3, priced: true },
      { name: 'B', qty: 1, unit: 2, line: 2, priced: true },
    ],
  })
  assert.equal(s.topExpensive.length, 2)
  assert.equal(s.topExpensive[0].name, 'A')
})

test('summarize — unpriced cards never enter top_expensive', () => {
  const s = summarize({
    total: 0,
    cards: [
      { name: 'Unpriced', qty: 1, unit: 0, line: 0, priced: false },
      { name: 'Free Basic', qty: 30, unit: 0, line: 0, priced: true },
    ],
    top_expensive: [],
  })
  assert.equal(s.topExpensive.length, 0)
})
