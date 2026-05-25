// Run with: node --test hexdek/src/utils/actionExplain.test.js

import { test } from 'node:test'
import assert from 'node:assert/strict'

import {
  EVAL_DIM_LABELS,
  ARCHETYPE_BIAS_SENTENCES,
  classifyAction,
  isNotableAction,
  topEvalDimensions,
  formatEval,
  explainAction,
} from './actionExplain.js'

const SEAT_EVAL = {
  archetype: 'combo',
  board_presence:     0.20,
  card_advantage:     0.55,
  mana_advantage:     0.10,
  life_resource:     -0.05,
  combo_proximity:    0.90, // top
  threat_exposure:    0.65, // second
  commander_progress: 0.30,
  graveyard_value:    0.15,
  score:              0.42,
  budget: 200,
  budget_used: 187,
}

// -----------------------------------------------------------------------------
// classifyAction
// -----------------------------------------------------------------------------

test('classifyAction picks the more specific cast category when text matches', () => {
  assert.equal(classifyAction({ kind: 'cast', action: 'casts Wrath of God' }),           'wipe')
  assert.equal(classifyAction({ kind: 'cast', action: 'casts planeswalker Teferi' }),    'planeswalker')
  assert.equal(classifyAction({ kind: 'cast', action: 'casts Atraxa, the Commander' }),  'commander_cast')
  assert.equal(classifyAction({ kind: 'cast', action: 'casts Sol Ring' }),               'cast')
})

test('classifyAction maps non-cast kinds to their category', () => {
  assert.equal(classifyAction({ kind: 'combat',     action: 'attacks with 3' }), 'combat')
  assert.equal(classifyAction({ kind: 'counter',    action: 'Counterspell' }),   'counter')
  assert.equal(classifyAction({ kind: 'removal',    action: 'destroys X' }),     'removal')
  assert.equal(classifyAction({ kind: 'reanimate',  action: 'returns Y' }),      'reanimate')
  assert.equal(classifyAction({ kind: 'extra_turn', action: 'extra turn' }),     'extra_turn')
  assert.equal(classifyAction({ kind: 'activate',   action: 'Sol Ring: tap' }),  'activate')
})

test('classifyAction upgrades removal → wipe via keyword scan', () => {
  assert.equal(classifyAction({ kind: 'removal', action: 'casts Damnation' }), 'wipe')
  assert.equal(classifyAction({ kind: 'removal', action: 'casts Toxic Deluge' }), 'wipe')
  assert.equal(classifyAction({ kind: 'removal', action: 'casts Swords to Plowshares' }), 'removal')
})

test('classifyAction returns null for unnotable kinds', () => {
  for (const k of ['land', 'draw', 'scry', 'surveil', 'tap', 'untap', 'discard', 'mill']) {
    assert.equal(classifyAction({ kind: k, action: '' }), null, `${k} is not a notable kind`)
  }
})

test('classifyAction returns null for malformed input', () => {
  assert.equal(classifyAction(null), null)
  assert.equal(classifyAction(undefined), null)
  assert.equal(classifyAction({}), null)
  assert.equal(classifyAction('not an event'), null)
})

test('isNotableAction is true exactly when classifyAction returns a category', () => {
  assert.equal(isNotableAction({ kind: 'cast', action: 'casts Sol Ring' }), true)
  assert.equal(isNotableAction({ kind: 'land', action: '' }), false)
})

// -----------------------------------------------------------------------------
// topEvalDimensions
// -----------------------------------------------------------------------------

test('topEvalDimensions returns the N highest axes in descending order', () => {
  const top2 = topEvalDimensions(SEAT_EVAL, 2)
  assert.equal(top2.length, 2)
  assert.equal(top2[0].key, 'combo_proximity', 'highest = combo_proximity (0.90)')
  assert.equal(top2[1].key, 'threat_exposure', 'next   = threat_exposure (0.65)')
})

test('topEvalDimensions ignores non-axis fields like score / archetype', () => {
  const top = topEvalDimensions(SEAT_EVAL, 10)
  for (const t of top) {
    assert.ok(t.key in EVAL_DIM_LABELS, `${t.key} should be a known dimension`)
  }
  assert.equal(top.length, 8, 'eight named axes only')
})

test('topEvalDimensions handles missing / malformed input', () => {
  assert.deepEqual(topEvalDimensions(null), [])
  assert.deepEqual(topEvalDimensions({}, 5), [])
  assert.deepEqual(topEvalDimensions(SEAT_EVAL, 0), [])
})

