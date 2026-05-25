import { useState, useRef, useEffect } from 'react'

// Inline marker that surfaces Freya's per-card coaching inside a card
// list / grid row. Today this only handles the `cut` kind: a small ✂
// chip that opens a popover with Freya's rationale on click.
//
// Owners get an action button inside the popover that opens the
// Workshop with the card removed (parent supplies onCut). Non-owners
// see the rationale only — the marker still appears so they can
// understand the deck's flagged cards.
//
// Renders nothing when `entry?.cut` is missing so callers can drop
// <CoachingMarker entry={coachingForCard(idx, name)} /> into every
// row unconditionally without branching at the call site.

export default function CoachingMarker({ entry, onCut }) {
  const [open, setOpen] = useState(false)
  const ref = useRef(null)

  // Click-outside / Escape dismiss. We attach listeners only while the
  // popover is open so other rows' markers can't fight for them.
  useEffect(() => {
    if (!open) return
    const onDocClick = (e) => {
      if (ref.current && !ref.current.contains(e.target)) setOpen(false)
    }
    const onKey = (e) => { if (e.key === 'Escape') setOpen(false) }
    document.addEventListener('mousedown', onDocClick)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onDocClick)
      document.removeEventListener('keydown', onKey)
    }
  }, [open])

  const cut = entry?.cut
  if (!cut) return null

  const hasDetail = cut.detected || cut.whyCut || cut.effect || (cut.suggested && cut.suggested.length > 0) || cut.reason

  return (
    <span ref={ref} style={{ position: 'relative', display: 'inline-flex', alignItems: 'center' }}>
      <button
        type="button"
        onClick={(e) => { e.stopPropagation(); setOpen(o => !o) }}
        title={cut.reason ? `Freya flagged this for cutting: ${cut.reason}` : 'Freya flagged this for cutting'}
        aria-label={`Freya cut rationale for ${cut.name}`}
        aria-expanded={open}
        data-testid="coaching-marker"
        style={{
          background: open ? 'var(--warn)' : 'transparent',
          color: open ? 'var(--bg)' : 'var(--warn)',
          border: '1px solid var(--warn)',
          padding: '0 4px',
          fontSize: 10,
          lineHeight: 1.4,
          fontFamily: 'inherit',
          fontWeight: 700,
          letterSpacing: '0.04em',
          cursor: 'pointer',
          marginLeft: 6,
        }}
      >
        ✂
      </button>
      {open && (
        <div
          role="dialog"
          aria-label={`Cut rationale for ${cut.name}`}
          style={{
            position: 'absolute',
            top: 'calc(100% + 4px)',
            right: 0,
            zIndex: 50,
            background: 'var(--bg)',
            border: '1px solid var(--warn)',
            padding: 10,
            minWidth: 260,
            maxWidth: 360,
            boxShadow: '2px 2px 0 rgba(0,0,0,0.2)',
            fontSize: 11,
            lineHeight: 1.45,
          }}
        >
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 6 }}>
            <span style={{ fontWeight: 700, letterSpacing: '0.08em', color: 'var(--warn)', fontSize: 9 }}>
              ✂ CONSIDER CUTTING
            </span>
            <button
              type="button"
              onClick={() => setOpen(false)}
              aria-label="Close cut rationale"
              style={{ background: 'transparent', border: 'none', color: 'var(--ink-3)', font: 'inherit', cursor: 'pointer', padding: 0 }}
            >ESC</button>
          </div>
          <div style={{ fontWeight: 700, marginBottom: 4 }}>{cut.name}</div>
          {hasDetail ? (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
              {cut.reason && <div className="t-xs muted" style={{ fontStyle: 'italic' }}>{cut.reason}</div>}
              {cut.detected && (
                <div className="t-xs"><strong style={{ color: 'var(--ink-2)' }}>Detected: </strong>{cut.detected}</div>
              )}
              {cut.whyCut && (
                <div className="t-xs"><strong style={{ color: 'var(--ink-2)' }}>Why: </strong>{cut.whyCut}</div>
              )}
              {cut.effect && (
                <div className="t-xs"><strong style={{ color: 'var(--ink-2)' }}>If cut: </strong>{cut.effect}</div>
              )}
              {cut.suggested && cut.suggested.length > 0 && (
                <div className="t-xs">
                  <strong style={{ color: 'var(--ink-2)' }}>Swap for: </strong>
                  {cut.suggested.join(', ')}
                </div>
              )}
            </div>
          ) : (
            <div className="t-xs muted">Flagged by Freya; no rationale was recorded for this entry.</div>
          )}
          {onCut && (
            <button
              type="button"
              onClick={(e) => { e.stopPropagation(); onCut(cut.name); setOpen(false) }}
              title={`Open Workshop with ${cut.name} removed`}
              style={{
                marginTop: 10,
                background: 'var(--warn)',
                color: 'var(--bg)',
                border: 'none',
                padding: '4px 10px',
                fontSize: 10,
                fontWeight: 700,
                letterSpacing: '0.08em',
                fontFamily: 'inherit',
                cursor: 'pointer',
              }}
            >
              CUT IN WORKSHOP ↗
            </button>
          )}
        </div>
      )}
    </span>
  )
}
