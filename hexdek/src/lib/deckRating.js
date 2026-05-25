// Pure storage helpers for the per-deck owner-private rating widget.
//
// The rating is a subjective 1–5 score the deck owner sets for their own
// tracking ("how does this deck FEEL to play"). It's intentionally stored
// in localStorage rather than the backend: it's per-person, per-browser,
// and never aggregated — the whole point is that it's the owner's note to
// themselves. A future PR can promote this to a server-side field if we
// ever want to surface it cross-device, without changing the call sites
// that read/write through this helper.
//
// Storage is injectable so the helpers can be unit-tested with a Map
// instead of the real localStorage. In production callers pass nothing
// and the helpers default to globalThis.localStorage with try/catch so
// Safari private mode (which throws on writes) degrades to a no-op.

const KEY_PREFIX = 'hexdek_deck_rating'

// Build the namespaced storage key. Owner-prefixed so a shared browser
// doesn't surface user A's ratings to user B after sign-out/sign-in.
export function deckRatingKey(userSlug, deckKey) {
  if (!userSlug || !deckKey) return null
  return `${KEY_PREFIX}::${String(userSlug).toLowerCase()}::${deckKey}`
}

// Clamp + integer-coerce a rating. 0 is the sentinel for "no rating".
// Anything outside [1,5] returns 0 so storage round-trips are stable.
export function normalizeRating(value) {
  const n = Number(value)
  if (!Number.isFinite(n)) return 0
  const i = Math.round(n)
  if (i < 1) return 0
  if (i > 5) return 0
  return i
}

// Resolve the active storage backend. We pass `storage` as an optional
// param so the unit tests can hand in a Map-like. Production callers
// reach for globalThis.localStorage; SSR / Safari private mode degrade
// to null and the read/write helpers no-op instead of throwing.
function resolveStorage(storage) {
  if (storage) return storage
  try {
    return globalThis.localStorage || null
  } catch {
    return null
  }
}

// Returns 0 when there's no rating, the user is signed-out, or storage
// is unavailable. Callers treat 0 as "unrated".
export function getDeckRating(userSlug, deckKey, storage) {
  const key = deckRatingKey(userSlug, deckKey)
  if (!key) return 0
  const s = resolveStorage(storage)
  if (!s) return 0
  try {
    return normalizeRating(s.getItem(key))
  } catch {
    return 0
  }
}

// Writes the rating. A rating of 0 (or anything that normalizes to 0)
// deletes the key so localStorage doesn't accumulate "unrated" entries.
// Returns the normalized rating actually persisted (0 on a delete).
export function setDeckRating(userSlug, deckKey, value, storage) {
  const key = deckRatingKey(userSlug, deckKey)
  if (!key) return 0
  const s = resolveStorage(storage)
  if (!s) return normalizeRating(value)
  const rating = normalizeRating(value)
  try {
    if (rating === 0) s.removeItem(key)
    else s.setItem(key, String(rating))
  } catch {
    // Safari private mode etc. — swallow; the in-memory UI still updates.
  }
  return rating
}
