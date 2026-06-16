// DeckRadar — deck-profile spider chart.
//
// Reuses the radar drawing from CursePanel's "TOP MEMBER PERSONALITY"
// chart, but generalized to a configurable axis set and fed DECK data
// (sourced from Freya's analysis) instead of personality traits. MTG-
// native axes, no Greed/Drain. Clean dark style (same CSS vars).
//
// Axis values are 0..1 NORMALIZED from Freya's existing computed outputs
// (cmd/hexdek-freya / internal/analytics). See freyaToRadar() for the
// exact field -> axis mapping. Nothing here is an invented measurement;
// every number traces to a Freya field.

// The six deck-fingerprint axes (flexible — drop/add by editing this list).
export const DECK_AXES = [
  { key: 'power',         label: 'POWER' },
  { key: 'speed',         label: 'SPEED' },
  { key: 'resilience',    label: 'RESILIENCE' },
  { key: 'consistency',   label: 'CONSISTENCY' },
  { key: 'interaction',   label: 'INTERACTION' },
  { key: 'inevitability', label: 'INEVITABILITY' },
]

const clamp01 = (x) => Math.max(0, Math.min(1, x))
const gradeScore = (g) => ({ A: 1.0, B: 0.75, C: 0.5, D: 0.25, F: 0.0 }[String(g || '').toUpperCase().charAt(0)] ?? 0.5)

// freyaToRadar maps a live Freya analysis object (the flattened
// unified_profile the /api/decks/{id}/analysis endpoint returns) to the
// six 0..1 axis values. Every axis traces to named Freya fields:
//
//   POWER         power_percentile (1-99 strength-for-bracket percentile)
//   SPEED         avg_turn_to_four_mana (ramp/closing speed, inverted)
//   RESILIENCE    draw_count + mana_base_grade + backup_count
//                 (card advantage to grind back + mana stability + redundant win lines)
//   CONSISTENCY   keepable_hand_pct (opening-hand keepable Monte-Carlo sim)
//   INTERACTION   interaction_avg_cmc (cheaper = denser, inverted) + counterspell_count
//   INEVITABILITY win_line_count + finisher_cards (paths to victory / redundancy)
//
// Missing fields fall back gracefully so the chart still renders.
export function freyaToRadar(a) {
  if (!a) return null
  const num = (v, d = 0) => (typeof v === 'number' && !Number.isNaN(v) ? v : d)
  const len = (v) => (Array.isArray(v) ? v.length : num(v, 0))

  const power = clamp01(num(a.power_percentile) / 99)
  const t4 = num(a.avg_turn_to_four_mana, 4.5)
  const speed = clamp01((7 - t4) / 4.5)
  const draw = num(a.draw_count, 4)
  const backup = num(a.backup_count, len(a.win_lines) || 8)
  const resilience = clamp01(0.5 * (draw / 12) + 0.3 * gradeScore(a.mana_base_grade) + 0.2 * (backup / 24))
  const consistency = clamp01(num(a.keepable_hand_pct, 50) / 100)
  const iCmc = num(a.interaction_avg_cmc ?? a.interaction_quality, 3.5)
  const ctr = num(a.counterspell_count, 0)
  const interaction = clamp01(0.6 * ((5 - iCmc) / 4) + 0.4 * (ctr / 7))
  const winLines = len(a.win_lines) || num(a.win_line_count, 8)
  const finishers = len(a.finisher_cards) || num(a.finisher_count, winLines)
  const inevitability = clamp01(0.6 * (winLines / 24) + 0.4 * (finishers / 20))

  return { power, speed, resilience, consistency, interaction, inevitability }
}

