import { useState, useEffect, useRef, useCallback, Fragment } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { Panel, KV, Bar, Tag, Btn, Tape } from '../components/chrome'
import { cardArtUrl, api } from '../services/api'
import { useSpectateRoom } from '../hooks/useSpectateRoom'
import { findGameChangerInText } from '../data/gameChangers'
import CardLink, { linkifyAction } from '../components/CardLink'
import { narrate } from '../components/NarratorOverlay'
import ArtAmbience from '../components/ArtAmbience'

const SPEED_MARKS = [0.1, 0.2, 0.3, 0.5, 0.75, 1, 1.5, 2]

const EVAL_GRID = [
  ['board_presence',  'card_advantage',      'mana_advantage'],
  ['combo_proximity', 'score',               'life_resource'],
  ['threat_exposure', 'commander_progress',  'graveyard_value'],
]

const EVAL_LABELS = {
  board_presence: 'Board', card_advantage: 'Cards', mana_advantage: 'Mana',
  combo_proximity: 'Combo', score: 'Score', life_resource: 'Life',
  threat_exposure: 'Threat', commander_progress: 'Cmdr', graveyard_value: 'Grave',
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
    if (p.is_commander) { order.push({ ...p, count: 1 }); continue }
    const key = p.name || '???'
    if (!groups[key]) { groups[key] = { ...p, count: 1 }; order.push(groups[key]) }
    else { groups[key].count++; if (p.tapped) groups[key].tapped = true }
  }
  return order
}

const LOG_COLORS = {
  cast: 'var(--ok)', land: 'var(--ink-2)', combat: 'var(--danger)', damage: 'var(--danger)',
  counter: 'var(--warn)', removal: 'var(--warn)', life: 'var(--ok)', draw: 'var(--ink)',
  elimination: 'var(--danger)', etb: 'var(--ink)', trigger: 'var(--ink-2)',
  activate: 'var(--ink)', token: 'var(--ok)', search: 'var(--ink-2)',
  reanimate: 'var(--warn)', extra_turn: 'var(--danger)', mill: 'var(--ink-2)',
  untap: 'var(--ink-2)', tap: 'var(--ink-2)', discard: 'var(--warn)',
  scry: 'var(--ink)', surveil: 'var(--ink)', shuffle: 'var(--ink-2)',
  bounce: 'var(--warn)', equip: 'var(--ink)', monarch: 'var(--warn)',
}

const SEAT_COLORS = ['#6ee7b7', '#93c5fd', '#fca5a5', '#fcd34d']

const ELIMINATION_KINDS = new Set([
  'elimination', 'sba_704_5a', 'sba_704_5b', 'sba_704_5c', 'sba_704_5d',
])
const ELIMINATION_REASONS = {
  sba_704_5a: 'LIFE <= 0', sba_704_5b: 'EMPTY LIBRARY',
  sba_704_5c: '10+ POISON', sba_704_5d: '21+ COMMANDER DMG',
}

function linkifyNarrated(text, source, targets) {
  if (!text) return text
  const cardNames = []
  if (source) cardNames.push(source)
  if (targets) for (const t of targets) if (t && !cardNames.some(c => c.toLowerCase() === t.toLowerCase())) cardNames.push(t)
  if (cardNames.length === 0) return text
  for (const card of cardNames) {
    const titleCard = card.toLowerCase().replace(/\b\w/g, c => c.toUpperCase())
    const idx = text.indexOf(titleCard)
    if (idx >= 0) {
      return <>{text.slice(0, idx)}<CardLink name={card} style={{ color: 'inherit', borderBottom: '1px dotted currentColor' }}>{titleCard}</CardLink>{text.slice(idx + titleCard.length)}</>
    }
    const idxU = text.toUpperCase().indexOf(card.toUpperCase())
    if (idxU >= 0) {
      return <>{text.slice(0, idxU)}<CardLink name={card} style={{ color: 'inherit', borderBottom: '1px dotted currentColor' }}>{text.slice(idxU, idxU + card.length)}</CardLink>{text.slice(idxU + card.length)}</>
    }
  }
  return text
}

