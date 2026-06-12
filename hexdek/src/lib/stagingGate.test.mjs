import { test } from 'node:test'
import assert from 'node:assert/strict'
import { stagingAllows, stagingRouteExempt, STAGING_WHITELIST } from './stagingGate.js'

test('whitelist admits exactly the two reviewers, case-insensitively', () => {
  assert.equal(STAGING_WHITELIST.length, 2)
  assert.ok(stagingAllows('titanicmistake@gmail.com'))
  assert.ok(stagingAllows('joshua.f.wiedeman@gmail.com'))
  assert.ok(stagingAllows('  Joshua.F.Wiedeman@Gmail.com  '))
  assert.ok(!stagingAllows('someone@else.com'))
  assert.ok(!stagingAllows(''))
  assert.ok(!stagingAllows(null))
})

test('login and auth callback stay reachable; everything else gated', () => {
  assert.ok(stagingRouteExempt('/login'))
  assert.ok(stagingRouteExempt('/auth/callback'))
  assert.ok(!stagingRouteExempt('/'))
  assert.ok(!stagingRouteExempt('/editor'))
  assert.ok(!stagingRouteExempt('/dash'))
  assert.ok(!stagingRouteExempt('/loginx'))
})
