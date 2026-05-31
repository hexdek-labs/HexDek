// Run with: node --test hexdek/src/hooks/turnAnnouncer.test.js
//
// Pure-helper coverage for the screen-reader turn announcer
// powering useTurnAnnouncer. Covers the formatting + dedup-key
// + phase-phrase mapping without spinning up React.

import { test } from 'node:test'
import assert from 'node:assert/strict'

import {
  rt,
  phasePhrase,
  announcementKey,
  buildAnnouncement,
} from './turnAnnouncerHelpers.js'

// -----------------------------------------------------------------------------
// rt — turn label
// -----------------------------------------------------------------------------

test('rt renders R{round}T{turn} for 4-seat games', () => {
  assert.equal(rt(1, 4), 'R1T1')
  assert.equal(rt(4, 4), 'R1T4')
  assert.equal(rt(5, 4), 'R2T5')
  assert.equal(rt(9, 4), 'R3T9')
})

test('rt handles arbitrary seat counts', () => {
  assert.equal(rt(7, 3), 'R3T7')
  assert.equal(rt(6, 6), 'R1T6')
  assert.equal(rt(7, 6), 'R2T7')
})

test('rt defaults to 4 seats when nSeats invalid', () => {
  assert.equal(rt(5, 0), 'R2T5')
  assert.equal(rt(5, -1), 'R2T5')
})

// -----------------------------------------------------------------------------
// phasePhrase — phase/step → long-form English
// -----------------------------------------------------------------------------

test('phasePhrase maps beginning steps to long form', () => {
  assert.equal(phasePhrase('beginning', 'untap'), 'untap step')
  assert.equal(phasePhrase('beginning', 'upkeep'), 'upkeep')
  assert.equal(phasePhrase('beginning', 'draw'), 'draw step')
  assert.equal(phasePhrase('beginning', ''), 'beginning phase')
})

test('phasePhrase splits main into precombat / postcombat', () => {
  assert.equal(phasePhrase('main', 'precombat_main'), 'precombat main')
  assert.equal(phasePhrase('main', 'postcombat_main'), 'postcombat main')
  assert.equal(phasePhrase('main', 'main2'), 'postcombat main')
  assert.equal(phasePhrase('main', ''), 'precombat main')
})

test('phasePhrase folds combat substep variants', () => {
  // Engine emits all three variants across different code paths.
  assert.equal(phasePhrase('combat', 'begin_of_combat'), 'beginning of combat')
  assert.equal(phasePhrase('combat', 'beginning_of_combat'), 'beginning of combat')
  assert.equal(phasePhrase('combat', 'combat_start'), 'beginning of combat')

  assert.equal(phasePhrase('combat', 'declare_attackers'), 'declare attackers')
  assert.equal(phasePhrase('combat', 'declare_blockers'), 'declare blockers')
  assert.equal(phasePhrase('combat', 'first_strike_damage'), 'first-strike damage')
  assert.equal(phasePhrase('combat', 'combat_damage'), 'combat damage')

  assert.equal(phasePhrase('combat', 'end_of_combat'), 'end of combat')
  assert.equal(phasePhrase('combat', 'combat_end'), 'end of combat')
})

test('phasePhrase folds ending step variants', () => {
  assert.equal(phasePhrase('ending', 'end'), 'end step')
  assert.equal(phasePhrase('ending', 'end_step'), 'end step')
  assert.equal(phasePhrase('ending', 'end_of_turn'), 'end step')
  assert.equal(phasePhrase('ending', 'cleanup'), 'cleanup')
  assert.equal(phasePhrase('ending', ''), 'ending phase')
})

test('phasePhrase falls back to humanized step when phase unknown', () => {
  assert.equal(phasePhrase('', 'declare_attackers'), 'declare attackers')
  assert.equal(phasePhrase('unknown_phase', 'foo_bar'), 'foo bar')
})

test('phasePhrase returns empty for fully-missing input', () => {
  assert.equal(phasePhrase('', ''), '')
  assert.equal(phasePhrase(null, null), '')
})

// -----------------------------------------------------------------------------
// announcementKey — dedup signature
// -----------------------------------------------------------------------------

test('announcementKey identical tuples produce identical keys', () => {
  const a = { turn: 5, phase: 'main', step: 'precombat_main', active_seat: 1 }
  const b = { turn: 5, phase: 'main', step: 'precombat_main', active_seat: 1 }
  assert.equal(announcementKey(a), announcementKey(b))
})

test('announcementKey distinguishes each tuple field', () => {
  const base = { turn: 5, phase: 'main', step: 'precombat_main', active_seat: 1 }
  assert.notEqual(announcementKey(base), announcementKey({ ...base, turn: 6 }))
  assert.notEqual(announcementKey(base), announcementKey({ ...base, phase: 'combat' }))
  assert.notEqual(announcementKey(base), announcementKey({ ...base, step: 'postcombat_main' }))
  assert.notEqual(announcementKey(base), announcementKey({ ...base, active_seat: 2 }))
})

test('announcementKey nil-safe', () => {
  assert.equal(announcementKey(null), '')
  assert.equal(announcementKey(undefined), '')
})

// -----------------------------------------------------------------------------
// buildAnnouncement — the SR sentence
// -----------------------------------------------------------------------------

test('buildAnnouncement renders the canonical sentence', () => {
  const game = { turn: 9, phase: 'combat', step: 'declare_attackers', active_seat: 0 }
  const seats = [{ commander: 'Atraxa' }, { commander: 'Krenko' }, {}, {}]
  assert.equal(
    buildAnnouncement(game, seats),
    "R3T9, Atraxa's declare attackers.",
  )
})

test('buildAnnouncement falls back without commander', () => {
  const game = { turn: 4, phase: 'main', step: 'precombat_main', active_seat: 5 }
  const seats = [{ commander: 'A' }] // active_seat out of range
  assert.equal(buildAnnouncement(game, seats), 'R1T4, precombat main.')
})

test('buildAnnouncement falls back without phase phrase', () => {
  const game = { turn: 4, active_seat: 0 }
  const seats = [{ commander: 'Atraxa' }]
  assert.equal(buildAnnouncement(game, seats), "R1T4, Atraxa's turn.")
})

test('buildAnnouncement bare turn-only when nothing else available', () => {
  assert.equal(buildAnnouncement({ turn: 1 }, []), 'R1T1.')
})

test('buildAnnouncement says game ended when finished', () => {
  const game = { turn: 12, phase: 'main', finished: true }
  assert.equal(buildAnnouncement(game, []), 'Game ended.')
})

test('buildAnnouncement empty for null input', () => {
  assert.equal(buildAnnouncement(null, []), '')
})
