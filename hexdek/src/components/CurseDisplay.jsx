import { useState, useEffect } from 'react'
import { Panel, KV, Tag } from './chrome'
import GlossaryTerm from './GlossaryTerm'
import { api } from '../services/api'
import { useT } from '../i18n'

// Three literals in this file used to MIRROR these values inline, which is a
// lie waiting to happen once the maths moves -- and baking them into 25
// catalogs would make it 25 lies. Named here, interpolated at the render site.
const PAR = 1.0            // fitness par; the sparkline's dashed baseline
const COLD_GAMES = 20      // observations required before corrections apply
const DEV_SATURATE = 0.2   // deviation at which a heatmap cell saturates

// 7 personality params, in radar-axis order. Order is the visual angle
// around the chart, starting at 12 o'clock. Keys match the JSON shape
// returned by /api/decks/:owner/:id/curse and the canonical key list in
// hat.CurseTraitKeys (Go side).
// `term` is a glossary id and stays. `labelKey` is a catalog PATH: a word
// stored here would freeze in whatever locale was active when the module
// first evaluated. The seven labels are a COORDINATED SET -- they render as
// the axes of a radar chart and as a 90px grid column, and one is abbreviated
// even in English -- so they are minted together rather than borrowed
// piecemeal from other namespaces that are not under that width constraint.
const TRAITS = [
  { key: 'aggression',         labelKey: 'crs.trait_aggression', term: 'curse_aggression' },
  { key: 'combo_patience',     labelKey: 'crs.trait_combo_pat',  term: 'curse_combo_patience' },
  { key: 'threat_paranoia',    labelKey: 'crs.trait_threat',     term: 'curse_threat_paranoia' },
  { key: 'resource_greed',     labelKey: 'crs.trait_greed',      term: 'curse_greed' },
  { key: 'political_memory',   labelKey: 'crs.trait_politics',   term: 'curse_politics' },
  { key: 'drain_affinity',     labelKey: 'crs.trait_drain',      term: 'curse_drain' },
  { key: 'artifact_affinity',  labelKey: 'crs.trait_artifact',   term: 'curse_artifact' },
]

function RadarChart({ values, locked, size = 220 }) {
  const t = useT()
  const cx = size / 2
  const cy = size / 2
  const r = size * 0.36
  const n = TRAITS.length
  const angle = (i) => -Math.PI / 2 + (2 * Math.PI * i) / n
  const point = (i, v) => {
    const a = angle(i)
    const rr = r * Math.max(0, Math.min(1, v))
    return [cx + Math.cos(a) * rr, cy + Math.sin(a) * rr]
  }
  const axisPoint = (i, frac = 1) => {
    const a = angle(i)
    return [cx + Math.cos(a) * r * frac, cy + Math.sin(a) * r * frac]
  }
  const polygon = values.map((v, i) => point(i, v).join(',')).join(' ')
  const rings = [0.25, 0.5, 0.75, 1.0]

  return (
    <div style={{ maxWidth: size, margin: '0 auto' }}>
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
      <polygon
        points={polygon}
        fill="var(--ok)"
        fillOpacity="0.18"
        stroke="var(--ok)"
        strokeWidth="1.5"
      />
      {values.map((v, i) => {
        const [px, py] = point(i, v)
        const isLocked = !!(locked && locked[TRAITS[i].key])
        // Locked traits render as solid filled dots in the warn color so
        // the radar visually distinguishes evolved vs owner-anchored axes.
        return (
          <circle
            key={i}
            cx={px}
            cy={py}
            r={isLocked ? 3.5 : 2.5}
            fill={isLocked ? 'var(--warn)' : 'var(--ok)'}
            stroke={isLocked ? 'var(--warn)' : 'none'}
            strokeWidth={isLocked ? 1 : 0}
          />
        )
      })}
      {TRAITS.map((tr, i) => {
        const [lx, ly] = axisPoint(i, 1.18)
        const anchor = Math.abs(lx - cx) < 4 ? 'middle' : lx < cx ? 'end' : 'start'
        const isLocked = !!(locked && locked[tr.key])
        return (
          <text
            key={tr.key}
            x={lx}
            y={ly}
            textAnchor={anchor}
            dominantBaseline="middle"
            fontSize="8"
            fill={isLocked ? 'var(--warn)' : 'var(--ink-2)'}
            style={{ letterSpacing: '0.04em', textTransform: 'uppercase', fontWeight: isLocked ? 700 : 400 }}
          >
            {isLocked ? '\u{1F512} ' : ''}{t(tr.labelKey)}
          </text>
        )
      })}
    </svg>
    </div>
  )
}

