// Run with: node --test hexdek/src/utils/replayTimeline.test.js

import { test } from 'node:test'
import assert from 'node:assert/strict'

import {
  KEY_MOMENT_KINDS,
  classifyEvent,
  summarizeTurn,
  extractKeyMoments,
  momentMeta,
  countsTooltip,
} from './replayTimeline.js'

// -----------------------------------------------------------------------------
// classifyEvent — kind dispatch
// -----------------------------------------------------------------------------

test('classifyEvent maps elimination kinds to "elimination"', () => {
  for (const k of ['elimination', 'sba_704_5a', 'sba_704_5b', 'sba_704_5c', 'sba_704_5d']) {
    assert.equal(classifyEvent({ kind: k, action: 'seat 2 lost' }), 'elimination', k)
  }
})

test('classifyEvent maps simple kinds to their moment id', () => {
  assert.equal(classifyEvent({ kind: 'combat',     action: 'attacks' }),     'combat')
  assert.equal(classifyEvent({ kind: 'counter',    action: 'Counterspell' }),'counter')
  assert.equal(classifyEvent({ kind: 'removal',    action: 'destroys X' }),  'removal')
  assert.equal(classifyEvent({ kind: 'reanimate',  action: 'returns Y' }),   'reanimate')
  assert.equal(classifyEvent({ kind: 'extra_turn', action: 'takes extra turn' }), 'extra_turn')
})

test('classifyEvent returns null for neutral kinds', () => {
  for (const k of ['land', 'draw', 'scry', 'surveil', 'tap', 'untap', 'mill', 'token', 'etb', 'shuffle']) {
    assert.equal(classifyEvent({ kind: k, action: '' }), null, `${k} should be neutral`)
  }
})

test('classifyEvent returns null for bad input', () => {
  assert.equal(classifyEvent(null), null)
  assert.equal(classifyEvent(undefined), null)
  assert.equal(classifyEvent('not an event'), null)
  assert.equal(classifyEvent({}), null)
})

test('classifyEvent is case-insensitive on kind', () => {
  assert.equal(classifyEvent({ kind: 'COMBAT', action: '' }), 'combat')
  assert.equal(classifyEvent({ kind: 'Removal', action: '' }), 'removal')
})

// -----------------------------------------------------------------------------
// Wipe vs removal — action-text upgrade path
// -----------------------------------------------------------------------------

test('classifyEvent upgrades removal → wipe when the action mentions mass effects', () => {
  const cases = [
    'casts Wrath of God',
    'casts Damnation',
    'destroys all creatures',
    'exiles all permanents',
    'deals 13 damage to each creature',
    'casts Toxic Deluge',
    'casts Cyclonic Rift',
    'casts Blasphemous Act',
    'casts Supreme Verdict',
    'casts Day of Judgment',
  ]
  for (const action of cases) {
    assert.equal(classifyEvent({ kind: 'removal', action }), 'wipe',
      `should classify as wipe: ${action}`)
  }
})

test('classifyEvent keeps removal as "removal" for single-target effects', () => {
  for (const action of ['casts Swords to Plowshares', 'casts Path to Exile', 'destroys target creature']) {
    assert.equal(classifyEvent({ kind: 'removal', action }), 'removal',
      `should remain removal: ${action}`)
  }
})

// -----------------------------------------------------------------------------
// Big cast detection
// -----------------------------------------------------------------------------

test('classifyEvent flags commander/planeswalker casts as big_cast', () => {
  assert.equal(classifyEvent({ kind: 'cast', action: 'casts Atraxa, the Commander' }), 'big_cast')
  assert.equal(classifyEvent({ kind: 'cast', action: 'casts planeswalker Teferi' }), 'big_cast')
  assert.equal(classifyEvent({ kind: 'cast', action: 'Jace, the Mind Sculptor (planeswalker)' }), 'big_cast')
})

test('classifyEvent leaves ordinary casts alone (no badge)', () => {
  assert.equal(classifyEvent({ kind: 'cast', action: 'casts Sol Ring' }), null)
  assert.equal(classifyEvent({ kind: 'cast', action: 'casts Lightning Bolt' }), null)
})

// -----------------------------------------------------------------------------
// summarizeTurn — primary picked by priority
// -----------------------------------------------------------------------------

