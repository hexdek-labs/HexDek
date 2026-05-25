// Pure helpers for the deck-page bracket changelog. Tracks bracket
// transitions across deck versions as the owner (or any viewer)
// observes them, so the page can render "B3 (upgraded from B2 on
// May 24)" when the deck has actually moved.
//
// Why localStorage and not the backend:
//
//   The version DAG (internal/versioning + internal/heimdall) stores
//   per-version card-count and archetype-evolution, but does NOT
//   persist per-version bracket. The scaffolding to read historical
//   bracket from a `<owner>/versions/<id>_v<N>.strategy.json` snapshot
//   exists (deck_archive.go:81) but the snapshots aren't written by
//   the engine today — the comment in that file calls it out as
//   "forward-compatible scaffolding". Until that ships, the only
//   bracket signal we get is the CURRENT-version reading from Freya
//   analysis at page-load time.
//
//   So this helper records bracket SIGHTINGS — one per (deck, version)
//   pair the viewer has loaded — and infers transitions from those.
//   First visit shows no changelog (we just established a baseline);
//   subsequent visits to a new version with a different bracket
//   produce a transition entry. The recording is idempotent on
//   (deck, version) so revisiting the same head doesn't double-log.
//
//   A future PR that wires server-side per-version bracket can call
//   `computeBracketChangelog` against the server's lineage directly,
//   without changing this helper's signature.

const STORAGE_PREFIX = 'hexdek_bracket_history'
// Soft cap so a long-lived deck doesn't accumulate unbounded sightings.
// Tail-trimmed: oldest dropped first.
export const MAX_SIGHTINGS = 24

function resolveStorage(storage) {
  if (storage) return storage
  try {
    return globalThis.localStorage || null
  } catch {
    return null
  }
}

export function bracketHistoryKey(deckKey) {
  if (!deckKey) return null
  return `${STORAGE_PREFIX}::${deckKey}`
}

// Bracket strings come from Freya as "1".."5" or "3.5" or "?".
// Normalize to a number (float) so "3" and 3 and "3.0" all match;
// returns null for unknown / "?" / "" / missing.
export function normalizeBracket(value) {
  if (value == null || value === '') return null
  const s = String(value).trim().toLowerCase()
  if (!s || s === '?') return null
  const n = parseFloat(s)
  return Number.isFinite(n) ? n : null
}

// Returns the persisted sightings (oldest → newest). Defensive parse —
// any corruption returns [] so a bad write can't permanently shadow the
// changelog.
export function getBracketHistory(deckKey, storage) {
  const key = bracketHistoryKey(deckKey)
  if (!key) return []
  const s = resolveStorage(storage)
  if (!s) return []
  let raw
  try {
    raw = s.getItem(key)
  } catch {
    return []
  }
  if (!raw) return []
  let parsed
  try {
    parsed = JSON.parse(raw)
  } catch {
    return []
  }
  if (!Array.isArray(parsed)) return []
  // Drop any entry that's not the expected shape. Explicit null/undefined
  // guards because Number(null) is 0 — a finite number that would slip past
  // a naive isFinite check and re-introduce the corrupt row we just rejected.
  return parsed.filter(e => e
    && typeof e === 'object'
    && e.version != null && Number.isFinite(Number(e.version))
    && e.bracket != null && Number.isFinite(Number(e.bracket))
    && e.observedAt != null && Number.isFinite(Number(e.observedAt))
  )
}

// Adds a sighting if (deck, version) hasn't been logged before. If the
// version IS logged but with a different bracket — which can happen
// when Freya re-analyzes the current version and the bracket calc
// changes — the existing entry's bracket is updated and `observedAt`
// is bumped, since that bracket change is itself worth showing.
//
// Returns the resulting history array (same shape as getBracketHistory).
// `version` may be null/undefined for decks with no version DAG yet —
// in that case we record under synthetic version 0 so the helper still
// produces a baseline.
export function recordBracketSighting(deckKey, version, bracket, observedAt, storage) {
  const key = bracketHistoryKey(deckKey)
  const b = normalizeBracket(bracket)
  if (!key || b == null) return getBracketHistory(deckKey, storage)
  const v = Number.isFinite(Number(version)) ? Number(version) : 0
  const at = Number.isFinite(Number(observedAt)) ? Number(observedAt) : Math.floor(Date.now() / 1000)

  const s = resolveStorage(storage)
  const current = getBracketHistory(deckKey, storage)
  const next = current.slice()
  const existingIdx = next.findIndex(e => Number(e.version) === v)
  if (existingIdx >= 0) {
    if (Number(next[existingIdx].bracket) === b) return current  // no-op
    next[existingIdx] = { version: v, bracket: b, observedAt: at }
  } else {
    next.push({ version: v, bracket: b, observedAt: at })
  }
  // Sort version asc so changelog computation walks chronologically.
  next.sort((a, b1) => Number(a.version) - Number(b1.version))
  // Trim from the front (oldest versions) if we've blown past the cap.
  while (next.length > MAX_SIGHTINGS) next.shift()

  if (s) {
    try { s.setItem(key, JSON.stringify(next)) } catch { /* private mode: in-memory only */ }
  }
  return next
}

// Walks the sightings array and emits one transition per actual change.
// A transition is { fromVersion, toVersion, fromBracket, toBracket,
// direction: 'up' | 'down', observedAt }.
//
// `observedAt` on the transition is the `observedAt` of the destination
// sighting — when the higher-numbered version's bracket was first seen.
// That's the answer to "when did the deck move?".
export function computeBracketChangelog(history) {
  if (!Array.isArray(history) || history.length < 2) return []
  const out = []
  for (let i = 1; i < history.length; i++) {
    const prev = history[i - 1]
    const cur = history[i]
    const pb = Number(prev.bracket)
    const cb = Number(cur.bracket)
    if (!Number.isFinite(pb) || !Number.isFinite(cb)) continue
    if (pb === cb) continue
    out.push({
      fromVersion: Number(prev.version),
      toVersion: Number(cur.version),
      fromBracket: pb,
      toBracket: cb,
      direction: cb > pb ? 'up' : 'down',
      observedAt: Number(cur.observedAt),
    })
  }
  return out
}

// Helper for "Month D" formatting. Standalone so the UI can avoid
// hand-rolling another date formatter; injectable nowFn for tests.
export function formatTransitionDate(epochSec, fmtOpts = { month: 'short', day: 'numeric' }) {
  const n = Number(epochSec)
  if (!Number.isFinite(n) || n <= 0) return ''
  try {
    return new Date(n * 1000).toLocaleDateString(undefined, fmtOpts)
  } catch {
    return ''
  }
}

// Builds the final user-facing string: "B3 (upgraded from B2 on May 24)".
// Returns '' when there's no transition to show.
export function describeLatestTransition(history) {
  const log = computeBracketChangelog(history)
  if (log.length === 0) return ''
  const t = log[log.length - 1]
  // "B3.5" reads better than "B3" for fractional brackets — Freya
  // emits half-step calls like "3.5" between integer rungs.
  const fmt = (b) => Number.isInteger(b) ? `B${b}` : `B${b}`
  const verb = t.direction === 'up' ? 'upgraded' : 'downgraded'
  const when = formatTransitionDate(t.observedAt)
  const tail = when ? ` on ${when}` : ''
  return `${fmt(t.toBracket)} (${verb} from ${fmt(t.fromBracket)}${tail})`
}
