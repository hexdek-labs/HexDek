// Run with: node --test hexdek/src/utils/replayShare.test.js

import { test } from 'node:test'
import assert from 'node:assert/strict'

import {
  SHARE_PARAM_TURN,
  SHARE_PARAM_SEAT,
  parseShareParams,
  buildShareParams,
  buildShareUrl,
} from './replayShare.js'

// -----------------------------------------------------------------------------
// parseShareParams
// -----------------------------------------------------------------------------

test('parseShareParams reads a query string with leading ?', () => {
  assert.deepEqual(parseShareParams('?t=5&seat=2'), { turn: 5, seat: 2 })
})

test('parseShareParams reads a query string without leading ?', () => {
  assert.deepEqual(parseShareParams('t=5&seat=2'), { turn: 5, seat: 2 })
})

test('parseShareParams reads a URLSearchParams instance', () => {
  const sp = new URLSearchParams('t=7&seat=0')
  assert.deepEqual(parseShareParams(sp), { turn: 7, seat: 0 })
})

test('parseShareParams reads a plain object', () => {
  assert.deepEqual(parseShareParams({ t: '5', seat: '1' }), { turn: 5, seat: 1 })
})

test('parseShareParams returns nulls for missing fields', () => {
  assert.deepEqual(parseShareParams('?t=5'), { turn: 5, seat: null })
  assert.deepEqual(parseShareParams('?seat=2'), { turn: null, seat: 2 })
  assert.deepEqual(parseShareParams(''), { turn: null, seat: null })
})

test('parseShareParams returns nulls for malformed values', () => {
  assert.deepEqual(parseShareParams('?t=abc&seat=xyz'), { turn: null, seat: null })
  assert.deepEqual(parseShareParams('?t=NaN&seat=NaN'), { turn: null, seat: null })
})

test('parseShareParams rejects out-of-range turn (< 1)', () => {
  assert.equal(parseShareParams('?t=0').turn, null,
    'turn 0 is not a valid share target')
  assert.equal(parseShareParams('?t=-3').turn, null)
})

test('parseShareParams rejects out-of-range seat (< 0)', () => {
  assert.equal(parseShareParams('?seat=-1').seat, null)
})

test('parseShareParams clamps with maxTurn / maxSeats', () => {
  // maxTurn=10 → t=11 is out of range; t=10 is fine.
  assert.equal(parseShareParams('?t=11', { maxTurn: 10 }).turn, null)
  assert.equal(parseShareParams('?t=10', { maxTurn: 10 }).turn, 10)
  // maxSeats=4 → seat indices 0..3 valid; 4 is out.
  assert.equal(parseShareParams('?seat=4', { maxSeats: 4 }).seat, null)
  assert.equal(parseShareParams('?seat=3', { maxSeats: 4 }).seat, 3)
})

test('parseShareParams truncates decimals (?t=5.7 → 5)', () => {
  assert.equal(parseShareParams('?t=5.7').turn, 5)
})

test('parseShareParams handles null / undefined input', () => {
  assert.deepEqual(parseShareParams(null), { turn: null, seat: null })
  assert.deepEqual(parseShareParams(undefined), { turn: null, seat: null })
})

// -----------------------------------------------------------------------------
// buildShareParams
// -----------------------------------------------------------------------------

test('buildShareParams emits a URLSearchParams with both keys', () => {
  const p = buildShareParams({ turn: 5, seat: 2 })
  assert.equal(p.get(SHARE_PARAM_TURN), '5')
  assert.equal(p.get(SHARE_PARAM_SEAT), '2')
})

test('buildShareParams omits null/undefined fields', () => {
  const p = buildShareParams({ turn: 5 })
  assert.equal(p.has(SHARE_PARAM_TURN), true)
  assert.equal(p.has(SHARE_PARAM_SEAT), false)
})

test('buildShareParams treats turn 0 as omit (not a valid share target)', () => {
  const p = buildShareParams({ turn: 0, seat: 1 })
  assert.equal(p.has(SHARE_PARAM_TURN), false)
  assert.equal(p.get(SHARE_PARAM_SEAT), '1')
})

test('buildShareParams keeps seat=0 (a valid seat index)', () => {
  const p = buildShareParams({ turn: 5, seat: 0 })
  assert.equal(p.get(SHARE_PARAM_SEAT), '0')
})

test('buildShareParams truncates decimal turns', () => {
  const p = buildShareParams({ turn: 5.9, seat: 1.4 })
  assert.equal(p.get(SHARE_PARAM_TURN), '5')
  assert.equal(p.get(SHARE_PARAM_SEAT), '1')
})

test('buildShareParams ignores NaN / negative values', () => {
  const p = buildShareParams({ turn: NaN, seat: -1 })
  assert.equal(p.has(SHARE_PARAM_TURN), false)
  assert.equal(p.has(SHARE_PARAM_SEAT), false)
})

// -----------------------------------------------------------------------------
// buildShareUrl + round-trip
// -----------------------------------------------------------------------------

test('buildShareUrl appends params to a bare URL', () => {
  const url = buildShareUrl('/report/123', { turn: 5, seat: 2 })
  assert.equal(url, '/report/123?t=5&seat=2')
})

test('buildShareUrl merges with existing query params', () => {
  const url = buildShareUrl('/report/123?tab=summary&q=foo', { turn: 5 })
  // Existing keys preserved, new keys added.
  const parsed = new URLSearchParams(url.split('?')[1])
  assert.equal(parsed.get('tab'), 'summary')
  assert.equal(parsed.get('q'), 'foo')
  assert.equal(parsed.get('t'), '5')
})

test('buildShareUrl strips share params when called without them', () => {
  // Unsharing — the call removes any prior t=/seat= rather than leaving stale.
  const url = buildShareUrl('/report/123?t=99&seat=3&tab=summary', {})
  assert.equal(url, '/report/123?tab=summary')
})

test('buildShareUrl emits a clean path when no params remain', () => {
  assert.equal(buildShareUrl('/report/123', {}), '/report/123')
  assert.equal(buildShareUrl('/report/123?t=5', { turn: null }), '/report/123')
})

test('buildShareUrl returns an empty string for bad input', () => {
  assert.equal(buildShareUrl('', { turn: 5 }), '')
  assert.equal(buildShareUrl(null, { turn: 5 }), '')
})

test('round-trip: build then parse recovers the original moment', () => {
  const orig = { turn: 7, seat: 2 }
  const url = buildShareUrl('/report/123', orig)
  const qs = url.split('?')[1]
  assert.deepEqual(parseShareParams(qs), orig)
})

test('round-trip: bare report URL parses to all-null', () => {
  const url = buildShareUrl('/report/123', {})
  // No query string when no params; parseShareParams handles ''.
  const qs = url.includes('?') ? url.split('?')[1] : ''
  assert.deepEqual(parseShareParams(qs), { turn: null, seat: null })
})
