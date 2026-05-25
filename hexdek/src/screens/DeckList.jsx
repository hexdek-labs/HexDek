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
import { toast } from '../components/Toast'
import {
  ARCHETYPE_CHIPS,
  BRACKET_CHIPS,
  COLOR_CHIPS,
  matchesArchetypeChip,
  matchesBracketChip,
  matchesColorChip,
} from './deckFilters'
import {
  inferCommander,
  parseDeckLines,
  summarize,
} from '../lib/deckParser'
import {
  POWER_TIER_KEYS,
  POWER_TIER_LABELS,
  peakFlavor,
  summarizePowerTiers,
  topArchetypes,
} from '../lib/deckPowerSummary'

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

        {/* QUICK PASTE — collapsed by default. Lets a signed-in user
            drop a decklist into the page and create a new deck without
            walking through the full import modal. Auto-derives name
            from commander + owner from session, so the textarea is the
            only required input for the happy path. */}
        {user && (
          <QuickPasteImport
            onImported={() => loadDecks()}
            navigate={navigate}
            defaultOwner={myName}
          />
        )}

        {/* Power-tier dashboard summary — only renders on the MY DECKS
            tab once the collection has ≥3 decks. Three is the
            minimum threshold where the distribution starts to read
            as a "collection" rather than incidental noise. */}
        {tab === 'mine' && hasMyDecks && myDecks.length >= 3 && (
          <DeckPowerSummaryPanel decks={myDecks} />
        )}

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

// Per-tier visual hue. Kept in sync with the BRACKET chip palette
// elsewhere on the deck shelf — bracket 1/2 lean cool, 3 neutral,
// 4/5 hot — so a user scanning chrome on /decks reads "your decks"
// and "filter by bracket" with the same visual vocabulary.
const POWER_TIER_COLORS = {
  b1: '#82c472',          // green — casual
  b2: '#6e8fa0',          // blue-gray — focused
  b3: '#cda73c',          // amber — optimized
  b4: '#cc5c4a',          // red — high power
  b5: '#9c6ab0',          // purple — cEDH
  unknown: 'var(--ink-3)',
}

