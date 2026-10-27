// Relative-age bucketing, kept pure so the Node unit suite can import it
// (it cannot resolve the JSON locale catalogs). The bucket is computed here;
// the WORD for the bucket is chosen at the render site via formatAge, so a
// language switch re-renders it instead of freezing whatever language was
// current when the data was fetched.
//
// kind: 'none' (no timestamp) | 'now' | 's' | 'm' | 'h' | 'd' | 'date'
// Anything older than 30 days falls back to an ISO date, which needs no
// translation at all.

const MINUTE = 60
const HOUR = 3600
const DAY = 86400

export function relativeAge(atMs, nowMs = Date.now()) {
  if (atMs == null || !Number.isFinite(atMs)) return { kind: 'none', value: 0 }
  const sec = Math.max(0, Math.floor((nowMs - atMs) / 1000))
  if (sec < MINUTE) return { kind: sec < 5 ? 'now' : 's', value: sec }
  if (sec < HOUR) return { kind: 'm', value: Math.floor(sec / MINUTE) }
  if (sec < DAY) return { kind: 'h', value: Math.floor(sec / HOUR) }
  if (sec < 30 * DAY) return { kind: 'd', value: Math.floor(sec / DAY) }
  return { kind: 'date', value: new Date(atMs).toISOString().slice(0, 10) }
}

// `t` is the translator from the call site. Passing it in keeps this module
// free of the i18n import (and therefore importable by the unit suite).
export function formatAge(t, age) {
  if (!age || age.kind === 'none') return ''
  if (age.kind === 'date') return age.value
  if (age.kind === 'now') return t('age.now')
  return t(`age.${age.kind}`, { count: age.value })
}
