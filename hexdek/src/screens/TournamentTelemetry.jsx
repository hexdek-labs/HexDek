import { useEffect, useMemo, useState } from 'react'
import { useParams, useSearchParams } from 'react-router-dom'
import { Panel, Tag, Tape } from '../components/chrome'
import { API_BASE } from '../services/api'
import { useT } from '../i18n'

// TournamentTelemetry — /tournaments/:id/telemetry
//
// Renders the derived analytics layer documented in
// docs/tournament-telemetry-schema.md:
//
//   * Convergence card — TrueSkill σ progress + "converged" badge
//   * ELO distribution — min/max/mean/spread/std_dev card row
//   * Commanders table — sorted by ELO desc
//   * Round-by-round win % heatmap — rounds × commanders matrix
//   * Archetype matrix — per-archetype winrate rollup
//   * OMW% trajectory — per-deck line chart across rounds
//   * Sigma convergence — per-round mean σ trend line
//
// Data source: GET /api/tournament/{id}/telemetry. The endpoint is
// not yet wired (telemetry persistence pending) so this screen
// falls back to an embedded sample fixture and renders a clear
// "showing sample data" notice. Once the endpoint lands, the
// fallback path elides itself.

// ---- Embedded sample fixture (mirrors testdata/swiss_converged_fixture.json
// after ComputeTelemetry processing). Used when the backend hasn't yet
// produced a telemetry payload for the requested run id.
const SAMPLE_TELEMETRY = {
  schema_version: '1.0.0',
  source_mode: 'swiss',
  games: 12,
  crashes: 0,
  commanders: [
    { name: 'Atraxa', games: 12, wins: 6, win_rate: 0.500, elo: 1620.0, trueskill_mu: 30.5, trueskill_sigma: 3.8, archetype: 'Combo / Infinite' },
    { name: 'Krenko', games: 12, wins: 3, win_rate: 0.250, elo: 1510.0, trueskill_mu: 26.0, trueskill_sigma: 4.0, archetype: 'Aggro / Go Wide' },
    { name: 'Yuriko', games: 12, wins: 2, win_rate: 0.167, elo: 1470.0, trueskill_mu: 24.0, trueskill_sigma: 4.1, archetype: 'Combo / Infinite' },
    { name: 'Edric',  games: 12, wins: 1, win_rate: 0.083, elo: 1400.0, trueskill_mu: 19.5, trueskill_sigma: 4.2, archetype: 'Control' },
  ],
  convergence: {
    initial_sigma: 8.333,
    mean_sigma: 4.025,
    min_sigma: 3.8,
    max_sigma: 4.2,
    convergence_ratio: 0.517,
    converged: true,
  },
  elo_distribution: { min: 1400.0, max: 1620.0, mean: 1500.0, std_dev: 80.6, spread: 220.0 },
  archetype_matrix: {
    'Combo / Infinite': { decks: 2, games: 24, wins: 8, win_rate: 0.333 },
    'Aggro / Go Wide':  { decks: 1, games: 12, wins: 3, win_rate: 0.250 },
    'Control':          { decks: 1, games: 12, wins: 1, win_rate: 0.083 },
  },
  rounds: [
    {
      round: 1, sigma_mean: 6.675, mu_spread: 3.0,
      win_rates: { Atraxa: 1.0, Krenko: 0.0, Yuriko: 0.0, Edric: 0.0 },
      omw: { Atraxa: 0.0, Krenko: 0.33, Yuriko: 0.33, Edric: 0.33 },
    },
    {
      round: 2, sigma_mean: 5.7, mu_spread: 5.5,
      win_rates: { Atraxa: 1.0, Krenko: 0.5, Yuriko: 0.0, Edric: 0.0 },
      omw: { Atraxa: 0.20, Krenko: 0.25, Yuriko: 0.50, Edric: 0.50 },
    },
    {
      round: 3, sigma_mean: 4.025, mu_spread: 11.0,
      win_rates: { Atraxa: 0.5, Krenko: 0.25, Yuriko: 0.167, Edric: 0.083 },
      omw: { Atraxa: 0.25, Krenko: 0.30, Yuriko: 0.33, Edric: 0.35 },
    },
  ],
}

