// Unit tests for the error-telemetry scoping filters (r61). Pins the
// signal/chatter contract: app errors (login/auth, core services,
// render crashes) report; resource-load noise, cross-origin script
// errors, and plain connectivity failures do not.
//
// Runs under plain `node --test` (npm run test:unit) — shouldReport and
// describeRejection are pure, and the module guards every window read.

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { shouldReport, describeRejection } from './errorTelemetry.js'

test('boundary render crash reports (the pls / TDZ class)', () => {
  assert.equal(shouldReport({
    message: "ReferenceError: pls is not defined",
    hasErrorObject: true,
    filename: '',
    source: 'boundary',
  }), true)
})

test('uncaught window error with an Error object reports', () => {
  assert.equal(shouldReport({
    message: 'TypeError: Cannot read properties of undefined',
    hasErrorObject: true,
    filename: '',
    source: 'window',
  }), true)
})

test('resource-load chatter (no Error object on window event) is excluded', () => {
  assert.equal(shouldReport({
    message: 'some resource failed',
    hasErrorObject: false,
    filename: '',
    source: 'window',
  }), false)
})

test('cross-origin "Script error." is excluded', () => {
  assert.equal(shouldReport({
    message: 'Script error.',
    hasErrorObject: true,
    filename: '',
    source: 'window',
  }), false)
  assert.equal(shouldReport({
    message: 'script error',
    hasErrorObject: true,
    filename: '',
    source: 'window',
  }), false)
})

test('empty message is excluded', () => {
  assert.equal(shouldReport({ message: '', hasErrorObject: true, filename: '', source: 'window' }), false)
  assert.equal(shouldReport({ message: '   ', hasErrorObject: true, filename: '', source: 'window' }), false)
})

test('connectivity noise is excluded from the promise channel only', () => {
  for (const msg of [
    'TypeError: Failed to fetch',
    'Failed to fetch',
    'NetworkError when attempting to fetch resource.',
    'Load failed',
  ]) {
    assert.equal(shouldReport({
      message: msg, hasErrorObject: true, filename: '', source: 'promise',
    }), false, `promise channel should exclude: ${msg}`)
  }
  // The same message from a render boundary is a code path that THREW —
  // that stays reportable (a component that doesn't handle fetch
  // failure is a bug, not weather).
  assert.equal(shouldReport({
    message: 'TypeError: Failed to fetch',
    hasErrorObject: true,
    filename: '',
    source: 'boundary',
  }), true)
})

test('non-network promise rejections report', () => {
  assert.equal(shouldReport({
    message: 'ReferenceError: gauntlet is not defined',
    hasErrorObject: true,
    filename: '',
    source: 'promise',
  }), true)
})

test('describeRejection normalizes Error, string, and object reasons', () => {
  const err = new Error('boom')
  const d1 = describeRejection(err)
  assert.equal(d1.message, 'Error: boom')
  assert.ok(d1.stack.length > 0)

  const d2 = describeRejection('plain string reason')
  assert.equal(d2.message, 'plain string reason')
  assert.equal(d2.stack, '')

  const d3 = describeRejection({ code: 42 })
  assert.match(d3.message, /non-error rejection/)
  assert.match(d3.message, /42/)
})