// TraitLockGrid renders a per-trait lock toggle + slider for owners.
// Each row: [label] [lock toggle] [slider when locked / value readout when free].
// Changes flow through onChange(nextConstraints) so the parent can PATCH
// once and propagate the new map back through the tree.
function TraitLockGrid({ values, constraints, onChange, busy }) {
  const t = useT()
  const setConstraint = (key, val) => {
    const next = { ...(constraints || {}) }
    next[key] = Math.max(0, Math.min(1, val))
    onChange(next)
  }
  const unlock = (key) => {
    const next = { ...(constraints || {}) }
    delete next[key]
    onChange(next)
  }
  return (
    <div style={{ display: 'grid', gridTemplateColumns: '1fr', gap: 4 }}>
      {TRAITS.map((tr, i) => {
        const cur = constraints && Object.prototype.hasOwnProperty.call(constraints, tr.key)
        const target = cur ? constraints[tr.key] : (values[i] ?? 0)
        return (
          <div
            key={tr.key}
            style={{
              display: 'grid',
              gridTemplateColumns: '90px 28px 1fr 42px',
              alignItems: 'center',
              gap: 6,
              padding: '4px 6px',
              border: '1px solid var(--rule-2)',
              background: cur ? 'color-mix(in srgb, var(--warn) 8%, transparent)' : 'transparent',
            }}
          >
            <div className="t-xs" style={{ fontWeight: 700, letterSpacing: '0.04em', color: cur ? 'var(--warn)' : 'var(--ink-2)' }}>
              <GlossaryTerm term={tr.term} compact>{t(tr.labelKey)}</GlossaryTerm>
            </div>
            <button
              type="button"
              disabled={busy}
              onClick={() => cur ? unlock(tr.key) : setConstraint(tr.key, values[i] ?? 0.5)}
              title={t(cur ? 'crs.tip_unlock' : 'crs.tip_lock')}
              style={{
                background: 'transparent',
                border: '1px solid var(--rule-2)',
                color: cur ? 'var(--warn)' : 'var(--ink-2)',
                padding: '2px 4px',
                cursor: busy ? 'wait' : 'pointer',
                fontSize: 12,
                lineHeight: 1,
              }}
            >
              {cur ? '\u{1F512}' : '\u{1F513}'}
            </button>
            {cur ? (
              <input
                type="range"
                min="0"
                max="1"
                step="0.01"
                value={target}
                disabled={busy}
                onChange={e => setConstraint(tr.key, parseFloat(e.target.value))}
                style={{ width: '100%' }}
              />
            ) : (
              <div style={{ height: 3, background: 'var(--rule-2)' }}>
                <div style={{ width: `${Math.round((values[i] ?? 0) * 100)}%`, height: '100%', background: 'var(--ok)' }} />
              </div>
            )}
            <div className="t-xs" style={{ textAlign: 'right', fontVariantNumeric: 'tabular-nums', color: cur ? 'var(--warn)' : 'var(--ink-2)' }}>
              {target.toFixed(2)}
            </div>
          </div>
        )
      })}
    </div>
  )
}

