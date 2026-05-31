// Run with: node --test hexdek/src/components/comboToast.test.js
//
// Unit coverage for the SSE-to-toast helpers powering the spectator
// combo-fired notification (useComboNotifications hook).

import { test } from 'node:test'
import assert from 'node:assert/strict'

import {
  formatComboMessage,
  parseSSEEventData,
  isComboFiredEvent,
} from './comboToastHelpers.js'

// -----------------------------------------------------------------------------
// formatComboMessage
// -----------------------------------------------------------------------------

test('formatComboMessage 2-card combo lists both names inline', () => {
  const got = formatComboMessage({
    participating_cards: ['Heliod, Sun-Crowned', 'Walking Ballista'],
  })
  assert.equal(got, '🎯 Combo fired: Heliod, Sun-Crowned + Walking Ballista')
})

test('formatComboMessage 1-card combo lists the lone name', () => {
  // Edge case: an "infinite" detected by a single self-recursive card
  // (e.g. some prison loops). Engine doesn't dedup IIDs to 1 often
  // but the formatter handles it cleanly.
  const got = formatComboMessage({
    participating_cards: ['Pestermite'],
  })
  assert.equal(got, '🎯 Combo fired: Pestermite')
})

test('formatComboMessage 3+ cards uses loop framing with lead name', () => {
  const got = formatComboMessage({
    cycle_length: 5,
    participating_cards: ['Atraxa, Praetors\' Voice', 'Vorinclex', 'Hardened Scales', 'Doubling Season', 'Inexorable Tide'],
  })
  assert.equal(got, '🎯 Combo fired: 5-card loop (Atraxa, Praetors\' Voice + 4 more)')
})

test('formatComboMessage falls back to cards.length when cycle_length missing', () => {
  const got = formatComboMessage({
    participating_cards: ['A', 'B', 'C', 'D'],
  })
  assert.equal(got, '🎯 Combo fired: 4-card loop (A + 3 more)')
})

test('formatComboMessage no cards but controller present → controller fallback', () => {
  const got = formatComboMessage({
    controller_commander: 'Krenko, Mob Boss',
  })
  assert.equal(got, '🎯 Combo fired (Krenko, Mob Boss)')
})

test('formatComboMessage empty payload → bare default', () => {
  assert.equal(formatComboMessage({}), '🎯 Combo fired')
  assert.equal(formatComboMessage(null), '🎯 Combo fired')
  assert.equal(formatComboMessage(undefined), '🎯 Combo fired')
})

test('formatComboMessage filters non-string cards defensively', () => {
  const got = formatComboMessage({
    participating_cards: ['Heliod', null, 42, 'Walking Ballista', ''],
  })
  // Filter drops null/number/empty-string; only the 2 real names survive.
  assert.equal(got, '🎯 Combo fired: Heliod + Walking Ballista')
})

test('formatComboMessage non-array participating_cards ignored', () => {
  const got = formatComboMessage({
    participating_cards: 'not-an-array',
    controller_commander: 'Atraxa',
  })
  assert.equal(got, '🎯 Combo fired (Atraxa)')
})

// -----------------------------------------------------------------------------
// parseSSEEventData
// -----------------------------------------------------------------------------

test('parseSSEEventData unboxes SSE wrapper (snake_case)', () => {
  const payload = {
    event: 'game.combo_fired',
    data: { game_id: 7, cycle_length: 3 },
    timestamp: '2026-05-31T00:00:00Z',
  }
  assert.deepEqual(parseSSEEventData(payload), { game_id: 7, cycle_length: 3 })
})

test('parseSSEEventData unboxes WS wrapper (CamelCase)', () => {
  // The /ws/events WebSocket bridge marshals Event with CamelCase
  // field names via the json-tagged Event struct — verify the
  // helper tolerates that shape too.
  const payload = {
    Type: 'game.combo_fired',
    Data: { game_id: 7 },
    Timestamp: '2026-05-31T00:00:00Z',
  }
  assert.deepEqual(parseSSEEventData(payload), { game_id: 7 })
})

test('parseSSEEventData null on unrecognized shape', () => {
  assert.equal(parseSSEEventData({ foo: 'bar' }), null)
  assert.equal(parseSSEEventData(null), null)
  assert.equal(parseSSEEventData('string'), null)
})

// -----------------------------------------------------------------------------
// isComboFiredEvent
// -----------------------------------------------------------------------------

test('isComboFiredEvent recognizes SSE wrapper', () => {
  assert.equal(isComboFiredEvent({ event: 'game.combo_fired', data: {} }), true)
})

test('isComboFiredEvent recognizes WS wrapper', () => {
  assert.equal(isComboFiredEvent({ Type: 'game.combo_fired', Data: {} }), true)
})

test('isComboFiredEvent rejects other event types', () => {
  assert.equal(isComboFiredEvent({ event: 'game.end' }), false)
  assert.equal(isComboFiredEvent({ event: 'game.player_loss' }), false)
  assert.equal(isComboFiredEvent({ Type: 'pong' }), false)
})

test('isComboFiredEvent rejects malformed input', () => {
  assert.equal(isComboFiredEvent(null), false)
  assert.equal(isComboFiredEvent({}), false)
  assert.equal(isComboFiredEvent('string'), false)
})
