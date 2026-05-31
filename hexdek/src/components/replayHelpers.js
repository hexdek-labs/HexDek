// Pure helpers powering the replay slider. Lives in a .js file so
// node --test can exercise it directly without a JSX transform.

// Max snapshots cached in the per-session ring buffer. Each
// GameSnapshot runs ~1-5 KB, so 120 caps the cache at roughly
// 600 KB worst case. Tuned for a typical 4-seat Commander game's
// step count (80 turns × ~6 phase-steps × dedup factor).
export const MAX_SNAPSHOT_HISTORY = 120

// snapshotKey returns a stable string identifying the (turn, phase,
// step, active_seat) tuple of a GameSnapshot. Used to dedupe
// successive WS payloads — if the engine re-emits a snapshot for the
// same step (which happens during long resolution chains), we don't
// want N duplicate entries in the scrub history.
export function snapshotKey(game) {
  if (!game) return ''
  const t = game.turn ?? 0
  const p = game.phase ?? ''
  const s = game.step ?? ''
  const a = game.active_seat ?? -1
  return `${t}|${p}|${s}|${a}`
}

// pushSnapshot appends a new snapshot to the history ring, deduping
// against the most recent entry (no point keeping back-to-back
// duplicates of the same step) and trimming the front to keep the
// cache below MAX_SNAPSHOT_HISTORY. Returns the new history slice;
// the caller is responsible for setting state with the return value.
//
// Pure: takes the current history + a new snapshot, returns the new
// history. Doesn't mutate the input.
export function pushSnapshot(history, snap, cap = MAX_SNAPSHOT_HISTORY) {
  if (!snap) return history
  const list = Array.isArray(history) ? history : []
  const lastKey = list.length > 0 ? snapshotKey(list[list.length - 1]) : ''
  const newKey = snapshotKey(snap)
  if (newKey === lastKey) return list
  const next = list.concat(snap)
  if (next.length > cap) {
    return next.slice(next.length - cap)
  }
  return next
}

// scrubLabel formats a snapshot's (turn, phase) tuple as a short
// label for the slider tick. Mirrors the rt() helper in Spectator
// ("R3T9 / COMBAT / DECLARE_ATTACKERS") but compressed for the slider
// strip: "R3T9 · COMBAT".
export function scrubLabel(snap, nSeats = 4) {
  if (!snap) return ''
  const turn = snap.turn ?? 0
  const round = Math.ceil(turn / Math.max(nSeats, 1))
  const phase = (snap.phase || '').toUpperCase()
  return phase ? `R${round}T${turn} · ${phase}` : `R${round}T${turn}`
}

// clampScrubIndex clamps an arbitrary index into the valid range
// [-1, history.length - 1]. -1 is the "live" sentinel (render the
// current real-time game, not a snapshot).
export function clampScrubIndex(idx, historyLength) {
  if (!Number.isFinite(idx)) return -1
  if (idx < -1) return -1
  if (idx >= historyLength) return historyLength - 1
  return idx
}

// effectiveGame returns the game state that should be rendered: the
// scrubbed snapshot when the slider is active, otherwise the live
// game. Centralizes the "live vs scrubbed" decision so consumers
// don't repeat the conditional.
export function effectiveGame(liveGame, history, scrubIndex) {
  if (scrubIndex == null || scrubIndex < 0) return liveGame
  if (!Array.isArray(history) || scrubIndex >= history.length) return liveGame
  return history[scrubIndex]
}