// ---- helpers ----

const fmtPct = v => (v == null ? '—' : `${(v * 100).toFixed(1)}%`)
const fmtFloat = (v, p = 2) => (v == null ? '—' : v.toFixed(p))

// Color stop for a win-rate cell in the heatmap. 0%=ink-2 (cold)
// up through 100%=ok (hot). Mirrors the meta-screen accent ramp.
function heatColor(rate) {
  if (rate == null || rate <= 0) return 'var(--rule-2)'
  if (rate >= 0.6) return 'var(--ok)'
  if (rate >= 0.4) return 'var(--accent)'
  if (rate >= 0.2) return 'var(--ink-2)'
  return 'var(--rule-2)'
}

// ---- subcomponents ----

function ConvergenceCard({ c }) {
  // useT before the early return - it is a hook.
  const t = useT()
  if (!c) return null
  const pct = (c.convergence_ratio * 100).toFixed(1)
  return (
    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 12 }}>
      <Stat label={t('tlm.status')} value={c.converged ? t('tlm.converged') : t('tlm.open')} kind={c.converged ? 'ok' : 'warn'} />
      <Stat label={t('tlm.ratio')} value={`${pct}%`} sub={t('tlm.thresh', { value: '50.0%' })} />
      <Stat label={t('tlm.mean_sigma')} value={fmtFloat(c.mean_sigma)} sub={t('tlm.prior', { value: fmtFloat(c.initial_sigma) })} />
      <Stat label={t('tlm.sigma_range')} value={`${fmtFloat(c.min_sigma)}-${fmtFloat(c.max_sigma)}`} sub={t('tlm.gap', { value: fmtFloat(c.max_sigma - c.min_sigma) })} />
    </div>
  )
}

function ELODistributionCard({ d }) {
  const t = useT()
  if (!d) return null
  return (
    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(5, 1fr)', gap: 12 }}>
      <Stat label={t('tlm.min')} value={fmtFloat(d.min, 0)} />
      <Stat label={t('tlm.max')} value={fmtFloat(d.max, 0)} />
      <Stat label={t('tlm.mean')} value={fmtFloat(d.mean, 0)} />
      <Stat label={t('tlm.std_dev')} value={fmtFloat(d.std_dev, 1)} />
      <Stat label={t('tlm.spread')} value={fmtFloat(d.spread, 0)} />
    </div>
  )
}

function Stat({ label, value, sub, kind }) {
  const tone = kind === 'ok' ? 'var(--ok)' : kind === 'warn' ? 'var(--danger)' : 'var(--ink)'
  return (
    <div className="panel" style={{ padding: '12px 14px' }}>
      <div className="t-xs muted" style={{ letterSpacing: '0.08em' }}>{label}</div>
      <div style={{ fontSize: 22, fontWeight: 700, color: tone, lineHeight: 1.1, marginTop: 4 }}>{value}</div>
      {sub && <div className="t-xs muted" style={{ marginTop: 4 }}>{sub}</div>}
    </div>
  )
}

