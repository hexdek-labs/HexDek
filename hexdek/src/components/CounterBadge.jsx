// CounterBadge — small inline badge stack rendered on a perm tile
// when the permanent carries counters (+1/+1, loyalty, charge, etc).
// Consumes PermanentSnapshot.Counters (added in the R60 spectator UX
// batch 3 — see internal/hexapi/showmatch.go).
//
// Renders nothing when the perm has no counters so non-creature
// non-planeswalker permanents stay visually quiet.
//
// Pure helpers (orderCounters, formatCounterLabel, counterBadgeColor)
// live in countersHelpers.js for node --test access.

import {
  counterBadgeColor,
  formatCounterLabel,
  orderCounters,
} from './countersHelpers.js'

export { counterBadgeColor, formatCounterLabel, orderCounters }

// Max kinds rendered before the "+N" overflow chip. Three is the
// realistic ceiling — a perm carrying more than 3 distinct counter
// kinds is rare and the tile is too small to surface them all
// legibly.
const MAX_VISIBLE_KINDS = 3

export default function CounterBadge({ counters }) {
  const ordered = orderCounters(counters)
  if (ordered.length === 0) return null
  const visible = ordered.slice(0, MAX_VISIBLE_KINDS)
  const overflow = ordered.length - visible.length
  return (
    <span
      className="counter-badge"
      style={{
        position: 'absolute',
        top: 2,
        left: 2,
        display: 'inline-flex',
        flexDirection: 'column',
        gap: 1,
        pointerEvents: 'none',
        zIndex: 2,
      }}
      aria-label={`Counters: ${ordered.map(c => `${c.count} ${c.kind}`).join(', ')}`}
    >
      {visible.map(({ kind, count, label }) => (
        <span
          key={kind}
          title={`${count} ${kind}`}
          style={{
            display: 'inline-flex',
            alignItems: 'center',
            justifyContent: 'center',
            minWidth: 14,
            height: 12,
            padding: '0 3px',
            background: counterBadgeColor(kind),
            color: '#1a1410',
            fontSize: 8,
            fontWeight: 700,
            lineHeight: 1,
            letterSpacing: '0.02em',
            border: '1px solid rgba(0, 0, 0, 0.3)',
            borderRadius: 2,
            textShadow: '0 0 1px rgba(255, 255, 255, 0.3)',
          }}
        >
          {label}
          {count > 1 || kind === '+1/+1' || kind === '-1/-1' || kind === 'loyalty'
            ? ` ${count}`
            : count !== 1 ? ` ${count}` : ''}
        </span>
      ))}
      {overflow > 0 && (
        <span
          title={`+${overflow} more counter kind${overflow === 1 ? '' : 's'}`}
          style={{
            display: 'inline-flex',
            alignItems: 'center',
            justifyContent: 'center',
            minWidth: 14,
            height: 10,
            padding: '0 3px',
            background: 'var(--ink-2)',
            color: '#1a1410',
            fontSize: 7,
            fontWeight: 700,
            lineHeight: 1,
            border: '1px solid rgba(0, 0, 0, 0.3)',
            borderRadius: 2,
          }}
        >
          +{overflow}
        </span>
      )}
    </span>
  )
}
