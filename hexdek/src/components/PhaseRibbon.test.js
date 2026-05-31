// Run with: node --test hexdek/src/components/PhaseRibbon.test.js
//
// Unit coverage for the pure helpers powering PhaseRibbon:
// phaseBucket, stepLabel, nextActiveSeat. The component itself is
// just a thin SVG-free render layer over these; the helpers carry
// every conditional that affects what spectators see.

import { test } from 'node:test'
import assert from 'node:assert/strict'

import { phaseBucket, stepLabel, nextActiveSeat } from './phaseRibbonHelpers.js'

// -----------------------------------------------------------------------------
// phaseBucket
// -----------------------------------------------------------------------------

test('phaseBucket maps beginning to 0', () => {
  assert.equal(phaseBucket('beginning', 'untap'), 0)
  assert.equal(phaseBucket('beginning', 'upkeep'), 0)
  assert.equal(phaseBucket('beginning', 'draw'), 0)
})

test('phaseBucket splits main into precombat (1) and postcombat (3)', () => {
  assert.equal(phaseBucket('main', 'precombat_main'), 1)
  assert.equal(phaseBucket('main', 'postcombat_main'), 3)
  assert.equal(phaseBucket('main', 'main2'), 3)
  assert.equal(phaseBucket('main', 'post_combat_main'), 3)
  // Default to MAIN 1 when step is absent or unknown.
  assert.equal(phaseBucket('main', ''), 1)
  assert.equal(phaseBucket('main', 'unknown_substep'), 1)
})

test('phaseBucket maps combat to 2 regardless of substep', () => {
  assert.equal(phaseBucket('combat', 'begin_of_combat'), 2)
  assert.equal(phaseBucket('combat', 'declare_attackers'), 2)
  assert.equal(phaseBucket('combat', 'combat_damage'), 2)
  assert.equal(phaseBucket('combat', 'end_of_combat'), 2)
})

test('phaseBucket maps ending to 4', () => {
  assert.equal(phaseBucket('ending', 'end'), 4)
  assert.equal(phaseBucket('ending', 'cleanup'), 4)
})

test('phaseBucket returns -1 for unknown / empty phase', () => {
  assert.equal(phaseBucket('', ''), -1)
  assert.equal(phaseBucket(null, null), -1)
  assert.equal(phaseBucket('bogus_phase', 'untap'), -1)
})

test('phaseBucket tolerates casing + whitespace', () => {
  // The engine emits lowercase but defensive callers may upper-case.
  assert.equal(phaseBucket('BEGINNING', 'UNTAP'), 0)
  assert.equal(phaseBucket('Main', 'PRECOMBAT_MAIN'), 1)
})

// -----------------------------------------------------------------------------
// stepLabel
// -----------------------------------------------------------------------------

test('stepLabel produces human-readable labels for canonical steps', () => {
  assert.equal(stepLabel('untap'), 'UNTAP')
  assert.equal(stepLabel('precombat_main'), 'PRECOMBAT')
  assert.equal(stepLabel('postcombat_main'), 'POSTCOMBAT')
  assert.equal(stepLabel('declare_attackers'), 'DECLARE ATKS')
  assert.equal(stepLabel('declare_blockers'), 'DECLARE BLKS')
  assert.equal(stepLabel('combat_damage'), 'COMBAT DAMAGE')
  assert.equal(stepLabel('end_step'), 'END STEP')
  assert.equal(stepLabel('cleanup'), 'CLEANUP')
})

test('stepLabel folds engine variant spellings to one label', () => {
  // The engine emits these three variants across different code paths;
  // the UI should not surface that inconsistency to spectators.
  assert.equal(stepLabel('begin_of_combat'), 'BEGIN COMBAT')
  assert.equal(stepLabel('beginning_of_combat'), 'BEGIN COMBAT')
  assert.equal(stepLabel('combat_start'), 'BEGIN COMBAT')

  assert.equal(stepLabel('end_of_combat'), 'END COMBAT')
  assert.equal(stepLabel('combat_end'), 'END COMBAT')

  assert.equal(stepLabel('end'), 'END STEP')
  assert.equal(stepLabel('end_step'), 'END STEP')
  assert.equal(stepLabel('end_of_turn'), 'END STEP')
})

test('stepLabel returns empty for empty input', () => {
  assert.equal(stepLabel(''), '')
  assert.equal(stepLabel(null), '')
  assert.equal(stepLabel(undefined), '')
})

test('stepLabel upper-cases unknown steps as a fallback', () => {
  assert.equal(stepLabel('exotic_new_step'), 'EXOTIC_NEW_STEP')
})

// -----------------------------------------------------------------------------
// nextActiveSeat
// -----------------------------------------------------------------------------

test('nextActiveSeat returns the next index in 4-player order', () => {
  const seats = [{}, {}, {}, {}]
  assert.equal(nextActiveSeat(seats, 0), 1)
  assert.equal(nextActiveSeat(seats, 1), 2)
  assert.equal(nextActiveSeat(seats, 2), 3)
  assert.equal(nextActiveSeat(seats, 3), 0)
})

test('nextActiveSeat skips lost seats', () => {
  // Seat 1 eliminated → from seat 0 the next active is seat 2.
  const seats = [{}, { lost: true }, {}, {}]
  assert.equal(nextActiveSeat(seats, 0), 2)
  // From seat 2, next is seat 3 (seat 0 is fine but seat 3 comes first).
  assert.equal(nextActiveSeat(seats, 2), 3)
  // From seat 3, wrap past lost seat 1 → seat 0.
  assert.equal(nextActiveSeat(seats, 3), 0)
})

test('nextActiveSeat returns -1 when only the active seat survives', () => {
  const seats = [{}, { lost: true }, { lost: true }, { lost: true }]
  assert.equal(nextActiveSeat(seats, 0), -1)
})

test('nextActiveSeat handles empty input', () => {
  assert.equal(nextActiveSeat([], 0), -1)
  assert.equal(nextActiveSeat(null, 0), -1)
})

test('nextActiveSeat tolerates active-seat out of range', () => {
  // Defensive: an invalid active-seat shouldn't crash the ribbon.
  const seats = [{}, {}, {}, {}]
  // (5 + 1) % 4 = 2, so this happens to return seat 2.
  assert.equal(nextActiveSeat(seats, 5), 2)
})