// FitnessSparkline plots best-fitness per generation across the last
// `window` generations. Values around 1.0 are par; >1 is above-par.
function FitnessSparkline({ values, width = 320, height = 56 }) {
  if (!values || values.length === 0) return null
  const n = values.length
  const max = Math.max(PAR, ...values)
  const min = Math.min(0, ...values)
  const range = max - min || 1
  const xStep = n > 1 ? (width - 8) / (n - 1) : 0
  const y = (v) => height - 4 - ((v - min) / range) * (height - 8)
  const x = (i) => 4 + i * xStep
  const path = values
    .map((v, i) => `${i === 0 ? 'M' : 'L'} ${x(i).toFixed(1)} ${y(v).toFixed(1)}`)
    .join(' ')
  const baselineY = y(PAR)
  return (
    <svg
      viewBox={`0 0 ${width} ${height}`}
      preserveAspectRatio="none"
      style={{ display: 'block', width: '100%', height }}
    >
      <line x1="0" y1={baselineY} x2={width} y2={baselineY} stroke="var(--rule-2)" strokeDasharray="3 3" strokeWidth="1" />
      <path d={path} fill="none" stroke="var(--ok)" strokeWidth="1.5" vectorEffect="non-scaling-stroke" />
      {values.map((v, i) => (
        <circle key={i} cx={x(i)} cy={y(v)} r="2" fill={v >= PAR ? 'var(--ok)' : 'var(--ink-2)'} vectorEffect="non-scaling-stroke" />
      ))}
    </svg>
  )
}

// DimHeatmap renders the 20 weight corrections as a 5×4 grid of cells
// tinted by deviation from 1.0. >1 (boosted) → green; <1 (suppressed)
// → red; ≈1 → neutral. Cold (no observations yet) renders muted with
// "—" labels so the panel reads honestly until data accumulates.
function DimHeatmap({ corrections, labels, n }) {
  const t = useT()
  if (!corrections || corrections.length === 0) return null
  const cold = !n || n < COLD_GAMES
  const cells = corrections.map((v, i) => ({
    label: labels?.[i] || `D${i}`,
    value: v,
    delta: v - 1.0,
  }))
  const cols = 5
  return (
    <div>
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: `repeat(${cols}, 1fr)`,
          gap: 3,
        }}
      >
        {cells.map((c, i) => {
          const intensity = Math.min(1, Math.abs(c.delta) / DEV_SATURATE)
          const colorVar = c.delta > 0 ? 'var(--ok)' : 'var(--danger)'
          const bg = cold
            ? 'transparent'
            : `color-mix(in srgb, ${colorVar} ${Math.round(intensity * 70)}%, transparent)`
          const sign = c.delta >= 0 ? '+' : ''
          const pct = `${sign}${Math.round(c.delta * 100)}`
          return (
            <div
              key={i}
              title={`${c.label}: ${c.value.toFixed(3)} (${sign}${(c.delta * 100).toFixed(1)}%)`}
              style={{
                border: '1px solid var(--rule-2)',
                background: bg,
                padding: '4px 5px',
                minHeight: 38,
                display: 'flex',
                flexDirection: 'column',
                justifyContent: 'space-between',
                opacity: cold ? 0.45 : 1,
              }}
            >
              <div
                style={{
                  fontSize: 7,
                  fontWeight: 700,
                  letterSpacing: '0.04em',
                  color: 'var(--ink-2)',
                  textTransform: 'uppercase',
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap',
                }}
              >
                {c.label}
              </div>
              <div
                style={{
                  fontSize: 10,
                  fontWeight: 800,
                  letterSpacing: '0.02em',
                  color: cold ? 'var(--ink-3)' : 'var(--ink)',
                  textAlign: 'right',
                  fontVariantNumeric: 'tabular-nums',
                }}
              >
                {cold ? '—' : pct}
              </div>
            </div>
          )
        })}
      </div>
      <div className="t-xs muted-2" style={{ marginTop: 4, lineHeight: 1.4 }}>
        {cold
          ? t('crs.heat_cold', { n: n || 0, threshold: COLD_GAMES })
          : t('crs.heat_note', { pct: Math.round(DEV_SATURATE * 100) })}
      </div>
    </div>
  )
}

