import { useState, useEffect, useRef } from 'react'
import { useParams, useSearchParams } from 'react-router-dom'
import { Panel, KV, Bar, Tag, Btn, Tape } from '../components/chrome'
import SummaryActions from '../components/SummaryActions'
import { api, cardArtUrl } from '../services/api'
import { useT } from '../i18n'
import {
  REPLAY_SPEEDS,
  REPLAY_DEFAULT_SPEED_IDX,
  tickIntervalMs,
  nextSpeedIdx,
  stepTurn,
  resolveReplayKeyAction,
} from '../utils/replayControls'
import {
  BOOKMARK_KINDS,
  DEFAULT_BOOKMARK_KIND,
  loadBookmarks,
  saveBookmarks,
  addBookmark,
  removeBookmark,
  nextBookmarkTurn,
  kindMeta,
  resolveBookmarkKeyAction,
} from '../utils/replayBookmarks'
import {
  parseShareParams,
  buildShareUrl,
} from '../utils/replayShare'
import {
  extractKeyMoments,
  momentMeta,
  countsTooltip,
} from '../utils/replayTimeline'
import {
  SHARE_PARAM_COMPARE,
  mapSharedTurn,
  compareSeatDeltas,
} from '../utils/replayCompare'
import {
  isVideoExportSupported,
  recordReplayVideo,
  videoFileName,
  formatVideoDuration,
  buildFramePlan,
  pickMimeType,
  DEFAULT_VIDEO_WIDTH,
  DEFAULT_VIDEO_HEIGHT,
} from '../utils/videoExport'

// ── API gaps for full report fidelity ──────────────────────────────────
// CompletedGame currently exposes: game_id, commanders[], deck_keys[],
// winner, winner_name, turns, end_reason, finished_at, final_seats[].
//
// The Report.jsx analysis sections (RPT.C/D/E/F) are now built strictly
// from those fields:
//   * Win condition          — classifyWinCondition(end_reason) +
//                              eliminated seats from final_seats[]
//   * MVP card               — composite score over winner battlefield
//                              (commander, P/T, counters). Tagged
//                              HEURISTIC since per-card impact telemetry
//                              isn't tracked yet.
//   * Game timeline          — open + per-elimination (loss_reason) +
//                              finishing line. Eliminations lack a turn
//                              stamp because per-turn log[] is not
//                              retained server-side; rendered as `— —`
//                              instead of fabricated.
//   * Per-seat performance   — pure final_seats[] readout. The old
//                              interpolated life-over-time sparkline
//                              was removed.
//
// Future server-side work that would unlock additional fidelity:
//   * log[] retention on CompletedGame would let us anchor each
//     elimination + key play to its turn.
//   * life_history[] per seat per turn → real life-over-time sparklines.
//   * per_card_impact[] → damage-dealt-based MVP ranking.
//   * commander_damage[][] seat × seat matrix → finer 21+ attribution.
// ───────────────────────────────────────────────────────────────────────

const Stat2 = ({ k, v, big }) => (
  <div>
    <div className="t-xs muted">{k}</div>
    <div className={big ? 't-3xl' : 't-2xl'} style={{ fontWeight: 800, marginTop: 4 }}>{v}</div>
  </div>
)

const HeuristicTag = () => {
  const t = useT()
  return <Tag kind="warn" style={{ fontSize: 8, padding: '1px 5px' }}>{t('rpt.heuristic')}</Tag>
}

// classifyWinCondition derives a human-readable win-condition label from
// CompletedGame.end_reason. Honest mapping; nothing mocked here.
// The table holds catalog KEYS, not words. Holding the English label
// here and reading it at render would freeze it at module load and
// ignore a language switch.
const WIN_CONDITION_MAP = {
  sba_704_5a: { labelKey: 'rpt.wc_damage', detailKey: 'rpt.wc_damage_d', kind: 'damage' },
  sba_704_5b: { labelKey: 'rpt.wc_mill', detailKey: 'rpt.wc_mill_d', kind: 'mill' },
  sba_704_5c: { labelKey: 'rpt.wc_poison', detailKey: 'rpt.wc_poison_d', kind: 'poison' },
  sba_704_5d: { labelKey: 'rpt.wc_cmdr', detailKey: 'rpt.wc_cmdr_d', kind: 'commander_damage' },
  concession: { labelKey: 'rpt.wc_concede', detailKey: 'rpt.wc_concede_d', kind: 'concession' },
  turn_limit: { labelKey: 'rpt.wc_turn_limit', detailKey: 'rpt.wc_turn_limit_d', kind: 'stall' },
}
const classifyWinCondition = (endReason) => {
  if (!endReason) return { labelKey: 'dash.unknown', detailKey: 'rpt.wc_unknown_d', kind: 'unknown' }
  const key = endReason.toLowerCase()
  // Unrecognised end reason: echo the server's own value rather than
  // inventing a label for it, exactly as before.
  return WIN_CONDITION_MAP[key] || {
    label: endReason.replace(/_/g, ' ').toUpperCase(),
    detailKey: 'rpt.wc_sba_d',
    kind: 'sba',
  }
}
// Resolves either shape — a keyed row, or the echoed server value.
const wcLabel = (t, wc) => (wc.labelKey ? t(wc.labelKey) : wc.label)

// Short label for the END stat in the result block — the column is
// narrow (1fr of 4 in result-block-grid) and the raw replacement
// produces strings like "LAST SEAT STANDING" that overflow.
const END_REASON_SHORT = {
  last_seat_standing: 'rpt.es_last_seat',
  sba_704_5a: 'rpt.es_damage',
  sba_704_5b: 'rpt.es_mill',
  sba_704_5c: 'rpt.es_poison',
  sba_704_5d: 'rpt.es_cmdr_dmg',
  concession: 'rpt.es_concede',
  turn_limit: 'rpt.es_turn_cap',
}
const endReasonShort = (t, r) => {
  if (!r) return '?'
  const key = END_REASON_SHORT[r.toLowerCase()]
  return key ? t(key) : r.replace(/_/g, ' ').toUpperCase()
}

// pickMVP scores the winner's final-board permanents by a composite
// weight and returns the top scorer with a human-readable reason. Still
// a heuristic in the strict sense (we don't have per-card impact
// telemetry — damage dealt, combos enabled, removal absorbed), but the
// scoring is multi-signal so a generic 1/1 doesn't beat the commander
// just because it sorted first.
//
// Score components (additive):
//   commander    +50      single hard signal we trust
//   creature     +10      creatures finish games more often than
//                         artifacts/enchantments at this scale
//   power        +power   raw beatdown weight
//   toughness    +toughness/2  staying-power tiebreak
//   non-land     +5       avoid promoting basics
//   counters     +Σcounters*3  +1/+1, charge, loyalty proxy
const pickMVP = (t, winnerSeat) => {
  if (!winnerSeat?.battlefield?.length) return null
  const perms = winnerSeat.battlefield
  const score = (p) => {
    let s = 0
    if (p.is_commander) s += 50
    if (!p.is_land) s += 5
    const types = (p.type || '').toLowerCase()
    if (types.includes('creature') || p.power != null) s += 10
    if (typeof p.power === 'number') s += Math.max(0, p.power)
    if (typeof p.toughness === 'number') s += Math.max(0, p.toughness) / 2
    if (p.counters && typeof p.counters === 'object') {
      for (const v of Object.values(p.counters)) {
        if (typeof v === 'number') s += v * 3
      }
    }
    return s
  }
  let best = perms[0]
  let bestScore = score(perms[0])
  for (const p of perms) {
    const s = score(p)
    if (s > bestScore) { best = p; bestScore = s }
  }
  // Build a reason string from the dominant signal. `t` comes from the
  // call site so this stays a plain function.
  let reason
  if (best.is_commander) {
    reason = t('rpt.mvp_commander')
  } else if (best.power != null && best.power >= 5) {
    reason = t('rpt.mvp_power', { pt: `${best.power}/${best.toughness ?? '?'}` })
  } else if (best.counters && Object.values(best.counters).some(v => v >= 3)) {
    const total = Object.values(best.counters).reduce((a, b) => a + (typeof b === 'number' ? b : 0), 0)
    reason = t(total === 1 ? 'rpt.mvp_counters_one' : 'rpt.mvp_counters', { count: total })
  } else if (!best.is_land) {
    reason = t('rpt.mvp_nonland')
  } else {
    reason = t('rpt.mvp_only')
  }
  return { perm: best, reason, score: bestScore }
}

