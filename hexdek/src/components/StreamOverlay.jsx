import { useEffect, useMemo } from 'react'
import { useParams, useSearchParams } from 'react-router-dom'
import { useLiveSocket } from '../hooks/useLiveSocket'
import { cardArtUrl } from '../services/api'
import { useT } from '../i18n'

// StreamOverlay — transparent OBS-friendly HUD for /stream/:gameId
// (also reachable at /obs/:gameId for streamer convenience).
//
// Design constraints:
//   * No AppShell chrome (route is mounted outside AppShell).
//   * html/body forced transparent on mount, restored on unmount,
//     so OBS browser source sees alpha through the entire surface.
//   * All elements sit in fixed-position regions with their own
//     semi-opaque backgrounds, never full-bleed — bystander pixels
//     stay alpha=0 so the underlying stream layer shows through.
//   * Reads live engine data via the same useLiveSocket the Spectator
//     uses. The :gameId URL param is a sanity check: when it doesn't
//     match the current live game, the HUD shows a small "waiting"
//     pill instead of the wrong game's data.
//
// URL params (all optional, all parsed defensively):
//   * ?phase=off            — hide the three-act caption strip.
//   * ?order=2,0,1,3        — explicit seat ordering for the lower-
//                             third tiles. Indices not listed are
//                             appended in their natural order.
//   * ?focus=N (or ?seat=N) — rotate the lower-third so seat N is
//                             on top (camera focus). Ignored when
//                             ?order is also set.

// Module-level formatter, so `t` arrives as a PARAMETER rather than being
// read from module scope: anything worded at module load freezes there and
// never re-words on a locale change. The non-fallback return is a CARD
// NAME, which stays English in all 25.
const SHORT_NAME = (commander, t) => {
  if (!commander) return t('spec.seat_unknown')
  const trimmed = commander.split(',')[0].split('//')[0].trim()
  return trimmed.toUpperCase()
}

// deriveStreamPhase classifies the current game state into a three-act
// caption — OPENING / MIDGAME / CLIMAX, plus a CURTAIN state for finished
// games. Pure function; no React state, no side effects, works on the
// raw `game` payload returned by useLiveSocket.
//
// Heuristics, in priority order:
//   - finished                                   → CURTAIN
//   - any seat eliminated in the last few logs   → CLIMAX · ELIMINATION
//   - 4-player pod down to two living seats      → CLIMAX · 1V1 ENDGAME
//   - any living seat ≤ 8 life                   → CLIMAX · LETHAL RANGE
//   - turn ≥ 9 / huge battlefield / loaded yard  → CLIMAX · COMBO ASSEMBLY
//   - turn ≥ 5                                   → MIDGAME · BOARD DEVELOPMENT
//   - turn ≥ 1                                   → OPENING · RAMP PHASE
//   - everything else                            → OPENING · MULLIGAN
//
// Returns { act, labelKey } or null when there's no game to caption.
//
// labelKey is a CATALOG PATH, not a sentence. Two reasons it has to be:
// this is an exported pure function with no access to a translator, and
// the caller holds the result in a useMemo keyed on [game, phaseEnabled]
// — a worded label would be stuck at the last game update and would not
// re-word when the viewer switched locale. A key in the memo is
// locale-independent, so the wording happens at the render site.
//
// `act` STAYS ENGLISH: it is the lookup key into PHASE_ACCENT, not prose.
export function deriveStreamPhase(game) {
  if (!game) return null
  if (game.finished) {
    return {
      act: 'CURTAIN',
      labelKey: game.winner >= 0 ? 'ovl.ph_curtain_winner' : 'ovl.ph_curtain_draw',
    }
  }
  const turn = game.turn ?? 0
  const seats = Array.isArray(game.seats) ? game.seats : []
  const totalSeats = seats.length
  const alive = seats.filter(s => s && !s.lost)
  const aliveCount = alive.length

  const log = Array.isArray(game.log) ? game.log : []
  const recentElim = log.slice(-6).some(e =>
    e && (e.kind === 'elimination' || e.kind === 'player_lost')
  )
  if (recentElim) {
    return { act: 'CLIMAX', labelKey: 'ovl.ph_climax_elim' }
  }
  if (totalSeats >= 3 && aliveCount === 2) {
    return { act: 'CLIMAX', labelKey: 'ovl.ph_climax_1v1' }
  }

  const minLife = aliveCount > 0
    ? Math.min(...alive.map(s => (typeof s.life === 'number' ? s.life : 40)))
    : 40
  if (minLife <= 8) {
    return { act: 'CLIMAX', labelKey: 'ovl.ph_climax_lethal' }
  }

  // Board complexity proxy — sum across living seats. Either large
  // battlefield or a loaded graveyard signals combo assembly window.
  const bf = alive.reduce((sum, s) => {
    const len = Array.isArray(s.battlefield) ? s.battlefield.length : (s.battlefield_count ?? 0)
    return sum + len
  }, 0)
  const gy = alive.reduce((sum, s) => {
    const len = Array.isArray(s.graveyard) ? s.graveyard.length : (s.graveyard_count ?? 0)
    return sum + len
  }, 0)

  if (turn >= 9 || bf >= 25 || gy >= 35) {
    return { act: 'CLIMAX', labelKey: 'ovl.ph_climax_combo' }
  }
  if (turn >= 5) {
    return { act: 'MIDGAME', labelKey: 'ovl.ph_midgame_board' }
  }
  if (turn >= 1) {
    return { act: 'OPENING', labelKey: 'ovl.ph_opening_ramp' }
  }
  return { act: 'OPENING', labelKey: 'ovl.ph_opening_mull' }
}

