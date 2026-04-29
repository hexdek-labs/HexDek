import { useState, useEffect } from 'react'
import { useParams } from 'react-router-dom'
import { Panel, KV, Bar, Tag, Btn, Tape } from '../components/chrome'
import { api, cardArtUrl } from '../services/api'

const Stat2 = ({ k, v, big }) => (
  <div>
    <div className="t-xs muted">{k}</div>
    <div className={big ? 't-3xl' : 't-2xl'} style={{ fontWeight: 800, marginTop: 4 }}>{v}</div>
  </div>
)

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
          const g = await api.getGame(gameId)
          setGame(g)
        } else {
          const list = await api.getGames(1)
          if (list?.length > 0) {
            setGame(list[0])
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
            <div className="panel-hd"><span>RESULT BLOCK</span><span>GAME.{featuredGame.game_id}</span></div>
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
              <Stat2 k="END" v={(featuredGame.end_reason || '?').replace(/_/g, ' ').toUpperCase().slice(0, 12)} />
            </div>
          </div>
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
            <div style={{ maxHeight: 400, overflow: 'auto' }}>
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

        {/* Analysis placeholder */}
        <Panel code="RPT.C" title="DECISION ANALYSIS" style={{ gridColumn: '1 / -1' }}>
          <div className="t-md muted" style={{ lineHeight: 1.7, textTransform: 'uppercase', letterSpacing: '0.03em' }}>
            &gt; DEEPER ANALYSIS REQUIRES FREYA<br />
            &gt; RUN <span style={{ color: 'var(--ok)' }}>MTGSQUAD-FREYA</span> TO GENERATE:<br />
            &gt; · TURN-BY-TURN DECISION GRADING<br />
            &gt; · PER-CARD MVP / LVP RANKINGS<br />
            &gt; · OPPONENT THREAT MODELING<br />
            &gt; · WIN LINE IDENTIFICATION<br />
            <span className="blink">_</span>
          </div>
        </Panel>
      </div>
    </>
  )
}
