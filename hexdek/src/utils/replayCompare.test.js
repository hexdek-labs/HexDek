// Run with: node --test hexdek/src/utils/replayCompare.test.js

import { test } from 'node:test'
import assert from 'node:assert/strict'

import {
  SHARE_PARAM_COMPARE,
  findIdxAtTurn,
  commonTurnRange,
  unionTurnRange,
  mapSharedTurn,
  compareSeatDeltas,
} from './replayCompare.js'

function tl(turns) {
  return turns.map(t => ({ turn: t, seats: [] }))
}

// -----------------------------------------------------------------------------
// findIdxAtTurn
// -----------------------------------------------------------------------------

test('findIdxAtTurn returns exact match when present', () => {
  const t = tl([1, 2, 3, 4, 5])
  assert.equal(findIdxAtTurn(t, 3), 2)
  assert.equal(findIdxAtTurn(t, 1), 0)
  assert.equal(findIdxAtTurn(t, 5), 4)
})

test('findIdxAtTurn returns closest preceding when there is a gap', () => {
  const t = tl([1, 2, 5, 8])
  assert.equal(findIdxAtTurn(t, 3), 1, 'no turn 3 snapshot — fall back to turn 2')
  assert.equal(findIdxAtTurn(t, 7), 2, 'no turn 7 — fall back to turn 5')
})

test('findIdxAtTurn returns 0 for an empty timeline or bad input', () => {
  assert.equal(findIdxAtTurn([], 5), 0)
  assert.equal(findIdxAtTurn(null, 5), 0)
  assert.equal(findIdxAtTurn(tl([1, 2, 3]), NaN), 0)
})

test('findIdxAtTurn for turn below the first snapshot returns the first idx', () => {
  // The compare panel keys its rendering off "beforeStart"; the idx
  // itself just needs to be in range. 0 is correct.
  assert.equal(findIdxAtTurn(tl([3, 4, 5]), 1), 0)
})

// -----------------------------------------------------------------------------
// commonTurnRange / unionTurnRange
// -----------------------------------------------------------------------------

test('commonTurnRange returns the overlapping window', () => {
  assert.deepEqual(commonTurnRange(tl([1, 2, 3, 4, 5]), tl([3, 4, 5, 6, 7])), { min: 3, max: 5 })
  assert.deepEqual(commonTurnRange(tl([1, 2, 3]), tl([1, 2, 3])), { min: 1, max: 3 })
})

test('commonTurnRange returns null when the windows do not overlap', () => {
  assert.equal(commonTurnRange(tl([1, 2, 3]), tl([5, 6, 7])), null)
})

test('commonTurnRange returns null when either timeline is empty', () => {
  assert.equal(commonTurnRange([], tl([1, 2, 3])), null)
  assert.equal(commonTurnRange(tl([1, 2, 3]), []), null)
})

test('unionTurnRange spans both timelines', () => {
  assert.deepEqual(unionTurnRange(tl([1, 2, 3]), tl([2, 3, 4, 5])), { min: 1, max: 5 })
  assert.deepEqual(unionTurnRange(tl([5, 6, 7]), tl([1, 2])), { min: 1, max: 7 })
})

test('unionTurnRange falls back to whichever side has data', () => {
  assert.deepEqual(unionTurnRange([], tl([1, 2, 3])), { min: 1, max: 3 })
  assert.deepEqual(unionTurnRange(tl([4, 5]), null), { min: 4, max: 5 })
})

test('unionTurnRange returns null for two empty inputs', () => {
  assert.equal(unionTurnRange([], []), null)
  assert.equal(unionTurnRange(null, null), null)
})

// -----------------------------------------------------------------------------
// mapSharedTurn
// -----------------------------------------------------------------------------

test('mapSharedTurn picks the matching snapshot in each timeline', () => {
  const a = tl([1, 2, 3, 4, 5])
  const b = tl([1, 2, 3, 4, 5])
  const m = mapSharedTurn(a, b, 3)
  assert.equal(m.a.idx, 2)
  assert.equal(m.b.idx, 2)
  assert.equal(m.a.snapshot.turn, 3)
  assert.equal(m.b.snapshot.turn, 3)
  assert.equal(m.a.ended, false)
  assert.equal(m.b.ended, false)
})

test('mapSharedTurn marks the side past its end as "ended"', () => {
  const a = tl([1, 2, 3])
  const b = tl([1, 2, 3, 4, 5, 6])
  const m = mapSharedTurn(a, b, 5)
  assert.equal(m.a.ended, true, 'A ended at turn 3; turn 5 is past')
  assert.equal(m.a.snapshot.turn, 3, 'still returns A\'s last snapshot for reference')
  assert.equal(m.b.ended, false)
})

