// Pure helpers powering ManaPoolPips. Lives in a .js file (vs the
// .jsx component file) so node --test can import it directly
// without a JSX transform.

// MANA_COLOR_ORDER is the canonical render order for the pips — WUBRG
// then colorless then any/generic. Matches the convention used by
// the existing chrome ColorPie component and the meta-screen
// archetype palette so visual ordering stays consistent across
// surfaces.
export const MANA_COLOR_ORDER = ['W', 'U', 'B', 'R', 'G', 'C', 'Any']

// MANA_COLORS maps the canonical letter token to the CSS color
// value the pip renders. The WUBRG stops mirror Meta.jsx exactly so
// the UI stays internally consistent.
export const MANA_COLORS = {
  W: '#E0EBD3',
  U: '#6E8FA0',
  B: '#3a3628',
  R: '#CC5C4A',
  G: '#82C472',
  C: '#8A9682',
  Any: '#A89A82',
}

// normalizeManaPool takes the SeatSnapshot.ManaPoolByColor map
// (which may be undefined / missing zero buckets) and produces an
// ordered array of {color, count} for the renderer. Returns an
// empty array when the pool is empty so the caller can branch on
// length cleanly.
//
// Filters zero counts defensively — backend omitempty should
// already prevent them, but a defensive filter keeps the render
// math correct if a stale payload sneaks through.
export function normalizeManaPool(map) {
  if (!map || typeof map !== 'object') return []
  const out = []
  for (const color of MANA_COLOR_ORDER) {
    const n = map[color]
    if (typeof n === 'number' && n > 0) {
      out.push({ color, count: n })
    }
  }
  return out
}

// totalMana sums every bucket in the ManaPoolByColor map. Useful as
// a sanity check against the legacy SeatSnapshot.ManaPool integer.
export function totalMana(map) {
  if (!map || typeof map !== 'object') return 0
  let sum = 0
  for (const color of MANA_COLOR_ORDER) {
    const n = map[color]
    if (typeof n === 'number') sum += n
  }
  return sum
}