// deriveTimeline builds a turn-anchored key-moments list from real
// CompletedGame fields only — game start, the winner's final board
// composition, each opponent's elimination (with loss reason), and the
// finishing-line classified from end_reason.
//
// Caveat: the engine doesn't yet retain per-turn log[] on
// CompletedGame, so we don't know the exact turn each elimination
// landed. Eliminations are placed as a single "ELIMINATIONS" pre-final
// entry with all losers stacked, ordering preserved by seat index.
// This is intentionally honest — every line in the returned list is
// derivable from data the API already returns; nothing is invented.
const deriveTimeline = (t, game, commanders) => {
  if (!game) return []
  const total = Math.max(1, game.turns || 1)
  const winner = game.winner ?? -1
  const winName = winner >= 0 ? (commanders[winner] || t('rpt.tl_winner_fallback')).split(',')[0].toUpperCase() : null
  const seats = game.final_seats || []
  const cond = classifyWinCondition(game.end_reason)

  const out = []
  // Game start — every game has one.
  out.push({
    turn: 1,
    seat: -1,
    kind: 'open',
    action: t(commanders.length === 1 ? 'rpt.tl_open_one' : 'rpt.tl_open', { count: commanders.length }),
    detail: commanders.map(c => (c || t('dash.unknown')).split(',')[0].toUpperCase()).join(' · '),
  })

  // Eliminations — collected from final_seats. Without per-turn data
  // we group them as a single mid-game block; loss_reason is real.
  const losers = seats
    .map((s, i) => ({ s, i }))
    .filter(x => x.s.lost && x.i !== winner)
  for (const { s, i } of losers) {
    const cmdrShort = (commanders[i] || t('dash.unknown')).split(',')[0].toUpperCase()
    const reason = (s.loss_reason || s.LossReason || '').replace(/_/g, ' ').toUpperCase().trim()
    out.push({
      turn: null, // unknown — we don't have per-elimination turn data
      seat: i,
      kind: 'eliminated',
      action: t('rpt.tl_eliminated', { name: cmdrShort }),
      detail: reason ? t('rpt.tl_loss', { reason }) : t('rpt.tl_loss_sba'),
    })
  }

  // Finishing line — anchored at the recorded turn count.
  if (winner >= 0) {
    const finishKeys = {
      damage:            'rpt.tl_fin_damage',
      mill:              'rpt.tl_fin_mill',
      poison:            'rpt.tl_fin_poison',
      commander_damage:  'rpt.tl_fin_cmdr',
      concession:        'rpt.tl_fin_concede',
      stall:             'rpt.tl_fin_stall',
    }
    const endReason = (game.end_reason || '?').replace(/_/g, ' ').toUpperCase()
    out.push({
      turn: total,
      seat: winner,
      kind: 'win',
      action: t(finishKeys[cond.kind] || 'rpt.tl_fin_generic', { name: winName }),
      detail: t('rpt.tl_end_reason', { reason: endReason }),
    })
  } else {
    out.push({
      turn: total,
      seat: -1,
      kind: 'draw',
      action: t('rpt.tl_draw'),
      detail: t('rpt.tl_end_reason', { reason: (game.end_reason || '?').replace(/_/g, ' ').toUpperCase() }),
    })
  }
  return out
}