test('topEvalDimensions clamps requests larger than the dimension count', () => {
  const top = topEvalDimensions(SEAT_EVAL, 99)
  assert.equal(top.length, 8)
})

// -----------------------------------------------------------------------------
// formatEval
// -----------------------------------------------------------------------------

test('formatEval renders signed floats with two decimals', () => {
  assert.equal(formatEval(0.34),   '+0.34')
  assert.equal(formatEval(-0.10),  '-0.10')
  assert.equal(formatEval(0),      '+0.00')
})

test('formatEval returns em-dash for missing values', () => {
  assert.equal(formatEval(null),        '—')
  assert.equal(formatEval(undefined),   '—')
  assert.equal(formatEval(NaN),         '—')
  assert.equal(formatEval('abc'),       '—')
})

// -----------------------------------------------------------------------------
// explainAction — end-to-end
// -----------------------------------------------------------------------------

test('explainAction returns null for unnotable events', () => {
  assert.equal(explainAction({ kind: 'land', action: '' }, SEAT_EVAL, 'X'), null)
  assert.equal(explainAction(null, SEAT_EVAL, 'X'), null)
})

test('explainAction headline matches the categorized action', () => {
  const wipe = explainAction({ kind: 'cast', action: 'casts Wrath of God' }, SEAT_EVAL, 'Hero')
  assert.equal(wipe.category, 'wipe')
  assert.equal(wipe.headline, 'Board wipe')
})

test('explainAction surfaces archetype + bias sentence when archetype is known', () => {
  const e = explainAction({ kind: 'cast', action: 'casts a spell' }, SEAT_EVAL, 'Combo Pilot')
  const labels = e.bullets.map(b => b.label)
  assert.ok(labels.includes('Archetype'), 'archetype bullet present')
  assert.ok(labels.includes('Bias'),      'bias sentence present')
  const biasLine = e.bullets.find(b => b.label === 'Bias').value
  assert.equal(biasLine, ARCHETYPE_BIAS_SENTENCES.combo)
})

test('explainAction surfaces the top-2 eval dimensions', () => {
  const e = explainAction({ kind: 'combat', action: 'attacks' }, SEAT_EVAL, 'Hero')
  const labels = e.bullets.map(b => b.label)
  assert.ok(labels.includes('Combo proximity'), 'top-1 dim should land in the tooltip')
  assert.ok(labels.includes('Threat exposure'), 'top-2 dim should land in the tooltip')
})

test('explainAction does NOT double-list threat_exposure when it\'s already the top dim', () => {
  const evalWithThreatTop = {
    ...SEAT_EVAL,
    combo_proximity: 0.10,
    threat_exposure: 0.95, // now top
  }
  const e = explainAction({ kind: 'counter', action: 'counters spell' }, evalWithThreatTop, 'Hero')
  const threatBullets = e.bullets.filter(b => b.label === 'Threat exposure' || b.label === 'Threat read')
  assert.equal(threatBullets.length, 1, 'no redundant Threat read row')
})

test('explainAction surfaces threat_exposure separately when it is NOT in the top-2', () => {
  const lowThreatEval = {
    ...SEAT_EVAL,
    combo_proximity: 0.90,
    card_advantage:  0.80,
    threat_exposure: 0.05, // ranked outside top-2
  }
  const e = explainAction({ kind: 'cast', action: 'casts a spell' }, lowThreatEval, 'Hero')
  assert.ok(e.bullets.some(b => b.label === 'Threat read'),
    'low threat_exposure should still surface as a separate Threat read bullet')
})

test('explainAction omits archetype bullets when archetype is missing', () => {
  const noArchEval = { ...SEAT_EVAL, archetype: undefined }
  const e = explainAction({ kind: 'cast', action: 'casts a spell' }, noArchEval, 'Hero')
  const labels = e.bullets.map(b => b.label)
  assert.equal(labels.includes('Archetype'), false)
  assert.equal(labels.includes('Bias'),      false)
})

test('explainAction includes pilot commander name when supplied', () => {
  const e = explainAction({ kind: 'combat', action: 'attacks' }, SEAT_EVAL, 'Atraxa, Praetors\' Voice // Phyrexia')
  const pilot = e.bullets.find(b => b.label === 'Pilot')
  assert.equal(pilot.value, "ATRAXA, PRAETORS' VOICE")
})

test('explainAction works with no eval at all (rare edge case)', () => {
  const e = explainAction({ kind: 'combat', action: 'attacks' }, null, null)
  assert.equal(e.category, 'combat')
  assert.equal(e.headline, 'Combat decision')
  assert.equal(e.bullets.length, 0, 'no eval ⇒ no per-dim bullets')
})
