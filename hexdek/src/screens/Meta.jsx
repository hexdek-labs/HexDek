import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { Panel, Tag, Tape, Btn } from '../components/chrome'
import { API_BASE } from '../services/api'
import { useT } from '../i18n'

// The one worded failure branch. Stored as a SENTINEL, not as English prose:
// a string worded inside .catch() is fixed at fetch time and would not follow
// a later locale switch, so the wording happens at the render site. Any other
// value in `error` is a technical status (`meta 503`) and is echoed as-is.
const FETCH_FAILED = 'fetch-failed'

// Meta — /meta. Pulls a single /api/meta snapshot and renders four
// pure-SVG visualizations of the current showmatch corpus:
//
//   * Top 10 commanders bar chart    (games + win-rate as inline %)
//   * Color identity pie / donut     (WUBRG distribution by games)
//   * Archetype breakdown bar chart  (Freya strategy.json archetype)
//   * Bracket distribution histogram (showmatch_elo.bracket)
//
// All charts are inline <svg>; no chart library. Each panel handles
// its own empty state so the page degrades cleanly when a particular
// data source isn't populated yet.

// MTG color palette — matches the chrome ColorPie tokens. Hybrid /
// multi-color identities composite from these single-color stops.
const MANA_COLORS = {
  W: '#E0EBD3', U: '#6E8FA0', B: '#3a3628', R: '#CC5C4A', G: '#82C472', C: '#8A9682',
}

// fmtCommander trims long commander names to first segment + ellipsis.
function fmtCommander(name, max = 22) {
  if (!name) return '—'
  const head = name.split(',')[0].trim()
  if (head.length <= max) return head.toUpperCase()
  return (head.slice(0, max - 1) + '…').toUpperCase()
}

// pickAccent returns the right accent token for a color-identity bucket.
// Multi-color buckets use a horizontal gradient through their colors;
// callers pull the gradient id with `colorIdGradient(colors)`.
function colorIdGradient(colors) {
  if (!colors || colors === '?' || colors === 'C') return MANA_COLORS.C
  if (colors.length === 1) return MANA_COLORS[colors[0]] || MANA_COLORS.C
  return `url(#meta-grad-${colors})`
}

// CommandersBarChart — horizontal bars, one per commander, length =
// games played. Win-rate appears at the right edge of each bar with
// kind-coded color (green ≥40%, ink-2 below).
function CommandersBarChart({ rows }) {
  const t = useT()
  if (!rows || rows.length === 0) {
    return (
      <div className="t-md muted" style={{ padding: '14px 0', textAlign: 'center' }}>
        {t('meta.no_commanders')}
      </div>
    )
  }
  const top = rows.slice(0, 10)
  const maxGames = Math.max(...top.map(r => r.games), 1)
  const rowH = 26
  const padX = 4
  const barTrack = 1
  const labelW = 200
  const wrPanelW = 80
  const totalW = labelW + 12 + 360 + 12 + wrPanelW
  const totalH = top.length * rowH + 4

  return (
    <svg viewBox={`0 0 ${totalW} ${totalH}`} width="100%" style={{ display: 'block' }}>
      {top.map((r, i) => {
        const y = i * rowH + 4
        const barX0 = labelW + 12
        const barW = ((r.games / maxGames) * 360) - padX * 2
        const wr = (r.win_rate * 100) || 0
        const wrColor = wr >= 40 ? 'var(--ok)' : wr >= 25 ? 'var(--ink-2)' : 'var(--danger)'
        return (
          <g key={r.commander + i}>
            {/* label */}
            <text x={labelW} y={y + 16} textAnchor="end" fontSize="11" fontWeight="600"
              fill="var(--ink)" style={{ letterSpacing: '0.04em' }}>
              {fmtCommander(r.commander)}
            </text>
            {/* bar track */}
            <rect x={barX0} y={y + 6} width={360 - padX * 2} height={rowH - 12 - barTrack}
              fill="var(--rule-2)" opacity="0.4" />
            {/* bar fill */}
            <rect x={barX0} y={y + 6} width={Math.max(2, barW)} height={rowH - 12 - barTrack}
              fill="var(--accent)" />
            {/* games count, inline at end of bar */}
            <text x={barX0 + barW + 6} y={y + 16} fontSize="10" fontWeight="700"
              fill="var(--ink-2)" style={{ fontVariantNumeric: 'tabular-nums' }}>
              {r.games.toLocaleString()}
            </text>
            {/* win rate panel on the right */}
            <text x={totalW - 4} y={y + 16} textAnchor="end" fontSize="11" fontWeight="800"
              fill={wrColor} style={{ fontVariantNumeric: 'tabular-nums' }}>
              {wr.toFixed(1)}% WR
            </text>
          </g>
        )
      })}
    </svg>
  )
}

