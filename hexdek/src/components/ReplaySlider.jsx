// ReplaySlider — horizontal scrubber for stepping backward through
// recent snapshot history. Sits at the top of the spectator screen
// (above the seats area) when the history ring has captured at
// least two snapshots, otherwise stays hidden so the live-only
// session doesn't carry visual noise.
//
// Snapshots are captured into a ring buffer in Spectator.jsx via
// pushSnapshot — one entry per unique (turn, phase, step, active_seat)
// tuple, capped at MAX_SNAPSHOT_HISTORY (120). The component itself
// is stateless: the parent owns the history + scrub index and passes
// them in.
//
// Scrub semantics:
//   scrubIndex === -1  → live (render the current real-time game)
//   scrubIndex >= 0    → render history[scrubIndex]
//
// Live-or-scrubbed selection happens upstream via effectiveGame() in
// replayHelpers.js.

import { scrubLabel } from './replayHelpers.js'

export { scrubLabel } from './replayHelpers.js'

export default function ReplaySlider({ history, scrubIndex, onChange, onLive, nSeats = 4 }) {
  if (!Array.isArray(history) || history.length < 2) return null

  const isScrubbed = scrubIndex >= 0
  const displayIndex = isScrubbed ? scrubIndex : history.length - 1
  const currentSnap = history[displayIndex]

  return (
    <div
      className="replay-slider"
      style={{
        display: 'flex',
        flexDirection: 'column',
        gap: 4,
        padding: '6px 10px',
        background: 'var(--bg-2, transparent)',
        borderTop: '1px solid var(--rule-2)',
        borderBottom: '1px solid var(--rule-2)',
        fontSize: 10,
        letterSpacing: '0.06em',
      }}
      aria-label="Replay history slider"
    >
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          fontSize: 9,
          fontWeight: 600,
          color: 'var(--ink-2)',
        }}
      >
        <span>REPLAY / {scrubLabel(currentSnap, nSeats)}</span>
        <span>
          {isScrubbed ? (
            <button
              type="button"
              onClick={onLive}
              style={{
                fontSize: 9,
                fontWeight: 700,
                padding: '2px 8px',
                background: 'var(--ok)',
                color: 'var(--bg-2, #000)',
                border: 'none',
                cursor: 'pointer',
                letterSpacing: '0.06em',
              }}
            >
              ⏵ RETURN TO LIVE
            </button>
          ) : (
            <span style={{ color: 'var(--ok)' }}>● LIVE</span>
          )}
        </span>
      </div>
      <input
        type="range"
        min={0}
        max={history.length - 1}
        step={1}
        value={displayIndex}
        onChange={(e) => {
          const idx = Number(e.target.value)
          // If user scrubs to the latest, treat as "live" so future
          // pushes continue updating the rendered state.
          if (idx === history.length - 1) {
            onLive()
          } else {
            onChange(idx)
          }
        }}
        aria-label="Scrub through snapshot history"
        style={{
          width: '100%',
          accentColor: 'var(--ok)',
          height: 4,
          cursor: 'pointer',
        }}
      />
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          fontSize: 8,
          color: 'var(--ink-2)',
          opacity: 0.7,
        }}
      >
        <span>{scrubLabel(history[0], nSeats)}</span>
        <span>
          {displayIndex + 1} / {history.length}
        </span>
        <span>{scrubLabel(history[history.length - 1], nSeats)}</span>
      </div>
    </div>
  )
}