function DeckPowerSummaryPanel({ decks }) {
  const summary = summarizePowerTiers(decks)
  const top = topArchetypes(decks, 5)
  const flavor = peakFlavor(summary)
  const segments = POWER_TIER_KEYS
    .map(k => ({ k, count: summary.counts[k], pct: summary.percentages[k] }))
    .filter(s => s.count > 0)

  return (
    <div
      data-testid="deck-power-summary"
      style={{
        border: '1px solid var(--rule-2)',
        background: 'var(--bg-2, rgba(0,0,0,0.2))',
        padding: 12,
        display: 'flex',
        flexDirection: 'column',
        gap: 10,
      }}
    >
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', gap: 12, flexWrap: 'wrap' }}>
        <span className="t-xs muted" style={{ letterSpacing: '0.08em', fontWeight: 700 }}>
          MY COLLECTION / / POWER-TIER DISTRIBUTION
        </span>
        <span className="t-xs muted" style={{ letterSpacing: '0.06em' }}>
          {summary.total} DECKS
        </span>
      </div>

      {/* Stacked bar — one segment per non-zero tier, width proportional
          to that tier's share. Each segment carries a `data-tier` attr
          for the e2e suite to assert against without re-deriving the
          percentages from rendered styles. */}
      <div
        data-testid="deck-power-tier-bar"
        role="img"
        aria-label={flavor || 'No bracket data'}
        style={{
          display: 'flex',
          width: '100%',
          height: 18,
          border: '1px solid var(--rule)',
          background: 'var(--bg, #181915)',
          overflow: 'hidden',
        }}
      >
        {segments.length === 0 ? (
          <div className="t-xs muted" style={{ alignSelf: 'center', margin: '0 auto', letterSpacing: '0.06em' }}>
            — NO TIER DATA —
          </div>
        ) : segments.map(seg => (
          <div
            key={seg.k}
            data-tier={seg.k}
            data-count={seg.count}
            title={`${POWER_TIER_LABELS[seg.k]} — ${seg.count} (${seg.pct}%)`}
            style={{
              width: `${seg.pct}%`,
              minWidth: seg.count > 0 ? 4 : 0, // visible sliver even at 1%
              background: POWER_TIER_COLORS[seg.k],
              borderRight: '1px solid rgba(0,0,0,0.25)',
            }}
          />
        ))}
      </div>

      {/* Per-tier card row */}
      <div
        data-testid="deck-power-tier-cards"
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(120px, 1fr))',
          gap: 8,
        }}
      >
        {POWER_TIER_KEYS.map(k => {
          const count = summary.counts[k]
          const pct = summary.percentages[k]
          const dim = count === 0
          return (
            <div
              key={k}
              data-tier={k}
              style={{
                padding: '6px 8px',
                border: '1px solid var(--rule-2)',
                background: dim ? 'transparent' : 'rgba(255,255,255,0.02)',
                opacity: dim ? 0.45 : 1,
                display: 'flex',
                flexDirection: 'column',
                gap: 2,
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                <span style={{ width: 8, height: 8, background: POWER_TIER_COLORS[k], border: '1px solid var(--rule-2)' }} />
                <span className="t-xs muted" style={{ letterSpacing: '0.06em', fontWeight: 700 }}>
                  {POWER_TIER_LABELS[k]}
                </span>
              </div>
              <div style={{ fontSize: 18, fontWeight: 700, fontVariantNumeric: 'tabular-nums' }}>
                {count}
              </div>
              <div className="t-xs muted" style={{ fontVariantNumeric: 'tabular-nums' }}>
                {pct}%
              </div>
            </div>
          )
        })}
      </div>

      {flavor && (
        <div className="t-xs" style={{ letterSpacing: '0.06em', color: 'var(--ink-2)' }}>
          {flavor}
        </div>
      )}

      {top.length > 0 && (
        <div data-testid="deck-power-top-archetypes" style={{ display: 'flex', alignItems: 'baseline', gap: 8, flexWrap: 'wrap' }}>
          <span className="t-xs muted" style={{ letterSpacing: '0.08em', fontWeight: 700 }}>
            TOP ARCHETYPES:
          </span>
          {top.map((r, i) => (
            <span key={r.archetype} className="t-xs" style={{ letterSpacing: '0.04em' }}>
              <strong style={{ color: 'var(--ink)' }}>{r.archetype.toUpperCase()}</strong>
              <span className="muted"> ×{r.count}</span>
              {i < top.length - 1 ? <span className="muted"> · </span> : null}
            </span>
          ))}
        </div>
      )}
    </div>
  )
}

