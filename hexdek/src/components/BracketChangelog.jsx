import { useState, useEffect } from 'react'
import { api } from '../services/api'
import {
  recordBracketSighting,
  describeLatestTransition,
  computeBracketChangelog,
  getBracketHistory,
} from '../lib/bracketHistory'

// Inline annotation that renders "B3 (upgraded from B2 on May 24)" under
// the bracket cell when the deck's bracket has changed across versions.
// Renders nothing when there's no transition yet — first visit to a deck
// just establishes a baseline sighting silently.
//
// Records the current version's bracket as a sighting at render time
// (idempotent on (deck, version, bracket)), then reads the resulting
// history to compute the changelog. Uses the "sync state during render"
// pattern instead of a useEffect so the new transition shows up in the
// same render that recorded it.

export default function BracketChangelog({ deckKey, bracket }) {
  // Latest version number for the deck. Fetched on mount so the bracket
  // sighting is keyed by the version that produced it (versions arrive
  // newest-first from /versions). -1 = pending; once resolved it stays
  // ≥0. We gate the sighting record on `version >= 0` so a page load
  // on a fresh deck doesn't log against synthetic v0 and then get
  // overwritten when the real version arrives a moment later.
  const [version, setVersion] = useState(-1)
  const [history, setHistory] = useState(() => getBracketHistory(deckKey))

  useEffect(() => {
    if (!deckKey) return
    let cancelled = false
    api.getDeckVersions(deckKey)
      .then((rows) => {
        if (cancelled) return
        const arr = Array.isArray(rows) ? rows : []
        const max = arr.reduce((m, v) => Math.max(m, Number(v?.version) || 0), 0)
        setVersion(max)
      })
      .catch(() => { if (!cancelled) setVersion(0) })
    return () => { cancelled = true }
  }, [deckKey])

  // Record-on-input-change. The helper is idempotent on (version, bracket)
  // so a re-render with unchanged props doesn't write — and when the
  // value DOES change, the deps array fires this effect exactly once.
  // Date.now() lives here (not in render) because the React compiler
  // flags impure calls during render.
  useEffect(() => {
    if (version < 0 || !deckKey) return
    if (bracket == null || bracket === '?' || bracket === '') return
    const now = Math.floor(Date.now() / 1000)
    // Syncing FROM an external store (localStorage) — the accepted shape
    // for this in the codebase is a one-line disable (see Report.jsx:292).
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setHistory(recordBracketSighting(deckKey, version, bracket, now))
  }, [deckKey, version, bracket])

  if (!deckKey) return null
  const line = describeLatestTransition(history)
  if (!line) return null

  const direction = computeBracketChangelog(history).slice(-1)[0]?.direction
  const arrow = direction === 'up' ? '↑' : direction === 'down' ? '↓' : '·'
  const color = direction === 'up' ? 'var(--ok)' : direction === 'down' ? 'var(--danger)' : 'var(--ink-2)'

  return (
    <span
      data-testid="bracket-changelog"
      title={line}
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: 4,
        fontSize: 9,
        letterSpacing: '0.06em',
        color,
        marginLeft: 6,
      }}
    >
      <span style={{ fontWeight: 700 }}>{arrow}</span>
      <span>{line.replace(/^B\d+(\.\d+)?\s*\(/, '').replace(/\)$/, '').toUpperCase()}</span>
    </span>
  )
}
