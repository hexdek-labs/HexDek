import { useState, useEffect } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Tag, Tape } from '../components/chrome'
import DeckShelf, { deckBracketLabel } from '../components/DeckShelf'
import { api, cardArtUrl } from '../services/api'
import { useAuth } from '../context/AuthContext'
import { useLiveSocket } from '../hooks/useLiveSocket'
import { useUploadDeck } from '../hooks/useUploadDeck'
import { MOCK_DECKS } from '../services/mock'
import ContextBox from '../components/ContextBox'
import {
  ARCHETYPE_CHIPS,
  BRACKET_CHIPS,
  COLOR_CHIPS,
  matchesArchetypeChip,
  matchesBracketChip,
  matchesColorChip,
} from './deckFilters'

const VIEW_KEY = 'hexdek_deck_view'
const SORT_KEY = 'hexdek_deck_sort'

// Bracket strings look like "B4", "B3.5", "B?". Pull out the numeric piece for sort;
// unknowns sink to the bottom regardless of direction.
function bracketSortValue(d) {
  const raw = d.pls || d.wbs || d.bracket
  const n = parseFloat(raw)
  return Number.isFinite(n) ? n : null
}

function compareDecks(a, b, key, dir, eloByDeckId) {
  const mult = dir === 'asc' ? 1 : -1
  const aKey = `${a.owner}/${a.id}`
  const bKey = `${b.owner}/${b.id}`
  const aElo = eloByDeckId[aKey] || eloByDeckId[a.id]
  const bElo = eloByDeckId[bKey] || eloByDeckId[b.id]
  const num = (x, y) => {
    if (x == null && y == null) return 0
    if (x == null) return 1   // missing always sinks
    if (y == null) return -1
    return (x - y) * mult
  }
  const str = (x, y) => (x || '').localeCompare(y || '') * mult
  switch (key) {
    case 'name':      return str(a.name || a.commander_card || a.commander, b.name || b.commander_card || b.commander)
    case 'commander': return str(a.commander_card || a.commander, b.commander_card || b.commander)
    case 'owner':     return str(a.owner, b.owner)
    case 'bracket':   return num(bracketSortValue(a), bracketSortValue(b))
    case 'elo':       return num(aElo?.rating, bElo?.rating)
    case 'record':    return num(aElo?.win_rate, bElo?.win_rate)
    default:          return 0
  }
}

