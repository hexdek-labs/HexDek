// Unit tests for the bracket-changelog helpers.
//
// Run: node --test tests/unit/bracketHistory.test.js   (from hexdek/)

import test from 'node:test'
import assert from 'node:assert/strict'
import {
  MAX_SIGHTINGS,
  bracketHistoryKey,
  normalizeBracket,
  getBracketHistory,
  recordBracketSighting,
  computeBracketChangelog,
  describeLatestTransition,
  formatTransitionDate,
} from '../../src/lib/bracketHistory.js'

function makeStorage(initial = {}) {
  const map = new Map(Object.entries(initial))
  return {
    map,
    getItem(k) { return map.has(k) ? map.get(k) : null },
    setItem(k, v) { map.set(k, String(v)) },
    removeItem(k) { map.delete(k) },
  }
}

const DECK = 'wiedeman/krenko'
const KEY = `hexdek_bracket_history::${DECK}`

// --- normalizeBracket / bracketHistoryKey ---------------------------------

test('normalizeBracket: numbers, strings, fractional, sentinels', () => {
  assert.equal(normalizeBracket(3), 3)
  assert.equal(normalizeBracket('3'), 3)
  assert.equal(normalizeBracket('3.5'), 3.5)
  assert.equal(normalizeBracket('?'), null)
  assert.equal(normalizeBracket(''), null)
  assert.equal(normalizeBracket(null), null)
  assert.equal(normalizeBracket('abc'), null)
})

test('bracketHistoryKey: null on missing deck key', () => {
  assert.equal(bracketHistoryKey(''), null)
  assert.equal(bracketHistoryKey(null), null)
  assert.equal(bracketHistoryKey(DECK), KEY)
})

// --- getBracketHistory: defensive parsing ---------------------------------

test('getBracketHistory: empty when storage absent or unset', () => {
  assert.deepEqual(getBracketHistory(DECK, makeStorage()), [])
})

test('getBracketHistory: drops malformed entries from a corrupt blob', () => {
  const s = makeStorage({ [KEY]: JSON.stringify([
    { version: 1, bracket: 2, observedAt: 1700000000 }, // good
    'string instead of object',                          // bad
    { version: 'abc', bracket: 3, observedAt: 100 },     // bad version
    { version: 2, bracket: null, observedAt: 1700000100 }, // bad bracket
    { version: 3, bracket: 4, observedAt: 1700000200 }, // good
  ]) })
  assert.deepEqual(getBracketHistory(DECK, s), [
    { version: 1, bracket: 2, observedAt: 1700000000 },
    { version: 3, bracket: 4, observedAt: 1700000200 },
  ])
})

test('getBracketHistory: bad JSON → [] (no crash)', () => {
  const s = makeStorage({ [KEY]: 'not-json{{' })
  assert.deepEqual(getBracketHistory(DECK, s), [])
})

// --- recordBracketSighting ------------------------------------------------

test('recordBracketSighting: first observation establishes a baseline', () => {
  const s = makeStorage()
  const out = recordBracketSighting(DECK, 1, '2', 1700000000, s)
  assert.deepEqual(out, [{ version: 1, bracket: 2, observedAt: 1700000000 }])
})

test('recordBracketSighting: revisit of same (version, bracket) is a no-op', () => {
  const s = makeStorage()
  recordBracketSighting(DECK, 1, 2, 1700000000, s)
  const blob1 = s.map.get(KEY)
  recordBracketSighting(DECK, 1, 2, 1700999999, s)  // later visit, same bracket
  assert.equal(s.map.get(KEY), blob1)               // exact same JSON — no rewrite
})

test('recordBracketSighting: same version with NEW bracket updates in place', () => {
  // Freya re-analyzes the current head and the bracket changes; the
  // sighting array gets the new value (with a fresh timestamp), not
  // a duplicate entry.
  const s = makeStorage()
  recordBracketSighting(DECK, 1, 2, 1700000000, s)
  const out = recordBracketSighting(DECK, 1, 3, 1700000500, s)
  assert.deepEqual(out, [{ version: 1, bracket: 3, observedAt: 1700000500 }])
})

test('recordBracketSighting: out-of-order versions sort ascending', () => {
  const s = makeStorage()
  recordBracketSighting(DECK, 3, 4, 1700000300, s)
  recordBracketSighting(DECK, 1, 2, 1700000100, s)
  recordBracketSighting(DECK, 2, 3, 1700000200, s)
  const out = getBracketHistory(DECK, s)
  assert.deepEqual(out.map(e => e.version), [1, 2, 3])
})

test('recordBracketSighting: ? / null bracket → no write, returns current', () => {
  const s = makeStorage()
  recordBracketSighting(DECK, 1, 2, 1700000000, s)
  const out = recordBracketSighting(DECK, 2, '?', 1700000100, s)
  assert.deepEqual(out, [{ version: 1, bracket: 2, observedAt: 1700000000 }])
})

