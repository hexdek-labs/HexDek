// Unit tests for the DECK HISTORY diff helpers (lib/deckHistoryDiff.js).
//
// Run: npm run test:unit                  (from hexdek/)
// Or:  node --test tests/unit/deckHistoryDiff.test.js
//
// The browser-side panel composes these into the timeline view; these
// cases pin the parser + diff math so a future refactor (e.g. swapping
// the text format for a JSON deck shape) can't silently shift the
// "+3 / -2 / Δ1" readout users expect.

import test from 'node:test'
import assert from 'node:assert/strict'

import {
  diffCounts,
  diffDeckText,
  diffSummary,
  parseDeckTextCounts,
} from '../../src/lib/deckHistoryDiff.js'

// ── parseDeckTextCounts ───────────────────────────────────────────────────

test('parseDeckTextCounts — basic "N name" lines', () => {
  const m = parseDeckTextCounts('1 Sol Ring\n2 Llanowar Elves\n30 Forest')
  assert.equal(m.get('Sol Ring'), 1)
  assert.equal(m.get('Llanowar Elves'), 2)
  assert.equal(m.get('Forest'), 30)
})

test('parseDeckTextCounts — COMMANDER: row folds in as qty 1', () => {
  const m = parseDeckTextCounts("COMMANDER: Atraxa, Praetors' Voice\n1 Sol Ring")
  assert.equal(m.get("Atraxa, Praetors' Voice"), 1)
  assert.equal(m.get('Sol Ring'), 1)
})

test('parseDeckTextCounts — duplicate lines accumulate', () => {
  const m = parseDeckTextCounts('1 Sol Ring\n2 Sol Ring')
  assert.equal(m.get('Sol Ring'), 3)
})

test('parseDeckTextCounts — blank / unparseable lines are skipped', () => {
  const m = parseDeckTextCounts('\n   \n# comment\n1 Sol Ring')
  assert.equal(m.size, 1)
  assert.equal(m.get('Sol Ring'), 1)
})

test('parseDeckTextCounts — null / empty input returns empty map', () => {
  assert.equal(parseDeckTextCounts('').size, 0)
  assert.equal(parseDeckTextCounts(null).size, 0)
})

// ── diffCounts ────────────────────────────────────────────────────────────

test('diffCounts — pure adds land in added[] with from=0', () => {
  const a = new Map([['Sol Ring', 1]])
  const b = new Map([['Sol Ring', 1], ['Forest', 30]])
  const d = diffCounts(a, b)
  assert.equal(d.added.length, 1)
  assert.equal(d.added[0].name, 'Forest')
  assert.equal(d.added[0].from, 0)
  assert.equal(d.added[0].to, 30)
  assert.equal(d.removed.length, 0)
  assert.equal(d.changed.length, 0)
  assert.equal(d.addedCards, 30)
  assert.equal(d.removedCards, 0)
  assert.equal(d.netCards, 30)
})

test('diffCounts — pure removals land in removed[] with to=0', () => {
  const a = new Map([['Sol Ring', 1], ['Forest', 30]])
  const b = new Map([['Sol Ring', 1]])
  const d = diffCounts(a, b)
  assert.equal(d.removed.length, 1)
  assert.equal(d.removed[0].name, 'Forest')
  assert.equal(d.removed[0].to, 0)
  assert.equal(d.removed[0].delta, 30)
  assert.equal(d.removedCards, 30)
  assert.equal(d.netCards, -30)
})

test('diffCounts — qty change (4 → 2) lands in changed[], NOT added/removed', () => {
  const a = new Map([['Brainstorm', 4]])
  const b = new Map([['Brainstorm', 2]])
  const d = diffCounts(a, b)
  assert.equal(d.added.length, 0)
  assert.equal(d.removed.length, 0)
  assert.equal(d.changed.length, 1)
  assert.deepEqual(d.changed[0], { name: 'Brainstorm', from: 4, to: 2, delta: -2 })
  assert.equal(d.addedCards, 0)
  assert.equal(d.removedCards, 2)
  assert.equal(d.netCards, -2)
})

test('diffCounts — qty change up (1 → 3) deltas as +2', () => {
  const a = new Map([['Forest', 1]])
  const b = new Map([['Forest', 3]])
  const d = diffCounts(a, b)
  assert.equal(d.changed[0].delta, 2)
  assert.equal(d.addedCards, 2)
  assert.equal(d.removedCards, 0)
})

test('diffCounts — identical maps produce a clean diff', () => {
  const a = new Map([['Sol Ring', 1], ['Forest', 30]])
  const b = new Map([['Sol Ring', 1], ['Forest', 30]])
  const d = diffCounts(a, b)
  assert.equal(d.isClean, true)
  assert.equal(d.added.length, 0)
  assert.equal(d.removed.length, 0)
  assert.equal(d.changed.length, 0)
  assert.equal(d.netCards, 0)
})