// parseSeatOrder turns the ?order / ?focus URL params into a permutation
// of seat indices. Always returns an array of length seatCount, with each
// valid seat index appearing exactly once. Pure function for testability.
export function parseSeatOrder(seatCount, orderParam, focusParam) {
  const fallback = []
  for (let i = 0; i < seatCount; i++) fallback.push(i)
  if (seatCount <= 0) return fallback

  if (orderParam) {
    const parts = String(orderParam)
      .split(',')
      .map(s => parseInt(String(s).trim(), 10))
      .filter(n => Number.isInteger(n) && n >= 0 && n < seatCount)
    const seen = new Set()
    const out = []
    for (const n of parts) {
      if (seen.has(n)) continue
      seen.add(n)
      out.push(n)
    }
    if (out.length > 0) {
      for (const i of fallback) {
        if (!seen.has(i)) out.push(i)
      }
      return out
    }
  }

  const focus = parseInt(String(focusParam ?? '').trim(), 10)
  if (Number.isInteger(focus) && focus >= 0 && focus < seatCount) {
    return [...fallback.slice(focus), ...fallback.slice(0, focus)]
  }

  return fallback
}

function useTransparentRoot() {
  useEffect(() => {
    const root = document.documentElement
    const body = document.body
    const prevHtmlBg = root.style.background
    const prevBodyBg = body.style.background
    const prevHtmlColor = root.style.colorScheme
    root.style.background = 'transparent'
    body.style.background = 'transparent'
    root.style.colorScheme = 'normal'
    return () => {
      root.style.background = prevHtmlBg
      body.style.background = prevBodyBg
      root.style.colorScheme = prevHtmlColor
    }
  }, [])
}

function SeatTile({ seat, idx, isActive, isWinner, isLost }) {
  const t = useT()
  const art = cardArtUrl(seat.commander)
  const accent = isWinner ? 'var(--ok)' : isActive ? 'var(--warn)' : isLost ? 'var(--danger)' : 'rgba(255,255,255,0.45)'
  return (
    <div style={{
      display: 'grid',
      gridTemplateColumns: '40px 1fr auto',
      gap: 8,
      alignItems: 'center',
      padding: '6px 8px',
      background: 'rgba(0,0,0,0.78)',
      border: `1px solid ${accent}`,
      boxShadow: isActive ? '0 0 0 1px rgba(212, 168, 67, 0.35)' : 'none',
      opacity: isLost && !isWinner ? 0.55 : 1,
    }}>
      <div
        style={{
          width: 40, height: 40,
          backgroundImage: art ? `url(${art})` : undefined,
          backgroundColor: '#0c0d0a',
          backgroundSize: 'cover',
          backgroundPosition: 'center 30%',
          border: '1px solid rgba(255,255,255,0.15)',
          filter: isLost && !isWinner ? 'grayscale(0.8) brightness(0.6)' : 'none',
        }}
      />
      <div style={{ minWidth: 0 }}>
        <div style={{
          fontSize: 11, fontWeight: 800,
          letterSpacing: '0.06em', textTransform: 'uppercase',
          color: '#f4f0e6',
          overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
          textShadow: '0 1px 2px rgba(0,0,0,0.85)',
        }}>{SHORT_NAME(seat.commander, t)}</div>
        <div style={{
          fontSize: 9, letterSpacing: '0.08em',
          color: 'rgba(255,255,255,0.55)',
          textTransform: 'uppercase',
        }}>
          SEAT.{String(idx + 1).padStart(2, '0')}
          {isActive && <span style={{ color: 'var(--warn)', marginLeft: 6 }}>● {t('common.active')}</span>}
          {isWinner && <span style={{ color: 'var(--ok)', marginLeft: 6 }}>★ {t('dash.result_win')}</span>}
          {isLost && !isWinner && <span style={{ color: 'var(--danger)', marginLeft: 6 }}>✕ {t('spec.gb_elim')}</span>}
        </div>
      </div>
      <div style={{
        fontSize: 18, fontWeight: 900,
        fontVariantNumeric: 'tabular-nums',
        letterSpacing: '-0.02em',
        color: seat.life <= 0 ? 'var(--danger)' : seat.life <= 10 ? 'var(--warn)' : '#f4f0e6',
        textShadow: '0 1px 3px rgba(0,0,0,0.85)',
        minWidth: 36, textAlign: 'right',
      }}>
        ♥{seat.life}
      </div>
    </div>
  )
}