/* ─── Replay Scrubber ───────────────────────────────────────────
   Renders a turn slider over CompletedGame.timeline[] (per-turn
   snapshots captured server-side during live play). At each turn
   position we show:
     · life totals + zone counts per seat
     · board state per seat (compact list of permanents)
     · the events fired during that turn (cast/play_land/combat/etc.)

   Fallback: games rehydrated from SQLite have no timeline (per-turn
   data isn't persisted); we render a single-line notice and skip the
   scrubber instead of fabricating interpolated frames.
*/
const ReplayScrubber = ({ game, commanders, compareGame = null, compareCommanders = [], onClearCompare }) => {
  const t = useT()
  const timeline = game?.timeline || []
  const totalTurns = timeline.length
  const [turnIdx, setTurnIdx] = useState(0) // 0-based into timeline
  const [playing, setPlaying] = useState(false)
  // Index into REPLAY_SPEEDS (0.5× / 1× / 2× / 4×). The autoplay tick
  // reads tickIntervalMs(REPLAY_SPEEDS[speedIdx]) so 1× preserves the
  // historical 900ms cadence and 4× lets users skim a long game in a
  // quarter the time.
  const [speedIdx, setSpeedIdx] = useState(REPLAY_DEFAULT_SPEED_IDX)
  // Bookmarks persist per game in localStorage. The state mirrors what's
  // on disk so the render is synchronous; we re-read on game switch
  // (game?.game_id dep) and write through whenever the list changes.
  const gameId = game?.game_id
  const [bookmarks, setBookmarks] = useState([])
  const [bookmarkKind, setBookmarkKind] = useState(DEFAULT_BOOKMARK_KIND)
  // Focused seat (0..3) for the share link's optional &seat=. null
  // means "no seat highlighted" — the seat card outline thickens
  // when this is set. Click any seat-card header to toggle focus.
  const [focusedSeat, setFocusedSeat] = useState(null)
  // Toast for the "share" button feedback. null when idle, a string
  // when the URL was just copied (auto-clears after ~2s).
  const [shareToast, setShareToast] = useState(null)
  // Video export state. exportStatus is one of: null (idle),
  // 'recording' (in progress), 'saved' (just finished — shown as a
  // toast briefly), 'error'. exportProgress is { idx, total } for
  // the progress bar. exportCanvasRef holds the offscreen canvas we
  // paint frames onto; it's an attached <canvas> (1280×720, opacity
  // 0 so it doesn't visually intrude) because OffscreenCanvas isn't
  // universally supported and a hidden <canvas> works in every
  // browser that has MediaRecorder.
  const [exportStatus, setExportStatus] = useState(null)
  const [exportProgress, setExportProgress] = useState({ idx: 0, total: 0 })
  const exportCanvasRef = useRef(null)
  const exportAbortRef = useRef(null)
  const [searchParams, setSearchParams] = useSearchParams()
  // Track whether we've already consumed an initial ?t=/&seat= for
  // THIS game so a later URL edit (back / forward) re-applies, but a
  // normal scrub doesn't keep snapping back.
  const consumedInitialParamsRef = useRef({ gameId: null, payload: '' })

  // Load bookmarks when the game id changes. This effect intentionally
  // calls setState — it's the only way to react to a foreign data source
  // (localStorage) keyed off a prop. The cascading-renders lint rule
  // doesn't have a way to express "external sync" so we silence it
  // here; the read is cheap (single JSON parse) and only runs on game
  // switch.
  useEffect(() => {
    if (typeof window === 'undefined') return
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setBookmarks(loadBookmarks(window.localStorage, gameId))
  }, [gameId])

  // Persist bookmarks on every mutation. Cheap (small JSON, infrequent).
  useEffect(() => {
    if (typeof window === 'undefined' || gameId == null) return
    saveBookmarks(window.localStorage, gameId, bookmarks)
  }, [gameId, bookmarks])

  // turnAtIdx maps a 0-based timeline index to the game's actual turn
  // number (timeline entries can start at turn 1 with optional gaps).
  // Used by the bookmark add/jump handlers so persisted bookmarks
  // survive a future replay-server change that re-orders snapshots.
  const turnAtIdx = (idx) => timeline[idx]?.turn ?? (idx + 1)

  const idxAtTurn = (turn) => {
    for (let i = 0; i < timeline.length; i++) {
      if (timeline[i].turn === turn) return i
    }
    // Closest preceding snapshot if the exact turn isn't in the timeline.
    let chosen = 0
    for (let i = 0; i < timeline.length; i++) {
      if (timeline[i].turn <= turn) chosen = i
      else break
    }
    return chosen
  }

  // Deep-link consumption. When the URL carries ?t=N (& optionally
  // &seat=I), jump the scrubber to that turn and highlight that seat.
  // The ref-guard re-applies only when the params or game change, so
  // a user scrubbing past the shared moment doesn't get yanked back.
  useEffect(() => {
    if (totalTurns === 0) return
    const lastTurn = timeline[totalTurns - 1].turn
    const seatCount = timeline[0]?.seats?.length || 0
    const { turn, seat } = parseShareParams(searchParams, { maxTurn: lastTurn, maxSeats: seatCount })
    const payload = `${turn}|${seat}`
    const consumed = consumedInitialParamsRef.current
    if (consumed.gameId === gameId && consumed.payload === payload) return
    consumedInitialParamsRef.current = { gameId, payload }
    // External-sync effect: react to a foreign data source (URL
    // query params) keyed off the gameId + searchParams. The
    // cascading-renders lint rule doesn't have a way to express
    // this; ref-guard above ensures we only setState when the URL
    // payload genuinely changes.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    if (turn != null) { setTurnIdx(idxAtTurn(turn)); setPlaying(false) }
    if (seat != null) setFocusedSeat(seat)
    // searchParams comes from useSearchParams; idxAtTurn / timeline
    // close over the same render snapshot, no extra deps needed.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [searchParams, gameId, totalTurns])

  // Auto-play tick — advances one turn at the user-selected speed.
  useEffect(() => {
    if (!playing || totalTurns === 0) return
    const id = setInterval(() => {
      setTurnIdx(i => {
        if (i >= totalTurns - 1) {
          setPlaying(false)
          return i
        }
        return i + 1
      })
    }, tickIntervalMs(REPLAY_SPEEDS[speedIdx]))
    return () => clearInterval(id)
  }, [playing, totalTurns, speedIdx])

  // Clamp the slider to a valid index when timeline shrinks (e.g. on
  // game switch). Defensive — totalTurns only changes when game does.
  useEffect(() => {
    if (turnIdx >= totalTurns) setTurnIdx(Math.max(0, totalTurns - 1))
  }, [totalTurns, turnIdx])

  // Keyboard shortcuts. Two handlers: replay transport (Space/K play,
  // ←J/→L step, Home/End jump, ↑↓/[] speed) and bookmarks (B to mark
  // the current turn, , and . to jump to the prev/next bookmark).
  useEffect(() => {
    if (totalTurns === 0) return
    const onKey = (e) => {
      const action = resolveReplayKeyAction(e)
      if (action) {
        e.preventDefault()
        switch (action) {
          case 'togglePlay':
            setPlaying(p => !p)
            break
          case 'first':
            setPlaying(false)
            setTurnIdx(stepTurn(turnIdx, totalTurns, 'first'))
            break
          case 'last':
            setPlaying(false)
            setTurnIdx(stepTurn(turnIdx, totalTurns, 'last'))
            break
          case 'prev':
            setPlaying(false)
            setTurnIdx(stepTurn(turnIdx, totalTurns, 'prev'))
            break
          case 'next':
            setPlaying(false)
            setTurnIdx(stepTurn(turnIdx, totalTurns, 'next'))
            break
          case 'speedDown':
            setSpeedIdx(i => nextSpeedIdx(i, -1))
            break
          case 'speedUp':
            setSpeedIdx(i => nextSpeedIdx(i, +1))
            break
        }
        return
      }
      const bm = resolveBookmarkKeyAction(e)
      if (!bm) return
      e.preventDefault()
      const currentTurn = turnAtIdx(turnIdx)
      switch (bm) {
        case 'addBookmark':
          setBookmarks(list => addBookmark(list, currentTurn, bookmarkKind, ''))
          break
        case 'prevBookmark': {
          const t = nextBookmarkTurn(bookmarks, currentTurn, -1)
          if (t != null) {
            setPlaying(false)
            setTurnIdx(idxAtTurn(t))
          }
          break
        }
        case 'nextBookmark': {
          const t = nextBookmarkTurn(bookmarks, currentTurn, +1)
          if (t != null) {
            setPlaying(false)
            setTurnIdx(idxAtTurn(t))
          }
          break
        }
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
    // turnAtIdx / idxAtTurn close over `timeline`; tracking
    // timeline.length covers the only mutation path that matters
    // (game switch / snapshot append).
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [turnIdx, totalTurns, bookmarks, bookmarkKind])

  if (totalTurns === 0) {
    return (
      <Panel code="RPT.X" title={t('rpl.title')} style={{ gridColumn: '1 / -1' }}>
        <div className="t-xs muted-2" style={{ lineHeight: 1.5 }}>
          &gt; {t('rpl.no_timeline')}
        </div>
      </Panel>
    )
  }

  const snap = timeline[turnIdx]
  const turnNo = snap?.turn ?? (turnIdx + 1)
  const activeSeat = snap?.active_seat ?? -1
  const events = snap?.events || []

  return (
    <Panel
      code="RPT.X"
      title={t('rpl.title_turn', { turn: turnNo, total: timeline[totalTurns - 1].turn })}
      style={{ gridColumn: '1 / -1' }}
      right={
        <span className="t-xs muted">
          {t(totalTurns === 1 ? 'rpl.snapshots_one' : 'rpl.snapshots', { count: totalTurns })}
        </span>
      }
    >
      {/* Transport: prev / play / next / jump-to-end + speed presets */}
      <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 10, flexWrap: 'wrap' }}>
        <Btn sm arrow={null} onClick={() => { setPlaying(false); setTurnIdx(stepTurn(turnIdx, totalTurns, 'first')) }} disabled={turnIdx === 0} title={t('rpl.tip_first')}>⏮</Btn>
        <Btn sm arrow={null} onClick={() => { setPlaying(false); setTurnIdx(stepTurn(turnIdx, totalTurns, 'prev')) }} disabled={turnIdx === 0} title={t('rpl.tip_prev')}>◀</Btn>
        <Btn sm arrow={null} solid={playing} onClick={() => setPlaying(p => !p)} title={t('rpl.tip_play')}>{playing ? t('rpl.pause') : t('rpl.play')}</Btn>
        <Btn sm arrow={null} onClick={() => { setPlaying(false); setTurnIdx(stepTurn(turnIdx, totalTurns, 'next')) }} disabled={turnIdx >= totalTurns - 1} title={t('rpl.tip_next')}>▶</Btn>
        <Btn sm arrow={null} onClick={() => { setPlaying(false); setTurnIdx(stepTurn(turnIdx, totalTurns, 'last')) }} disabled={turnIdx >= totalTurns - 1} title={t('rpl.tip_last')}>⏭</Btn>
        {/* Speed: discrete presets. Active mark = ok-color. Hotkeys
            ↑/↓ + [/] step through the same list. */}
        <div role="group" aria-label={t('rpl.aria_speed')} style={{ display: 'flex', gap: 2, marginLeft: 8, padding: '0 4px', borderLeft: '1px solid var(--rule-2)' }}>
          {REPLAY_SPEEDS.map((s, i) => {
            const active = i === speedIdx
            return (
              <button
                type="button"
                key={s}
                onClick={() => setSpeedIdx(i)}
                aria-pressed={active}
                aria-label={t('rpl.aria_set_speed', { speed: s })}
                title={t('rpl.tip_speed', { speed: s })}
                className="t-xs"
                style={{
                  background: 'transparent',
                  border: 'none',
                  cursor: 'pointer',
                  padding: '2px 6px',
                  fontWeight: active ? 700 : 400,
                  color: active ? 'var(--ok)' : 'var(--ink-2)',
                }}
              >
                {s}×
              </button>
            )
          })}
        </div>
        {/* Bookmark add: kind picker + "+" button. The picker stores
            the kind for the next add; pressing B uses the same kind.
            Existing-mark detection swaps "+" for a "remove" affordance
            when the current turn already carries a bookmark of the
            selected kind. */}
        <div role="group" aria-label={t('rpl.aria_bookmark_group')} style={{ display: 'flex', gap: 2, marginLeft: 8, padding: '0 4px', borderLeft: '1px solid var(--rule-2)', alignItems: 'center' }}>
          <select
            value={bookmarkKind}
            onChange={(e) => setBookmarkKind(e.target.value)}
            aria-label={t('rpl.aria_bookmark_kind')}
            className="t-xs"
            style={{
              background: 'transparent',
              color: 'var(--ink)',
              border: '1px solid var(--rule-2)',
              padding: '1px 4px',
              cursor: 'pointer',
            }}
          >
            {BOOKMARK_KINDS.map(k => (
              <option key={k.id} value={k.id}>{k.icon} {t(k.labelKey)}</option>
            ))}
          </select>
          {(() => {
            const currentTurn = turnAtIdx(turnIdx)
            const existingId = `${currentTurn}-${bookmarkKind}`
            const isMarked = bookmarks.some(b => b.id === existingId)
            return (
              <Btn
                sm
                arrow={null}
                solid={isMarked}
                title={t(isMarked ? 'rpl.tip_unmark' : 'rpl.tip_mark', { kind: t(kindMeta(bookmarkKind).labelKey), turn: currentTurn })}
                onClick={() => {
                  if (isMarked) {
                    setBookmarks(list => removeBookmark(list, existingId))
                  } else {
                    setBookmarks(list => addBookmark(list, currentTurn, bookmarkKind, ''))
                  }
                }}
              >
                {isMarked ? '−' : '+'} {kindMeta(bookmarkKind).icon}
              </Btn>
            )
          })()}
        </div>
        {/* Share this exact moment. Builds a URL with ?t=N (and
            &seat=I if a seat is focused), copies it to the clipboard,
            and writes the same params into the address bar via
            setSearchParams so a manual refresh lands on the same
            moment. The "SHARED" toast auto-clears after ~2s. */}
        <div style={{ display: 'flex', gap: 6, marginLeft: 8, padding: '0 4px', borderLeft: '1px solid var(--rule-2)', alignItems: 'center' }}>
          <Btn
            sm
            arrow={null}
            title={focusedSeat != null
              ? t('rpl.tip_copy_link_seat', { turn: turnAtIdx(turnIdx), seat: String(focusedSeat + 1).padStart(2, '0') })
              : t('rpl.tip_copy_link', { turn: turnAtIdx(turnIdx) })}
            onClick={() => {
              const turn = turnAtIdx(turnIdx)
              const next = new URLSearchParams(searchParams)
              next.set('t', String(turn))
              if (focusedSeat != null) next.set('seat', String(focusedSeat))
              else next.delete('seat')
              // Reflect the share state in the URL bar without pushing
              // a history entry per share click.
              setSearchParams(next, { replace: true })
              if (typeof window !== 'undefined') {
                const url = buildShareUrl(window.location.origin + window.location.pathname, {
                  turn,
                  seat: focusedSeat,
                })
                if (navigator?.clipboard?.writeText) {
                  // The toast holds a catalog KEY, not a worded string:
                  // wording it inside .then() would freeze the language at
                  // click time and ignore a later switch.
                  navigator.clipboard.writeText(url).then(
                    () => { setShareToast('rpl.toast_copied') },
                    () => { setShareToast('rpl.toast_copy_failed') },
                  )
                } else {
                  setShareToast('rpl.toast_ready')
                }
                setTimeout(() => setShareToast(null), 2000)
              }
            }}
          >
            {t('rpl.share')}
          </Btn>
          {shareToast && (
            <span className="t-xs" style={{ color: 'var(--ok)', fontWeight: 700 }}>
              {t(shareToast)}
            </span>
          )}
        </div>
        {/* Video export. Renders only when the browser supports
            MediaRecorder + canvas.captureStream + at least one of
            our preferred MIME types — node tests + older browsers
            silently hide the button. Click starts a recording that
            paints each turn onto a hidden 1280×720 canvas; the
            resulting Blob downloads as `hexdek-replay-<id>.mp4`
            (Safari) or `.webm` (Chrome/Firefox). */}
        {typeof window !== 'undefined' && isVideoExportSupported(window) && (
          <div style={{ display: 'flex', gap: 6, marginLeft: 8, padding: '0 4px', borderLeft: '1px solid var(--rule-2)', alignItems: 'center' }}>
            <Btn
              sm
              arrow={null}
              solid={exportStatus === 'recording'}
              title={(() => {
                if (exportStatus === 'recording') return t('rpl.tip_cancel_rec')
                const plan = buildFramePlan(timeline)
                const mime = pickMimeType(window) || ''
                const ext = mime.includes('mp4') ? 'mp4' : 'webm'
                return t('rpl.tip_export', { ext, duration: formatVideoDuration(plan.totalMs) })
              })()}
              onClick={async () => {
                if (exportStatus === 'recording') {
                  // Mid-recording click cancels.
                  exportAbortRef.current?.abort()
                  return
                }
                const canvas = exportCanvasRef.current
                if (!canvas) return
                setExportStatus('recording')
                setExportProgress({ idx: 0, total: 0 })
                const controller = new AbortController()
                exportAbortRef.current = controller
                try {
                  const blob = await recordReplayVideo({
                    canvas,
                    timeline,
                    commanders,
                    gameId: game?.game_id ?? 'unknown',
                    signal: controller.signal,
                    onProgress: ({ idx, total }) => setExportProgress({ idx, total }),
                  })
                  const mime = pickMimeType(window) || 'video/webm'
                  const url = URL.createObjectURL(blob)
                  const a = document.createElement('a')
                  a.href = url
                  a.download = videoFileName(game?.game_id, mime)
                  document.body.appendChild(a)
                  a.click()
                  document.body.removeChild(a)
                  URL.revokeObjectURL(url)
                  setExportStatus('saved')
                  setTimeout(() => setExportStatus(null), 2500)
                } catch (err) {
                  console.warn('replay export failed:', err)
                  setExportStatus('error')
                  setTimeout(() => setExportStatus(null), 2500)
                } finally {
                  exportAbortRef.current = null
                }
              }}
            >
              {exportStatus === 'recording'
                ? (exportProgress.total
                    ? t('rpl.rec_progress', { idx: exportProgress.idx + 1, total: exportProgress.total })
                    : t('rpl.rec'))
                : t('rpl.export')}
            </Btn>
            {exportStatus === 'saved' && (
              <span className="t-xs" style={{ color: 'var(--ok)', fontWeight: 700 }}>{t('rpl.saved')}</span>
            )}
            {exportStatus === 'error' && (
              <span className="t-xs" style={{ color: 'var(--danger)', fontWeight: 700 }}>{t('rpl.failed')}</span>
            )}
            {/* The offscreen render target. position:absolute +
                opacity:0 keeps it laid out (so captureStream() works)
                while staying invisible. pointerEvents none so it
                doesn't intercept clicks. */}
            <canvas
              ref={exportCanvasRef}
              width={DEFAULT_VIDEO_WIDTH}
              height={DEFAULT_VIDEO_HEIGHT}
              aria-hidden="true"
              style={{
                position: 'absolute', left: -9999, top: -9999,
                width: 1, height: 1, opacity: 0, pointerEvents: 'none',
              }}
            />
          </div>
        )}
        <span className="t-xs muted" style={{ marginLeft: 8 }}>
          {t('rpl.active_label')}: {activeSeat >= 0 ? `SEAT.${String(activeSeat + 1).padStart(2, '0')} · ${(commanders[activeSeat] || t('dash.unknown')).split(',')[0].toUpperCase()}` : '—'}
          {focusedSeat != null && (
            <span style={{ marginLeft: 6, color: 'var(--accent)' }}>
              · {t('rpl.focus_label')}: SEAT.{String(focusedSeat + 1).padStart(2, '0')}
            </span>
          )}
        </span>
      </div>

      {/* Key-moment timeline strip — one cell per snapshot, colored
          by that turn's highest-priority moment. Click a cell to
          jump to that turn. Empty cells (no key moments) render as a
          dim placeholder so the strip width tracks the slider width
          exactly. extractKeyMoments is recomputed on every render —
          cheap (single linear pass over events) and avoids stale
          summaries on game switch. */}
      {(() => {
        const moments = extractKeyMoments(timeline)
        if (moments.length === 0) return null
        return (
          <div
            role="group"
            aria-label={t('rpl.aria_moments')}
            style={{
              display: 'grid',
              gridTemplateColumns: `repeat(${moments.length}, 1fr)`,
              gap: 1,
              marginBottom: 6,
              minHeight: 16,
            }}
          >
            {moments.map((m, i) => {
              const meta = momentMeta(m.primary)
              const isCurrent = i === turnIdx
              if (!meta) {
                return (
                  <button
                    key={i}
                    type="button"
                    onClick={() => { setPlaying(false); setTurnIdx(i) }}
                    title={t('rpl.tip_quiet', { turn: m.turn ?? i + 1 })}
                    aria-label={t('rpl.aria_jump_turn', { turn: m.turn ?? i + 1 })}
                    style={{
                      height: 16,
                      background: isCurrent ? 'var(--rule)' : 'var(--rule-2)',
                      border: 'none',
                      cursor: 'pointer',
                      padding: 0,
                      opacity: 0.4,
                    }}
                  />
                )
              }
              const momentTurn = m.turn ?? i + 1
              const momentLabel = t(meta.labelKey)
              const tip = m.totalKeyEvents > 1
                ? t('rpl.tip_moment_multi', { turn: momentTurn, label: momentLabel, counts: countsTooltip(t, m.counts) })
                : t('rpl.tip_moment', { turn: momentTurn, label: momentLabel })
              return (
                <button
                  key={i}
                  type="button"
                  onClick={() => { setPlaying(false); setTurnIdx(i) }}
                  title={tip}
                  aria-label={tip}
                  style={{
                    height: 16,
                    background: meta.color,
                    border: 'none',
                    cursor: 'pointer',
                    padding: 0,
                    color: 'var(--bg)',
                    fontSize: 10,
                    fontWeight: 700,
                    lineHeight: 1,
                    boxShadow: isCurrent ? 'inset 0 0 0 2px var(--ink)' : undefined,
                    opacity: m.totalKeyEvents > 0 ? 1 : 0.5,
                  }}
                >
                  {meta.icon}
                </button>
              )
            })}
          </div>
        )
      })()}

      {/* The slider itself, with bookmark markers overlaid. Markers
          sit absolutely-positioned above the track at the % offset
          that matches each bookmark's snapshot index; clicking one
          jumps to that turn. */}
      <div style={{ position: 'relative', width: '100%' }}>
        <input
          type="range"
          min={0}
          max={totalTurns - 1}
          value={turnIdx}
          onChange={e => { setPlaying(false); setTurnIdx(parseInt(e.target.value, 10)) }}
          style={{ width: '100%', accentColor: 'var(--accent)', display: 'block' }}
          aria-label={t('rpl.aria_slider')}
        />
        {bookmarks.length > 0 && (
          <div
            aria-hidden="true"
            style={{ position: 'absolute', left: 0, right: 0, top: -6, height: 6, pointerEvents: 'none' }}
          >
            {bookmarks.map(b => {
              const idx = idxAtTurn(b.turn)
              const pct = totalTurns > 1 ? (idx / (totalTurns - 1)) * 100 : 0
              const meta = kindMeta(b.kind)
              return (
                <button
                  key={b.id}
                  type="button"
                  onClick={() => { setPlaying(false); setTurnIdx(idx) }}
                  title={t(b.note ? 'rpl.tip_bookmark_at_note' : 'rpl.tip_bookmark_at', { kind: t(meta.labelKey), turn: b.turn, note: b.note })}
                  aria-label={t('rpl.aria_jump_bookmark', { kind: t(meta.labelKey), turn: b.turn })}
                  style={{
                    position: 'absolute',
                    left: `calc(${pct}% - 6px)`,
                    top: 0,
                    width: 12,
                    height: 6,
                    padding: 0,
                    border: 'none',
                    background: meta.color,
                    cursor: 'pointer',
                    pointerEvents: 'auto',
                    borderRadius: 2,
                  }}
                />
              )
            })}
          </div>
        )}
      </div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginTop: 2 }}>
        <span className="t-xs muted-2">T1</span>
        <span className="t-xs muted-2">T{timeline[totalTurns - 1].turn}</span>
      </div>

      {/* Bookmark list — chronological, click-to-jump, ✕ to remove.
          Only renders when at least one bookmark exists so the empty
          state doesn't add chrome. */}
      {bookmarks.length > 0 && (
        <div style={{ marginTop: 8, display: 'flex', flexWrap: 'wrap', gap: 6 }}>
          <span className="t-xs muted" style={{ alignSelf: 'center' }}>{t('rpl.bookmarks')}</span>
          {bookmarks.map(b => {
            const meta = kindMeta(b.kind)
            const idx = idxAtTurn(b.turn)
            const isCurrent = idx === turnIdx
            return (
              <span
                key={b.id}
                style={{
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: 4,
                  padding: '2px 6px',
                  borderRadius: 3,
                  border: `1px solid ${meta.color}`,
                  background: isCurrent ? meta.color : 'transparent',
                  color: isCurrent ? 'var(--bg)' : 'var(--ink)',
                  fontSize: 11,
                  fontWeight: isCurrent ? 700 : 400,
                }}
              >
                <button
                  type="button"
                  onClick={() => { setPlaying(false); setTurnIdx(idx) }}
                  className="t-xs"
                  title={b.note || t(meta.labelKey)}
                  style={{
                    background: 'transparent',
                    border: 'none',
                    cursor: 'pointer',
                    padding: 0,
                    color: 'inherit',
                  }}
                >
                  {meta.icon} T{b.turn}{b.note ? ` · ${b.note}` : ''}
                </button>
                <button
                  type="button"
                  onClick={() => setBookmarks(list => removeBookmark(list, b.id))}
                  aria-label={t('rpl.aria_remove_bookmark', { kind: t(meta.labelKey), turn: b.turn })}
                  title={t('rpl.tip_remove_bookmark')}
                  style={{
                    background: 'transparent',
                    border: 'none',
                    cursor: 'pointer',
                    padding: '0 2px',
                    color: 'inherit',
                    fontSize: 10,
                  }}
                >
                  ✕
                </button>
              </span>
            )
          })}
        </div>
      )}

      <div className="hr" style={{ margin: '14px 0 10px' }} />

      {/* Per-seat board state at this turn */}
      <div className="grid col-4 gap-4">
        {(snap?.seats || []).map((s, i) => {
          const cmdr = commanders[i] || t('dash.unknown')
          const perms = s.battlefield || []
          const isActive = i === activeSeat
          const isFocused = i === focusedSeat
          const lifePct = Math.max(0, Math.min(100, (s.life / 40) * 100))
          const accent = isFocused ? 'var(--ok)'
            : s.lost ? 'var(--danger)'
            : isActive ? 'var(--accent)'
            : 'var(--rule-2)'
          return (
            <div
              key={i}
              className="panel"
              style={{
                padding: 0,
                borderColor: accent,
                borderWidth: isFocused ? 2 : undefined,
              }}
            >
              <div style={{ padding: '8px 10px' }}>
                <div
                  className="flex justify-between items-center"
                  style={{ marginBottom: 4, cursor: 'pointer' }}
                  onClick={() => setFocusedSeat(prev => prev === i ? null : i)}
                  role="button"
                  tabIndex={0}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault()
                      setFocusedSeat(prev => prev === i ? null : i)
                    }
                  }}
                  aria-pressed={isFocused}
                  aria-label={t(isFocused ? 'rpl.aria_unfocus_seat' : 'rpl.aria_focus_seat', { seat: i + 1 })}
                  title={t(isFocused ? 'rpl.tip_unfocus' : 'rpl.tip_focus')}
                >
                  <span className="t-xs muted">SEAT.{String(i + 1).padStart(2, '0')}</span>
                  {s.lost && <Tag kind="bad">{t('common.eliminated')}</Tag>}
                  {!s.lost && isActive && <Tag kind="ok" solid>{t('common.active')}</Tag>}
                  {isFocused && <Tag kind="ok">{t('rpl.focus_tag')}</Tag>}
                </div>
                <div className="t-md" style={{ fontWeight: 700, lineHeight: 1.2 }}>
                  {cmdr.split(',')[0].toUpperCase()}
                </div>
                <div className="hr" style={{ margin: '8px 0' }} />
                <div className="t-xs muted" style={{ marginBottom: 2 }}>{t('common.life')}</div>
                <Bar value={lifePct} />
                <div className="t-xs" style={{ marginTop: 3, fontWeight: 700, fontVariantNumeric: 'tabular-nums' }}>
                  {s.life} / 40
                  {s.lost && s.loss_reason && (
                    <span className="muted-2" style={{ marginLeft: 6, fontWeight: 400 }}>
                      · {s.loss_reason.replace(/_/g, ' ').toUpperCase()}
                    </span>
                  )}
                </div>
                <div className="hr" style={{ margin: '8px 0' }} />
                <KV rows={[
                  [t('common.hand'), String(s.hand_size)],
                  [t('common.library'), String(s.library_size)],
                  [t('common.graveyard'), String(s.gy_size)],
                  [t('common.battlefield'), String(perms.length)],
                ]} />
                {perms.length > 0 && (
                  <>
                    <div className="hr" style={{ margin: '8px 0' }} />
                    <div className="t-xs muted" style={{ marginBottom: 4 }}>{t('common.battlefield')}</div>
                    <div style={{ maxHeight: 140, overflowY: 'auto', borderTop: '1px dashed var(--rule-2)' }}>
                      {perms.map((p, j) => (
                        <div key={j} style={{
                          display: 'grid', gridTemplateColumns: '1fr auto',
                          padding: '3px 0', borderBottom: '1px dashed var(--rule-2)',
                          alignItems: 'center', gap: 6,
                        }}>
                          <span className="t-xs" style={{
                            fontWeight: p.is_commander ? 700 : 400,
                            color: p.is_commander ? 'var(--warn)' : 'var(--ink)',
                            overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                          }}>
                            {p.is_commander ? '★ ' : ''}{p.name?.toUpperCase()}
                          </span>
                          <span className="t-xs muted-2">
                            {p.power != null ? `${p.power}/${p.toughness ?? '?'}` : (p.type || '').slice(0, 4)}
                            {p.tapped ? ' ⤵' : ''}
                          </span>
                        </div>
                      ))}
                    </div>
                  </>
                )}
              </div>
            </div>
          )
        })}
      </div>

      {/* Compare mode: a parallel seat grid for a second game, locked
          to the same shared turn number. snapA / snapB come from
          mapSharedTurn(); ended-side renders a "GAME ENDED" badge
          rather than the stale last snapshot. */}
      {compareGame && (() => {
        const sharedTurn = turnAtIdx(turnIdx)
        const compareTimeline = compareGame?.timeline || []
        const { a: aMap, b: bMap } = mapSharedTurn(timeline, compareTimeline, sharedTurn)
        const deltas = compareSeatDeltas(aMap.snapshot, bMap.snapshot)
        const compareSeats = bMap.snapshot?.seats || []
        const compareActiveSeat = bMap.snapshot?.active_seat ?? -1
        const lastB = compareTimeline.length > 0 ? compareTimeline[compareTimeline.length - 1].turn : '?'
        return (
          <>
            <div className="hr" style={{ margin: '14px 0 10px' }} />
            <div className="flex justify-between items-center" style={{ marginBottom: 8 }}>
              <span className="t-xs muted" style={{ letterSpacing: '0.08em', fontWeight: 700 }}>
                {t('rpl.vs_game', { id: compareGame.game_id })} · {bMap.ended
                  ? t('rpl.vs_ended')
                  : bMap.beforeStart
                    ? t('rpl.vs_not_started')
                    : t('rpl.vs_turn', { turn: bMap.snapshot?.turn ?? '?', total: lastB })}
              </span>
              {onClearCompare && (
                <Btn sm arrow={null} onClick={onClearCompare} title={t('rpl.tip_close_compare')}>{t('rpl.close_compare')}</Btn>
              )}
            </div>
            <div className="grid col-4 gap-4">
              {compareSeats.map((s, i) => {
                const cmdr = compareCommanders[i] || t('dash.unknown')
                const perms = s.battlefield || []
                const isActive = i === compareActiveSeat
                const lifePct = Math.max(0, Math.min(100, (s.life / 40) * 100))
                const accent = s.lost ? 'var(--danger)' : isActive ? 'var(--accent)' : 'var(--rule-2)'
                const d = deltas[i]
                const fmtDelta = (v) => v === 0 ? '0' : v > 0 ? `+${v}` : String(v)
                const deltaColor = (v) => v === 0 ? 'var(--ink-3)' : v > 0 ? 'var(--ok)' : 'var(--danger)'
                return (
                  <div key={i} className="panel" style={{ padding: 0, borderColor: accent }}>
                    <div style={{ padding: '8px 10px' }}>
                      <div className="flex justify-between items-center" style={{ marginBottom: 4 }}>
                        <span className="t-xs muted">SEAT.{String(i + 1).padStart(2, '0')}</span>
                        {s.lost && <Tag kind="bad">{t('common.eliminated')}</Tag>}
                        {!s.lost && isActive && <Tag kind="ok" solid>{t('common.active')}</Tag>}
                      </div>
                      <div className="t-md" style={{ fontWeight: 700, lineHeight: 1.2 }}>
                        {cmdr.split(',')[0].toUpperCase()}
                      </div>
                      <div className="hr" style={{ margin: '8px 0' }} />
                      <div className="t-xs muted" style={{ marginBottom: 2 }}>{t('common.life')}</div>
                      <Bar value={lifePct} />
                      <div className="t-xs" style={{ marginTop: 3, fontWeight: 700, fontVariantNumeric: 'tabular-nums' }}>
                        {s.life} / 40
                        {d && (
                          <span style={{ marginLeft: 6, color: deltaColor(d.lifeDelta), fontWeight: 400 }}>
                            ({fmtDelta(-d.lifeDelta)} {t('rpl.vs_a')})
                          </span>
                        )}
                      </div>
                      <div className="hr" style={{ margin: '8px 0' }} />
                      <KV rows={[
                        [t('common.hand'),        `${s.hand_size}`    + (d ? `  (${fmtDelta(-d.handDelta)})` : '')],
                        [t('common.library'),     `${s.library_size}` + (d ? `  (${fmtDelta(-d.librarayDelta)})` : '')],
                        [t('common.graveyard'),   `${s.gy_size}`      + (d ? `  (${fmtDelta(-d.graveyardDelta)})` : '')],
                        [t('common.battlefield'), `${perms.length}`   + (d ? `  (${fmtDelta(-d.battlefieldDelta)})` : '')],
                      ]} />
                    </div>
                  </div>
                )
              })}
              {compareSeats.length === 0 && (
                <div className="t-xs muted-2" style={{ gridColumn: '1 / -1', padding: '12px 4px' }}>
                  &gt; {t(bMap.beforeStart ? 'rpl.compare_not_started' : 'rpl.compare_ended')}
                </div>
              )}
            </div>
          </>
        )
      })()}

      {/* Event log for this turn */}
      <div className="hr" style={{ margin: '14px 0 8px' }} />
      <div className="t-xs muted" style={{ marginBottom: 6 }}>{t('rpl.events_turn', { turn: turnNo })}</div>
      {events.length === 0 ? (
        <div className="t-xs muted-2">{t('rpl.no_events')}</div>
      ) : (
        <div style={{ maxHeight: 220, overflowY: 'auto' }}>
          {events.map((ev, i) => (
            <div key={i} style={{
              display: 'grid', gridTemplateColumns: '54px 1fr 80px',
              padding: '4px 0', borderBottom: '1px dashed var(--rule-2)',
              alignItems: 'baseline', gap: 8,
            }}>
              <span className="t-xs muted-2">T{ev.turn}</span>
              <span className="t-xs" style={{ lineHeight: 1.35 }}>{ev.action}</span>
              <span className="t-xs muted text-right" style={{ textTransform: 'uppercase' }}>{ev.kind}</span>
            </div>
          ))}
        </div>
      )}
    </Panel>
  )
}

