// Pure helpers for the opt-in anonymous-share feature.
//
// The "share" URL is a variant of the existing /share/:owner/:id route
// with `?anon=1` appended; SharePreview reads the flag and strips owner-
// revealing UI (tape label, corner badge, doc title, "open in archive"
// link) when it's set. The OG meta description was already owner-free,
// so social embeds also render anonymized.
//
// Opt-in lives in localStorage rather than the backend: the owner toggles
// whether THEIR deck-page surfaces the anon-share URL. The URL itself
// works regardless (the route is reachable by anyone who has the link),
// so the opt-in is a UX gate on the owner's side — not a server-enforced
// access control. The UI copy in DeckShareDisclosure makes that explicit.
// A future PR can promote this to a backend token without changing the
// call sites that use buildAnonShareUrl / isAnonymousShare.

const SHARE_OPTIN_KEY_PREFIX = 'hexdek_share_optin'
export const ANON_QUERY_FLAG = 'anon'

// Build the anonymized share URL for a given deck. `origin` is typically
// `window.location.origin`; we accept it as a parameter so the helper is
// testable without DOM and so callers can build absolute URLs for
// non-browser surfaces (clipboard, QR generators).
export function buildAnonShareUrl(origin, owner, id) {
  if (!origin || !owner || !id) return ''
  const safeOrigin = String(origin).replace(/\/+$/, '')
  return `${safeOrigin}/share/${owner}/${id}?${ANON_QUERY_FLAG}=1`
}

// Truthy detection — accepts a URLSearchParams, a leading-`?` string, or
// a bare query string. Returns true when the anon flag is present and set
// to anything other than '0' / 'false'. Permissive on purpose: a typo'd
// `?anon` (bare flag) should still anonymize, since the worst case is the
// user accidentally sees less.
export function isAnonymousShare(search) {
  if (!search) return false
  let params
  if (search instanceof URLSearchParams) {
    params = search
  } else {
    const str = String(search).replace(/^\?/, '')
    if (!str) return false
    params = new URLSearchParams(str)
  }
  if (!params.has(ANON_QUERY_FLAG)) return false
  const v = params.get(ANON_QUERY_FLAG)
  if (v == null || v === '') return true       // bare ?anon → opt in
  const lower = v.toLowerCase()
  return lower !== '0' && lower !== 'false' && lower !== 'no'
}

// Per-deck opt-in key. The owner toggles this flag on their browser to
// surface the anon-share URL in the deck-page disclosure.
export function shareOptInKey(deckKey) {
  if (!deckKey) return null
  return `${SHARE_OPTIN_KEY_PREFIX}::${deckKey}`
}

function resolveStorage(storage) {
  if (storage) return storage
  try {
    return globalThis.localStorage || null
  } catch {
    return null
  }
}

export function isShareOptIn(deckKey, storage) {
  const key = shareOptInKey(deckKey)
  if (!key) return false
  const s = resolveStorage(storage)
  if (!s) return false
  try {
    return s.getItem(key) === '1'
  } catch {
    return false
  }
}

// Returns the new flag state actually persisted. A `false` toggle clears
// the key entirely so storage doesn't accumulate disabled rows.
export function setShareOptIn(deckKey, enabled, storage) {
  const key = shareOptInKey(deckKey)
  if (!key) return false
  const s = resolveStorage(storage)
  const flag = !!enabled
  if (!s) return flag
  try {
    if (flag) s.setItem(key, '1')
    else s.removeItem(key)
  } catch {
    // Safari private mode — UI state still flips; just no persistence.
  }
  return flag
}
