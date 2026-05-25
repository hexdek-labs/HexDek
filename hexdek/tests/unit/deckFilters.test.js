// Unit tests for the Decks-screen chip filter helpers.
//
// Run: node --test tests/unit/deckFilters.test.js   (from hexdek/)
// Exercises the pure predicates only — no React, no DOM — so the chip
// matching logic stays verified independent of the Playwright e2e suite
// (which runs against the deployed staging site and only catches
// regressions post-deploy).

import test from 'node:test'
import assert from 'node:assert/strict'

import {
  ARCHETYPE_CHIPS,
  BRACKET_CHIPS,
  COLOR_CHIPS,
  deckBracketInt,
  deckColorIdentity,
  matchesArchetypeChip,
  matchesBracketChip,
  matchesColorChip,
} from '../../src/screens/deckFilters.js'

// ── Bracket ───────────────────────────────────────────────────────────────

test('deckBracketInt prefers pls > wbs > bracket', () => {
  assert.equal(deckBracketInt({ pls: 4, wbs: 2, bracket: '1' }), 4)
  assert.equal(deckBracketInt({ wbs: 3, bracket: '1' }), 3)
  assert.equal(deckBracketInt({ bracket: '2' }), 2)
})

test('deckBracketInt floors half-brackets (PLS 3.5 → B3)', () => {
  assert.equal(deckBracketInt({ pls: 3.5 }), 3)
})

test('deckBracketInt returns null for unknown / "?"', () => {
  assert.equal(deckBracketInt({ bracket: '?' }), null)
  assert.equal(deckBracketInt({}), null)
  assert.equal(deckBracketInt(null), null)
})

test('matchesBracketChip — "all" matches every deck (including unknown)', () => {
  for (const d of [{ pls: 4 }, { bracket: '?' }, {}]) {
    assert.equal(matchesBracketChip(d, 'all'), true, JSON.stringify(d))
  }
})

test('matchesBracketChip — exact integer match', () => {
  assert.equal(matchesBracketChip({ pls: 3 }, 'b3'), true)
  assert.equal(matchesBracketChip({ pls: 3 }, 'b4'), false)
  assert.equal(matchesBracketChip({ wbs: 4 }, 'b4'), true)
  assert.equal(matchesBracketChip({ bracket: '1' }, 'b1'), true)
})

test('matchesBracketChip — unknown bracket fails every numeric chip', () => {
  for (const chip of BRACKET_CHIPS.filter(c => c.id !== 'all')) {
    assert.equal(matchesBracketChip({ bracket: '?' }, chip.id), false, chip.id)
  }
})

// ── Color identity ────────────────────────────────────────────────────────

test('deckColorIdentity parses MONO-X correctly', () => {
  assert.deepEqual([...deckColorIdentity({ color: 'MONO-B' })], ['B'])
  assert.deepEqual([...deckColorIdentity({ color: 'MONO-W' })], ['W'])
})

test('deckColorIdentity parses multi-letter strings (WUBRG order-independent)', () => {
  const ids = deckColorIdentity({ color: 'WUB' })
  assert.equal(ids.size, 3)
  assert.ok(ids.has('W') && ids.has('U') && ids.has('B'))
})

test('deckColorIdentity returns empty set for "?" / missing', () => {
  assert.equal(deckColorIdentity({ color: '?' }).size, 0)
  assert.equal(deckColorIdentity({}).size, 0)
  assert.equal(deckColorIdentity(null).size, 0)
})

test('deckColorIdentity prefers analysis.color_identity over raw string', () => {
  // Backend ships structured analysis — should win over filename-derived
  // color so a deck whose filename lost color info still filters correctly.
  const d = { color: '?', analysis: { color_identity: ['w', 'b'] } }
  const ids = deckColorIdentity(d)
  assert.equal(ids.size, 2)
  assert.ok(ids.has('W') && ids.has('B'))
})