/* ─── Deck Context Selector (top bar) ────────────────────────── */
const DeckSelector = ({ commanders, selected, onSelect }) => {
  const t = useT()
  // Deduplicate commander names from all games
  const unique = [...new Set(commanders)].sort()
  if (unique.length === 0) return null

  return (
    <div style={{
      display: 'flex', gap: 6, alignItems: 'center', flexWrap: 'wrap',
      padding: '10px 18px', borderBottom: '1px solid var(--rule-2)',
    }}>
      <span className="t-xs muted" style={{ marginRight: 4 }}>{t('rpt.deck_context')}</span>
      <Tag solid={!selected} onClick={() => onSelect(null)} style={{ cursor: 'pointer' }}>{t('common.all')}</Tag>
      {unique.map(c => (
        <Tag
          key={c}
          solid={selected === c}
          onClick={() => onSelect(selected === c ? null : c)}
          style={{ cursor: 'pointer' }}
        >
          {c.split(',')[0].toUpperCase()}
        </Tag>
      ))}
    </div>
  )
}

/* ─── Per-Deck Stats Panel ───────────────────────────────────── */
const DeckStatsPanel = ({ games, elo, selectedDeck }) => {
  const t = useT()
  if (!selectedDeck) return null

  // Find ELO entry for this commander
  const eloEntry = elo.find(e =>
    e.commander?.toLowerCase() === selectedDeck.toLowerCase() ||
    e.commander?.toLowerCase().startsWith(selectedDeck.split(',')[0].trim().toLowerCase())
  )

  // Calculate stats from filtered games
  const wins = games.filter(g => g.winner >= 0 && g.commanders?.[g.winner]?.toLowerCase() === selectedDeck.toLowerCase()).length
  const totalFiltered = games.length
  const losses = totalFiltered - wins
  const avgTurns = totalFiltered > 0
    ? Math.round(games.reduce((sum, g) => sum + (g.turns || 0), 0) / totalFiltered * 10) / 10
    : 0
  const wr = totalFiltered > 0 ? Math.round(wins / totalFiltered * 1000) / 10 : 0

  return (
    <div className="panel" style={{ padding: 0, gridColumn: '1 / -1' }}>
      <div className="panel-hd">
        <span>{t('rpt.deck_stats', { name: selectedDeck.split(',')[0].toUpperCase() })}</span>
        <span>{t(totalFiltered === 1 ? 'rpt.games_count_one' : 'rpt.games_count', { count: totalFiltered })}</span>
      </div>
      <div className="deck-stats-row" style={{ padding: '14px 22px' }}>
        <div>
          <div className="t-xs muted">{t('rpt.games')}</div>
          <div className="t-2xl" style={{ fontWeight: 800, marginTop: 4 }}>{totalFiltered}</div>
        </div>
        <div>
          <div className="t-xs muted">{t('rpt.wins')}</div>
          <div className="t-2xl" style={{ fontWeight: 800, marginTop: 4, color: 'var(--ok)' }}>{wins}</div>
        </div>
        <div>
          <div className="t-xs muted">{t('rpt.losses')}</div>
          <div className="t-2xl" style={{ fontWeight: 800, marginTop: 4, color: 'var(--danger)' }}>{losses}</div>
        </div>
        <div>
          <div className="t-xs muted">{t('rpt.win_rate')}</div>
          <div className="t-2xl" style={{ fontWeight: 800, marginTop: 4 }}>{wr}%</div>
        </div>
        <div>
          <div className="t-xs muted">{t('rpt.avg_turns')}</div>
          <div className="t-2xl" style={{ fontWeight: 800, marginTop: 4 }}>{avgTurns}</div>
        </div>
        <div>
          <div className="t-xs muted">{t('rpt.elo')}</div>
          <div className="t-2xl" style={{ fontWeight: 800, marginTop: 4 }}>
            {eloEntry ? Math.round(eloEntry.rating) : '—'}
            {eloEntry && (
              <span className="t-xs" style={{ marginLeft: 6, color: eloEntry.delta >= 0 ? 'var(--ok)' : 'var(--danger)' }}>
                {eloEntry.delta >= 0 ? '+' : ''}{eloEntry.delta}
              </span>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

export default function Report() {
  const t = useT()
  const { gameId } = useParams()
  const [searchParams, setSearchParams] = useSearchParams()
  const [game, setGame] = useState(null)
  const [games, setGames] = useState([])
  const [elo, setElo] = useState([])
  const [selectedDeck, setSelectedDeck] = useState(null)
  const [loading, setLoading] = useState(true)
  // Compare mode. compareGame is the second game's full report (or
  // null when compare is off). compareId mirrors `?vs=<id>` and
  // drives the fetch; clearing it removes the second panel.
  const compareId = searchParams.get(SHARE_PARAM_COMPARE)
  const [compareGame, setCompareGame] = useState(null)
  const [comparePickerOpen, setComparePickerOpen] = useState(false)

  useEffect(() => {
    let cancelled = false
    // External-sync effect: react to URL ?vs= changes. The setState
    // calls are guarded so we only update when the requested compare
    // game actually differs from what's loaded.
    /* eslint-disable react-hooks/set-state-in-effect */
    if (!compareId) {
      setCompareGame(null)
      return
    }
    if (String(compareId) === String(gameId || game?.game_id)) {
      // Don't try to compare against ourselves.
      setCompareGame(null)
      return
    }
    api.getGameReport(compareId).then(g => {
      if (!cancelled) setCompareGame(g)
    }).catch(() => {
      if (!cancelled) setCompareGame(null)
    })
    /* eslint-enable react-hooks/set-state-in-effect */
    return () => { cancelled = true }
  }, [compareId, gameId, game?.game_id])

  useEffect(() => {
    const load = async () => {
      try {
        // Fetch ELO data
        api.getLiveELO().then(setElo).catch(() => {})

        if (gameId) {
          // /report carries the per-turn Timeline used by the
          // scrubber; the plain /games/{id} endpoint strips it.
          const g = await api.getGameReport(gameId)
          setGame(g)
        } else {
          const list = await api.getGames(1)
          if (list?.length > 0) {
            // Hydrate the most-recent game with its full report so
            // the scrubber works on the default "no gameId" view too.
            try {
              const full = await api.getGameReport(list[0].game_id)
              setGame(full || list[0])
            } catch {
              setGame(list[0])
            }
          }
        }
        const full = await api.getGames(50)
        setGames(full || [])
      } catch (err) {
        console.warn('Report load failed:', err.message)
      } finally {
        setLoading(false)
      }
    }
    load()
  }, [gameId])

  if (loading) {
    return (
      <>
        <Tape left={t('rpt.tape_left')} mid={t('common.loading')} right="" />
        <div style={{ padding: 36, textAlign: 'center' }}>
          <div className="t-md muted">&gt; {t('rpt.loading')}<span className="blink">_</span></div>
        </div>
      </>
    )
  }

  // Collect all unique commander names across games
  const allCommanders = []
  for (const g of games) {
    if (g.commanders) {
      for (const c of g.commanders) {
        if (c && !allCommanders.includes(c)) allCommanders.push(c)
      }
    }
  }

  // Filter games by selected deck
  const filteredGames = selectedDeck
    ? games.filter(g => g.commanders?.some(c => c?.toLowerCase() === selectedDeck.toLowerCase()))
    : games

  if (!game && filteredGames.length === 0) {
    return (
      <>
        <Tape left={t('rpt.tape_left')} mid={t('gs.no_data')} right="" />
        <DeckSelector commanders={allCommanders} selected={selectedDeck} onSelect={setSelectedDeck} />
        <div style={{ padding: 36, textAlign: 'center' }}>
          <div className="t-md muted">&gt; {t('rpt.no_games')}<span className="blink">_</span></div>
        </div>
      </>
    )
  }

  // Use the first filtered game as the featured game if no specific gameId
  const featuredGame = gameId ? game : (filteredGames[0] || game)

  const commanders = featuredGame?.commanders || []
  const seats = featuredGame?.final_seats || []
  const winnerIdx = featuredGame?.winner
  const winnerName = featuredGame?.winner_name || t('gs.draw')
  const isVictory = winnerIdx >= 0

  return (
    <>
      <Tape
        left={featuredGame ? t('rpt.tape_game', { id: featuredGame.game_id }) : t('rpt.tape_left')}
        mid={isVictory ? t('rpt.victory') : t('gs.draw')}
        right={featuredGame?.finished_at ? new Date(featuredGame.finished_at).toLocaleString().toUpperCase() : ''}
      />

      {/* Deck context selector */}
      <DeckSelector commanders={allCommanders} selected={selectedDeck} onSelect={setSelectedDeck} />

      <div className="report-grid">
        {/* Per-deck stats if a deck is selected */}
        <DeckStatsPanel games={filteredGames} elo={elo} selectedDeck={selectedDeck} />

        {/* Result block */}
        {featuredGame && (
          <div className="panel" style={{ padding: 0, gridColumn: '1 / -1' }}>
            <div className="panel-hd">
              <span>{t('rpt.result_block')}</span>
              <span style={{ display: 'inline-flex', alignItems: 'center', gap: 12 }}>
                <SummaryActions gameId={featuredGame.game_id} />
                <span>GAME.{featuredGame.game_id}</span>
              </span>
            </div>
            <div className="result-block-grid" style={{ padding: '18px 22px' }}>
              <div>
                <div className="t-xs muted">{t('rpt.outcome')}</div>
                <div className="t-3xl" style={{ color: isVictory ? 'var(--ok)' : 'var(--warn)', fontWeight: 800 }}>
                  {isVictory ? t('rpt.victory') : t('gs.draw')}
                </div>
                <div className="t-md muted" style={{ marginTop: 6 }}>
                  {t('rpt.turn_reason', { turn: featuredGame.turns, reason: (featuredGame.end_reason || '').replace(/_/g, ' ').toUpperCase() })}
                </div>
                <div className="t-xs muted-2" style={{ marginTop: 4 }}>
                  {t('rpt.winner_is', { name: winnerName.toUpperCase() })}
                </div>
              </div>
              <Stat2 k={t('rpt.turns')} v={String(featuredGame.turns)} />
              <Stat2 k={t('rpt.players')} v={String(commanders.length)} big />
              <Stat2 k={t('rpt.end')} v={endReasonShort(t, featuredGame.end_reason)} />
            </div>
          </div>
        )}

        {/* Replay scrubber — per-turn timeline with board/life/event delta */}
        {featuredGame && (
          <div style={{ gridColumn: '1 / -1' }}>
            {/* Compare picker — shows when not yet comparing. Lists
                the 8 most-recent other games; click one to start
                compare. Writes `?vs=<id>` to the URL. */}
            {!compareGame && (
              <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 8 }}>
                <Btn
                  sm
                  arrow={null}
                  onClick={() => setComparePickerOpen(v => !v)}
                  title={t('rpt.tip_compare')}
                >
                  {comparePickerOpen ? t('rpt.compare_close') : t('rpt.compare_open')}
                </Btn>
                {comparePickerOpen && (
                  <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4, alignItems: 'center' }}>
                    <span className="t-xs muted">{t('rpt.pick_game')}</span>
                    {games
                      .filter(g => g.game_id !== featuredGame.game_id)
                      .slice(0, 8)
                      .map(g => (
                        <button
                          key={g.game_id}
                          type="button"
                          className="t-xs"
                          onClick={() => {
                            const next = new URLSearchParams(searchParams)
                            next.set(SHARE_PARAM_COMPARE, String(g.game_id))
                            setSearchParams(next, { replace: true })
                            setComparePickerOpen(false)
                          }}
                          style={{
                            background: 'transparent',
                            border: '1px solid var(--rule-2)',
                            padding: '2px 6px',
                            cursor: 'pointer',
                            color: 'var(--ink)',
                          }}
                        >
                          #{g.game_id} · {(g.commanders?.[g.winner]?.split(',')[0] || t('gs.draw')).toUpperCase()}
                        </button>
                      ))}
                  </div>
                )}
              </div>
            )}
            <ReplayScrubber
              game={featuredGame}
              commanders={commanders}
              compareGame={compareGame}
              compareCommanders={compareGame?.commanders || []}
              onClearCompare={() => {
                const next = new URLSearchParams(searchParams)
                next.delete(SHARE_PARAM_COMPARE)
                setSearchParams(next, { replace: true })
              }}
            />
          </div>
        )}

        {/* Final board state — all seats */}
        {featuredGame && (
          <div className="panel" style={{ gridColumn: '1 / -1', padding: 0 }}>
            <div className="panel-hd"><span>{t('rpt.final_board')}</span><span>{t(commanders.length === 1 ? 'rpt.seats_one' : 'rpt.seats', { count: commanders.length })}</span></div>
            <div style={{ padding: '18px 22px' }}>
              <div className="grid col-4 gap-4">
                {seats.map((s, i) => {
                  const isWinner = i === winnerIdx
                  const cmdr = commanders[i] || t('dash.unknown')
                  const perms = s.battlefield || []
                  const artUrl = cardArtUrl(cmdr)

                  return (
                    <div key={i} className="panel" style={{ padding: 0, borderColor: isWinner ? 'var(--ok)' : s.lost ? 'var(--danger)' : 'var(--rule-2)' }}>
                      {artUrl && (
                        <div style={{
                          height: 80,
                          backgroundImage: `url(${artUrl})`,
                          backgroundSize: 'cover', backgroundPosition: 'center',
                          borderBottom: '1px solid var(--rule-2)',
                          opacity: s.lost && !isWinner ? 0.3 : 0.8,
                        }} />
                      )}
                      <div style={{ padding: '8px 10px' }}>
                        <div className="flex justify-between items-center" style={{ marginBottom: 4 }}>
                          <span className="t-xs muted">SEAT.{String(i + 1).padStart(2, '0')}</span>
                          {isWinner && <Tag kind="ok" solid>{t('common.winner')}</Tag>}
                          {s.lost && !isWinner && <Tag kind="bad">{t('common.eliminated')}</Tag>}
                          {!s.lost && !isWinner && <Tag>{t('common.alive')}</Tag>}
                        </div>
                        <div className="t-md" style={{ fontWeight: 700, lineHeight: 1.2 }}>
                          {cmdr.toUpperCase()}
                        </div>
                        <div className="hr" style={{ margin: '8px 0' }} />
                        <KV rows={[
                          [t('common.life'), String(s.life)],
                          [t('common.hand'), String(s.hand_size)],
                          [t('common.library'), String(s.library_size)],
                          [t('common.graveyard'), String(s.gy_size)],
                          [t('common.battlefield'), String(perms.length)],
                        ]} />
                      </div>
                    </div>
                  )
                })}
              </div>
            </div>
          </div>
        )}

        {/* Battlefield breakdown for winner */}
        {featuredGame && isVictory && seats[winnerIdx] && (
          <Panel code="RPT.A" title={t('rpt.winner_bf', { name: winnerName.toUpperCase() })}>
            <div className="t-xs muted" style={{ marginBottom: 6 }}>{t('rpt.perms_at_end')}</div>
            {(seats[winnerIdx].battlefield || []).length === 0 ? (
              <div className="t-xs muted-2">{t('rpt.no_perms')}</div>
            ) : (
              (seats[winnerIdx].battlefield || []).map((p, i) => (
                <div key={i} style={{ display: 'grid', gridTemplateColumns: '1fr 60px 60px', padding: '4px 0', borderBottom: '1px dashed var(--rule-2)', alignItems: 'center', gap: 8 }}>
                  <span className="t-xs" style={{ fontWeight: p.is_commander ? 700 : 400, color: p.is_commander ? 'var(--warn)' : 'var(--ink)' }}>
                    {p.is_commander ? '* ' : ''}{p.name?.toUpperCase()}
                  </span>
                  <span className="t-xs muted">{p.type || '—'}</span>
                  <span className="t-xs muted text-right">{t(p.tapped ? 'common.tapped' : 'common.untapped')}</span>
                </div>
              ))
            )}
          </Panel>
        )}

        {/* Game history list (filtered by deck) */}
        {filteredGames.length > 0 && (
          <Panel code="RPT.B" title={selectedDeck
            ? t(filteredGames.length === 1 ? 'rpt.game_log_deck_one' : 'rpt.game_log_deck',
                { deck: selectedDeck.split(',')[0].toUpperCase(), count: filteredGames.length })
            : t(filteredGames.length === 1 ? 'rpt.game_log_one' : 'rpt.game_log', { count: filteredGames.length })}
            style={featuredGame && isVictory && seats[winnerIdx] ? {} : { gridColumn: '1 / -1' }}
          >
            <div>
              {filteredGames.map((g, i) => {
                const isWin = g.winner >= 0
                const winnerCmdr = isWin && g.commanders?.[g.winner] ? g.commanders[g.winner] : null
                // If filtering by deck, highlight if the selected deck won
                const deckWon = selectedDeck && winnerCmdr?.toLowerCase() === selectedDeck.toLowerCase()

                return (
                  <div key={i} style={{
                    display: 'grid', gridTemplateColumns: '50px 1fr 80px 60px',
                    padding: '8px 0',
                    borderBottom: i < filteredGames.length - 1 ? '1px dashed var(--rule-2)' : 'none',
                    alignItems: 'center', gap: 8,
                    opacity: selectedDeck && !deckWon && isWin ? 0.5 : 1,
                  }}>
                    <span className="t-xs muted-2">#{g.game_id}</span>
                    <div>
                      <div className="t-md" style={{ fontWeight: 600 }}>{g.winner_name?.toUpperCase() || t('gs.draw')}</div>
                      <div className="t-xs muted">T{g.turns} · {(g.end_reason || '').replace(/_/g, ' ')}</div>
                    </div>
                    {selectedDeck ? (
                      <Tag kind={deckWon ? 'ok' : isWin ? 'bad' : 'warn'} solid>
                        {deckWon ? t('dash.result_win') : isWin ? t('rpt.loss') : t('gs.draw')}
                      </Tag>
                    ) : (
                      <Tag kind={isWin ? 'ok' : 'warn'} solid>{isWin ? t('dash.result_win') : t('gs.draw')}</Tag>
                    )}
                    <span className="t-xs muted text-right">{t('rpt.players_short', { count: g.commanders?.length || 0 })}</span>
                  </div>
                )
              })}
            </div>
          </Panel>
        )}

        {/* ── ANALYSIS BLOCK ──
           Four sections derived from CompletedGame: win condition (real),
           timeline (mock), MVP card (heuristic), per-seat performance
           (final stats real, life curve mock). Mocked sections wear a
           tag so reviewers know what's synthetic. */}

        {/* Win condition analysis — derived from end_reason. Real data. */}
        {featuredGame && isVictory && (() => {
          const wc = classifyWinCondition(featuredGame.end_reason)
          const eliminated = seats
            .map((s, i) => ({ s, i, name: commanders[i] }))
            .filter(x => x.s.lost && x.i !== winnerIdx)
          return (
            <Panel code="RPT.C" title={t('rpt.win_condition')} style={{ gridColumn: '1 / -1' }}
              right={<Tag solid kind="ok">{wcLabel(t, wc)}</Tag>}>
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1.6fr', gap: 22, padding: '4px 0' }}>
                <div>
                  <div className="t-xs muted">{t('rpt.resolution')}</div>
                  <div className="t-2xl" style={{ fontWeight: 800, marginTop: 4, color: 'var(--ok)' }}>{wcLabel(t, wc)}</div>
                  <div className="t-md muted" style={{ marginTop: 8, lineHeight: 1.55, textTransform: 'uppercase', letterSpacing: '0.04em' }}>
                    &gt; {t(wc.detailKey)}
                    <br />
                    &gt; {t('rpt.cr_line', { code: (featuredGame.end_reason || '').replace(/_/g, '.'), turn: featuredGame.turns })}
                    <br />
                    &gt; {t('rpt.winner_is', { name: winnerName.toUpperCase() })}
                  </div>
                </div>
                <div>
                  <div className="t-xs muted" style={{ marginBottom: 6 }}>{t('rpt.elim_seats')}</div>
                  {eliminated.length === 0 ? (
                    <div className="t-xs muted-2">{t('rpt.none')} </div>
                  ) : eliminated.map(x => (
                    <div key={x.i} style={{ display: 'grid', gridTemplateColumns: '1fr auto', padding: '5px 0', borderBottom: '1px dashed var(--rule-2)', alignItems: 'center', gap: 8 }}>
                      <div>
                        <div className="t-md" style={{ fontWeight: 700 }}>{x.name?.toUpperCase()}</div>
                        <div className="t-xs muted">SEAT.{String(x.i + 1).padStart(2, '0')} · {(x.s.loss_reason || x.s.LossReason)
                          ? (x.s.loss_reason || x.s.LossReason).replace(/_/g, ' ').toUpperCase()
                          : t('rpt.state_based')}</div>
                      </div>
                      <Tag kind="bad">{t('rpt.elim')}</Tag>
                    </div>
                  ))}
                </div>
              </div>
            </Panel>
          )
        })()}

        {/* MVP card — heuristic pick from winner's battlefield. */}
        {featuredGame && isVictory && (() => {
          const mvp = pickMVP(t, seats[winnerIdx])
          if (!mvp) return null
          const art = cardArtUrl(mvp.perm.name)
          return (
            <Panel code="RPT.D" title={t('rpt.mvp_card')} right={<HeuristicTag />}>
              <div style={{ display: 'flex', gap: 14, alignItems: 'flex-start' }}>
                <div style={{ width: 96, aspectRatio: '5/7', flexShrink: 0, border: '1px solid var(--rule-2)', overflow: 'hidden', background: 'var(--bg-2)' }}
                  className={art ? '' : 'hatch'}>
                  {art && <img src={art} alt={mvp.perm.name} style={{ width: '100%', height: '100%', objectFit: 'cover', filter: 'saturate(0.7) contrast(1.05)' }} onError={e => { e.target.style.display = 'none'; e.target.parentElement.classList.add('hatch') }} />}
                </div>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div className="t-xs muted">{t('rpt.card')}</div>
                  <div className="t-xl" style={{ fontWeight: 800, marginTop: 2, lineHeight: 1.1 }}>
                    {mvp.perm.is_commander && <span style={{ color: 'var(--warn)' }}>★ </span>}
                    {mvp.perm.name?.toUpperCase()}
                  </div>
                  <div className="t-xs muted" style={{ marginTop: 4 }}>
                    {(mvp.perm.type || t('rpt.permanent')).toUpperCase()}
                    {mvp.perm.power != null ? ` · ${mvp.perm.power}/${mvp.perm.toughness ?? '?'}` : ''}
                  </div>
                  <div className="hr" style={{ margin: '10px 0' }} />
                  <div className="t-xs muted">{t('rpt.selection_reason')}</div>
                  <div className="t-md" style={{ marginTop: 4, lineHeight: 1.5, textTransform: 'uppercase', letterSpacing: '0.04em' }}>
                    &gt; {mvp.reason}
                  </div>
                  <div className="t-xs muted-2" style={{ marginTop: 8, lineHeight: 1.5 }}>
                    {t('rpt.mvp_note')}
                  </div>
                </div>
              </div>
            </Panel>
          )
        })()}

        {/* Game timeline — derived strictly from CompletedGame fields
            (open, eliminations w/ loss_reason, finishing line). Per-turn
            log[] isn't retained server-side yet, so mid-game eliminations
            are anchored without a turn number rather than fabricated. */}
        {featuredGame && (() => {
          const timeline = deriveTimeline(t, featuredGame, commanders)
          return (
            <Panel code="RPT.E" title={t('rpt.game_timeline')}
              style={{ gridColumn: featuredGame && isVictory ? 'auto' : '1 / -1' }}>
              <div style={{ position: 'relative', paddingLeft: 18 }}>
                <div style={{ position: 'absolute', left: 6, top: 4, bottom: 4, width: 1, background: 'var(--rule-2)' }} />
                {timeline.map((entry, i) => {
                  const cmdrName = entry.seat >= 0 ? (commanders[entry.seat] || '') : ''
                  const seatColor = entry.kind === 'win' ? 'var(--ok)'
                    : entry.kind === 'eliminated' ? 'var(--danger)'
                    : entry.kind === 'draw' ? 'var(--warn)'
                    : 'var(--ink-2)'
                  return (
                    <div key={i} style={{ display: 'grid', gridTemplateColumns: '52px 1fr', gap: 10, padding: '8px 0', borderBottom: i < timeline.length - 1 ? '1px dashed var(--rule-2)' : 'none', alignItems: 'flex-start' }}>
                      <span className="t-xs" style={{ fontWeight: 800, color: entry.turn != null ? 'var(--accent)' : 'var(--ink-3)', position: 'relative' }}>
                        <span style={{ position: 'absolute', left: -16, top: 4, width: 9, height: 9, borderRadius: '50%', background: seatColor, border: '2px solid var(--bg)' }} />
                        {entry.turn != null ? `T${entry.turn}` : '— —'}
                      </span>
                      <div>
                        <div className="t-md" style={{ fontWeight: 600, lineHeight: 1.3 }}>{entry.action}</div>
                        {entry.detail && (
                          <div className="t-xs muted" style={{ marginTop: 2 }}>
                            {entry.seat >= 0 ? `SEAT.${String(entry.seat + 1).padStart(2, '0')} · ${(cmdrName.split(',')[0] || '').toUpperCase()} · ` : ''}
                            {entry.detail}
                          </div>
                        )}
                      </div>
                    </div>
                  )
                })}
                <div className="t-xs muted-2" style={{ marginTop: 10, lineHeight: 1.5 }}>
                  &gt; {t('rpt.timeline_note')}
                </div>
              </div>
            </Panel>
          )
        })()}

        {/* Per-seat performance — final-state stats only. The life-over-
            time sparkline was a fabricated curve; dropped until the
            engine retains a per-turn life_history field. */}
        {featuredGame && (
          <Panel code="RPT.F" title={t('rpt.per_seat')} style={{ gridColumn: '1 / -1' }}>
            <div className="grid col-2 gap-4">
              {seats.map((s, i) => {
                const isWinner = i === winnerIdx
                const cmdr = commanders[i] || t('dash.unknown')
                const perms = s.battlefield || []
                const lands = perms.filter(p => p.is_land).length
                const nonLand = perms.length - lands
                const creatures = perms.filter(p => (p.type || '').toLowerCase().includes('creature') || p.power != null).length
                const totalCardsKnown = perms.length + (s.hand_size || 0) + (s.library_size || 0) + (s.gy_size || 0)
                const lifePct = Math.max(0, Math.min(100, (s.life / 40) * 100))
                const accent = isWinner ? 'var(--ok)' : s.lost ? 'var(--danger)' : 'var(--ink-2)'
                return (
                  <div key={i} className="panel" style={{ padding: 0, borderColor: accent }}>
                    <div className="panel-hd">
                      <span>{cmdr.split(',')[0].toUpperCase()}</span>
                      <span className="t-xs">
                        SEAT.{String(i + 1).padStart(2, '0')}
                        {isWinner && <span style={{ color: 'var(--ok)', marginLeft: 6 }}>{t('rpt.winner_mark')}</span>}
                        {s.lost && !isWinner && <span style={{ color: 'var(--danger)', marginLeft: 6 }}>{t('rpt.lost_mark')}</span>}
                      </span>
                    </div>
                    <div style={{ padding: 12 }}>
                      <div className="t-xs muted" style={{ marginBottom: 2 }}>{t('rpt.final_life')}</div>
                      <Bar value={lifePct} lg />
                      <div className="t-xs" style={{ marginTop: 3, fontWeight: 700, color: accent, fontVariantNumeric: 'tabular-nums' }}>
                        {s.life} / 40
                        {s.lost && s.loss_reason && (
                          <span className="muted-2" style={{ marginLeft: 8, fontWeight: 400 }}>
                            · {s.loss_reason.replace(/_/g, ' ').toUpperCase()}
                          </span>
                        )}
                      </div>
                      <div className="hr" style={{ margin: '10px 0' }} />
                      <KV rows={[
                        [t('common.battlefield'), String(perms.length)],
                        [t('rpt.creatures'), String(creatures)],
                        [t('rpt.nonland'), String(nonLand)],
                        [t('rpt.lands'), String(lands)],
                        [t('common.hand'), String(s.hand_size)],
                        [t('common.library'), String(s.library_size)],
                        [t('common.graveyard'), String(s.gy_size)],
                        [t('rpt.zone_total'), String(totalCardsKnown)],
                      ]} />
                    </div>
                  </div>
                )
              })}
            </div>
            <div className="t-xs muted-2" style={{ marginTop: 10, lineHeight: 1.5 }}>
              &gt; {t('rpt.seat_note')}
            </div>
          </Panel>
        )}
      </div>
    </>
  )
}
