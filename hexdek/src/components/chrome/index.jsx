import { Fragment } from 'react'
import GlossaryTerm from '../GlossaryTerm'
import { useT } from '../../i18n'

export const Crops = () => (
  <>
    <span className="crop crop--tl" />
    <span className="crop crop--tr" />
    <span className="crop crop--bl" />
    <span className="crop crop--br" />
  </>
)

export const Panel = ({ title, code, right, children, solid, style, className = '' }) => (
  <div className={`panel ${solid ? 'panel--solid' : ''} ${className}`} style={style}>
    {(title || right) && (
      <div className="panel-hd">
        <span>{code && <span className="muted-2" style={{ marginRight: 8 }}>{code}</span>}{title}</span>
        <span>{right}</span>
      </div>
    )}
    <div className="panel-bd">{children}</div>
  </div>
)

// KV — label/value rows with leader-dot separators. Each row is
// `[label, value]` or `[label, value, glossaryTerm]`. When a glossary
// term id is provided as the third tuple slot, the label is wrapped
// in <GlossaryTerm> so users can tap to read what the metric means.
export const KV = ({ rows }) => (
  <div className="kv">
    {rows.map((r, i) => {
      const term = r[2]
      const label = term ? <GlossaryTerm term={term} compact>{r[0]}</GlossaryTerm> : r[0]
      return (
        <Fragment key={i}>
          <span className="k">{label}</span>
          <span className="dots">{'.'.repeat(60)}</span>
          <span className="v">{r[1]}</span>
        </Fragment>
      )
    })}
  </div>
)

export const Bar = ({ value, max = 100, lg }) => (
  <div className={`bar ${lg ? 'bar--lg' : ''}`}>
    <i style={{ width: `${(value / max) * 100}%`, transition: 'width 0.3s ease' }} />
  </div>
)

export const Tag = ({ children, kind, solid, onClick, style }) => (
  <span className={`tag ${kind ? `tag--${kind}` : ''} ${solid ? 'tag--solid' : ''}`} onClick={onClick} style={style}>
    {children}
  </span>
)

export const Btn = ({ children, solid, sm, ghost, arrow = '↗', onClick, disabled, title, style }) => (
  <button
    className={`btn ${solid ? 'btn--solid' : ''} ${sm ? 'btn--sm' : ''} ${ghost ? 'btn--ghost' : ''}`}
    onClick={onClick}
    disabled={disabled}
    title={title}
    style={style}
  >
    <span>{children}</span>
    {arrow && <span className="arr">{arrow}</span>}
  </button>
)

export const Tape = ({ left, mid, right }) => (
  <div
    className="tape-bar flex items-center justify-between"
    style={{
      borderTop: '1px solid var(--rule-2)',
      borderBottom: '1px solid var(--rule-2)',
      padding: '4px 10px',
      fontSize: 9,
      letterSpacing: '0.1em',
      textTransform: 'uppercase',
      color: 'var(--ink-2)',
    }}
  >
    <span>{left}</span>
    {mid && <span className="muted-2">{mid}</span>}
    <span>{right}</span>
  </div>
)

export const Stripes = ({ height = 18, w = '100%' }) => (
  <div className="stripes" style={{ height, width: w }} />
)

export const MiniBars = ({ data }) => (
  <div className="minibars">
    {data.map((v, i) => <i key={i} style={{ height: `${v}%` }} />)}
  </div>
)

const CONFIDENCE_TIERS = [
  { min: 1000, labelKey: 'common.conf_locked',      dots: 5 },
  { min: 300,  labelKey: 'common.conf_high',        dots: 4 },
  { min: 100,  labelKey: 'common.conf_converging',  dots: 3 },
  { min: 50,   labelKey: 'common.conf_warming',     dots: 2 },
  { min: 0,    labelKey: 'common.conf_provisional', dots: 1 },
]

function getConfidenceTier(games) {
  for (const tier of CONFIDENCE_TIERS) {
    if (games >= tier.min) return tier
  }
  return CONFIDENCE_TIERS[CONFIDENCE_TIERS.length - 1]
}

export const ConfidenceDots = ({ games, showLabel, size = 'sm' }) => {
  const t = useT()
  const tier = getConfidenceTier(games || 0)
  const dotSize = size === 'lg' ? 10 : 6
  const gap = size === 'lg' ? 3 : 2
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: size === 'lg' ? 8 : 4 }} title={t('common.conf_title', { count: games, tier: t(tier.labelKey) })}>
      <span style={{ display: 'inline-flex', gap }}>
        {[1, 2, 3, 4, 5].map(i => (
          <span key={i} style={{
            width: dotSize, height: dotSize, borderRadius: '50%',
            background: i <= tier.dots ? 'var(--ink)' : 'transparent',
            border: `1px solid ${i <= tier.dots ? 'var(--ink)' : 'var(--ink-3)'}`,
            transition: 'background 0.2s',
          }} />
        ))}
      </span>
      {showLabel && <span className="t-xs muted">{t(tier.labelKey)}</span>}
    </span>
  )
}

const CMC_LABELS = ['0', '1', '2', '3', '4', '5', '6', '7+']
const MANA_COLORS = { W: '#E0EBD3', U: '#6E8FA0', B: '#3a3628', R: '#CC5C4A', G: '#82C472', C: '#8A9682' }
const COLOR_ORDER = ['W', 'U', 'B', 'R', 'G']