function Ticker({ entries }) {
  if (!entries || entries.length === 0) return null
  return (
    <div style={{
      display: 'flex',
      flexDirection: 'column',
      gap: 4,
      maxWidth: 540,
      padding: '8px 12px',
      background: 'rgba(0,0,0,0.78)',
      border: '1px solid rgba(255,255,255,0.18)',
    }}>
      {entries.map((e, i) => (
        <div
          key={`${e.turn}-${i}`}
          style={{
            display: 'grid',
            gridTemplateColumns: '38px 1fr',
            gap: 8,
            fontSize: 11,
            letterSpacing: '0.04em',
            color: i === 0 ? '#f4f0e6' : 'rgba(255,255,255,0.55)',
            opacity: i === 0 ? 1 : 0.7 - i * 0.15,
            fontWeight: i === 0 ? 700 : 500,
          }}
        >
          <span style={{
            color: 'rgba(255,255,255,0.45)',
            fontVariantNumeric: 'tabular-nums',
            fontSize: 10,
          }}>T{e.turn}</span>
          <span style={{
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
            textTransform: 'uppercase',
            textShadow: '0 1px 2px rgba(0,0,0,0.85)',
          }}>
            &gt; {e.action}
          </span>
        </div>
      ))}
    </div>
  )
}

export default function StreamOverlay() {
  const t = useT()
  useTransparentRoot()
  const { gameId } = useParams()
  const [searchParams] = useSearchParams()
  const { game, status } = useLiveSocket()

  const seats = game?.seats || []
  const log = game?.log || []
  const activeSeat = game?.active_seat ?? -1
  const turn = game?.turn ?? 0
  const finished = !!game?.finished
  const winner = finished ? game?.winner ?? -1 : -1
  const liveGameId = game?.game_id != null ? String(game.game_id) : null

  const phaseEnabled = (searchParams.get('phase') ?? 'on').toLowerCase() !== 'off'
  const phase = useMemo(
    () => phaseEnabled ? deriveStreamPhase(game) : null,
    [game, phaseEnabled]
  )

  const seatOrder = useMemo(
    () => parseSeatOrder(seats.length, searchParams.get('order'), searchParams.get('focus') ?? searchParams.get('seat')),
    [seats.length, searchParams]
  )

  // The most-recent 3 log lines, newest first. Reverse since the
  // engine appends in chronological order.
  const tickerEntries = useMemo(() => {
    if (!Array.isArray(log) || log.length === 0) return []
    return [...log].slice(-3).reverse().map(e => ({
      turn: e.turn,
      action: e.action,
      kind: e.kind,
    }))
  }, [log])

  // URL gameId is optional; when present, mismatch shows a waiting
  // pill instead of the wrong game's data so a streamer can pre-set
  // their OBS source to a specific game id.
  const mismatch = gameId && liveGameId && gameId !== liveGameId

  if (status === 'disconnected' || !game) {
    return (
      <div style={overlayShellStyle}>
        <StatusPill label={t('spec.tape_offline')} tone="danger" />
      </div>
    )
  }

  if (mismatch) {
    return (
      <div style={overlayShellStyle}>
        <StatusPill label={t('ovl.waiting_game', { id: gameId })} tone="muted" />
      </div>
    )
  }

  const activeName = activeSeat >= 0 && seats[activeSeat]
    ? SHORT_NAME(seats[activeSeat].commander, t)
    : '—'
  const activeArt = activeSeat >= 0 && seats[activeSeat]?.commander
    ? cardArtUrl(seats[activeSeat].commander)
    : null

  return (
    <div style={overlayShellStyle}>
      {/* Top-left: turn + active player */}
      <div style={{
        position: 'absolute',
        top: 18, left: 18,
        display: 'flex', alignItems: 'center', gap: 10,
      }}>
        <div style={{
          padding: '8px 14px',
          background: 'rgba(0,0,0,0.82)',
          border: '1px solid var(--warn)',
          display: 'flex', alignItems: 'center', gap: 12,
        }}>
          <div>
            <div style={{
              fontSize: 9, letterSpacing: '0.14em',
              color: 'rgba(255,255,255,0.55)',
              textTransform: 'uppercase',
            }}>{t('adm.th_turn')}</div>
            <div style={{
              fontSize: 24, fontWeight: 900, lineHeight: 1,
              color: '#f4f0e6',
              fontVariantNumeric: 'tabular-nums',
              letterSpacing: '-0.03em',
              textShadow: '0 1px 3px rgba(0,0,0,0.85)',
            }}>{turn}</div>
          </div>
          <div style={{
            width: 1, height: 36,
            background: 'rgba(255,255,255,0.2)',
          }} />
          {activeArt && (
            <div style={{
              width: 36, height: 36,
              backgroundImage: `url(${activeArt})`,
              backgroundSize: 'cover',
              backgroundPosition: 'center 30%',
              border: '1px solid rgba(255,255,255,0.2)',
            }} />
          )}
          <div>
            <div style={{
              fontSize: 9, letterSpacing: '0.14em',
              color: 'rgba(255,255,255,0.55)',
              textTransform: 'uppercase',
            }}>{t('common.active')}</div>
            <div style={{
              fontSize: 13, fontWeight: 800,
              color: 'var(--warn)',
              letterSpacing: '0.04em',
              textTransform: 'uppercase',
              textShadow: '0 1px 2px rgba(0,0,0,0.85)',
              maxWidth: 220,
              overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
            }}>{activeName}</div>
          </div>
        </div>
        {finished && <StatusPill
          label={winner >= 0
            ? t('rpt.winner_is', { name: SHORT_NAME(seats[winner]?.commander, t) })
            : t('spec.sr_draw')}
          tone="ok"
        />}
      </div>

      {/* Top-center: three-act narrative caption strip */}
      {phase && (
        <div style={{
          position: 'absolute',
          top: 18, left: '50%',
          transform: 'translateX(-50%)',
          padding: '6px 14px',
          background: 'rgba(0,0,0,0.82)',
          border: `1px solid ${PHASE_ACCENT[phase.act] || 'rgba(255,255,255,0.25)'}`,
          fontSize: 11, fontWeight: 800, letterSpacing: '0.18em',
          color: '#f4f0e6', textTransform: 'uppercase',
          textShadow: '0 1px 2px rgba(0,0,0,0.85)',
          maxWidth: '60%',
          overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
        }}>
          {t(phase.labelKey)}
        </div>
      )}

      {/* Top-right: seat life totals + commanders (lower-third tiles
          stacked top-down; order respects ?order / ?focus URL params). */}
      <div style={{
        position: 'absolute',
        top: 18, right: 18,
        display: 'flex', flexDirection: 'column',
        gap: 6, width: 280,
      }}>
        {seatOrder.map((i) => {
          const s = seats[i]
          if (!s) return null
          return (
            <SeatTile
              key={i}
              seat={s}
              idx={i}
              isActive={i === activeSeat && !finished}
              isWinner={finished && i === winner}
              isLost={!!s.lost}
            />
          )
        })}
      </div>

      {/* Bottom-left: action ticker */}
      <div style={{ position: 'absolute', bottom: 18, left: 18 }}>
        <Ticker entries={tickerEntries} />
      </div>

      {/* Bottom-right: tiny brand mark */}
      <div style={{
        position: 'absolute',
        bottom: 18, right: 18,
        fontSize: 9, letterSpacing: '0.18em',
        color: 'rgba(255,255,255,0.35)',
        textTransform: 'uppercase',
        textShadow: '0 1px 2px rgba(0,0,0,0.85)',
      }}>HEXDEK//STREAM</div>
    </div>
  )
}

const overlayShellStyle = {
  position: 'fixed',
  inset: 0,
  pointerEvents: 'none',
  fontFamily: "'JetBrains Mono', ui-monospace, monospace",
  background: 'transparent',
  overflow: 'hidden',
}

// Border accents for the three-act caption strip — picks up the same
// CSS variables the rest of the overlay already uses, so theme changes
// flow through without touching this file.
const PHASE_ACCENT = {
  OPENING: 'rgba(160, 200, 255, 0.55)',
  MIDGAME: 'var(--warn)',
  CLIMAX:  'var(--danger)',
  CURTAIN: 'var(--ok)',
}

function StatusPill({ label, tone = 'muted' }) {
  const color = tone === 'danger' ? 'var(--danger)' : tone === 'ok' ? 'var(--ok)' : 'rgba(255,255,255,0.55)'
  return (
    <div style={{
      padding: '6px 12px',
      background: 'rgba(0,0,0,0.82)',
      border: `1px solid ${color}`,
      color,
      fontSize: 11,
      fontWeight: 800,
      letterSpacing: '0.14em',
      textTransform: 'uppercase',
      textShadow: '0 1px 2px rgba(0,0,0,0.85)',
    }}>
      {label}
    </div>
  )
}