export default function DeckList() {
  const [searchParams, setSearchParams] = useSearchParams()
  const [decks, setDecks] = useState([])
  const [filter, setFilter] = useState(searchParams.get('q') || '')
  const ownerParam = searchParams.get('owner') || ''
  const containsParam = searchParams.get('contains') || ''
  const { user } = useAuth()
  const [tab, setTab] = useState(
    ownerParam || containsParam ? 'all' :
    searchParams.get('tab') === 'all' ? 'all' :
    searchParams.get('tab') === 'mine' ? 'mine' :
    user ? 'mine' : 'all'
  )
  const [legalFilter, setLegalFilter] = useState('all')
  const [archetypeFilter, setArchetypeFilter] = useState(searchParams.get('archetype') || 'all')
  const [bracketFilter, setBracketFilter] = useState(searchParams.get('bracket') || 'all')
  const [colorFilter, setColorFilter] = useState(searchParams.get('color') || 'all')
  const [loading, setLoading] = useState(true)
  const [viewMode, setViewMode] = useState(() => {
    if (typeof localStorage === 'undefined') return 'shelf'
    return localStorage.getItem(VIEW_KEY) === 'list' ? 'list' : 'shelf'
  })
  const [sort, setSort] = useState(() => {
    if (typeof localStorage === 'undefined') return { key: 'elo', dir: 'desc' }
    try {
      const stored = JSON.parse(localStorage.getItem(SORT_KEY) || '')
      if (stored && stored.key && stored.dir) return stored
    } catch {
      // fall through to default
    }
    return { key: 'elo', dir: 'desc' }
  })
  const navigate = useNavigate()
  const { elo } = useLiveSocket()
  const upload = useUploadDeck(() => loadDecks())

  useEffect(() => {
    if (typeof localStorage !== 'undefined') localStorage.setItem(VIEW_KEY, viewMode)
  }, [viewMode])

  useEffect(() => {
    if (typeof localStorage !== 'undefined') localStorage.setItem(SORT_KEY, JSON.stringify(sort))
  }, [sort])

  const onSort = (key) => {
    setSort(s => s.key === key
      ? { key, dir: s.dir === 'asc' ? 'desc' : 'asc' }
      : { key, dir: (key === 'name' || key === 'commander' || key === 'owner') ? 'asc' : 'desc' })
  }

  useEffect(() => {
    const t = searchParams.get('tab')
    if (t === 'all' || t === 'mine') setTab(t)
  }, [searchParams])

  const loadDecks = () => {
    setLoading(true)
    api.getDecks({ owner: ownerParam, contains: containsParam })
      .then(setDecks)
      .catch(() => setDecks(MOCK_DECKS.map(d => ({ ...d, owner: 'josh' }))))
      .finally(() => setLoading(false))
  }

  useEffect(() => { loadDecks() }, [ownerParam, containsParam])

  const eloByDeckId = {}
  for (const e of elo) {
    if (e.deck_id) eloByDeckId[e.deck_id] = e
  }

  const storedOwner = typeof localStorage !== 'undefined' ? localStorage.getItem('hexdek_owner') : null
  const emailPrefix = user?.email?.split('@')[0]?.split('.')[0]?.toLowerCase() || ''
  const myName = storedOwner || user?.displayName?.toLowerCase() || emailPrefix || ''
  const myDecks = myName ? decks.filter(d => {
    const o = d.owner?.toLowerCase()
    return o === myName || myName.startsWith(o) || o.startsWith(myName)
  }) : []
  const hasMyDecks = myDecks.length > 0

  const baseDecks = (tab === 'mine' && user) ? myDecks : decks
  const hasLegalityData = decks.some(d => d.legal != null)
  const matched = baseDecks.filter(d => {
    if (legalFilter === 'legal' && d.legal === false) return false
    if (legalFilter === 'illegal' && d.legal !== false) return false
    if (!matchesArchetypeChip(d, archetypeFilter)) return false
    if (!matchesBracketChip(d, bracketFilter)) return false
    if (!matchesColorChip(d, colorFilter)) return false
    if (!filter) return true
    const q = filter.toLowerCase()
    const haystack = `${d.name} ${d.commander_card || ''} ${d.commander || ''} ${d.owner || ''}`.toLowerCase()
    return haystack.includes(q)
  })

  const filtered = [...matched].sort((a, b) => compareDecks(a, b, sort.key, sort.dir, eloByDeckId))

  const tapeLabel = tab === 'mine' && hasMyDecks
    ? `DECK ARCHIVE / / MY BUILDS`
    : `DECK ARCHIVE / / ALL BUILDS`

  return (
    <>
      <Tape left={tapeLabel} mid={`${filtered.length} / ${decks.length} TOTAL`} right="DOC HX-400" />

      <div style={{ padding: 18, display: 'flex', flexDirection: 'column', gap: 14 }}>
        {user && (
          <ContextBox id="decklist.intro">
            Browse and search every deck on the platform. Click any deck to open its archive — analysis, gauntlet results, decklist, and matchups.
            {' '}Use <strong>ADD YOUR DECK</strong> (in the list view, or the upload tile on the shelf) to import a Moxfield link or paste a decklist; Freya analyzes it automatically after upload.
          </ContextBox>
        )}
        {(ownerParam || containsParam) && (
          <div style={{ display: 'flex', gap: 10, alignItems: 'center', fontSize: 10, letterSpacing: '0.08em', textTransform: 'uppercase', color: 'var(--ink-2)' }}>
            <span>FILTER:</span>
            {ownerParam && (
              <Tag solid style={{ cursor: 'pointer' }} onClick={() => {
                const next = new URLSearchParams(searchParams)
                next.delete('owner')
                setSearchParams(next, { replace: true })
              }}>OWNER · {ownerParam.toUpperCase()} ✕</Tag>
            )}
            {containsParam && (
              <Tag solid style={{ cursor: 'pointer' }} onClick={() => {
                const next = new URLSearchParams(searchParams)
                next.delete('contains')
                setSearchParams(next, { replace: true })
              }}>CONTAINS · {containsParam.toUpperCase()} ✕</Tag>
            )}
          </div>
        )}
        {/* Tabs + Search */}
        <div style={{ display: 'flex', gap: 10, alignItems: 'center', flexWrap: 'wrap' }}>
          {user && (
            <>
              <Tag solid={tab === 'mine'} onClick={() => setTab('mine')} style={{ cursor: 'pointer' }}>MY DECKS</Tag>
              <Tag solid={tab === 'all'} onClick={() => setTab('all')} style={{ cursor: 'pointer' }}>ALL DECKS</Tag>
              <div style={{ width: 1, height: 16, background: 'var(--rule-2)' }} />
            </>
          )}
          {hasLegalityData && (
            <>
              <Tag solid={legalFilter === 'all'} onClick={() => setLegalFilter('all')} style={{ cursor: 'pointer' }}>ALL</Tag>
              <Tag solid={legalFilter === 'legal'} onClick={() => setLegalFilter('legal')} style={{ cursor: 'pointer', color: legalFilter === 'legal' ? undefined : 'var(--ok)' }}>✓ LEGAL</Tag>
              <Tag solid={legalFilter === 'illegal'} onClick={() => setLegalFilter('illegal')} style={{ cursor: 'pointer', color: legalFilter === 'illegal' ? undefined : 'var(--danger)' }}>✗ ILLEGAL</Tag>
              <div style={{ width: 1, height: 16, background: 'var(--rule-2)' }} />
            </>
          )}
          <div className="panel decklist-search" style={{ padding: 0, flex: 1, minWidth: 200, borderStyle: filter ? 'solid' : 'dashed' }}>
            <input
              type="text"
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              placeholder="SEARCH DECKS..."
              aria-label="Search decks"
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
          <div style={{ width: 1, height: 16, background: 'var(--rule-2)' }} />
          <Tag solid={viewMode === 'shelf'} onClick={() => setViewMode('shelf')} style={{ cursor: 'pointer' }}>SHELF</Tag>
          <Tag solid={viewMode === 'list'} onClick={() => setViewMode('list')} style={{ cursor: 'pointer' }}>LIST</Tag>
        </div>

        {/* Filter chip rows: bracket / color / archetype. Each chip mutates
            the matching URL param so deep-links carry the active filter. */}
        <ChipRow
          aria-label="Bracket filter"
          dataTestId="bracket-chips"
          legend="BRACKET"
          chips={BRACKET_CHIPS}
          active={bracketFilter}
          paramKey="bracket"
          onSelect={setBracketFilter}
          searchParams={searchParams}
          setSearchParams={setSearchParams}
        />
        <ChipRow
          aria-label="Color filter"
          dataTestId="color-chips"
          legend="COLOR"
          chips={COLOR_CHIPS}
          active={colorFilter}
          paramKey="color"
          onSelect={setColorFilter}
          searchParams={searchParams}
          setSearchParams={setSearchParams}
        />
        <ChipRow
          aria-label="Archetype filter"
          dataTestId="archetype-chips"
          legend="ARCHETYPE"
          chips={ARCHETYPE_CHIPS}
          active={archetypeFilter}
          paramKey="archetype"
          onSelect={setArchetypeFilter}
          searchParams={searchParams}
          setSearchParams={setSearchParams}
        />

        {/* Deck grid */}
        {loading ? (
          <div className="t-md muted" style={{ textAlign: 'center', padding: 36 }}>&gt; LOADING DECK ARCHIVE<span className="blink">_</span></div>
        ) : viewMode === 'shelf' ? (
          <DeckShelf decks={filtered.slice(0, 60)} eloByDeckId={eloByDeckId} navigate={navigate} onAddCard={upload.open} />
        ) : (
          <ListView decks={filtered.slice(0, 60)} eloByDeckId={eloByDeckId} navigate={navigate} onUpload={upload.open} sort={sort} onSort={onSort} />
        )}

        {filtered.length > 60 && (
          <div className="t-xs muted" style={{ textAlign: 'center' }}>
            &gt; SHOWING 60 / {filtered.length} — REFINE SEARCH TO SEE MORE
          </div>
        )}
      </div>

      {upload.modal}
    </>
  )
}

