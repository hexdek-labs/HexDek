import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Panel, KV, Bar, Tag, Btn, Tape, MiniBars } from '../components/chrome'
import { useProfile, useDecks, useGames, useMatchups } from '../hooks/useData'
import { useLiveSocket } from '../hooks/useLiveSocket'
import { cardArtUrl, api } from '../services/api'

/* ─── Deck Import Modal ──────────────────────────────────────────── */
const ImportModal = ({ onClose, onImported }) => {
  const [name, setName] = useState('')
  const [owner, setOwner] = useState('')
  const [deckList, setDeckList] = useState('')
  const [importing, setImporting] = useState(false)
  const [error, setError] = useState(null)

  const handleSubmit = async () => {
    if (!deckList.trim()) {
      setError('DECK LIST REQUIRED')
      return
    }
    setImporting(true)
    setError(null)
    try {
      await api.importDeck(name || 'Imported Deck', owner || 'imported', deckList)
      onImported()
      onClose()
    } catch (err) {
      setError(err.message)
    } finally {
      setImporting(false)
    }
  }

  return (
    <div style={{
      position: 'fixed', inset: 0, zIndex: 1000,
      background: 'rgba(0,0,0,0.7)', display: 'flex', alignItems: 'center', justifyContent: 'center',
    }} onClick={onClose}>
      <div className="panel import-modal" onClick={e => e.stopPropagation()}>
        <div className="panel-hd">
          <span>DECK IMPORT / / PASTE LIST</span>
          <span style={{ cursor: 'pointer' }} onClick={onClose}>X</span>
        </div>
        <div className="panel-bd" style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <div className="grid col-2" style={{ gap: 10 }}>
            <div>
              <div className="t-xs muted" style={{ marginBottom: 4 }}>DECK NAME</div>
              <input
                type="text"
                value={name}
                onChange={e => setName(e.target.value)}
                placeholder="TINYBONES, THE PICKPOCKET"
                style={{
                  width: '100%', padding: '8px 10px', background: 'var(--bg-2)',
                  border: '1px solid var(--rule-2)', color: 'var(--ink)',
                  fontFamily: 'inherit', fontSize: 11, letterSpacing: '0.06em',
                  textTransform: 'uppercase', outline: 'none',
                }}
              />
            </div>
            <div>
              <div className="t-xs muted" style={{ marginBottom: 4 }}>OWNER</div>
              <input
                type="text"
                value={owner}
                onChange={e => setOwner(e.target.value)}
                placeholder="IMPORTED"
                style={{
                  width: '100%', padding: '8px 10px', background: 'var(--bg-2)',
                  border: '1px solid var(--rule-2)', color: 'var(--ink)',
                  fontFamily: 'inherit', fontSize: 11, letterSpacing: '0.06em',
                  textTransform: 'uppercase', outline: 'none',
                }}
              />
            </div>
          </div>
          <div>
            <div className="t-xs muted" style={{ marginBottom: 4 }}>DECK LIST (1 CARD NAME PER LINE)</div>
            <textarea
              value={deckList}
              onChange={e => setDeckList(e.target.value)}
              placeholder={"COMMANDER: Tinybones, the Pickpocket\n1 Swamp\n1 Dark Ritual\n1 Sol Ring\n1 Thoughtseize\n..."}
              rows={14}
              style={{
                width: '100%', padding: '8px 10px', background: 'var(--bg-2)',
                border: '1px solid var(--rule-2)', color: 'var(--ink)',
                fontFamily: 'inherit', fontSize: 11, letterSpacing: '0.04em',
                outline: 'none', resize: 'vertical',
              }}
            />
          </div>
          <div className="t-xs muted-2">
            FORMAT: "COMMANDER: Card Name" ON FIRST LINE, THEN "1 Card Name" PER LINE.
            COMMENTS (#) AND BLANK LINES ARE IGNORED.
          </div>
          {error && <div className="t-xs" style={{ color: 'var(--danger)' }}>&gt; ERROR: {error}</div>}
          <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
            <Btn sm ghost onClick={onClose}>CANCEL</Btn>
            <Btn sm solid onClick={handleSubmit} arrow={importing ? '...' : '↗'}>
              {importing ? 'IMPORTING' : 'IMPORT'}
            </Btn>
          </div>
        </div>
      </div>
    </div>
  )
}

/* ─── Context Deck Selector (sidebar) ──────────────────────────── */
const ContextDeckSelector = ({ decks, activeDeck, onSelect }) => (
  <Panel code="III.H" title="CONTEXT DECK" solid>
    <div className="t-xs muted" style={{ marginBottom: 8 }}>SELECT PRIMARY DECK FOR REPORTS + STATS</div>
    <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
      <div
        style={{
          padding: '6px 8px', cursor: 'pointer',
          border: !activeDeck ? '1px solid var(--ok)' : '1px dashed var(--rule-2)',
          background: !activeDeck ? 'var(--bg-2)' : 'transparent',
        }}
        onClick={() => onSelect(null)}
      >
        <div className="t-xs" style={{ fontWeight: !activeDeck ? 700 : 400 }}>ALL DECKS</div>
        <div className="t-xs muted-2">AGGREGATE VIEW</div>
      </div>
      {decks.map(d => {
        const isActive = activeDeck === `${d.owner}/${d.id}`
        return (
          <div
            key={`${d.owner}/${d.id}`}
            style={{
              padding: '6px 8px', cursor: 'pointer',
              border: isActive ? '1px solid var(--ok)' : '1px dashed var(--rule-2)',
              background: isActive ? 'var(--bg-2)' : 'transparent',
            }}
            onClick={() => onSelect(`${d.owner}/${d.id}`)}
          >
            <div className="t-xs" style={{ fontWeight: isActive ? 700 : 400 }}>{d.name}</div>
            <div className="t-xs muted-2">{d.owner?.toUpperCase()}</div>
          </div>
        )
      })}
    </div>
  </Panel>
)

/* ─── Deck Card (with ELO stats) ─────────────────────────────── */
const DeckCard = ({ slot, name, color, power, bracket, winRate, archetype, gold, commanderCard, owner, id, onClick, eloStats }) => (
  <div className="panel" style={{ padding: 0, borderColor: gold ? 'var(--warn)' : 'var(--rule-2)', cursor: 'pointer' }} onClick={onClick}>
    <div className="panel-hd" style={{ borderColor: gold ? 'var(--warn)' : 'var(--rule-2)' }}>
      <span>SLOT.{slot}</span>
      <span>B{bracket}{power !== '?' ? ` / / P${power}` : ''}</span>
    </div>
    <div style={{ aspectRatio: '1.4/1', borderBottom: '1px solid var(--rule-2)', position: 'relative', overflow: 'hidden' }} className={commanderCard ? '' : 'hatch'}>
      {commanderCard ? (
        <img
          src={cardArtUrl(commanderCard)}
          alt={commanderCard}
          loading="lazy"
          style={{ width: '100%', height: '100%', objectFit: 'cover', filter: 'saturate(0.5) contrast(1.1) brightness(0.85)' }}
          onError={e => { e.target.style.display = 'none'; e.target.parentElement.classList.add('hatch') }}
        />
      ) : (
        <>
          <span style={{ position: 'absolute', top: 4, left: 6, fontSize: 9, letterSpacing: '0.1em', color: 'var(--ink-3)' }}>CMDR.IMG</span>
          <span style={{ position: 'absolute', bottom: 4, right: 6, fontSize: 9, letterSpacing: '0.1em', color: 'var(--ink-3)' }}>{color}</span>
        </>
      )}
    </div>
    <div style={{ padding: '8px 10px' }}>
      <div className="t-md" style={{ fontWeight: 700, lineHeight: 1.2, minHeight: 30 }}>{name}</div>
      {commanderCard && commanderCard.toUpperCase() !== (name || '').toUpperCase() && (
        <div className="t-xs" style={{ marginTop: 2, color: 'var(--ink-2)', lineHeight: 1.2 }}>{commanderCard}</div>
      )}
      <div className="t-xs muted" style={{ marginTop: 4 }}>{owner?.toUpperCase() || ''}{archetype && archetype !== '—' ? ` / / ${archetype}` : ''}</div>
      <div className="hr" style={{ margin: '8px 0' }} />
      {eloStats ? (
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 4, marginBottom: 6 }}>
          <div>
            <div className="t-xs muted">ELO</div>
            <div className="t-md" style={{ fontWeight: 700 }}>{Math.round(eloStats.rating)}</div>
          </div>
          <div>
            <div className="t-xs muted">RECORD</div>
            <div className="t-md" style={{ fontWeight: 700 }}>
              <span style={{ color: 'var(--ok)' }}>{eloStats.wins}</span>
              <span className="muted-2">-</span>
              <span style={{ color: 'var(--danger)' }}>{eloStats.losses}</span>
            </div>
          </div>
          <div>
            <div className="t-xs muted">WR</div>
            <div className="t-md" style={{ fontWeight: 700 }}>{eloStats.win_rate}%</div>
          </div>
        </div>
      ) : (
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <div>
            <div className="t-xs muted">WR</div>
            <div className="t-lg" style={{ fontWeight: 700 }}>{winRate}%</div>
          </div>
        </div>
      )}
      <Btn sm arrow="↗">OPEN</Btn>
    </div>
  </div>
)

