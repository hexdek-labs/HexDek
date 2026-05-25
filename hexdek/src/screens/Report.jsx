import { useState, useEffect } from 'react'
import { useParams } from 'react-router-dom'
import { Panel, KV, Bar, Tag, Btn, Tape } from '../components/chrome'
import SummaryActions from '../components/SummaryActions'
import { api, cardArtUrl } from '../services/api'

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

const HeuristicTag = () => (
  <Tag kind="warn" style={{ fontSize: 8, padding: '1px 5px' }}>HEURISTIC</Tag>
)

// classifyWinCondition derives a human-readable win-condition label from
// CompletedGame.end_reason. Honest mapping; nothing mocked here.
const WIN_CONDITION_MAP = {
  sba_704_5a: { label: 'COMBAT / BURN DAMAGE', detail: 'OPPONENTS REDUCED TO 0 LIFE', kind: 'damage' },
  sba_704_5b: { label: 'MILL — DECKED OUT', detail: 'OPPONENTS DREW FROM EMPTY LIBRARY', kind: 'mill' },
  sba_704_5c: { label: 'INFECT / POISON', detail: '10+ POISON COUNTERS DELIVERED', kind: 'poison' },
  sba_704_5d: { label: 'COMMANDER DAMAGE', detail: '21+ DAMAGE FROM A SINGLE COMMANDER', kind: 'commander_damage' },
  concession: { label: 'SCOOP / CONCESSION', detail: 'OPPONENT CONCEDED', kind: 'concession' },
  turn_limit: { label: 'TURN LIMIT REACHED', detail: 'GAME ENDED ON SCHEDULED CAP', kind: 'stall' },
}
const classifyWinCondition = (endReason) => {
  if (!endReason) return { label: 'UNKNOWN', detail: 'NO END REASON RECORDED', kind: 'unknown' }
  const key = endReason.toLowerCase()
  return WIN_CONDITION_MAP[key] || {
    label: endReason.replace(/_/g, ' ').toUpperCase(),
    detail: 'STATE-BASED ELIMINATION',
    kind: 'sba',
  }
}