test('recordBracketSighting: trims to MAX_SIGHTINGS, oldest dropped first', () => {
  const s = makeStorage()
  for (let v = 1; v <= MAX_SIGHTINGS + 3; v++) {
    recordBracketSighting(DECK, v, (v % 5) + 1, 1700000000 + v, s)
  }
  const out = getBracketHistory(DECK, s)
  assert.equal(out.length, MAX_SIGHTINGS)
  // Lowest version numbers should have been trimmed.
  assert.equal(out[0].version, 4)               // 1, 2, 3 dropped
  assert.equal(out[out.length - 1].version, MAX_SIGHTINGS + 3)
})

test('recordBracketSighting: storage that throws still returns the new array', () => {
  const broken = {
    getItem() { return null },
    setItem() { throw new Error('quota') },
    removeItem() {},
  }
  const out = recordBracketSighting(DECK, 1, 2, 1700000000, broken)
  assert.deepEqual(out, [{ version: 1, bracket: 2, observedAt: 1700000000 }])
})

// --- computeBracketChangelog ---------------------------------------------

test('computeBracketChangelog: empty / single-sighting → no transitions', () => {
  assert.deepEqual(computeBracketChangelog([]), [])
  assert.deepEqual(computeBracketChangelog([{ version: 1, bracket: 2, observedAt: 1 }]), [])
})

test('computeBracketChangelog: only emits rows where the bracket actually changed', () => {
  const out = computeBracketChangelog([
    { version: 1, bracket: 2, observedAt: 100 },
    { version: 2, bracket: 2, observedAt: 200 }, // no change — skip
    { version: 3, bracket: 3, observedAt: 300 }, // upgrade
    { version: 4, bracket: 3, observedAt: 400 }, // no change
    { version: 5, bracket: 2, observedAt: 500 }, // downgrade
  ])
  assert.deepEqual(out, [
    { fromVersion: 2, toVersion: 3, fromBracket: 2, toBracket: 3, direction: 'up',   observedAt: 300 },
    { fromVersion: 4, toVersion: 5, fromBracket: 3, toBracket: 2, direction: 'down', observedAt: 500 },
  ])
})

test('computeBracketChangelog: fractional brackets carry through', () => {
  const out = computeBracketChangelog([
    { version: 1, bracket: 3,   observedAt: 100 },
    { version: 2, bracket: 3.5, observedAt: 200 },
  ])
  assert.equal(out.length, 1)
  assert.equal(out[0].fromBracket, 3)
  assert.equal(out[0].toBracket, 3.5)
  assert.equal(out[0].direction, 'up')
})

// --- formatTransitionDate -------------------------------------------------

test('formatTransitionDate: empty on bad input', () => {
  assert.equal(formatTransitionDate(0), '')
  assert.equal(formatTransitionDate(null), '')
  assert.equal(formatTransitionDate('not-a-number'), '')
})

test('formatTransitionDate: non-empty on a valid epoch', () => {
  // Don't pin to a specific locale string — just assert non-empty.
  const s = formatTransitionDate(1700000000)
  assert.ok(s && s.length > 0, `expected non-empty date, got: ${JSON.stringify(s)}`)
})

// --- describeLatestTransition --------------------------------------------

test('describeLatestTransition: empty history → empty string', () => {
  assert.equal(describeLatestTransition([]), '')
})

test('describeLatestTransition: single sighting → empty string (no transition yet)', () => {
  assert.equal(describeLatestTransition([{ version: 1, bracket: 3, observedAt: 100 }]), '')
})

test('describeLatestTransition: upgrade reads as "upgraded from"', () => {
  const out = describeLatestTransition([
    { version: 1, bracket: 2, observedAt: 1700000000 },
    { version: 2, bracket: 3, observedAt: 1700864000 },
  ])
  assert.match(out, /^B3 \(upgraded from B2/)
})

test('describeLatestTransition: downgrade reads as "downgraded from"', () => {
  const out = describeLatestTransition([
    { version: 1, bracket: 4, observedAt: 1700000000 },
    { version: 2, bracket: 3, observedAt: 1700864000 },
  ])
  assert.match(out, /^B3 \(downgraded from B4/)
})

test('describeLatestTransition: shows only the MOST RECENT transition, not all', () => {
  const out = describeLatestTransition([
    { version: 1, bracket: 2, observedAt: 100 },
    { version: 2, bracket: 3, observedAt: 200 }, // older upgrade
    { version: 3, bracket: 4, observedAt: 1700864000 }, // newer upgrade — this one
  ])
  assert.match(out, /^B4 \(upgraded from B3/)
  assert.doesNotMatch(out, /B2/)   // older transition not surfaced
})