export default function SpectateRoom() {
  const { roomId } = useParams()
  const navigate = useNavigate()
  const { game, elo, speed, viewers, roomInfo, status, sendSpeed } = useSpectateRoom(roomId)
  const logContainerRef = useRef(null)
  const userScrolledRef = useRef(false)
  const heatmapRefs = useRef([])
  const heatmapAnimsRef = useRef([])
  const heatmapDrawnRef = useRef([])
  const [heatmapTip, setHeatmapTip] = useState(null)

  useEffect(() => {
    document.title = roomInfo
      ? `Spectating ${roomInfo.commander?.toUpperCase() || 'UNKNOWN'}`
      : 'HEXDEK Spectate Room'
  }, [roomInfo])

  const handleHeatmapHover = useCallback((e, seatIdx) => {
    const canvas = heatmapRefs.current[seatIdx]
    if (!canvas) return
    const rect = canvas.getBoundingClientRect()
    const col = Math.min(2, ((e.clientX - rect.left) / rect.width * 3) | 0)
    const row = Math.min(2, ((e.clientY - rect.top) / rect.height * 3) | 0)
    const key = EVAL_GRID[row][col]
    const ev = game?.seats?.[seatIdx]?.eval
    if (!ev) return
    const val = ev[key]
    setHeatmapTip({ label: EVAL_LABELS[key], value: val != null ? (val >= 0 ? '+' : '') + val.toFixed(2) : '—', x: e.clientX, y: e.clientY })
  }, [game])

  const clearHeatmapTip = useCallback(() => setHeatmapTip(null), [])

  useEffect(() => {
    const el = logContainerRef.current
    if (!el || userScrolledRef.current) return
    requestAnimationFrame(() => { el.scrollTop = 0 })
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
    for (const u of urls) { const img = new Image(); img.src = u }
  }, [game?.game_id, game?.turn])

  // Heatmap tween
  useEffect(() => {
    if (!game?.seats) return
    const HEATMAP_KEYS = [
      'board_presence', 'card_advantage', 'mana_advantage',
      'combo_proximity', 'score', 'life_resource',
      'threat_exposure', 'commander_progress', 'graveyard_value',
    ]
    const DURATION = 500
    const ease = t => (t < 0.5 ? 2 * t * t : 1 - Math.pow(-2 * t + 2, 2) / 2)
    const EPS = 1e-3
    const equal = (a, b) => {
      if (a === b) return true; if (!a || !b) return false
      for (const k of HEATMAP_KEYS) if (Math.abs((a[k] ?? 0) - (b[k] ?? 0)) > EPS) return false
      return true
    }

    game.seats.forEach((s, i) => {
      const canvas = heatmapRefs.current[i]
      if (!canvas) return
      const drawn = heatmapDrawnRef.current[i] || null
      const targetEval = s.eval || null
      if (s.lost || !targetEval) {
        if (heatmapAnimsRef.current[i]) cancelAnimationFrame(heatmapAnimsRef.current[i])
        heatmapAnimsRef.current[i] = null
        drawEvalContour(canvas, targetEval, s.lost)
        heatmapDrawnRef.current[i] = targetEval
        return
      }
      if (drawn && equal(drawn, targetEval) && !heatmapAnimsRef.current[i]) return
      if (!drawn) {
        if (heatmapAnimsRef.current[i]) cancelAnimationFrame(heatmapAnimsRef.current[i])
        drawEvalContour(canvas, targetEval, false)
        heatmapDrawnRef.current[i] = targetEval
        return
      }
      if (heatmapAnimsRef.current[i]) cancelAnimationFrame(heatmapAnimsRef.current[i])
      const src = { ...drawn }
      const t0 = performance.now()
      const tick = (now) => {
        const t = Math.min(1, (now - t0) / DURATION)
        const k = ease(t)
        const interp = {}
        for (const key of HEATMAP_KEYS) interp[key] = (src[key] ?? 0) + ((targetEval[key] ?? 0) - (src[key] ?? 0)) * k
        drawEvalContour(canvas, interp, false)
        heatmapDrawnRef.current[i] = interp
        if (t < 1) heatmapAnimsRef.current[i] = requestAnimationFrame(tick)
        else { heatmapAnimsRef.current[i] = null; heatmapDrawnRef.current[i] = targetEval }
      }
      heatmapAnimsRef.current[i] = requestAnimationFrame(tick)
    })
  }, [game])

  useEffect(() => () => {
    for (const h of heatmapAnimsRef.current) if (h) cancelAnimationFrame(h)
  }, [])

  if (status === 'disconnected' && !game) {
    return (
      <>
        <Tape left="SPECTATE ROOM" mid="CONNECTING" right={roomId || ''} />
        <div style={{ padding: 36, textAlign: 'center' }}>
          <div className="t-md muted" style={{ lineHeight: 1.8, textTransform: 'uppercase', letterSpacing: '0.04em' }}>
            &gt; CONNECTING TO SPECTATE ROOM<span className="blink">_</span>
          </div>
        </div>
      </>
    )
  }

  if (!game) {
    return (
      <>
        <Tape left="SPECTATE ROOM" mid="LOADING" right={roomId || ''} />
        <div style={{ padding: 36, textAlign: 'center' }}>
          <div className="t-md muted">&gt; WAITING FOR FIRST GAME<span className="blink">_</span></div>
        </div>
      </>
    )
  }

  const seats = game.seats || []
  const log = game.log || []
  const numSeats = seats.length || 4
  const rt = (t) => `R${Math.ceil(t / numSeats)}T${t}`

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

  const targetCommander = roomInfo?.commander || seats[0]?.commander || 'UNKNOWN'
  const targetDeckKey = roomInfo?.deck_key || ''
  const [targetOwner, targetDeckId] = targetDeckKey.includes('/') ? targetDeckKey.split('/') : ['', '']

  return (
    <>
      {heatmapTip && (
        <div className="heatmap-tooltip" style={{ left: heatmapTip.x + 12, top: heatmapTip.y - 8 }}>
          {heatmapTip.label}: {heatmapTip.value}
        </div>
      )}
      <Tape
        left="SPECTATE ROOM"
        mid={game.finished ? 'GAME OVER' : 'LIVE'}
        right={`GAME ${roomInfo?.game_number || game.game_id || '?'} / ${rt(game.turn)}`}
      />

      {/* Room indicator bar */}
      <div style={{
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        padding: '8px 16px', background: 'var(--bg-2)', borderBottom: '1px solid var(--rule)',
        flexWrap: 'wrap', gap: 8,
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <Tag solid kind="ok">SPECTATING</Tag>
          <span
            className="t-md"
            style={{ fontWeight: 700, cursor: targetOwner ? 'pointer' : 'default', textDecoration: targetOwner ? 'underline' : 'none', textDecorationColor: 'var(--rule-2)' }}
            onClick={() => targetOwner && targetDeckId && navigate(`/decks/${targetOwner}/${targetDeckId}`)}
          >
            {targetCommander.toUpperCase()}
          </span>
          {(() => {
            const e = eloByCommander[targetCommander] || {}
            const r = e.hex_rating ? Math.round(e.hex_rating) : null
            return r ? <span className="t-xs muted">ELO {r}</span> : null
          })()}
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <span className="t-xs muted">{viewers} VIEWER{viewers !== 1 ? 'S' : ''}</span>
          <span className="t-xs muted">GAME #{roomInfo?.game_number || '?'}</span>
          <span className={`led led--on ${status === 'live' && !game.finished ? 'blink' : ''}`} />
        </div>
      </div>

      <div className="spectator-layout">
        <ArtAmbience name={seats[game.active_seat]?.commander || seats[0]?.commander} />

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
              const isTarget = i === 0
              const artUrl = cardArtUrl(s.commander)

              return (
                <div key={i} className="seat-panel" style={{
                  borderColor: isWinner ? 'var(--ok)' : isActive ? 'var(--warn)' : isTarget ? 'var(--accent)' : undefined,
                  borderWidth: isTarget ? 2 : undefined,
                }}>
                  <div className="seat-hd">
                    <span className="seat-name">
                      {isTarget && <span style={{ color: 'var(--accent)', fontSize: 9, marginRight: 4 }}>TARGET</span>}
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
                  <div className="seat-body" style={{ position: 'relative' }}>
                    {s.lost && !isWinner && (
                      <div style={{
                        position: 'absolute', inset: 0, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center',
                        zIndex: 2, pointerEvents: 'none',
                      }}>
                        <span style={{ fontSize: 28, fontWeight: 900, letterSpacing: '0.15em', color: 'var(--danger)', opacity: 0.7, textShadow: '0 0 12px rgba(0,0,0,0.8)' }}>GG</span>
                        {s.loss_reason && (
                          <span style={{ fontSize: 9, fontWeight: 600, color: 'var(--danger)', opacity: 0.6, marginTop: 2, textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                            {s.loss_reason.replace(/\s*\(CR.*\)/, '')}
                          </span>
                        )}
                      </div>
                    )}
                    <div className="seat-art-col">
                      {artUrl && <div className="seat-art" style={{ backgroundImage: `url(${artUrl})`, opacity: s.lost && !isWinner ? 0.3 : 0.85 }} />}
                      <canvas ref={el => heatmapRefs.current[i] = el} className="seat-eval-map" width={80} height={80}
                        onMouseMove={e => handleHeatmapHover(e, i)} onMouseLeave={clearHeatmapTip} />
                    </div>
                    <div className="seat-perms">
                      {perms.length === 0 ? <span className="t-xs muted-2">—</span> : (() => {
                        const stacked = stackPerms(perms)
                        return stacked.slice(0, 12).map((p, j) => (
                          <div key={j} title={`${p.name}${p.count > 1 ? ` ×${p.count}` : ''}`} className="perm-tile"
                            data-stack={p.count >= 6 ? '3' : p.count >= 3 ? '2' : p.count >= 2 ? '1' : undefined}
                            style={{
                              borderColor: p.is_commander ? 'var(--warn)' : 'var(--rule-2)',
                              opacity: p.tapped ? 0.4 : 1, transform: p.tapped ? 'rotate(6deg)' : 'none',
                              backgroundImage: p.name ? `url(${cardArtUrl(p.name)})` : undefined,
                            }}>
                            <span className="perm-tag">{typeTag(p)}{p.count > 1 ? `×${p.count}` : ''}</span>
                            {permStat(p) && <span className="perm-stat">{permStat(p)}</span>}
                          </div>
                        ))
                      })()}
                      {(() => { const stacked = stackPerms(perms); return stacked.length > 12 ? <span className="t-xs muted" style={{ alignSelf: 'center', fontSize: 9 }}>+{stacked.length - 12}</span> : null })()}
                    </div>
                  </div>
                  <div className="seat-ft">
                    <span>
                      H{s.hand_size}{' '}
                      <span
                        className={s.library_size <= 3 ? 'lib-pip--crit' : s.library_size <= 7 ? 'lib-pip--low' : ''}
                        title={s.library_size <= 3 ? 'Mill danger' : s.library_size <= 7 ? 'Low library' : `${s.library_size} cards left`}
                      >L{s.library_size}</span>{' '}
                      G{s.gy_size} B{perms.length}
                    </span>
                    {isActive && <span style={{ color: 'var(--ok)' }}>● PRI</span>}
                  </div>
                </div>
              )
            })}
          </div>

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
                  {game.game_id != null && <Btn sm ghost arrow="↗" onClick={() => navigate(`/report/${game.game_id}`)}>VIEW REPORT</Btn>}
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

        <div className="spectator-lower">
          <div className="spectator-lower-main">
            <Panel code="SR.LOG" title="LIVE ACTION LOG" right={<span className="t-xs muted">{log.length} EVT</span>}>
              <div ref={logContainerRef} onScroll={(e) => { userScrolledRef.current = e.target.scrollTop >= 30 }}
                style={{ maxHeight: 480, overflow: 'auto', fontSize: 11, lineHeight: 1.6 }}>
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
                    const narrated2 = narrate(entry, seats)
                    const seatColor = SEAT_COLORS[entry.seat] || 'var(--ink-2)'
                    const showTurnHeader = entry.turn !== lastTurn
                    lastTurn = entry.turn
                    return (
                      <Fragment key={i}>
                        {showTurnHeader && (
                          <div style={{ fontSize: 9, color: 'var(--ink-3)', padding: '4px 0 2px', borderTop: i > 0 ? '1px solid var(--rule-2)' : 'none', letterSpacing: '0.08em', fontWeight: 600 }}>
                            ── {rt(entry.turn)} ──
                          </div>
                        )}
                        <div className={[isElim ? 'log-elimination' : null, gc ? 'gc-card' : null].filter(Boolean).join(' ') || undefined}
                          style={{
                            display: 'grid', gridTemplateColumns: '4px 1fr', gap: 8,
                            padding: isElim || gc ? '4px 6px' : '2px 4px',
                            opacity: isOldRound && !isElim && !gc ? 0.5 : 1,
                            borderLeft: `3px solid ${seatColor}`, marginBottom: 1,
                          }}>
                          <span />
                          <div>
                            {narrated2 ? (
                              <span style={{ color: narrated2.tone === 'combat' || narrated2.tone === 'elim' ? 'var(--danger)' : narrated2.tone === 'changer' ? 'var(--warn)' : 'var(--ink)', letterSpacing: '0.01em' }}>
                                {gc && <span className="gc-pill" title="Game Changer">★ GC</span>}
                                {entry.count > 1 && <span style={{ background: 'var(--ink-3)', color: 'var(--bg)', borderRadius: 3, padding: '0 4px', fontSize: 9, marginRight: 4, fontWeight: 700 }}>×{entry.count}</span>}
                                {linkifyNarrated(narrated2.text, entry.source, entry.targets)}
                              </span>
                            ) : isElim ? (
                              <span style={{ color: 'var(--danger)', letterSpacing: '0.04em', fontWeight: 700, fontSize: 12 }}>
                                &gt;&gt;&gt; {entry.action}{elimReason ? ` [${elimReason}]` : ''}
                              </span>
                            ) : (
                              <span style={{ color: LOG_COLORS[entry.kind] || 'var(--ink)', letterSpacing: '0.02em' }}>
                                {gc && <span className="gc-pill" title="Game Changer">★ GC</span>}
                                {entry.count > 1 && <span style={{ background: 'var(--ink-3)', color: 'var(--bg)', borderRadius: 3, padding: '0 4px', fontSize: 9, marginRight: 4, fontWeight: 700 }}>×{entry.count}</span>}
                                {(() => {
                                  const { prefix, cardName } = linkifyAction(entry.action)
                                  if (!cardName) return entry.action
                                  return <>{prefix}<CardLink name={cardName} style={{ color: 'inherit', borderBottom: '1px dotted currentColor' }}>{cardName}</CardLink></>
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
          </div>

          <div className="spectator-lower-side">
            <Panel code="SR.SPD" title="SPEED CONTROL">
              <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                {(() => {
                  const idx = SPEED_MARKS.indexOf(speed) >= 0 ? SPEED_MARKS.indexOf(speed) : 5
                  const pct = (idx / (SPEED_MARKS.length - 1)) * 100
                  return (
                    <input type="range" className="slider" min={0} max={SPEED_MARKS.length - 1} step={1} value={idx}
                      onChange={(e) => sendSpeed(SPEED_MARKS[e.target.value])}
                      style={{ flex: 1, background: `linear-gradient(to right, var(--ok) ${pct}%, var(--rule-2) ${pct}%)` }} />
                  )
                })()}
                <span className="t-md" style={{ fontWeight: 700, minWidth: 50, textAlign: 'right' }}>
                  {parseFloat(speed.toFixed(2))}×
                </span>
              </div>
              <div className="speed-marks">
                {SPEED_MARKS.map((m, i) => (
                  <span key={i} className="t-xs" style={{ cursor: 'pointer', color: Math.abs(speed - m) < 0.01 ? 'var(--ok)' : 'var(--ink-2)' }}
                    onClick={() => sendSpeed(m)}>{m}×</span>
                ))}
              </div>
            </Panel>

            <Panel code="SR.ELO" title="LIVE ELO" right={<span className={`led led--on ${!game.finished ? 'blink' : ''}`} />}>
              <div style={{ minHeight: 140 }}>
                {elo.length === 0 ? <div className="t-xs muted">NO ELO DATA YET</div> : (
                  elo.slice(0, 8).map((r) => (
                    <div key={r.deck_id || r.commander} style={{ marginBottom: 8, cursor: 'pointer' }} onClick={() => {
                      if (r.owner && r.deck_id) navigate(`/decks/${r.owner}/${r.deck_id}`)
                      else navigate(`/decks?q=${encodeURIComponent(r.commander)}`)
                    }}>
                      <div className="flex justify-between">
                        <span className="t-xs" style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 160, textDecoration: 'underline', textDecorationColor: 'var(--rule-2)' }}>
                          {r.commander?.toUpperCase() || r.deck_id?.toUpperCase()}
                        </span>
                        <span className="t-xs" style={{ color: (r.hex_delta || r.delta) >= 0 ? 'var(--ok)' : 'var(--danger)', whiteSpace: 'nowrap' }}>
                          {Math.round(r.hex_rating || r.rating || 1500)} ({(r.hex_delta || r.delta) >= 0 ? '+' : ''}{Math.round(r.hex_delta || r.delta || 0)})
                        </span>
                      </div>
                      <div style={{ transition: 'width 0.3s ease' }}>
                        <Bar value={Math.max(0, ((r.hex_rating || r.rating || 1500) - 1300) / 4)} />
                      </div>
                    </div>
                  ))
                )}
              </div>
            </Panel>

            <Panel code="SR.R" title="ROOM INFO">
              <KV rows={[
                ['ROOM', roomId || '—'],
                ['DECK', targetDeckKey || '—'],
                ['VIEWERS', `${viewers}`],
                ['STATUS', status?.toUpperCase() || 'UNKNOWN'],
              ]} />
              <div style={{ marginTop: 12 }}>
                <Btn ghost arrow="←" onClick={() => navigate('/spectate')}>BACK TO FISHTANK</Btn>
              </div>
            </Panel>
          </div>
        </div>
      </div>
    </>
  )
}