export default function CurseDisplay({ curse, isOwner = false, deckId = null, onConstraintsChange }) {
  const t = useT()
  // Hooks must run unconditionally; bail out via render branch below.
  const remoteConstraints = curse?.constraints || null
  const [constraints, setConstraints] = useState(remoteConstraints)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState(null)

  // Keep local state in sync when the parent refetches the curse payload.
  useEffect(() => { setConstraints(remoteConstraints) }, [remoteConstraints])

  if (!curse || !curse.population || curse.population.length === 0) return null

  const pop = curse.population
  const sorted = [...pop].sort((a, b) => b.fitness - a.fitness)
  const top = sorted[0]
  const maxGen = pop.reduce((m, d) => Math.max(m, d.generation || 0), 0)
  const bestFitness = top.fitness
  const avgFitness = pop.reduce((s, d) => s + d.fitness, 0) / pop.length

  const submitConstraints = async (next) => {
    if (!deckId) return
    setBusy(true)
    setError(null)
    const prev = constraints
    setConstraints(next)
    try {
      const resp = await api.patchDeckCurse(deckId, next)
      const accepted = resp?.constraints || {}
      setConstraints(accepted)
      if (onConstraintsChange) onConstraintsChange(accepted)
    } catch (e) {
      setError(String(e.message || e))
      setConstraints(prev)
    } finally {
      setBusy(false)
    }
  }

  // Best fitness per generation, last 20 generations. Each member
  // carries the gen it was created in; group by gen and take the max
  // so the line plots evolution's leading edge over time.
  const fitnessByGen = (() => {
    const bestByGen = new Map()
    for (const d of pop) {
      const g = d.generation ?? 0
      const f = d.fitness ?? 0
      const cur = bestByGen.get(g)
      if (cur == null || f > cur) bestByGen.set(g, f)
    }
    const gens = [...bestByGen.keys()].sort((a, b) => a - b).slice(-20)
    return gens.map(g => bestByGen.get(g))
  })()

  const topValues = TRAITS.map(tr => top[tr.key] ?? 0)

  return (
    <Panel
      code="04.AM"
      title={t('crs.title')}
      right={<Tag solid>{t('crs.tag_dna', { count: pop.length, gen: maxGen })}</Tag>}
    >
      <KV rows={[
        [t('crs.kv_generations'), `${maxGen}`],
        [t('crs.kv_population'), `${pop.length}`],
        [t('crs.kv_games_logged'), `${(curse.total_games ?? curse.game_count ?? 0).toLocaleString()}`],
        [t('crs.kv_best_fitness'), <span style={{ color: bestFitness >= PAR ? 'var(--ok)' : 'var(--warn)', fontWeight: 700 }}>{bestFitness.toFixed(2)}</span>],
        [t('crs.kv_avg_fitness'), `${avgFitness.toFixed(2)}`],
        [t('crs.kv_top_games'), `${top.games_played ?? 0}`],
      ]} />

      <div className="hr" style={{ margin: '12px 0' }} />
      <div className="t-xs muted" style={{ marginBottom: 4 }}>{t('crs.hd_personality', { count: TRAITS.length })}</div>
      <RadarChart values={topValues} locked={constraints} />

      {isOwner && (
        <>
          <div className="hr" style={{ margin: '12px 0' }} />
          <div className="t-xs muted" style={{ marginBottom: 4, display: 'flex', justifyContent: 'space-between' }}>
            <span>{t('crs.hd_overrides')}</span>
            {busy && <span className="t-xs muted-2">{t('dka.saving_ell')}</span>}
          </div>
          <TraitLockGrid
            values={topValues}
            constraints={constraints}
            onChange={submitConstraints}
            busy={busy}
          />
          {error && <div className="t-xs" style={{ color: 'var(--danger)', marginTop: 4 }}>{t('frg.err_prefix', { error })}</div>}
          <div className="t-xs muted-2" style={{ marginTop: 4, lineHeight: 1.4 }}>
            {t('crs.locks_note')}
          </div>
        </>
      )}

      <div className="hr" style={{ margin: '12px 0' }} />
      <div className="t-xs muted" style={{ marginBottom: 4 }}>{t('crs.hd_fitness', { count: fitnessByGen.length })}</div>
      <FitnessSparkline values={fitnessByGen} />
      <div className="t-xs muted-2" style={{ marginTop: 2 }}>
        {t('crs.sparkline_note', { par: PAR.toFixed(2) })}
      </div>

      <div className="hr" style={{ margin: '12px 0' }} />
      <div className="t-xs muted" style={{ marginBottom: 6 }}>{t('crs.hd_dims', { count: (curse.dim_corrections || []).length })}</div>
      <DimHeatmap
        corrections={curse.dim_corrections}
        labels={curse.dim_labels}
        n={curse.dim_stats_n}
      />
    </Panel>
  )
}