test('diffCounts — sorts added/removed/changed by name (deterministic UI)', () => {
  const a = new Map([['Forest', 2]])
  const b = new Map([['Apple', 1], ['Zebra', 1], ['Banana', 1]])
  const d = diffCounts(a, b)
  assert.deepEqual(d.added.map(x => x.name), ['Apple', 'Banana', 'Zebra'])
})

test('diffCounts — accepts plain object inputs (legacy callers)', () => {
  const d = diffCounts({ 'Sol Ring': 1 }, { 'Sol Ring': 2 })
  assert.equal(d.changed[0].name, 'Sol Ring')
  assert.equal(d.changed[0].delta, 1)
})

// ── diffDeckText (text → parse → diff convenience) ────────────────────────

test('diffDeckText — commander swap renders as remove + add', () => {
  const a = `COMMANDER: Krenko, Mob Boss\n1 Sol Ring`
  const b = `COMMANDER: Atraxa, Praetors' Voice\n1 Sol Ring`
  const d = diffDeckText(a, b)
  assert.equal(d.removed.length, 1)
  assert.equal(d.added.length, 1)
  assert.equal(d.removed[0].name, 'Krenko, Mob Boss')
  assert.equal(d.added[0].name, "Atraxa, Praetors' Voice")
})

test('diffDeckText — multi-line realistic deck edit', () => {
  const baseline = '1 Sol Ring\n4 Brainstorm\n30 Forest\n1 Bad Card'
  const current  = '1 Sol Ring\n2 Brainstorm\n30 Forest\n1 New Toy'
  const d = diffDeckText(baseline, current)
  assert.equal(d.added.length, 1)
  assert.equal(d.removed.length, 1)
  assert.equal(d.changed.length, 1)
  assert.equal(d.added[0].name, 'New Toy')
  assert.equal(d.removed[0].name, 'Bad Card')
  assert.equal(d.changed[0].name, 'Brainstorm')
  assert.equal(d.changed[0].delta, -2)
})

test('diffDeckText — empty baseline → everything is "added"', () => {
  const d = diffDeckText('', '1 Sol Ring\n30 Forest')
  assert.equal(d.added.length, 2)
  assert.equal(d.removed.length, 0)
  assert.equal(d.changed.length, 0)
  assert.equal(d.addedCards, 31)
})

// ── diffSummary ───────────────────────────────────────────────────────────

test('diffSummary — clean diff returns "NO CHANGES"', () => {
  assert.equal(diffSummary(diffDeckText('1 Sol Ring', '1 Sol Ring')), 'NO CHANGES')
  assert.equal(diffSummary({ isClean: true }), 'NO CHANGES')
})

test('diffSummary — shows +/-/Δ counts plus net delta', () => {
  const d = diffDeckText('1 A\n4 B\n1 C', '1 D\n2 B\n1 C')
  // added: [D], removed: [A], changed: [B 4→2 = -2]
  const s = diffSummary(d)
  assert.match(s, /\+1/)
  assert.match(s, /-1/)
  assert.match(s, /Δ1/)
  assert.match(s, /net -2/)
})

test('diffSummary — null / undefined are tolerated', () => {
  assert.equal(diffSummary(null), 'NO CHANGES')
  assert.equal(diffSummary(undefined), 'NO CHANGES')
})

// ── end-to-end realistic workshop iteration ───────────────────────────────

test('end-to-end — three sequential edits track cleanly through diffs', () => {
  // Simulate the same deck moving through three saved versions.
  const v1 = '1 Sol Ring\n4 Brainstorm\n30 Forest'
  const v2 = '1 Sol Ring\n3 Brainstorm\n30 Forest\n1 Counterspell'   // -1 BS, +1 CS
  const v3 = '1 Sol Ring\n3 Brainstorm\n29 Forest\n1 Counterspell\n1 Wasteland' // -1 Forest, +1 Wasteland

  const d12 = diffDeckText(v1, v2)
  assert.equal(d12.added.length, 1)
  assert.equal(d12.changed.length, 1)
  assert.equal(d12.changed[0].name, 'Brainstorm')
  assert.equal(d12.changed[0].delta, -1)
  assert.equal(d12.netCards, 0)

  const d23 = diffDeckText(v2, v3)
  assert.equal(d23.added.length, 1)
  assert.equal(d23.changed.length, 1)
  assert.equal(d23.changed[0].name, 'Forest')
  assert.equal(d23.changed[0].delta, -1)
  assert.equal(d23.netCards, 0)
})