// Short label for the END stat in the result block — the column is
// narrow (1fr of 4 in result-block-grid) and the raw replacement
// produces strings like "LAST SEAT STANDING" that overflow.
const END_REASON_SHORT = {
  last_seat_standing: 'LAST SEAT',
  sba_704_5a: 'DAMAGE',
  sba_704_5b: 'MILL',
  sba_704_5c: 'POISON',
  sba_704_5d: 'CMDR DMG',
  concession: 'CONCEDE',
  turn_limit: 'TURN CAP',
}
const endReasonShort = (r) => {
  if (!r) return '?'
  return END_REASON_SHORT[r.toLowerCase()] || r.replace(/_/g, ' ').toUpperCase()
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
const pickMVP = (winnerSeat) => {
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
  // Build a reason string from the dominant signal.
  let reason
  if (best.is_commander) {
    reason = 'COMMANDER ALIVE AT VICTORY — PRIMARY THREAT VECTOR'
  } else if (best.power != null && best.power >= 5) {
    reason = `${best.power}/${best.toughness ?? '?'} CREATURE — HIGHEST POWER ON BOARD`
  } else if (best.counters && Object.values(best.counters).some(v => v >= 3)) {
    const total = Object.values(best.counters).reduce((a, b) => a + (typeof b === 'number' ? b : 0), 0)
    reason = `LOADED WITH ${total} COUNTER${total === 1 ? '' : 'S'} — VALUE ENGINE`
  } else if (!best.is_land) {
    reason = 'TOP-RANKED NON-LAND PERMANENT BY COMPOSITE SCORE'
  } else {
    reason = 'ONLY SURVIVING PERMANENT ON FINAL BOARD'
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
const deriveTimeline = (game, commanders) => {
  if (!game) return []
  const total = Math.max(1, game.turns || 1)
  const winner = game.winner ?? -1
  const winName = winner >= 0 ? (commanders[winner] || 'WINNER').split(',')[0].toUpperCase() : null
  const seats = game.final_seats || []
  const cond = classifyWinCondition(game.end_reason)

  const out = []
  // Game start — every game has one.
  out.push({
    turn: 1,
    seat: -1,
    kind: 'open',
    action: `GAME OPENS · ${commanders.length} SEATS`,
    detail: commanders.map(c => (c || 'UNKNOWN').split(',')[0].toUpperCase()).join(' · '),
  })

  // Eliminations — collected from final_seats. Without per-turn data
  // we group them as a single mid-game block; loss_reason is real.
  const losers = seats
    .map((s, i) => ({ s, i }))
    .filter(x => x.s.lost && x.i !== winner)
  for (const { s, i } of losers) {
    const cmdrShort = (commanders[i] || 'UNKNOWN').split(',')[0].toUpperCase()
    const reason = (s.loss_reason || s.LossReason || '').replace(/_/g, ' ').toUpperCase().trim()
    out.push({
      turn: null, // unknown — we don't have per-elimination turn data
      seat: i,
      kind: 'eliminated',
      action: `${cmdrShort} ELIMINATED`,
      detail: reason ? `LOSS: ${reason}` : 'LOSS: STATE-BASED',
    })
  }

  // Finishing line — anchored at the recorded turn count.
  if (winner >= 0) {
    const finishMap = {
      damage:            `${winName} CLOSES OUT — LETHAL DAMAGE`,
      mill:              `${winName} DECKS OUT FINAL OPPONENT`,
      poison:            `${winName} HITS 10+ POISON ON FINAL TARGET`,
      commander_damage:  `${winName} DEALS 21+ COMMANDER DAMAGE`,
      concession:        `${winName} ACCEPTS CONCESSION`,
      stall:             `GAME HITS TURN-LIMIT CAP — ${winName} ON TOP`,
    }
    out.push({
      turn: total,
      seat: winner,
      kind: 'win',
      action: finishMap[cond.kind] || `${winName} SECURES VICTORY`,
      detail: `END REASON: ${(game.end_reason || '?').replace(/_/g, ' ').toUpperCase()}`,
    })
  } else {
    out.push({
      turn: total,
      seat: -1,
      kind: 'draw',
      action: 'GAME ENDS IN DRAW',
      detail: `END REASON: ${(game.end_reason || '?').replace(/_/g, ' ').toUpperCase()}`,
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
const ReplayScrubber = ({ game, commanders }) => {
  const timeline = game?.timeline || []
  const totalTurns = timeline.length
  const [turnIdx, setTurnIdx] = useState(0) // 0-based into timeline
  const [playing, setPlaying] = useState(false)

  // Auto-play tick — advance one turn per second while `playing`.
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
    }, 900)
    return () => clearInterval(id)
  }, [playing, totalTurns])

  // Clamp the slider to a valid index when timeline shrinks (e.g. on
  // game switch). Defensive — totalTurns only changes when game does.
  useEffect(() => {
    if (turnIdx >= totalTurns) setTurnIdx(Math.max(0, totalTurns - 1))
  }, [totalTurns, turnIdx])

  if (totalTurns === 0) {
    return (
      <Panel code="RPT.X" title="REPLAY VIEWER" style={{ gridColumn: '1 / -1' }}>
        <div className="t-xs muted-2" style={{ lineHeight: 1.5 }}>
          &gt; NO PER-TURN TIMELINE FOR THIS GAME. LIVE GAMES CAPTURED AFTER
          THE REPLAY VIEWER LANDED CARRY A TIMELINE; OLDER GAMES REHYDRATED
          FROM SQLITE DO NOT (THE SCHEMA ONLY STORES END-OF-GAME SEATS).
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
      title={`REPLAY VIEWER / / TURN ${turnNo} OF ${timeline[totalTurns - 1].turn}`}
      style={{ gridColumn: '1 / -1' }}
      right={
        <span className="t-xs muted">
          {turnIdx + 1}/{totalTurns} SNAPSHOTS
        </span>
      }
    >
      {/* Transport: prev / play / next / jump-to-end */}
      <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 10 }}>
        <Btn sm arrow={null} onClick={() => { setPlaying(false); setTurnIdx(0) }} disabled={turnIdx === 0} title="Jump to start">⏮</Btn>
        <Btn sm arrow={null} onClick={() => { setPlaying(false); setTurnIdx(Math.max(0, turnIdx - 1)) }} disabled={turnIdx === 0} title="Previous turn">◀</Btn>
        <Btn sm arrow={null} solid={playing} onClick={() => setPlaying(p => !p)}>{playing ? '⏸ PAUSE' : '▶ PLAY'}</Btn>
        <Btn sm arrow={null} onClick={() => { setPlaying(false); setTurnIdx(Math.min(totalTurns - 1, turnIdx + 1)) }} disabled={turnIdx >= totalTurns - 1} title="Next turn">▶</Btn>
        <Btn sm arrow={null} onClick={() => { setPlaying(false); setTurnIdx(totalTurns - 1) }} disabled={turnIdx >= totalTurns - 1} title="Jump to end">⏭</Btn>
        <span className="t-xs muted" style={{ marginLeft: 8 }}>
          ACTIVE: {activeSeat >= 0 ? `SEAT.${String(activeSeat + 1).padStart(2, '0')} · ${(commanders[activeSeat] || 'UNKNOWN').split(',')[0].toUpperCase()}` : '—'}
        </span>
      </div>

      {/* The slider itself */}
      <input
        type="range"
        min={0}
        max={totalTurns - 1}
        value={turnIdx}
        onChange={e => { setPlaying(false); setTurnIdx(parseInt(e.target.value, 10)) }}
        style={{ width: '100%', accentColor: 'var(--accent)' }}
        aria-label="Turn slider"
      />
      <div style={{ display: 'flex', justifyContent: 'space-between', marginTop: 2 }}>
        <span className="t-xs muted-2">T1</span>
        <span className="t-xs muted-2">T{timeline[totalTurns - 1].turn}</span>
      </div>

      <div className="hr" style={{ margin: '14px 0 10px' }} />

      {/* Per-seat board state at this turn */}
      <div className="grid col-4 gap-4">
        {(snap?.seats || []).map((s, i) => {
          const cmdr = commanders[i] || 'UNKNOWN'
          const perms = s.battlefield || []
          const isActive = i === activeSeat
          const lifePct = Math.max(0, Math.min(100, (s.life / 40) * 100))
          const accent = s.lost ? 'var(--danger)' : isActive ? 'var(--accent)' : 'var(--rule-2)'
          return (
            <div key={i} className="panel" style={{ padding: 0, borderColor: accent }}>
              <div style={{ padding: '8px 10px' }}>
                <div className="flex justify-between items-center" style={{ marginBottom: 4 }}>
                  <span className="t-xs muted">SEAT.{String(i + 1).padStart(2, '0')}</span>
                  {s.lost && <Tag kind="bad">ELIMINATED</Tag>}
                  {!s.lost && isActive && <Tag kind="ok" solid>ACTIVE</Tag>}
                </div>
                <div className="t-md" style={{ fontWeight: 700, lineHeight: 1.2 }}>
                  {cmdr.split(',')[0].toUpperCase()}
                </div>
                <div className="hr" style={{ margin: '8px 0' }} />
                <div className="t-xs muted" style={{ marginBottom: 2 }}>LIFE</div>
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
                  ['HAND', String(s.hand_size)],
                  ['LIBRARY', String(s.library_size)],
                  ['GRAVEYARD', String(s.gy_size)],
                  ['BATTLEFIELD', String(perms.length)],
                ]} />
                {perms.length > 0 && (
                  <>
                    <div className="hr" style={{ margin: '8px 0' }} />
                    <div className="t-xs muted" style={{ marginBottom: 4 }}>BATTLEFIELD</div>
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

      {/* Event log for this turn */}
      <div className="hr" style={{ margin: '14px 0 8px' }} />
      <div className="t-xs muted" style={{ marginBottom: 6 }}>EVENTS — TURN {turnNo}</div>
      {events.length === 0 ? (
        <div className="t-xs muted-2">— NO RECORDED EVENTS THIS TURN —</div>
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
  // Deduplicate commander names from all games
  const unique = [...new Set(commanders)].sort()
  if (unique.length === 0) return null

  return (
    <div style={{
      display: 'flex', gap: 6, alignItems: 'center', flexWrap: 'wrap',
      padding: '10px 18px', borderBottom: '1px solid var(--rule-2)',
    }}>
      <span className="t-xs muted" style={{ marginRight: 4 }}>DECK CONTEXT:</span>
      <Tag solid={!selected} onClick={() => onSelect(null)} style={{ cursor: 'pointer' }}>ALL</Tag>
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
        <span>DECK STATS / / {selectedDeck.split(',')[0].toUpperCase()}</span>
        <span>{totalFiltered} GAMES</span>
      </div>
      <div className="deck-stats-row" style={{ padding: '14px 22px' }}>
        <div>
          <div className="t-xs muted">GAMES</div>
          <div className="t-2xl" style={{ fontWeight: 800, marginTop: 4 }}>{totalFiltered}</div>
        </div>
        <div>
          <div className="t-xs muted">WINS</div>
          <div className="t-2xl" style={{ fontWeight: 800, marginTop: 4, color: 'var(--ok)' }}>{wins}</div>
        </div>
        <div>
          <div className="t-xs muted">LOSSES</div>
          <div className="t-2xl" style={{ fontWeight: 800, marginTop: 4, color: 'var(--danger)' }}>{losses}</div>
        </div>
        <div>
          <div className="t-xs muted">WIN RATE</div>
          <div className="t-2xl" style={{ fontWeight: 800, marginTop: 4 }}>{wr}%</div>
        </div>
        <div>
          <div className="t-xs muted">AVG TURNS</div>
          <div className="t-2xl" style={{ fontWeight: 800, marginTop: 4 }}>{avgTurns}</div>
        </div>
        <div>
          <div className="t-xs muted">ELO</div>
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
  const { gameId } = useParams()
  const [game, setGame] = useState(null)
  const [games, setGames] = useState([])
  const [elo, setElo] = useState([])
  const [selectedDeck, setSelectedDeck] = useState(null)
  const [loading, setLoading] = useState(true)

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
        <Tape left="POST-GAME REPORT" mid="LOADING" right="" />
        <div style={{ padding: 36, textAlign: 'center' }}>
          <div className="t-md muted">&gt; LOADING REPORT<span className="blink">_</span></div>
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
        <Tape left="POST-GAME REPORT" mid="NO DATA" right="" />
        <DeckSelector commanders={allCommanders} selected={selectedDeck} onSelect={setSelectedDeck} />
        <div style={{ padding: 36, textAlign: 'center' }}>
          <div className="t-md muted">&gt; NO COMPLETED GAMES YET — WAIT FOR SHOWMATCH TO FINISH<span className="blink">_</span></div>
        </div>
      </>
    )
  }

  // Use the first filtered game as the featured game if no specific gameId
  const featuredGame = gameId ? game : (filteredGames[0] || game)

  const commanders = featuredGame?.commanders || []
  const seats = featuredGame?.final_seats || []
  const winnerIdx = featuredGame?.winner
  const winnerName = featuredGame?.winner_name || 'DRAW'
  const isVictory = winnerIdx >= 0

  return (
    <>
      <Tape
        left={featuredGame ? `POST-GAME / / GAME #${featuredGame.game_id}` : 'POST-GAME REPORT'}
        mid={isVictory ? 'VICTORY' : 'DRAW'}
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
              <span>RESULT BLOCK</span>
              <span style={{ display: 'inline-flex', alignItems: 'center', gap: 12 }}>
                <SummaryActions gameId={featuredGame.game_id} />
                <span>GAME.{featuredGame.game_id}</span>
              </span>
            </div>
            <div className="result-block-grid" style={{ padding: '18px 22px' }}>
              <div>
                <div className="t-xs muted">OUTCOME</div>
                <div className="t-3xl" style={{ color: isVictory ? 'var(--ok)' : 'var(--warn)', fontWeight: 800 }}>
                  {isVictory ? 'VICTORY' : 'DRAW'}
                </div>
                <div className="t-md muted" style={{ marginTop: 6 }}>
                  TURN {featuredGame.turns} · {(featuredGame.end_reason || '').replace(/_/g, ' ').toUpperCase()}
                </div>
                <div className="t-xs muted-2" style={{ marginTop: 4 }}>
                  WINNER: {winnerName.toUpperCase()}
                </div>
              </div>
              <Stat2 k="TURNS" v={String(featuredGame.turns)} />
              <Stat2 k="PLAYERS" v={String(commanders.length)} big />
              <Stat2 k="END" v={endReasonShort(featuredGame.end_reason)} />
            </div>
          </div>
        )}

        {/* Replay scrubber — per-turn timeline with board/life/event delta */}
        {featuredGame && (
          <ReplayScrubber game={featuredGame} commanders={commanders} />
        )}

        {/* Final board state — all seats */}
        {featuredGame && (
          <div className="panel" style={{ gridColumn: '1 / -1', padding: 0 }}>
            <div className="panel-hd"><span>FINAL BOARD STATE</span><span>{commanders.length} SEATS</span></div>
            <div style={{ padding: '18px 22px' }}>
              <div className="grid col-4 gap-4">
                {seats.map((s, i) => {
                  const isWinner = i === winnerIdx
                  const cmdr = commanders[i] || 'UNKNOWN'
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
                          {isWinner && <Tag kind="ok" solid>WINNER</Tag>}
                          {s.lost && !isWinner && <Tag kind="bad">ELIMINATED</Tag>}
                          {!s.lost && !isWinner && <Tag>ALIVE</Tag>}
                        </div>
                        <div className="t-md" style={{ fontWeight: 700, lineHeight: 1.2 }}>
                          {cmdr.toUpperCase()}
                        </div>
                        <div className="hr" style={{ margin: '8px 0' }} />
                        <KV rows={[
                          ['LIFE', String(s.life)],
                          ['HAND', String(s.hand_size)],
                          ['LIBRARY', String(s.library_size)],
                          ['GRAVEYARD', String(s.gy_size)],
                          ['BATTLEFIELD', String(perms.length)],
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
          <Panel code="RPT.A" title={`WINNER BATTLEFIELD / / ${winnerName.toUpperCase()}`}>
            <div className="t-xs muted" style={{ marginBottom: 6 }}>PERMANENTS AT GAME END</div>
            {(seats[winnerIdx].battlefield || []).length === 0 ? (
              <div className="t-xs muted-2">— NO PERMANENTS —</div>
            ) : (
              (seats[winnerIdx].battlefield || []).map((p, i) => (
                <div key={i} style={{ display: 'grid', gridTemplateColumns: '1fr 60px 60px', padding: '4px 0', borderBottom: '1px dashed var(--rule-2)', alignItems: 'center', gap: 8 }}>
                  <span className="t-xs" style={{ fontWeight: p.is_commander ? 700 : 400, color: p.is_commander ? 'var(--warn)' : 'var(--ink)' }}>
                    {p.is_commander ? '* ' : ''}{p.name?.toUpperCase()}
                  </span>
                  <span className="t-xs muted">{p.type || '—'}</span>
                  <span className="t-xs muted text-right">{p.tapped ? 'TAPPED' : 'UNTAPPED'}</span>
                </div>
              ))
            )}
          </Panel>
        )}

        {/* Game history list (filtered by deck) */}
        {filteredGames.length > 0 && (
          <Panel code="RPT.B" title={`${selectedDeck ? selectedDeck.split(',')[0].toUpperCase() + ' ' : ''}GAME LOG / / ${filteredGames.length} GAMES`}
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
                      <div className="t-md" style={{ fontWeight: 600 }}>{g.winner_name?.toUpperCase() || 'DRAW'}</div>
                      <div className="t-xs muted">T{g.turns} · {(g.end_reason || '').replace(/_/g, ' ')}</div>
                    </div>
                    {selectedDeck ? (
                      <Tag kind={deckWon ? 'ok' : isWin ? 'bad' : 'warn'} solid>
                        {deckWon ? 'WIN' : isWin ? 'LOSS' : 'DRAW'}
                      </Tag>
                    ) : (
                      <Tag kind={isWin ? 'ok' : 'warn'} solid>{isWin ? 'WIN' : 'DRAW'}</Tag>
                    )}
                    <span className="t-xs muted text-right">{g.commanders?.length || 0}P</span>
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
            <Panel code="RPT.C" title="WIN CONDITION" style={{ gridColumn: '1 / -1' }}
              right={<Tag solid kind="ok">{wc.label}</Tag>}>
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1.6fr', gap: 22, padding: '4px 0' }}>
                <div>
                  <div className="t-xs muted">RESOLUTION</div>
                  <div className="t-2xl" style={{ fontWeight: 800, marginTop: 4, color: 'var(--ok)' }}>{wc.label}</div>
                  <div className="t-md muted" style={{ marginTop: 8, lineHeight: 1.55, textTransform: 'uppercase', letterSpacing: '0.04em' }}>
                    &gt; {wc.detail}
                    <br />
                    &gt; CR.{(featuredGame.end_reason || '').replace(/_/g, '.')} · TURN {featuredGame.turns}
                    <br />
                    &gt; WINNER: {winnerName.toUpperCase()}
                  </div>
                </div>
                <div>
                  <div className="t-xs muted" style={{ marginBottom: 6 }}>ELIMINATED SEATS</div>
                  {eliminated.length === 0 ? (
                    <div className="t-xs muted-2">— NONE — </div>
                  ) : eliminated.map(x => (
                    <div key={x.i} style={{ display: 'grid', gridTemplateColumns: '1fr auto', padding: '5px 0', borderBottom: '1px dashed var(--rule-2)', alignItems: 'center', gap: 8 }}>
                      <div>
                        <div className="t-md" style={{ fontWeight: 700 }}>{x.name?.toUpperCase()}</div>
                        <div className="t-xs muted">SEAT.{String(x.i + 1).padStart(2, '0')} · {(x.s.loss_reason || x.s.LossReason || 'STATE-BASED').replace(/_/g, ' ').toUpperCase()}</div>
                      </div>
                      <Tag kind="bad">ELIM</Tag>
                    </div>
                  ))}
                </div>
              </div>
            </Panel>
          )
        })()}

        {/* MVP card — heuristic pick from winner's battlefield. */}
        {featuredGame && isVictory && (() => {
          const mvp = pickMVP(seats[winnerIdx])
          if (!mvp) return null
          const art = cardArtUrl(mvp.perm.name)
          return (
            <Panel code="RPT.D" title="MVP CARD" right={<HeuristicTag />}>
              <div style={{ display: 'flex', gap: 14, alignItems: 'flex-start' }}>
                <div style={{ width: 96, aspectRatio: '5/7', flexShrink: 0, border: '1px solid var(--rule-2)', overflow: 'hidden', background: 'var(--bg-2)' }}
                  className={art ? '' : 'hatch'}>
                  {art && <img src={art} alt={mvp.perm.name} style={{ width: '100%', height: '100%', objectFit: 'cover', filter: 'saturate(0.7) contrast(1.05)' }} onError={e => { e.target.style.display = 'none'; e.target.parentElement.classList.add('hatch') }} />}
                </div>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div className="t-xs muted">CARD</div>
                  <div className="t-xl" style={{ fontWeight: 800, marginTop: 2, lineHeight: 1.1 }}>
                    {mvp.perm.is_commander && <span style={{ color: 'var(--warn)' }}>★ </span>}
                    {mvp.perm.name?.toUpperCase()}
                  </div>
                  <div className="t-xs muted" style={{ marginTop: 4 }}>
                    {(mvp.perm.type || 'PERMANENT').toUpperCase()}
                    {mvp.perm.power != null ? ` · ${mvp.perm.power}/${mvp.perm.toughness ?? '?'}` : ''}
                  </div>
                  <div className="hr" style={{ margin: '10px 0' }} />
                  <div className="t-xs muted">SELECTION REASON</div>
                  <div className="t-md" style={{ marginTop: 4, lineHeight: 1.5, textTransform: 'uppercase', letterSpacing: '0.04em' }}>
                    &gt; {mvp.reason}
                  </div>
                  <div className="t-xs muted-2" style={{ marginTop: 8, lineHeight: 1.5 }}>
                    HEURISTIC PICK FROM FINAL BOARD STATE. PROPER MVP RANKING NEEDS PER-CARD IMPACT TELEMETRY (DAMAGE DEALT, COMBO ENABLED, ETC.) FROM THE ENGINE.
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
          const timeline = deriveTimeline(featuredGame, commanders)
          return (
            <Panel code="RPT.E" title="GAME TIMELINE"
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
                  &gt; ENTRIES MARKED `— —` LACK A TURN STAMP — PER-TURN LOG RETENTION IS A SERVER-SIDE TODO.
                </div>
              </div>
            </Panel>
          )
        })()}

        {/* Per-seat performance — final-state stats only. The life-over-
            time sparkline was a fabricated curve; dropped until the
            engine retains a per-turn life_history field. */}
        {featuredGame && (
          <Panel code="RPT.F" title="PER-SEAT PERFORMANCE" style={{ gridColumn: '1 / -1' }}>
            <div className="grid col-2 gap-4">
              {seats.map((s, i) => {
                const isWinner = i === winnerIdx
                const cmdr = commanders[i] || 'UNKNOWN'
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
                        {isWinner && <span style={{ color: 'var(--ok)', marginLeft: 6 }}>★ WINNER</span>}
                        {s.lost && !isWinner && <span style={{ color: 'var(--danger)', marginLeft: 6 }}>✕ LOST</span>}
                      </span>
                    </div>
                    <div style={{ padding: 12 }}>
                      <div className="t-xs muted" style={{ marginBottom: 2 }}>FINAL LIFE</div>
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
                        ['BATTLEFIELD', String(perms.length)],
                        ['CREATURES', String(creatures)],
                        ['NON-LAND', String(nonLand)],
                        ['LANDS', String(lands)],
                        ['HAND', String(s.hand_size)],
                        ['LIBRARY', String(s.library_size)],
                        ['GRAVEYARD', String(s.gy_size)],
                        ['ZONE TOTAL', String(totalCardsKnown)],
                      ]} />
                    </div>
                  </div>
                )
              })}
            </div>
            <div className="t-xs muted-2" style={{ marginTop: 10, lineHeight: 1.5 }}>
              &gt; ALL FIGURES READ DIRECTLY FROM `final_seats[]` — END-OF-GAME SNAPSHOT, NOT INTERPOLATED.
            </div>
          </Panel>
        )}
      </div>
    </>
  )
}
