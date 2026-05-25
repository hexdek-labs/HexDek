import { useState, useEffect, useMemo, useRef } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { Panel, KV, Bar, Tag, Btn, Tape, ConfidenceDots, ManaCurveChart, ColorPie, computeColorByCmc } from '../components/chrome'
import CreditsPanel from '../components/CreditsPanel'
import GlossaryTerm from '../components/GlossaryTerm'
import { ConsiderCuttingRationale, ValueEngineRationale, WinConditionRationale } from '../components/RationalePanels'
import CardRolesGrid from '../components/CardRolesGrid'
import CardLink from '../components/CardLink'
import CurseDisplay from '../components/CurseDisplay'
import MatchupsPanel from '../components/MatchupsPanel'
import TagInput from '../components/TagInput'
import ManaCost from '../components/ManaCost'
import { AchievementsPanel, BadgeShowcase } from '../components/AchievementsPanel'
import { toast } from '../components/Toast'
import { api, cardArtUrl, cardImageUrl, API_BASE } from '../services/api'
import { useArtContrast } from '../hooks/useArtContrast'
import { useLiveSocket } from '../hooks/useLiveSocket'
import { useModalKeyboard } from '../hooks/useModalKeyboard'
import { useAuth } from '../context/AuthContext'
import { trackEvent } from '../hooks/useAnalytics'
import { DeckPicker } from './DeckCompare'
import DeckExportModal from '../components/DeckExportModal'
import ContextBox from '../components/ContextBox'
import EloSparkline from '../components/EloSparkline'
import ArchetypeChipRow from '../components/ArchetypeChipRow'
import CoachingMarker from '../components/CoachingMarker'
import { buildCoachingIndex, coachingForCard } from '../lib/freyaCoaching'
import DeckRating from '../components/DeckRating'
import DeckShareDisclosure from '../components/DeckShareDisclosure'
import BracketChangelog from '../components/BracketChangelog'
import { deckGlanceStats } from '../lib/deckStats'
import { diffDeckText, diffSummary } from '../lib/deckHistoryDiff'
import { formatUSD, summarize as summarizeBudget } from '../lib/deckBudget'
import {
  cardCMCForSort,
  cardColorIdentityString,
  cardRole,
  cardTypeBucket,
  sortCards,
  toggleSort,
} from '../lib/cardListDense'
import {
  applySuggestion,
  extractFragment,
  MIN_FRAGMENT_CHARS,
  nextSuggestionIndex,
} from './textareaAutocomplete'
import {
  outcomeForDeck,
  opponentCommanders,
  formatRelativeFinished,
  summarizeRecentGames,
} from '../lib/recentGames'

// Brutalist stat-summary panel: mana curve, card-type breakdown, color
// pips. Computed entirely from the in-memory deck card list — no extra
// API roundtrip, so it renders instantly even when Freya analysis hasn't
// run yet. The deeper Freya-driven curve / color charts live in the
// ANALYSIS tab; this is the always-visible top-of-page summary.
const TYPE_BUCKETS = [
  // Highest priority first — a card lands in the first bucket whose
  // keyword appears in its type_line. Land beats everything (so artifact
  // lands count as lands), Creature beats Artifact/Enchantment (so
  // enchantment-creatures and artifact-creatures count as creatures —
  // matches EDHREC convention).
  { key: 'land',         label: 'LANDS',        match: /\bland\b/i,         color: '#8a9682' },
  { key: 'planeswalker', label: 'PLANESWALKERS', match: /\bplaneswalker\b/i, color: '#cda73c' },
  { key: 'creature',     label: 'CREATURES',    match: /\bcreature\b/i,     color: '#82C472' },
  { key: 'enchantment',  label: 'ENCHANTMENTS', match: /\benchantment\b/i,  color: '#b48ad6' },
  { key: 'artifact',     label: 'ARTIFACTS',    match: /\bartifact\b/i,     color: '#9aa6b8' },
  { key: 'sorcery',      label: 'SORCERIES',    match: /\bsorcery\b/i,      color: '#cc5c4a' },
  { key: 'instant',      label: 'INSTANTS',     match: /\binstant\b/i,      color: '#6e8fa0' },
]
const PIP_COLORS = { W: '#E0EBD3', U: '#6E8FA0', B: '#3a3628', R: '#CC5C4A', G: '#82C472' }

function computeDeckStats(cards) {
  const curve = [0, 0, 0, 0, 0, 0, 0, 0] // 0..6, 7+
  const types = Object.fromEntries(TYPE_BUCKETS.map(b => [b.key, 0]))
  let typesTotal = 0
  const pips = { W: 0, U: 0, B: 0, R: 0, G: 0 }
  let pipsTotal = 0

  for (const c of cards || []) {
    const qty = c.quantity || 1
    const typeStr = (c.type_line || (Array.isArray(c.types) ? c.types.join(' ') : '') || '').toLowerCase()
    const isLand = /\bland\b/.test(typeStr) || ((c.cmc ?? -1) === 0 && !c.mana_cost && !typeStr)

    // Mana curve — non-land only.
    if (!isLand) {
      const cmc = Math.max(0, Math.min(7, c.cmc ?? 0))
      curve[cmc] += qty
    }

    // Type bucket — first match wins.
    if (typeStr) {
      const bucket = TYPE_BUCKETS.find(b => b.match.test(typeStr))
      if (bucket) {
        types[bucket.key] += qty
        typesTotal += qty
      }
    } else if (isLand) {
      types.land += qty
      typesTotal += qty
    }

    // Color pips — count {W}{U}{B}{R}{G} in mana_cost, including hybrid
    // halves like {W/U} (each half scores once for its color).
    if (c.mana_cost) {
      const matches = c.mana_cost.match(/[WUBRG]/gi) || []
      for (const m of matches) {
        const k = m.toUpperCase()
        if (pips[k] != null) { pips[k] += qty; pipsTotal += qty }
      }
    }
  }

  return { curve, types, typesTotal, pips, pipsTotal }
}

function DeckStatsSummary({ cards }) {
  const { curve, types, typesTotal, pips, pipsTotal } = computeDeckStats(cards)
  const curveMax = Math.max(1, ...curve)
  const curveLabels = ['0', '1', '2', '3', '4', '5', '6', '7+']

  // Pie geometry — one circle, segments drawn as stroked arcs via
  // stroke-dasharray. circumference 2πr; r=15.9155 keeps circumference≈100
  // so dasharray values are simply percentages.
  const segments = TYPE_BUCKETS.map(b => ({
    bucket: b,
    count: types[b.key],
    pct: typesTotal > 0 ? (types[b.key] / typesTotal) * 100 : 0,
  })).filter(s => s.count > 0)
  let pieOffset = 25 // shift starting angle to 12 o'clock
  const pieSegs = segments.map(s => {
    const seg = { ...s, offset: pieOffset }
    pieOffset += s.pct
    return seg
  })

  const pipMax = Math.max(1, ...Object.values(pips))

  return (
    <Panel code="04.S" title="DECK STATS" right={<span className="t-xs muted">{(cards || []).reduce((n, c) => n + (c.quantity || 1), 0)} CARDS</span>}>
      <div className="deck-stats-summary">
        {/* Mana curve histogram */}
        <div className="deck-stats-summary__col">
          <div className="t-xs muted" style={{ marginBottom: 6 }}>MANA CURVE / / NONLAND CMC</div>
          <svg viewBox="0 0 200 90" preserveAspectRatio="none" style={{ width: '100%', height: 90, display: 'block', border: '1px solid var(--rule-2)' }}>
            {curve.map((n, i) => {
              const w = 200 / curve.length
              const x = i * w
              const h = (n / curveMax) * 70
              const y = 80 - h
              return (
                <g key={i}>
                  <rect x={x + 2} y={y} width={w - 4} height={h} fill="var(--accent, var(--ink))" />
                  {n > 0 && (
                    <text x={x + w / 2} y={y - 2} textAnchor="middle" fontSize="7" fill="var(--ink-2)" fontFamily="inherit">{n}</text>
                  )}
                  <text x={x + w / 2} y={88} textAnchor="middle" fontSize="8" fill="var(--ink-3)" fontFamily="inherit" letterSpacing="0.05em">{curveLabels[i]}</text>
                </g>
              )
            })}
          </svg>
        </div>

        {/* Card type breakdown — pie + legend */}
        <div className="deck-stats-summary__col">
          <div className="t-xs muted" style={{ marginBottom: 6 }}>CARD TYPES / / {typesTotal}</div>
          <div style={{ display: 'flex', gap: 10, alignItems: 'center' }}>
            <svg viewBox="0 0 42 42" style={{ width: 90, height: 90, flexShrink: 0 }}>
              <circle cx="21" cy="21" r="15.9155" fill="var(--bg-2, #181915)" stroke="var(--rule-2)" strokeWidth="0.4" />
              {pieSegs.map(s => (
                <circle
                  key={s.bucket.key}
                  cx="21" cy="21" r="15.9155"
                  fill="transparent"
                  stroke={s.bucket.color}
                  strokeWidth="9"
                  strokeDasharray={`${s.pct.toFixed(2)} ${(100 - s.pct).toFixed(2)}`}
                  strokeDashoffset={(100 - s.offset).toFixed(2)}
                  transform="rotate(-90 21 21)"
                >
                  <title>{`${s.bucket.label}: ${s.count} (${s.pct.toFixed(1)}%)`}</title>
                </circle>
              ))}
            </svg>
            <div style={{ display: 'grid', gridTemplateColumns: 'auto 1fr auto', gap: '2px 6px', flex: 1, fontSize: 9, alignContent: 'center' }}>
              {TYPE_BUCKETS.map(b => {
                const n = types[b.key]
                if (n === 0) return null
                const pct = typesTotal > 0 ? (n / typesTotal) * 100 : 0
                return (
                  <div key={b.key} style={{ display: 'contents' }}>
                    <span style={{ width: 8, height: 8, background: b.color, border: '1px solid var(--rule-2)', alignSelf: 'center' }} />
                    <span style={{ letterSpacing: '0.05em' }}>{b.label}</span>
                    <span style={{ fontVariantNumeric: 'tabular-nums', textAlign: 'right' }}>{n} · {pct.toFixed(0)}%</span>
                  </div>
                )
              })}
            </div>
          </div>
        </div>

        {/* Color pip distribution */}
        <div className="deck-stats-summary__col">
          <div className="t-xs muted" style={{ marginBottom: 6 }}>COLOR PIPS / / {pipsTotal}</div>
          {pipsTotal === 0 ? (
            <div className="t-xs muted-2" style={{ padding: '14px 0', textAlign: 'center' }}>— COLORLESS —</div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
              {Object.entries(pips).filter(([, n]) => n > 0).map(([color, n]) => {
                const pct = (n / pipsTotal) * 100
                const barW = (n / pipMax) * 100
                return (
                  <div key={color} style={{ display: 'grid', gridTemplateColumns: '14px 1fr 56px', alignItems: 'center', gap: 6 }}>
                    <span style={{ fontSize: 11, fontWeight: 700, textAlign: 'center' }}>{color}</span>
                    <div style={{ height: 10, border: '1px solid var(--rule-2)', background: 'var(--bg-2, rgba(0,0,0,0.2))', position: 'relative' }}>
                      <div style={{ position: 'absolute', inset: 0, width: `${barW}%`, background: PIP_COLORS[color], opacity: 0.85 }} />
                    </div>
                    <span className="t-xs" style={{ textAlign: 'right', fontVariantNumeric: 'tabular-nums' }}>{n} · {pct.toFixed(0)}%</span>
                  </div>
                )
              })}
            </div>
          )}
        </div>
      </div>
    </Panel>
  )
}

const CardThumb = ({ name, cmc, score, compact }) => {
  const imgUrl = cardArtUrl(name)
  // Whole tile is a CardLink. underline=false because the click
  // affordance is the art tile itself; a dotted underline on a 5/7
  // image would be visual noise.
  if (compact) {
    return (
      <CardLink name={name} underline={false} style={{ display: 'block' }}>
        <div className="panel" style={{ padding: 0 }}>
          <div style={{ aspectRatio: '5/4', position: 'relative', overflow: 'hidden' }}>
            <img src={imgUrl} alt={name} style={{ width: '100%', height: '100%', objectFit: 'cover', filter: 'saturate(0.6) contrast(1.1)' }} onError={e => { e.target.style.display = 'none'; e.target.parentElement.classList.add('hatch') }} />
          </div>
          <div style={{ padding: '3px 5px' }}>
            <div style={{ fontSize: 7, fontWeight: 700, letterSpacing: '0.04em', textTransform: 'uppercase', lineHeight: 1.1, minHeight: 14, overflow: 'hidden', textOverflow: 'ellipsis' }}>{name}</div>
          </div>
        </div>
      </CardLink>
    )
  }
  return (
    <CardLink name={name} underline={false} style={{ display: 'block' }}>
      <div className="panel" style={{ padding: 0 }}>
        <div style={{ aspectRatio: '5/7', borderBottom: '1px solid var(--rule-2)', position: 'relative', overflow: 'hidden' }}>
          <img src={imgUrl} alt={name} style={{ width: '100%', height: '100%', objectFit: 'cover', filter: 'saturate(0.6) contrast(1.1)' }} onError={e => { e.target.style.display = 'none'; e.target.parentElement.classList.add('hatch') }} />
          <span style={{ position: 'absolute', top: 4, left: 5, background: 'rgba(12,13,10,0.6)', padding: '0 3px' }} className="t-xs muted-2">{cmc || ''}</span>
          {score && <span style={{ position: 'absolute', top: 4, right: 5, fontSize: 9, color: 'var(--ok)' }}>■{score}</span>}
        </div>
        <div style={{ padding: '5px 7px' }}>
          <div style={{ fontSize: 9, fontWeight: 700, letterSpacing: '0.04em', textTransform: 'uppercase', lineHeight: 1.2, minHeight: 24 }}>{name}</div>
        </div>
      </div>
    </CardLink>
  )
}

// CardListDense — sortable spreadsheet alternative to CardRolesGrid.
// Same source-of-truth (Freya-tagged card list); trades large tile
// images for a flat scannable table when the user wants to compare
// CMC / type / role across the whole 99.
//
// Sort state is owned by the parent so a tab switch + return doesn't
// jolt the user back to a default column they didn't pick. Stable
// sort: identical-key cards always fall back to name ascending.
const DENSE_TYPE_LABEL = {
  creature: 'CRE',
  planeswalker: 'PW',
  instant: 'INS',
  sorcery: 'SOR',
  artifact: 'ART',
  enchantment: 'ENC',
  land: 'LND',
  other: '—',
}

function DenseSortHeader({ label, sortKey, sort, onSort, align = 'left' }) {
  const active = sort?.key === sortKey
  const arrow = active ? (sort.dir === 'asc' ? '▲' : '▼') : ''
  return (
    <button
      type="button"
      onClick={() => onSort(sortKey)}
      style={{
        background: 'transparent',
        border: 0,
        padding: 0,
        font: 'inherit',
        cursor: 'pointer',
        color: active ? 'var(--ink)' : 'var(--ink-3)',
        textAlign: align,
        letterSpacing: '0.08em',
        fontWeight: 700,
        textTransform: 'uppercase',
        fontSize: 9,
        userSelect: 'none',
      }}
      data-testid={`dense-col-${sortKey}`}
      aria-sort={active ? (sort.dir === 'asc' ? 'ascending' : 'descending') : 'none'}
    >
      {label}{arrow ? ` ${arrow}` : ''}
    </button>
  )
}

function CardListDense({ cards, cardRoles, sort, onSort, coachingIndex, onCut }) {
  const rows = useMemo(
    () => sortCards(cards, sort.key, sort.dir, cardRoles),
    [cards, sort.key, sort.dir, cardRoles],
  )

  if (!rows.length) {
    return (
      <div className="t-xs muted" style={{ padding: '14px 0', textAlign: 'center' }}>
        &gt; NO CARDS TO LIST
      </div>
    )
  }

  // Grid template — picked so QTY + CMC sit as right-aligned tabular
  // numerics, name eats the rest, and TYPE/ROLE/COLOR pin to fixed
  // narrow columns on the right.
  const cols = '32px 1fr 130px 42px 56px 56px 42px'

  return (
    <div data-testid="card-list-dense">
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: cols,
          gap: 6,
          padding: '4px 6px',
          borderBottom: '1px solid var(--rule-2)',
          alignItems: 'baseline',
        }}
      >
        <DenseSortHeader label="QTY"   sortKey="qty"   sort={sort} onSort={onSort} align="right" />
        <DenseSortHeader label="NAME"  sortKey="name"  sort={sort} onSort={onSort} />
        <DenseSortHeader label="MANA"  sortKey="cmc"   sort={sort} onSort={onSort} />
        <DenseSortHeader label="CMC"   sortKey="cmc"   sort={sort} onSort={onSort} align="right" />
        <DenseSortHeader label="TYPE"  sortKey="type"  sort={sort} onSort={onSort} />
        <DenseSortHeader label="ROLE"  sortKey="role"  sort={sort} onSort={onSort} />
        <DenseSortHeader label="COLOR" sortKey="color" sort={sort} onSort={onSort} />
      </div>
      {rows.map((c, i) => {
        const linkName = (c.name || '').replace(/^COMMANDER:\s*/i, '').trim()
        const bucket = cardTypeBucket(c)
        const cmcSort = cardCMCForSort(c)
        const cmcDisplay = bucket === 'land' ? '—' : String(c.cmc ?? '—')
        const role = cardRole(c, cardRoles) || ''
        const color = cardColorIdentityString(c) || (bucket === 'land' ? '—' : 'C')
        return (
          <div
            key={`${c.name}-${i}`}
            data-testid="card-list-dense-row"
            data-card-name={c.name}
            style={{
              display: 'grid',
              gridTemplateColumns: cols,
              gap: 6,
              padding: '3px 6px',
              borderBottom: i < rows.length - 1 ? '1px dotted var(--rule)' : 'none',
              alignItems: 'center',
              fontSize: 11,
            }}
          >
            <span
              style={{
                textAlign: 'right',
                fontVariantNumeric: 'tabular-nums',
                color: (c.quantity || 1) > 1 ? 'var(--ink)' : 'var(--ink-2)',
                fontWeight: (c.quantity || 1) > 1 ? 700 : 400,
              }}
            >
              {c.quantity || 1}
            </span>
            <span style={{ display: 'flex', alignItems: 'center', minWidth: 0 }}>
              <CardLink
                name={linkName}
                className="t-xs"
                style={{
                  borderBottom: 'none',
                  minWidth: 0,
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap',
                }}
              >
                {c.name}
              </CardLink>
              {coachingIndex && (
                <CoachingMarker
                  entry={coachingForCard(coachingIndex, c.name)}
                  onCut={onCut}
                />
              )}
            </span>
            <span style={{ display: 'flex', alignItems: 'center', minHeight: 14 }}>
              {c.mana_cost ? <ManaCost cost={c.mana_cost} size={11} gap={1} /> : <span className="t-xs muted">—</span>}
            </span>
            <span style={{ textAlign: 'right', fontVariantNumeric: 'tabular-nums', color: 'var(--ink-2)' }}>
              {cmcDisplay}
              {/* hidden sort value for tests / debugging; keeps the
                  visible number distinct from the -1 land sort key */}
              <span style={{ display: 'none' }} data-cmc-sort={cmcSort}></span>
            </span>
            <span className="t-xs" style={{ letterSpacing: '0.06em', color: 'var(--ink-3)' }}>
              {DENSE_TYPE_LABEL[bucket]}
            </span>
            <span className="t-xs" style={{ letterSpacing: '0.06em', color: role ? 'var(--ink-2)' : 'var(--ink-3)' }}>
              {role ? role.toUpperCase() : '—'}
            </span>
            <span
              className="t-xs"
              style={{
                letterSpacing: '0.04em',
                color: color === 'C' || color === '—' ? 'var(--ink-3)' : 'var(--ink-2)',
                fontWeight: 700,
              }}
            >
              {color}
            </span>
          </div>
        )
      })}
    </div>
  )
}

