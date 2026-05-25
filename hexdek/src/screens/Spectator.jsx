import { useState, useEffect, useLayoutEffect, useRef, useCallback, Fragment } from 'react'
import { useNavigate } from 'react-router-dom'
import { Panel, KV, Bar, Tag, Btn, Tape } from '../components/chrome'
import GlossaryTerm from '../components/GlossaryTerm'
import { cardArtUrl, API_BASE } from '../services/api'
import { useLiveSocket } from '../hooks/useLiveSocket'
import { findGameChangerInText } from '../data/gameChangers'
import CardLink, { linkifyAction } from '../components/CardLink'
import { narrate } from '../components/NarratorOverlay'
import ContextBox from '../components/ContextBox'
import ArtAmbience from '../components/ArtAmbience'

const SPEED_MARKS = [0.1, 0.2, 0.3, 0.5, 0.75, 1, 1.5, 2]

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

// Glossary id for each eval-grid cell. Lets the spectator panel wrap
// every heatmap label with a tap-to-define disclosure without bloating
// EVAL_LABELS itself (the short labels are also rendered into the
// per-seat heatmap, where a glossary disclosure would be way too much
// chrome).
const EVAL_TERMS = {
  board_presence: 'eval_board',
  card_advantage: 'eval_cards',
  mana_advantage: 'eval_mana',
  combo_proximity: 'eval_combo',
  score: 'eval_score',
  life_resource: 'eval_life',
  threat_exposure: 'eval_threat',
  commander_progress: 'eval_cmdr',
  graveyard_value: 'eval_grave',
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

function drawEvalContour(canvas, ev, lost) {
  if (!canvas) return
  const S = canvas.width
  const ctx = canvas.getContext('2d')
  if (!ev || lost) {
    ctx.fillStyle = '#0c0d0a'
    ctx.fillRect(0, 0, S, S)
    return
  }
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
  for (let i = 0; i < S * S; i++) {
    const [r, gr, b] = magma(buf[i])
    d[i * 4] = r; d[i * 4 + 1] = gr; d[i * 4 + 2] = b; d[i * 4 + 3] = 210
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
  untap: 'var(--ink-2)',
  tap: 'var(--ink-2)',
  discard: 'var(--warn)',
  scry: 'var(--ink)',
  surveil: 'var(--ink)',
  shuffle: 'var(--ink-2)',
  bounce: 'var(--warn)',
  equip: 'var(--ink)',
  monarch: 'var(--warn)',
}

const SEAT_COLORS = [
  '#6ee7b7', // seat 0 — emerald
  '#93c5fd', // seat 1 — sky
  '#fca5a5', // seat 2 — rose
  '#fcd34d', // seat 3 — amber
]

const ELIMINATION_KINDS = new Set([
  'elimination', 'sba_704_5a', 'sba_704_5b', 'sba_704_5c', 'sba_704_5d',
])

const ELIMINATION_REASONS = {
  sba_704_5a: 'LIFE ≤ 0',
  sba_704_5b: 'EMPTY LIBRARY',
  sba_704_5c: '10+ POISON',
  sba_704_5d: '21+ COMMANDER DMG',
}

function linkifyNarrated(text, source, targets) {
  if (!text) return text
  const cardNames = []
  if (source) cardNames.push(source)
  if (targets) {
    for (const t of targets) {
      if (t && !cardNames.some(c => c.toLowerCase() === t.toLowerCase())) {
        cardNames.push(t)
      }
    }
  }
  if (cardNames.length === 0) return text

  for (const card of cardNames) {
    const titleCard = card.toLowerCase().replace(/\b\w/g, c => c.toUpperCase())
    const idx = text.indexOf(titleCard)
    if (idx >= 0) {
      const before = text.slice(0, idx)
      const after = text.slice(idx + titleCard.length)
      return (
        <>
          {before}
          <CardLink name={card} style={{ color: 'inherit', borderBottom: '1px dotted currentColor' }}>
            {titleCard}
          </CardLink>
          {after}
        </>
      )
    }
    const upperCard = card.toUpperCase()
    const idxU = text.toUpperCase().indexOf(upperCard)
    if (idxU >= 0) {
      const before = text.slice(0, idxU)
      const matched = text.slice(idxU, idxU + card.length)
      const after = text.slice(idxU + card.length)
      return (
        <>
          {before}
          <CardLink name={card} style={{ color: 'inherit', borderBottom: '1px dotted currentColor' }}>
            {matched}
          </CardLink>
          {after}
        </>
      )
    }
  }
  return text
}

export default function Spectator() {
  const navigate = useNavigate()
  const { game, elo, stats, speed, status, history } = useLiveSocket()
  const logContainerRef = useRef(null)
  const userScrolledRef = useRef(false)
  // Last observed scrollHeight of the log container, captured at the
  // tail of each render's useLayoutEffect. Used to scroll-anchor the
  // user's reading position when new events prepend to the (reversed,
  // newest-first) log while they're scrolled into the past.
  const lastLogScrollHeightRef = useRef(0)
  const heatmapRefs = useRef([])
  const heatmapAnimsRef = useRef([])
  const heatmapPrevEvalRef = useRef([])
  const heatmapPrevLostRef = useRef([])
  // heatmapDrawnRef tracks the eval values most recently *painted* per seat
  // (including mid-tween interpolated frames). New tweens start from here
  // rather than heatmapPrevEvalRef so a fast-arriving WS push doesn't
  // snap the canvas back to the pre-tween state — the source of the
  // triple-jitter bug.
  const heatmapDrawnRef = useRef([])
  const [heatmapTip, setHeatmapTip] = useState(null)
  const error = status === 'disconnected' ? 'WebSocket disconnected' : null

  useEffect(() => {
    document.title = 'HEXDEK Live'
  }, [])

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
    // Edge-aware placement: the tooltip default-anchors 12px right /
    // 8px above the cursor, which clips off-screen for the
    // right-column heatmaps in a 2x2 seat grid and for any seat near
    // the bottom edge on short viewports. Flip the anchor when the
    // cursor sits near a viewport edge.
    const vw = window.innerWidth || document.documentElement.clientWidth || 0
    const vh = window.innerHeight || document.documentElement.clientHeight || 0
    const tipW = 150 // generous upper bound for "Cmdr: +0.34"
    const tipH = 24
    const margin = 8
    const flipX = e.clientX + 12 + tipW > vw - margin
    const flipY = e.clientY + tipH > vh - margin
    const left = flipX
      ? Math.max(margin, e.clientX - 12 - tipW)
      : e.clientX + 12
    const top = flipY
      ? Math.max(margin, e.clientY - 8 - tipH)
      : e.clientY - 8
    setHeatmapTip({
      label: EVAL_LABELS[key],
      value: val != null ? (val >= 0 ? '+' : '') + val.toFixed(2) : '—',
      left,
      top,
    })
  }, [game])

  const clearHeatmapTip = useCallback(() => setHeatmapTip(null), [])

  const setSpeedMultiplier = async (mult) => {
    try {
      await fetch(`${API_BASE}/api/live/speed?multiplier=${mult}`, { method: 'POST' })
    } catch {}
  }

  // Log scroll management. The log container renders newest-at-top
  // (see `[...log].reverse()` below), so each incoming event prepends
  // a row. With no compensation, that pushes whatever the user is
  // reading downward by one row each tick — visually their viewport
  // "rewinds" to an earlier entry while they sit still. useLayoutEffect
  // runs after the new rows are in the DOM but before paint, so we can
  // measure the height delta and add it to scrollTop in the same
  // frame, anchoring the user's reading position with no flicker.
  // When the user is at the top (userScrolledRef false) we just snap
  // back to 0 so they stay on the newest entry.
  useLayoutEffect(() => {
    const el = logContainerRef.current
    if (!el) return
    const newHeight = el.scrollHeight
    if (userScrolledRef.current) {
      if (lastLogScrollHeightRef.current > 0) {
        const delta = newHeight - lastLogScrollHeightRef.current
        if (delta > 0) el.scrollTop += delta
      }
    } else {
      el.scrollTop = 0
    }
    lastLogScrollHeightRef.current = newHeight
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
    const HEATMAP_KEYS = [
      'board_presence', 'card_advantage', 'mana_advantage',
      'combo_proximity', 'score', 'life_resource',
      'threat_exposure', 'commander_progress', 'graveyard_value',
    ]
    const DURATION = 500
    const easeInOutQuad = t => (t < 0.5 ? 2 * t * t : 1 - Math.pow(-2 * t + 2, 2) / 2)

    const EPS = 1e-3
    const evalsEqual = (a, b) => {
      if (a === b) return true
      if (!a || !b) return false
      for (const key of HEATMAP_KEYS) {
        if (Math.abs((a[key] ?? 0) - (b[key] ?? 0)) > EPS) return false
      }
      return true
    }

    game.seats.forEach((s, i) => {
      const canvas = heatmapRefs.current[i]
      if (!canvas) return

      const drawn = heatmapDrawnRef.current[i] || null
      const targetEval = s.eval || null
      const prevLost = !!heatmapPrevLostRef.current[i]
      const targetLost = !!s.lost

      // Lost state paints solid; skip the lerp — the GG overlay + .seat-art opacity
      // transition handle the visual fade.
      if (targetLost || !targetEval) {
        const prevAnim = heatmapAnimsRef.current[i]
        if (prevAnim) cancelAnimationFrame(prevAnim)
        heatmapAnimsRef.current[i] = null
        drawEvalContour(canvas, targetEval, targetLost)
        heatmapPrevEvalRef.current[i] = targetEval
        heatmapPrevLostRef.current[i] = targetLost
        heatmapDrawnRef.current[i] = targetEval
        return
      }

      // No-op when the eval is unchanged within epsilon — prevents the
      // WS firing every event from re-arming the tween.
      if (!prevLost && drawn && evalsEqual(drawn, targetEval) && !heatmapAnimsRef.current[i]) {
        heatmapPrevEvalRef.current[i] = targetEval
        heatmapPrevLostRef.current[i] = targetLost
        return
      }

      // First paint or coming back from lost: snap, don't morph.
      if (!drawn || prevLost) {
        const prevAnim = heatmapAnimsRef.current[i]
        if (prevAnim) cancelAnimationFrame(prevAnim)
        heatmapAnimsRef.current[i] = null
        drawEvalContour(canvas, targetEval, false)
        heatmapPrevEvalRef.current[i] = targetEval
        heatmapPrevLostRef.current[i] = targetLost
        heatmapDrawnRef.current[i] = targetEval
        return
      }

      // Mid-tween interruption: start the new tween from whatever we
      // last painted, NOT from heatmapPrevEvalRef (which is the value
      // before the in-flight animation started). This is the fix for the
      // triple-jitter: rapid WS pushes used to keep restarting from the
      // same stale source, visibly snapping the canvas back to the
      // start of the prior tween.
      const prevAnim = heatmapAnimsRef.current[i]
      if (prevAnim) cancelAnimationFrame(prevAnim)
      const sourceEval = { ...drawn }
      const start = performance.now()
      const tick = (now) => {
        const t = Math.min(1, (now - start) / DURATION)
        const k = easeInOutQuad(t)
        const interp = {}
        for (const key of HEATMAP_KEYS) {
          const a = sourceEval[key] ?? 0
          const b = targetEval[key] ?? 0
          interp[key] = a + (b - a) * k
        }
        drawEvalContour(canvas, interp, false)
        heatmapDrawnRef.current[i] = interp
        if (t < 1) {
          heatmapAnimsRef.current[i] = requestAnimationFrame(tick)
        } else {
          heatmapAnimsRef.current[i] = null
          heatmapPrevEvalRef.current[i] = targetEval
          heatmapPrevLostRef.current[i] = targetLost
          heatmapDrawnRef.current[i] = targetEval
        }
      }
      heatmapAnimsRef.current[i] = requestAnimationFrame(tick)
    })
  }, [game])

  useEffect(() => () => {
    for (const h of heatmapAnimsRef.current) if (h) cancelAnimationFrame(h)
  }, [])

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

  // Most recent Game Changer on the current turn — drives the turn-bar callout.
  let activeGameChanger = null
  for (let i = log.length - 1; i >= 0; i--) {
    const e = log[i]
    if (e.turn !== game.turn) break
    const gc = findGameChangerInText(e.action)
    if (gc) { activeGameChanger = gc; break }
  }
  const eloByCommander = {}
  for (const e of elo) {
    if (!eloByCommander[e.commander] || (e.hex_rating || 0) > (eloByCommander[e.commander].hex_rating || 0)) {
      eloByCommander[e.commander] = e
    }
  }

  return (
    <div className="spectator-page">
      {heatmapTip && (
        <div className="heatmap-tooltip" style={{ left: heatmapTip.left, top: heatmapTip.top }}>
          {heatmapTip.label}: {heatmapTip.value}
        </div>
      )}
      <Tape
        left={`SPECTATOR / / FISHTANK`}
        mid={game.finished ? 'GAME OVER' : 'LIVE TELEMETRY'}
        right={`GAME ${game.game_id} / ${rt(game.turn)}`}
      />

      <div className="spectator-layout">
        {/* Ambient blurred-art background — uses the active (or seat 0)
            commander's art_crop. Blur + brightness handled by .art-ambience.
            Crossfades between commanders so the color-identity bleed
            morphs instead of snapping when the active seat rotates. */}
        <ArtAmbience name={seats[game.active_seat]?.commander || seats[0]?.commander} />
        {/* All 4 seats — full width, above the fold */}
        <div className="spectator-seats">
          <div className="seat-grid">
            {[0, 1, 3, 2].filter(i => i < seats.length).map(i => {
              const s = seats[i]
              const e = eloByCommander[s.commander] || {}
              const delta = e.hex_delta || e.delta || 0
              const rating = e.hex_rating ? Math.round(e.hex_rating) : 1500
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
                      <span
                        className={`life-pip ${s.life <= 5 ? 'life-pip--crit' : s.life <= 10 ? 'life-pip--low' : ''}`}
                        title={s.life <= 5 ? 'Lethal range' : s.life <= 10 ? 'Low life' : `${s.life} life`}
                      >
                        ♥{s.life}
                      </span>
                      {' · '}{rating}{' '}
                      <span style={{ color: delta >= 0 ? 'var(--ok)' : 'var(--danger)', fontSize: 9 }}>
                        {delta >= 0 ? '+' : ''}{delta}
                      </span>
                    </span>
                  </div>
                  <div className="seat-body" style={{ position: 'relative' }}>
                    {s.lost && !isWinner && (
                      <div style={{
                        position: 'absolute', inset: 0, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center',
                        zIndex: 2, pointerEvents: 'none',
                      }}>
                        <span style={{
                          fontSize: 28, fontWeight: 900, letterSpacing: '0.15em', color: 'var(--danger)',
                          opacity: 0.7, textShadow: '0 0 12px rgba(0,0,0,0.8)',
                        }}>GG</span>
                        {s.loss_reason && (
                          <span style={{
                            fontSize: 9, fontWeight: 600, color: 'var(--danger)',
                            opacity: 0.6, marginTop: 2, textTransform: 'uppercase', letterSpacing: '0.05em',
                          }}>{s.loss_reason.replace(/\s*\(CR.*\)/, '')}</span>
                        )}
                      </div>
                    )}
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
                            data-stack={p.count >= 6 ? '3' : p.count >= 3 ? '2' : p.count >= 2 ? '1' : undefined}
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
          <div className={`turn-bar${activeGameChanger ? ' gc-card' : ''}`}>
            {activeGameChanger && (
              <span className="gc-pill turn-bar-gc-pill" title={`Game Changer: ${activeGameChanger}`}>
                ★ GAME CHANGER · {activeGameChanger.toUpperCase()}
              </span>
            )}
            {game.finished ? (
              <>
                <span className="turn-bar-left">
                  GAME OVER — {game.end_reason?.replace(/_/g, ' ')?.toUpperCase()} — WINNER: {game.winner >= 0 ? seats[game.winner]?.commander?.toUpperCase() : 'DRAW'}
                </span>
                <span className="turn-bar-right" style={{ display: 'flex', gap: 10, alignItems: 'center' }}>
                  <span>{rt(game.turn)}</span>
                  {game.game_id != null && (
                    <Btn sm ghost arrow="↗" onClick={() => navigate(`/report/${game.game_id}`)}>VIEW REPORT</Btn>
                  )}
                </span>
              </>
            ) : (
              <>
                <span className="turn-bar-left">
                  {rt(game.turn)} · {(game.phase || '').toUpperCase()}{game.step ? ` / ${game.step.toUpperCase()}` : ''} · {seats[game.active_seat]?.commander?.toUpperCase()}
                </span>
                <span className="turn-bar-right" style={{ display: 'flex', gap: 10, alignItems: 'center' }}>
                  <span>{seats.reduce((a, s) => a + (s.battlefield?.length || 0), 0)} PERMS</span>
                </span>
              </>
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
              }} style={{ maxHeight: 480, overflow: 'auto', fontSize: 11, lineHeight: 1.6 }}>
                {log.length === 0 ? (
                  <div className="t-xs muted-2">— WAITING FOR EVENTS —</div>
                ) : (() => {
                  const currentRound = Math.ceil(game.turn / numSeats)
                  const reversed = [...log].reverse().slice(0, 100)
                  let lastTurn = -1
                  return reversed.map((entry, i) => {
                    const entryRound = Math.ceil(entry.turn / numSeats)
                    const isOldRound = entryRound < currentRound
                    const isElim = ELIMINATION_KINDS.has(entry.kind)
                    const elimReason = ELIMINATION_REASONS[entry.kind]
                    const gc = findGameChangerInText(entry.action)
                    const narrated = narrate(entry, seats)
                    const seatColor = SEAT_COLORS[entry.seat] || 'var(--ink-2)'
                    const showTurnHeader = entry.turn !== lastTurn
                    lastTurn = entry.turn
                    const rowClasses = [
                      isElim ? 'log-elimination' : null,
                      gc ? 'gc-card' : null,
                    ].filter(Boolean).join(' ') || undefined
                    return (
                      <Fragment key={i}>
                        {showTurnHeader && (
                          <div style={{
                            fontSize: 9,
                            color: 'var(--ink-3)',
                            padding: '4px 0 2px',
                            borderTop: i > 0 ? '1px solid var(--rule-2)' : 'none',
                            letterSpacing: '0.08em',
                            fontWeight: 600,
                          }}>
                            ── {rt(entry.turn)} ──
                          </div>
                        )}
                        <div
                          className={rowClasses}
                          style={{
                            display: 'grid',
                            gridTemplateColumns: '4px 1fr',
                            gap: 8,
                            padding: isElim || gc ? '4px 6px' : '2px 4px',
                            opacity: isOldRound && !isElim && !gc ? 0.5 : 1,
                            borderLeft: `3px solid ${seatColor}`,
                            marginBottom: 1,
                          }}
                        >
                          <span />
                          <div>
                            {narrated ? (
                              <span style={{
                                color: narrated.tone === 'combat' ? 'var(--danger)'
                                  : narrated.tone === 'elim' ? 'var(--danger)'
                                  : narrated.tone === 'changer' ? 'var(--warn)'
                                  : 'var(--ink)',
                                letterSpacing: '0.01em',
                              }}>
                                {gc && <span className="gc-pill" title="Game Changer">★ GC</span>}
                                {entry.count > 1 && <span style={{ background: 'var(--ink-3)', color: 'var(--bg)', borderRadius: 3, padding: '0 4px', fontSize: 9, marginRight: 4, fontWeight: 700 }}>×{entry.count}</span>}
                                {linkifyNarrated(narrated.text, entry.source, entry.targets)}
                              </span>
                            ) : isElim ? (
                              <span style={{
                                color: 'var(--danger)',
                                letterSpacing: '0.04em',
                                fontWeight: 700,
                                fontSize: 12,
                              }}>
                                &gt;&gt;&gt; {entry.action}{elimReason ? ` [${elimReason}]` : ''}
                              </span>
                            ) : (
                              <span style={{ color: LOG_COLORS[entry.kind] || 'var(--ink)', letterSpacing: '0.02em' }}>
                                {gc && <span className="gc-pill" title="Game Changer">★ GC</span>}
                                {entry.count > 1 && <span style={{ background: 'var(--ink-3)', color: 'var(--bg)', borderRadius: 3, padding: '0 4px', fontSize: 9, marginRight: 4, fontWeight: 700 }}>×{entry.count}</span>}
                                {(() => {
                                  const { prefix, cardName } = linkifyAction(entry.action)
                                  if (!cardName) return entry.action
                                  return (
                                    <>
                                      {prefix}
                                      <CardLink name={cardName} style={{ color: 'inherit', borderBottom: '1px dotted currentColor' }}>
                                        {cardName}
                                      </CardLink>
                                    </>
                                  )
                                })()}
                              </span>
                            )}
                          </div>
                        </div>
                      </Fragment>
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
            <Panel code="FT.VM" title="EVAL HEATMAP KEY">
              <div className="volcmap-legend">
                <div className="volcmap-grid">
                  {EVAL_GRID.flat().map(key => (
                    <span key={key} className="volcmap-cell">
                      <GlossaryTerm term={EVAL_TERMS[key]} compact>
                        {EVAL_LABELS[key]}
                      </GlossaryTerm>
                    </span>
                  ))}
                </div>
                <div className="volcmap-scale">
                  <div className="volcmap-bar" />
                  <div className="volcmap-scale-labels">
                    <span>LOW</span>
                    <span>HIGH</span>
                  </div>
                </div>
                {game.seats?.some(s => s.eval?.archetype) && (
                  <div className="volcmap-archetypes">
                    {game.seats.filter(s => !s.lost && s.eval).map((s, i) => (
                      <div key={i} className="volcmap-archetype-row">
                        <span className="t-xs" style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 80 }}>
                          {s.commander?.split('//')[0]?.trim().split(' ').pop()?.toUpperCase()}
                        </span>
                        <Tag>{s.eval.archetype?.toUpperCase() || '?'}</Tag>
                        {s.eval.budget > 0 && (
                          <span className="t-xs muted-2">
                            <GlossaryTerm term="budget" compact>
                              ⚡{s.eval.budget_used}/{s.eval.budget}
                            </GlossaryTerm>
                          </span>
                        )}
                      </div>
                    ))}
                  </div>
                )}
                <div className="hr" style={{ margin: '8px 0' }} />
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '4px 12px' }}>
                  <span className="t-xs"><span style={{ color: 'var(--ink)' }}>♥</span> <span className="muted">LIFE TOTAL</span></span>
                  <span className="t-xs"><span style={{ color: 'var(--ink)' }}>H</span> <span className="muted">HAND SIZE</span></span>
                  <span className="t-xs"><span style={{ color: 'var(--ink)' }}>L</span> <span className="muted">LIBRARY SIZE</span></span>
                  <span className="t-xs"><span style={{ color: 'var(--ink)' }}>G</span> <span className="muted">GRAVEYARD</span></span>
                  <span className="t-xs"><span style={{ color: 'var(--ink)' }}>B</span> <span className="muted">BATTLEFIELD</span></span>
                  <span className="t-xs"><span style={{ color: 'var(--ink)' }}>R</span> <span className="muted">ROUND #</span></span>
                  <span className="t-xs"><span style={{ color: 'var(--ink)' }}>T</span> <span className="muted">TURN #</span></span>
                  <span className="t-xs"><span style={{ color: 'var(--ok)' }}>●</span> <span className="muted">ACTIVE PLAYER</span></span>
                  <span className="t-xs"><span style={{ color: 'var(--ok)' }}>★</span> <span className="muted">WINNER</span></span>
                  <span className="t-xs"><span style={{ color: 'var(--danger)' }}>✕</span> <span className="muted">ELIMINATED</span></span>
                  <span className="t-xs"><span style={{ color: 'var(--ok)' }}>+N</span> <span className="muted">ELO GAINED</span></span>
                  <span className="t-xs"><span style={{ color: 'var(--danger)' }}>-N</span> <span className="muted">ELO LOST</span></span>
                </div>
              </div>
            </Panel>

            <Panel code="FT.SPD" title="SPEED CONTROL">
              <ContextBox id="spectator.speed" compact>Adjusts how fast the live game ticks. Drag the slider or click a preset — 0.1× is slow study mode, 2× is fast forward.</ContextBox>
              <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                {(() => {
                  const idx = SPEED_MARKS.indexOf(speed) >= 0 ? SPEED_MARKS.indexOf(speed) : 2
                  const pct = (idx / (SPEED_MARKS.length - 1)) * 100
                  return (
                    <input
                      type="range"
                      className="slider"
                      min={0}
                      max={SPEED_MARKS.length - 1}
                      step={1}
                      value={idx}
                      onChange={(e) => setSpeedMultiplier(SPEED_MARKS[e.target.value])}
                      aria-label="Playback speed"
                      aria-valuetext={`${parseFloat(speed.toFixed(2))}× playback`}
                      style={{
                        flex: 1,
                        background: `linear-gradient(to right, var(--ok) ${pct}%, var(--rule-2) ${pct}%)`,
                      }}
                    />
                  )
                })()}
                <span className="t-md" style={{ fontWeight: 700, minWidth: 50, textAlign: 'right' }}>
                  {parseFloat(speed.toFixed(2))}×
                </span>
              </div>
              <div className="speed-marks" role="group" aria-label="Speed presets">
                {SPEED_MARKS.map((m, i) => {
                  const active = Math.abs(speed - m) < 0.01
                  return (
                    <button
                      type="button"
                      key={i}
                      className="t-xs speed-mark-btn"
                      onClick={() => setSpeedMultiplier(m)}
                      aria-label={`Set playback speed to ${m}×`}
                      aria-pressed={active}
                      style={{ color: active ? 'var(--ok)' : 'var(--ink-2)' }}
                    >
                      {m}×
                    </button>
                  )
                })}
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
                        <span className="t-xs" style={{ color: (r.hex_delta || r.delta) >= 0 ? 'var(--ok)' : 'var(--danger)', whiteSpace: 'nowrap' }}>
                          {Math.round(r.hex_rating || r.rating || 1500)} ({(r.hex_delta || r.delta) >= 0 ? '+' : ''}{Math.round(r.hex_delta || r.delta || 0)})
                        </span>
                      </div>
                      {r.owner && (
                        <div className="t-xs muted-2" style={{ fontSize: 9, marginTop: 1 }}>{r.owner?.toUpperCase()} / {r.wins}W-{r.losses}L</div>
                      )}
                      <div style={{ transition: 'width 0.3s ease' }}>
                        <Bar value={Math.max(0, ((r.hex_rating || r.rating || 1500) - 1300) / 4)} />
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
                    ['GAMES PLAYED', `${stats.games_played}`, 'games'],
                    ['AVG GAME', `${stats.avg_turns} TURNS`],
                    ['DOMINANT', (stats.dominant?.split('//')[0]?.trim() || '—').toUpperCase()],
                    ['WIN RATE', `${stats.dominant_win_rate}%`, 'win_rate'],
                    ['GAMES/MIN', `${stats.games_per_min}`],
                    ['UPTIME', stats.uptime],
                    ['STATUS', stats.status?.toUpperCase()],
                  ]} />
                ) : (
                  <div className="t-xs muted">LOADING...</div>
                )}
              </div>
            </Panel>

            <Panel code="FT.D" title="RECENT GAMES" right={<span className="t-xs muted">{history.length}</span>}>
              <div style={{ maxHeight: 340, overflow: 'auto' }}>
                {history.length === 0 ? (
                  <div className="t-xs muted">NO GAME HISTORY YET</div>
                ) : (
                  history.slice(0, 20).map((g) => (
                    <div
                      key={g.game_id}
                      style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '4px 0', borderBottom: '1px solid var(--rule)' }}
                    >
                      <div style={{ flex: 1, minWidth: 0 }}>
                        <div className="t-xs" style={{ fontWeight: 600, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                          {(g.winner_name || g.commanders?.[g.winner] || '???').split('//')[0].trim().toUpperCase()}
                        </div>
                        <div className="t-xs muted-2" style={{ fontSize: 8 }}>
                          {g.turns}T · {g.end_reason?.replace(/_/g, ' ')?.toUpperCase() || 'UNKNOWN'}
                        </div>
                      </div>
                      <span
                        className="t-xs"
                        style={{ color: 'var(--accent)', cursor: 'pointer', whiteSpace: 'nowrap', marginLeft: 8, textDecoration: 'underline', textDecorationColor: 'var(--rule-2)' }}
                        onClick={() => navigate(`/report/${g.game_id}`)}
                      >
                        REPORT →
                      </span>
                    </div>
                  ))
                )}
              </div>
            </Panel>


          </div>
        </div>
      </div>
      {/* narrator fused into action log — no separate overlay */}
    </div>
  )
}