function parsePipsFromManaCost(manaCost) {
  if (!manaCost) return {}
  const pips = {}
  const matches = manaCost.match(/\{([^}]+)\}/g) || []
  for (const m of matches) {
    const sym = m.replace(/[{}]/g, '')
    if (COLOR_ORDER.includes(sym)) {
      pips[sym] = (pips[sym] || 0) + 1
    } else if (sym.includes('/')) {
      for (const c of COLOR_ORDER) {
        if (sym.includes(c)) pips[c] = (pips[c] || 0) + 1
      }
    }
  }
  return pips
}

export function computeColorByCmc(cards) {
  const grid = Array.from({ length: 8 }, () => ({}))
  if (!cards?.length) return null
  for (const c of cards) {
    const cmc = Math.min(c.cmc ?? 0, 7)
    const pips = parsePipsFromManaCost(c.mana_cost)
    for (const [color, count] of Object.entries(pips)) {
      grid[cmc][color] = (grid[cmc][color] || 0) + count * (c.quantity || 1)
    }
  }
  return grid
}

export const ManaCurveChart = ({ distribution, avgCmc, curveShape, warnings, landCount, nonlandCount, colorByCmc }) => {
  const t = useT()
  if (!distribution || !distribution.length) return null
  const max = Math.max(...distribution, 1)
  const barMaxHeight = 80
  const stacked = colorByCmc && colorByCmc.some(slot => Object.keys(slot).length > 0)
  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'flex-end', gap: 3, height: barMaxHeight + 20, padding: '0 2px' }}>
        {distribution.map((count, i) => {
          const slotColors = stacked ? colorByCmc[i] || {} : null
          const slotTotal = slotColors ? Object.values(slotColors).reduce((s, v) => s + v, 0) : 0
          const barH = Math.max((count / max) * barMaxHeight, 2)
          return (
            <div key={i} style={{ flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 2 }}>
              <span className="t-xs muted" style={{ fontSize: 8 }}>{count || ''}</span>
              {stacked && slotTotal > 0 ? (
                <div style={{ width: '100%', maxWidth: 32, height: barH, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
                  {COLOR_ORDER.filter(c => slotColors[c] > 0).map(c => (
                    <div key={c} style={{
                      flex: slotColors[c],
                      background: MANA_COLORS[c],
                      borderBottom: '1px solid var(--bg, #0c0d0a)',
                      minHeight: 2,
                    }} title={`${c}: ${slotColors[c]} pips`} />
                  ))}
                </div>
              ) : (
                <div style={{
                  width: '100%', maxWidth: 32, height: barH,
                  background: 'var(--ink)', opacity: count > 0 ? 1 : 0.15,
                  transition: 'height 0.3s ease',
                }} />
              )}
              <span className="t-xs muted" style={{ fontSize: 9 }}>{CMC_LABELS[i]}</span>
            </div>
          )
        })}
      </div>
      {stacked && (
        <div style={{ display: 'flex', gap: 8, justifyContent: 'center', marginTop: 4 }}>
          {COLOR_ORDER.map(c => (
            <span key={c} style={{ display: 'inline-flex', alignItems: 'center', gap: 2 }}>
              <span style={{ width: 6, height: 6, background: MANA_COLORS[c], border: '1px solid var(--rule-2)' }} />
              <span className="t-xs muted" style={{ fontSize: 8 }}>{c}</span>
            </span>
          ))}
        </div>
      )}
      <div className="hr" style={{ margin: '8px 0' }} />
      <div style={{ display: 'flex', justifyContent: 'space-between', flexWrap: 'wrap', gap: 8 }}>
        {avgCmc != null && <span className="t-xs">{t('common.avg_cmc')} <span className="punch">{avgCmc.toFixed(2)}</span></span>}
        {curveShape && <span className="t-xs muted">{t(curveShape)}</span>}
        {landCount != null && <span className="t-xs muted">{landCount}L / {nonlandCount}NL</span>}
      </div>
      {warnings && warnings.length > 0 && (
        <div style={{ marginTop: 6, display: 'flex', flexWrap: 'wrap', gap: 4 }}>
          {warnings.map((w, i) => <Tag key={i} kind="warn" solid>{w}</Tag>)}
        </div>
      )}
    </div>
  )
}

export const ColorPie = ({ demand }) => {
  if (!demand) return null
  const entries = Object.entries(demand).filter(([, v]) => v > 0).sort((a, b) => b[1] - a[1])
  const total = entries.reduce((s, [, v]) => s + v, 0)
  if (total === 0) return null
  let offset = 0
  const segments = entries.map(([color, count]) => {
    const pct = (count / total) * 100
    const seg = { color: MANA_COLORS[color] || 'var(--ink-3)', pct, offset, label: color, count }
    offset += pct
    return seg
  })
  const gradientParts = segments.flatMap(s => [
    `${s.color} ${s.offset}%`,
    `${s.color} ${s.offset + s.pct}%`,
  ])
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
      <div style={{
        width: 56, height: 56, borderRadius: '50%',
        background: `conic-gradient(${gradientParts.join(', ')})`,
        border: '2px solid var(--rule-2)',
        flexShrink: 0,
      }} />
      <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
        {segments.map(s => (
          <div key={s.label} style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
            <span style={{ width: 8, height: 8, background: s.color, border: '1px solid var(--rule-2)', borderRadius: 1, flexShrink: 0 }} />
            <span className="t-xs">{s.label}</span>
            <span className="t-xs muted">({s.count})</span>
          </div>
        ))}
      </div>
    </div>
  )
}