test('mapSharedTurn marks the side before its start as "beforeStart"', () => {
  // Compare a normal game (starts turn 1) with one that has a turn-3
  // first snapshot (synthetic edge case). At sharedTurn=1 the
  // turn-3-starter is beforeStart.
  const a = tl([1, 2, 3])
  const b = tl([3, 4, 5])
  const m = mapSharedTurn(a, b, 1)
  assert.equal(m.b.beforeStart, true)
  assert.equal(m.b.snapshot, null)
  assert.equal(m.a.beforeStart, false)
})

test('mapSharedTurn handles a gap by falling back to closest preceding', () => {
  const a = tl([1, 2, 5, 8])
  const b = tl([1, 2, 3, 4])
  const m = mapSharedTurn(a, b, 4)
  assert.equal(m.a.idx, 1, 'A has no turn-4 snapshot — falls back to turn 2')
  assert.equal(m.a.snapshot.turn, 2)
  assert.equal(m.b.idx, 3)
  assert.equal(m.b.snapshot.turn, 4)
})

test('mapSharedTurn on empty/null timelines returns ended sentinels', () => {
  const m = mapSharedTurn([], null, 5)
  assert.equal(m.a.ended, true)
  assert.equal(m.b.ended, true)
  assert.equal(m.a.snapshot, null)
  assert.equal(m.b.snapshot, null)
})

// -----------------------------------------------------------------------------
// compareSeatDeltas
// -----------------------------------------------------------------------------

test('compareSeatDeltas returns A − B for matching seats', () => {
  const snapA = { seats: [
    { life: 30, hand_size: 5, library_size: 80, gy_size: 4, battlefield: [{}, {}, {}] },
  ]}
  const snapB = { seats: [
    { life: 22, hand_size: 3, library_size: 78, gy_size: 7, battlefield: [{}, {}] },
  ]}
  const d = compareSeatDeltas(snapA, snapB)
  assert.equal(d.length, 1)
  assert.equal(d[0].lifeDelta,        8)   // 30 − 22
  assert.equal(d[0].handDelta,        2)   // 5 − 3
  assert.equal(d[0].librarayDelta,    2)   // 80 − 78
  assert.equal(d[0].graveyardDelta,  -3)   // 4 − 7
  assert.equal(d[0].battlefieldDelta, 1)   // 3 − 2
})

test('compareSeatDeltas pads with zeros when seat counts differ', () => {
  const snapA = { seats: [{ life: 40, hand_size: 7, library_size: 99, gy_size: 0, battlefield: [{}] }] }
  const snapB = { seats: [
    { life: 30, hand_size: 5, library_size: 90, gy_size: 2, battlefield: [] },
    { life: 25, hand_size: 4, library_size: 85, gy_size: 3, battlefield: [{}, {}] },
  ]}
  const d = compareSeatDeltas(snapA, snapB)
  assert.equal(d.length, 2)
  // Seat 1 (only in B) — A side is treated as zero so deltas come out negative.
  assert.equal(d[1].lifeDelta,        -25)
  assert.equal(d[1].handDelta,        -4)
  assert.equal(d[1].battlefieldDelta, -2)
})

test('compareSeatDeltas tolerates missing fields by treating them as zero', () => {
  const snapA = { seats: [{ life: 30, battlefield: [{}] }] }
  const snapB = { seats: [{ life: 22 }] }
  const d = compareSeatDeltas(snapA, snapB)
  assert.equal(d[0].lifeDelta, 8)
  assert.equal(d[0].handDelta, 0)
  assert.equal(d[0].battlefieldDelta, 1)
})

test('compareSeatDeltas handles a null snapshot on either side', () => {
  // Common case: one game ended at turn N, the other still has
  // snapshots after. mapSharedTurn returns snapshot=null on the
  // ended side past N; compareSeatDeltas should not crash.
  const snap = { seats: [{ life: 30, hand_size: 5 }] }
  const dA = compareSeatDeltas(null, snap)
  const dB = compareSeatDeltas(snap, null)
  assert.equal(dA[0].lifeDelta, -30)
  assert.equal(dB[0].lifeDelta,  30)
})

// -----------------------------------------------------------------------------
// URL key
// -----------------------------------------------------------------------------

test('SHARE_PARAM_COMPARE is "vs"', () => {
  assert.equal(SHARE_PARAM_COMPARE, 'vs')
})