test('summarizeTurn picks the highest-priority moment present', () => {
  const snap = {
    turn: 7,
    events: [
      { kind: 'cast',    action: 'casts Sol Ring' },        // (neutral)
      { kind: 'combat',  action: 'attacks' },                // combat (idx 3)
      { kind: 'removal', action: 'casts Wrath of God' },     // wipe (idx 1)
      { kind: 'counter', action: 'casts Counterspell' },     // counter (idx 4)
    ],
  }
  const s = summarizeTurn(snap)
  assert.equal(s.turn, 7)
  assert.equal(s.primary, 'wipe', 'wipe outranks combat / counter / removal')
})

test('summarizeTurn counts every classifiable event', () => {
  const snap = {
    turn: 5,
    events: [
      { kind: 'removal', action: 'destroys creature A' },
      { kind: 'removal', action: 'destroys creature B' },
      { kind: 'counter', action: 'counters spell C' },
      { kind: 'combat',  action: 'attacks' },
      { kind: 'land',    action: 'plays a land' },           // neutral, skipped
    ],
  }
  const s = summarizeTurn(snap)
  assert.equal(s.counts.get('removal'), 2)
  assert.equal(s.counts.get('counter'), 1)
  assert.equal(s.counts.get('combat'),  1)
  assert.equal(s.counts.has('land'),    false)
  assert.equal(s.totalKeyEvents, 4)
})

test('summarizeTurn returns null primary for a quiet turn', () => {
  const snap = {
    turn: 2,
    events: [
      { kind: 'land', action: 'plays Forest' },
      { kind: 'draw', action: 'draws a card' },
    ],
  }
  const s = summarizeTurn(snap)
  assert.equal(s.primary, null)
  assert.equal(s.totalKeyEvents, 0)
})

test('summarizeTurn handles missing / empty events array', () => {
  assert.equal(summarizeTurn({ turn: 1 }).primary, null)
  assert.equal(summarizeTurn({ turn: 1, events: [] }).primary, null)
  assert.equal(summarizeTurn(null).primary, null)
})

test('summarizeTurn picks elimination over wipe even when both fire same turn', () => {
  // E.g. Cyclonic Rift overload that finishes off a player at 0
  // life and wipes the board same turn.
  const snap = {
    turn: 11,
    events: [
      { kind: 'removal',     action: 'casts Cyclonic Rift' },
      { kind: 'elimination', action: 'seat 1 lost from commander damage' },
    ],
  }
  assert.equal(summarizeTurn(snap).primary, 'elimination')
})

// -----------------------------------------------------------------------------
// extractKeyMoments — full-timeline walk
// -----------------------------------------------------------------------------

test('extractKeyMoments returns one summary per snapshot, in order', () => {
  const timeline = [
    { turn: 1, events: [{ kind: 'land', action: '' }] },                              // quiet
    { turn: 2, events: [{ kind: 'combat', action: 'attacks' }] },                    // combat
    { turn: 3, events: [{ kind: 'removal', action: 'casts Damnation' }] },           // wipe
    { turn: 4, events: [{ kind: 'elimination', action: 'seat 0 lost' }] },           // elimination
  ]
  const list = extractKeyMoments(timeline)
  assert.equal(list.length, 4)
  assert.deepEqual(list.map(s => s.primary), [null, 'combat', 'wipe', 'elimination'])
})

test('extractKeyMoments tolerates an empty timeline or bad input', () => {
  assert.deepEqual(extractKeyMoments([]), [])
  assert.deepEqual(extractKeyMoments(null), [])
  assert.deepEqual(extractKeyMoments(undefined), [])
})

// -----------------------------------------------------------------------------
// Static helpers
// -----------------------------------------------------------------------------

test('momentMeta returns the matching kind entry', () => {
  assert.equal(momentMeta('combat').label, 'Combat')
  assert.equal(momentMeta('wipe').color,   'var(--danger)')
  assert.equal(momentMeta('unknown'),      null)
  assert.equal(momentMeta(null),           null)
})

test('countsTooltip builds a readable comma-separated summary', () => {
  const counts = new Map([
    ['removal', 3],
    ['counter', 1],
    ['combat',  2],
  ])
  const text = countsTooltip(counts)
  // Order follows KEY_MOMENT_KINDS priority, not insertion order.
  assert.equal(text, '2 combats, 1 counter, 3 removals')
})

test('countsTooltip returns empty string when nothing happened', () => {
  assert.equal(countsTooltip(new Map()), '')
  assert.equal(countsTooltip(null), '')
})

test('KEY_MOMENT_KINDS ordering is stable (elimination first)', () => {
  assert.equal(KEY_MOMENT_KINDS[0].id, 'elimination',
    'priority order: elimination must lead')
})