function ChipRow({ legend, chips, active, paramKey, onSelect, searchParams, setSearchParams, dataTestId, ...rest }) {
  return (
    <div
      role="group"
      data-testid={dataTestId}
      style={{
        display: 'flex',
        flexWrap: 'wrap',
        gap: 6,
        alignItems: 'center',
        overflowX: 'auto',
        WebkitOverflowScrolling: 'touch',
      }}
      {...rest}
    >
      <span
        className="t-xs muted"
        style={{ minWidth: 76, fontWeight: 700, letterSpacing: '0.08em' }}
      >
        {legend}
      </span>
      {chips.map(chip => (
        <Tag
          key={chip.id}
          solid={active === chip.id}
          onClick={() => {
            onSelect(chip.id)
            const next = new URLSearchParams(searchParams)
            if (chip.id === 'all') next.delete(paramKey)
            else next.set(paramKey, chip.id)
            setSearchParams(next, { replace: true })
          }}
          style={{ cursor: 'pointer', flexShrink: 0 }}
          data-chip-id={chip.id}
          data-chip-group={paramKey}
        >
          {chip.label}
        </Tag>
      ))}
    </div>
  )
}

function SortHeader({ sortKey, sort, onSort, children }) {
  const active = sort?.key === sortKey
  const arrow = active ? (sort.dir === 'asc' ? ' ▲' : ' ▼') : ''
  return (
    <span
      onClick={() => onSort?.(sortKey)}
      style={{
        cursor: 'pointer',
        color: active ? 'var(--ink)' : 'var(--ink-3)',
        userSelect: 'none',
      }}
      title={`Sort by ${sortKey}`}
    >
      {children}{arrow}
    </span>
  )
}

