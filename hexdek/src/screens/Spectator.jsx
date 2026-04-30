import { useState, useEffect, useRef, useCallback, Fragment } from 'react'
import { useNavigate } from 'react-router-dom'
import { Panel, KV, Bar, Tag, Btn, Tape } from '../components/chrome'
import { cardArtUrl, API_BASE } from '../services/api'
import { useLiveSocket } from '../hooks/useLiveSocket'

const SPEED_MARKS = [0.2, 0.5, 1, 2, 5, 10, 20, 40, 100, 200]

const EVAL_GRID = [
  ['board_presence',  'card_advantage',      'mana_advantage'],
  ['combo_proximity', 'score',               'life_resource'],
  ['threat_exposure', 'commander_progress',  'graveyard_value'],
]

const EVAL_LABELS = {
  board_presence: 'Board',
  card_advantage: 'Cards',
  mana_advantage: 'Mana',
  combo_proximity: 'Combo',
  score: 'Score',
  life_resource: 'Life',
  threat_exposure: 'Threat',
  commander_progress: 'Cmdr',
  graveyard_value: 'Grave',
}

const MAGMA = [
  [0, 0, 4], [27, 6, 68], [72, 13, 106], [114, 28, 100],
  [159, 42, 99], [205, 71, 73], [237, 105, 37], [251, 155, 6],
  [252, 209, 55], [252, 253, 191],
]

function magma(t) {
  t = Math.max(0, Math.min(1, t))
  const n = MAGMA.length - 1
  const i = Math.min(t * n | 0, n - 1)
  const f = t * n - i
  const a = MAGMA[i], b = MAGMA[i + 1]
  return [a[0] + (b[0] - a[0]) * f | 0, a[1] + (b[1] - a[1]) * f | 0, a[2] + (b[2] - a[2]) * f | 0]
}

function drawEvalContour(canvas, ev) {
  if (!canvas || !ev) return
  const S = canvas.width
  const ctx = canvas.getContext('2d')
  const nm = v => ((v || 0) + 1) / 2
  const g = [
    [nm(ev.board_presence),  nm(ev.card_advantage),      nm(ev.mana_advantage)],
    [nm(ev.combo_proximity), nm(ev.score),               nm(ev.life_resource)],
    [nm(ev.threat_exposure), nm(ev.commander_progress),  nm(ev.graveyard_value)],
  ]
  const buf = new Float32Array(S * S)
  for (let y = 0; y < S; y++) {
    for (let x = 0; x < S; x++) {
      const gx = x / (S - 1) * 2, gy = y / (S - 1) * 2
      const ix = Math.min(gx | 0, 1), iy = Math.min(gy | 0, 1)
      const fx = gx - ix, fy = gy - iy
      buf[y * S + x] =
        (1 - fx) * (1 - fy) * g[iy][ix] + fx * (1 - fy) * g[iy][ix + 1] +
        (1 - fx) * fy * g[iy + 1][ix] + fx * fy * g[iy + 1][ix + 1]
    }
  }
  const img = ctx.createImageData(S, S)
  const d = img.data
  const ISO = [0.25, 0.45, 0.65, 0.85]
  for (let i = 0; i < S * S; i++) {
    const [r, gr, b] = magma(buf[i])
    d[i * 4] = r; d[i * 4 + 1] = gr; d[i * 4 + 2] = b; d[i * 4 + 3] = 210
    const x = i % S, y = i / S | 0
    if (x > 0 || y > 0) {
      for (const th of ISO) {
        if ((x > 0 && (buf[y * S + x - 1] < th) !== (buf[i] < th)) ||
            (y > 0 && (buf[(y - 1) * S + x] < th) !== (buf[i] < th))) {
          d[i * 4] = Math.min(255, r + 55)
          d[i * 4 + 1] = Math.min(255, gr + 55)
          d[i * 4 + 2] = Math.min(255, b + 55)
          d[i * 4 + 3] = 240
          break
        }
      }
    }
  }
  ctx.putImageData(img, 0, 0)
}

const typeTag = (p) => {
  if (p.is_commander) return 'CMDR'
  if (p.is_land) return 'LAND'
  if (p.type === 'CREATURE') return 'CREA'
  if (p.type === 'ARTIFACT') return 'ART'
  if (p.type === 'ENCHANTMENT') return 'ENC'
  if (p.type === 'PLANESWALKER') return 'PW'
  return p.type?.slice(0, 4)?.toUpperCase() || '???'
}

