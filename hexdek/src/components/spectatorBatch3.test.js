// Run with: node --test hexdek/src/components/spectatorBatch3.test.js
//
// Unit coverage for the pure helpers powering the R60 spectator UX
// batch 3 components: CounterBadge + CommandZoneStrip + ReplaySlider.

import { test } from 'node:test'
import assert from 'node:assert/strict'

import {
  counterBadgeColor,
  formatCounterLabel,
  orderCounters,
} from './countersHelpers.js'

import {
  MAX_SNAPSHOT_HISTORY,
  snapshotKey,
  pushSnapshot,
  scrubLabel,
  clampScrubIndex,
  effectiveGame,
} from './replayHelpers.js'

// -----------------------------------------------------------------------------
// countersHelpers
// -----------------------------------------------------------------------------

test('formatCounterLabel known kinds get short symbols', () => {
  assert.equal(formatCounterLabel('+1/+1'), '+1')
  assert.equal(formatCounterLabel('-1/-1'), '-1')
  assert.equal(formatCounterLabel('loyalty'), 'L')
  assert.equal(formatCounterLabel('charge'), 'C')
})

test('formatCounterLabel falls back to 3-char upper for unknown', () => {
  assert.equal(formatCounterLabel('flying_squirrel'), 'FLY')
})

test('formatCounterLabel empty/null input returns empty', () => {
  assert.equal(formatCounterLabel(''), '')
  assert.equal(formatCounterLabel(null), '')
})

test('orderCounters returns empty for nil/empty input', () => {
  assert.deepEqual(orderCounters(null), [])
  assert.deepEqual(orderCounters({}), [])
})

test('orderCounters filters zero counts defensively', () => {
  const got = orderCounters({ '+1/+1': 3, '-1/-1': 0, 'loyalty': 2 })
  const kinds = got.map(e => e.kind)
  assert.deepEqual(kinds, ['+1/+1', 'loyalty'])
})

test('orderCounters sorts by priority desc', () => {
  // +1/+1 (100) > -1/-1 (95) > loyalty (90) > charge (70).
  const got = orderCounters({ 'charge': 1, '+1/+1': 1, 'loyalty': 1, '-1/-1': 1 })
  assert.deepEqual(got.map(e => e.kind), ['+1/+1', '-1/-1', 'loyalty', 'charge'])
})

test('orderCounters puts unknown kinds at the end (alphabetical within)', () => {
  const got = orderCounters({ 'zest': 1, '+1/+1': 1, 'apple': 1 })
  assert.equal(got[0].kind, '+1/+1')
  assert.equal(got[1].kind, 'apple')
  assert.equal(got[2].kind, 'zest')
})

test('orderCounters carries count + label per entry', () => {
  const got = orderCounters({ '+1/+1': 5 })
  assert.deepEqual(got, [{ kind: '+1/+1', count: 5, label: '+1' }])
})

test('counterBadgeColor maps canonical kinds to var() colors', () => {
  assert.equal(counterBadgeColor('+1/+1'), 'var(--ok)')
  assert.equal(counterBadgeColor('-1/-1'), 'var(--danger)')
  assert.equal(counterBadgeColor('loyalty'), 'var(--accent, #c8a26b)')
  assert.equal(counterBadgeColor('unknown_kind'), 'var(--ink-2)')
})

// -----------------------------------------------------------------------------
// replayHelpers
// -----------------------------------------------------------------------------

test('snapshotKey identical (turn, phase, step, active_seat) → identical key', () => {
  const a = { turn: 5, phase: 'main', step: 'precombat_main', active_seat: 1 }
  const b = { turn: 5, phase: 'main', step: 'precombat_main', active_seat: 1 }
  assert.equal(snapshotKey(a), snapshotKey(b))
})

test('snapshotKey changes when any field differs', () => {
  const base = { turn: 5, phase: 'main', step: 'precombat_main', active_seat: 1 }
  assert.notEqual(snapshotKey(base), snapshotKey({ ...base, turn: 6 }))
  assert.notEqual(snapshotKey(base), snapshotKey({ ...base, phase: 'combat' }))
  assert.notEqual(snapshotKey(base), snapshotKey({ ...base, step: 'postcombat_main' }))
  assert.notEqual(snapshotKey(base), snapshotKey({ ...base, active_seat: 2 }))
})