function CommandersTable({ rows }) {
  const t = useT()
  if (!rows || rows.length === 0) return <Empty>{t('tlm.no_commanders')}</Empty>
  return (
    <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 12 }}>
      <thead>
        <tr style={{ textAlign: 'left', borderBottom: '1px solid var(--rule-2)' }}>
          <th style={th}>{t('tlm.th_commander')}</th>
          <th style={th}>{t('tlm.th_archetype')}</th>
          <th style={thR}>{t('tlm.th_games')}</th>
          <th style={thR}>{t('tlm.th_wins')}</th>
          <th style={thR}>{t('tlm.th_winpct')}</th>
          <th style={thR}>ELO</th>
          <th style={thR}>μ</th>
          <th style={thR}>σ</th>
        </tr>
      </thead>
      <tbody>
        {rows.map(r => (
          <tr key={r.name} style={{ borderBottom: '1px solid var(--rule-3, var(--rule-2))' }}>
            <td style={td}><strong>{r.name}</strong></td>
            <td style={td}>{r.archetype || <span className="muted">—</span>}</td>
            <td style={tdR}>{r.games}</td>
            <td style={tdR}>{r.wins}</td>
            <td style={tdR}>{fmtPct(r.win_rate)}</td>
            <td style={tdR}>{fmtFloat(r.elo, 0)}</td>
            <td style={tdR}>{fmtFloat(r.trueskill_mu)}</td>
            <td style={tdR}>{fmtFloat(r.trueskill_sigma)}</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}

function WinRateHeatmap({ rounds, commanders }) {
  const t = useT()
  if (!rounds || rounds.length === 0) return <Empty>{t('tlm.no_round_data')}</Empty>
  const names = commanders.map(c => c.name)
  const cellH = 26
  const cellW = 78
  const labelW = 140
  const headerH = 26
  const w = labelW + names.length * cellW
  const h = headerH + rounds.length * cellH
  return (
    <svg viewBox={`0 0 ${w} ${h}`} width="100%" style={{ display: 'block', maxHeight: 400 }}>
      {/* column headers */}
      {names.map((n, i) => (
        <text key={n} x={labelW + i * cellW + cellW / 2} y={headerH - 8} textAnchor="middle"
          fontSize="10" fontWeight="600" fill="var(--ink)" style={{ letterSpacing: '0.06em' }}>
          {n.toUpperCase().slice(0, 10)}
        </text>
      ))}
      {/* rows */}
      {rounds.map((r, ri) => {
        const y = headerH + ri * cellH
        return (
          <g key={r.round}>
            <text x={labelW - 8} y={y + cellH / 2 + 4} textAnchor="end" fontSize="11"
              fontWeight="600" fill="var(--ink-2)">R{r.round}</text>
            {names.map((n, ci) => {
              const wr = r.win_rates?.[n] ?? null
              const x = labelW + ci * cellW
              return (
                <g key={n}>
                  <rect x={x + 2} y={y + 2} width={cellW - 4} height={cellH - 4}
                    fill={heatColor(wr)} opacity="0.75" />
                  <text x={x + cellW / 2} y={y + cellH / 2 + 4} textAnchor="middle"
                    fontSize="11" fontWeight="600" fill="var(--ink)">
                    {wr == null ? '—' : `${(wr * 100).toFixed(0)}%`}
                  </text>
                </g>
              )
            })}
          </g>
        )
      })}
    </svg>
  )
}

function ArchetypeMatrix({ matrix }) {
  const t = useT()
  const rows = useMemo(() => {
    if (!matrix) return []
    return Object.entries(matrix)
      .map(([arch, stats]) => ({ archetype: arch, ...stats }))
      .sort((a, b) => b.win_rate - a.win_rate)
  }, [matrix])
  if (rows.length === 0) return <Empty>{t('tlm.no_archetype_tags')}</Empty>
  const maxWR = Math.max(...rows.map(r => r.win_rate), 0.01)
  return (
    <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 12 }}>
      <thead>
        <tr style={{ textAlign: 'left', borderBottom: '1px solid var(--rule-2)' }}>
          <th style={th}>{t('tlm.th_archetype')}</th>
          <th style={thR}>{t('tlm.th_decks')}</th>
          <th style={thR}>{t('tlm.th_games')}</th>
          <th style={thR}>{t('tlm.th_wins')}</th>
          <th style={thR}>{t('tlm.th_winpct')}</th>
          <th style={{ ...th, width: '40%' }}></th>
        </tr>
      </thead>
      <tbody>
        {rows.map(r => (
          <tr key={r.archetype} style={{ borderBottom: '1px solid var(--rule-3, var(--rule-2))' }}>
            <td style={td}><strong>{r.archetype}</strong></td>
            <td style={tdR}>{r.decks}</td>
            <td style={tdR}>{r.games}</td>
            <td style={tdR}>{r.wins}</td>
            <td style={tdR}>{fmtPct(r.win_rate)}</td>
            <td style={{ ...td, padding: '6px 8px' }}>
              <div style={{ height: 8, background: 'var(--rule-2)', position: 'relative' }}>
                <div style={{
                  position: 'absolute', left: 0, top: 0, bottom: 0,
                  width: `${(r.win_rate / maxWR) * 100}%`,
                  background: r.win_rate >= 0.4 ? 'var(--ok)' : r.win_rate >= 0.25 ? 'var(--accent)' : 'var(--ink-2)',
                }} />
              </div>
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}

// LineChart — generic per-deck trajectory chart (used for OMW% + sigma).
// `series` is [{ name, points: [{ x, y }] }, ...]. yDomain = [0, max].
function LineChart({ series, height = 220, yLabel, xLabel, yFormat = v => v.toFixed(2), yMaxOverride }) {
  const t = useT()
  if (!series || series.length === 0) return <Empty>{t('tlm.no_traj_data')}</Empty>
  const allX = series.flatMap(s => s.points.map(p => p.x))
  const allY = series.flatMap(s => s.points.map(p => p.y))
  if (allX.length === 0) return <Empty>{t('tlm.no_traj_points')}</Empty>
  const minX = Math.min(...allX), maxX = Math.max(...allX)
  const minY = 0
  const maxY = yMaxOverride ?? Math.max(...allY, 0.1)
  const pad = { t: 18, r: 110, b: 32, l: 44 }
  const w = 640, h = height
  const innerW = w - pad.l - pad.r, innerH = h - pad.t - pad.b
  const sx = x => pad.l + (maxX === minX ? innerW / 2 : ((x - minX) / (maxX - minX)) * innerW)
  const sy = y => pad.t + innerH - ((y - minY) / (maxY - minY || 1)) * innerH
  const seriesColors = ['var(--ok)', 'var(--accent)', 'var(--ink-2)', 'var(--danger)', 'var(--ink)']

  return (
    <svg viewBox={`0 0 ${w} ${h}`} width="100%" style={{ display: 'block', maxHeight: 320 }}>
      {/* y axis */}
      <line x1={pad.l} y1={pad.t} x2={pad.l} y2={pad.t + innerH} stroke="var(--rule-2)" />
      {/* x axis */}
      <line x1={pad.l} y1={pad.t + innerH} x2={pad.l + innerW} y2={pad.t + innerH} stroke="var(--rule-2)" />
      {/* y ticks */}
      {[0, 0.25, 0.5, 0.75, 1].map(f => {
        const y = pad.t + innerH - f * innerH
        return (
          <g key={f}>
            <line x1={pad.l - 4} y1={y} x2={pad.l} y2={y} stroke="var(--rule-2)" />
            <text x={pad.l - 6} y={y + 3} textAnchor="end" fontSize="9" fill="var(--ink-2)">
              {yFormat(minY + f * (maxY - minY))}
            </text>
          </g>
        )
      })}
      {/* x ticks — assume integer x (rounds) */}
      {Array.from(new Set(allX)).sort((a, b) => a - b).map(xv => {
        const x = sx(xv)
        return (
          <g key={xv}>
            <line x1={x} y1={pad.t + innerH} x2={x} y2={pad.t + innerH + 4} stroke="var(--rule-2)" />
            <text x={x} y={pad.t + innerH + 16} textAnchor="middle" fontSize="9" fill="var(--ink-2)">
              R{xv}
            </text>
          </g>
        )
      })}
      {/* axis labels */}
      {yLabel && (
        <text x={pad.l - 32} y={pad.t + innerH / 2} textAnchor="middle" fontSize="10" fill="var(--ink-2)"
          transform={`rotate(-90 ${pad.l - 32} ${pad.t + innerH / 2})`}>
          {yLabel}
        </text>
      )}
      {xLabel && (
        <text x={pad.l + innerW / 2} y={h - 6} textAnchor="middle" fontSize="10" fill="var(--ink-2)">
          {xLabel}
        </text>
      )}
      {/* series */}
      {series.map((s, idx) => {
        const color = seriesColors[idx % seriesColors.length]
        const d = s.points.map((p, i) => `${i === 0 ? 'M' : 'L'}${sx(p.x)},${sy(p.y)}`).join(' ')
        return (
          <g key={s.name}>
            <path d={d} fill="none" stroke={color} strokeWidth="1.5" />
            {s.points.map(p => (
              <circle key={`${s.name}-${p.x}`} cx={sx(p.x)} cy={sy(p.y)} r="2.5" fill={color} />
            ))}
            {/* legend */}
            <g transform={`translate(${pad.l + innerW + 8}, ${pad.t + idx * 18 + 6})`}>
              <line x1={0} y1={4} x2={12} y2={4} stroke={color} strokeWidth="1.5" />
              <circle cx={6} cy={4} r="2.5" fill={color} />
              <text x={18} y={7} fontSize="10" fill="var(--ink)">{s.name}</text>
            </g>
          </g>
        )
      })}
    </svg>
  )
}

function Empty({ children }) {
  return (
    <div className="t-md muted" style={{ padding: '14px 0', textAlign: 'center' }}>
      &gt; {children}
    </div>
  )
}

const th = { padding: '6px 8px', fontSize: 10, fontWeight: 600, letterSpacing: '0.08em', color: 'var(--ink-2)' }
const thR = { ...th, textAlign: 'right' }
const td = { padding: '6px 8px' }
const tdR = { ...td, textAlign: 'right', fontVariantNumeric: 'tabular-nums' }

// ---- main screen ----

// Sentinel for "the fetch failed and carried no message of its own". A
// worded fallback parked in state would freeze in whatever language was
// active at fetch time, so state holds the catalog path and the render
// site words it.
const FETCH_FAILED = 'tlm.fetch_failed_short'

export default function TournamentTelemetry() {
  const t = useT()
  const { id = 'sample' } = useParams()
  const [searchParams] = useSearchParams()
  const [data, setData] = useState(null)
  const [error, setError] = useState(null)
  const [isSample, setIsSample] = useState(false)

  useEffect(() => {
    if (id === 'sample' || searchParams.get('sample') === '1') {
      setData(SAMPLE_TELEMETRY)
      setIsSample(true)
      return
    }
    let cancelled = false
    fetch(`${API_BASE}/api/tournament/${id}/telemetry`)
      .then(r => {
        if (r.status === 404) {
          // Endpoint not yet wired or unknown id — fall back to sample
          // so the screen is still demoable.
          return null
        }
        if (!r.ok) throw new Error(`telemetry ${r.status}`)
        return r.json()
      })
      .then(d => {
        if (cancelled) return
        if (!d) {
          setData(SAMPLE_TELEMETRY)
          setIsSample(true)
        } else {
          setData(d)
          setIsSample(false)
        }
      })
      .catch(e => {
        if (cancelled) return
        // Network / parse failure — show sample plus an error notice.
        setError(e.message || FETCH_FAILED)
        setData(SAMPLE_TELEMETRY)
        setIsSample(true)
      })
    return () => { cancelled = true }
  }, [id, searchParams])

  // Build OMW% trajectory series (one line per commander).
  const omwSeries = useMemo(() => {
    if (!data?.rounds || !data?.commanders) return []
    return data.commanders.map(c => ({
      name: c.name,
      points: data.rounds
        .filter(r => r.omw && r.omw[c.name] != null)
        .map(r => ({ x: r.round, y: r.omw[c.name] })),
    })).filter(s => s.points.length > 0)
  }, [data])

  // Sigma trajectory (single line — mean σ across rounds).
  const sigmaPoints = useMemo(() => {
    if (!data?.rounds) return []
    return data.rounds.map(r => ({ x: r.round, y: r.sigma_mean }))
  }, [data])
  // Worded out here, not inside the memo: the legend label is the one
  // string on this screen that a memo would hold across a switch.
  const sigmaSeries = sigmaPoints.length
    ? [{ name: t('tlm.mean_sigma'), points: sigmaPoints }]
    : []
  const sigmaMax = data?.convergence?.initial_sigma || 8.333

  return (
    <>
      <Tape
        left={t('tlm.tape_left', { what: id === 'sample' ? t('tlm.tape_sample') : t('tlm.tape_run', { id }) })}
        mid={data
          ? t('tlm.tape_games', { count: data.games, mode: (data.source_mode || '').toUpperCase() })
          : (error ? t('tlm.tape_error') : t('tlm.tape_loading'))}
        right={data?.schema_version ? t('tlm.tape_schema', { version: data.schema_version }) : ''}
      />

      <div style={{ padding: '24px 30px', maxWidth: 1100, margin: '0 auto', display: 'flex', flexDirection: 'column', gap: 16 }}>
        {isSample && (
          <div className="panel" style={{ borderColor: 'var(--accent, var(--ink-2))' }}>
            <div className="panel-bd t-xs">
              {/* The two route templates ride in as values so they are written
                  once here rather than 25 times across the catalogs. */}
              &gt; {t('tlm.sample_notice', {
                endpoint: '/api/tournament/{id}/telemetry',
                path: '/tournaments/{runId}/telemetry',
              })}
            </div>
          </div>
        )}
        {error && !isSample && (
          <div className="panel" style={{ borderColor: 'var(--danger)' }}>
            <div className="panel-bd t-xs" style={{ color: 'var(--danger)' }}>
              &gt; {t('tlm.fetch_failed', { error: error === FETCH_FAILED ? t(FETCH_FAILED) : error })}
            </div>
          </div>
        )}

        {!data && !error && (
          <div className="t-md muted" style={{ padding: '40px 0', textAlign: 'center' }}>
            &gt; {t('tlm.fetching')}<span className="blink">_</span>
          </div>
        )}

        {data && (
          <>
            <Panel
              code="TLM.A"
              title={t('tlm.p_convergence')}
              right={<Tag solid kind={data.convergence?.converged ? 'ok' : undefined}>
                {data.convergence?.converged ? t('tlm.yes') : t('tlm.open')}
              </Tag>}
            >
              <ConvergenceCard c={data.convergence} />
            </Panel>

            <Panel
              code="TLM.B"
              title={t('tlm.p_elo')}
            >
              <ELODistributionCard d={data.elo_distribution} />
            </Panel>

            <Panel
              code="TLM.C"
              title={t('tlm.p_commanders')}
              right={<Tag solid>{data.commanders?.length || 0}</Tag>}
            >
              <CommandersTable rows={data.commanders} />
            </Panel>

            <Panel
              code="TLM.D"
              title={t('tlm.p_heatmap', { rounds: data.rounds?.length || 0 })}
            >
              <WinRateHeatmap rounds={data.rounds} commanders={data.commanders || []} />
            </Panel>

            <Panel
              code="TLM.E"
              title={t('tlm.p_archetype')}
              right={<Tag solid>{Object.keys(data.archetype_matrix || {}).length}</Tag>}
            >
              <ArchetypeMatrix matrix={data.archetype_matrix} />
            </Panel>

            <Panel
              code="TLM.F"
              title={t('tlm.p_omw')}
            >
              <LineChart
                series={omwSeries}
                yLabel="OMW %"
                xLabel={t('tlm.ax_round')}
                yFormat={v => `${(v * 100).toFixed(0)}%`}
              />
            </Panel>

            <Panel
              code="TLM.G"
              title={t('tlm.p_sigma')}
            >
              <LineChart
                series={sigmaSeries}
                yLabel="σ"
                xLabel={t('tlm.ax_round')}
                yFormat={v => v.toFixed(2)}
                yMaxOverride={sigmaMax}
              />
            </Panel>
          </>
        )}
      </div>
    </>
  )
}
