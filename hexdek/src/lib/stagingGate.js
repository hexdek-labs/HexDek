// stagingGate — pure helpers for the staging review whitelist
// (r63, owner direction: sweeping UX changes get human review on a
// whitelisted staging build before production).
//
// The build-time switch itself (import.meta.env.VITE_STAGING) lives in
// the StagingGate component — these helpers stay import.meta-free so
// they run under plain `node --test`.

// Reviewers allowed into the staging build.
export const STAGING_WHITELIST = [
  'titanicmistake@gmail.com',
  'joshua.f.wiedeman@gmail.com',
]

// stagingAllows reports whether an authenticated email is whitelisted.
export function stagingAllows(email) {
  if (!email) return false
  const e = String(email).trim().toLowerCase()
  return STAGING_WHITELIST.includes(e)
}

// stagingRouteExempt — routes that must remain reachable WITHOUT a
// whitelisted session, or reviewers could never sign in at all: the
// login screen and the magic-link landing page. Everything else on the
// staging host is gated.
export function stagingRouteExempt(pathname) {
  const p = String(pathname || '')
  return p === '/login' || p.startsWith('/auth/')
}

// --- Staging passphrase bypass (r63) -------------------------------------
//
// The email-link login path is flaky for the two reviewers (Firebase's
// default sender lands in spam — the known William-login issue), so the
// gate also accepts a shared passphrase baked in at build time via
// VITE_STAGING_PASSPHRASE (the env read itself happens in the
// StagingGate component; these helpers stay import.meta-free for
// `node --test`). NOTE this is a soft gate: a build-time client secret
// is recoverable from the bundle by anyone determined — fine for a
// staging curtain, never for real auth.

// localStorage key for the persisted passphrase grant. The STORED VALUE
// is the passphrase itself, and a grant only counts when it equals the
// CURRENT build's expected passphrase — so rotating the passphrase at
// the next deploy silently invalidates every old grant.
export const STAGING_PASS_STORAGE_KEY = 'hexdek_staging_pass'

// passphraseEnabled reports whether a non-empty expected passphrase was
// baked into this build.
export function passphraseEnabled(expected) {
  return typeof expected === 'string' && expected.trim().length > 0
}

// passphraseMatches compares reviewer input against the build's
// expected passphrase. Trimmed, case-sensitive, and never matches when
// the build carries no passphrase.
export function passphraseMatches(input, expected) {
  if (!passphraseEnabled(expected)) return false
  return String(input ?? '').trim() === expected.trim()
}
