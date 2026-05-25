// Replay comparison helpers.
//
// Side-by-side replay viewer for two games on the same turn clock.
// The Report screen owns the data fetch (for both the primary game
// and the comparison game); this module owns the math that maps a
// shared turn number to each timeline's snapshot index and derives
// per-seat compare summaries.
//
// Why "shared turn" and not "shared idx":
//   Different games have different timeline lengths. Game A might
//   end on turn 14 while game B grinds to turn 22. The user wants
//   to see "turn 8 in both games" — the snapshot index for turn 8
//   isn't guaranteed to be the same in both timelines (gaps, mulligan
//   snapshots, etc). All sync math uses turn NUMBER and resolves to
//   index per timeline.

// findIdxAtTurn returns the closest preceding snapshot index for the
// given turn number. If the exact turn isn't in the timeline (gap),
// returns the most recent snapshot whose turn ≤ target. Returns 0
// for an empty timeline.
export function findIdxAtTurn(timeline, turn) {
  if (!Array.isArray(timeline) || timeline.length === 0) return 0
  if (!Number.isFinite(turn)) return 0
  let chosen = 0
  for (let i = 0; i < timeline.length; i++) {
    const t = timeline[i]?.turn
    if (!Number.isFinite(t)) continue
    if (t === turn) return i
    if (t <= turn) chosen = i
    else break
  }
  return chosen
}

// commonTurnRange returns { min, max } of the overlap between two
// timelines' turn ranges. Returns null when there's no overlap
// (one is empty, or one's max < other's min).
export function commonTurnRange(timelineA, timelineB) {
  const rangeA = turnRange(timelineA)
  const rangeB = turnRange(timelineB)
  if (!rangeA || !rangeB) return null
  const min = Math.max(rangeA.min, rangeB.min)
  const max = Math.min(rangeA.max, rangeB.max)
  if (min > max) return null
  return { min, max }
}

// unionTurnRange returns { min, max } of the union of the two
// timelines' turn ranges. Returns null when both are empty. Useful
// when you want to scrub across the entire span — past the shorter
// game's end the compare panel falls back to "GAME ENDED."
export function unionTurnRange(timelineA, timelineB) {
  const rangeA = turnRange(timelineA)
  const rangeB = turnRange(timelineB)
  if (!rangeA && !rangeB) return null
  if (!rangeA) return rangeB
  if (!rangeB) return rangeA
  return {
    min: Math.min(rangeA.min, rangeB.min),
    max: Math.max(rangeA.max, rangeB.max),
  }
}

function turnRange(timeline) {
  if (!Array.isArray(timeline) || timeline.length === 0) return null
  let min = Infinity
  let max = -Infinity
  for (const s of timeline) {
    const t = s?.turn
    if (!Number.isFinite(t)) continue
    if (t < min) min = t
    if (t > max) max = t
  }
  if (min === Infinity) return null
  return { min, max }
}

// mapSharedTurn returns each timeline's snapshot for the requested
// shared turn. Each side reports whether its game has ENDED by that
// turn (i.e. the shared turn is past that game's last snapshot turn
// + 1) so the compare panel can render "GAME ENDED" instead of a
// stale last-turn snapshot.
export function mapSharedTurn(timelineA, timelineB, sharedTurn) {
  return {
    a: snapshotAtSharedTurn(timelineA, sharedTurn),
    b: snapshotAtSharedTurn(timelineB, sharedTurn),
  }
}

function snapshotAtSharedTurn(timeline, sharedTurn) {
  if (!Array.isArray(timeline) || timeline.length === 0) {
    return { idx: 0, snapshot: null, ended: true, beforeStart: false }
  }
  const range = turnRange(timeline)
  if (!range) return { idx: 0, snapshot: null, ended: true, beforeStart: false }
  if (sharedTurn < range.min) {
    return { idx: 0, snapshot: null, ended: false, beforeStart: true }
  }
  if (sharedTurn > range.max) {
    const lastIdx = timeline.length - 1
    return { idx: lastIdx, snapshot: timeline[lastIdx], ended: true, beforeStart: false }
  }
  const idx = findIdxAtTurn(timeline, sharedTurn)
  return { idx, snapshot: timeline[idx], ended: false, beforeStart: false }
}

// compareSeatDeltas returns one entry per seat with the per-seat
// difference between the two snapshots (A − B). Seats are aligned by
// index (seat 0 vs seat 0, seat 1 vs seat 1, ...). Pads with zeros
// when one game has more seats than the other so the UI can still
// render a row.
export function compareSeatDeltas(snapA, snapB) {
  const seatsA = snapA?.seats || []
  const seatsB = snapB?.seats || []
  const n = Math.max(seatsA.length, seatsB.length)
  const out = []
  for (let i = 0; i < n; i++) {
    const sa = seatsA[i] || {}
    const sb = seatsB[i] || {}
    out.push({
      seat: i,
      lifeDelta:        num(sa.life)        - num(sb.life),
      handDelta:        num(sa.hand_size)   - num(sb.hand_size),
      librarayDelta:    num(sa.library_size) - num(sb.library_size),
      graveyardDelta:   num(sa.gy_size)     - num(sb.gy_size),
      battlefieldDelta: (sa.battlefield?.length || 0) - (sb.battlefield?.length || 0),
    })
  }
  return out
}

function num(v) {
  return Number.isFinite(v) ? v : 0
}

// SHARE_PARAM_COMPARE — the URL key used by Report to thread the
// comparison game id through `?vs=<id>`. Lives here (not in
// replayShare.js) so the compare module owns its own URL key and
// share/compare can evolve independently.
export const SHARE_PARAM_COMPARE = 'vs'
