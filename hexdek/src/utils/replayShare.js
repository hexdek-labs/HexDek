// Replay deep-link helpers.
//
// Lets a spectator copy a URL that points at a specific moment in a
// past game — turn number plus an optional highlighted seat. The
// Report.jsx ReplayScrubber consumes `parseShareParams` on mount to
// restore the scrubber to that moment, and writes a fresh URL via
// `buildShareParams` when the user clicks "share."
//
// Param shape (URLSearchParams):
//   t=<turn>          turn number to jump to. Required for the link
//                     to "mean something" — bare /report/123 still
//                     loads the report, just at turn 1.
//   seat=<i>          0-based seat index to highlight (0..3). Omitted
//                     when no seat is selected.
//
// Both values are validated on the way in: anything non-numeric, out
// of range, or NaN comes back as null so the consumer can fall back
// to the default state without throwing.

export const SHARE_PARAM_TURN = 't'
export const SHARE_PARAM_SEAT = 'seat'

// parseShareParams accepts either a query string ("?t=5&seat=2"), a
// URLSearchParams, or a plain object. Returns { turn, seat } where
// each field is a finite number or null.
//
// `maxTurn` and `maxSeats` clamp range — if the caller knows the
// timeline length or seat count, anything outside [1, maxTurn] /
// [0, maxSeats - 1] yields null. Pass 0/undefined to skip the range
// check (parser still rejects negatives + NaN).
export function parseShareParams(input, { maxTurn = 0, maxSeats = 0 } = {}) {
  const get = paramReader(input)
  if (!get) return { turn: null, seat: null }
  const turn = readPositiveInt(get(SHARE_PARAM_TURN), maxTurn, 1)
  const seat = readPositiveInt(get(SHARE_PARAM_SEAT), maxSeats ? maxSeats - 1 : 0, 0)
  return { turn, seat }
}

// buildShareParams returns a URLSearchParams-compatible object that
// represents the given { turn, seat }. The caller serializes via
// .toString() or merges into an existing URLSearchParams.
//
// Null / undefined fields are omitted so the resulting URL stays
// short. A turn of 0 or 1 IS included because turn 1 is a meaningful
// share target; only `null` is treated as "leave out."
export function buildShareParams({ turn, seat } = {}) {
  const p = new URLSearchParams()
  if (Number.isFinite(turn) && turn >= 1) p.set(SHARE_PARAM_TURN, String(Math.trunc(turn)))
  if (Number.isFinite(seat) && seat >= 0) p.set(SHARE_PARAM_SEAT, String(Math.trunc(seat)))
  return p
}

// buildShareUrl combines a base URL (typically window.location.origin
// + pathname) with the share params. Existing query parameters on the
// base ARE preserved — we merge ours in via URLSearchParams so an
// already-deep-linked report (e.g. ?tab=summary) doesn't lose its
// other state when the user copies a share link.
export function buildShareUrl(baseUrl, params) {
  if (typeof baseUrl !== 'string' || baseUrl.length === 0) return ''
  const built = buildShareParams(params)
  // Split off the existing query string so we can merge without
  // pulling in a URL polyfill (the constructor needs an origin in
  // some node versions for relative URLs).
  const [path, query = ''] = baseUrl.split('?')
  const merged = new URLSearchParams(query)
  for (const [k, v] of built.entries()) merged.set(k, v)
  // Also strip our keys when not set, so unsharing a moment removes
  // stale t=/seat= leftover from a prior share.
  if (!built.has(SHARE_PARAM_TURN)) merged.delete(SHARE_PARAM_TURN)
  if (!built.has(SHARE_PARAM_SEAT)) merged.delete(SHARE_PARAM_SEAT)
  const qs = merged.toString()
  return qs ? `${path}?${qs}` : path
}

// ---- internals ----

function paramReader(input) {
  if (input == null) return null
  if (typeof input === 'string') {
    const s = input.startsWith('?') ? input.slice(1) : input
    const sp = new URLSearchParams(s)
    return (k) => sp.get(k)
  }
  if (typeof input.get === 'function') {
    return (k) => input.get(k)
  }
  if (typeof input === 'object') {
    return (k) => (k in input ? String(input[k]) : null)
  }
  return null
}

function readPositiveInt(raw, max, min) {
  if (raw == null || raw === '') return null
  const n = Number(raw)
  if (!Number.isFinite(n)) return null
  const v = Math.trunc(n)
  if (v < min) return null
  if (max && v > max) return null
  return v
}