function QuickPasteImport({ onImported, navigate, defaultOwner }) {
  const [open, setOpen] = useState(false)
  const [text, setText] = useState('')
  const [name, setName] = useState('')
  const [importing, setImporting] = useState(false)
  const [error, setError] = useState(null)

  const parsed = parseDeckLines(text)
  const detectedCommander = inferCommander(text) || parsed.find(c => c.isCommander)?.name || ''
  const summary = summarize(parsed)
  const effectiveName = name.trim() || detectedCommander || (parsed.length > 0 ? parsed[0].name : '')

  const canSubmit = !importing && parsed.length > 0 && (effectiveName || detectedCommander)

  const submit = async () => {
    if (!canSubmit) return
    setError(null)
    setImporting(true)
    try {
      const ownerToUse = defaultOwner?.trim() || 'imported'
      const finalName = effectiveName || detectedCommander
      // Server expects either an embedded "COMMANDER: X" line or a
      // commander designated via the parsed isCommander flag — keep
      // the raw text so the full import flow's commander detection
      // (which mirrors lib/deckParser.inferCommander) finds it.
      const result = await api.importDeck(finalName, ownerToUse, text, [])
      onImported?.()
      const owner = result?.owner || ownerToUse
      const id = result?.id
      setText('')
      setName('')
      setOpen(false)
      setImporting(false)
      if (id) navigate(`/decks/${encodeURIComponent(owner)}/${encodeURIComponent(id)}`)
      else toast.success('DECK IMPORTED')
    } catch (err) {
      setImporting(false)
      setError(err?.message || 'IMPORT FAILED')
    }
  }

  if (!open) {
    return (
      <button
        type="button"
        data-testid="quick-paste-toggle"
        onClick={() => setOpen(true)}
        style={{
          alignSelf: 'flex-start',
          background: 'transparent',
          border: '1px dashed var(--rule-2)',
          padding: '6px 12px',
          color: 'var(--ink-2)',
          font: 'inherit',
          fontSize: 11,
          letterSpacing: '0.08em',
          textTransform: 'uppercase',
          cursor: 'pointer',
        }}
      >
        + QUICK PASTE A DECKLIST
      </button>
    )
  }

  return (
    <div
      data-testid="quick-paste-panel"
      style={{
        border: '1px solid var(--rule-2)',
        background: 'var(--bg-2, rgba(0,0,0,0.2))',
        padding: 10,
        display: 'flex',
        flexDirection: 'column',
        gap: 8,
      }}
    >
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline' }}>
        <span className="t-xs muted" style={{ letterSpacing: '0.08em', fontWeight: 700 }}>
          QUICK PASTE / / PLAIN DECKLIST
        </span>
        <button
          type="button"
          onClick={() => { setOpen(false); setError(null); setText(''); setName('') }}
          style={{ background: 'transparent', border: 0, color: 'var(--ink-3)', cursor: 'pointer', fontSize: 11 }}
        >
          ✕ CLOSE
        </button>
      </div>
      <textarea
        data-testid="quick-paste-textarea"
        value={text}
        onChange={e => setText(e.target.value)}
        placeholder={`Paste a decklist. Supports "1 Sol Ring", "1x Sol Ring (CMM) 339", "Commander: Atraxa, Praetors' Voice", sideboard sections, # comments…`}
        rows={10}
        spellCheck={false}
        style={{
          width: '100%',
          padding: 8,
          background: 'var(--bg-2, rgba(0,0,0,0.3))',
          border: '1px solid var(--rule-2)',
          color: 'var(--ink)',
          fontFamily: 'inherit',
          fontSize: 11,
          letterSpacing: '0.04em',
          lineHeight: 1.6,
          resize: 'vertical',
        }}
      />
      <div
        data-testid="quick-paste-summary"
        style={{ display: 'flex', flexWrap: 'wrap', gap: 12, alignItems: 'baseline', fontSize: 11 }}
      >
        <span className="t-xs muted" style={{ letterSpacing: '0.08em' }}>
          PARSED:{' '}
          <strong style={{ color: 'var(--ink)' }}>{summary.cardCount}</strong>{' '}
          CARDS · {summary.uniqueCount} UNIQUE
        </span>
        {detectedCommander && (
          <span className="t-xs muted" style={{ letterSpacing: '0.08em' }}>
            COMMANDER:{' '}
            <strong style={{ color: 'var(--ink)' }}>{detectedCommander.toUpperCase()}</strong>
          </span>
        )}
        {summary.hasIllegalMultiples && (
          <span className="t-xs" style={{ color: 'var(--warn, #c0a060)', letterSpacing: '0.08em' }}>
            ⚠ MULTIPLES OF A NON-BASIC DETECTED
          </span>
        )}
      </div>
      <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
        <input
          type="text"
          value={name}
          onChange={e => setName(e.target.value)}
          placeholder={detectedCommander ? `Deck name (defaults to ${detectedCommander})` : 'Deck name'}
          aria-label="Deck name"
          style={{
            flex: 1, minWidth: 200,
            padding: '6px 10px',
            background: 'transparent',
            border: '1px solid var(--rule-2)',
            color: 'var(--ink)',
            font: 'inherit',
            fontSize: 11,
            letterSpacing: '0.04em',
          }}
        />
        <button
          type="button"
          data-testid="quick-paste-submit"
          disabled={!canSubmit}
          onClick={submit}
          style={{
            background: canSubmit ? 'var(--accent, var(--ink))' : 'transparent',
            color: canSubmit ? 'var(--bg)' : 'var(--ink-3)',
            border: '1px solid var(--rule-2)',
            padding: '6px 14px',
            font: 'inherit',
            fontSize: 11,
            letterSpacing: '0.1em',
            cursor: canSubmit ? 'pointer' : 'not-allowed',
            fontWeight: 700,
            textTransform: 'uppercase',
          }}
        >
          {importing ? 'IMPORTING…' : 'IMPORT + ANALYZE'}
        </button>
      </div>
      {error && (
        <div className="t-xs" style={{ color: 'var(--danger)', letterSpacing: '0.06em' }}>
          ✗ {error.toUpperCase()}
        </div>
      )}
    </div>
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
