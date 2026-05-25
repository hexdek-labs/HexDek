// Unit tests for the MY COLLECTION power-tier dashboard helpers
// (lib/deckPowerSummary.js).
//
// Run: npm run test:unit              (from hexdek/)
// Or:  node --test tests/unit/deckPowerSummary.test.js
//
// React panel (DeckPowerSummaryPanel) composes these into the
// dashboard chrome; cases below pin the bucketing rules, peak-tie
// resolution, archetype ranking, and the no-decks zero shape so the
// dashboard stays stable as the underlying deck shape evolves.

import test from 'node:test'
import assert from 'node:assert/strict'

import {
  POWER_TIER_KEYS,
  POWER_TIER_LABELS,
  peakFlavor,
  summarizePowerTiers,
  topArchetypes,
} from '../../src/lib/deckPowerSummary.js'

// Shorthand for fixture construction. The summary helpers consume the
// DeckSummary shape /api/decks returns; only pls/wbs/bracket and
// archetype matter to these tests.
const deck = ({ pls, wbs, bracket, archetype } = {}) => ({ pls, wbs, bracket, archetype })

// ── summarizePowerTiers ───────────────────────────────────────────────────

test('summarizePowerTiers — buckets PLS / WBS / bracket precedence correctly', () => {
  // pls wins over wbs wins over bracket — same precedence as
  // deckBracketInt + the BRACKET chip filter row.
  const s = summarizePowerTiers([
    deck({ pls: 4, wbs: 2, bracket: '1' }), // → b4
    deck({ wbs: 3, bracket: '1' }),         // → b3
    deck({ bracket: '2' }),                 // → b2
  ])
  assert.equal(s.counts.b4, 1)
  assert.equal(s.counts.b3, 1)
  assert.equal(s.counts.b2, 1)
  assert.equal(s.total, 3)
})

test('summarizePowerTiers — unknown brackets bucket into the unknown slot', () => {
  const s = summarizePowerTiers([
    deck({}),                          // no bracket data → unknown
    deck({ bracket: '?' }),            // explicit "?" → unknown
    deck({ pls: 3 }),                  // → b3
  ])
  assert.equal(s.counts.unknown, 2)
  assert.equal(s.counts.b3, 1)
  assert.equal(s.total, 3)
})

test('summarizePowerTiers — percentages round to whole numbers and sum ≤ 100', () => {
  const s = summarizePowerTiers([
    deck({ pls: 1 }), deck({ pls: 2 }), deck({ pls: 3 }),
    deck({ pls: 4 }), deck({ pls: 5 }),
  ])
  // 5 decks → 20% each
  for (const k of ['b1', 'b2', 'b3', 'b4', 'b5']) {
    assert.equal(s.percentages[k], 20, `pct ${k}`)
  }
  const total = POWER_TIER_KEYS.reduce((n, k) => n + s.percentages[k], 0)
  assert.ok(total <= 100, `pct sum ${total} > 100`)
})

test('summarizePowerTiers — peak selects the most-populated bracket', () => {
  const s = summarizePowerTiers([
    deck({ pls: 3 }), deck({ pls: 3 }), deck({ pls: 3 }),
    deck({ pls: 4 }),
  ])
  assert.equal(s.peak, 'b3')
  assert.equal(s.peakCount, 3)
})

test('summarizePowerTiers — peak ties break toward the HIGHER bracket', () => {
  // Per the helper's comment: a 3/4 split reads as "you're a B4
  // collector with a B3 backup" — the higher bracket wins ties.
  const s = summarizePowerTiers([
    deck({ pls: 3 }), deck({ pls: 3 }),
    deck({ pls: 4 }), deck({ pls: 4 }),
  ])
  assert.equal(s.peak, 'b4')
  assert.equal(s.peakCount, 2)
})

test('summarizePowerTiers — peak is null when every deck is unknown', () => {
  const s = summarizePowerTiers([deck({}), deck({}), deck({ bracket: '?' })])
  assert.equal(s.peak, null)
  assert.equal(s.peakCount, 0)
  assert.equal(s.counts.unknown, 3)
})

test('summarizePowerTiers — pricedShare = (known)/total', () => {
  const s = summarizePowerTiers([
    deck({ pls: 3 }), deck({ pls: 4 }), deck({}), deck({ bracket: '?' }),
  ])
  // 2 known of 4 total → 0.5
  assert.equal(s.pricedShare, 0.5)
})

test('summarizePowerTiers — empty input returns a usable zero shape', () => {
  const s = summarizePowerTiers([])
  assert.equal(s.total, 0)
  for (const k of POWER_TIER_KEYS) {
    assert.equal(s.counts[k], 0)
    assert.equal(s.percentages[k], 0)
  }
  assert.equal(s.peak, null)
  assert.equal(s.peakCount, 0)
  assert.equal(s.pricedShare, 0)
})

