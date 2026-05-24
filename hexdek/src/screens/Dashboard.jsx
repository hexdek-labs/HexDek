import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { Panel, KV, Bar, Tag, Btn, Tape, MiniBars } from '../components/chrome'
import ImportModal from '../components/ImportModal'
import { useProfile, useDecks, useGames } from '../hooks/useData'
import { useLiveSocket } from '../hooks/useLiveSocket'
import { useAuth } from '../context/AuthContext'
import { api, cardArtUrl } from '../services/api'

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
const DeckCard = ({ slot, name, color, power, bracket, winRate, archetype, gold, commanderCard, owner, id, onClick, eloStats, friendCount }) => (
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
      <div className="t-xs muted" style={{ marginTop: 4, display: 'flex', justifyContent: 'space-between', gap: 6 }}>
        <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
          {owner?.toUpperCase() || ''}{archetype && archetype !== '—' ? ` / / ${archetype}` : ''}
        </span>
        {friendCount != null && (
          <span className="muted-2" title={`${owner?.toUpperCase()} has ${friendCount} friend${friendCount === 1 ? '' : 's'}`}>● {friendCount}</span>
        )}
      </div>
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

/* ─── Recent Imports panel ───────────────────────────────────── */
function relativeTime(unixSec) {
  if (!unixSec) return ''
  const diff = Math.max(0, Math.floor(Date.now() / 1000 - unixSec))
  if (diff < 60) return `${diff}s ago`
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
  if (diff < 604800) return `${Math.floor(diff / 86400)}d ago`
  return new Date(unixSec * 1000).toISOString().slice(0, 10)
}

function RecentImports({ imports, onOpen }) {
  return (
    <Panel code="III.A.1" title={`RECENT IMPORTS / / ${imports.length} EVENTS`}>
      <div style={{ display: 'flex', flexDirection: 'column' }}>
        {imports.map((entry, i) => {
          const last = i === imports.length - 1
          const sourceLabel = entry.source === 'moxfield' ? 'MOXFIELD' : 'PASTE'
          return (
            <div
              key={entry.id}
              onClick={() => onOpen(entry.deck_key)}
              style={{
                display: 'grid',
                gridTemplateColumns: '70px 1fr 70px 70px 70px',
                gap: 10,
                padding: '8px 0',
                borderBottom: last ? 'none' : '1px dashed var(--rule-2)',
                alignItems: 'center',
                cursor: 'pointer',
              }}
            >
              <Tag solid={entry.source === 'moxfield'}>{sourceLabel}</Tag>
              <div style={{ minWidth: 0 }}>
                <div className="t-md" style={{ fontWeight: 700, lineHeight: 1.2, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {entry.deck_name || entry.deck_key}
                </div>
                {entry.commander && (
                  <div className="t-xs muted" style={{ marginTop: 2, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{entry.commander}</div>
                )}
                {entry.source === 'moxfield' && entry.source_url && (
                  <div className="t-xs muted-2" style={{ marginTop: 2, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{entry.source_url}</div>
                )}
              </div>
              <span className="t-xs muted text-right">{entry.card_count} CARDS</span>
              <span className="t-xs muted text-right">{relativeTime(entry.imported_at)}</span>
              <span className="t-xs muted-2 text-right">{entry.deck_key}</span>
            </div>
          )
        })}
      </div>
    </Panel>
  )
}

/* ─── Main Dashboard ─────────────────────────────────────────── */
export default function Dashboard() {
  const { user, loading: authLoading } = useAuth()
  const { data: profile } = useProfile()
  const { data: decks, refetch: refetchDecks } = useDecks()
  const { data: games } = useGames()
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
  let storedOwner = ''
  try { storedOwner = localStorage.getItem('hexdek_owner') || '' } catch {}
  const userOwner = user ? (storedOwner || user.displayName?.toLowerCase() || user.email?.split('@')[0]?.split('.')[0] || '') : ''
  const [ownerFilter, setOwnerFilter] = useState('')
  const [activeDeck, setActiveDeck] = useState(null)
  const [showImport, setShowImport] = useState(false)
  const [showFullLeaderboard, setShowFullLeaderboard] = useState(false)
  const [recentImports, setRecentImports] = useState([])
  const [friends, setFriends] = useState([])
  const [friendCounts, setFriendCounts] = useState({})
  const navigate = useNavigate()

  // Refetch friends when the user changes; also on a custom 'hexdek-friends-changed'
  // window event so DeckArchive's add/remove can ping us without prop drilling.
  useEffect(() => {
    if (!userOwner) { setFriends([]); return }
    let alive = true
    const load = () => {
      api.listFriends(userOwner)
        .then(r => { if (alive) setFriends(r?.friends || []) })
        .catch(() => { if (alive) setFriends([]) })
    }
    load()
    const onChanged = () => load()
    window.addEventListener('hexdek-friends-changed', onChanged)
    return () => {
      alive = false
      window.removeEventListener('hexdek-friends-changed', onChanged)
    }
  }, [userOwner])

  // Recent imports for the user's owner. Refetches when the import modal is
  // closed (which is when a new deck would have just landed).
  useEffect(() => {
    if (!userOwner) { setRecentImports([]); return }
    let alive = true
    api.getImports(userOwner, 10)
      .then(r => { if (alive) setRecentImports(r?.imports || []) })
      .catch(() => { if (alive) setRecentImports([]) })
    return () => { alive = false }
  }, [userOwner, showImport])

  // Friend count per owner across the visible decks. There's no batch
  // endpoint, so we fan out one listFriends call per unique slug — fine
  // at dashboard scale (a handful of owners). Keyed-on a stable joined
  // string so the effect doesn't refire on identity-only deck reorders.
  const ownerListKey = [...new Set(decks.map(d => d.owner).filter(Boolean))].sort().join(',')
  useEffect(() => {
    if (!ownerListKey) { setFriendCounts({}); return }
    let alive = true
    const slugs = ownerListKey.split(',')
    const load = () => {
      Promise.all(slugs.map(slug =>
        api.listFriends(slug)
          .then(r => [slug, (r?.friends || []).length])
          .catch(() => [slug, null])
      )).then(pairs => {
        if (!alive) return
        const next = {}
        for (const [k, v] of pairs) if (v != null) next[k] = v
        setFriendCounts(next)
      })
    }
    load()
    const onChanged = () => load()
    window.addEventListener('hexdek-friends-changed', onChanged)
    return () => {
      alive = false
      window.removeEventListener('hexdek-friends-changed', onChanged)
    }
  }, [ownerListKey])

  const owners = [...new Set(decks.map(d => d.owner).filter(Boolean))].sort()
  const effectiveFilter = ownerFilter === '__all__' ? '' : (ownerFilter || (user && userOwner && owners.includes(userOwner) ? userOwner : ''))
  const filtered = effectiveFilter ? decks.filter(d => d.owner === effectiveFilter) : decks
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

  if (authLoading) {
    return (
      <>
        <Tape left="DASHBOARD / / DOC HX-301" mid="AUTHENTICATING" right="STANDBY" />
        <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: 300 }}>
          <div className="t-xs muted">VERIFYING CREDENTIALS . . .</div>
        </div>
      </>
    )
  }

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
              <Tag solid={!effectiveFilter} onClick={() => setOwnerFilter('__all__')} style={{ cursor: 'pointer' }}>ALL</Tag>
              {owners.map(o => (
                <Tag key={o} solid={effectiveFilter === o} onClick={() => setOwnerFilter(effectiveFilter === o ? '__all__' : o)} style={{ cursor: 'pointer' }}>{o.toUpperCase()}</Tag>
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
                  friendCount={friendCounts[d.owner] ?? null}
                  onClick={() => navigate(`/decks/${d.owner}/${d.id}`)}
                />
              ))}
              <NewSlot slot={nextSlot} onImport={() => setShowImport(true)} />
            </div>
          </Panel>

          {/* Recent imports — only shown when the signed-in user has a known owner */}
          {userOwner && recentImports.length > 0 && (
            <RecentImports
              imports={recentImports}
              onOpen={(deckKey) => navigate(`/decks/${deckKey}`)}
            />
          )}

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
              ['FRIENDS', String(friends.length)],
            ]} />
          </Panel>

          {/* Friends — only meaningful when the visitor has a known owner slug. */}
          {userOwner && (
            <Panel code="III.FR" title={`FRIENDS / / ${friends.length}`}>
              {friends.length === 0 ? (
                <div className="t-xs muted" style={{ padding: '8px 0', lineHeight: 1.6 }}>
                  &gt; NO FRIENDS YET<br />
                  &gt; ADD ONE FROM ANY DECK PAGE<span className="blink">_</span>
                </div>
              ) : (
                <div style={{ display: 'flex', flexDirection: 'column' }}>
                  {friends.map((f, i) => (
                    <div
                      key={f}
                      onClick={() => navigate(`/decks?q=${encodeURIComponent(f)}`)}
                      style={{
                        display: 'flex', justifyContent: 'space-between', alignItems: 'center',
                        padding: '6px 0',
                        borderBottom: i < friends.length - 1 ? '1px dashed var(--rule-2)' : 'none',
                        cursor: 'pointer',
                      }}
                      title={`View ${f}'s decks`}
                    >
                      <span className="t-md" style={{ fontWeight: 700, letterSpacing: '0.04em' }}>
                        <span style={{ color: 'var(--ok)', marginRight: 6 }}>●</span>{f.toUpperCase()}
                      </span>
                      <span className="t-xs muted-2">↗</span>
                    </div>
                  ))}
                </div>
              )}
            </Panel>
          )}

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
                <div>
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