const NewSlot = ({ slot, onImport }) => (
  <div className="panel" style={{ padding: 0, borderStyle: 'dashed', display: 'flex', flexDirection: 'column' }}>
    <div className="panel-hd" style={{ borderStyle: 'dashed' }}>
      <span>SLOT.{slot}</span>
      <span>EMPTY</span>
    </div>
    <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', flexDirection: 'column', gap: 10, padding: 18 }}>
      <div style={{ fontSize: 42, lineHeight: 1, fontWeight: 300 }}>+</div>
      <div className="t-xs muted">NEW DECK</div>
      <Btn sm arrow="↗">FORGE</Btn>
      <Btn sm ghost arrow="↗" onClick={onImport}>IMPORT</Btn>
    </div>
  </div>
)

/* ─── Main Dashboard ─────────────────────────────────────────── */
export default function Dashboard() {
  const { data: profile } = useProfile()
  const { data: decks, refetch: refetchDecks } = useDecks()
  const { data: games } = useGames()
  const { data: matchups } = useMatchups()
  const { stats: rawStats, elo: rawElo } = useLiveSocket()
  const live = rawStats ? {
    activeForges: 1,
    totalGames: rawStats.games_played || 0,
    gamesPerMin: rawStats.games_per_min || 0,
    userContrib: rawStats.games_played || 0,
    userRank: 'LOCAL',
    throughput: [],
  } : { activeForges: 0, totalGames: 0, gamesPerMin: 0, userContrib: 0, userRank: 'LOCAL', throughput: [] }
  const elo = rawElo || []
  const [ownerFilter, setOwnerFilter] = useState('')
  const [activeDeck, setActiveDeck] = useState(null)
  const [showImport, setShowImport] = useState(false)
  const [showFullLeaderboard, setShowFullLeaderboard] = useState(false)
  const navigate = useNavigate()

  const owners = [...new Set(decks.map(d => d.owner).filter(Boolean))].sort()
  const filtered = ownerFilter ? decks.filter(d => d.owner === ownerFilter) : decks
  const nextSlot = String(filtered.length + 1).padStart(2, '0')

  // ELO lookup by deck_id for per-deck matching
  const eloByDeckId = {}
  for (const e of elo) {
    if (e.deck_id) eloByDeckId[e.deck_id] = e
  }
  const getDeckELO = (deck) => eloByDeckId[deck.id] || null

  // Aggregate ELO by commander for the leaderboard
  const aggregatedElo = (() => {
    const byCmd = {}
    for (const e of elo) {
      const key = e.commander?.toLowerCase() || e.deck_id
      if (!byCmd[key]) {
        byCmd[key] = { commander: e.commander, rating: 0, games: 0, wins: 0, losses: 0, delta: 0, _count: 0 }
      }
      const a = byCmd[key]
      a.games += e.games || 0
      a.wins += e.wins || 0
      a.losses += e.losses || 0
      a.rating += (e.rating || 1500) * (e.games || 1)
      a.delta += e.delta || 0
      a._count++
    }
    return Object.values(byCmd)
      .map(a => ({ ...a, rating: a.games > 0 ? a.rating / a.games : 1500, win_rate: a.games > 0 ? Math.round(a.wins / a.games * 1000) / 10 : 0 }))
      .sort((a, b) => b.rating - a.rating)
  })()

  // Derive ELO from live data if profile doesn't have it from showmatch
  const topElo = elo.length > 0 ? elo[0] : null
  const displayElo = profile.elo || (topElo ? Math.round(topElo.rating) : 1500)
  const displayEloChange = profile.eloChange || (topElo ? Math.round(topElo.delta) : 0)

  // If a context deck is selected, compute per-deck stats for the KPI strip
  const activeDeckData = activeDeck ? decks.find(d => `${d.owner}/${d.id}` === activeDeck) : null
  const activeDeckELO = activeDeckData ? getDeckELO(activeDeckData) : null

  const kpiElo = activeDeckELO ? Math.round(activeDeckELO.rating) : displayElo
  const kpiEloChange = activeDeckELO ? Math.round(activeDeckELO.delta) : displayEloChange
  const kpiGames = activeDeckELO ? activeDeckELO.games : profile.gamesPlayed
  const kpiWinRate = activeDeckELO ? activeDeckELO.win_rate : profile.winRate
  const kpiLabel = activeDeckData ? activeDeckData.name : 'ALL DECKS'

  return (
    <>
      <Tape left="DASHBOARD / / DOC HX-301" mid={activeDeckData ? `CTX: ${kpiLabel}` : 'REV C.25'} right={ownerFilter ? `VIEW: ${ownerFilter.toUpperCase()}` : 'VIEW: ALL'} />

      {showImport && (
        <ImportModal
          onClose={() => setShowImport(false)}
          onImported={refetchDecks}
        />
      )}

      <div className="dashboard-layout">
        <div className="dashboard-main">
          {/* KPI strip */}
          <div className="kpi-strip">
            {[
              ['ELO RATING', String(kpiElo), `${kpiEloChange > 0 ? '+' : ''}${kpiEloChange} / SESSION`],
              ['GAMES PLAYED', String(kpiGames), activeDeckELO ? `${activeDeckELO.wins}W ${activeDeckELO.losses}L` : 'OF ∞'],
              ['WIN RATE', `${kpiWinRate}%`, activeDeckELO ? `${activeDeckELO.games} SAMPLE` : 'TARGET 40%'],
              ['AVG WIN TURN', String(profile.avgWinTurn), 'B5 BENCH 6.0'],
            ].map(([k, v, sub], i) => (
              <div key={i}>
                <div className="t-xs muted">{k}</div>
                <div className="punch" style={{ fontSize: 36, marginTop: 4 }}>{v}</div>
                <div className="t-xs muted-2" style={{ marginTop: 2 }}>{sub}</div>
              </div>
            ))}
          </div>

          {/* Owner filter + Deck grid */}
          <Panel code="III.A" title={`DECK ARCHIVE / / ${filtered.length} DECKS`} right={
            <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
              <Btn sm ghost onClick={() => setShowImport(true)} arrow="↑">IMPORT</Btn>
              <Tag solid={!ownerFilter} onClick={() => setOwnerFilter('')} style={{ cursor: 'pointer' }}>ALL</Tag>
              {owners.map(o => (
                <Tag key={o} solid={ownerFilter === o} onClick={() => setOwnerFilter(ownerFilter === o ? '' : o)} style={{ cursor: 'pointer' }}>{o.toUpperCase()}</Tag>
              ))}
            </div>
          }>
            <div className="grid col-4 gap-3">
              {filtered.map((d) => (
                <DeckCard
                  key={`${d.owner}/${d.id}`}
                  slot={d.slot}
                  name={d.name}
                  color={d.color}
                  power={d.power}
                  bracket={d.bracket}
                  winRate={d.winRate}
                  archetype={d.archetype}
                  gold={d.gold}
                  commanderCard={d.commanderCard}
                  owner={d.owner}
                  id={d.id}
                  eloStats={getDeckELO(d)}
                  onClick={() => navigate(`/decks/${d.owner}/${d.id}`)}
                />
              ))}
              <NewSlot slot={nextSlot} onImport={() => setShowImport(true)} />
            </div>
          </Panel>

          {/* Recent games + Dossier */}
          <div className="grid col-2 gap-3">
            <Panel code="III.B" title={`RECENT GAMES / / LOG.${profile.gamesPlayed}`}>
              <div style={{ display: 'flex', flexDirection: 'column' }}>
                {games.length === 0 ? (
                  <div className="t-xs muted" style={{ padding: '12px 0', textAlign: 'center' }}>
                    &gt; NO GAMES YET — START THE SHOWMATCH ENGINE<span className="blink">_</span>
                  </div>
                ) : games.map((g, i) => (
                  <div key={i} style={{ display: 'grid', gridTemplateColumns: '48px 1fr 80px 60px', gap: 8, padding: '8px 0', borderBottom: i < games.length - 1 ? '1px dashed var(--rule-2)' : 'none', alignItems: 'center', cursor: 'pointer' }} onClick={() => navigate(`/report/${g.id.replace('G.', '')}`)}>
                    <span className="t-xs muted-2">{g.id}</span>
                    <div>
                      <div className="t-md" style={{ fontWeight: 600 }}>{g.deck} <span className="muted-2">·</span> <span className="muted">{g.opponent}</span></div>
                      <div className="t-xs muted">{g.detail}</div>
                    </div>
                    <Tag kind={g.kind} solid>{g.result}</Tag>
                    <span className="t-xs muted text-right">{g.time}</span>
                  </div>
                ))}
              </div>
            </Panel>

            <Panel code="III.C" title="OPERATOR DOSSIER">
              <div className="grid col-2 gap-3" style={{ marginBottom: 10 }}>
                <div>
                  <div className="t-xs muted">ARCHETYPE</div>
                  <div className="t-xl" style={{ fontWeight: 700, marginTop: 2 }}>{profile.archetype}</div>
                </div>
                <div>
                  <div className="t-xs muted">PERCENTILE</div>
                  <div className="t-xl" style={{ fontWeight: 700, marginTop: 2 }}>{profile.percentile}</div>
                </div>
              </div>
              <div className="hr" style={{ margin: '8px 0 12px' }} />
              <div className="t-xs muted" style={{ marginBottom: 6 }}>SKILL DELTA / / B5 BENCHMARK</div>
              {profile.skills.map((s, i) => (
                <div key={i} style={{ display: 'grid', gridTemplateColumns: '130px 1fr 36px', alignItems: 'center', gap: 8, marginBottom: 4 }}>
                  <span className="t-xs">{s.name}</span>
                  <Bar value={s.value} />
                  <span className="t-xs text-right muted">{s.value}%</span>
                </div>
              ))}
            </Panel>
          </div>

          {/* Live forge */}
          <Panel code="III.D" title="LIVE FORGE / / SESSION TELEMETRY" right={<span className={`led led--on ${live.totalGames > 0 ? 'blink' : ''}`} />}>
            <div className="grid col-4 gap-4">
              <div>
                <div className="t-xs muted">ACTIVE FORGES</div>
                <div className="punch" style={{ fontSize: 42 }}>{live.activeForges.toLocaleString()}</div>
              </div>
              <div>
                <div className="t-xs muted">GAMES SIM (SESSION)</div>
                <div className="t-xl" style={{ fontWeight: 700, marginTop: 8, fontVariantNumeric: 'tabular-nums' }}>{live.totalGames.toLocaleString()}</div>
                <div className="t-xs muted-2" style={{ marginTop: 2 }}>+ {live.gamesPerMin.toLocaleString()} / MIN</div>
              </div>
              <div>
                <div className="t-xs muted">YOUR CONTRIB.</div>
                <div className="t-xl" style={{ fontWeight: 700, marginTop: 8 }}>{live.userContrib.toLocaleString()}</div>
                <div className="t-xs muted-2">RANK {live.userRank}</div>
              </div>
              <div>
                <div className="t-xs muted">TPS / 60S</div>
                <MiniBars data={live.throughput} />
              </div>
            </div>
          </Panel>
        </div>

        <div className="dashboard-sidebar">
          <Panel code="III.E" title="OPERATOR // ID CARD" solid>
            <div className="hatch" style={{ aspectRatio: '1/1', marginBottom: 10, position: 'relative' }}>
              <div style={{ position: 'absolute', top: 6, left: 6 }} className="t-xs muted-2">PORTRAIT.HEX</div>
              <div style={{ position: 'absolute', bottom: 6, right: 6 }} className="t-xs muted-2">128x128</div>
            </div>
            <div className="t-2xl" style={{ fontWeight: 700 }}>{profile.username}</div>
            <div className="t-xs muted">{profile.userId} / / JOINED {profile.joined}</div>
            <div className="hr" style={{ margin: '10px 0' }} />
            <KV rows={[
              ['CLASS', (profile.archetype || '').replace(/"/g, '')],
              ['ELO', String(displayElo)],
              ['TIER', profile.tier],
              ['STREAK', profile.streak],
              ['PRIMARY', profile.primaryColor],
            ]} />
          </Panel>

          {/* Context Deck selector */}
          <ContextDeckSelector
            decks={decks}
            activeDeck={activeDeck}
            onSelect={setActiveDeck}
          />

          {/* Live ELO leaderboard — full scrollable */}
          <Panel code="III.F" title={`ELO LEADERBOARD / / ${aggregatedElo.length} COMMANDERS`} right={
            <span className={`led led--on ${aggregatedElo.length > 0 ? 'blink' : ''}`} />
          }>
            {aggregatedElo.length === 0 ? (
              <div className="t-xs muted" style={{ padding: '8px 0' }}>NO ELO DATA — START SHOWMATCH ENGINE</div>
            ) : (
              <>
                {/* Header row */}
                <div style={{
                  display: 'grid', gridTemplateColumns: '24px 1fr 54px 70px 50px',
                  gap: 4, padding: '4px 0', borderBottom: '1px solid var(--rule-2)', marginBottom: 4,
                }}>
                  <span className="t-xs muted">#</span>
                  <span className="t-xs muted">COMMANDER</span>
                  <span className="t-xs muted text-right">ELO</span>
                  <span className="t-xs muted text-right">RECORD</span>
                  <span className="t-xs muted text-right">WR%</span>
                </div>
                {/* Scrollable list */}
                <div style={{ maxHeight: showFullLeaderboard ? 600 : 280, overflow: 'auto' }}>
                  {(showFullLeaderboard ? aggregatedElo : aggregatedElo.slice(0, 10)).map((r, i) => (
                    <div key={i} style={{
                      display: 'grid', gridTemplateColumns: '24px 1fr 54px 70px 50px',
                      gap: 4, padding: '5px 0',
                      borderBottom: '1px dashed var(--rule-2)',
                      alignItems: 'center',
                    }}>
                      <span className="t-xs muted-2">{i + 1}</span>
                      <span className="t-xs" style={{ fontWeight: i < 3 ? 700 : 400, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        {r.commander?.toUpperCase()}
                      </span>
                      <span className="t-xs text-right" style={{ color: r.delta >= 0 ? 'var(--ok)' : 'var(--danger)', fontVariantNumeric: 'tabular-nums' }}>
                        {Math.round(r.rating)}
                      </span>
                      <span className="t-xs text-right" style={{ fontVariantNumeric: 'tabular-nums' }}>
                        <span style={{ color: 'var(--ok)' }}>{r.wins ?? 0}</span>
                        <span className="muted-2">-</span>
                        <span style={{ color: 'var(--danger)' }}>{r.losses ?? 0}</span>
                      </span>
                      <span className="t-xs text-right" style={{ fontVariantNumeric: 'tabular-nums' }}>{r.win_rate ?? 0}%</span>
                    </div>
                  ))}
                </div>
                {aggregatedElo.length > 10 && (
                  <div style={{ marginTop: 8, textAlign: 'center' }}>
                    <Btn sm ghost onClick={() => setShowFullLeaderboard(!showFullLeaderboard)} arrow={showFullLeaderboard ? '↑' : '↓'}>
                      {showFullLeaderboard ? `COLLAPSE` : `SHOW ALL ${aggregatedElo.length}`}
                    </Btn>
                  </div>
                )}
              </>
            )}
          </Panel>

          <Panel code="III.G" title={`ACHIEVEMENTS // ${profile.achievements.filter(a => a.unlocked).length}/${profile.achievements.length}`}>
            {profile.achievements.map((a, i) => (
              <div key={i} style={{ display: 'flex', justifyContent: 'space-between', padding: '4px 0', borderBottom: i < profile.achievements.length - 1 ? '1px dashed var(--rule-2)' : 'none', opacity: a.unlocked ? 1 : 0.4 }}>
                <span><span style={{ marginRight: 6 }}>{a.icon}</span><span className="t-md">{a.name}</span></span>
                {a.unlocked && <span className="t-xs" style={{ color: 'var(--ok)' }}>UNLOCKED</span>}
              </div>
            ))}
          </Panel>
        </div>
      </div>
    </>
  )
}