test('snapshotKey nil-safe', () => {
  assert.equal(snapshotKey(null), '')
  assert.equal(snapshotKey(undefined), '')
})

test('pushSnapshot deduplicates against the most recent entry', () => {
  const a = { turn: 1, phase: 'main', step: 'precombat_main', active_seat: 0 }
  let h = pushSnapshot([], a)
  h = pushSnapshot(h, a)
  h = pushSnapshot(h, a)
  assert.equal(h.length, 1, 'three identical pushes should yield 1 entry')
})

test('pushSnapshot accumulates distinct snapshots', () => {
  const a = { turn: 1, phase: 'main', step: 'precombat_main', active_seat: 0 }
  const b = { turn: 1, phase: 'combat', step: 'begin_of_combat', active_seat: 0 }
  let h = pushSnapshot([], a)
  h = pushSnapshot(h, b)
  assert.equal(h.length, 2)
})

test('pushSnapshot trims front when over cap', () => {
  let h = []
  for (let i = 0; i < 150; i++) {
    h = pushSnapshot(h, { turn: i, phase: 'main', step: 's', active_seat: 0 })
  }
  assert.equal(h.length, MAX_SNAPSHOT_HISTORY)
  // Front trimmed: the oldest remaining snapshot is at index 30.
  assert.equal(h[0].turn, 150 - MAX_SNAPSHOT_HISTORY)
  assert.equal(h[h.length - 1].turn, 149)
})

test('pushSnapshot respects custom cap', () => {
  let h = []
  for (let i = 0; i < 10; i++) {
    h = pushSnapshot(h, { turn: i, phase: 'main', step: 's', active_seat: 0 }, 5)
  }
  assert.equal(h.length, 5)
  assert.equal(h[0].turn, 5)
})

test('pushSnapshot nil snap is a no-op', () => {
  const h = pushSnapshot([{ turn: 1 }], null)
  assert.equal(h.length, 1)
})

test('scrubLabel renders turn + phase token', () => {
  const snap = { turn: 9, phase: 'combat', step: 'declare_attackers' }
  assert.equal(scrubLabel(snap, 4), 'R3T9 · COMBAT')
})

test('scrubLabel handles missing phase gracefully', () => {
  assert.equal(scrubLabel({ turn: 4 }, 4), 'R1T4')
})

test('scrubLabel nil-safe', () => {
  assert.equal(scrubLabel(null), '')
})

test('clampScrubIndex clamps below -1 to -1', () => {
  assert.equal(clampScrubIndex(-5, 10), -1)
})

test('clampScrubIndex clamps above length-1', () => {
  assert.equal(clampScrubIndex(100, 10), 9)
})

test('clampScrubIndex passes through valid values', () => {
  assert.equal(clampScrubIndex(0, 10), 0)
  assert.equal(clampScrubIndex(5, 10), 5)
  assert.equal(clampScrubIndex(-1, 10), -1)
})

test('clampScrubIndex non-finite → -1 (live)', () => {
  assert.equal(clampScrubIndex(NaN, 10), -1)
  assert.equal(clampScrubIndex(undefined, 10), -1)
})

test('effectiveGame returns live when scrubIndex is -1', () => {
  const live = { turn: 10 }
  const hist = [{ turn: 5 }, { turn: 6 }]
  assert.equal(effectiveGame(live, hist, -1), live)
})

test('effectiveGame returns history entry when scrubbed', () => {
  const live = { turn: 10 }
  const hist = [{ turn: 5 }, { turn: 6 }, { turn: 7 }]
  assert.equal(effectiveGame(live, hist, 1), hist[1])
})

test('effectiveGame falls back to live on out-of-range index', () => {
  const live = { turn: 10 }
  const hist = [{ turn: 5 }]
  assert.equal(effectiveGame(live, hist, 99), live)
})
