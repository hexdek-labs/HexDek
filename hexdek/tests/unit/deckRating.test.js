// Unit tests for the per-deck owner-private rating storage.
//
// Run: node --test tests/unit/deckRating.test.js   (from hexdek/)
// Uses an injected Map-backed storage stub so the helpers can be
// exercised without touching globalThis.localStorage.

import test from 'node:test'
import assert from 'node:assert/strict'
import {
  deckRatingKey,
  normalizeRating,
  getDeckRating,
  setDeckRating,
} from '../../src/lib/deckRating.js'

// Minimal localStorage-shape stub. Tracks writes/removes so we can
// assert both the stored value AND the side effects (clear-on-zero).
function makeStorage(initial = {}) {
  const map = new Map(Object.entries(initial))
  return {
    map,
    getItem(k) { return map.has(k) ? map.get(k) : null },
    setItem(k, v) { map.set(k, String(v)) },
    removeItem(k) { map.delete(k) },
  }
}

const USER = 'wiedeman'
const DECK = 'wiedeman/krenko'
const KEY = `hexdek_deck_rating::${USER}::${DECK}`

// --- deckRatingKey ---------------------------------------------------------

test('deckRatingKey: builds namespaced key, lowercases user slug', () => {
  assert.equal(deckRatingKey('Wiedeman', DECK), KEY)
})

test('deckRatingKey: null when either piece is missing', () => {
  assert.equal(deckRatingKey('', DECK), null)
  assert.equal(deckRatingKey(USER, ''), null)
  assert.equal(deckRatingKey(null, DECK), null)
})

// --- normalizeRating -------------------------------------------------------

test('normalizeRating: rounds to nearest int and clamps to 0/[1,5]', () => {
  assert.equal(normalizeRating(0), 0)
  assert.equal(normalizeRating(1), 1)
  assert.equal(normalizeRating(5), 5)
  assert.equal(normalizeRating(3.4), 3)
  assert.equal(normalizeRating(3.6), 4)
  assert.equal(normalizeRating(6), 0)   // out of range → unrated sentinel
  assert.equal(normalizeRating(-2), 0)
  assert.equal(normalizeRating('4'), 4)
  assert.equal(normalizeRating('abc'), 0)
  assert.equal(normalizeRating(null), 0)
  assert.equal(normalizeRating(undefined), 0)
})

// --- getDeckRating ---------------------------------------------------------

test('getDeckRating: returns 0 for an unrated deck', () => {
  const s = makeStorage()
  assert.equal(getDeckRating(USER, DECK, s), 0)
})

test('getDeckRating: reads and normalizes a persisted value', () => {
  const s = makeStorage({ [KEY]: '4' })
  assert.equal(getDeckRating(USER, DECK, s), 4)
})

test('getDeckRating: corrupted value → 0 (treat as unrated)', () => {
  const s = makeStorage({ [KEY]: 'nope' })
  assert.equal(getDeckRating(USER, DECK, s), 0)
})

test('getDeckRating: missing user or deck → 0 without touching storage', () => {
  const s = makeStorage({ [KEY]: '5' })
  assert.equal(getDeckRating('', DECK, s), 0)
  assert.equal(getDeckRating(USER, '', s), 0)
})

test('getDeckRating: null storage available (SSR / private mode) → 0', () => {
  // Pass a storage that throws to simulate the locked-down case.
  const broken = {
    getItem() { throw new Error('access denied') },
    setItem() {},
    removeItem() {},
  }
  assert.equal(getDeckRating(USER, DECK, broken), 0)
})

// --- setDeckRating ---------------------------------------------------------

test('setDeckRating: persists a valid rating and returns it', () => {
  const s = makeStorage()
  const r = setDeckRating(USER, DECK, 3, s)
  assert.equal(r, 3)
  assert.equal(s.map.get(KEY), '3')
})

test('setDeckRating: zero / out-of-range deletes the key', () => {
  const s = makeStorage({ [KEY]: '4' })
  const r = setDeckRating(USER, DECK, 0, s)
  assert.equal(r, 0)
  assert.equal(s.map.has(KEY), false)
})

test('setDeckRating: rating > 5 normalizes to 0 (delete) — no silent clamp', () => {
  // Intentional: we don't want a stray 7 to land as a 5. Bad input
  // should clear, not coerce, so the user notices and re-enters.
  const s = makeStorage({ [KEY]: '4' })
  setDeckRating(USER, DECK, 7, s)
  assert.equal(s.map.has(KEY), false)
})

test('setDeckRating: rounds floats before persisting', () => {
  const s = makeStorage()
  setDeckRating(USER, DECK, 4.4, s)
  assert.equal(s.map.get(KEY), '4')
})

test('setDeckRating: missing user/deck → no-op return-only', () => {
  const s = makeStorage()
  assert.equal(setDeckRating('', DECK, 5, s), 0)
  assert.equal(s.map.size, 0)
})

test('setDeckRating: storage that throws does not crash, still returns normalized', () => {
  const broken = {
    getItem() { return null },
    setItem() { throw new Error('quota exceeded') },
    removeItem() { throw new Error('quota exceeded') },
  }
  // Should not throw; UI layer can still trust the return value.
  assert.equal(setDeckRating(USER, DECK, 4, broken), 4)
  assert.equal(setDeckRating(USER, DECK, 0, broken), 0)
})
