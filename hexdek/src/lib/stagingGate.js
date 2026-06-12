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
