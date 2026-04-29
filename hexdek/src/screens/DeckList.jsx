import { useState, useEffect } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Panel, Tag, Btn, Tape } from '../components/chrome'
import { api, cardArtUrl } from '../services/api'
import { useAuth } from '../context/AuthContext'
import { useLiveSocket } from '../hooks/useLiveSocket'
import { MOCK_DECKS } from '../services/mock'

export default function DeckList() {
  const [searchParams] = useSearchParams()
  const [decks, setDecks] = useState([])
  const [filter, setFilter] = useState(searchParams.get('q') || '')
  const [tab, setTab] = useState('mine')
  const [loading, setLoading] = useState(true)
  const navigate = useNavigate()
  const { user } = useAuth()
  const { elo } = useLiveSocket()

  useEffect(() => {
    api.getDecks()
      .then(setDecks)
      .catch(() => setDecks(MOCK_DECKS.map(d => ({ ...d, owner: 'josh' }))))
      .finally(() => setLoading(false))
  }, [])

  const eloByDeckId = {}
  for (const e of elo) {
    if (e.deck_id) eloByDeckId[e.deck_id] = e
  }

  const myName = user?.displayName?.toLowerCase() || user?.email?.split('@')[0]?.toLowerCase() || ''
  const myDecks = myName ? decks.filter(d => d.owner?.toLowerCase() === myName) : []
  const hasMyDecks = myDecks.length > 0

  const baseDecks = (tab === 'mine' && hasMyDecks) ? myDecks : decks
  const filtered = baseDecks.filter(d => {
    if (!filter) return true
    const q = filter.toLowerCase()
    const haystack = `${d.name} ${d.commander_card || ''} ${d.commander || ''} ${d.owner || ''}`.toLowerCase()
    return haystack.includes(q)
  })

  const tapeLabel = tab === 'mine' && hasMyDecks
    ? `DECK ARCHIVE / / MY BUILDS`
    : `DECK ARCHIVE / / ALL BUILDS`

  return (
    <>
      <Tape left={tapeLabel} mid={`${filtered.length} / ${decks.length} TOTAL`} right="DOC HX-400" />

      <div style={{ padding: 18, flex: 1, display: 'flex', flexDirection: 'column', gap: 14, overflow: 'auto' }}>
        {/* Tabs + Search */}
        <div style={{ display: 'flex', gap: 10, alignItems: 'center', flexWrap: 'wrap' }}>
          {hasMyDecks && (
            <>
              <Tag solid={tab === 'mine'} onClick={() => setTab('mine')} style={{ cursor: 'pointer' }}>MY DECKS</Tag>
              <Tag solid={tab === 'all'} onClick={() => setTab('all')} style={{ cursor: 'pointer' }}>ALL DECKS</Tag>
              <div style={{ width: 1, height: 16, background: 'var(--rule-2)' }} />
            </>
          )}
          <div className="panel" style={{ padding: 0, flex: 1, minWidth: 200, borderStyle: filter ? 'solid' : 'dashed' }}>
            <input
              type="text"
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              placeholder="SEARCH DECKS..."
              style={{
                width: '100%',
                padding: '8px 12px',
                background: 'transparent',
                border: 'none',
                color: 'var(--ink)',
                fontFamily: 'inherit',
                fontSize: 11,
                letterSpacing: '0.06em',
                textTransform: 'uppercase',
                outline: 'none',
              }}
            />
          </div>
          <span className="t-xs muted">{filtered.length} MATCHES</span>
        </div>

        {/* Deck grid */}
        {loading ? (
          <div className="t-md muted" style={{ textAlign: 'center', padding: 36 }}>&gt; LOADING DECK ARCHIVE<span className="blink">_</span></div>
        ) : (
          <div className="grid col-4 gap-3">
            {filtered.slice(0, 60).map((d) => {
              const deckKey = `${d.owner}/${d.id}`
              const deckElo = eloByDeckId[deckKey] || eloByDeckId[d.id]
              return (
                <div
                  key={deckKey}
                  className="panel"
                  style={{ padding: 0, cursor: 'pointer' }}
                  onClick={() => navigate(`/decks/${d.owner}/${d.id}`)}
                >
                  <div className="panel-hd">
                    <span className="t-xs">{d.owner?.toUpperCase()}</span>
                    <span className="t-xs">{deckElo ? `ELO ${Math.round(deckElo.rating)}` : `B${d.bracket}`}</span>
                  </div>
                  <div style={{ aspectRatio: '1.4/1', borderBottom: '1px solid var(--rule-2)', position: 'relative', overflow: 'hidden' }} className={(d.commander_card || d.commander) ? '' : 'hatch'}>
                    {(d.commander_card || d.commander) ? (
                      <img
                        src={cardArtUrl(d.commander_card || d.commander)}
                        alt={d.commander_card || d.commander}
                        loading="lazy"
                        style={{ width: '100%', height: '100%', objectFit: 'cover', filter: 'saturate(0.5) contrast(1.1) brightness(0.85)' }}
                        onError={e => { e.target.style.display = 'none'; e.target.parentElement.classList.add('hatch') }}
                      />
                    ) : (
                      <span style={{ position: 'absolute', top: 4, left: 6, fontSize: 9, letterSpacing: '0.1em', color: 'var(--ink-3)' }}>CMDR</span>
                    )}
                  </div>
                  <div style={{ padding: '8px 10px' }}>
                    <div className="t-md" style={{ fontWeight: 700, lineHeight: 1.2, minHeight: 20 }}>{d.name || d.commander}</div>
                    {d.commander_card && d.commander_card.toUpperCase() !== (d.name || '').toUpperCase() && (
                      <div className="t-xs" style={{ marginTop: 2, color: 'var(--ink-2)', lineHeight: 1.2 }}>{d.commander_card}</div>
                    )}
                    <div style={{ display: 'flex', justifyContent: 'space-between', marginTop: 4, alignItems: 'center' }}>
                      <span className="t-xs muted">{d.card_count || d.cardCount} CARDS</span>
                      {deckElo && deckElo.games > 0 && (
                        <span className="t-xs" style={{ color: 'var(--ok)' }}>
                          {deckElo.wins}W-{deckElo.losses}L ({deckElo.win_rate}%)
                        </span>
                      )}
                    </div>
                  </div>
                </div>
              )
            })}
          </div>
        )}

        {filtered.length > 60 && (
          <div className="t-xs muted" style={{ textAlign: 'center' }}>
            &gt; SHOWING 60 / {filtered.length} — REFINE SEARCH TO SEE MORE
          </div>
        )}
      </div>
    </>
  )
}