// REFERENCE_FINGERPRINTS — 7174n1c's three real B3 decks, with axis values
// pre-computed from a live Freya run (`hexdek-freya --deck … --format json`)
// so the staging mock shows real, comparable fingerprints. `raw` keeps the
// underlying Freya numbers for the legend / verification.
export const REFERENCE_FINGERPRINTS = [
  {
    id: 'coram',
    name: 'Coram, the Undertaker',
    sub: 'Value / Midrange · B4',
    color: 'var(--ok)',
    values: { power: 0.788, speed: 0.651, resilience: 0.717, consistency: 0.725, interaction: 0.279, inevitability: 0.64 },
    raw: { power_percentile: 78, avg_turn_to_four_mana: 4.07, draw_count: 7, mana_base_grade: 'A', backup_count: 15, keepable_hand_pct: 72.5, interaction_avg_cmc: 3.14, counterspell_count: 0, win_line_count: 16, finishers: 12 },
  },
  {
    id: 'sin',
    name: 'Sin, Spira’s Punishment',
    sub: 'Land Reanimator · B3',
    color: 'var(--accent, #6cf)',
    values: { power: 0.707, speed: 0.649, resilience: 0.5, consistency: 0.749, interaction: 0.7, inevitability: 0.49 },
    raw: { power_percentile: 70, avg_turn_to_four_mana: 4.08, draw_count: 4, mana_base_grade: 'B', backup_count: 13, keepable_hand_pct: 74.9, interaction_avg_cmc: 3.0, counterspell_count: 7, win_line_count: 14, finishers: 7 },
  },
  {
    id: 'varina',
    name: 'Varina, Lich Queen',
    sub: 'Tribal Wide/Tall · B2',
    color: 'var(--warn)',
    values: { power: 0.636, speed: 0.637, resilience: 0.7, consistency: 0.63, interaction: 0.388, inevitability: 0.98 },
    raw: { power_percentile: 63, avg_turn_to_four_mana: 4.13, draw_count: 5, mana_base_grade: 'A', backup_count: 23, keepable_hand_pct: 63.0, interaction_avg_cmc: 3.56, counterspell_count: 3, win_line_count: 24, finishers: 19 },
  },
]

// DeckRadar — the SVG spider chart. `values` is a {axisKey: 0..1} map (or
// array aligned to `axes`); `axes` defaults to DECK_AXES; `color` tints the
// fill/stroke; `showLabels` toggles the outer axis labels (off for small
// multiples). Mirrors the CursePanel radar geometry + dark styling.
export default function DeckRadar({ values, axes = DECK_AXES, color = 'var(--ok)', size = 200, showLabels = true }) {
  const cx = size / 2
  const cy = size / 2
  const r = size * 0.38
  const n = axes.length
  const angle = (i) => -Math.PI / 2 + (2 * Math.PI * i) / n
  const valAt = (i) => {
    const k = axes[i].key
    const v = Array.isArray(values) ? values[i] : values?.[k]
    return Math.max(0, Math.min(1, typeof v === 'number' ? v : 0))
  }
  const point = (i, v) => {
    const a = angle(i)
    const rr = r * Math.max(0, Math.min(1, v))
    return [cx + Math.cos(a) * rr, cy + Math.sin(a) * rr]
  }
  const axisPoint = (i, frac = 1) => {
    const a = angle(i)
    return [cx + Math.cos(a) * r * frac, cy + Math.sin(a) * r * frac]
  }
  const polygon = axes.map((_, i) => point(i, valAt(i)).join(',')).join(' ')
  const rings = [0.25, 0.5, 0.75, 1.0]

  return (
    <svg viewBox={`0 0 ${size} ${size}`} width="100%" style={{ display: 'block', overflow: 'visible' }}>
      {rings.map((f, i) => (
        <polygon
          key={i}
          points={Array.from({ length: n }, (_, j) => axisPoint(j, f).join(',')).join(' ')}
          fill="none"
          stroke="var(--rule)"
          strokeOpacity={f === 1 ? 0.6 : 0.25}
          strokeWidth="1"
        />
      ))}
      {Array.from({ length: n }, (_, i) => {
        const [x, y] = axisPoint(i, 1)
        return <line key={i} x1={cx} y1={cy} x2={x} y2={y} stroke="var(--rule)" strokeOpacity="0.25" strokeWidth="1" />
      })}
      <polygon points={polygon} fill={color} fillOpacity="0.18" stroke={color} strokeWidth="1.5" />
      {axes.map((_, i) => {
        const [px, py] = point(i, valAt(i))
        return <circle key={i} cx={px} cy={py} r={2.5} fill={color} />
      })}
      {showLabels && axes.map((t, i) => {
        const [lx, ly] = axisPoint(i, 1.2)
        const anchor = Math.abs(lx - cx) < 4 ? 'middle' : lx < cx ? 'end' : 'start'
        return (
          <text
            key={t.key}
            x={lx}
            y={ly}
            textAnchor={anchor}
            dominantBaseline="middle"
            fontSize="8"
            fill="var(--ink-2)"
            style={{ letterSpacing: '0.04em', textTransform: 'uppercase' }}
          >
            {t.label}
          </text>
        )
      })}
    </svg>
  )
}