// ColorIdentityPie — donut chart bucketed by colors string ("WUB",
// "RG", "C", "?"). Each wedge fills with a horizontal gradient over
// its component colors so a Grixis (UBR) wedge reads as blue → black
// → red instead of a single token.
function ColorIdentityPie({ rows }) {
  const t = useT()
  const real = (rows || []).filter(r => r.games > 0)
  if (real.length === 0) {
    return (
      <div className="t-md muted" style={{ padding: '14px 0', textAlign: 'center' }}>
        {t('meta.no_ci_data')}
      </div>
    )
  }
  const total = real.reduce((s, r) => s + r.games, 0)
  // Sort by games desc so the legend aligns with wedge prominence.
  const sorted = [...real].sort((a, b) => b.games - a.games)

  const cx = 110, cy = 110, rOuter = 100, rInner = 56
  let acc = 0
  const polarToCart = (angle, r) => {
    const a = (angle - 90) * Math.PI / 180
    return [cx + r * Math.cos(a), cy + r * Math.sin(a)]
  }
  const arcPath = (start, end) => {
    const [x1, y1] = polarToCart(end, rOuter)
    const [x2, y2] = polarToCart(start, rOuter)
    const [x3, y3] = polarToCart(start, rInner)
    const [x4, y4] = polarToCart(end, rInner)
    const large = end - start > 180 ? 1 : 0
    return [
      `M ${x1} ${y1}`,
      `A ${rOuter} ${rOuter} 0 ${large} 0 ${x2} ${y2}`,
      `L ${x3} ${y3}`,
      `A ${rInner} ${rInner} 0 ${large} 1 ${x4} ${y4}`,
      'Z',
    ].join(' ')
  }

  return (
    <div className="meta-color-grid">
      <svg viewBox="0 0 220 220" style={{ display: 'block', width: '100%', maxWidth: 220 }}>
        <defs>
          {/* Pre-define a horizontal gradient for every multi-color
              identity present so wedges read as blends. */}
          {sorted
            .filter(r => r.colors && r.colors.length > 1 && r.colors !== '?')
            .map(r => (
              <linearGradient key={r.colors} id={`meta-grad-${r.colors}`} x1="0%" y1="0%" x2="100%" y2="0%">
                {[...r.colors].map((c, i, arr) => (
                  <stop key={i} offset={`${(i / Math.max(1, arr.length - 1)) * 100}%`}
                    stopColor={MANA_COLORS[c] || MANA_COLORS.C} />
                ))}
              </linearGradient>
            ))}
        </defs>
        {sorted.map(r => {
          const start = (acc / total) * 360
          const end = ((acc + r.games) / total) * 360
          acc += r.games
          // Skip near-zero wedges so we don't draw NaN paths.
          if (end - start < 0.5) return null
          return (
            <path key={r.colors} d={arcPath(start, end)} fill={colorIdGradient(r.colors)}
              stroke="var(--bg)" strokeWidth="1" />
          )
        })}
        <circle cx={cx} cy={cy} r={rInner - 1} fill="var(--bg-2)" />
        <text x={cx} y={cy - 4} textAnchor="middle" fontSize="9" fill="var(--ink-2)"
          style={{ letterSpacing: '0.10em', textTransform: 'uppercase' }}>{t('op.kv_total_games')}</text>
        <text x={cx} y={cy + 14} textAnchor="middle" fontSize="20" fontWeight="800"
          fill="var(--ink)" style={{ fontVariantNumeric: 'tabular-nums' }}>
          {total.toLocaleString()}
        </text>
      </svg>
      {/* Legend */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
        {sorted.slice(0, 12).map(r => {
          const pct = (r.games / total) * 100
          const wr = (r.win_rate * 100) || 0
          return (
            <div key={r.colors} style={{
              display: 'grid', gridTemplateColumns: '14px 50px 1fr auto auto',
              gap: 6, alignItems: 'center', padding: '3px 0',
              borderBottom: '1px dashed var(--rule-2)',
            }}>
              <span style={{
                width: 10, height: 10,
                background: r.colors.length > 1 && r.colors !== '?' ? colorIdGradient(r.colors) : (MANA_COLORS[r.colors[0]] || MANA_COLORS.C),
                border: '1px solid var(--rule-2)',
              }} />
              <span className="t-xs" style={{ fontWeight: 700, letterSpacing: '0.04em' }}>
                {r.colors === '?' ? t('shr.unknown') : r.colors === 'C' ? t('dka.ci_colorless') : r.colors}
              </span>
              <span className="t-xs muted" style={{ fontVariantNumeric: 'tabular-nums' }}>
                {t('meta.legend_games', { count: r.games.toLocaleString(), pct: pct.toFixed(1) })}
              </span>
              <span className="t-xs text-right" style={{ fontVariantNumeric: 'tabular-nums', fontWeight: 700, color: wr >= 40 ? 'var(--ok)' : 'var(--ink-2)' }}>
                {wr.toFixed(1)}%
              </span>
              <span className="t-xs muted-2 text-right" style={{ fontVariantNumeric: 'tabular-nums' }}>
                WR
              </span>
            </div>
          )
        })}
      </div>
    </div>
  )
}

// ArchetypeBars — vertical bar chart, one bar per archetype. Bar
// height is deck count, label below.
function ArchetypeBars({ rows }) {
  const t = useT()
  if (!rows || rows.length === 0) {
    return (
      <div className="t-md muted" style={{ padding: '14px 0', textAlign: 'center' }}>
        {t('meta.no_freya')}
      </div>
    )
  }
  const max = Math.max(...rows.map(r => r.decks), 1)
  const w = 720
  const h = 200
  const barGap = 12
  const barW = Math.max(20, (w - barGap * (rows.length + 1)) / rows.length)
  const labelH = 56
  const chartH = h - labelH

  return (
    <svg viewBox={`0 0 ${w} ${h}`} width="100%" preserveAspectRatio="xMinYMin meet" style={{ display: 'block' }}>
      {rows.map((r, i) => {
        const x = barGap + i * (barW + barGap)
        const bh = (r.decks / max) * (chartH - 10)
        const y = chartH - bh
        return (
          <g key={r.archetype + i}>
            <rect x={x} y={y} width={barW} height={bh} fill="var(--accent)" />
            <text x={x + barW / 2} y={y - 4} textAnchor="middle"
              fontSize="11" fontWeight="800" fill="var(--ink)"
              style={{ fontVariantNumeric: 'tabular-nums' }}>
              {r.decks}
            </text>
            <text x={x + barW / 2} y={chartH + 16} textAnchor="middle"
              fontSize="9" fill="var(--ink-2)"
              style={{ letterSpacing: '0.06em', textTransform: 'uppercase' }}>
              {r.archetype.length > 11 ? r.archetype.slice(0, 10) + '…' : r.archetype}
            </text>
          </g>
        )
      })}
      {/* baseline */}
      <line x1={0} y1={chartH} x2={w} y2={chartH} stroke="var(--rule-2)" strokeWidth="1" />
    </svg>
  )
}

// BracketHistogram — fixed columns (B0–B5) with deck counts. Uses
// kind colors so brackets read at-a-glance: B1-2 ok, B3 ink-2,
// B4-5 warn (high power level).
function BracketHistogram({ rows }) {
  const t = useT()
  // Fill gaps so we always render B0..B5 even when some are empty.
  const counts = {}
  for (const r of (rows || [])) counts[r.bracket] = r.decks
  const buckets = []
  for (let b = 0; b <= 5; b++) buckets.push({ bracket: b, decks: counts[b] || 0 })
  const totalDecks = buckets.reduce((s, r) => s + r.decks, 0)
  if (totalDecks === 0) {
    return (
      <div className="t-md muted" style={{ padding: '14px 0', textAlign: 'center' }}>
        {t('meta.no_deck_elo')}
      </div>
    )
  }
  const max = Math.max(...buckets.map(r => r.decks), 1)
  const w = 480
  const h = 180
  const padX = 14
  const colW = (w - padX * 2) / buckets.length
  const labelH = 50
  const chartH = h - labelH

  const bracketColor = (b) => {
    if (b === 0) return 'var(--ink-3)'
    if (b <= 2) return 'var(--ok)'
    if (b === 3) return 'var(--ink-2)'
    return 'var(--warn)'
  }

  return (
    <svg viewBox={`0 0 ${w} ${h}`} width="100%" preserveAspectRatio="xMinYMin meet" style={{ display: 'block' }}>
      {buckets.map((r, i) => {
        const x = padX + i * colW + 8
        const bw = colW - 16
        const bh = (r.decks / max) * (chartH - 10)
        const y = chartH - bh
        return (
          <g key={r.bracket}>
            <rect x={x} y={y} width={bw} height={bh} fill={bracketColor(r.bracket)} />
            <text x={x + bw / 2} y={y - 4} textAnchor="middle"
              fontSize="11" fontWeight="800" fill="var(--ink)"
              style={{ fontVariantNumeric: 'tabular-nums' }}>
              {r.decks}
            </text>
            <text x={x + bw / 2} y={chartH + 18} textAnchor="middle"
              fontSize="14" fontWeight="800" fill="var(--ink-2)"
              style={{ letterSpacing: '0.06em' }}>
              B{r.bracket}
            </text>
            <text x={x + bw / 2} y={chartH + 32} textAnchor="middle"
              fontSize="9" fill="var(--ink-3)"
              style={{ letterSpacing: '0.06em', textTransform: 'uppercase' }}>
              {r.bracket === 0 ? t('shelf.unrated') : r.bracket <= 2 ? t('meta.tier_casual') : r.bracket === 3 ? t('meta.tier_optimized') : 'CEDH'}
            </text>
          </g>
        )
      })}
      <line x1={padX} y1={chartH} x2={w - padX} y2={chartH} stroke="var(--rule-2)" strokeWidth="1" />
    </svg>
  )
}

export default function Meta() {
  const t = useT()
  const [data, setData] = useState(null)
  const [error, setError] = useState(null)

  useEffect(() => {
    let cancelled = false
    fetch(`${API_BASE}/api/meta`)
      .then(r => r.ok ? r.json() : Promise.reject(new Error(`meta ${r.status}`)))
      .then(d => { if (!cancelled) setData(d) })
      .catch(e => { if (!cancelled) setError(e.message || FETCH_FAILED) })
    return () => { cancelled = true }
  }, [])

  const totalGames = data?.total_games || 0

  return (
    <>
      <Tape
        left="META / / DOC HX-200"
        mid={data
          ? t('meta.games_sampled', { count: totalGames.toLocaleString() })
          : (error ? t('spec.tape_offline') : t('common.loading'))}
        right="REV C.25"
      />

      <div className="meta-page-body" style={{ padding: '24px 30px', maxWidth: 1100, margin: '0 auto', display: 'flex', flexDirection: 'column', gap: 16, overflowX: 'hidden', width: '100%', boxSizing: 'border-box' }}>
        {error && (
          <div className="panel" style={{ borderColor: 'var(--danger)' }}>
            <div className="panel-bd t-xs" style={{ color: 'var(--danger)' }}>
              {t('meta.feed_unavailable', {
                error: error === FETCH_FAILED ? t('dka.fetch_failed') : error.toUpperCase(),
              })}
            </div>
          </div>
        )}

        {!data && !error && (
          <div className="t-md muted" style={{ padding: '40px 0', textAlign: 'center' }}>
            {t('meta.fetching')}<span className="blink">_</span>
          </div>
        )}

        {data && (
          <>
            <Panel
              code="META.A"
              title={t('meta.title_commanders')}
              right={<Tag solid>{data.top_commanders?.length || 0}</Tag>}
            >
              <CommandersBarChart rows={data.top_commanders} />
            </Panel>

            <div className="grid col-2 gap-3">
              <Panel
                code="META.B"
                title={t('meta.title_ci')}
                right={<Tag solid>{data.color_identity_winrates?.length || 0}</Tag>}
              >
                <ColorIdentityPie rows={data.color_identity_winrates} />
              </Panel>

              <Panel
                code="META.D"
                title={t('crd.bracket_dist')}
                right={<Tag solid>B0–B5</Tag>}
              >
                <BracketHistogram rows={data.bracket_distribution} />
              </Panel>
            </div>

            <Panel
              code="META.C"
              title={t('meta.title_archetypes', { count: (data.top_archetypes || []).length })}
              right={<Tag solid>FREYA</Tag>}
            >
              <ArchetypeBars rows={data.top_archetypes} />
              <div className="t-xs muted-2" style={{ marginTop: 6, lineHeight: 1.55 }}>
                {t('meta.sourced_from', { path: 'data/decks/<OWNER>/freya/<ID>.STRATEGY.JSON' })}
              </div>
            </Panel>

            <div className="t-xs muted-2" style={{ textAlign: 'center', letterSpacing: '0.10em', marginTop: 8 }}>
              {t('meta.footer')}{' '}
              <Link to="/spectate" style={{ color: 'var(--ink-2)' }}>{t('splash.fishtank_watch')}</Link>
            </div>
          </>
        )}
      </div>
    </>
  )
}
