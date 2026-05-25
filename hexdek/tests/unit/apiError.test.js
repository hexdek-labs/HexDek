// Unit tests for services/api.js's unwrapApiError decoder. Pins the
// deploy-crossover invariant: whether the backend is on the r60
// nested envelope ({error: {code, message, details}, request_id,
// status}) OR the pre-r60 flat shape ({error: "msg", status}) OR
// returns plain text / arbitrary JSON, the resulting Error object
// always exposes flat .code / .message / .details / .requestId /
// .status / .body fields.
//
// Run: npm run test:unit
// Or:  node --test tests/unit/apiError.test.js

import test from 'node:test'
import assert from 'node:assert/strict'

import { unwrapApiError } from '../../src/services/api.js'

test('r60 nested envelope — populates code, message, details, requestId', () => {
  const err = unwrapApiError({
    status: 402,
    path: '/api/gauntlet/alice/korvold',
    body: JSON.stringify({
      error: {
        code: 'free_quota_exhausted',
        message: 'out of free runs',
        details: { balance: 7, needed: 25, quota: 5 },
      },
      request_id: 'trace-deadbeef',
      status: 402,
    }),
  })
  assert.equal(err.status, 402)
  assert.equal(err.code, 'free_quota_exhausted')
  assert.equal(err.message, 'out of free runs')
  assert.deepEqual(err.details, { balance: 7, needed: 25, quota: 5 })
  assert.equal(err.requestId, 'trace-deadbeef')
})

test('pre-r60 flat envelope — hoists string error to both message AND code', () => {
  // Switch-on-code consumers (DeckArchive credits-gate at
  // err.code === "free_quota_exhausted") must continue to fire even
  // when the backend is the older flat-shape build during deploy
  // crossover. Tested by faking the legacy body.
  const err = unwrapApiError({
    status: 402,
    path: '/api/gauntlet/alice/korvold',
    body: JSON.stringify({
      error: 'free_quota_exhausted',
      status: 402,
      balance: 7,
      needed: 25,
      quota: 5,
    }),
  })
  assert.equal(err.status, 402)
  assert.equal(err.code, 'free_quota_exhausted', 'legacy string error MUST hoist to .code')
  assert.equal(err.message, 'free_quota_exhausted', 'legacy string error also populates .message')
  // Pre-r60 top-level extras get surfaced under .details so the
  // new-shape consumer path (details.needed / details.balance) works
  // against either server version.
  assert.deepEqual(err.details, { balance: 7, needed: 25, quota: 5 })
})

test('r60 envelope with nested message wins over status default', () => {
  const err = unwrapApiError({
    status: 400,
    path: '/api/decks',
    body: JSON.stringify({
      error: { code: 'bad_request', message: 'deck_list is required' },
      status: 400,
    }),
  })
  assert.equal(err.message, 'deck_list is required')
  assert.equal(err.code, 'bad_request')
  assert.equal(err.details, null)
})

test('plain text body — falls back to text as message, empty code/details', () => {
  const err = unwrapApiError({
    status: 500,
    path: '/api/foo',
    body: 'internal server boom',
  })
  assert.equal(err.message, 'internal server boom')
  assert.equal(err.code, '')
  assert.equal(err.details, null)
  assert.equal(err.requestId, undefined)
})

test('empty body — synthesizes "API <status>: <path>" message', () => {
  const err = unwrapApiError({ status: 504, path: '/api/slow', body: '' })
  assert.equal(err.message, 'API 504: /api/slow')
  assert.equal(err.status, 504)
})

test('malformed JSON body — treated as plain text', () => {
  const err = unwrapApiError({
    status: 502,
    path: '/api/foo',
    body: '{not-json',
  })
  assert.equal(err.message, '{not-json')
  assert.equal(err.code, '')
})

test('r60 envelope without request_id — requestId stays undefined', () => {
  const err = unwrapApiError({
    status: 404,
    path: '/api/foo',
    body: JSON.stringify({
      error: { code: 'not_found', message: 'missing' },
      status: 404,
    }),
  })
  assert.equal(err.requestId, undefined)
  assert.equal(err.code, 'not_found')
})

test('raw body always preserved on err.body for legacy consumers', () => {
  const raw = JSON.stringify({
    error: { code: 'forbidden', message: 'nope' },
    status: 403,
  })
  const err = unwrapApiError({ status: 403, path: '/api/foo', body: raw })
  assert.equal(err.body, raw, 'err.body is the raw text for callers that re-parse it')
})

test('details only surfaced when shape provides an object', () => {
  // Nested empty-details object → propagated as empty object.
  const eEmpty = unwrapApiError({
    status: 400, path: '/x',
    body: JSON.stringify({ error: { code: 'x', message: 'y', details: {} }, status: 400 }),
  })
  assert.deepEqual(eEmpty.details, {})

  // Nested missing-details → stays null.
  const eMissing = unwrapApiError({
    status: 400, path: '/x',
    body: JSON.stringify({ error: { code: 'x', message: 'y' }, status: 400 }),
  })
  assert.equal(eMissing.details, null)
})
