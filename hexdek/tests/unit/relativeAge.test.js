import test from 'node:test'
import assert from 'node:assert/strict'
import { relativeAge, formatAge } from '../../src/lib/relativeAge.js'

const NOW = Date.UTC(2026, 9, 26, 12, 0, 0)
const ago = (sec) => relativeAge(NOW - sec * 1000, NOW)

test('relativeAge buckets by magnitude', () => {
  assert.deepEqual(ago(0), { kind: 'now', value: 0 })
  assert.deepEqual(ago(4), { kind: 'now', value: 4 })
  assert.deepEqual(ago(30), { kind: 's', value: 30 })
  assert.deepEqual(ago(90), { kind: 'm', value: 1 })
  assert.deepEqual(ago(3599), { kind: 'm', value: 59 })
  assert.deepEqual(ago(7200), { kind: 'h', value: 2 })
  assert.deepEqual(ago(86399), { kind: 'h', value: 23 })
  assert.deepEqual(ago(2 * 86400), { kind: 'd', value: 2 })
})

test('relativeAge falls back to an ISO date past 30 days', () => {
  const age = ago(40 * 86400)
  assert.equal(age.kind, 'date')
  assert.equal(age.value, '2026-09-16')
})

test('relativeAge clamps future timestamps instead of going negative', () => {
  assert.deepEqual(relativeAge(NOW + 60_000, NOW), { kind: 'now', value: 0 })
})

test('relativeAge reports none for a missing or unparseable timestamp', () => {
  assert.deepEqual(relativeAge(null, NOW), { kind: 'none', value: 0 })
  assert.deepEqual(relativeAge(NaN, NOW), { kind: 'none', value: 0 })
  assert.deepEqual(relativeAge(undefined, NOW), { kind: 'none', value: 0 })
})

test('formatAge picks the key at the call site and passes the count', () => {
  const t = (key, vars) => `${key}:${vars ? vars.count : ''}`
  assert.equal(formatAge(t, ago(7200)), 'age.h:2')
  assert.equal(formatAge(t, ago(2)), 'age.now:')
  assert.equal(formatAge(t, relativeAge(null, NOW)), '')
  // A date needs no translation, so it must come back verbatim.
  assert.equal(formatAge(t, ago(40 * 86400)), '2026-09-16')
})
