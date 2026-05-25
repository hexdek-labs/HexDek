// Unit tests for the anonymous deck-share helpers.
//
// Run: node --test tests/unit/deckShare.test.js   (from hexdek/)

import test from 'node:test'
import assert from 'node:assert/strict'
import {
  ANON_QUERY_FLAG,
  buildAnonShareUrl,
  isAnonymousShare,
  shareOptInKey,
  isShareOptIn,
  setShareOptIn,
} from '../../src/lib/deckShare.js'

function makeStorage(initial = {}) {
  const map = new Map(Object.entries(initial))
  return {
    map,
    getItem(k) { return map.has(k) ? map.get(k) : null },
    setItem(k, v) { map.set(k, String(v)) },
    removeItem(k) { map.delete(k) },
  }
}

// --- buildAnonShareUrl -----------------------------------------------------

test('buildAnonShareUrl: composes /share/:owner/:id?anon=1', () => {
  assert.equal(
    buildAnonShareUrl('https://hexdek.dev', 'wiedeman', 'krenko'),
    'https://hexdek.dev/share/wiedeman/krenko?anon=1',
  )
})

test('buildAnonShareUrl: strips trailing slash on origin', () => {
  assert.equal(
    buildAnonShareUrl('https://hexdek.dev/', 'w', 'k'),
    'https://hexdek.dev/share/w/k?anon=1',
  )
  assert.equal(
    buildAnonShareUrl('https://hexdek.dev///', 'w', 'k'),
    'https://hexdek.dev/share/w/k?anon=1',
  )
})

test('buildAnonShareUrl: empty pieces → empty string', () => {
  assert.equal(buildAnonShareUrl('', 'w', 'k'), '')
  assert.equal(buildAnonShareUrl('https://h.dev', '', 'k'), '')
  assert.equal(buildAnonShareUrl('https://h.dev', 'w', ''), '')
})

// --- isAnonymousShare ------------------------------------------------------

test('isAnonymousShare: true for ?anon=1', () => {
  assert.equal(isAnonymousShare('?anon=1'), true)
  assert.equal(isAnonymousShare('anon=1'), true)
})

test('isAnonymousShare: bare flag (no value) also anonymizes', () => {
  // Permissive on purpose — a typo'd `?anon` should still hide owner.
  assert.equal(isAnonymousShare('?anon'), true)
  assert.equal(isAnonymousShare('?anon&foo=bar'), true)
})

test('isAnonymousShare: explicit falsy values bypass anonymization', () => {
  assert.equal(isAnonymousShare('?anon=0'), false)
  assert.equal(isAnonymousShare('?anon=false'), false)
  assert.equal(isAnonymousShare('?anon=NO'), false)
})

test('isAnonymousShare: missing flag → false', () => {
  assert.equal(isAnonymousShare(''), false)
  assert.equal(isAnonymousShare('?foo=bar'), false)
  assert.equal(isAnonymousShare(null), false)
  assert.equal(isAnonymousShare(undefined), false)
})

test('isAnonymousShare: accepts a URLSearchParams instance', () => {
  const sp = new URLSearchParams({ [ANON_QUERY_FLAG]: '1', foo: 'bar' })
  assert.equal(isAnonymousShare(sp), true)
  const off = new URLSearchParams({ foo: 'bar' })
  assert.equal(isAnonymousShare(off), false)
})

// --- shareOptInKey ---------------------------------------------------------

test('shareOptInKey: namespaced by deck key', () => {
  assert.equal(shareOptInKey('wiedeman/krenko'), 'hexdek_share_optin::wiedeman/krenko')
})

test('shareOptInKey: null when deck key missing', () => {
  assert.equal(shareOptInKey(''), null)
  assert.equal(shareOptInKey(null), null)
})

// --- isShareOptIn / setShareOptIn round-trip -------------------------------

const DECK = 'wiedeman/krenko'
const KEY = `hexdek_share_optin::${DECK}`

test('isShareOptIn: false when no flag persisted', () => {
  const s = makeStorage()
  assert.equal(isShareOptIn(DECK, s), false)
})

test('isShareOptIn: true when flag = "1"', () => {
  const s = makeStorage({ [KEY]: '1' })
  assert.equal(isShareOptIn(DECK, s), true)
})

test('isShareOptIn: any non-"1" value is treated as off', () => {
  // Intentional: only the canonical "1" reads as enabled, so corrupted
  // values or older formats fail closed (don't surface the URL).
  const s = makeStorage({ [KEY]: 'true' })
  assert.equal(isShareOptIn(DECK, s), false)
})

test('setShareOptIn: enabling writes "1"', () => {
  const s = makeStorage()
  assert.equal(setShareOptIn(DECK, true, s), true)
  assert.equal(s.map.get(KEY), '1')
})

test('setShareOptIn: disabling removes the key (no "off" residue)', () => {
  const s = makeStorage({ [KEY]: '1' })
  assert.equal(setShareOptIn(DECK, false, s), false)
  assert.equal(s.map.has(KEY), false)
})

test('setShareOptIn: missing deck key → false, no storage touched', () => {
  const s = makeStorage()
  assert.equal(setShareOptIn('', true, s), false)
  assert.equal(s.map.size, 0)
})

test('setShareOptIn: storage that throws does not crash, returns intent', () => {
  const broken = {
    getItem() { return null },
    setItem() { throw new Error('quota') },
    removeItem() { throw new Error('quota') },
  }
  // Caller should still see what they ASKED for, so the in-memory UI
  // can flip even if persistence failed.
  assert.equal(setShareOptIn(DECK, true, broken), true)
  assert.equal(setShareOptIn(DECK, false, broken), false)
})