const permStat = (p) => {
  if (p.type === 'CREATURE' && (p.power != null || p.toughness != null)) {
    return `${p.power ?? '?'}/${p.toughness ?? '?'}`
  }
  return ''
}

const stackPerms = (perms) => {
  const groups = {}
  const order = []
  for (const p of perms) {
    if (p.is_commander) {
      order.push({ ...p, count: 1 })
      continue
    }
    const key = p.name || '???'
    if (!groups[key]) {
      groups[key] = { ...p, count: 1 }
      order.push(groups[key])
    } else {
      groups[key].count++
      if (p.tapped) groups[key].tapped = true
    }
  }
  return order
}

const LOG_COLORS = {
  cast: 'var(--ok)',
  land: 'var(--ink-2)',
  combat: 'var(--danger)',
  damage: 'var(--danger)',
  counter: 'var(--warn)',
  removal: 'var(--warn)',
  life: 'var(--ok)',
  draw: 'var(--ink)',
  elimination: 'var(--danger)',
  etb: 'var(--ink)',
  trigger: 'var(--ink-2)',
  activate: 'var(--ink)',
  token: 'var(--ok)',
  search: 'var(--ink-2)',
  reanimate: 'var(--warn)',
  extra_turn: 'var(--danger)',
  mill: 'var(--ink-2)',
}