// WorkshopAddCard — typeahead-style card-add input for the Workshop
// editor. Debounced search against /api/cards/search, dropdown shows
// up to 6 matches, Enter or click appends. Lets the user add cards
// without manually typing the card name into the textarea (with all
// the typo risk that implies).
function WorkshopAddCard({ onAdd }) {
  const [q, setQ] = useState('')
  const [results, setResults] = useState([])
  const [focused, setFocused] = useState(false)
  useEffect(() => {
    if (!q.trim() || q.trim().length < 2) { setResults([]); return }
    let cancelled = false
    const t = setTimeout(() => {
      api.searchCards(q.trim(), 6).then(res => {
        if (cancelled) return
        const rows = Array.isArray(res) ? res : (res?.results || res?.cards || [])
        setResults(rows.slice(0, 6))
      }).catch(() => { if (!cancelled) setResults([]) })
    }, 200)
    return () => { cancelled = true; clearTimeout(t) }
  }, [q])
  const pick = (name) => {
    onAdd(name)
    setQ('')
    setResults([])
  }
  return (
    <div style={{ position: 'relative', marginBottom: 8 }}>
      <input
        type="text"
        value={q}
        onChange={(e) => setQ(e.target.value)}
        onFocus={() => setFocused(true)}
        onBlur={() => setTimeout(() => setFocused(false), 150)}
        onKeyDown={(e) => {
          if (e.key === 'Enter' && results[0]) {
            e.preventDefault()
            pick(results[0].name || results[0])
          }
        }}
        placeholder="+ ADD CARD — type to search..."
        style={{
          width: '100%', padding: '8px 10px',
          background: 'var(--bg-2, rgba(0,0,0,0.3))',
          border: '1px solid var(--rule-2)',
          color: 'var(--ink)', fontFamily: 'inherit', fontSize: 11,
          letterSpacing: '0.04em',
        }}
        spellCheck={false}
      />
      {focused && results.length > 0 && (
        <div style={{
          position: 'absolute', top: '100%', left: 0, right: 0, zIndex: 10,
          background: 'var(--panel)', border: '1px solid var(--rule-2)',
          borderTop: 'none', maxHeight: 240, overflowY: 'auto',
        }}>
          {results.map((r, i) => {
            const name = r.name || r
            return (
              <div
                key={i}
                onMouseDown={(e) => { e.preventDefault(); pick(name) }}
                style={{
                  padding: '6px 10px', cursor: 'pointer', fontSize: 11,
                  borderBottom: '1px solid var(--rule)',
                }}
                onMouseEnter={(e) => e.currentTarget.style.background = 'var(--bg-2, rgba(255,255,255,0.04))'}
                onMouseLeave={(e) => e.currentTarget.style.background = 'transparent'}
              >
                {name}
                {r.type_line && (
                  <span className="t-xs muted" style={{ marginLeft: 8 }}>— {r.type_line}</span>
                )}
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}

// WorkshopTextarea — the editable deck list with inline card-name
// autocomplete. Wraps the raw <textarea> so we can detect the partial
// name the user is typing on the current line, fire a debounced
// /api/cards/search (the oracle-backed endpoint), and offer pickable
// completions without forcing them to leave the textarea and click the
// + ADD CARD widget above. Arrow keys + Tab/Enter navigate and accept,
// Escape dismisses without applying.
function WorkshopTextarea({ value, onChange, textareaRef: externalRef }) {
  const internalRef = useRef(null)
  const taRef = externalRef || internalRef
  const [caret, setCaret] = useState(0)
  const [suggestions, setSuggestions] = useState([])
  const [highlighted, setHighlighted] = useState(0)
  const [open, setOpen] = useState(false)
  // dismissed pins the suggestion box closed when the user hits Escape
  // — it stays closed until they type another character, so an aborted
  // completion doesn't re-pop on every selection change.
  const [dismissed, setDismissed] = useState(false)

  const fragmentInfo = useMemo(
    () => (dismissed ? null : extractFragment(value, caret)),
    [value, caret, dismissed],
  )
  const fragment = fragmentInfo?.fragment || ''

  useEffect(() => {
    if (!fragmentInfo || fragment.length < MIN_FRAGMENT_CHARS) {
      setSuggestions([])
      setOpen(false)
      return
    }
    let cancelled = false
    const t = setTimeout(() => {
      api.searchCards(fragment, 8).then(res => {
        if (cancelled) return
        const rows = Array.isArray(res) ? res : (res?.results || res?.cards || [])
        setSuggestions(rows.slice(0, 8))
        setHighlighted(0)
        setOpen(rows.length > 0)
      }).catch(() => {
        if (!cancelled) { setSuggestions([]); setOpen(false) }
      })
    }, 180)
    return () => { cancelled = true; clearTimeout(t) }
  }, [fragment, fragmentInfo])

  const accept = (suggestion) => {
    if (!fragmentInfo || !suggestion) return
    const name = suggestion?.name || suggestion
    const { text: nextText, caret: nextCaret } = applySuggestion(value, fragmentInfo, name)
    onChange(nextText)
    setOpen(false)
    setSuggestions([])
    setDismissed(true)
    // Restore caret after React's re-render. Otherwise the textarea
    // value updates but the cursor stays at the prior offset and the
    // next keystroke lands in the middle of the just-completed name.
    requestAnimationFrame(() => {
      const ta = taRef.current
      if (ta) {
        ta.focus()
        ta.setSelectionRange(nextCaret, nextCaret)
        setCaret(nextCaret)
      }
    })
  }

  const onKeyDown = (e) => {
    if (!open || suggestions.length === 0) return
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setHighlighted(h => nextSuggestionIndex(h, 1, suggestions.length))
      return
    }
    if (e.key === 'ArrowUp') {
      e.preventDefault()
      setHighlighted(h => nextSuggestionIndex(h, -1, suggestions.length))
      return
    }
    if (e.key === 'Tab' || (e.key === 'Enter' && !e.shiftKey)) {
      e.preventDefault()
      accept(suggestions[highlighted])
      return
    }
    if (e.key === 'Escape') {
      e.preventDefault()
      setOpen(false)
      setDismissed(true)
    }
  }

  const onSelect = (e) => setCaret(e.target.selectionStart || 0)

  return (
    <div style={{ position: 'relative' }}>
      <textarea
        ref={taRef}
        value={value}
        onChange={e => {
          onChange(e.target.value)
          setCaret(e.target.selectionStart || 0)
          setDismissed(false)
        }}
        onSelect={onSelect}
        onClick={onSelect}
        onKeyUp={onSelect}
        onKeyDown={onKeyDown}
        onBlur={() => setTimeout(() => setOpen(false), 150)}
        style={{
          width: '100%', minHeight: 300, padding: 10,
          background: 'var(--bg-2, rgba(0,0,0,0.3))', border: '1px solid var(--rule-2)',
          color: 'var(--ink)', fontFamily: 'inherit', fontSize: 11,
          letterSpacing: '0.04em', lineHeight: 1.6, resize: 'vertical',
        }}
        spellCheck={false}
        autoComplete="off"
        autoCorrect="off"
        autoCapitalize="off"
        data-testid="workshop-textarea"
      />
      {open && suggestions.length > 0 && (
        <div
          data-testid="workshop-autocomplete"
          style={{
            position: 'absolute',
            // Anchor the popup to the textarea's lower-left corner. The
            // raw textarea API doesn't expose caret coords, so caret-
            // anchored positioning would need a hidden mirror div — too
            // heavy for the value. A persistent dropdown at the bottom
            // matches the WorkshopAddCard pattern users already know.
            top: '100%', left: 0, right: 0,
            zIndex: 10,
            marginTop: -1,
            background: 'var(--panel)',
            border: '1px solid var(--rule-2)',
            borderTop: 'none',
            maxHeight: 240, overflowY: 'auto',
            boxShadow: '0 6px 16px rgba(0,0,0,0.35)',
          }}
        >
          <div className="t-xs muted" style={{ padding: '4px 10px', borderBottom: '1px solid var(--rule)', letterSpacing: '0.08em' }}>
            ORACLE / / {suggestions.length} MATCH{suggestions.length === 1 ? '' : 'ES'} FOR "{fragment.toUpperCase()}"
          </div>
          {suggestions.map((s, i) => {
            const name = s?.name || s
            const active = i === highlighted
            return (
              <div
                key={`${name}-${i}`}
                role="option"
                aria-selected={active}
                onMouseDown={(e) => { e.preventDefault(); accept(s) }}
                onMouseEnter={() => setHighlighted(i)}
                style={{
                  padding: '6px 10px',
                  cursor: 'pointer',
                  fontSize: 11,
                  borderBottom: '1px solid var(--rule)',
                  background: active ? 'var(--bg-2, rgba(255,255,255,0.06))' : 'transparent',
                  color: active ? 'var(--ink)' : 'var(--ink-2)',
                }}
              >
                {name}
                {s.type_line && (
                  <span className="t-xs muted" style={{ marginLeft: 8 }}>— {s.type_line}</span>
                )}
              </div>
            )
          })}
          <div className="t-xs muted" style={{ padding: '4px 10px', letterSpacing: '0.08em', borderTop: '1px solid var(--rule)' }}>
            ↑↓ NAV · TAB/↵ ACCEPT · ESC DISMISS
          </div>
        </div>
      )}
    </div>
  )
}

// DeckBudgetPanel — USD price rollup pulled from the Scryfall-backed
// oracle cache. Lazy-fetched: only hits the backend when the panel
// first mounts (it's invisible on other tabs, so the cost is paid
// once-per-deck-view). Cards Scryfall has no $ for show as "—" in
// the per-card list and surface in a "N CARDS UNPRICED" subtitle so
// the user understands why the headline number might be low.
function DeckBudgetPanel({ deckId }) {
  const [payload, setPayload] = useState(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(null)

  useEffect(() => {
    if (!deckId) return
    let cancelled = false
    setLoading(true)
    setError(null)
    api.getDeckBudget(deckId)
      .then(res => {
        if (cancelled) return
        setPayload(res)
        setLoading(false)
      })
      .catch(err => {
        if (cancelled) return
        setError(err?.message || 'BUDGET FETCH FAILED')
        setLoading(false)
      })
    return () => { cancelled = true }
  }, [deckId])

  if (loading && !payload) {
    return (
      <Panel code="04.$" title="DECK BUDGET">
        <div className="t-xs muted" style={{ padding: '14px 0', textAlign: 'center' }}>
          &gt; PRICING DECK<span className="blink">_</span>
        </div>
      </Panel>
    )
  }
  if (error) {
    return (
      <Panel code="04.$" title="DECK BUDGET">
        <div className="t-xs" style={{ padding: '12px 0', color: 'var(--danger)' }}>
          ✗ {error}
        </div>
      </Panel>
    )
  }
  if (!payload) return null

  const summary = summarizeBudget(payload)
  const coveragePct = Math.round(summary.coverage * 100)

  return (
    <Panel
      code="04.$"
      title="DECK BUDGET"
      right={<Tag solid>{summary.tier}</Tag>}
    >
      <div data-testid="deck-budget-summary" style={{ display: 'flex', alignItems: 'baseline', justifyContent: 'space-between', gap: 12, flexWrap: 'wrap' }}>
        <div>
          <div className="t-xs muted" style={{ letterSpacing: '0.08em' }}>YOUR DECK COSTS</div>
          <div style={{ fontSize: 28, fontWeight: 700, letterSpacing: '0.02em' }} data-testid="deck-budget-total">
            {formatUSD(summary.total)}
          </div>
        </div>
        <div style={{ display: 'flex', gap: 18, flexWrap: 'wrap' }}>
          <BudgetMetric label="AVG / CARD" value={formatUSD(summary.avgPerCard)} />
          <BudgetMetric label="CARDS" value={`${summary.cardCount}`} />
          <BudgetMetric
            label="PRICED"
            value={`${summary.pricedCount} / ${summary.uniqueCount}`}
            sub={summary.uniqueCount > 0 ? `${coveragePct}% COVERAGE` : null}
          />
          {summary.missingCount > 0 && (
            <BudgetMetric
              label="UNPRICED"
              value={`${summary.missingCount}`}
              sub="SCRYFALL HAS NO $"
              warn
            />
          )}
        </div>
      </div>

      {summary.topExpensive.length > 0 && (
        <>
          <div className="hr" style={{ margin: '10px 0' }} />
          <div className="t-xs muted" style={{ marginBottom: 6, letterSpacing: '0.08em' }}>
            TOP CONTRIBUTORS
          </div>
          <div data-testid="deck-budget-top">
            {summary.topExpensive.map((c, i) => (
              <div
                key={`${c.name}-${i}`}
                style={{
                  display: 'grid',
                  gridTemplateColumns: '20px 1fr 80px 80px',
                  gap: 6,
                  alignItems: 'baseline',
                  padding: '3px 0',
                  borderBottom: i < summary.topExpensive.length - 1 ? '1px dotted var(--rule)' : 'none',
                  fontSize: 11,
                }}
              >
                <span className="t-xs muted" style={{ fontVariantNumeric: 'tabular-nums' }}>{i + 1}.</span>
                <CardLink name={c.name} className="t-xs" style={{ borderBottom: 'none', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {c.name}
                </CardLink>
                <span className="t-xs muted" style={{ textAlign: 'right', fontVariantNumeric: 'tabular-nums' }}>
                  {(c.qty || 1) > 1 ? `${formatUSD(c.unit)} × ${c.qty}` : formatUSD(c.unit)}
                </span>
                <span style={{ textAlign: 'right', fontWeight: 700, fontVariantNumeric: 'tabular-nums' }}>
                  {formatUSD(c.line)}
                </span>
              </div>
            ))}
          </div>
        </>
      )}
      <div className="t-xs muted" style={{ marginTop: 8, letterSpacing: '0.04em' }}>
        Prices from Scryfall · {summary.currency} · basics not counted
      </div>
    </Panel>
  )
}

function BudgetMetric({ label, value, sub, warn }) {
  return (
    <div>
      <div className="t-xs muted" style={{ letterSpacing: '0.08em' }}>{label}</div>
      <div style={{
        fontSize: 15, fontWeight: 700, fontVariantNumeric: 'tabular-nums',
        color: warn ? 'var(--warn, #c0a060)' : 'var(--ink)',
      }}>
        {value}
      </div>
      {sub && <div className="t-xs muted" style={{ fontSize: 9, letterSpacing: '0.06em' }}>{sub}</div>}
    </div>
  )
}

// DeckHistoryPanel — full deck-history view. Lists every archived
// version of the deck (plus a synthetic CURRENT entry for the live
// file), and on row expand fetches the version body + the immediately
// previous version's body and renders the per-card diff (added /
// removed / qty-changed). Lazy fetch: only the rows the user actually
// opens hit the network, so a 50-version deck doesn't burst-fetch on
// mount.
function DeckHistoryPanel({ versions, deckId, currentDeckText, commanderName }) {
  // Synthetic CURRENT row. Its diff is computed against the latest
  // archived version using already-known data (currentDeckText +
  // a lazy fetch of versions[0]'s body).
  const rows = useMemo(() => {
    // versions is returned newest-first by /api/decks/{id}/versions.
    const v = Array.isArray(versions) ? [...versions] : []
    const out = []
    if (currentDeckText) {
      out.push({
        kind: 'current',
        version: 'current',
        label: 'CURRENT',
        saved_at: null,
        prevVersion: v[0]?.version ?? null,
      })
    }
    for (let i = 0; i < v.length; i++) {
      out.push({
        kind: 'archived',
        version: v[i].version,
        label: `V${v[i].version}`,
        saved_at: v[i].saved_at || null,
        prevVersion: v[i + 1]?.version ?? null,
      })
    }
    return out
  }, [versions, currentDeckText])

  const [open, setOpen] = useState({})
  const [bodies, setBodies] = useState({})    // version# → deck text
  const [errors, setErrors] = useState({})    // version# → message
  const [loadingKey, setLoadingKey] = useState(null)

  const fetchBody = async (version) => {
    if (version == null) return null
    if (bodies[version] != null) return bodies[version]
    setLoadingKey(version)
    try {
      const v = await api.getDeckVersion(deckId, version)
      const text = v?.deck_list ?? ''
      setBodies(prev => ({ ...prev, [version]: text }))
      return text
    } catch (err) {
      setErrors(prev => ({ ...prev, [version]: err?.message || 'FETCH FAILED' }))
      return null
    } finally {
      setLoadingKey(null)
    }
  }

  const toggle = async (row) => {
    const key = row.version
    if (open[key]) {
      setOpen(prev => ({ ...prev, [key]: false }))
      return
    }
    setOpen(prev => ({ ...prev, [key]: true }))
    // Lazy load both sides of the diff.
    if (row.kind === 'archived') await fetchBody(row.version)
    if (row.prevVersion != null) await fetchBody(row.prevVersion)
  }

  if (rows.length === 0) {
    return (
      <Panel code="04.H" title="DECK HISTORY">
        <div className="t-xs muted" style={{ padding: '12px 0' }}>
          &gt; NO HISTORY YET — VERSIONS ARE ARCHIVED ON EVERY SAVE UPDATE
        </div>
      </Panel>
    )
  }

  return (
    <Panel code="04.H" title="DECK HISTORY" right={<Tag>{rows.length} ENTRIES</Tag>}>
      <ContextBox id="deck.history" compact style={{ marginBottom: 8 }}>
        Every <strong>SAVE UPDATE</strong> archives the prior deck as a new version.
        Click a row to see what changed since the previous version — added cards, cut cards, and quantity tweaks.
      </ContextBox>
      <div data-testid="deck-history-list">
        {rows.map(row => {
          const isOpen = !!open[row.version]
          const baselineText = row.prevVersion != null ? bodies[row.prevVersion] : ''
          const currentText = row.kind === 'current' ? (currentDeckText || '') : (bodies[row.version] || '')
          const ready = row.kind === 'current'
            ? (row.prevVersion == null || baselineText != null)
            : (bodies[row.version] != null && (row.prevVersion == null || baselineText != null))
          const diff = ready ? diffDeckText(baselineText, currentText) : null
          const isLoading = loadingKey === row.version || (row.prevVersion != null && loadingKey === row.prevVersion)
          const err = errors[row.version] || (row.prevVersion != null ? errors[row.prevVersion] : null)
          return (
            <div
              key={row.version}
              data-testid={`deck-history-row-${row.version}`}
              style={{ borderBottom: '1px dotted var(--rule)' }}
            >
              <button
                type="button"
                onClick={() => toggle(row)}
                aria-expanded={isOpen}
                style={{
                  width: '100%',
                  display: 'grid',
                  gridTemplateColumns: '24px 90px 1fr auto',
                  alignItems: 'center',
                  gap: 8,
                  padding: '6px 0',
                  background: 'transparent',
                  border: 0,
                  color: 'inherit',
                  font: 'inherit',
                  cursor: 'pointer',
                  textAlign: 'left',
                }}
              >
                <span style={{
                  fontSize: 10,
                  color: 'var(--ink-2)',
                  transition: 'transform 120ms ease',
                  transform: isOpen ? 'rotate(90deg)' : 'rotate(0deg)',
                  display: 'inline-block',
                }}>▶</span>
                <span style={{ fontSize: 11, fontWeight: 700, letterSpacing: '0.06em' }}>
                  {row.label}{row.kind === 'current' && <span className="t-xs muted" style={{ marginLeft: 6 }}>(LIVE)</span>}
                </span>
                <span className="t-xs muted">
                  {row.saved_at ? new Date(row.saved_at).toLocaleString() : (row.kind === 'current' ? '— UNSAVED IF EDITED' : '')}
                </span>
                <span className="t-xs" style={{ letterSpacing: '0.06em', color: diff && !diff.isClean ? 'var(--ink)' : 'var(--ink-3)' }}>
                  {isOpen && isLoading ? 'LOADING…' : (diff ? diffSummary(diff) : (row.prevVersion == null ? 'INITIAL IMPORT' : ''))}
                </span>
              </button>
              {isOpen && (
                <div style={{ padding: '4px 0 10px 32px' }}>
                  {err && (
                    <div className="t-xs" style={{ color: 'var(--danger)' }}>✗ {err}</div>
                  )}
                  {!ready && !err && (
                    <div className="t-xs muted">&gt; FETCHING VERSION{row.prevVersion != null ? ' + BASELINE' : ''}…</div>
                  )}
                  {ready && row.prevVersion == null && (
                    <div className="t-xs muted">&gt; INITIAL IMPORT — NO PRIOR VERSION TO DIFF AGAINST</div>
                  )}
                  {ready && diff && row.prevVersion != null && (
                    <DeckHistoryDiffBlock diff={diff} baselineLabel={`V${row.prevVersion}`} currentLabel={row.label} />
                  )}
                </div>
              )}
            </div>
          )
        })}
      </div>
      {commanderName && (
        <div className="t-xs muted" style={{ marginTop: 6, letterSpacing: '0.06em' }}>
          COMMANDER: {commanderName.toUpperCase()}
        </div>
      )}
    </Panel>
  )
}

function DeckHistoryDiffBlock({ diff, baselineLabel, currentLabel }) {
  if (!diff || diff.isClean) {
    return <div className="t-xs muted">&gt; NO CHANGES BETWEEN {baselineLabel} AND {currentLabel}</div>
  }
  return (
    <div data-testid="deck-history-diff" style={{ fontSize: 11 }}>
      <div className="t-xs muted" style={{ marginBottom: 4, letterSpacing: '0.06em' }}>
        {baselineLabel} → {currentLabel} · +{diff.addedCards} cards / -{diff.removedCards} cards / net {diff.netCards >= 0 ? '+' : ''}{diff.netCards}
      </div>
      {diff.added.length > 0 && (
        <div style={{ marginTop: 4 }}>
          <div style={{ fontSize: 9, color: 'var(--ok)', letterSpacing: '0.08em', fontWeight: 700 }}>ADDED</div>
          {diff.added.map((a, i) => (
            <div key={`a-${i}`} style={{ fontSize: 11 }}>
              <span style={{ color: 'var(--ok)' }}>+{a.delta}</span> {a.name}
            </div>
          ))}
        </div>
      )}
      {diff.removed.length > 0 && (
        <div style={{ marginTop: 4 }}>
          <div style={{ fontSize: 9, color: 'var(--danger)', letterSpacing: '0.08em', fontWeight: 700 }}>REMOVED</div>
          {diff.removed.map((r, i) => (
            <div key={`r-${i}`} style={{ fontSize: 11 }}>
              <span style={{ color: 'var(--danger)' }}>-{r.delta}</span> {r.name}
            </div>
          ))}
        </div>
      )}
      {diff.changed.length > 0 && (
        <div style={{ marginTop: 4 }}>
          <div style={{ fontSize: 9, color: 'var(--ink-2)', letterSpacing: '0.08em', fontWeight: 700 }}>QTY CHANGED</div>
          {diff.changed.map((c, i) => (
            <div key={`c-${i}`} style={{ fontSize: 11 }}>
              <span style={{ color: c.delta > 0 ? 'var(--ok)' : 'var(--danger)' }}>
                {c.delta > 0 ? '+' : ''}{c.delta}
              </span>{' '}
              {c.name}{' '}
              <span className="t-xs muted">({c.from} → {c.to})</span>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

// WorkshopDiff — shows what would happen if the user clicks SAVE UPDATE.
// Parses both the baseline (workshop-open snapshot) and the current
// editText into card-count maps and renders a +N / -M summary plus a
// collapsible per-card list. Lets the user verify their changes before
// committing a new deck version.
function WorkshopDiff({ baseline, current }) {
  const [open, setOpen] = useState(false)
  const diff = useMemo(() => computeDeckDiff(baseline, current), [baseline, current])
  if (diff.added.length === 0 && diff.removed.length === 0) {
    return (
      <div className="t-xs muted" style={{ margin: '6px 0', opacity: 0.6 }}>
        NO CHANGES YET
      </div>
    )
  }
  return (
    <div style={{ margin: '6px 0', fontSize: 11 }}>
      <button
        type="button"
        onClick={() => setOpen(o => !o)}
        style={{
          background: 'transparent', border: '1px solid var(--rule-2)',
          padding: '4px 10px', color: 'inherit', font: 'inherit', cursor: 'pointer',
          letterSpacing: '0.06em',
        }}
      >
        <span style={{ marginRight: 8 }}>{open ? '▼' : '▶'}</span>
        DIFF{' '}
        <span style={{ color: 'var(--ok)', fontWeight: 700 }}>+{diff.added.length}</span>
        {' / '}
        <span style={{ color: 'var(--danger)', fontWeight: 700 }}>-{diff.removed.length}</span>
      </button>
      {open && (
        <div style={{
          marginTop: 6, padding: '8px 10px',
          border: '1px solid var(--rule-2)',
          background: 'var(--bg-2, rgba(0,0,0,0.2))',
          maxHeight: 240, overflowY: 'auto',
        }}>
          {diff.added.length > 0 && (
            <div style={{ marginBottom: 6 }}>
              <div style={{ fontSize: 9, color: 'var(--ok)', letterSpacing: '0.08em', fontWeight: 700 }}>ADDED</div>
              {diff.added.map((a, i) => (
                <div key={`a-${i}`} style={{ fontSize: 11 }}>
                  <span style={{ color: 'var(--ok)' }}>+{a.delta}</span> {a.name}
                </div>
              ))}
            </div>
          )}
          {diff.removed.length > 0 && (
            <div>
              <div style={{ fontSize: 9, color: 'var(--danger)', letterSpacing: '0.08em', fontWeight: 700 }}>REMOVED</div>
              {diff.removed.map((r, i) => (
                <div key={`r-${i}`} style={{ fontSize: 11 }}>
                  <span style={{ color: 'var(--danger)' }}>-{r.delta}</span> {r.name}
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  )
}

// Parse a deck-list textarea into a {name: qty} map. Commander row is
// preserved by its prefix and treated like a regular 1-qty entry for diff
// purposes (so swapping commanders shows up as -OLD / +NEW).
function parseDeckText(text) {
  const counts = new Map()
  for (const raw of (text || '').split('\n')) {
    const line = raw.trim()
    if (!line) continue
    const cmdrMatch = line.match(/^COMMANDER:\s*(.+)$/i)
    if (cmdrMatch) {
      const name = cmdrMatch[1].trim()
      counts.set(name, (counts.get(name) || 0) + 1)
      continue
    }
    const m = line.match(/^(\d+)\s+(.+)$/)
    if (m) {
      const name = m[2].trim()
      counts.set(name, (counts.get(name) || 0) + parseInt(m[1], 10))
    }
  }
  return counts
}

function computeDeckDiff(baseline, current) {
  const a = parseDeckText(baseline)
  const b = parseDeckText(current)
  const names = new Set([...a.keys(), ...b.keys()])
  const added = []
  const removed = []
  for (const name of names) {
    const before = a.get(name) || 0
    const after = b.get(name) || 0
    if (after > before) added.push({ name, delta: after - before })
    else if (before > after) removed.push({ name, delta: before - after })
  }
  added.sort((x, y) => x.name.localeCompare(y.name))
  removed.sort((x, y) => x.name.localeCompare(y.name))
  return { added, removed }
}

// CollapsiblePanel wraps a Panel with a click-to-toggle header. Used to
// hide lower-tier deep analysis sections by default so the top of the
// deck page stays scannable. The expand/collapse caret uses the same
// rotating triangle pattern as RationalePanels.jsx's per-row Caret.
// EloSparkline lives in components/EloSparkline.jsx — imported above.

function CollapsiblePanel({ code, title, right, defaultOpen = false, children }) {
  const [open, setOpen] = useState(defaultOpen)
  return (
    <Panel
      code={code}
      className={open ? '' : 'panel--collapsed'}
      title={(
        <button
          type="button"
          className="collapsible-panel__toggle"
          onClick={() => setOpen(o => !o)}
          aria-expanded={open}
          style={{
            cursor: 'pointer', userSelect: 'none', display: 'inline-flex', alignItems: 'center', gap: 6,
            background: 'none', border: 0, padding: 0, font: 'inherit', color: 'inherit',
          }}
        >
          <span style={{
            fontSize: 10, color: 'var(--ink-2)', width: 12, textAlign: 'center',
            transition: 'transform 0.15s', transform: open ? 'rotate(90deg)' : 'rotate(0deg)', display: 'inline-block',
          }}>▶</span>
          <span>{title}</span>
        </button>
      )}
      right={right}
    >
      {open && children}
    </Panel>
  )
}

export default function DeckArchive() {
  const { owner, id } = useParams()
  const navigate = useNavigate()
  const [deck, setDeck] = useState(null)
  const [analysis, setAnalysis] = useState(null)
  const [loading, setLoading] = useState(true)
  const [analyzing, setAnalyzing] = useState(false)
  const [editing, setEditing] = useState(false)
  const [editText, setEditText] = useState('')
  // Snapshot of editText taken when the workshop was opened, used as
  // the baseline for the "SAVE UPDATE (+3 / -1)" diff readout. Stays
  // stable while the user types — only refreshes on the next workshop-open.
  const [originalEditText, setOriginalEditText] = useState('')
  const [saving, setSaving] = useState(false)
  // Ref for the Workshop edit panel so we can scroll it into view when it
  // opens. On mobile the sidebar (which holds the WORKSHOP button) renders
  // ABOVE the main content column, so the panel materializes far below the
  // viewport and the action looks like a no-op without an auto-scroll.
  const editPanelRef = useRef(null)
  useEffect(() => {
    if (editing && editPanelRef.current) {
      editPanelRef.current.scrollIntoView({ behavior: 'smooth', block: 'start' })
    }
  }, [editing])

  // Cmd/Ctrl+K or bare K opens the in-deck card search modal. Bare K is
  // suppressed when the user is typing in an editable field so the
  // letter still types normally there.
  useEffect(() => {
    const isEditable = (el) => {
      if (!el) return false
      const tag = el.tagName
      return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || el.isContentEditable
    }
    const onKey = (e) => {
      // Escape handled inside the modal via useModalKeyboard.
      const isK = e.key === 'k' || e.key === 'K'
      if (!isK) return
      if (e.metaKey || e.ctrlKey) {
        // Cmd+K / Ctrl+K — always open, override browser default
        e.preventDefault()
        setActiveTab('decklist')
        setCardSearchOpen(true)
        return
      }
      if (e.altKey || e.shiftKey) return
      if (isEditable(e.target)) return
      e.preventDefault()
      setActiveTab('decklist')
      setCardSearchOpen(true)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  useEffect(() => {
    if (cardSearchOpen && cardSearchInputRef.current) {
      cardSearchInputRef.current.focus()
      cardSearchInputRef.current.select()
    }
  }, [cardSearchOpen])
  // Snapshot the editText when the workshop opens, so the diff readout
  // ("SAVE UPDATE (+3 / -1)") has a stable baseline to compare against
  // even after the user starts typing.
  useEffect(() => {
    if (editing) setOriginalEditText(editText)
    // We intentionally only react to `editing`, not editText, so the
    // baseline captures the value at OPEN time and stays put while the
    // user makes edits. ESLint would want editText in the deps but that
    // would invalidate the baseline every keystroke.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [editing])
  // Lazy-load versions when the HISTORY tab opens. Other entry points
  // (workshop save, clone, etc.) refresh `versions` already; this
  // covers the read-only viewer who clicks HISTORY without opening
  // the workshop first.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => {
    if (activeTab === 'history' && owner && id) {
      api.getDeckVersions(`${owner}/${id}`).then(setVersions).catch(() => {})
    }
  }, [activeTab, owner, id])

  const [confirmDelete, setConfirmDelete] = useState(false)
  const [comparePickerOpen, setComparePickerOpen] = useState(false)
  const [exportOpen, setExportOpen] = useState(false)
  const [versions, setVersions] = useState([])
  const [gauntlet, setGauntlet] = useState(null)
  const [curse, setCurse] = useState(null)
  const [achievements, setAchievements] = useState(null)
  const [editingName, setEditingName] = useState(false)
  const [nameDraft, setNameDraft] = useState('')
  const [savingName, setSavingName] = useState(false)
  const [winLinesExpanded, setWinLinesExpanded] = useState(false)
  // PR #78 — extended deck data panels.
  // matchupMatrix: rows from /api/decks/{id}/matchups (head-to-head detail).
  // commanderCardStats: aggregate per-card win rates across all decks of
  // this commander — surfaces "is this card pulling weight" signal.
  const [matchupMatrix, setMatchupMatrix] = useState(null)
  const [commanderCardStats, setCommanderCardStats] = useState(null)
  // deckCardStats: per-deck card win rates from /api/deck-card-stats/{owner}/{id}.
  // Preferred source for the HOT CARDS widget — richer than the commander
  // aggregate because the server intersects the card_stats pool with this
  // deck's actual list and ranks by win-rate-above-baseline. null = not yet
  // fetched; { cards: [...] } populated on success or after a 404 fallback.
  const [deckCardStats, setDeckCardStats] = useState(null)
  // PR #79 — ELO history runs (oldest-first). Each entry is one completed
  // gauntlet, captures elo_start / elo_end / win_rate / placements. Drives
  // the rating-over-time chart on the deck page.
  const [eloHistory, setEloHistory] = useState(null)
  // Recent games — last N persisted GameSummaries this deck appeared in.
  // null while pending; [] when the deck has no archived games.
  const [recentGames, setRecentGames] = useState(null)
  const [cloning, setCloning] = useState(false)
  const [creditsRefreshKey, setCreditsRefreshKey] = useState(0)
  const [confirmClone, setConfirmClone] = useState(false)
  const [confirmFork, setConfirmFork] = useState(false)
  const [forking, setForking] = useState(false)
  const [spawningRoom, setSpawningRoom] = useState(false)
  const [isFriend, setIsFriend] = useState(false)
  const [friendBusy, setFriendBusy] = useState(false)
  const [ownerFriendCount, setOwnerFriendCount] = useState(null)
  const [similarDecks, setSimilarDecks] = useState(null) // null=loading, []=resolved
  const [activeTab, setActiveTab] = useState('analysis')
  // DECK LIST view mode — persists across tab switches and across
  // page loads (localStorage). 'tiles' = role-grouped image grid
  // (existing CardRolesGrid + per-list panel); 'dense' = sortable
  // spreadsheet (CardListDense) for "what's my 2-CMC removal count"
  // style scanning.
  const [decklistView, setDecklistView] = useState(() => {
    if (typeof localStorage === 'undefined') return 'tiles'
    return localStorage.getItem('hexdek_decklist_view') === 'dense' ? 'dense' : 'tiles'
  })
  useEffect(() => {
    if (typeof localStorage !== 'undefined') localStorage.setItem('hexdek_decklist_view', decklistView)
  }, [decklistView])
  const [denseSort, setDenseSort] = useState({ key: 'type', dir: 'asc' })
  const [cardSearch, setCardSearch] = useState('')
  const [cardSearchOpen, setCardSearchOpen] = useState(false)
  const cardSearchInputRef = useRef(null)
  const { elo } = useLiveSocket()
  const { user } = useAuth()

  const startNameEdit = () => {
    setNameDraft(deck?.custom_name || deck?.commander || '')
    setEditingName(true)
  }

  const cancelNameEdit = () => {
    setEditingName(false)
    setNameDraft('')
  }

  // Tags are persisted via deck_meta; the UI debounces autosave so the
  // user can pile chips on without an explicit SAVE click. 600ms after
  // the last edit we PATCH the new array. Optimistic state update keeps
  // the chip render instant even when the network is slow.
  const [savingTags, setSavingTags] = useState(false)
  const tagsSaveTimer = useRef(null)
  const handleTagsChange = (next) => {
    setDeck(d => ({ ...(d || {}), tags: next }))
    if (!owner || !id) return
    if (tagsSaveTimer.current) clearTimeout(tagsSaveTimer.current)
    tagsSaveTimer.current = setTimeout(async () => {
      setSavingTags(true)
      try {
        const updated = await api.patchDeck(`${owner}/${id}`, { tags: next })
        setDeck(d => ({ ...(d || {}), tags: updated.tags || [] }))
        trackEvent('tag_deck', { deck: `${owner}/${id}`, count: next.length })
      } catch {
        toast.error('TAG SAVE FAILED')
      } finally {
        setSavingTags(false)
      }
    }, 600)
  }

  const commitNameEdit = async () => {
    if (!owner || !id || savingName) return
    const trimmed = nameDraft.trim()
    const current = deck?.custom_name || ''
    if (trimmed === current) {
      cancelNameEdit()
      return
    }
    setSavingName(true)
    try {
      const updated = await api.patchDeck(`${owner}/${id}`, { name: trimmed })
      setDeck(d => ({ ...(d || {}), custom_name: updated.custom_name || '' }))
      trackEvent('rename_deck', { deck: `${owner}/${id}`, len: trimmed.length })
      setEditingName(false)
      toast.success('DECK RENAMED')
    } catch (err) {
      toast.error('RENAME FAILED')
    } finally {
      setSavingName(false)
    }
  }

  // localStorage access during render is unsafe on iOS Safari private
  // browsing (and inside some embedded WebViews) — getItem can throw
  // SecurityError, which would unmount the entire route to a black
  // screen if uncaught. ErrorBoundary covers us now, but guard at the
  // call site too so the route stays interactive in private mode.
  let storedOwnerSlug = ''
  try { storedOwnerSlug = localStorage.getItem('hexdek_owner') || '' } catch {}
  const userOwnerSlug = user
    ? (storedOwnerSlug || user.displayName?.toLowerCase() || user.email?.split('@')[0]?.split('.')[0] || '')
    : ''
  const isOwner = !!owner && !!userOwnerSlug && userOwnerSlug === owner.toLowerCase()
  const canFriend = !!user && !!userOwnerSlug && !!owner && !isOwner

  useEffect(() => {
    if (!canFriend) { setIsFriend(false); return }
    let cancelled = false
    api.listFriends(userOwnerSlug)
      .then(r => { if (!cancelled) setIsFriend((r.friends || []).includes(owner.toLowerCase())) })
      .catch(() => {})
    return () => { cancelled = true }
  }, [canFriend, owner, userOwnerSlug])

  // Pull the deck owner's friend count for the DECK SPECS panel. Refetches
  // when the owner changes or when this visitor's add/remove fires the
  // 'hexdek-friends-changed' event (mutual-add updates the owner's count too).
  useEffect(() => {
    if (!owner) { setOwnerFriendCount(null); return }
    let cancelled = false
    const load = () => {
      api.listFriends(owner)
        .then(r => { if (!cancelled) setOwnerFriendCount((r.friends || []).length) })
        .catch(() => { if (!cancelled) setOwnerFriendCount(null) })
    }
    load()
    const onChanged = () => load()
    window.addEventListener('hexdek-friends-changed', onChanged)
    return () => {
      cancelled = true
      window.removeEventListener('hexdek-friends-changed', onChanged)
    }
  }, [owner])

  const toggleFriend = async () => {
    if (!canFriend || friendBusy) return
    setFriendBusy(true)
    const target = owner.toLowerCase()
    const wasFriend = isFriend
    setIsFriend(!wasFriend) // optimistic
    try {
      if (wasFriend) await api.removeFriend(target, userOwnerSlug)
      else           await api.addFriend(target, userOwnerSlug)
      trackEvent(wasFriend ? 'remove_friend' : 'add_friend', { target })
      window.dispatchEvent(new CustomEvent('hexdek-friends-changed'))
      toast.success(wasFriend ? `UNFRIENDED ${target.toUpperCase()}` : `FRIEND ADDED · ${target.toUpperCase()}`)
    } catch {
      setIsFriend(wasFriend) // rollback
      toast.error(wasFriend ? 'UNFRIEND FAILED' : 'ADD FRIEND FAILED')
    } finally {
      setFriendBusy(false)
    }
  }

  const handleShare = async () => {
    if (!owner || !id) return
    const url = `${window.location.origin}/decks/${owner}/${id}`
    let copied = false
    try {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(url)
        copied = true
      } else {
        const ta = document.createElement('textarea')
        ta.value = url
        ta.style.position = 'fixed'
        ta.style.opacity = '0'
        document.body.appendChild(ta)
        ta.select()
        copied = document.execCommand('copy')
        document.body.removeChild(ta)
      }
    } catch {}
    trackEvent('share_deck', { deck: `${owner}/${id}`, copied })
    if (copied) toast.success('SHARE LINK COPIED')
    else toast.error('COPY FAILED — ' + url, 5000)
  }

  const eloByDeckId = {}
  for (const e of elo) {
    if (e.deck_id) eloByDeckId[e.deck_id] = e
  }
  const deckKey = owner && id ? `${owner}/${id}` : null
  const deckElo = eloByDeckId[deckKey] || eloByDeckId[id] || null

  const fetchAnalysis = (ownerId, deckId) => {
    api.getDeckAnalysis(`${ownerId}/${deckId}`).then(data => {
      if (data.status === 'analyzing') {
        setAnalyzing(true)
      } else {
        setAnalysis(data)
        setAnalyzing(false)
      }
    }).catch(() => setAnalyzing(false))
  }

  useEffect(() => {
    if (!owner || !id) {
      setAnalysis(null)
      setLoading(false)
      return
    }
    Promise.allSettled([
      api.getDeck(`${owner}/${id}`),
      api.getDeckAnalysis(`${owner}/${id}`),
      api.getGauntlet(`${owner}/${id}`),
      api.getDeckCurse(`${owner}/${id}`),
      api.getAchievements(owner),
    ]).then(([deckRes, analysisRes, gauntletRes, curseRes, achievementsRes]) => {
      if (deckRes.status === 'fulfilled') setDeck(deckRes.value)
      if (analysisRes.status === 'fulfilled') {
        const data = analysisRes.value
        if (data.status === 'analyzing') {
          setAnalyzing(true)
        } else {
          setAnalysis(data)
        }
      }
      if (curseRes.status === 'fulfilled' && curseRes.value && curseRes.value.population) {
        setCurse(curseRes.value)
      }
      if (achievementsRes.status === 'fulfilled' && achievementsRes.value) {
        setAchievements(achievementsRes.value)
      }
      if (gauntletRes.status === 'fulfilled' && gauntletRes.value.status !== 'none') {
        setGauntlet(gauntletRes.value)
        if (gauntletRes.value.status === 'running') {
          const poll = () => {
            api.getGauntlet(`${owner}/${id}`).then(r => {
              setGauntlet(r)
              if (r.status === 'running') setTimeout(poll, 3000)
            })
          }
          setTimeout(poll, 3000)
        }
      }
      setLoading(false)
    })
  }, [owner, id])

  // PR #78 — matchup matrix fetch. Independent of the main page load so
  // the deck page renders immediately even if matchups are slow.
  useEffect(() => {
    if (!owner || !id) { setMatchupMatrix(null); return }
    let cancelled = false
    api.getDeckMatchups(`${owner}/${id}`)
      .then(res => {
        if (cancelled) return
        const rows = Array.isArray(res?.matchups) ? res.matchups : []
        setMatchupMatrix(rows)
      })
      .catch(() => { if (!cancelled) setMatchupMatrix([]) })
    return () => { cancelled = true }
  }, [owner, id])

  // PR #79 — ELO history fetch. Pulls the last 20 gauntlet runs from
  // /api/decks/{id}/elo-history (oldest-first). Empty array if the deck
  // has no gauntlet runs yet — chart panel hides on empty.
  useEffect(() => {
    if (!owner || !id) { setEloHistory(null); return }
    let cancelled = false
    api.getDeckEloHistory(`${owner}/${id}`, 20)
      .then(res => {
        if (cancelled) return
        const rows = Array.isArray(res?.runs) ? res.runs : []
        setEloHistory(rows)
      })
      .catch(() => { if (!cancelled) setEloHistory([]) })
    return () => { cancelled = true }
  }, [owner, id])

  // Recent games — last 10 persisted GameSummary rows this deck appeared in.
  // Backed by /api/games/summaries?deck=owner/id&limit=10 (substring match on
  // showmatch_game_seat.deck_key). Empty array means "no archive entries yet"
  // (no observation persisted), which is distinct from "deck never played" —
  // the panel just stays hidden in either case.
  useEffect(() => {
    // Initial state is null — no need to reset on the missing-route case,
    // and avoiding setState in this branch keeps the effect compiler-clean.
    if (!owner || !id) return
    let cancelled = false
    api.searchGameSummaries({ deck: `${owner}/${id}`, limit: 10 })
      .then(res => {
        if (cancelled) return
        setRecentGames(Array.isArray(res?.rows) ? res.rows : [])
      })
      .catch(() => { if (!cancelled) setRecentGames([]) })
    return () => { cancelled = true }
  }, [owner, id])

  // PR #78 — commander-aggregate card stats fetch. Surfaces "is this card
  // pulling weight" across all decks of the same commander. True per-deck
  // card performance is a future enhancement; the commander aggregate is
  // a useful proxy in the meantime.
  useEffect(() => {
    const cmdr = deck?.commander_card
    if (!cmdr) { setCommanderCardStats(null); return }
    let cancelled = false
    api.getCardStatsByCommander(cmdr)
      .then(res => {
        if (cancelled) return
        // Endpoint may return either an array or {cards: [...]}; normalize.
        const rows = Array.isArray(res) ? res : (res?.cards || [])
        setCommanderCardStats(rows)
      })
      .catch(() => { if (!cancelled) setCommanderCardStats([]) })
    return () => { cancelled = true }
  }, [deck?.commander_card])

  // Per-deck card stats — primary source for the HOT CARDS widget.
  // Server returns the cards pre-intersected with this deck's list and
  // pre-sorted by win-rate-above-baseline, so the widget renders directly
  // off `cards` with no client-side ranking. A 404 (older server without
  // the endpoint deployed) silently degrades to an empty result so the
  // widget falls back to the commander aggregate already fetched above.
  useEffect(() => {
    if (!owner || !id) { setDeckCardStats(null); return }
    let cancelled = false
    api.getDeckCardStats(`${owner}/${id}`)
      .then(res => {
        if (cancelled) return
        setDeckCardStats(res && Array.isArray(res.cards) ? res : { cards: [] })
      })
      .catch(() => { if (!cancelled) setDeckCardStats({ cards: [] }) })
    return () => { cancelled = true }
  }, [owner, id])

  // Similar decks — independent fetch so the rest of the page renders
  // immediately. Server scans DecksDir and returns a ranked top-5.
  useEffect(() => {
    if (!owner || !id) { setSimilarDecks([]); return }
    let cancelled = false
    api.getSimilarDecks(`${owner}/${id}`, 5)
      .then(rows => { if (!cancelled) setSimilarDecks(Array.isArray(rows) ? rows : []) })
      .catch(() => { if (!cancelled) setSimilarDecks([]) })
    return () => { cancelled = true }
  }, [owner, id])

  // SSE listener — auto-refresh analysis when Freya completes.
  useEffect(() => {
    if (!owner || !id) return
    const es = new EventSource(`${API_BASE}/api/decks/${owner}/${id}/events`)
    es.addEventListener('freya_started', () => setAnalyzing(true))
    es.addEventListener('freya_complete', () => {
      api.getDeckAnalysis(`${owner}/${id}`).then(data => {
        setAnalysis(data)
        setAnalyzing(false)
      }).catch(() => setAnalyzing(false))
    })
    es.onerror = () => {}
    return () => es.close()
  }, [owner, id])

  // Fallback when commander/custom_name isn't loaded yet — strip
  // bracket marker, owner slug, and trailing moxfield hash so we don't
  // render titles like "HERIGAST ERUPTING NULLKITE B2 LAODI Z8BQG8TF".
  const slugToTitle = (slug, ownerSlug) => {
    if (!slug) return 'DECK'
    let s = String(slug)
    // Moxfield-style trailing 8+char random hash.
    s = s.replace(/_[A-Za-z0-9]{8,}$/, '')
    if (ownerSlug) {
      s = s.replace(new RegExp(`_${ownerSlug.toLowerCase()}$`, 'i'), '')
    }
    // Bracket marker (_b1.._b5).
    s = s.replace(/_b[0-5]$/i, '')
    return s.replace(/_/g, ' ').toUpperCase() || 'DECK'
  }
  const deckName = deck?.custom_name || deck?.commander || slugToTitle(id, owner)

  useEffect(() => {
    if (!deckName) return
    const ownerLabel = owner ? ` · ${owner.toUpperCase()}` : ''
    document.title = `${deckName}${ownerLabel} — HEXDEK`
  }, [deckName, owner])

  // Sum quantities (Plains × 8 counts as 8, not 1); cards?.length would
  // undercount basic-land stacks. Fall back to backend card_count when present.
  const cardCount = deck?.card_count
    || (deck?.cards || []).reduce((n, c) => n + (c.quantity || 1), 0)
    || 99
  // Bracket can be unset on imported decks that haven't been analyzed yet.
  // Keep `wbs` null in that case so callers can pick their own placeholder
  // ("—", "PENDING", hide entirely) instead of every site rendering "B?".
  const userBracket = deck?.bracket || null
  const wbs = analysis?.bracket || userBracket
  const wbsLabel = analysis?.bracket_label || ''
  const pls = analysis?.plays_like || null
  const plsLabel = analysis?.plays_like_label || ''
  const gameChangers = analysis?.game_changer_count ?? null
  const archetype = analysis?.archetype?.toUpperCase() || 'UNKNOWN'
  const summary = analysis?.gameplan_summary || ''
  const winLines = analysis?.win_lines || []
  const valueKeys = analysis?.value_engine_keys || []
  const evalWeights = analysis?.eval_weights || {}
  const cards = deck?.cards || []
  const manaBaseGrade = analysis?.mana_base_grade || null
  const keepableHandPct = analysis?.keepable_hand_pct ?? null
  const powerPercentile = analysis?.power_percentile ?? null
  const commanderSynergy = analysis?.commander_synergy ?? null
  const commanderThemes = analysis?.commander_themes || []
  const starCards = analysis?.star_cards || []
  // Prefer the structured rationale list when Freya has produced it; fall
  // back to the flat name list for older strategy.json files on disk.
  const cuttableCards = analysis?.cuttable_card_rationale || analysis?.cuttable_cards || []
  // Per-card coaching lookup so inline UI (dense list rows, future grid
  // tiles) can show a marker against any flagged card without re-walking
  // the analysis blob. Same source-of-truth as the CONSIDER CUTTING
  // panel — pure helper, see freyaCoaching.js.
  const coachingIndex = buildCoachingIndex(analysis)
  // Shared "open Workshop with this card removed" handler — used by the
  // CONSIDER CUTTING panel's CUT button AND the new inline coaching
  // markers. Owners only; the parent's isOwner gate decides whether
  // either call site even passes this in.
  const handleCutCardFromWorkshop = (cardName) => {
    const lines = cards.map(c => {
      const cmdr = deck?.commander_card
      if (cmdr && c.name === cmdr) return `COMMANDER: ${c.name}`
      return c.quantity > 1 ? `${c.quantity} ${c.name}` : `1 ${c.name}`
    }).filter(line => {
      if (line.startsWith('COMMANDER:')) return true
      const tail = line.replace(/^\d+\s+/, '')
      return tail !== cardName
    })
    setEditText(lines.join('\n'))
    setEditing(true)
    api.getDeckVersions(`${owner}/${id}`).then(setVersions).catch(() => {})
    setTimeout(() => editPanelRef.current?.scrollIntoView({ behavior: 'smooth', block: 'start' }), 100)
  }
  const valueChains = analysis?.value_chains || []
  const vulnerableTo = analysis?.vulnerable_to || []
  const finisherCards = analysis?.finisher_cards || []
  const comboNotes = analysis?.combo_notes || []
  const curveWarnings = analysis?.curve_warnings || []
  const colorMismatch = analysis?.color_mismatch || []
  const legality = analysis?.legality || null
  const gameChangerCards = analysis?.game_changer_cards || []
  const interactionAvgCmc = analysis?.interaction_avg_cmc ?? null
  const cheapInteraction = analysis?.cheap_interaction ?? null
  const emergentSynergies = analysis?.emergent_synergies || []
  const metaMatchups = analysis?.meta_matchups || []
  const cardRoles = analysis?.card_roles || null

  // AT A GLANCE — consolidated Freya stats panel. Pure helper so the shape
  // is unit-tested independently of the page render (see deckStats.test.mjs).
  const glance = deckGlanceStats({ deck, analysis, deckElo, eloHistory })

  // In-deck name search. Applied only to the visible decklist panels
  // (CardRolesGrid + FULL CARD LIST) — stats, curve, and analysis stay
  // computed off the full list.
  const cardSearchQuery = cardSearch.trim().toLowerCase()
  const matchesSearch = (name) => {
    if (!cardSearchQuery) return true
    const n = (name || '').replace(/^COMMANDER:\s*/i, '').toLowerCase()
    return n.includes(cardSearchQuery)
  }
  const filteredCards = cardSearchQuery
    ? (deck?.cards || []).filter(c => matchesSearch(c.name))
    : (deck?.cards || [])
  const filteredCardRoles = cardSearchQuery && cardRoles
    ? Object.fromEntries(Object.entries(cardRoles).filter(([n]) => matchesSearch(n)))
    : cardRoles

  // Derive commander color identity for page theming. Prefer Freya's analysis
  // (authoritative), then commander mana cost, then any pip in the decklist.
  const colorIdentity = (() => {
    if (Array.isArray(analysis?.color_identity) && analysis.color_identity.length) {
      return [...analysis.color_identity].map(c => c.toUpperCase()).filter(c => 'WUBRG'.includes(c))
        .sort((a, b) => 'WUBRG'.indexOf(a) - 'WUBRG'.indexOf(b))
    }
    const ci = new Set()
    const scan = mc => {
      if (!mc) return
      const pips = mc.match(/\{([WUBRG])\}/gi) || []
      for (const p of pips) ci.add(p.replace(/[{}]/g, '').toUpperCase())
    }
    const cmdrName = deck?.commander_card
    if (cmdrName) {
      const cmdr = cards.find(c => c.name === cmdrName)
      if (cmdr) scan(cmdr.mana_cost)
    }
    if (ci.size === 0) for (const c of cards) scan(c.mana_cost)
    return Array.from(ci).sort((a, b) => 'WUBRG'.indexOf(a) - 'WUBRG'.indexOf(b))
  })()

  const pageTheme = (() => {
    // Per-color palette: rgba base for the wash, hex accent for highlights.
    const COLORS = {
      W: { base: '226, 218, 188', accent: '#d8c878' },
      U: { base: '34, 70, 110',   accent: '#5a8fbf' },
      B: { base: '36, 26, 42',    accent: '#9c6ab0' },
      R: { base: '78, 28, 22',    accent: '#cc5c4a' },
      G: { base: '36, 70, 36',    accent: '#7ac28a' },
    }
    const ids = colorIdentity.length ? colorIdentity : []
    if (ids.length === 0) {
      return { wash: 'linear-gradient(135deg, rgba(28,29,22,0.9), rgba(20,21,15,0.9))', accent: '#8a9682', label: 'COLORLESS' }
    }
    // Build a 135deg gradient across the colors. Single colors get a soft
    // top-left → bottom-right fade between two intensities of the same hue.
    let stops
    if (ids.length === 1) {
      const c = COLORS[ids[0]]
      stops = `rgba(${c.base}, 0.85) 0%, rgba(${c.base}, 0.35) 100%`
    } else {
      stops = ids.map((c, i) => {
        const pct = (i / (ids.length - 1)) * 100
        return `rgba(${COLORS[c].base}, 0.7) ${pct.toFixed(0)}%`
      }).join(', ')
    }
    // Pick accent by visual distinctiveness priority: R > G > U > B > W.
    const accentPriority = ['R', 'G', 'U', 'B', 'W']
    const accentColor = ids.find(c => accentPriority.includes(c))
      ? COLORS[accentPriority.find(c => ids.includes(c))].accent
      : '#8a9682'
    const COMBO_NAMES = {
      W: 'MONO WHITE', U: 'MONO BLUE', B: 'MONO BLACK', R: 'MONO RED', G: 'MONO GREEN',
      WU: 'AZORIUS', UB: 'DIMIR', BR: 'RAKDOS', RG: 'GRUUL', GW: 'SELESNYA',
      WB: 'ORZHOV', UR: 'IZZET', BG: 'GOLGARI', RW: 'BOROS', UG: 'SIMIC',
      WUB: 'ESPER', UBR: 'GRIXIS', BRG: 'JUND', RGW: 'NAYA', GWU: 'BANT',
      WBG: 'ABZAN', URW: 'JESKAI', BGU: 'SULTAI', RWB: 'MARDU', GUR: 'TEMUR',
      WUBR: 'YORE-TILLER', WUBG: 'WITCH-MAW', WURG: 'INK-TREADER', WBRG: 'DUNE-BROOD', UBRG: 'GLINT-EYE',
      WUBRG: 'FIVE-COLOR',
    }
    const label = COMBO_NAMES[ids.join('')] || ids.join('')
    return { wash: `linear-gradient(135deg, ${stops})`, accent: accentColor, label }
  })()

  const clientCurve = (() => {
    if (!cards.length) return null
    const dist = Array(8).fill(0)
    let totalCmc = 0, nonlandCount = 0, landCount = 0
    const demand = {}
    for (const c of cards) {
      const qty = c.quantity || 1
      const hasType = c.type_line || c.types
      const typeStr = (c.type_line || (c.types && c.types.join(' ')) || '').toLowerCase()
      const isLand = hasType ? /\bland\b/.test(typeStr) : ((c.cmc ?? -1) === 0 && !c.mana_cost)
      if (isLand) { landCount += qty; continue }
      const cmc = Math.min(c.cmc ?? 0, 7)
      dist[cmc] += qty
      totalCmc += (c.cmc ?? 0) * qty
      nonlandCount += qty
      if (c.mana_cost) {
        const pips = c.mana_cost.match(/\{([WUBRG])}/gi) || []
        for (const p of pips) {
          const color = p.replace(/[{}]/g, '')
          demand[color] = (demand[color] || 0) + qty
        }
      }
    }
    const avgCmc = nonlandCount > 0 ? totalCmc / nonlandCount : 0
    const peak = dist.indexOf(Math.max(...dist))
    const shape = peak <= 2 ? 'LOW CURVE' : peak <= 4 ? 'MID CURVE' : 'HIGH CURVE'
    return { distribution: dist, avg_cmc: avgCmc, curve_shape: shape, land_count: landCount, nonland_count: nonlandCount, demand }
  })()

  const curveData = analysis?.mana_curve || clientCurve
  const colorData = analysis?.color_balance?.demand || clientCurve?.demand

  const manaProduction = deck?.mana_production || (() => {
    if (!cards.length) return null
    const production = {}
    const basicMap = { plains: 'W', island: 'U', swamp: 'B', mountain: 'R', forest: 'G' }
    for (const c of cards) {
      const qty = c.quantity || 1
      const typeStr = (c.type_line || '').toLowerCase()
      if (!/\bland\b/.test(typeStr)) continue
      for (const [basic, color] of Object.entries(basicMap)) {
        if (typeStr.includes(basic)) {
          production[color] = (production[color] || 0) + qty
        }
      }
    }
    return production
  })()

  const demandColors = colorData ? Object.keys(colorData).filter(k => colorData[k] > 0) : []
  const isMultiColor = demandColors.length >= 2

  const cmdrCardName = deck?.commander_card || cards.find(c => c.name?.startsWith('COMMANDER:'))?.name?.replace('COMMANDER:', '').trim()
  const cmdrImageUrl = cmdrCardName
    ? cardArtUrl(cmdrCardName)
    : null
  const cmdrFullUrl = cmdrCardName ? cardImageUrl(cmdrCardName) : null
  const cmdrContrast = useArtContrast(cmdrImageUrl)

  if (loading) {
    return (
      <>
        <Tape left="DECK ARCHIVE / / LOADING" mid="" right="" />
        <div style={{ padding: 36, textAlign: 'center' }}>
          <div className="t-md muted">&gt; LOADING DECK DATA<span className="blink">_</span></div>
        </div>
      </>
    )
  }

  return (
    <div
      className="deck-archive-page"
      style={{
        '--page-wash': pageTheme.wash,
        '--accent': pageTheme.accent,
      }}
    >
      {/* Blown-up gaussian-blurred commander art behind everything —
          shared mechanism with CardPage via the .art-ambience class. */}
      {cmdrImageUrl && (
        <img
          className="art-ambience"
          src={cmdrImageUrl}
          alt=""
          aria-hidden="true"
        />
      )}

      <Tape
        left={`DECK ARCHIVE / / ${owner?.toUpperCase()} / / ${deckName}`}
        mid={
          pls && wbs
            ? `Plays Like B${pls} (Bracket B${wbs}) · ${pageTheme.label}`
            : wbs
              ? `Bracket B${wbs} · ${pageTheme.label}`
              : `Bracket pending · ${pageTheme.label}`
        }
        right="EXPORT ↗ ANALYZE ↗"
      />

      <div
        className={`deck-hero ${cmdrImageUrl ? '' : 'hatch'}`}
        data-art-contrast={cmdrContrast || undefined}
        style={cmdrImageUrl
          ? { backgroundImage: `url(${cmdrImageUrl})`, ...(cmdrContrast ? { '--art-contrast': cmdrContrast } : null) }
          : undefined}
      >
        <div className="deck-hero__scrim" />
        <div className="deck-hero__corner deck-hero__corner--tl">04.HERO / / {pageTheme.label}</div>
        <div className="deck-hero__corner deck-hero__corner--tr">{owner?.toUpperCase()} / / {id}</div>
        <div className="deck-hero__actions">
          {canFriend && (
            <button
              type="button"
              className={`deck-hero__friend ${isFriend ? 'is-on' : ''}`}
              onClick={toggleFriend}
              disabled={friendBusy}
              title={isFriend ? `Unfriend ${owner.toUpperCase()}` : `Add ${owner.toUpperCase()} as a friend`}
            >
              <span>{isFriend ? '✓ FRIEND' : '+ ADD FRIEND'}</span>
            </button>
          )}
          {owner && id && (
            <button type="button" className="deck-hero__share" onClick={handleShare} title="Copy shareable link">
              <span>SHARE</span>
              <span className="arr">↗</span>
            </button>
          )}
          {owner && id && (
            <button type="button" className="deck-hero__share" onClick={() => setComparePickerOpen(true)} title="Compare against another deck">
              <span>COMPARE</span>
              <span className="arr">⇄</span>
            </button>
          )}
        </div>
        <div className="deck-hero__body">
          {cmdrFullUrl && (
            <div className="deck-hero__card">
              <img
                src={cmdrFullUrl}
                alt={cmdrCardName}
                className="deck-hero__card-img"
                onError={(e) => { e.target.style.display = 'none' }}
              />
            </div>
          )}
          <div style={{ flex: 1, minWidth: 0 }}>
          <div className="deck-hero__meta">
            <Tag solid>{wbs ? `B${wbs}` : 'BRACKET PENDING'}{wbs && wbsLabel ? ' · ' + wbsLabel : ''}</Tag>
            {pls && pls !== wbs && <Tag solid kind="warn">PLAYS LIKE B{pls}</Tag>}
            <Tag>{archetype}</Tag>
            {colorIdentity.length > 0 && <Tag>{colorIdentity.join('')}</Tag>}
          </div>
          <div className="deck-hero__title-row">
            {editingName ? (
              <input
                autoFocus
                className="deck-hero__title-input"
                value={nameDraft}
                maxLength={120}
                disabled={savingName}
                onChange={e => setNameDraft(e.target.value)}
                onBlur={commitNameEdit}
                onKeyDown={e => {
                  if (e.key === 'Enter') { e.preventDefault(); commitNameEdit() }
                  else if (e.key === 'Escape') { e.preventDefault(); cancelNameEdit() }
                }}
              />
            ) : (
              <>
                <h1 className="deck-hero__title">{deckName}</h1>
                {isOwner && (
                  <button
                    type="button"
                    className="deck-hero__rename"
                    onClick={startNameEdit}
                    title="Rename deck"
                    aria-label="Rename deck"
                  >✎</button>
                )}
              </>
            )}
          </div>
          {cmdrCardName && cmdrCardName.toUpperCase() !== deckName && (
            <div className="deck-hero__sub">{cmdrCardName}</div>
          )}
          {/* Fork attribution — surfaces "Forked from <owner>/<id>" when
              this deck originated as a /fork of someone else's. Clickable
              link lets the viewer jump back to the source. */}
          {deck?.forked_from && (
            <div className="deck-hero__sub" style={{ fontSize: 11, opacity: 0.8, marginTop: 4, letterSpacing: '0.04em' }}>
              FORKED FROM{' '}
              <Link
                to={`/decks/${deck.forked_from}`}
                style={{ color: 'inherit', borderBottom: '1px dotted currentColor', textDecoration: 'none' }}
              >
                {deck.forked_from}
              </Link>
            </div>
          )}
          {/* gameplan_summary hidden — Freya win-line detection needs accuracy pass */}
          {/* System tags (Freya-derived archetype, prefixed "archetype:")
              are rendered as a distinct read-only chip row above the
              user-editable tag area. They're locked because they're
              derived from Freya's analysis — the user "overrides" by
              adding their own tag with whatever text they prefer, not
              by deleting the system tag. */}
          {Array.isArray(deck?.system_tags) && deck.system_tags.length > 0 && (
            <ArchetypeChipRow
              deck={deck}
              isOwner={isOwner}
              onFeedbackChange={setDeck}
            />
          )}
          {/* Tags: owners get an editable autocomplete chip field; visitors
              see a static chip row when there are any. The field is hidden
              from visitors with no tags to avoid an empty box. */}
          {isOwner ? (
            <div style={{ marginTop: 8, maxWidth: 520 }}>
              <TagInput
                value={Array.isArray(deck?.tags) ? deck.tags : []}
                onChange={handleTagsChange}
                owner={owner}
                placeholder={savingTags ? 'SAVING…' : 'ADD TAG — e.g. cedh, budget, brew'}
              />
            </div>
          ) : (Array.isArray(deck?.tags) && deck.tags.length > 0 && (
            <div style={{ marginTop: 8, display: 'flex', flexWrap: 'wrap', gap: 4 }}>
              {deck.tags.map(t => (
                <Tag key={t}>{t.toUpperCase()}</Tag>
              ))}
            </div>
          ))}
          </div>
        </div>
      </div>

      {/* Hero quick-actions context — explains the floating SHARE / COMPARE
          / FRIEND buttons in the hero. Dismissible so it disappears once
          the user has read it. */}
      {owner && id && (
        <div className="deck-hero__actions-context">
          <ContextBox id="deck.hero.actions">
            <strong>SHARE</strong> copies a public link to this deck page to your clipboard.
            {' '}<strong>COMPARE</strong> opens a side-by-side diff with another deck (overlap, color identity, archetype).
            {canFriend && <> <strong>+ ADD FRIEND</strong> follows {owner?.toUpperCase()} so their decks surface in your feed.</>}
          </ContextBox>
        </div>
      )}

      {/* Vital signs strip — the three big numbers that matter at a glance.
          HexELO, Power Level (Bracket), Win Rate. Decks without gauntlet
          data show "NOT YET RANKED" sublabels so the placeholders aren't
          bare em-dashes — gives the user a hint about how to populate. */}
      <div className="deck-vital-signs">
        <div className="deck-vital-signs__cell">
          <div className="deck-vital-signs__num">
            {deckElo?.hex_rating != null ? Math.round(deckElo.hex_rating) : '—'}
          </div>
          <div className="deck-vital-signs__lbl">HexELO</div>
          {deckElo?.games > 0 ? (
            <div className="deck-vital-signs__sub">{deckElo.games.toLocaleString()} GAMES</div>
          ) : (
            <div className="deck-vital-signs__sub" style={{ opacity: 0.55 }}>RUN GAUNTLET</div>
          )}
          {eloHistory && eloHistory.length >= 2 && (
            <EloSparkline runs={eloHistory} />
          )}
        </div>
        <div className="deck-vital-signs__cell">
          <div className="deck-vital-signs__num">
            {/* Backend can return "?" (string) or null for unknown bracket;
                both should render as em-dash, not a literal "B?". */}
            {wbs && wbs !== '?' ? `B${wbs}${pls && pls !== wbs ? ` → B${pls}` : ''}` : '—'}
          </div>
          <div className="deck-vital-signs__lbl">POWER LEVEL</div>
          {wbsLabel ? (
            <div className="deck-vital-signs__sub">{wbsLabel.toUpperCase()}</div>
          ) : (
            <div className="deck-vital-signs__sub" style={{ opacity: 0.55 }}>PENDING ANALYSIS</div>
          )}
          {owner && id && wbs && wbs !== '?' && (
            <div className="deck-vital-signs__sub" style={{ marginTop: 2 }}>
              <BracketChangelog deckKey={`${owner}/${id}`} bracket={wbs} />
            </div>
          )}
        </div>
        <div className="deck-vital-signs__cell">
          <div className="deck-vital-signs__num">
            {deckElo?.win_rate != null ? `${deckElo.win_rate}%` : '—'}
          </div>
          <div className="deck-vital-signs__lbl">WIN RATE</div>
          {deckElo?.wins != null && deckElo?.losses != null ? (
            <div className="deck-vital-signs__sub">
              <span style={{ color: 'var(--ok)' }}>{deckElo.wins.toLocaleString()}W</span>
              {' · '}
              <span style={{ color: 'var(--danger)' }}>{deckElo.losses.toLocaleString()}L</span>
            </div>
          ) : (
            <div className="deck-vital-signs__sub" style={{ opacity: 0.55 }}>NO SAMPLE</div>
          )}
        </div>
      </div>

      {/* AT A GLANCE — bracket + archetype + win conditions + recent gauntlet
          winrate, surfaced together so spectators don't have to scroll the
          full Freya analysis to read the deck. The all-time win rate already
          lives in the vital-signs strip; the value added here is "what's it
          done lately" (last ≤5 gauntlet runs, weighted by games). */}
      {(glance.bracket || glance.archetype || glance.winConditions.length > 0 || glance.recent) && (
        <Panel code="04.AG" title="AT A GLANCE">
          <div className="deck-glance-grid" style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))',
            gap: 12,
          }}>
            <div>
              <div className="t-xs muted">BRACKET</div>
              <div className="t-lg" style={{ fontWeight: 700, marginTop: 2 }}>
                {glance.bracket
                  ? `B${glance.bracket}${glance.playsLike ? ` → B${glance.playsLike}` : ''}`
                  : '—'}
              </div>
              {glance.bracketLabel && (
                <div className="t-xs muted" style={{ marginTop: 2 }}>{glance.bracketLabel.toUpperCase()}</div>
              )}
            </div>
            <div>
              <div className="t-xs muted">ARCHETYPE</div>
              <div className="t-lg" style={{ fontWeight: 700, marginTop: 2 }}>
                {glance.archetype || '—'}
              </div>
            </div>
            <div>
              <div className="t-xs muted">RECENT WIN RATE</div>
              <div className="t-lg" style={{ fontWeight: 700, marginTop: 2 }}>
                {glance.recent
                  ? `${glance.recent.pct}%`
                  : glance.allTime
                    ? `${glance.allTime.pct}%`
                    : '—'}
              </div>
              <div className="t-xs muted" style={{ marginTop: 2 }}>
                {glance.recent
                  ? `LAST ${glance.recent.runs} RUN${glance.recent.runs === 1 ? '' : 'S'} · ${glance.recent.games.toLocaleString()} GAMES`
                  : glance.allTime
                    ? `ALL-TIME · ${glance.allTime.games.toLocaleString()} GAMES`
                    : 'NO GAUNTLET YET'}
              </div>
            </div>
            <div>
              <div className="t-xs muted">WIN CONDITIONS</div>
              {glance.winConditions.length > 0 ? (
                <div style={{ marginTop: 4, display: 'flex', flexWrap: 'wrap', gap: 4 }}>
                  {glance.winConditions.map((name, i) => (
                    <CardLink key={i} name={name}>
                      <Tag>{name.toUpperCase()}</Tag>
                    </CardLink>
                  ))}
                </div>
              ) : (
                <div className="t-md" style={{ fontWeight: 700, marginTop: 2 }}>—</div>
              )}
            </div>
          </div>
        </Panel>
      )}

      {/* Deck stats summary — always visible between hero and main columns. */}
      <div className="deck-stats-summary-row">
        <DeckStatsSummary cards={cards} />
      </div>

      <div className="archive-layout">
        <div className="archive-sidebar">
          <Panel code="04.A" title="DECK SPECS" solid>
            {/* BRACKET / PLAYS LIKE moved to the vital-signs strip above
                (rendered as "B{wbs} → B{pls}" inside the POWER LEVEL cell).
                Kept here only as the archetype/legality/themes detail block. */}
            <KV rows={[
              ['OWNER', <Link to={`/profile/${owner}`} style={{ color: 'var(--ink)', textDecoration: 'none', borderBottom: '1px dotted var(--ink-3)' }}>{owner?.toUpperCase()}</Link>],
              ...(ownerFriendCount != null ? [['FRIENDS', String(ownerFriendCount)]] : []),
              ['CARDS', `${cardCount}`],
              ['GAME CHANGERS', gameChangers != null ? `${gameChangers}` : '—', 'game_changers'],
              ['ARCHETYPE', archetype, 'archetype'],
              ...(legality ? [['LEGALITY', <span style={{ color: legality.valid ? 'var(--ok)' : 'var(--danger)', fontWeight: 700 }}>{legality.valid ? 'LEGAL' : 'ILLEGAL'}</span>, 'legality']] : []),
              ...(manaBaseGrade ? [['MANA BASE', manaBaseGrade, 'mana_base_grade']] : []),
              ...(powerPercentile != null ? [['POWER', `TOP ${powerPercentile}%`, 'power_percentile']] : []),
              ...(commanderSynergy != null ? [['COMMANDER SYNERGY', `${Math.round(commanderSynergy * 100)}%`, 'cmdr_synergy']] : []),
              ...(keepableHandPct != null ? [['KEEPABLE HANDS', `${Math.round(keepableHandPct)}%`, 'keepable_hands']] : []),
              ...(interactionAvgCmc != null ? [['INTERACTION CMC', `AVG ${Math.round(interactionAvgCmc * 10) / 10}`, 'interaction_avg_cmc']] : []),
              ...(cheapInteraction != null ? [['CHEAP REMOVAL', `${cheapInteraction} AT ≤2 CMC`, 'cheap_interaction']] : []),
            ]} />
            {commanderThemes.length > 0 && (
              <div style={{ marginTop: 8, display: 'flex', flexWrap: 'wrap', gap: 4 }}>
                {commanderThemes.map((t, i) => <Tag key={i}>{t.toUpperCase()}</Tag>)}
              </div>
            )}
            {deckElo && (
              <>
                <div className="hr" style={{ margin: '10px 0' }} />
                {/* HexELO / Record / Win Rate / Games moved to the vital-signs strip
                    above the layout. This block keeps only the confidence dots and
                    delta — the per-session ELO movement worth seeing in detail. */}
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
                  <GlossaryTerm term="confidence" compact>
                    <span className="t-xs muted">CONFIDENCE</span>
                  </GlossaryTerm>
                  <ConfidenceDots games={deckElo.games} showLabel size="lg" />
                </div>
                {deckElo.delta != null && deckElo.delta !== 0 && (
                  <KV rows={[
                    ['DELTA', <span style={{ color: deckElo.delta >= 0 ? 'var(--ok)' : 'var(--danger)' }}>{deckElo.delta >= 0 ? '+' : ''}{Math.round(deckElo.delta)}</span>, 'delta'],
                  ]} />
                )}
              </>
            )}
            {isOwner && owner && id && (
              <>
                <div className="hr" style={{ margin: '10px 0' }} />
                <DeckRating userSlug={userOwnerSlug} deckKey={`${owner}/${id}`} />
                <div className="hr" style={{ margin: '10px 0' }} />
                <DeckShareDisclosure owner={owner} id={id} />
              </>
            )}
            <div className="hr" style={{ margin: '10px 0' }} />
            <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
              {owner && id && (
                <>
                  <ContextBox id="deck.edit" compact>Opens the deck in the Workshop — the builder view where you swap cards and tune the list. Saving re-runs Freya analysis on the new list.</ContextBox>
                  <Btn arrow="↗" onClick={() => {
                    if (editing) return
                    const lines = cards.map(c => {
                      const cmdr = deck?.commander_card
                      if (cmdr && c.name === cmdr) return `COMMANDER: ${c.name}`
                      return c.quantity > 1 ? `${c.quantity} ${c.name}` : `1 ${c.name}`
                    })
                    setEditText(lines.join('\n'))
                    setEditing(true)
                    api.getDeckVersions(`${owner}/${id}`).then(setVersions).catch(() => {})
                  }}>WORKSHOP</Btn>
                </>
              )}
              <ContextBox id="deck.export" compact>Downloads the decklist in your chosen format (Moxfield, Arena, plain text).</ContextBox>
              <Btn ghost arrow="↗" onClick={() => {
                if (!cards.length) return
                setExportOpen(true)
              }}>EXPORT</Btn>
              {analyzing && <Tag solid kind="info">ANALYZING...</Tag>}
              {owner && id && (
                <>
                  <ContextBox id="deck.forge" compact>Opens this deck in the Forge — interactive playtester for testing draws, mulligans, and lines.</ContextBox>
                  <Btn ghost arrow="↗" onClick={() => navigate(`/forge?deck=${owner}/${id}`)}>OPEN IN FORGE</Btn>
                </>
              )}
              {owner && id && !isOwner && user && (
                <>
                  <ContextBox id="deck.clone" compact>Copies this deck into your account so you can edit and tune your own version. The clone re-runs Freya analysis on import.</ContextBox>
                  {!confirmClone ? (
                    <Btn solid arrow="⎘" onClick={() => setConfirmClone(true)}>
                      CLONE DECK
                    </Btn>
                  ) : (
                    <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
                      <Btn
                        solid
                        arrow="⎘"
                        disabled={cloning}
                        onClick={() => {
                          if (cloning) return
                          setCloning(true)
                          trackEvent('clone_deck', { deck: `${owner}/${id}` })
                          api.cloneDeck(`${owner}/${id}`).then(res => {
                            toast.success('DECK CLONED — RUNNING FREYA')
                            navigate(`/decks/${res.owner}/${res.id}`)
                          }).catch(err => {
                            if (err?.status === 401) toast.error('SIGN IN TO CLONE')
                            else if (err?.status === 429) toast.error('CLONE LIMIT REACHED — TRY AGAIN IN AN HOUR')
                            else if (err?.status === 400) toast.error(err.message || 'CLONE REJECTED')
                            else if (err?.status === 404) toast.error('SOURCE DECK NOT FOUND')
                            else toast.error('CLONE FAILED')
                            setCloning(false)
                            setConfirmClone(false)
                          })
                        }}
                      >
                        {cloning ? 'CLONING (FREYA RUNNING)...' : 'CONFIRM CLONE'}
                      </Btn>
                      <Btn ghost arrow="✕" onClick={() => setConfirmClone(false)} disabled={cloning}>CANCEL</Btn>
                    </div>
                  )}
                </>
              )}
              {owner && id && !isOwner && user && (
                <>
                  <ContextBox id="deck.fork" compact>Forks this deck into your own collection with attribution preserved — "Forked from {owner}/{id}" stays visible on your copy, so the original builder gets credit. Use FORK when you're building on top of someone's public work; use CLONE for a quieter private duplicate.</ContextBox>
                  {!confirmFork ? (
                    <Btn arrow="⑂" onClick={() => setConfirmFork(true)}>
                      FORK DECK
                    </Btn>
                  ) : (
                    <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
                      <Btn
                        arrow="⑂"
                        disabled={forking}
                        onClick={() => {
                          if (forking) return
                          setForking(true)
                          trackEvent('fork_deck', { deck: `${owner}/${id}` })
                          api.forkDeck(`${owner}/${id}`).then(res => {
                            toast.success('DECK FORKED — RUNNING FREYA')
                            navigate(`/decks/${res.owner}/${res.id}`)
                          }).catch(err => {
                            if (err?.status === 401) toast.error('SIGN IN TO FORK')
                            else if (err?.status === 429) toast.error('FORK LIMIT REACHED — TRY AGAIN IN AN HOUR')
                            else if (err?.status === 400) toast.error(err.message || 'FORK REJECTED')
                            else if (err?.status === 404) toast.error('SOURCE DECK NOT FOUND')
                            else toast.error('FORK FAILED')
                            setForking(false)
                            setConfirmFork(false)
                          })
                        }}
                      >
                        {forking ? 'FORKING (FREYA RUNNING)...' : 'CONFIRM FORK'}
                      </Btn>
                      <Btn ghost arrow="✕" onClick={() => setConfirmFork(false)} disabled={forking}>CANCEL</Btn>
                    </div>
                  )}
                </>
              )}
              {owner && id && !isOwner && !user && (
                <>
                  <ContextBox id="deck.clone" compact>Sign in to clone this deck into your own collection — Freya will re-analyze the copy on import.</ContextBox>
                  <Btn ghost arrow="↗" onClick={() => navigate('/login')}>SIGN IN TO CLONE</Btn>
                </>
              )}
              {owner && id && (
                <>
                  <div className="hr" style={{ margin: '4px 0' }} />
                  {!confirmDelete ? (
                    <>
                      <ContextBox id="deck.delete" compact tone="danger">Permanently removes this deck and its analysis. This cannot be undone.</ContextBox>
                      <Btn ghost onClick={() => setConfirmDelete(true)} style={{ color: 'var(--danger)', borderColor: 'var(--danger)' }}>DELETE DECK</Btn>
                    </>
                  ) : (
                    <>
                      <ContextBox compact tone="danger">Final confirmation — CONFIRM deletes the deck for good. CANCEL backs out.</ContextBox>
                      <div style={{ display: 'flex', gap: 6 }}>
                        <Btn solid onClick={() => {
                          api.deleteDeck(`${owner}/${id}`).then(() => navigate('/decks')).catch(() => setConfirmDelete(false))
                        }} style={{ flex: 1, background: 'var(--danger)', borderColor: 'var(--danger)' }}>CONFIRM</Btn>
                        <Btn ghost onClick={() => setConfirmDelete(false)} style={{ flex: 1 }}>CANCEL</Btn>
                      </div>
                    </>
                  )}
                </>
              )}
            </div>
            {owner && (
              <>
                <div className="hr" style={{ margin: '10px 0' }} />
                <BadgeShowcase owner={owner} />
              </>
            )}
          </Panel>

          {/* MATCHUPS — head-to-head record per opposing commander
              from showmatch_game history. Best/worst leaderboards
              gate on a min-games threshold so 1-0 small samples don't
              dominate the rankings. */}
          <MatchupsPanel owner={owner} id={id} />

          {/* SIMILAR DECKS — server-ranked by shared-card overlap with
              bonuses for matching commander / archetype / bracket. The
              endpoint already drops noise (≤10 shared cards and no
              bonus); an empty response means we genuinely have nothing
              to recommend yet. */}
          <Panel
            code="04.SIM"
            title={`SIMILAR DECKS / / ${similarDecks == null ? '…' : similarDecks.length}`}
            right={similarDecks && similarDecks.length > 0 ? <Tag solid>{similarDecks.length}</Tag> : null}
          >
            {similarDecks == null ? (
              <div className="t-xs muted" style={{ padding: '10px 0', textAlign: 'center' }}>
                &gt; SCANNING DECK INDEX<span className="blink">_</span>
              </div>
            ) : similarDecks.length === 0 ? (
              <div className="t-xs muted" style={{ padding: '10px 0', textAlign: 'center', lineHeight: 1.6 }}>
                &gt; NO SIMILAR DECKS FOUND.
              </div>
            ) : (
              <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                {similarDecks.map((d, i) => {
                  const cmdrArt = d.commander_card ? cardArtUrl(d.commander_card) : null
                  const showName = (d.commander || d.name || d.id || '').toUpperCase()
                  const tags = []
                  if (d.same_commander) tags.push('CMDR')
                  if (d.same_archetype) tags.push('ARCHE')
                  if (d.same_bracket)   tags.push(`B${d.bracket}`)
                  return (
                    <Link
                      key={`${d.owner}/${d.id}`}
                      to={`/decks/${d.owner}/${d.id}`}
                      style={{
                        display: 'grid',
                        gridTemplateColumns: '52px 1fr',
                        gap: 8,
                        padding: 4,
                        border: '1px solid var(--rule-2)',
                        textDecoration: 'none',
                        color: 'var(--ink)',
                        background: i === 0 ? 'color-mix(in srgb, var(--accent) 10%, transparent)' : 'transparent',
                      }}
                      title={`${showName} · ${d.shared_cards} shared`}
                    >
                      <div
                        className={cmdrArt ? '' : 'hatch'}
                        style={{
                          width: 52, height: 40, overflow: 'hidden',
                          backgroundImage: cmdrArt ? `url(${cmdrArt})` : undefined,
                          backgroundSize: 'cover', backgroundPosition: 'center 30%',
                          filter: 'saturate(0.6) contrast(1.05)',
                        }}
                      />
                      <div style={{ minWidth: 0 }}>
                        <div className="t-xs" style={{
                          fontWeight: 700, letterSpacing: '0.04em',
                          overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                        }}>
                          {showName}
                        </div>
                        <div className="t-xs muted-2" style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                          {(d.owner || '').toUpperCase()}
                        </div>
                        <div className="t-xs" style={{
                          marginTop: 2, display: 'flex', alignItems: 'center', gap: 6, flexWrap: 'wrap',
                        }}>
                          <span style={{ color: 'var(--ok)', fontWeight: 700, fontVariantNumeric: 'tabular-nums' }}>
                            {d.shared_cards} SHARED
                          </span>
                          {tags.map(t => (
                            <span key={t} style={{
                              fontSize: 8, letterSpacing: '0.08em', padding: '0 4px',
                              border: '1px solid color-mix(in srgb, var(--accent) 50%, var(--rule-2))',
                              color: 'var(--ink-2)',
                            }}>{t}</span>
                          ))}
                        </div>
                      </div>
                    </Link>
                  )
                })}
              </div>
            )}
          </Panel>
        </div>

        <div className="archive-main">
          {/* Tab bar */}
          <div className="deck-tabs">
            <button type="button" className={`deck-tab ${activeTab === 'analysis' ? 'active' : ''}`} onClick={() => setActiveTab('analysis')}>ANALYSIS</button>
            <button type="button" className={`deck-tab ${activeTab === 'decklist' ? 'active' : ''}`} onClick={() => setActiveTab('decklist')}>DECK LIST</button>
            <button type="button" className={`deck-tab ${activeTab === 'history' ? 'active' : ''}`} onClick={() => setActiveTab('history')} data-testid="history-tab">HISTORY</button>
            <button type="button" className={`deck-tab ${activeTab === 'achievements' ? 'active' : ''}`} onClick={() => setActiveTab('achievements')}>ACHIEVEMENTS</button>
          </div>

          {/* Edit mode — always visible regardless of tab */}
          {editing && (
            <div ref={editPanelRef}>
            <Panel code="04.X" title="WORKSHOP / / DECK LIST" right={
              <span className="t-xs" style={{ color: 'var(--warn)' }}>IN WORKSHOP</span>
            }>
              <WorkshopAddCard onAdd={(cardName) => {
                // Append "1 CardName" to the bottom of editText, deduplicating
                // against existing lines so a card already in the list bumps
                // to qty+1 instead of getting a second "1 CardName" entry.
                const lines = editText.split('\n')
                const existingIdx = lines.findIndex(l => {
                  const m = l.match(/^(\d+)\s+(.+)$/)
                  return m && m[2].trim() === cardName
                })
                if (existingIdx >= 0) {
                  const m = lines[existingIdx].match(/^(\d+)\s+(.+)$/)
                  if (m) lines[existingIdx] = `${parseInt(m[1], 10) + 1} ${m[2]}`
                } else {
                  lines.push(`1 ${cardName}`)
                }
                setEditText(lines.filter(l => l !== '' || lines.indexOf(l) === lines.length - 1).join('\n'))
              }} />
              <WorkshopTextarea value={editText} onChange={setEditText} />
              <ContextBox id="deck.edit-save" style={{ marginTop: 10 }}>
                <strong>SAVE UPDATE</strong> writes a new version of the deck and re-runs Freya analysis.
                {' '}<strong>CANCEL</strong> discards your edits.
              </ContextBox>
              <WorkshopDiff baseline={originalEditText} current={editText} />
              <div style={{ display: 'flex', gap: 8 }}>
                <Btn solid onClick={() => {
                  if (!editText.trim() || saving) return
                  setSaving(true)
                  api.updateDeck(`${owner}/${id}`, editText).then(updated => {
                    setEditing(false)
                    setSaving(false)
                    setAnalyzing(true)
                    api.getDeck(`${owner}/${id}`).then(setDeck)
                    api.getDeckVersions(`${owner}/${id}`).then(setVersions).catch(() => {})
                  }).catch(() => setSaving(false))
                }}>{saving ? 'SAVING...' : 'SAVE UPDATE'}</Btn>
                {/* REVERT — reset to baseline without leaving the workshop.
                    Only renders when there's something to revert. */}
                {editText !== originalEditText && (
                  <Btn ghost onClick={() => setEditText(originalEditText)}
                       title="Reset textarea to the workshop-open snapshot">
                    REVERT
                  </Btn>
                )}
                <Btn ghost onClick={() => { setEditing(false); setSaving(false) }}>CANCEL</Btn>
              </div>
              {versions.length > 0 && (
                <div style={{ marginTop: 12 }}>
                  <div className="t-xs muted" style={{ marginBottom: 6 }}>VERSION HISTORY</div>
                  {versions.slice(0, 10).map((v, i) => (
                    <div key={i} style={{ display: 'flex', justifyContent: 'space-between', padding: '3px 0', borderBottom: '1px dotted var(--rule)' }}>
                      <span className="t-xs">V{v.version}</span>
                      <span className="t-xs muted">{v.saved_at ? new Date(v.saved_at).toLocaleDateString() : ''}</span>
                    </div>
                  ))}
                </div>
              )}
            </Panel>
            </div>
          )}

          {/* === ANALYSIS TAB === */}
          {activeTab === 'analysis' && <>
          {/* USD price rollup. Lazy-fetched on first analysis-tab
              mount via /api/decks/{id}/budget — pulls from the
              Scryfall-backed oracle cache and shows total, top
              contributors, unpriced count. Sits above the Freya
              panel so the "your deck costs $X" headline is the
              first thing visible. */}
          <DeckBudgetPanel deckId={`${owner}/${id}`} />
          <Panel code="04.C" title="FREYA / / ENGINE ANALYSIS" right={<Tag solid>{wbs ? `Bracket B${wbs}${pls && pls !== wbs ? ` → Plays Like B${pls}` : ''}` : 'Bracket pending'}</Tag>}>
            {!analysis ? (
              <div style={{ padding: '20px 0', textAlign: 'center' }}>
                <div className="t-md muted" style={{ lineHeight: 1.8, textTransform: 'uppercase', letterSpacing: '0.04em' }}>
                  {analyzing ? (
                    <>&gt; FREYA ENGINE ANALYZING DECK<span className="blink">_</span><br />&gt; DETECTING COMBOS, SYNERGIES, WIN LINES...<br />&gt; THIS MAY TAKE A FEW SECONDS</>
                  ) : (
                    <>&gt; NO FREYA ANALYSIS ON FILE<br />&gt; RUN <span style={{ color: 'var(--ink)' }}>HEXDEK-FREYA</span> TO GENERATE STRATEGY REPORT<br />&gt; BRACKET, ARCHETYPE, WIN LINES, STRATEGY FOCUS<span className="blink">_</span></>
                  )}
                </div>
              </div>
            ) : (
              <div className="analysis-grid">
                <div>
                  <div className="t-xs muted">ARCHETYPE</div>
                  <div className="t-2xl" style={{ fontWeight: 700, marginTop: 2 }}>{archetype}</div>
                </div>
                <div className="analysis-weights">
                  <div className="t-xs muted">STRATEGY FOCUS</div>
                  {Object.entries(evalWeights).slice(0, 6).map(([k, v], i) => (
                    <div key={i} style={{ display: 'grid', gridTemplateColumns: '100px 1fr 36px', alignItems: 'center', gap: 6, marginTop: 6 }}>
                      <span className="t-xs" style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{k.replace(/_/g, ' ').toUpperCase()}</span>
                      <Bar value={v * 100} />
                      <span className="t-xs muted text-right">{Math.round(v * 100) / 100}</span>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </Panel>

          {/* Credits posture — shown above the run button so the user
              sees free-tier remaining + balance before clicking. Only
              renders for signed-in users; the panel itself silently
              degrades on 401. */}
          {owner && id && user && (
            <CreditsPanel compact refreshKey={creditsRefreshKey} />
          )}

          {/* Gauntlet button — prominent, right under Freya */}
          {owner && id && (
            <div>
              <ContextBox id="deck.run-actions">
                <strong>RUN GAUNTLET (500)</strong> queues 500 AI-vs-AI games against bracket-matched meta decks on the server. Win rate, ELO delta, and best/worst matchups land in the GAUNTLET REPORT panel below; takes a few minutes.
                {' '}<strong>SPECTATE LIVE</strong> spawns a fresh 4-player room with this deck and opens the live spectator view — you can watch every decision as the AI plays it out.
                {' '}<strong>TEST VERSION</strong> opens a scratch editor where you can swap cards and rerun Freya analysis without overwriting the saved deck.
              </ContextBox>
              <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap' }}>
              <Btn solid arrow="▶" onClick={() => {
                if (gauntlet?.status === 'running') return
                trackEvent('start_gauntlet', { deck: `${owner}/${id}`, games: 500 })
                api.startGauntlet(`${owner}/${id}`, 500).then((res) => {
                  // Surface "credits charged" feedback so the user knows
                  // a paid run actually debited their balance.
                  if (res?.credits_charged) {
                    toast.info(`PAID RUN — ${res.credits_charged} CR DEDUCTED`)
                    setCreditsRefreshKey(k => k + 1)
                  }
                  const poll = () => {
                    api.getGauntlet(`${owner}/${id}`).then(r => {
                      setGauntlet(r)
                      if (r.status === 'running') setTimeout(poll, 3000)
                    })
                  }
                  setTimeout(poll, 2000)
                  setGauntlet({ status: 'running', games: 0, target: 500, win_rate: 0 })
                }).catch(err => {
                  // 402 from the server when the user is out of free
                  // runs and has no credits, or when the spend itself
                  // would overdraft. services/api.js's unwrapApiError
                  // exposes the error code + details across both r60
                  // (nested) and pre-r60 (flat) envelope shapes, so
                  // we can switch on err.code regardless of which
                  // backend version is answering during the deploy
                  // crossover.
                  if (err?.status === 402) {
                    const details = err?.details || {}
                    if (err?.code === 'free_quota_exhausted') {
                      toast.error('OUT OF FREE GAUNTLETS — EARN CREDITS OR WAIT FOR RESET')
                    } else {
                      toast.error(`INSUFFICIENT CREDITS — NEED ${details.needed ?? '?'} CR (HAVE ${details.balance ?? 0})`)
                    }
                    setCreditsRefreshKey(k => k + 1)
                  } else if (err?.status === 401) {
                    toast.error('SIGN IN TO RUN A GAUNTLET')
                  } else if (err?.status === 429) {
                    toast.error('SERVER BUSY — TRY AGAIN SOON')
                  } else {
                    toast.error('GAUNTLET FAILED TO START')
                  }
                })
              }}>{gauntlet?.status === 'running' ? 'GAUNTLET RUNNING...' : 'RUN GAUNTLET (500)'}</Btn>
              <Btn solid arrow="▶" onClick={() => {
                if (spawningRoom) return
                setSpawningRoom(true)
                trackEvent('spawn_spectate_room', { deck: `${owner}/${id}` })
                api.spawnSpectateRoom(`${owner}/${id}`).then(r => {
                  setSpawningRoom(false)
                  if (r.room_id) navigate(`/spectate/${r.room_id}`)
                }).catch(() => setSpawningRoom(false))
              }}>{spawningRoom ? 'SPAWNING...' : 'SPECTATE LIVE'}</Btn>
              <Btn ghost arrow="▶">TEST VERSION</Btn>
              </div>
            </div>
          )}

          {gauntlet && gauntlet.status !== 'none' && (
            <Panel code="04.G" title="GAUNTLET REPORT" right={
              gauntlet.status === 'running'
                ? <Tag kind="warn">{`${gauntlet.games}/${gauntlet.target}`}</Tag>
                : <Tag solid kind={gauntlet.status === 'complete' ? 'ok' : 'bad'}>
                    {gauntlet.status?.toUpperCase()}
                  </Tag>
            }>
              {gauntlet.status === 'running' ? (
                <div style={{ padding: '16px 0', textAlign: 'center' }}>
                  <div className="t-md muted" style={{ lineHeight: 1.8, textTransform: 'uppercase', letterSpacing: '0.04em' }}>
                    &gt; GAUNTLET IN PROGRESS<span className="blink">_</span><br />
                    &gt; {gauntlet.games?.toLocaleString()} / {gauntlet.target?.toLocaleString()} GAMES ({gauntlet.win_rate || 0}% WIN RATE)
                  </div>
                  <Bar value={gauntlet.games / gauntlet.target * 100} />
                </div>
              ) : gauntlet.status === 'complete' ? (
                <div>
                  <div className="gauntlet-stat-grid">
                    <div>
                      <div className="t-xs muted">WIN RATE</div>
                      <div className="t-2xl gauntlet-stat-num" style={{ fontWeight: 700, color: gauntlet.win_rate >= 25 ? 'var(--ok)' : 'var(--danger)' }}>{gauntlet.win_rate}%</div>
                    </div>
                    <div>
                      <div className="t-xs muted">RECORD</div>
                      <div className="t-2xl gauntlet-stat-num" style={{ fontWeight: 700 }}><span style={{ color: 'var(--ok)' }}>{gauntlet.wins}W</span> — <span style={{ color: 'var(--danger)' }}>{gauntlet.losses}L</span></div>
                    </div>
                    <div>
                      <div className="t-xs muted">ELO DELTA</div>
                      <div className="t-2xl gauntlet-stat-num" style={{ fontWeight: 700, color: gauntlet.elo_delta >= 0 ? 'var(--ok)' : 'var(--danger)' }}>
                        {gauntlet.elo_delta >= 0 ? '+' : ''}{Math.round(gauntlet.elo_delta)}
                      </div>
                    </div>
                  </div>
                  <KV rows={[
                    ['GAMES', `${gauntlet.games?.toLocaleString()}`],
                    ['AVG TURNS', `${gauntlet.avg_turns}`],
                    // STARTING / ENDING ELO frame the calibration arc; the
                    // delta itself is already prominent in the tile above,
                    // so we don't repeat it here.
                    ['STARTING ELO', gauntlet.elo_start != null ? `${Math.round(gauntlet.elo_start)}` : '—'],
                    ['ENDING ELO', gauntlet.elo_end != null ? `${Math.round(gauntlet.elo_end)}` : '—'],
                  ]} />

                  {/* Finishing-position breakdown — shows the per-place
                      distribution that the binary W/L view hides. Backed by
                      gauntlet.placements [4]int (1st/2nd/3rd/4th). Hidden
                      when placements is absent (pre-update runs) to stay
                      backward compatible with cached results. */}
                  {gauntlet.placements?.length === 4 && gauntlet.games > 0 && (
                    <>
                      <div className="hr" style={{ margin: '8px 0' }} />
                      <div className="t-xs muted" style={{ marginBottom: 4 }}>FINISHING POSITION</div>
                      <div style={{ display: 'grid', gridTemplateColumns: 'auto 1fr 64px', gap: '4px 8px', fontSize: 11 }}>
                        {[
                          { label: '1st (wins)', idx: 0, color: 'var(--ok)' },
                          { label: '2nd',        idx: 1, color: 'var(--ink)' },
                          { label: '3rd',        idx: 2, color: 'var(--ink-2)' },
                          { label: '4th',        idx: 3, color: 'var(--danger)' },
                        ].map(row => {
                          const n = gauntlet.placements[row.idx] || 0
                          const pct = gauntlet.games > 0 ? (n / gauntlet.games * 100) : 0
                          return (
                            <div key={row.idx} style={{ display: 'contents' }}>
                              <span style={{ color: row.color, letterSpacing: '0.05em' }}>{row.label}</span>
                              <div style={{ height: 12, background: 'var(--bg-2, rgba(0,0,0,0.2))', border: '1px solid var(--rule-2)', position: 'relative' }}>
                                <div style={{ position: 'absolute', inset: 0, width: `${pct}%`, background: row.color, opacity: 0.5 }} />
                              </div>
                              <span style={{ textAlign: 'right', fontVariantNumeric: 'tabular-nums', color: row.color }}>
                                {n} · {pct.toFixed(1)}%
                              </span>
                            </div>
                          )
                        })}
                      </div>
                    </>
                  )}

                  {gauntlet.top_beaten?.length > 0 && (
                    <>
                      <div className="hr" style={{ margin: '8px 0' }} />
                      <div className="t-xs muted" style={{ marginBottom: 4 }}>MOST BEATEN</div>
                      {gauntlet.top_beaten.map((b, i) => (
                        <div key={i} className="t-xs" style={{ color: 'var(--ok)', padding: '1px 0' }}>&gt; {b}</div>
                      ))}
                    </>
                  )}
                  {gauntlet.top_lost_to?.length > 0 && (
                    <>
                      <div className="hr" style={{ margin: '8px 0' }} />
                      <div className="t-xs muted" style={{ marginBottom: 4 }}>MOST LOST TO</div>
                      {gauntlet.top_lost_to.map((b, i) => (
                        <div key={i} className="t-xs" style={{ color: 'var(--danger)', padding: '1px 0' }}>&gt; {b}</div>
                      ))}
                    </>
                  )}
                </div>
              ) : gauntlet.status === 'error' ? (
                <div className="t-xs" style={{ color: 'var(--danger)', padding: '10px 0' }}>
                  &gt; GAUNTLET ERROR — deck may not be loaded in the engine pool. Try again or contact support.
                </div>
              ) : null}
            </Panel>
          )}

          {/* Mana Curve + Color Balance */}
          {curveData && (
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 14 }} className="curve-grid">
              <Panel code="04.M" title="MANA CURVE">
                <ManaCurveChart
                  distribution={curveData.distribution}
                  avgCmc={curveData.avg_cmc}
                  curveShape={curveData.curve_shape}
                  warnings={curveData.warnings}
                  landCount={curveData.land_count}
                  nonlandCount={curveData.nonland_count}
                  colorByCmc={computeColorByCmc(cards)}
                />
              </Panel>
              <Panel code="04.N" title="COLOR BALANCE">
                <ColorPie demand={colorData} />
                {isMultiColor && manaProduction && colorData && (() => {
                  const MANA_COLORS = { W: '#E0EBD3', U: '#6E8FA0', B: '#3a3628', R: '#CC5C4A', G: '#82C472', C: '#8A9682' }
                  const allColors = [...new Set([...Object.keys(colorData), ...Object.keys(manaProduction)])].filter(k => (colorData[k] || 0) > 0).sort()
                  const totalProd = allColors.reduce((s, k) => s + (manaProduction[k] || 0), 0)
                  const totalDem = allColors.reduce((s, k) => s + (colorData[k] || 0), 0)
                  if (totalProd === 0 || totalDem === 0) return null
                  return (
                    <div style={{ marginTop: 12 }}>
                      <div className="t-xs muted" style={{ marginBottom: 6 }}>PRODUCTION vs DEMAND</div>
                      {allColors.map(color => {
                        const prodPct = Math.round(((manaProduction[color] || 0) / totalProd) * 100)
                        const demPct = Math.round(((colorData[color] || 0) / totalDem) * 100)
                        const diff = prodPct - demPct
                        const ok = diff >= -3
                        return (
                          <div key={color} style={{ marginBottom: 6 }}>
                            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', marginBottom: 2 }}>
                              <span className="t-xs" style={{ fontWeight: 700 }}>{color}</span>
                              <span className="t-xs" style={{ color: ok ? 'var(--ok)' : 'var(--danger)' }}>
                                {prodPct}% / {demPct}%{diff !== 0 ? ` (${diff > 0 ? '+' : ''}${diff})` : ''}
                              </span>
                            </div>
                            <div style={{ display: 'flex', gap: 1, height: 6 }}>
                              <div style={{ width: `${prodPct}%`, height: '100%', background: MANA_COLORS[color] || 'var(--ink-3)', opacity: 0.9, borderRadius: 1 }} title={`Production: ${prodPct}% (${manaProduction[color] || 0} sources)`} />
                            </div>
                            <div style={{ display: 'flex', gap: 1, height: 3, marginTop: 1 }}>
                              <div style={{ width: `${demPct}%`, height: '100%', background: 'var(--ink-3)', opacity: 0.4, borderRadius: 1 }} title={`Demand: ${demPct}% (${colorData[color] || 0} pips)`} />
                            </div>
                          </div>
                        )
                      })}
                      <div className="t-xs muted" style={{ marginTop: 4 }}>% OF SOURCES / % OF PIPS</div>
                    </div>
                  )
                })()}
                {analysis?.color_balance?.warnings?.length > 0 && (
                  <div style={{ marginTop: 8, display: 'flex', flexWrap: 'wrap', gap: 4 }}>
                    {analysis.color_balance.warnings.map((w, i) => <Tag key={i} kind="warn" solid>{w}</Tag>)}
                  </div>
                )}
              </Panel>
            </div>
          )}

          {/* Win lines */}
          {winLines.length > 0 && (() => {
            const WINLINE_CAP = 8
            const visible = winLinesExpanded ? winLines : winLines.slice(0, WINLINE_CAP)
            const hidden = winLines.length - WINLINE_CAP
            return (
            <Panel code="04.D" title={`WIN LINES / / ${winLines.length} DETECTED`}>
              {visible.map((wl, i) => {
                const kindMap = { finisher: 'bad', combat: 'warn', commander_damage: 'ok', combo: 'bad', synergy: null }
                const symbols = ['α', 'β', 'γ', 'δ', 'ε', 'ζ']
                return (
                  <div key={i} className="winline-row" style={{ padding: '10px 0', borderBottom: i < visible.length - 1 ? '1px dashed var(--rule-2)' : 'none' }}>
                    <div style={{ fontSize: 24, fontWeight: 700, color: kindMap[wl.type] === 'bad' ? 'var(--danger)' : kindMap[wl.type] === 'warn' ? 'var(--warn)' : kindMap[wl.type] === 'ok' ? 'var(--ok)' : 'var(--ink)' }}>
                      {symbols[i] || '·'}
                    </div>
                    <Tag kind={kindMap[wl.type]} solid>{wl.type?.toUpperCase()}</Tag>
                    <div>
                      <div className="t-md" style={{ fontWeight: 700 }}>{wl.pieces?.join(' + ')}</div>
                      {wl.tutor_paths && (
                        <div className="t-xs muted" style={{ marginTop: 2 }}>
                          TUTORS: {wl.tutor_paths.map(t => t.tutor).join(', ')}
                        </div>
                      )}
                    </div>
                  </div>
                )
              })}
              {!winLinesExpanded && hidden > 0 && (
                <button
                  type="button"
                  onClick={() => setWinLinesExpanded(true)}
                  style={{ width: '100%', padding: '10px 0', marginTop: 6, background: 'none', border: '1px dashed var(--rule-2)', color: 'var(--ink-2)', fontFamily: 'inherit', fontSize: 11, fontWeight: 700, letterSpacing: '0.06em', textTransform: 'uppercase', cursor: 'pointer' }}
                >
                  SHOW {hidden} MORE WIN LINE{hidden === 1 ? '' : 'S'} ↓
                </button>
              )}
              {winLinesExpanded && winLines.length > WINLINE_CAP && (
                <button
                  type="button"
                  onClick={() => setWinLinesExpanded(false)}
                  style={{ width: '100%', padding: '10px 0', marginTop: 6, background: 'none', border: '1px dashed var(--rule-2)', color: 'var(--ink-2)', fontFamily: 'inherit', fontSize: 11, fontWeight: 700, letterSpacing: '0.06em', textTransform: 'uppercase', cursor: 'pointer' }}
                >
                  COLLAPSE ↑
                </button>
              )}
            </Panel>
            )
          })()}

          {/* Win condition rationale — explains detection logic per line */}
          <WinConditionRationale winLines={winLines} />

          {/* Legality violations */}
          {legality && !legality.valid && (
            <Panel code="04.L" title="LEGALITY VIOLATIONS" right={<Tag kind="bad" solid>ILLEGAL</Tag>}>
              {legality.errors?.map((e, i) => (
                <div key={i} className="t-xs" style={{ color: 'var(--danger)', padding: '2px 0' }}>&gt; {e}</div>
              ))}
              {legality.warnings?.map((w, i) => (
                <div key={i} className="t-xs" style={{ color: 'var(--warn)', padding: '2px 0' }}>&gt; {w}</div>
              ))}
            </Panel>
          )}

          {/* Warnings: curve, color, combo */}
          {(curveWarnings.length > 0 || colorMismatch.length > 0 || comboNotes.length > 0) && (
            <Panel code="04.W" title="WARNINGS" right={<Tag kind="warn" solid>{curveWarnings.length + colorMismatch.length + comboNotes.length}</Tag>}>
              {curveWarnings.map((w, i) => (
                <div key={`c${i}`} className="t-xs" style={{ color: 'var(--warn)', padding: '2px 0' }}>&gt; CURVE: {w}</div>
              ))}
              {colorMismatch.map((w, i) => (
                <div key={`m${i}`} className="t-xs" style={{ color: 'var(--warn)', padding: '2px 0' }}>&gt; COLOR: {w}</div>
              ))}
              {comboNotes.map((w, i) => (
                <div key={`n${i}`} className="t-xs" style={{ color: 'var(--ink-2)', padding: '2px 0' }}>&gt; COMBO: {w}</div>
              ))}
            </Panel>
          )}

          {/* Meta matchups */}
          {metaMatchups.length > 0 && (
            <CollapsiblePanel code="04.MM" title={`META POSITIONING / / ${archetype}`}>
              <div style={{ display: 'grid', gap: 0 }}>
                {metaMatchups.map((m, i) => {
                  const ratingColor = m.rating === 'favored' ? 'var(--ok)' : m.rating === 'unfavored' ? 'var(--danger)' : 'var(--ink-2)'
                  const ratingSymbol = m.rating === 'favored' ? '▲' : m.rating === 'unfavored' ? '▼' : '—'
                  return (
                    <div key={i} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '6px 0', borderBottom: i < metaMatchups.length - 1 ? '1px dotted var(--rule)' : 'none' }}>
                      <div>
                        <span className="t-xs" style={{ fontWeight: 700 }}>vs {m.archetype?.toUpperCase()}</span>
                        {m.reason && <div className="t-xs muted" style={{ marginTop: 1 }}>{m.reason}</div>}
                      </div>
                      <Tag solid kind={m.rating === 'favored' ? 'ok' : m.rating === 'unfavored' ? 'bad' : null}>
                        {ratingSymbol} {m.rating?.toUpperCase()}
                      </Tag>
                    </div>
                  )
                })}
              </div>
            </CollapsiblePanel>
          )}

          {/* Vulnerable to */}
          {vulnerableTo.length > 0 && (
            <CollapsiblePanel code="04.V" title="VULNERABLE TO">
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
                {vulnerableTo.map((v, i) => <Tag key={i} kind="warn" solid>{v.toUpperCase()}</Tag>)}
              </div>
            </CollapsiblePanel>
          )}

          {/* Star cards */}
          {starCards.length > 0 && (
            <CollapsiblePanel code="04.S" title={`STAR CARDS / / ${starCards.length}`}>
              <div className="grid col-5 gap-2">
                {starCards.slice(0, 10).map((name, i) => (
                  <CardThumb key={i} name={name} score="★" />
                ))}
              </div>
            </CollapsiblePanel>
          )}

          {/* Finisher cards */}
          {finisherCards.length > 0 && (
            <CollapsiblePanel code="04.K" title={`WIN CONDITIONS / / ${finisherCards.length}`}>
              <div className="grid col-5 gap-2">
                {finisherCards.slice(0, 10).map((name, i) => (
                  <CardThumb key={i} name={name} />
                ))}
              </div>
            </CollapsiblePanel>
          )}

          {/* Value engine keys */}
          {valueKeys.length > 0 && (
            <CollapsiblePanel code="04.E" title={`VALUE ENGINE / / ${valueKeys.length} KEY CARDS`}>
              <div className="grid col-5 gap-2">
                {valueKeys.slice(0, 10).map((name, i) => (
                  <CardThumb key={i} name={name} />
                ))}
              </div>
            </CollapsiblePanel>
          )}

          {/* Value engine rationale — explains why each engine was identified */}
          <ValueEngineRationale chains={valueChains} />

          {/* Game Changer cards */}
          {gameChangerCards.length > 0 && (
            <CollapsiblePanel code="04.GC" title={`GAME CHANGERS / / ${gameChangerCards.length}`} right={<Tag kind="bad" solid>B4+</Tag>}>
              <div className="grid col-5 gap-2">
                {gameChangerCards.map((name, i) => (
                  <CardThumb key={i} name={name} />
                ))}
              </div>
            </CollapsiblePanel>
          )}

          {/* Card packages (theme-grouped synergy clusters) */}
          {emergentSynergies.length > 0 && (
            <CollapsiblePanel code="04.H" title={`CARD PACKAGES / / ${emergentSynergies.length} DISCOVERED`}>
              {emergentSynergies.slice(0, 12).map((syn, i) => (
                <div key={i} style={{ padding: '6px 0', borderBottom: i < emergentSynergies.length - 1 ? '1px dashed var(--rule-2)' : 'none', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <div>
                    <div className="t-md" style={{ fontWeight: 700 }}>{syn.cards?.join(' + ')}</div>
                    {syn.effect_pattern && <div className="t-xs muted" style={{ marginTop: 2 }}>{syn.effect_pattern}</div>}
                  </div>
                  <div style={{ textAlign: 'right', whiteSpace: 'nowrap' }}>
                    <Tag solid kind={syn.tier >= 3 ? 'ok' : null}>T{syn.tier}</Tag>
                    {syn.observation_count > 0 && <span className="t-xs muted" style={{ marginLeft: 6 }}>{syn.observation_count}× seen</span>}
                  </div>
                </div>
              ))}
            </CollapsiblePanel>
          )}

          {/* Cuttable cards rationale (replaces older thumbnail-only panel).
              Owners get a working CUT button that opens the workshop with
              that card already removed from the textarea — Save Update
              commits. Non-owners see the CUT label as a passive flag. */}
          <ConsiderCuttingRationale
            cuts={cuttableCards}
            onCut={isOwner ? handleCutCardFromWorkshop : undefined}
          />

          {/* Tutor targets */}
          {analysis?.tutor_targets && (
            <CollapsiblePanel code="04.F" title="TUTOR TARGETS">
              <KV rows={analysis.tutor_targets.map((t, i) => [`TARGET.${i + 1}`, t])} />
            </CollapsiblePanel>
          )}

          {/* PR #78 — Matchup Matrix panel. Pulls from /api/decks/{id}/matchups.
              Each row carries opponent commander + W/L counts + WR. Hidden
              when matchup data is empty (deck hasn't been gauntleted enough
              to have head-to-head records yet). */}
          {matchupMatrix && matchupMatrix.length > 0 && (
            <Panel code="04.MX" title={`MATCHUP MATRIX / / ${matchupMatrix.length} OPPONENTS`}>
              <div className="t-xs muted" style={{ marginBottom: 6 }}>
                Head-to-head record against each opponent commander. Color-coded by win rate vs the 25% 4-player baseline.
              </div>
              {/* Per-row flex layout instead of CSS grid contents — lets the
                  opponent name take the full row width and pushes the stat
                  group onto a second line on narrow viewports. Each row is
                  its own block so wrap is per-opponent, not global. */}
              <div className="matchup-matrix">
                {matchupMatrix.slice(0, 30).map((m, i) => {
                  const games = (m.wins || 0) + (m.losses || 0)
                  if (games === 0) return null
                  const wr = (m.wins || 0) / games * 100
                  const color = wr >= 35 ? 'var(--ok)' : wr >= 20 ? 'var(--ink-2)' : 'var(--danger)'
                  return (
                    <div key={i} className="matchup-matrix__row">
                      <span className="matchup-matrix__name" title={m.opponent_commander || m.opponent || '?'}>
                        {m.opponent_commander || m.opponent || '?'}
                      </span>
                      <span className="matchup-matrix__stats">
                        <span className="t-xs muted">{games}g</span>
                        <span style={{ color, fontWeight: 700, fontVariantNumeric: 'tabular-nums' }}>{wr.toFixed(0)}%</span>
                        <span style={{ fontVariantNumeric: 'tabular-nums' }}>
                          <span style={{ color: 'var(--ok)' }}>{m.wins || 0}W</span>
                          <span className="muted">—</span>
                          <span style={{ color: 'var(--danger)' }}>{m.losses || 0}L</span>
                        </span>
                      </span>
                    </div>
                  )
                })}
              </div>
              {matchupMatrix.length > 30 && (
                <div className="t-xs muted" style={{ marginTop: 6 }}>
                  Showing top 30 of {matchupMatrix.length}. Full matrix coming in a future expansion.
                </div>
              )}
            </Panel>
          )}

          {/* PR #78 — Commander Card Stats panel. Aggregates across all
              decks of this commander to surface which cards correlate with
              wins. Top performers + worst performers, with sample sizes
              so users can sanity-check. Empty state hides the panel until
              the commander has enough data. */}
          {commanderCardStats && commanderCardStats.length > 0 && (
            <Panel code="04.CS" title={`CARD STATS / / ${deck?.commander_card || 'COMMANDER'} ECOSYSTEM`}>
              <div className="t-xs muted" style={{ marginBottom: 6 }}>
                Win rate of each card across all decks running {deck?.commander_card || 'this commander'}. Filtered to cards in YOUR list. ≥20 games for sample-size confidence.
              </div>
              {(() => {
                // Filter to cards that are actually in this deck's list, with
                // enough sample size to be meaningful. Sort by win rate desc.
                const deckCardNames = new Set(cards.map(c => c.name))
                // Backend ships PascalCase (CardName/Games/Wins/WinRate); legacy
                // pipelines used snake_case. Read both so the panel survives
                // either schema.
                const rows = commanderCardStats
                  .filter(s => deckCardNames.has(s.card_name || s.name || s.CardName))
                  .filter(s => (s.games_included || s.games || s.Games || 0) >= 20)
                  .map(s => {
                    const games = s.games_included || s.games || s.Games || 0
                    const wins = s.wins_when_included || s.wins || s.Wins || 0
                    return {
                      name: s.card_name || s.name || s.CardName,
                      games,
                      wins,
                      wr: games > 0 ? wins / games * 100 : 0,
                    }
                  })
                  .sort((a, b) => b.wr - a.wr)
                if (rows.length === 0) {
                  return (
                    <div className="t-xs muted" style={{ padding: '10px 0' }}>
                      &gt; Not enough card-level data yet for this commander. Run more gauntlets to populate.
                    </div>
                  )
                }
                const top = rows.slice(0, 8)
                const bottom = rows.length > 16 ? rows.slice(-8).reverse() : []
                return (
                  <>
                    <div className="t-xs muted" style={{ marginBottom: 4, marginTop: 4 }}>TOP PERFORMERS</div>
                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 60px 60px', gap: '2px 8px', fontSize: 10 }}>
                      {top.map((r, i) => (
                        <div key={i} style={{ display: 'contents' }}>
                          <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{r.name}</span>
                          <span style={{ textAlign: 'right', fontVariantNumeric: 'tabular-nums', color: 'var(--ok)', fontWeight: 700 }}>{r.wr.toFixed(1)}%</span>
                          <span style={{ textAlign: 'right', fontVariantNumeric: 'tabular-nums', color: 'var(--ink-3)' }}>{r.games}g</span>
                        </div>
                      ))}
                    </div>
                    {bottom.length > 0 && (
                      <>
                        <div className="hr" style={{ margin: '8px 0' }} />
                        <div className="t-xs muted" style={{ marginBottom: 4 }}>UNDERPERFORMERS</div>
                        <div style={{ display: 'grid', gridTemplateColumns: '1fr 60px 60px', gap: '2px 8px', fontSize: 10 }}>
                          {bottom.map((r, i) => (
                            <div key={i} style={{ display: 'contents' }}>
                              <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{r.name}</span>
                              <span style={{ textAlign: 'right', fontVariantNumeric: 'tabular-nums', color: 'var(--danger)', fontWeight: 700 }}>{r.wr.toFixed(1)}%</span>
                              <span style={{ textAlign: 'right', fontVariantNumeric: 'tabular-nums', color: 'var(--ink-3)' }}>{r.games}g</span>
                            </div>
                          ))}
                        </div>
                      </>
                    )}
                  </>
                )
              })()}
            </Panel>
          )}

          {/* HOT CARDS — top 5 by win-rate contribution.
              Primary source: /api/deck-card-stats/{owner}/{id} (per-deck —
              richer signal, server already intersected with this deck's
              list and computed delta vs a data-driven baseline). Fallback:
              the commander aggregate from CARD STATS above, used when the
              new endpoint 404s or returns no matches. In both cases tiles
              are sorted by lift = delta × √games so a 5-game hot streak
              can't outrank a 200-game performer. Mobile-friendly: tiles
              wrap from 5 columns down to 2 on narrow viewports. */}
          {(() => {
            const perDeckRows = deckCardStats?.cards
            const usingPerDeck = Array.isArray(perDeckRows) && perDeckRows.length > 0
            let baseline = 25
            let ranked = []
            if (usingPerDeck) {
              // Server returns win_rate / win_rate_delta as 0..1; the widget
              // displays percentages, so scale once here.
              baseline = (typeof deckCardStats.baseline_win_rate === 'number'
                ? deckCardStats.baseline_win_rate
                : 0.25) * 100
              ranked = perDeckRows
                .map(s => {
                  const games = s.games || 0
                  const wins = s.wins || 0
                  const wr = (typeof s.win_rate === 'number' ? s.win_rate : (games > 0 ? wins / games : 0)) * 100
                  const delta = (typeof s.win_rate_delta === 'number' ? s.win_rate_delta * 100 : wr - baseline)
                  return { name: s.card_name, games, wins, wr, lift: delta * Math.sqrt(games) }
                })
                .filter(r => r.lift > 0)
                .sort((a, b) => b.lift - a.lift)
                .slice(0, 5)
            } else if (commanderCardStats && commanderCardStats.length > 0) {
              // Fallback: commander aggregate. PascalCase + snake_case schema
              // compat; see CARD STATS above.
              const deckCardNames = new Set(cards.map(c => c.name))
              ranked = commanderCardStats
                .filter(s => deckCardNames.has(s.card_name || s.name || s.CardName))
                .filter(s => (s.games_included || s.games || s.Games || 0) >= 20)
                .map(s => {
                  const games = s.games_included || s.games || s.Games || 0
                  const wins = s.wins_when_included || s.wins || s.Wins || 0
                  const wr = games > 0 ? wins / games * 100 : 0
                  return { name: s.card_name || s.name || s.CardName, games, wins, wr, lift: (wr - baseline) * Math.sqrt(games) }
                })
                .filter(r => r.lift > 0)
                .sort((a, b) => b.lift - a.lift)
                .slice(0, 5)
            }
            if (ranked.length === 0) return null
            const description = usingPerDeck
              ? `Cards in this deck pulling the most weight across recorded games — sorted by win-rate lift over the ${baseline.toFixed(0)}% baseline, sample-size weighted (√games).`
              : `Cards in this deck pulling the most weight in ${deck?.commander_card || 'commander'} games — sorted by win-rate lift over the 25% 4-player baseline, sample-size weighted (√games).`
            return (
              <Panel code="04.HC" title={`HOT CARDS / / TOP ${ranked.length} BY WR CONTRIBUTION`}>
                <div className="t-xs muted" style={{ marginBottom: 8 }}>
                  {description}
                </div>
                <div className="hot-cards-grid">
                  {ranked.map((r, i) => (
                    <div key={i} className="hot-cards-tile">
                      <CardThumb name={r.name} />
                      {/* WR chip pinned to the top-right of the art. games + lift
                          chip sits in the top-left so neither overlaps the card
                          name footer rendered by CardThumb. */}
                      <span className="hot-cards-chip hot-cards-chip--wr">{r.wr.toFixed(0)}%</span>
                      <span className="hot-cards-chip hot-cards-chip--games">{r.games}g · +{(r.wr - baseline).toFixed(0)}</span>
                    </div>
                  ))}
                </div>
              </Panel>
            )
          })()}

          {/* SIMILAR DECKS — thumbnail tile version below HOT CARDS.
              Reuses the similarDecks fetch already running for the sidebar
              widget. Surfaces commander art, owner, and live HexELO so a
              reader scanning a deck page can jump straight to a peer build
              without scrolling back up to the sidebar. Hidden until the
              fetch resolves with at least one match. */}
          {similarDecks && similarDecks.length > 0 && (
            <Panel code="04.SD" title={`SIMILAR DECKS / / ${similarDecks.length} MATCH${similarDecks.length === 1 ? '' : 'ES'}`}>
              <div className="t-xs muted" style={{ marginBottom: 8 }}>
                Decks ranked by shared-card overlap with bonuses for matching commander, archetype, and bracket. HexELO pulled live from the gauntlet ladder.
              </div>
              <div className="similar-decks-grid">
                {similarDecks.map((d) => {
                  const cmdrArt = d.commander_card ? cardArtUrl(d.commander_card) : null
                  const showName = (d.commander || d.name || d.id || '').toUpperCase()
                  const peerElo = eloByDeckId[`${d.owner}/${d.id}`] || eloByDeckId[d.id] || null
                  const hexRating = peerElo && peerElo.hex_rating ? Math.round(peerElo.hex_rating) : null
                  return (
                    <Link
                      key={`sd-${d.owner}/${d.id}`}
                      to={`/decks/${d.owner}/${d.id}`}
                      className="panel similar-decks-tile"
                      title={`${showName} · ${d.owner} · ${d.shared_cards} shared`}
                    >
                      <div
                        className={`similar-decks-tile__art ${cmdrArt ? '' : 'hatch'}`}
                        style={cmdrArt ? { backgroundImage: `url(${cmdrArt})` } : undefined}
                      >
                        {hexRating != null && (
                          <span className="similar-decks-tile__chip similar-decks-tile__chip--elo">{hexRating}</span>
                        )}
                        <span className="similar-decks-tile__chip similar-decks-tile__chip--shared">{d.shared_cards} SHARED</span>
                      </div>
                      <div className="similar-decks-tile__body">
                        <div className="similar-decks-tile__name t-xs">{showName}</div>
                        <div className="similar-decks-tile__owner t-xs muted-2">{(d.owner || '').toUpperCase()}</div>
                      </div>
                    </Link>
                  )
                })}
              </div>
            </Panel>
          )}

          {/* RECENT GAMES — last 10 persisted GameSummary rows this deck
              appeared in. Each row navigates to /games/:id/summary. Hidden
              when the archive has nothing yet so the rail stays clean. */}
          {recentGames && recentGames.length > 0 && (() => {
            const myKey = `${owner}/${id}`
            const summary = summarizeRecentGames(recentGames, myKey)
            return (
              <Panel
                code="04.RG"
                title={`RECENT GAMES / / ${recentGames.length}`}
                right={summary.total > 0 ? (
                  <span className="t-xs muted">
                    <span style={{ color: 'var(--ok)' }}>{summary.wins}W</span>
                    {' · '}
                    <span style={{ color: 'var(--danger)' }}>{summary.losses}L</span>
                    {summary.draws > 0 && (
                      <>
                        {' · '}
                        <span className="muted-2">{summary.draws}D</span>
                      </>
                    )}
                  </span>
                ) : null}
              >
                <div className="t-xs muted" style={{ marginBottom: 6 }}>
                  Last {recentGames.length} archived games this deck appeared in (newest first). Click a row to open the post-game summary.
                </div>
                <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 11 }} data-testid="recent-games-table">
                  <thead>
                    <tr style={{ borderBottom: '1px solid var(--rule-2)', textAlign: 'left' }}>
                      <th style={{ padding: '4px 6px', width: 50 }}>RESULT</th>
                      <th style={{ padding: '4px 6px' }}>GAME</th>
                      <th style={{ padding: '4px 6px' }}>OPPONENTS</th>
                      <th style={{ padding: '4px 6px', width: 38 }}>T</th>
                      <th style={{ padding: '4px 6px' }}>END</th>
                      <th style={{ padding: '4px 6px', width: 80 }}>WHEN</th>
                    </tr>
                  </thead>
                  <tbody>
                    {recentGames.map((r) => {
                      const outcome = outcomeForDeck(r, myKey)
                      const opps = opponentCommanders(r, myKey)
                      const resultColor =
                        outcome === 'win'  ? 'var(--ok)'      :
                        outcome === 'loss' ? 'var(--danger)'  :
                        outcome === 'draw' ? 'var(--ink-2)'   : 'var(--ink-3)'
                      const resultLabel =
                        outcome === 'win'  ? 'WIN'  :
                        outcome === 'loss' ? 'LOSS' :
                        outcome === 'draw' ? 'DRAW' : '—'
                      return (
                        <tr
                          key={r.game_id}
                          onClick={() => navigate(`/games/${r.game_id}/summary`)}
                          style={{ borderBottom: '1px solid var(--rule-3)', cursor: 'pointer' }}
                          data-testid={`recent-game-${r.game_id}`}
                          title="Open game summary"
                        >
                          <td style={{ padding: '4px 6px', fontWeight: 700, color: resultColor, letterSpacing: '0.06em' }}>
                            {resultLabel}
                          </td>
                          <td style={{ padding: '4px 6px' }}><strong>#{r.game_id}</strong></td>
                          <td style={{ padding: '4px 6px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 0 }}>
                            <span className="muted">vs </span>{opps.join(' · ') || '—'}
                          </td>
                          <td style={{ padding: '4px 6px' }}>{r.turns || '—'}</td>
                          <td style={{ padding: '4px 6px' }}>
                            <span className="muted-2">{(r.end_reason || '').toUpperCase() || '—'}</span>
                          </td>
                          <td style={{ padding: '4px 6px' }}>
                            <span className="muted">{formatRelativeFinished(r.finished_at)}</span>
                          </td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              </Panel>
            )
          })()}

          {/* PR #79 — ELO HISTORY chart. Pulls /api/decks/{id}/elo-history,
              renders rating-over-time SVG. Hidden when no runs exist (new
              deck, never gauntleted) so we don't show an empty axis box. */}
          {eloHistory && eloHistory.length >= 2 && (
            <Panel code="04.EH" title={`ELO HISTORY / / ${eloHistory.length} RUNS`}>
              <div className="t-xs muted" style={{ marginBottom: 6 }}>
                Rating trajectory across the last {eloHistory.length} completed gauntlets, oldest left → newest right. Calibration arc visible — big swings early, smaller swings as the rating converges on true position.
              </div>
              {(() => {
                // Collect series + axis math. Use elo_end as the canonical
                // post-run rating; show elo_start as a faint dotted track
                // for "starting point of each run" context.
                const ends = eloHistory.map(r => r.elo_end || 0)
                const starts = eloHistory.map(r => r.elo_start || 0)
                const all = [...ends, ...starts]
                const minY = Math.min(...all)
                const maxY = Math.max(...all)
                const padY = Math.max(50, (maxY - minY) * 0.1)
                const lo = Math.floor((minY - padY) / 100) * 100
                const hi = Math.ceil((maxY + padY) / 100) * 100
                const w = 600  // viewBox width
                const h = 160 // viewBox height
                const pad = { top: 10, right: 10, bottom: 22, left: 44 }
                const plotW = w - pad.left - pad.right
                const plotH = h - pad.top - pad.bottom
                const xAt = (i) => pad.left + (eloHistory.length === 1 ? plotW / 2 : (plotW * i / (eloHistory.length - 1)))
                const yAt = (v) => pad.top + plotH - ((v - lo) / (hi - lo) * plotH)
                const endLine = ends.map((v, i) => `${i === 0 ? 'M' : 'L'} ${xAt(i).toFixed(1)} ${yAt(v).toFixed(1)}`).join(' ')
                const startLine = starts.map((v, i) => `${i === 0 ? 'M' : 'L'} ${xAt(i).toFixed(1)} ${yAt(v).toFixed(1)}`).join(' ')
                // Y-axis ticks: 4 evenly-spaced rating gridlines
                const ticks = [0, 0.25, 0.5, 0.75, 1].map(t => Math.round((lo + (hi - lo) * t) / 10) * 10)
                return (
                  <>
                    <svg viewBox={`0 0 ${w} ${h}`} preserveAspectRatio="none"
                         style={{ width: '100%', height: 200, display: 'block', border: '1px solid var(--rule-2)', background: 'var(--bg-2, rgba(0,0,0,0.12))' }}>
                      {/* Gridlines */}
                      {ticks.map((t, i) => (
                        <g key={`g-${i}`}>
                          <line x1={pad.left} x2={w - pad.right}
                                y1={yAt(t)} y2={yAt(t)}
                                stroke="var(--rule-2)" strokeWidth="0.5" strokeDasharray="2,3" />
                          <text x={pad.left - 5} y={yAt(t) + 3}
                                textAnchor="end" fontSize="8" fill="var(--ink-3)"
                                fontFamily="inherit" letterSpacing="0.04em">{t}</text>
                        </g>
                      ))}
                      {/* Start-of-run track (faint, dotted) */}
                      <path d={startLine}
                            stroke="var(--ink-3)"
                            strokeWidth="1" strokeDasharray="3,2"
                            fill="none" opacity="0.6" />
                      {/* End-of-run line (primary) */}
                      <path d={endLine}
                            stroke="var(--accent, var(--ink))"
                            strokeWidth="2"
                            fill="none" />
                      {/* End-of-run dots, color-coded by delta direction */}
                      {ends.map((v, i) => {
                        const delta = eloHistory[i].elo_delta || 0
                        const fill = delta >= 0 ? 'var(--ok)' : 'var(--danger)'
                        return (
                          <circle key={`d-${i}`}
                                  cx={xAt(i)} cy={yAt(v)} r="3"
                                  fill={fill}
                                  stroke="var(--bg)" strokeWidth="0.5">
                            <title>{`Run ${i + 1}: ELO ${Math.round(eloHistory[i].elo_start)} → ${Math.round(v)} (Δ${delta >= 0 ? '+' : ''}${Math.round(delta)}, WR ${eloHistory[i].win_rate}%, ${eloHistory[i].games} games)`}</title>
                          </circle>
                        )
                      })}
                      {/* X-axis ticks: first, middle, last run */}
                      {[0, Math.floor((eloHistory.length - 1) / 2), eloHistory.length - 1].filter((v, i, a) => a.indexOf(v) === i && v >= 0).map((i) => {
                        const r = eloHistory[i]
                        const label = r?.finished_at ? new Date(r.finished_at).toLocaleDateString(undefined, { month: 'numeric', day: 'numeric' }) : `R${i + 1}`
                        return (
                          <text key={`x-${i}`}
                                x={xAt(i)} y={h - 6}
                                textAnchor="middle" fontSize="8" fill="var(--ink-3)"
                                fontFamily="inherit" letterSpacing="0.04em">{label}</text>
                        )
                      })}
                    </svg>
                    <div className="t-xs muted" style={{ marginTop: 6, display: 'flex', justifyContent: 'space-between', flexWrap: 'wrap', gap: 8 }}>
                      <span>━ ending ELO  · · ·  starting ELO</span>
                      <span>● green = positive delta · red = negative · hover for run details</span>
                    </div>
                  </>
                )
              })()}
            </Panel>
          )}
          {eloHistory && eloHistory.length === 1 && (
            <div className="t-xs muted" style={{ padding: '6px 12px' }}>
              &gt; Run a second gauntlet to start populating the ELO history chart.
            </div>
          )}

          {curse && (
            <div className="archive-curse-section">
              <CurseDisplay
                curse={curse}
                isOwner={isOwner}
                deckId={deckKey}
                onConstraintsChange={(constraints) => setCurse(c => ({ ...(c || {}), constraints }))}
              />
            </div>
          )}

          </>}

          {/* === DECK LIST TAB === */}
          {activeTab === 'decklist' && <>
          {cardSearchQuery && (
            <div className="panel" style={{
              display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 10,
              padding: '6px 10px', fontSize: 10, letterSpacing: '0.08em', textTransform: 'uppercase',
              borderStyle: 'solid', borderColor: 'var(--accent)',
            }}>
              <span>
                FILTER: <strong>"{cardSearch}"</strong>
                <span className="muted"> · {filteredCards.length} / {cards.length} CARDS</span>
              </span>
              <span style={{ display: 'inline-flex', gap: 6 }}>
                <Tag solid style={{ cursor: 'pointer' }} onClick={() => setCardSearchOpen(true)}>EDIT</Tag>
                <Tag style={{ cursor: 'pointer' }} onClick={() => setCardSearch('')}>CLEAR ✕</Tag>
              </span>
            </div>
          )}
          {cards.length > 0 && (
            <div
              data-testid="decklist-view-toggle"
              style={{ display: 'flex', gap: 6, alignItems: 'center', padding: '0 2px' }}
            >
              <span className="t-xs muted" style={{ letterSpacing: '0.08em', marginRight: 4 }}>VIEW:</span>
              <Tag
                solid={decklistView === 'tiles'}
                onClick={() => setDecklistView('tiles')}
                style={{ cursor: 'pointer' }}
                data-testid="decklist-view-tiles"
              >
                TILES
              </Tag>
              <Tag
                solid={decklistView === 'dense'}
                onClick={() => setDecklistView('dense')}
                style={{ cursor: 'pointer' }}
                data-testid="decklist-view-dense"
              >
                DENSE
              </Tag>
              <span className="t-xs muted" style={{ marginLeft: 'auto', letterSpacing: '0.06em' }}>
                {decklistView === 'dense'
                  ? `${filteredCards.length} ROWS · SORT BY ${denseSort.key.toUpperCase()} ${denseSort.dir.toUpperCase()}`
                  : `${filteredCards.length} CARDS GROUPED BY ROLE`}
              </span>
            </div>
          )}

          {cards.length > 0 && decklistView === 'tiles' && (
            <CardRolesGrid cards={filteredCards} cardRoles={filteredCardRoles} />
          )}

          {cards.length > 0 && decklistView === 'tiles' && (
            <Panel code="04.B" title={`FULL CARD LIST / / ${cardSearchQuery ? `${filteredCards.length} / ${cards.length}` : cards.length} ENTRIES`}>
              <div>
                {filteredCards.length === 0 ? (
                  <div className="t-xs muted" style={{ padding: '20px 0', textAlign: 'center' }}>
                    &gt; NO CARDS MATCH "{cardSearch}"
                  </div>
                ) : filteredCards.map((c, i) => {
                  const linkName = (c.name || '').replace(/^COMMANDER:\s*/i, '').trim()
                  return (
                    <div key={i} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 8, padding: '3px 0', borderBottom: i < filteredCards.length - 1 ? '1px dotted var(--rule)' : 'none' }}>
                      <CardLink name={linkName} className="t-xs" style={{ borderBottom: 'none', minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        {c.name}
                      </CardLink>
                      <span style={{ display: 'flex', alignItems: 'center', gap: 6, flexShrink: 0 }}>
                        {c.mana_cost && <ManaCost cost={c.mana_cost} size={12} gap={1} />}
                        <span className="t-xs muted">{c.quantity > 1 ? `×${c.quantity}` : ''}</span>
                      </span>
                    </div>
                  )
                })}
              </div>
            </Panel>
          )}

          {cards.length > 0 && decklistView === 'dense' && (
            <Panel code="04.B" title={`DENSE CARD LIST / / ${cardSearchQuery ? `${filteredCards.length} / ${cards.length}` : cards.length} ENTRIES`}>
              <CardListDense
                cards={filteredCards}
                cardRoles={filteredCardRoles}
                sort={denseSort}
                onSort={(key) => setDenseSort(s => toggleSort(s, key))}
                coachingIndex={coachingIndex}
                onCut={isOwner ? handleCutCardFromWorkshop : undefined}
              />
            </Panel>
          )}
          </>}

          {/* === HISTORY TAB === */}
          {activeTab === 'history' && (
            <DeckHistoryPanel
              versions={versions}
              deckId={`${owner}/${id}`}
              currentDeckText={cards.map(c => {
                const cmdr = deck?.commander_card
                if (cmdr && c.name === cmdr) return `COMMANDER: ${c.name}`
                const qty = c.quantity > 1 ? c.quantity : 1
                return `${qty} ${c.name}`
              }).join('\n')}
              commanderName={deck?.commander_card || deck?.commander || ''}
            />
          )}

          {/* === ACHIEVEMENTS TAB === */}
          {activeTab === 'achievements' && <>
          {achievements && (achievements.badges?.length > 0 || achievements.total_games > 0) ? (
            <Panel
              code="04.ACH"
              title={`ACHIEVEMENTS / / ${owner?.toUpperCase() || ''}`}
              right={<Tag solid kind={achievements.badges?.length > 0 ? 'ok' : null}>{achievements.badges?.length || 0} EARNED</Tag>}
            >
              {(achievements.total_games > 0 || achievements.opponents_faced > 0) && (
                <KV rows={[
                  ['GAMES', `${achievements.total_games?.toLocaleString() || 0}`],
                  ['WINS', `${achievements.total_wins?.toLocaleString() || 0}`],
                  ['STREAK', `${achievements.current_win_streak || 0} (BEST ${achievements.max_win_streak || 0})`],
                  ['OPPONENTS', `${achievements.opponents_faced?.toLocaleString() || 0}`],
                ]} />
              )}
              {achievements.badges?.length > 0 && (
                <>
                  <div className="hr" style={{ margin: '10px 0' }} />
                  {(() => {
                    const RARITY_COLOR = {
                      common:   { border: '#8a9682', bg: 'rgba(138,150,130,0.06)', label: 'COMMON' },
                      uncommon: { border: '#6e8fa0', bg: 'rgba(110,143,160,0.08)', label: 'UNCOMMON' },
                      rare:     { border: '#d8c878', bg: 'rgba(216,200,120,0.10)', label: 'RARE' },
                      mythic:   { border: '#cc5c4a', bg: 'rgba(204,92,74,0.12)', label: 'MYTHIC' },
                      secret:   { border: '#9c6ab0', bg: 'rgba(156,106,176,0.14)', label: 'SECRET' },
                    }
                    const catalogById = {}
                    for (const b of (achievements.catalog || [])) catalogById[b.id] = b
                    return (
                      <div style={{
                        display: 'grid',
                        gridTemplateColumns: 'repeat(auto-fill, minmax(140px, 1fr))',
                        gap: 8,
                      }}>
                        {achievements.badges.map(badge => {
                          const def = catalogById[badge.id] || badge
                          const palette = RARITY_COLOR[def.rarity] || RARITY_COLOR.common
                          return (
                            <div
                              key={badge.id}
                              title={`${def.name}\n${def.description}`}
                              style={{
                                border: `2px solid ${palette.border}`,
                                background: palette.bg,
                                padding: '8px 10px',
                                display: 'flex',
                                flexDirection: 'column',
                                gap: 4,
                              }}
                            >
                              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                                <span style={{ fontSize: 22, lineHeight: 1 }}>{def.icon}</span>
                                <span className="t-xs" style={{ color: palette.border, letterSpacing: '0.06em', fontWeight: 700 }}>{palette.label}</span>
                              </div>
                              <div className="t-xs" style={{ fontWeight: 700, letterSpacing: '0.04em' }}>{def.name}</div>
                              <div className="t-xs muted" style={{ lineHeight: 1.3 }}>{def.description}</div>
                              <div className="t-xs muted-2" style={{ marginTop: 2 }}>
                                {badge.awarded_at ? new Date(badge.awarded_at).toLocaleDateString() : ''}
                              </div>
                            </div>
                          )
                        })}
                      </div>
                    )
                  })()}
                </>
              )}
            </Panel>
          ) : (
            <Panel code="04.ACH" title="ACHIEVEMENTS">
              <div className="t-xs muted" style={{ padding: '20px 0', textAlign: 'center', lineHeight: 1.8 }}>
                &gt; NO ACHIEVEMENTS EARNED YET.<br />
                &gt; RUN GAMES TO UNLOCK BADGES.
              </div>
            </Panel>
          )}
          </>}
        </div>
      </div>
      {cardSearchOpen && (
        <CardSearchModal
          value={cardSearch}
          onChange={setCardSearch}
          totalCards={cards.length}
          matchCount={filteredCards.length}
          inputRef={cardSearchInputRef}
          onClose={() => setCardSearchOpen(false)}
        />
      )}
      {exportOpen && (
        <DeckExportModal
          deck={deck}
          deckId={id}
          onClose={() => setExportOpen(false)}
        />
      )}
      {comparePickerOpen && (
        <DeckPicker
          excludeKey={`${owner}/${id}`}
          onClose={() => setComparePickerOpen(false)}
          onPick={(d) => {
            setComparePickerOpen(false)
            navigate(`/compare/${owner}/${id}/${d.owner}/${d.id}`)
          }}
        />
      )}
    </div>
  )
}

function CardSearchModal({ value, onChange, totalCards, matchCount, inputRef, onClose }) {
  const panelRef = useModalKeyboard({ onClose })
  const chromeBtn = {
    background: 'transparent', border: 'none', color: 'inherit',
    font: 'inherit', cursor: 'pointer', padding: 0, letterSpacing: '0.08em',
  }
  return (
    <div
      onMouseDown={onClose}
      style={{
        position: 'fixed',
        inset: 0,
        background: 'rgba(8, 9, 7, 0.55)',
        display: 'flex',
        alignItems: 'flex-start',
        justifyContent: 'center',
        paddingTop: '14vh',
        zIndex: 1000,
      }}
    >
      <div
        ref={panelRef}
        onMouseDown={e => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-label="Find card in deck"
        className="panel"
        style={{
          width: 'min(460px, 92vw)',
          padding: 0,
          background: 'var(--panel)',
          borderColor: 'var(--accent)',
          borderStyle: 'solid',
          boxShadow: '0 12px 40px rgba(0,0,0,0.55)',
        }}
      >
        <div style={{
          display: 'flex', justifyContent: 'space-between', alignItems: 'center',
          padding: '6px 10px', borderBottom: '1px solid var(--rule-2)',
          fontSize: 9, letterSpacing: '0.12em', color: 'var(--ink-3)', fontWeight: 700,
        }}>
          <span>FIND CARD IN DECK</span>
          <button type="button" onClick={onClose} aria-label="Close search" style={chromeBtn}>ESC</button>
        </div>
        <input
          ref={inputRef}
          type="text"
          value={value}
          onChange={e => onChange(e.target.value)}
          onKeyDown={e => { if (e.key === 'Enter') onClose() }}
          placeholder="Type a card name..."
          aria-label="Card name filter"
          style={{
            width: '100%',
            padding: '12px 14px',
            background: 'transparent',
            border: 'none',
            color: 'var(--ink)',
            fontFamily: 'inherit',
            fontSize: 14,
            letterSpacing: '0.04em',
            outline: 'none',
          }}
        />
        <div style={{
          display: 'flex', justifyContent: 'space-between', alignItems: 'center',
          padding: '6px 10px', borderTop: '1px solid var(--rule-2)',
          fontSize: 9, letterSpacing: '0.1em', color: 'var(--ink-3)',
        }}>
          <span>
            {value.trim() ? `${matchCount} / ${totalCards} MATCH` : `${totalCards} CARDS TOTAL`}
          </span>
          <span style={{ display: 'inline-flex', gap: 8, alignItems: 'center' }}>
            {value && (
              <button type="button" onClick={() => onChange('')} style={chromeBtn}>CLEAR</button>
            )}
            <span className="muted">ENTER OR ESC TO CLOSE</span>
          </span>
        </div>
      </div>
    </div>
  )
}