test('summarizePowerTiers — null / non-array input does not crash', () => {
  for (const x of [null, undefined, {}, 'nope']) {
    const s = summarizePowerTiers(x)
    assert.equal(s.total, 0)
  }
})

test('summarizePowerTiers — PLS half-bracket (3.5) floors to B3 via deckBracketInt', () => {
  const s = summarizePowerTiers([deck({ pls: 3.5 })])
  assert.equal(s.counts.b3, 1)
  assert.equal(s.counts.b4, 0)
})

// ── topArchetypes ─────────────────────────────────────────────────────────

test('topArchetypes — ranks by deck count, ties break alphabetically', () => {
  const rows = topArchetypes([
    deck({ archetype: 'Combo' }), deck({ archetype: 'Combo' }),
    deck({ archetype: 'Aggro' }), deck({ archetype: 'Aggro' }),
    deck({ archetype: 'Stax' }),
  ])
  // Combo and Aggro tied at 2; Aggro sorts first alphabetically.
  assert.equal(rows[0].archetype, 'Aggro')
  assert.equal(rows[0].count, 2)
  assert.equal(rows[1].archetype, 'Combo')
  assert.equal(rows[1].count, 2)
  assert.equal(rows[2].archetype, 'Stax')
  assert.equal(rows[2].count, 1)
})

test('topArchetypes — case-insensitive grouping, retains first observed casing', () => {
  const rows = topArchetypes([
    deck({ archetype: 'Aggro' }),
    deck({ archetype: 'AGGRO' }),
    deck({ archetype: 'aggro' }),
  ])
  assert.equal(rows.length, 1)
  assert.equal(rows[0].count, 3)
  assert.equal(rows[0].archetype, 'Aggro') // first casing observed
})

test('topArchetypes — falls back to analysis.archetype when top-level missing', () => {
  const rows = topArchetypes([
    { analysis: { archetype: 'Tribal' } },
    { analysis: { archetype: 'Tribal' } },
    deck({ archetype: 'Tribal' }),
  ])
  assert.equal(rows[0].archetype, 'Tribal')
  assert.equal(rows[0].count, 3)
})

test('topArchetypes — drops decks with no archetype label', () => {
  const rows = topArchetypes([
    deck({ archetype: '' }),
    deck({}),
    deck({ archetype: 'Combo' }),
  ])
  assert.equal(rows.length, 1)
  assert.equal(rows[0].count, 1)
})

test('topArchetypes — limit caps the slice (default 5, overridable)', () => {
  const seven = ['A', 'B', 'C', 'D', 'E', 'F', 'G'].map(a => deck({ archetype: a }))
  assert.equal(topArchetypes(seven).length, 5)
  assert.equal(topArchetypes(seven, 3).length, 3)
  assert.equal(topArchetypes(seven, 10).length, 7)
})

test('topArchetypes — empty / null returns []', () => {
  assert.deepEqual(topArchetypes([]), [])
  assert.deepEqual(topArchetypes(null), [])
  assert.deepEqual(topArchetypes(undefined), [])
})

// ── peakFlavor ────────────────────────────────────────────────────────────

test('peakFlavor — produces the dashboard headline line', () => {
  const s = summarizePowerTiers([
    deck({ pls: 4 }), deck({ pls: 4 }), deck({ pls: 4 }),
    deck({ pls: 3 }),
  ])
  const f = peakFlavor(s)
  assert.match(f, /YOUR COLLECTION PEAKS AT/)
  assert.match(f, /B4/)
  assert.match(f, /3 DECKS/)
  assert.match(f, /75%/)
})

test('peakFlavor — empty collection returns empty string', () => {
  assert.equal(peakFlavor(summarizePowerTiers([])), '')
})

test('peakFlavor — all-unknown collection returns the populate-Freya hint', () => {
  const s = summarizePowerTiers([deck({}), deck({}), deck({})])
  assert.equal(peakFlavor(s), 'NO BRACKETS YET — RUN FREYA TO POPULATE')
})

// ── Module exports sanity ─────────────────────────────────────────────────

test('POWER_TIER_KEYS — covers b1..b5 plus unknown, in render order', () => {
  assert.deepEqual(POWER_TIER_KEYS, ['b1', 'b2', 'b3', 'b4', 'b5', 'unknown'])
})

test('POWER_TIER_LABELS — every tier key has a non-empty label', () => {
  for (const k of POWER_TIER_KEYS) {
    assert.ok(POWER_TIER_LABELS[k], `missing label for ${k}`)
  }
})
