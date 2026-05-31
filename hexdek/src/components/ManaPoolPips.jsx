// ManaPoolPips — small WUBRG/C/Any pips rendering the per-seat
// floating mana pool. Consumes SeatSnapshot.ManaPoolByColor (added
// in the R60 spectator UX batch — see internal/hexapi/showmatch.go
// buildManaPoolByColor). Renders nothing when the pool is empty so
// the seat footer doesn't carry visual noise during the
// no-floating-mana steady state.
//
// Each pip is a tiny colored chip with the count. Mirrors the
// chrome ColorPie / meta-screen archetype palette so visual
// language stays consistent.
//
// Pure helpers (normalizeManaPool, totalMana) live in
// manaPoolHelpers.js so node --test can exercise them directly.

import { MANA_COLORS, normalizeManaPool } from './manaPoolHelpers.js'

export { normalizeManaPool, totalMana } from './manaPoolHelpers.js'

export default function ManaPoolPips({ pool, size = 14 }) {
  const items = normalizeManaPool(pool)
  if (items.length === 0) return null
  return (
    <span
      className="mana-pool-pips"
      style={{
        display: 'inline-flex',
        gap: 3,
        alignItems: 'center',
        verticalAlign: 'middle',
      }}
      aria-label="Floating mana pool"
    >
      {items.map(({ color, count }) => {
        const bg = MANA_COLORS[color] || MANA_COLORS.C
        // Black background needs light text; everything else uses
        // dark ink so the count stays legible.
        const fg = color === 'B' ? '#E8E2D1' : '#1a1410'
        return (
          <span
            key={color}
            title={`${count} ${color === 'Any' ? 'generic' : color}`}
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              justifyContent: 'center',
              minWidth: size,
              height: size,
              padding: '0 4px',
              borderRadius: size / 2,
              background: bg,
              color: fg,
              fontSize: 9,
              fontWeight: 700,
              lineHeight: 1,
              border: '1px solid rgba(0, 0, 0, 0.2)',
            }}
          >
            {color === 'Any' ? '◇' : color}
            {count > 1 ? count : ''}
          </span>
        )
      })}
    </span>
  )
}