function ListView({ decks, eloByDeckId, navigate, onUpload, sort, onSort }) {
  return (
    <div className="panel" style={{ padding: 0 }}>
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: '40px 1fr 1fr 80px 60px 70px 100px',
          gap: 8,
          padding: '6px 10px',
          borderBottom: '1px solid var(--rule-2)',
          fontSize: 9,
          letterSpacing: '0.1em',
          color: 'var(--ink-3)',
          fontWeight: 700,
        }}
      >
        <span></span>
        <SortHeader sortKey="name" sort={sort} onSort={onSort}>NAME</SortHeader>
        <SortHeader sortKey="commander" sort={sort} onSort={onSort}>COMMANDER</SortHeader>
        <SortHeader sortKey="owner" sort={sort} onSort={onSort}>OWNER</SortHeader>
        <SortHeader sortKey="bracket" sort={sort} onSort={onSort}>BRACKET</SortHeader>
        <SortHeader sortKey="elo" sort={sort} onSort={onSort}>ELO</SortHeader>
        <SortHeader sortKey="record" sort={sort} onSort={onSort}>RECORD</SortHeader>
      </div>
      <div style={{ padding: '6px 10px 0', borderBottom: '1px solid var(--rule)' }}>
        <ContextBox id="decklist.import" compact>Click below to import a deck — paste a Moxfield URL or raw decklist. Freya analyzes it automatically (~10–20 seconds) and then redirects you to the new deck page.</ContextBox>
      </div>
      <div
        onClick={onUpload}
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          gap: 10,
          padding: '10px',
          borderBottom: '2px dashed var(--rule-2)',
          background: 'transparent',
          cursor: 'pointer',
          color: 'var(--ink)',
          fontWeight: 800,
          letterSpacing: '0.1em',
          fontSize: 12,
          textTransform: 'uppercase',
          transition: 'background 80ms ease, color 80ms ease',
        }}
        onMouseEnter={(e) => {
          e.currentTarget.style.background = 'var(--accent)'
          e.currentTarget.style.color = 'var(--bg)'
        }}
        onMouseLeave={(e) => {
          e.currentTarget.style.background = 'transparent'
          e.currentTarget.style.color = 'var(--ink)'
        }}
      >
        <span style={{ fontSize: 18, lineHeight: 1, fontWeight: 900 }}>+</span>
        <span>ADD YOUR DECK</span>
      </div>
      {decks.map((d) => {
        const deckKey = `${d.owner}/${d.id}`
        const deckElo = eloByDeckId[deckKey] || eloByDeckId[d.id]
        const cmdrName = d.commander_card || d.commander
        const bracketLabel = deckBracketLabel(d)
        return (
          <div
            key={deckKey}
            onClick={() => navigate(`/decks/${d.owner}/${d.id}`)}
            style={{
              display: 'grid',
              gridTemplateColumns: '40px 1fr 1fr 80px 60px 70px 100px',
              gap: 8,
              padding: '6px 10px',
              borderBottom: '1px solid var(--rule)',
              alignItems: 'center',
              cursor: 'pointer',
              fontSize: 11,
            }}
            onMouseEnter={(e) => { e.currentTarget.style.background = 'var(--panel-2)' }}
            onMouseLeave={(e) => { e.currentTarget.style.background = 'transparent' }}
          >
            <div
              className={cmdrName ? '' : 'hatch'}
              style={{
                width: 40,
                height: 28,
                overflow: 'hidden',
                border: '1px solid var(--rule-2)',
                background: 'var(--bg-2)',
              }}
            >
              {cmdrName && (
                <img
                  src={cardArtUrl(cmdrName)}
                  alt=""
                  loading="lazy"
                  style={{ width: '100%', height: '100%', objectFit: 'cover', display: 'block' }}
                  onError={(e) => {
                    e.target.style.display = 'none'
                    e.target.parentElement.classList.add('hatch')
                  }}
                />
              )}
            </div>
            <span style={{ fontWeight: 600, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{d.name || cmdrName}</span>
            <span className="muted" style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{cmdrName || '—'}</span>
            <span className="t-xs">{d.owner?.toUpperCase()}</span>
            <span style={{ fontWeight: 700, letterSpacing: '0.06em' }}>
              {bracketLabel}
              {d.legal != null && (
                <span style={{ marginLeft: 4, color: d.legal ? 'var(--ok)' : 'var(--danger)', fontSize: 9 }}>{d.legal ? '✓' : '✗'}</span>
              )}
            </span>
            <span style={{ fontWeight: 700 }}>{deckElo ? Math.round(deckElo.rating) : '—'}</span>
            <span className="t-xs">
              {deckElo && deckElo.games > 0 ? (
                <>
                  <span style={{ color: 'var(--ok)' }}>{deckElo.wins}</span>
                  <span className="muted"> · </span>
                  <span style={{ color: 'var(--danger)' }}>{deckElo.losses}</span>
                  <span className="muted"> ({deckElo.win_rate}%)</span>
                </>
              ) : (
                <span className="muted">—</span>
              )}
            </span>
          </div>
        )
      })}
    </div>
  )
}
