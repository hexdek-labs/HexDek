// CommandZoneStrip — per-seat command-zone display rendering each
// commander card sitting in the zone with its §903.8 cast-tax count.
// Consumes SeatSnapshot.CommandZone (added in the R60 spectator UX
// batch 3 — see internal/hexapi/showmatch.go).
//
// Renders nothing when the zone is empty (commander on battlefield),
// so this stays visually quiet during the dominant steady state.
// Surfaces during the early game (pre-cast), after a §903.9 commander
// kill, and any time the controller chose to return their commander
// to the zone via §903.9b.
//
// The cast count is the spectator-relevant signal: a deck that's
// returned its commander twice ("4 surcharge") tells the spectator
// the controller is leveraging cast-recursion as a strategy, or
// has lost their commander to repeated removal.

export default function CommandZoneStrip({ zone }) {
  if (!Array.isArray(zone) || zone.length === 0) return null
  return (
    <span
      className="command-zone-strip"
      style={{
        display: 'inline-flex',
        gap: 4,
        alignItems: 'center',
        verticalAlign: 'middle',
        fontSize: 9,
        letterSpacing: '0.05em',
      }}
      aria-label="Command zone"
    >
      <span
        style={{ fontSize: 8, color: 'var(--ink-2)', fontWeight: 600 }}
      >
        CZ
      </span>
      {zone.map((entry, i) => {
        const taxSurcharge = (entry.cast_count || 0) * 2
        const title = taxSurcharge > 0
          ? `${entry.name} — next cast +${taxSurcharge} (cast ${entry.cast_count}× before)`
          : `${entry.name} — never cast`
        return (
          <span
            key={`${entry.name}-${i}`}
            title={title}
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: 2,
              padding: '1px 4px',
              borderRadius: 2,
              background: 'var(--rule-2)',
              color: 'var(--ink)',
              fontWeight: 600,
              fontSize: 9,
              border: taxSurcharge > 0 ? '1px solid var(--accent, #c8a26b)' : '1px solid transparent',
            }}
          >
            <span style={{ maxWidth: 80, overflow: 'hidden', textOverflow: 'ellipsis' }}>
              {(entry.name || '?').toUpperCase().slice(0, 12)}
            </span>
            {taxSurcharge > 0 && (
              <span
                style={{
                  fontSize: 8,
                  color: 'var(--accent, #c8a26b)',
                  fontWeight: 700,
                }}
              >
                +{taxSurcharge}
              </span>
            )}
          </span>
        )
      })}
    </span>
  )
}