export default function Spectator() {
  const navigate = useNavigate()
  const { game, elo, stats, speed, status } = useLiveSocket()
  const logContainerRef = useRef(null)
  const userScrolledRef = useRef(false)
  const heatmapRefs = useRef([])
  const [heatmapTip, setHeatmapTip] = useState(null)
  const error = status === 'disconnected' ? 'WebSocket disconnected' : null

  const handleHeatmapHover = useCallback((e, seatIdx) => {
    const canvas = heatmapRefs.current[seatIdx]
    if (!canvas) return
    const rect = canvas.getBoundingClientRect()
    const x = e.clientX - rect.left
    const y = e.clientY - rect.top
    const col = Math.min(2, (x / rect.width * 3) | 0)
    const row = Math.min(2, (y / rect.height * 3) | 0)
    const key = EVAL_GRID[row][col]
    const ev = game?.seats?.[seatIdx]?.eval
    if (!ev) return
    const val = ev[key]
    setHeatmapTip({
      label: EVAL_LABELS[key],
      value: val != null ? (val >= 0 ? '+' : '') + val.toFixed(2) : '—',
      x: e.clientX,
      y: e.clientY,
    })
  }, [game])

  const clearHeatmapTip = useCallback(() => setHeatmapTip(null), [])

  const setSpeedMultiplier = async (mult) => {
    try {
      await fetch(`${API_BASE}/api/live/speed?multiplier=${mult}`, { method: 'POST' })
    } catch {}
  }

  useEffect(() => {
    const el = logContainerRef.current
    if (!el || userScrolledRef.current) return
    el.scrollTop = 0
  }, [game?.log?.length])

  useEffect(() => {
    if (!game?.seats) return
    const urls = new Set()
    for (const s of game.seats) {
      const cu = cardArtUrl(s.commander)
      if (cu) urls.add(cu)
      for (const p of (s.battlefield || []).slice(0, 14)) {
        const pu = cardArtUrl(p.name)
        if (pu) urls.add(pu)
      }
    }
    for (const u of urls) {
      const img = new Image()
      img.src = u
    }
  }, [game?.game_id, game?.turn])

  useEffect(() => {
    if (!game?.seats) return
    game.seats.forEach((s, i) => {
      if (heatmapRefs.current[i]) drawEvalContour(heatmapRefs.current[i], s.eval)
    })
  }, [game])

  if (error && !game) {
    return (
      <>
        <Tape left="SPECTATOR / / FISHTANK" mid="OFFLINE" right="" />
        <div style={{ padding: 36, textAlign: 'center' }}>
          <div className="t-md muted" style={{ lineHeight: 1.8, textTransform: 'uppercase', letterSpacing: '0.04em' }}>
            &gt; SHOWMATCH ENGINE OFFLINE<br />
            &gt; START SERVER WITH AST CORPUS TO ENABLE FISHTANK<br />
            &gt; {error}<span className="blink">_</span>
          </div>
        </div>
      </>
    )
  }

  if (!game || game.status === 'starting') {
    return (
      <>
        <Tape left="SPECTATOR / / FISHTANK" mid="LOADING" right="" />
        <div style={{ padding: 36, textAlign: 'center' }}>
          <div className="t-md muted">&gt; LOADING FIRST SHOWMATCH<span className="blink">_</span></div>
        </div>
      </>
    )
  }

  const seats = game.seats || []
  const log = game.log || []
  const numSeats = seats.length || 4
  const rt = (t) => `R${Math.ceil(t / numSeats)}T${t}`
  const eloByCommander = {}
  for (const e of elo) {
    if (!eloByCommander[e.commander] || e.rating > eloByCommander[e.commander].rating) {
      eloByCommander[e.commander] = e
    }
  }

  return (
    <>
      {heatmapTip && (
        <div className="heatmap-tooltip" style={{ left: heatmapTip.x + 12, top: heatmapTip.y - 8 }}>
          {heatmapTip.label}: {heatmapTip.value}
        </div>
      )}
      <Tape
        left={`SPECTATOR / / FISHTANK`}
        mid={game.finished ? 'GAME OVER' : 'LIVE TELEMETRY'}
        right={`GAME ${game.game_id} / ${rt(game.turn)}`}
      />

      <div className="spectator-layout">
        {/* All 4 seats — full width, above the fold */}
        <div className="spectator-seats">
          <div className="seat-grid">
            {[0, 1, 3, 2].filter(i => i < seats.length).map(i => {
              const s = seats[i]
              const e = eloByCommander[s.commander] || {}
              const delta = e.delta || 0
              const rating = e.rating ? Math.round(e.rating) : 1500
              const perms = s.battlefield || []
              const isActive = i === game.active_seat && !game.finished
              const isWinner = game.finished && game.winner === i
              const artUrl = cardArtUrl(s.commander)

              return (
                <div key={i} className="seat-panel" style={{ borderColor: isWinner ? 'var(--ok)' : isActive ? 'var(--warn)' : undefined }}>
                  <div className="seat-hd">
                    <span className="seat-name">
                      {s.commander?.toUpperCase() || 'UNKNOWN'}
                      {isActive && <span style={{ color: 'var(--warn)' }}> ●</span>}
                      {isWinner && <span style={{ color: 'var(--ok)' }}> ★</span>}
                      {s.lost && !isWinner && <span style={{ color: 'var(--danger)' }}> ✕</span>}
                    </span>
                    <span className="seat-stats">
                      ♥{s.life} · {rating}{' '}
                      <span style={{ color: delta >= 0 ? 'var(--ok)' : 'var(--danger)', fontSize: 9 }}>
                        {delta >= 0 ? '+' : ''}{delta}
                      </span>
                    </span>
                  </div>
                  <div className="seat-body">
                    <div className="seat-art-col">
                      {artUrl && (
                        <div className="seat-art" style={{
                          backgroundImage: `url(${artUrl})`,
                          opacity: s.lost && !isWinner ? 0.3 : 0.85,
                        }} />
                      )}
                      <canvas
                        ref={el => heatmapRefs.current[i] = el}
                        className="seat-eval-map"
                        width={80}
                        height={80}
                        onMouseMove={e => handleHeatmapHover(e, i)}
                        onMouseLeave={clearHeatmapTip}
                      />
                    </div>
                    <div className="seat-perms">
                      {perms.length === 0 ? (
                        <span className="t-xs muted-2">—</span>
                      ) : (() => {
                        const stacked = stackPerms(perms)
                        return stacked.slice(0, 12).map((p, j) => (
                          <div
                            key={j}
                            title={`${p.name}${p.count > 1 ? ` ×${p.count}` : ''}`}
                            className="perm-tile"
                            style={{
                              borderColor: p.is_commander ? 'var(--warn)' : 'var(--rule-2)',
                              opacity: p.tapped ? 0.4 : 1,
                              transform: p.tapped ? 'rotate(6deg)' : 'none',
                              backgroundImage: p.name ? `url(${cardArtUrl(p.name)})` : undefined,
                            }}
                          >
                            <span className="perm-tag">{typeTag(p)}{p.count > 1 ? `×${p.count}` : ''}</span>
                            {permStat(p) && <span className="perm-stat">{permStat(p)}</span>}
                          </div>
                        ))
                      })()}
                      {(() => {
                        const stacked = stackPerms(perms)
                        return stacked.length > 12 ? (
                          <span className="t-xs muted" style={{ alignSelf: 'center', fontSize: 9 }}>+{stacked.length - 12}</span>
                        ) : null
                      })()}
                    </div>
                  </div>
                  <div className="seat-ft">
                    <span>H{s.hand_size} L{s.library_size} G{s.gy_size} B{perms.length}</span>
                    {isActive && <span style={{ color: 'var(--ok)' }}>● PRI</span>}
                  </div>
                </div>
              )
            })}
          </div>

          {/* Turn status — single compact line */}
          <div className="turn-bar">
            {game.finished ? (
              <span>
                GAME OVER — {game.end_reason?.replace(/_/g, ' ')?.toUpperCase()} — WINNER: {game.winner >= 0 ? seats[game.winner]?.commander?.toUpperCase() : 'DRAW'} — {rt(game.turn)}
                <span className="blink"> _</span>
              </span>
            ) : (
              <span>
                {rt(game.turn)} · {(game.phase || '').toUpperCase()}{game.step ? ` / ${game.step.toUpperCase()}` : ''} · {seats[game.active_seat]?.commander?.toUpperCase()} · {seats.reduce((a, s) => a + (s.battlefield?.length || 0), 0)} PERMS
                <span className="blink"> █</span>
              </span>
            )}
          </div>
        </div>

        {/* Below the fold — log + sidebar controls */}
        <div className="spectator-lower">
          <div className="spectator-lower-main">
            <Panel code="FT.LOG" title="LIVE ACTION LOG" right={<span className="t-xs muted">{log.length} EVT</span>}>
              <div ref={logContainerRef} onScroll={(e) => {
                const el = e.target
                const atTop = el.scrollTop < 30
                userScrolledRef.current = !atTop
              }} style={{ maxHeight: 280, overflow: 'auto', fontSize: 11, lineHeight: 1.6 }}>
                {log.length === 0 ? (
                  <div className="t-xs muted-2">— WAITING FOR EVENTS —</div>
                ) : (() => {
                  const currentRound = Math.ceil(game.turn / numSeats)
                  const reversed = [...log].reverse()
                  return reversed.map((entry, i) => {
                    const entryRound = Math.ceil(entry.turn / numSeats)
                    const isOldRound = entryRound < currentRound
                    return (
                      <div
                        key={i}
                        style={{
                          display: 'grid',
                          gridTemplateColumns: '50px 1fr',
                          gap: 8,
                          padding: '2px 0',
                          borderBottom: i < reversed.length - 1 ? '1px dotted var(--rule)' : 'none',
                          opacity: isOldRound ? 0.4 : 1,
                        }}
                      >
                        <span className="muted-2" style={{ fontSize: 10 }}>{rt(entry.turn)}</span>
                        <span style={{ color: LOG_COLORS[entry.kind] || 'var(--ink)', letterSpacing: '0.02em' }}>
                          &gt; {entry.action}
                        </span>
                      </div>
                    )
                  })
                })()}
              </div>
            </Panel>

            <Panel code="FT.D" title="CURRENT GAME">
              <div style={{ minHeight: 80 }}>
                {game && (
                  <div className="kv" style={{ gridTemplateColumns: 'max-content 1fr minmax(80px, 140px)' }}>
                    {[
                      ['GAME', `#${game.game_id}`],
                      ['ROUND/TURN', rt(game.turn)],
                      ['PHASE', (game.phase || '?').toUpperCase()],
                      ['ACTIVE', seats[game.active_seat]?.commander?.toUpperCase() || '—'],
                      ['ALIVE', `${seats.filter(s => !s.lost).length} / ${seats.length}`],
                    ].map(([k, v]) => (
                      <Fragment key={k}>
                        <span className="k">{k}</span>
                        <span className="dots">{'.'.repeat(60)}</span>
                        <span className="v" style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{v}</span>
                      </Fragment>
                    ))}
                  </div>
                )}
              </div>
            </Panel>
          </div>

          <div className="spectator-lower-side">
            <Panel code="FT.SPD" title="SPEED CONTROL">
              <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                <input
                  type="range"
                  min={0}
                  max={SPEED_MARKS.length - 1}
                  step={1}
                  value={SPEED_MARKS.indexOf(speed) >= 0 ? SPEED_MARKS.indexOf(speed) : 2}
                  onChange={(e) => setSpeedMultiplier(SPEED_MARKS[e.target.value])}
                  style={{ flex: 1, accentColor: 'var(--ok)' }}
                />
                <span className="t-md" style={{ fontWeight: 700, minWidth: 50, textAlign: 'right' }}>
                  {speed}×
                </span>
              </div>
              <div className="speed-marks">
                {SPEED_MARKS.map((m, i) => (
                  <span
                    key={i}
                    className="t-xs"
                    style={{ cursor: 'pointer', color: speed === m ? 'var(--ok)' : 'var(--ink-2)' }}
                    onClick={() => setSpeedMultiplier(m)}
                  >
                    {m}×
                  </span>
                ))}
              </div>
            </Panel>

            <Panel code="FT.B" title="LIVE ELO" right={<span className={`led led--on ${!game.finished ? 'blink' : ''}`} />}>
              <div style={{ minHeight: 200 }}>
                {elo.length === 0 ? (
                  <div className="t-xs muted">NO ELO DATA YET</div>
                ) : (
                  elo.slice(0, 10).map((r) => (
                    <div key={r.deck_id || r.commander} style={{ marginBottom: 8, cursor: 'pointer' }} onClick={() => {
                      if (r.owner && r.deck_id) {
                        navigate(`/decks/${r.owner}/${r.deck_id}`)
                      } else {
                        navigate(`/decks?q=${encodeURIComponent(r.commander)}`)
                      }
                    }}>
                      <div className="flex justify-between">
                        <span className="t-xs" style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 160, textDecoration: 'underline', textDecorationColor: 'var(--rule-2)' }}>
                          {r.commander?.toUpperCase() || r.deck_id?.toUpperCase()}
                        </span>
                        <span className="t-xs" style={{ color: r.delta >= 0 ? 'var(--ok)' : 'var(--danger)', whiteSpace: 'nowrap' }}>
                          {Math.round(r.rating)} ({r.delta >= 0 ? '+' : ''}{r.delta})
                        </span>
                      </div>
                      {r.owner && (
                        <div className="t-xs muted-2" style={{ fontSize: 9, marginTop: 1 }}>{r.owner?.toUpperCase()} / {r.wins}W-{r.losses}L</div>
                      )}
                      <div style={{ transition: 'width 0.3s ease' }}>
                        <Bar value={Math.max(0, (r.rating - 1300) / 4)} />
                      </div>
                    </div>
                  ))
                )}
              </div>
            </Panel>

            <Panel code="FT.C" title="SESSION STATS">
              <div style={{ minHeight: 120 }}>
                {stats ? (
                  <KV rows={[
                    ['GAMES PLAYED', `${stats.games_played}`],
                    ['AVG GAME', `${stats.avg_turns} TURNS`],
                    ['DOMINANT', (stats.dominant?.split('//')[0]?.trim() || '—').toUpperCase()],
                    ['WIN RATE', `${stats.dominant_win_rate}%`],
                    ['GAMES/MIN', `${stats.games_per_min}`],
                    ['UPTIME', stats.uptime],
                    ['STATUS', stats.status?.toUpperCase()],
                  ]} />
                ) : (
                  <div className="t-xs muted">LOADING...</div>
                )}
              </div>
            </Panel>

            <Panel code="FT.VM" title="EVAL HEATMAP KEY">
              <div className="volcmap-legend">
                <div className="volcmap-grid">
                  {EVAL_GRID.flat().map(key => (
                    <span key={key} className="volcmap-cell">{EVAL_LABELS[key]}</span>
                  ))}
                </div>
                <div className="volcmap-scale">
                  <div className="volcmap-bar" />
                  <div className="volcmap-scale-labels">
                    <span>LOW</span>
                    <span>HIGH</span>
                  </div>
                </div>
              </div>
            </Panel>
          </div>
        </div>
      </div>
    </>
  )
}
