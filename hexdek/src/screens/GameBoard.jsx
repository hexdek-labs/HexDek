import { useState, useEffect, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { Panel, Bar, Tag, Btn, Tape } from '../components/chrome'
import { cardArtUrl } from '../services/api'
import { useLiveSocket } from '../hooks/useLiveSocket'
import CardLink, { linkifyAction } from '../components/CardLink'
import { useT } from '../i18n'

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

// Takes the translator as a PARAMETER: a module-level formatter that
// reached for one of its own would word every tile once, at import.
// Five of these six plates are the same keys the fishtank's typeTag
// reads. The sixth is not: this screen prints ARTI where that one prints
// ART, one character apart and almost certainly an accident — but what a
// tile prints is a UI decision, so it gets its own key (whose every
// non-English value is tag_art's) rather than a silent unification.
// An unrecognised type is derived from the card's own name, not worded.
const permLabel = (t, p) => {
  if (p.is_commander) return t('spec.tag_cmdr')
  if (p.is_land) return t('spec.tag_land')
  if (p.type === 'CREATURE') return p.name?.split(' ')[0]?.slice(0, 6)?.toUpperCase() || t('spec.tag_crea')
  if (p.type === 'ARTIFACT') return t('spec.tag_arti')
  if (p.type === 'ENCHANTMENT') return t('spec.tag_enc')
  if (p.type === 'PLANESWALKER') return t('spec.tag_pw')
  return p.name?.split(' ')[0]?.slice(0, 5)?.toUpperCase() || '???'
}

const Stat = ({ k, v }) => (
  <div>
    <div className="t-xs muted" style={{ lineHeight: 1 }}>{k}</div>
    <div style={{ fontSize: 14, fontWeight: 700, lineHeight: 1.2 }}>{v}</div>
  </div>
)

const SeatPanel = ({ seat, seatIdx, isActive, isWinner, isYou, compact, right }) => {
  const t = useT()
  const perms = seat.battlefield || []
  const cmdrArt = cardArtUrl(seat.commander)
  const borderColor = isWinner ? 'var(--ok)' : isActive ? 'var(--warn)' : 'var(--rule-2)'

  return (
    <div className="panel" style={{ padding: 0, borderColor }}>
      <div className="panel-hd" style={{ borderColor }}>
        <span>
          SEAT.{String(seatIdx + 1).padStart(2, '0')} / / {seat.commander?.toUpperCase() || t('spec.seat_unknown')}
          {isActive && <span style={{ color: 'var(--warn)', marginLeft: 6 }}>● {t('common.active')}</span>}
          {isWinner && <span style={{ color: 'var(--ok)', marginLeft: 6 }}>★ {t('common.winner')}</span>}
          {seat.lost && !isWinner && <span style={{ color: 'var(--danger)', marginLeft: 6 }}>✕ {t('spec.gb_elim')}</span>}
          {isYou && <span style={{ color: 'var(--ok)', marginLeft: 6 }}>● {t('spec.gb_you')}</span>}
        </span>
        <span style={{ display: 'flex', gap: 8 }}>
          <span>♥ {seat.life}</span><span>✋ {seat.hand_size}</span><span>{t('spec.gb_lib')} {seat.library_size}</span>
        </span>
      </div>
      <div style={{ display: 'flex' }}>
        {cmdrArt && (
          <div style={{
            width: compact ? 60 : 80, minHeight: compact ? 60 : 80,
            backgroundImage: `url(${cmdrArt})`,
            backgroundSize: 'cover', backgroundPosition: 'center',
            borderRight: '1px solid var(--rule-2)',
            opacity: seat.lost && !isWinner ? 0.3 : 0.8,
            flexShrink: 0,
          }} />
        )}
        <div style={{ padding: 8, display: 'flex', gap: 4, flexWrap: 'wrap', minHeight: compact ? 48 : 60, flex: 1, justifyContent: right ? 'flex-end' : 'flex-start' }}>
          {perms.length === 0 ? (
            <span className="t-xs muted-2">{t('spec.gb_no_perms')}</span>
          ) : (
            perms.slice(0, compact ? 8 : 14).map((p, j) => (
              <div
                key={j}
                title={p.name}
                style={{
                  width: compact ? 36 : 44, aspectRatio: '1/1.3',
                  border: '1px solid var(--rule-2)',
                  display: 'flex', alignItems: 'flex-end', justifyContent: 'center',
                  fontSize: 7, color: '#fff',
                  borderColor: p.is_commander ? 'var(--warn)' : 'var(--rule-2)',
                  opacity: p.tapped ? 0.45 : 1,
                  transform: p.tapped ? 'rotate(8deg)' : 'none',
                  backgroundImage: p.name ? `url(${cardArtUrl(p.name)})` : undefined,
                  backgroundSize: 'cover', backgroundPosition: 'center',
                  position: 'relative', overflow: 'hidden',
                }}
              >
                <span style={{
                  background: 'rgba(0,0,0,0.7)',
                  padding: '1px 3px',
                  lineHeight: 1.1,
                  letterSpacing: '0.03em',
                  maxWidth: '100%',
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap',
                }}>
                  {permLabel(t, p)}
                </span>
              </div>
            ))
          )}
          {perms.length > (compact ? 8 : 14) && (
            <span className="t-xs muted" style={{ alignSelf: 'center' }}>+{perms.length - (compact ? 8 : 14)}</span>
          )}
        </div>
      </div>
      <div style={{ borderTop: '1px solid var(--rule-2)', padding: '4px 10px', display: 'flex', justifyContent: 'space-between' }}>
        <span className="t-xs muted">{t('spec.gb_gy_bf', { gy: seat.gy_size, bf: perms.length })}</span>
        {isActive && <span className="t-xs" style={{ color: 'var(--ok)' }}>● {t('spec.gb_priority')}</span>}
      </div>
    </div>
  )
}

export default function GameBoard() {
  const navigate = useNavigate()
  const t = useT()
  const { game, elo: eloArr, status } = useLiveSocket()
  const elo = eloArr || []
  // A FLAG, not a sentence: this used to hold the English text itself and
  // render it directly, which froze the line at whatever the socket last
  // reported. The render site words it now.
  const disconnected = status === 'disconnected'
  const logEndRef = useRef(null)

  useEffect(() => {
    logEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [game?.log?.length])

  if (disconnected && !game) {
    return (
      <>
        <Tape left={t('spec.gb_tape_title')} mid={t('spec.tape_offline')} right="" />
        <div style={{ padding: 36, textAlign: 'center' }}>
          <div className="t-md muted" style={{ lineHeight: 1.8, textTransform: 'uppercase', letterSpacing: '0.04em' }}>
            {t('spec.offline_1')}<br />
            {t('spec.gb_offline_2')}<br />
            &gt; {t('spec.err_disconnected')}<span className="blink">_</span>
          </div>
        </div>
      </>
    )
  }

  if (!game || game.status === 'starting') {
    return (
      <>
        <Tape left={t('spec.gb_tape_title')} mid={t('spec.tape_loading')} right="" />
        <div style={{ padding: 36, textAlign: 'center' }}>
          {/* A server-sent message is echoed; only the fallback is ours. */}
          <div className="t-md muted">&gt; {game?.message || t('spec.gb_loading_state')}<span className="blink">_</span></div>
        </div>
      </>
    )
  }

  const seats = game.seats || []
  const log = game.log || []
  const activeSeat = game.active_seat
  const eloMap = {}
  for (const e of elo) {
    eloMap[e.commander] = e
  }

  // Board layout: seat 0 at bottom (player POV), seats 1-3 as opponents
  // Clockwise: top-left=seat 2, top-center=seat 3 (or top), top-right=seat 1, bottom=seat 0
  const mySeat = seats[0]
  const oppSeats = seats.slice(1)

  return (
    <>
      <Tape
        left={t('spec.gb_tape_game', { game: game.game_id })}
        mid={game.finished ? t('spec.tape_game_over') : t('spec.sr_tape_live')}
        right={`T${game.turn} / / ${(game.phase || '').toUpperCase()}`}
      />

      <div className="game-board-layout" style={{ flex: 1, display: 'grid', gridTemplateColumns: '1fr 280px', overflow: 'hidden', position: 'relative', isolation: 'isolate' }}>
        {/* Ambient blurred-art background — your seat's commander, falls back
            to the active seat. Keyed by name so swaps don't restyle an
            in-flight image. */}
        {(() => {
          const ambientName = mySeat?.commander || seats[game.active_seat]?.commander
          const ambientUrl = ambientName ? cardArtUrl(ambientName) : null
          if (!ambientUrl) return null
          return <img key={ambientName} className="art-ambience" src={ambientUrl} alt="" aria-hidden="true" />
        })()}
        {/* Board */}
        <div style={{ padding: 14, display: 'flex', flexDirection: 'column', gap: 10, overflow: 'auto' }}>
          {/* Opponent row — top */}
          {oppSeats.length >= 2 && (
            <SeatPanel seat={oppSeats[1]} seatIdx={2} isActive={activeSeat === 2 && !game.finished} isWinner={game.finished && game.winner === 2} compact={false} />
          )}

          {/* Middle row — two side opponents + stack */}
          <div style={{ display: 'grid', gridTemplateColumns: oppSeats.length >= 3 ? '1fr 220px 1fr' : '1fr 220px', gap: 10, alignItems: 'start' }}>
            {oppSeats.length >= 1 && (
              <SeatPanel seat={oppSeats[0]} seatIdx={1} isActive={activeSeat === 1 && !game.finished} isWinner={game.finished && game.winner === 1} compact />
            )}

            {/* Stack / Phase info */}
            <div className="panel" style={{ padding: 0 }}>
              <div className="panel-hd"><span>{t('spec.gb_stack')}</span><span>{game.finished ? t('spec.gb_stack_end') : '00'}</span></div>
              <div style={{ padding: '24px 12px', textAlign: 'center' }}>
                {game.finished ? (
                  <>
                    <div className="t-xs" style={{ color: 'var(--ok)' }}>[{t('spec.tape_game_over')}]</div>
                    <div className="hr" style={{ margin: '8px 0' }} />
                    <div className="t-xs muted">{t('common.winner')}</div>
                    <div className="t-md" style={{ fontWeight: 700, color: 'var(--ok)' }}>
                      {game.winner >= 0 ? seats[game.winner]?.commander?.toUpperCase() : t('spec.sr_draw')}
                    </div>
                    <div className="hr" style={{ margin: '8px 0' }} />
                    {/* end_reason is backend-worded, so it is echoed. */}
                    <div className="t-xs muted">{t('spec.gb_reason')}</div>
                    <div className="t-md" style={{ fontWeight: 700 }}>{(game.end_reason || '').replace(/_/g, ' ').toUpperCase()}</div>
                    {game.game_id != null && (
                      <div style={{ marginTop: 12 }}>
                        <Btn sm ghost arrow="↗" onClick={() => navigate(`/report/${game.game_id}`)}>{t('spec.view_report')}</Btn>
                      </div>
                    )}
                  </>
                ) : (
                  <>
                    <div className="t-xs muted-2">{t('spec.gb_empty')}</div>
                    <div className="hr" style={{ margin: '12px 0' }} />
                    <div className="t-xs muted">{t('spec.kv_phase')}</div>
                    <div className="t-md" style={{ fontWeight: 700 }}>{(game.phase || '').toUpperCase()} / / T{game.turn}</div>
                    <div className="hr" style={{ margin: '12px 0' }} />
                    <div className="t-xs muted">{t('spec.gb_priority')}</div>
                    <div className="t-md" style={{ fontWeight: 700 }}>SEAT.{String(activeSeat + 1).padStart(2, '0')}</div>
                  </>
                )}
              </div>
            </div>

            {oppSeats.length >= 3 && (
              <SeatPanel seat={oppSeats[2]} seatIdx={3} isActive={activeSeat === 3 && !game.finished} isWinner={game.finished && game.winner === 3} compact right />
            )}
          </div>

          {/* Your seat — bottom, full width */}
          {mySeat && (
            <SeatPanel seat={mySeat} seatIdx={0} isActive={activeSeat === 0 && !game.finished} isWinner={game.finished && game.winner === 0} isYou />
          )}
        </div>

        {/* Side panel — game log + ELO */}
        <div style={{ borderLeft: '1px solid var(--rule-2)', padding: 14, overflow: 'auto', display: 'flex', flexDirection: 'column', gap: 12 }}>
          <Panel code="LOG" title={t('spec.gb_log_title', { turn: game.turn })} right={<span className="t-xs muted">{t('spec.evt_count', { count: log.length })}</span>}>
            <div style={{ maxHeight: 280, overflow: 'auto', fontSize: 11, lineHeight: 1.6 }}>
              {log.length === 0 ? (
                <div className="t-xs muted-2">{t('spec.waiting_events')}</div>
              ) : (
                log.map((entry, i) => (
                  <div
                    key={i}
                    style={{
                      display: 'grid',
                      gridTemplateColumns: '36px 1fr',
                      gap: 8,
                      padding: '2px 0',
                      borderBottom: i < log.length - 1 ? '1px dotted var(--rule)' : 'none',
                    }}
                  >
                    <span className="muted-2" style={{ fontSize: 10 }}>T{entry.turn}</span>
                    <span style={{ color: LOG_COLORS[entry.kind] || 'var(--ink)', letterSpacing: '0.02em' }}>
                      {(() => {
                        const { prefix, cardName } = linkifyAction(entry.action)
                        if (!cardName) return <>&gt; {entry.action}</>
                        return (
                          <>
                            &gt; {prefix}
                            <CardLink name={cardName} style={{ color: 'inherit', borderBottom: '1px dotted currentColor' }}>
                              {cardName}
                            </CardLink>
                          </>
                        )
                      })()}
                    </span>
                  </div>
                ))
              )}
              <div ref={logEndRef} />
            </div>
          </Panel>

          <Panel code="ELO" title={t('spec.live_elo')} right={<span className={`led led--on ${!game.finished ? 'blink' : ''}`} />}>
            {elo.length === 0 ? (
              <div className="t-xs muted">{t('spec.no_elo')}</div>
            ) : (
              elo.slice(0, 6).map((r, i) => (
                <div key={i} style={{ marginBottom: 8 }}>
                  <div className="flex justify-between">
                    <span className="t-xs">{r.commander?.toUpperCase()}</span>
                    <span className="t-xs" style={{ color: r.delta >= 0 ? 'var(--ok)' : 'var(--danger)' }}>
                      {Math.round(r.rating)} ({r.delta >= 0 ? '+' : ''}{r.delta})
                    </span>
                  </div>
                  <Bar value={Math.max(0, (r.rating - 1300) / 4)} />
                </div>
              ))
            )}
          </Panel>

          <Panel code="ASS" title={t('spec.gb_threat')}>
            {seats.length === 0 ? (
              <div className="t-xs muted">{t('spec.gb_no_seats')}</div>
            ) : (
              seats.filter((s, i) => i > 0 && !s.lost).map((s, i) => {
                const threat = Math.max(0, Math.min(100, 50 + (s.battlefield?.length || 0) * 5 - (40 - s.life)))
                const kind = threat > 60 ? 'bad' : threat > 35 ? 'warn' : 'ok'
                const label = threat > 60 ? t('spec.scale_high') : threat > 35 ? t('spec.gb_scale_mid') : t('spec.scale_low')
                return (
                  <div key={i} style={{ marginBottom: 8 }}>
                    <div className="flex justify-between" style={{ marginBottom: 3 }}>
                      <span className="t-xs">{s.commander?.toUpperCase()}</span>
                      <Tag kind={kind}>{label}</Tag>
                    </div>
                    <Bar value={threat} />
                  </div>
                )
              })
            )}
          </Panel>
        </div>
      </div>
    </>
  )
}
