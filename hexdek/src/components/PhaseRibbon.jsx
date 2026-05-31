// PhaseRibbon — visual 5-phase track showing where the current turn
// is in the MTG turn sequence (Beginning → Main 1 → Combat → Main 2
// → End) with the current substep called out beneath the active
// phase. Plus a compact "next up" preview showing whose turn comes
// after the current active seat (skipping eliminated seats).
//
// Spectator UX gap closed (R60 polish pass): pre-this, the turn-bar
// rendered a single text line "R2T7 · MAIN / PRECOMBAT_MAIN ·
// ATRAXA" — readable but hard to scan at-a-glance for "where in the
// turn are we?". The ribbon replaces that with a five-cell strip
// where the active phase is solidly lit and adjacent phases are
// dimmed, with the canonical step name spelled out below. Combat
// substeps get the most benefit since spectators care which combat
// substep is live (declare attackers vs blockers vs damage step).
//
// Pure helpers (phaseBucket, stepLabel, nextActiveSeat) live in
// phaseRibbonHelpers.js so node --test can exercise them directly.

import { PHASE_LABELS, phaseBucket, stepLabel, nextActiveSeat } from './phaseRibbonHelpers.js'

// Re-export the helpers for consumers who want them without a
// separate import path.
export { phaseBucket, stepLabel, nextActiveSeat }

// PhaseRibbon renders the 5-phase track + active-step label +
// next-up seat preview. Designed to sit ABOVE the existing turn-bar
// (which still carries the round/turn counter and perms-total).
// Compact: total height ~52px including the next-up row.
//
// Props:
//   phase, step    — game.phase / game.step strings (engine vocab)
//   seats          — seats[] array (used for next-up name lookup)
//   activeSeat     — game.active_seat (zero-indexed)
//   finished       — game.finished (suppresses next-up when true)
//
// All styling uses existing CSS vars (--ok / --ink / --ink-2 /
// --rule-2 / --bg-2). No new chunked CSS — inline styles keep the
// component drop-in for any consumer that has the chrome design
// tokens loaded.
export default function PhaseRibbon({ phase, step, seats, activeSeat, finished }) {
  const bucket = phaseBucket(phase, step)
  const stepText = stepLabel(step)
  const nextSeat = !finished ? nextActiveSeat(seats, activeSeat) : -1
  const nextCommander = nextSeat >= 0 ? seats[nextSeat]?.commander : null

  return (
    <div
      className="phase-ribbon"
      style={{
        display: 'flex',
        flexDirection: 'column',
        gap: 4,
        padding: '6px 8px',
        background: 'var(--bg-2, transparent)',
        borderTop: '1px solid var(--rule-2)',
        borderBottom: '1px solid var(--rule-2)',
        fontSize: 10,
        letterSpacing: '0.08em',
      }}
      aria-label={`Phase ${PHASE_LABELS[bucket] || 'unknown'}${stepText ? `, step ${stepText}` : ''}`}
    >
      <div style={{ display: 'flex', alignItems: 'stretch', gap: 0 }}>
        {PHASE_LABELS.map((label, idx) => {
          const isActive = idx === bucket
          const isPast = bucket >= 0 && idx < bucket
          // Active = solid accent; past = dimmed ink; future = muted rule.
          const bg = isActive ? 'var(--ok)' : isPast ? 'var(--ink-2)' : 'var(--rule-2)'
          const color = isActive ? 'var(--bg-2, #000)' : isPast ? 'var(--bg, #000)' : 'var(--ink-2)'
          const opacity = isActive ? 1 : isPast ? 0.55 : 0.4
          return (
            <div
              key={label}
              style={{
                flex: 1,
                background: bg,
                color,
                opacity,
                textAlign: 'center',
                padding: '5px 2px',
                fontWeight: isActive ? 700 : 500,
                fontSize: 9,
                letterSpacing: '0.1em',
                marginRight: idx < PHASE_LABELS.length - 1 ? 2 : 0,
              }}
            >
              {label}
            </div>
          )
        })}
      </div>
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          gap: 8,
          color: 'var(--ink-2)',
        }}
      >
        <span style={{ fontSize: 9, fontWeight: 600 }}>
          {stepText ? `STEP / ${stepText}` : (finished ? 'GAME ENDED' : '—')}
        </span>
        {nextCommander && (
          <span style={{ fontSize: 9, fontWeight: 600 }}>
            NEXT &rsaquo; {nextCommander.toUpperCase()}
          </span>
        )}
      </div>
    </div>
  )
}
