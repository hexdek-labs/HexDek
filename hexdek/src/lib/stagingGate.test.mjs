import { test } from 'node:test'
import assert from 'node:assert/strict'
import { stagingAllows, stagingRouteExempt, STAGING_WHITELIST, passphraseEnabled, passphraseMatches, previewParamMatches, STAGING_PREVIEW_PARAM, STAGING_PASS_STORAGE_KEY } from './stagingGate.js'

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

test('passphraseEnabled: only non-empty strings enable the bypass', () => {
  assert.equal(passphraseEnabled('correct horse'), true)
  assert.equal(passphraseEnabled(''), false)
  assert.equal(passphraseEnabled('   '), false)
  assert.equal(passphraseEnabled(undefined), false)
  assert.equal(passphraseEnabled(null), false)
})

test('passphraseMatches: trimmed, case-sensitive, never matches a passphrase-less build', () => {
  assert.equal(passphraseMatches('correct horse', 'correct horse'), true)
  assert.equal(passphraseMatches('  correct horse  ', 'correct horse'), true)
  assert.equal(passphraseMatches('Correct Horse', 'correct horse'), false)
  assert.equal(passphraseMatches('wrong', 'correct horse'), false)
  assert.equal(passphraseMatches('', 'correct horse'), false)
  // a build with no passphrase must accept NOTHING — including empty input
  assert.equal(passphraseMatches('', ''), false)
  assert.equal(passphraseMatches('anything', undefined), false)
})

test('STAGING_PASS_STORAGE_KEY is stable (persisted grants reference it)', () => {
  assert.equal(STAGING_PASS_STORAGE_KEY, 'hexdek_staging_pass')
})

test('previewParamMatches: ?preview=<passphrase> bypasses; wrong/missing does not', () => {
  assert.equal(STAGING_PREVIEW_PARAM, 'preview')
  // correct phrase in the query — with and without a leading '?'
  assert.equal(previewParamMatches('?preview=ribbit', 'ribbit'), true)
  assert.equal(previewParamMatches('preview=ribbit', 'ribbit'), true)
  // alongside other params, and URL-encoded values decode
  assert.equal(previewParamMatches('?foo=1&preview=ribbit&bar=2', 'ribbit'), true)
  assert.equal(previewParamMatches('?preview=correct%20horse', 'correct horse'), true)
  // wrong phrase / wrong param / no param / empty search
  assert.equal(previewParamMatches('?preview=nope', 'ribbit'), false)
  assert.equal(previewParamMatches('?preview=Ribbit', 'ribbit'), false) // case-sensitive
  assert.equal(previewParamMatches('?other=ribbit', 'ribbit'), false)
  assert.equal(previewParamMatches('', 'ribbit'), false)
  assert.equal(previewParamMatches(null, 'ribbit'), false)
  assert.equal(previewParamMatches(undefined, 'ribbit'), false)
  // a build with no passphrase accepts nothing — even ?preview=
  assert.equal(previewParamMatches('?preview=ribbit', ''), false)
  assert.equal(previewParamMatches('?preview=', ''), false)
})