test('matchesColorChip — "all" matches everything', () => {
  for (const d of [{ color: 'WUB' }, { color: '?' }, {}]) {
    assert.equal(matchesColorChip(d, 'all'), true)
  }
})

test('matchesColorChip — individual color matches when present in identity', () => {
  const d = { color: 'WUB' }
  assert.equal(matchesColorChip(d, 'w'), true)
  assert.equal(matchesColorChip(d, 'u'), true)
  assert.equal(matchesColorChip(d, 'b'), true)
  assert.equal(matchesColorChip(d, 'r'), false)
  assert.equal(matchesColorChip(d, 'g'), false)
})

test('matchesColorChip — colorless chip matches only empty identity', () => {
  assert.equal(matchesColorChip({ color: '?' }, 'colorless'), true)
  assert.equal(matchesColorChip({}, 'colorless'), true)
  assert.equal(matchesColorChip({ color: 'MONO-B' }, 'colorless'), false)
  assert.equal(matchesColorChip({ color: 'WUBRG' }, 'colorless'), false)
})

test('matchesColorChip — multi chip requires 2+ colors', () => {
  assert.equal(matchesColorChip({ color: 'MONO-B' }, 'multi'), false)
  assert.equal(matchesColorChip({ color: 'WU' }, 'multi'), true)
  assert.equal(matchesColorChip({ color: 'WUBRG' }, 'multi'), true)
  assert.equal(matchesColorChip({ color: '?' }, 'multi'), false)
})

// ── Archetype ─────────────────────────────────────────────────────────────

test('matchesArchetypeChip — "all" matches every deck (including missing)', () => {
  assert.equal(matchesArchetypeChip({}, 'all'), true)
  assert.equal(matchesArchetypeChip({ archetype: 'Combo' }, 'all'), true)
})

test('matchesArchetypeChip — missing archetype fails non-"all" chips', () => {
  assert.equal(matchesArchetypeChip({}, 'combo'), false)
  assert.equal(matchesArchetypeChip({ archetype: '' }, 'aggro'), false)
})

test('matchesArchetypeChip — combo chip captures storm', () => {
  assert.equal(matchesArchetypeChip({ archetype: 'Storm' }, 'combo'), true)
  assert.equal(matchesArchetypeChip({ archetype: 'Combo / Infinite' }, 'combo'), true)
})

test('matchesArchetypeChip — tokens chip captures go-wide and aristocrats variants', () => {
  assert.equal(matchesArchetypeChip({ archetype: 'Aggro / Go Wide' }, 'tokens'), true)
  assert.equal(matchesArchetypeChip({ archetype: 'Aristocrats' }, 'tokens'), true)
  assert.equal(matchesArchetypeChip({ archetype: 'Token Production' }, 'tokens'), true)
})

test('matchesArchetypeChip — voltron excludes generic aggro', () => {
  assert.equal(matchesArchetypeChip({ archetype: 'Voltron' }, 'voltron'), true)
  assert.equal(matchesArchetypeChip({ archetype: 'Aggro' }, 'voltron'), false)
})

test('matchesArchetypeChip — falls back to analysis.archetype when top-level absent', () => {
  const d = { analysis: { archetype: 'Stax' } }
  assert.equal(matchesArchetypeChip(d, 'stax'), true)
})

// ── Chip arrays — basic shape sanity ──────────────────────────────────────

test('every chip array starts with the "all" chip', () => {
  for (const arr of [BRACKET_CHIPS, COLOR_CHIPS, ARCHETYPE_CHIPS]) {
    assert.equal(arr[0].id, 'all')
    assert.equal(arr[0].label, 'ALL')
  }
})

test('chip ids are unique within each row', () => {
  for (const arr of [BRACKET_CHIPS, COLOR_CHIPS, ARCHETYPE_CHIPS]) {
    const ids = arr.map(c => c.id)
    assert.equal(new Set(ids).size, ids.length, `dup id in ${ids.join(',')}`)
  }
})
