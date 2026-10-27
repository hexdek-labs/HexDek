import { useState, useMemo, useEffect } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Tape, Tag, ConfidenceDots } from '../components/chrome'
import GlossaryTerm from '../components/GlossaryTerm'
import Meta from './Meta'
import { useLiveSocket } from '../hooks/useLiveSocket'
import { useArtContrast } from '../hooks/useArtContrast'
import { api, cardArtUrl } from '../services/api'
import { countryFlagEmoji } from '../lib/flag'
import ContextBox from '../components/ContextBox'
import { useT, getLocale } from '../i18n'

// A plain `label` survives only where the text stays English in all 25
// locales: HexELO is a product term and TS μ is a symbol. Everything else
// carries a catalog PATH worded at the render site, because a module-level
// map of English is read once at module load and then never re-words on a
// locale change -- the dominant defect of this whole program.
//
// All five paths are REUSED from screens that already settled this exact
// column vocabulary; none of them is new.
const SORT_KEYS = [
  { key: 'hex_rating', label: 'HEXELO',            term: 'hexelo' },
  { key: 'mu',         label: 'TS μ',              term: 'ts_mu' },
  { key: 'games',      labelKey: 'deck.games',     term: 'games' },
  { key: 'win_rate',   labelKey: 'tlm.th_winpct',  term: 'win_rate' },
  { key: 'wins',       labelKey: 'rpt.wins',       term: 'record' },
  { key: 'losses',     labelKey: 'rpt.losses',     term: 'record' },
  { key: 'delta',      labelKey: 'crd.delta',      term: 'delta' },
]

// B1-B5 are plates and stay English; only ALL is a word. Each row carries an
// `id` because the React key used to be the LABEL -- keying a list on
// translated text remounts the whole list on every locale change.
const BRACKETS = [
  { id: 'all', value: null, labelKey: 'common.all' },
  { id: 'b1', value: 1, label: 'B1' },
  { id: 'b2', value: 2, label: 'B2' },
  { id: 'b3', value: 3, label: 'B3' },
  { id: 'b4', value: 4, label: 'B4' },
  { id: 'b5', value: 5, label: 'B5' },
]

const BRACKET_KIND = { 1: undefined, 2: 'ok', 3: 'info', 4: 'warn', 5: 'danger' }

function BandTag({ band, bracket }) {
  if (!band) return null
  return (
    <Tag kind={BRACKET_KIND[bracket]} style={{ fontSize: 8, padding: '1px 4px', marginLeft: 4 }}>
      {band}
    </Tag>
  )
}

// Returns a catalog PATH, not a word: this is a module-level pure function
// with no translator, and its result is read at render time by ShameBadge.
//
// 'MID' here is the slang sense -- mediocre, unremarkable -- and is NOT
// spec.gb_scale_mid, which is the board-SCALE mid (de MITTEL, ja 中). Two
// concepts spelled the same in English, so reusing that key would have
// rendered a deck insult as "medium-sized".
function shameTier(rating) {
  const r = rating ?? 0
  if (r <= -500) return { labelKey: 'lb.tier_uninstall', kind: 'bad' }
  if (r <= -300) return { labelKey: 'lb.tier_packitup', kind: 'bad' }
  if (r <= -200) return { labelKey: 'lb.tier_cooked', kind: 'bad' }
  if (r <= -100) return { labelKey: 'lb.tier_downbad', kind: 'bad' }
  if (r <= 0) return { labelKey: 'lb.tier_mid', kind: 'warn' }
  return null
}

// `rating` is the TrueSkill-style score (can swing very negative for
// decks that lose to high-rated opponents). `hexRating` is the leaderboard
// rank metric, normalized to stay close to 0+ for any deck with a
// reasonable win rate. Showing a shame badge on a deck whose hex_rating
// is positive contradicts the leaderboard's own ranking — only fire when
// BOTH metrics agree the deck is underperforming.
function ShameBadge({ rating, hexRating }) {
  const t = useT()
  // Suppress badge when the hex_rating signal says the deck is fine.
  // 600 is the rough "median deck" threshold (range usually 0-2500).
  if (hexRating != null && hexRating > 600) return null
  const tier = shameTier(rating)
  if (!tier) return null
  return (
    <Tag kind={tier.kind} solid style={{ fontSize: 8, padding: '1px 4px', marginLeft: 4 }}>
      {t(tier.labelKey)}
    </Tag>
  )
}

function DeltaDisplay({ delta }) {
  if (delta == null || delta === 0) return <span className="muted-2">--</span>
  const positive = delta > 0
  return (
    <span style={{ color: positive ? 'var(--ok)' : 'var(--danger)', fontWeight: 600 }}>
      {positive ? '+' : ''}{delta.toFixed(1)}
    </span>
  )
}

// The W and L are real strings, not glyphs: the catalogs carry a settled
// per-locale pair (cs V/P, de S/N, ja 勝/敗, ru П/Пр) and th puts its word
// in FRONT of the number, which is why each side is a whole fragment rather
// than a bare letter, and why there are two keys -- the halves render in
// different colours, so no single composed value is reusable.
//
// These were briefly lb.rec_w/lb.rec_l, which duplicated the pre-existing
// op.wins_suffix/op.losses_suffix: 22 of 25 values were byte-identical and
// in the other 3 the op pair is better (ar full words over single letters,
// hi जीत/हार over a transliteration of the Latin letters G and H, tr G/K).
// Repointed and the duplicates deleted.
function RecordDisplay({ wins, losses }) {
  const t = useT()
  return (
    <span>
      <span style={{ color: 'var(--ok)' }}>{t('op.wins_suffix', { count: wins.toLocaleString() })}</span>
      <span className="muted-2"> - </span>
      <span style={{ color: 'var(--danger)' }}>{t('op.losses_suffix', { count: losses.toLocaleString() })}</span>
    </span>
  )
}

export default function Leaderboard() {
  const t = useT()
  const [searchParams, setSearchParams] = useSearchParams()
  const view = searchParams.get('view') === 'meta' ? 'meta' : 'rankings'
  const setView = (v) => {
    const next = new URLSearchParams(searchParams)
    if (v === 'meta') next.set('view', 'meta')
    else next.delete('view')
    setSearchParams(next, { replace: true })
  }

  return (
    <>
      <div style={{ display: 'flex', gap: 8, padding: '10px 18px 0', alignItems: 'center' }}>
        <Tag solid={view === 'rankings'} onClick={() => setView('rankings')} style={{ cursor: 'pointer' }}>{t('nav.leaderboard')}</Tag>
        <Tag solid={view === 'meta'} onClick={() => setView('meta')} style={{ cursor: 'pointer' }}>{t('nav.meta')}</Tag>
      </div>
      {view === 'meta' ? <Meta /> : <LeaderboardContent />}
    </>
  )
}

function LeaderboardContent() {
  const t = useT()
  // Depends on the locale STRING, not on `t`: useT returns the stable
  // module-level translator, so `[t]` would never re-fire and the tab title
  // would keep whatever language it was mounted in.
  const locale = getLocale()
  useEffect(() => {
    document.title = t('lb.doc_title')
  }, [locale, t])

  const [filter, setFilter] = useState('')
  const [sortKey, setSortKey] = useState('hex_rating')
  const [sortAsc, setSortAsc] = useState(false)
  const [bracket, setBracket] = useState(null)
  const [countries, setCountries] = useState({}) // { owner -> "US" }
  const navigate = useNavigate()
  const { elo } = useLiveSocket()

  // Batch-fetch country flags for every unique owner present in the
  // ELO snapshot. One round trip per leaderboard load (capped at 200
  // owners on the backend) — avoids the N+1 fetch pattern of asking
  // /api/profile/{owner} per row. Re-runs when new owners appear in
  // the live ELO feed; the union check below skips refetches when no
  // new owner has shown up.
  useEffect(() => {
    if (!elo || elo.length === 0) return
    const owners = []
    const seen = new Set()
    for (const e of elo) {
      const o = e.owner
      if (!o || seen.has(o)) continue
      seen.add(o)
      owners.push(o)
    }
    const missing = owners.filter(o => !(o in countries))
    if (missing.length === 0) return
    api.getOwnerProfiles(missing)
      .then(map => setCountries(prev => ({ ...prev, ...map })))
      .catch(() => {})
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [elo])

  const flagFor = (owner) => countryFlagEmoji(countries[owner])

  const handleSort = (key) => {
    if (sortKey === key) {
      setSortAsc(!sortAsc)
    } else {
      setSortKey(key)
      setSortAsc(false)
    }
  }

  const sorted = useMemo(() => {
    let list = [...(elo || [])]

    if (bracket != null) {
      list = list.filter(e => e.bracket === bracket)
    }

    if (filter) {
      const q = filter.toLowerCase()
      list = list.filter(e => {
        const haystack = `${e.commander || ''} ${e.owner || ''}`.toLowerCase()
        return haystack.includes(q)
      })
    }

    list.sort((a, b) => {
      const av = a[sortKey] ?? 0
      const bv = b[sortKey] ?? 0
      const cmp = sortAsc ? av - bv : bv - av
      if (cmp !== 0) return cmp
      return (b.hex_rating ?? 0) - (a.hex_rating ?? 0)
    })

    return list
  }, [elo, filter, sortKey, sortAsc, bracket])

  const wallOfShame = useMemo(() => {
    let list = [...(elo || [])]
    if (bracket != null) list = list.filter(e => e.bracket === bracket)
    list = list.filter(e => (e.rating ?? 0) <= 0)
    list.sort((a, b) => (a.rating ?? 0) - (b.rating ?? 0))
    return list.slice(0, 10)
  }, [elo, bracket])

  const sortArrow = (key) => {
    if (sortKey !== key) return ''
    return sortAsc ? ' ▲' : ' ▼'
  }

  const handleRowClick = (entry) => {
    if (entry.owner && entry.deck_id) {
      navigate(`/decks/${entry.owner}/${entry.deck_id}`)
    } else if (entry.deck_id) {
      const parts = entry.deck_id.split('/')
      if (parts.length === 2) {
        navigate(`/decks/${parts[0]}/${parts[1]}`)
      } else {
        navigate(`/decks?q=${encodeURIComponent(entry.commander || entry.deck_id)}`)
      }
    } else if (entry.commander) {
      navigate(`/decks?q=${encodeURIComponent(entry.commander)}`)
    }
  }

  // B{n} is a plate and stays English; ALL is a word.
  const bracketLabel = bracket != null ? `B${bracket}` : t('common.all')

  return (
    <>
      <Tape
        left={t('lb.tape_left', { bracket: bracketLabel })}
        mid={t(sorted.length === 1 ? 'deckpower.decks_one' : 'deckpower.decks', { count: sorted.length })}
        right="DOC HX-500"
      />

      <div style={{ padding: 18, display: 'flex', flexDirection: 'column', gap: 14 }}>
        <ContextBox id="leaderboard.intro">{t('lb.intro')}</ContextBox>
        {/* Search + Sort controls */}
        <div className="lb-search-row" style={{ display: 'flex', gap: 10, alignItems: 'center', flexWrap: 'wrap' }}>
          <div className="panel" style={{ padding: 0, flex: 1, minWidth: 200, borderStyle: filter ? 'solid' : 'dashed' }}>
            <input
              type="text"
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              placeholder={t('lb.search_ph')}
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
          <span className="t-xs muted">
            {t(sorted.length === 1 ? 'lb.n_ranked_one' : 'lb.n_ranked', { count: sorted.length })}
          </span>
        </div>

        {/* Bracket filter */}
        <div className="lb-sort-bar" style={{ alignItems: 'center', gap: 8 }}>
          <GlossaryTerm term="bracket" compact>
            <span className="t-xs muted">{t('deck.bracket')}</span>
          </GlossaryTerm>
          {BRACKETS.map(b => (
            <Tag
              key={b.id}
              kind={bracket === b.value ? (BRACKET_KIND[b.value] || 'ok') : undefined}
              solid={bracket === b.value}
              onClick={() => setBracket(b.value)}
              style={{ cursor: 'pointer', fontSize: 9 }}
            >
              {b.labelKey ? t(b.labelKey) : b.label}
            </Tag>
          ))}
        </div>

        {/* Sort controls */}
        <div className="lb-sort-bar">
          {SORT_KEYS.map(sk => (
            <Tag
              key={sk.key}
              kind={sortKey === sk.key ? 'ok' : undefined}
              solid={sortKey === sk.key}
              onClick={() => handleSort(sk.key)}
              style={{ cursor: 'pointer', fontSize: 9 }}
            >
              {sk.labelKey ? t(sk.labelKey) : sk.label}{sortArrow(sk.key)}
            </Tag>
          ))}
        </div>

        {/* Desktop table */}
        <div className="lb-table-wrap">
          <table className="lb-table">
            <thead>
              <tr>
                <th className="lb-th lb-th--rank">#</th>
                <th className="lb-th lb-th--cmdr">{t('common.commander')}</th>
                <th className="lb-th lb-th--owner">{t('deck.owner')}</th>
                {SORT_KEYS.map(sk => (
                  <th
                    key={sk.key}
                    className={`lb-th lb-th--${sk.key} lb-th--sortable${sortKey === sk.key ? ' lb-th--active' : ''}`}
                  >
                    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4 }}>
                      <GlossaryTerm term={sk.term} compact>
                        <span onClick={() => handleSort(sk.key)} style={{ cursor: 'pointer' }}>
                          {sk.labelKey ? t(sk.labelKey) : sk.label}{sortArrow(sk.key)}
                        </span>
                      </GlossaryTerm>
                    </span>
                  </th>
                ))}
                <th className="lb-th lb-th--conf">
                  <GlossaryTerm term="confidence" compact>{t('lb.th_conf')}</GlossaryTerm>
                </th>
                <th className="lb-th lb-th--record">
                  <GlossaryTerm term="record" compact>{t('deck.record')}</GlossaryTerm>
                </th>
              </tr>
            </thead>
            <tbody>
              {sorted.map((entry, i) => (
                <tr
                  key={entry.deck_id || i}
                  className="lb-row"
                  onClick={() => handleRowClick(entry)}
                >
                  <td className="lb-td lb-td--rank">
                    <span className={i < 3 ? 'lb-medal' : ''}>{i + 1}</span>
                  </td>
                  <td className="lb-td lb-td--cmdr" style={{ textDecoration: 'underline', textDecorationColor: 'var(--rule-2)' }}>
                    {entry.commander || '--'}
                    <BandTag band={entry.band} bracket={entry.bracket} />
                    <ShameBadge rating={entry.rating} hexRating={entry.hex_rating} />
                  </td>
                  <td className="lb-td lb-td--owner muted">
                    {entry.owner ? (
                      <a
                        onClick={(e) => { e.stopPropagation(); navigate(`/profile/${entry.owner}`) }}
                        style={{ cursor: 'pointer', color: 'var(--ink-2)', textDecoration: 'none', borderBottom: '1px dotted var(--ink-3)', display: 'inline-flex', alignItems: 'center', gap: 6 }}
                      >
                        {flagFor(entry.owner) && (
                          <span aria-label={t('op.country_aria', { code: countries[entry.owner] })} style={{ fontSize: 13, lineHeight: 1 }}>
                            {flagFor(entry.owner)}
                          </span>
                        )}
                        {entry.owner.toUpperCase()}
                      </a>
                    ) : '--'}
                  </td>
                  <td className="lb-td lb-td--rating">
                    <span style={{ fontWeight: 700 }}>{Math.round(entry.hex_rating || 0)}</span>
                  </td>
                  <td className="lb-td lb-td--mu">
                    <span className="t-xs muted-2">{Math.round(entry.mu || 0)}</span>
                  </td>
                  <td className="lb-td lb-td--games">{(entry.games || 0).toLocaleString()}</td>
                  <td className="lb-td lb-td--winrate">
                    {entry.win_rate != null ? `${entry.win_rate.toFixed(1)}%` : '--'}
                  </td>
                  <td className="lb-td lb-td--delta">
                    <DeltaDisplay delta={entry.delta} />
                  </td>
                  <td className="lb-td lb-td--conf">
                    <ConfidenceDots games={entry.games} />
                  </td>
                  <td className="lb-td lb-td--record">
                    <RecordDisplay wins={entry.wins || 0} losses={entry.losses || 0} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {sorted.length === 0 && (
            <div className="t-md muted" style={{ textAlign: 'center', padding: 36 }}>
              {elo?.length === 0
                ? <>&gt; {t('lb.awaiting_elo')}<span className="blink">_</span></>
                : <>&gt; {t('cmp.no_matches')}</>
              }
            </div>
          )}
        </div>

        {/* Mobile card layout */}
        <div className="lb-cards">
          {sorted.map((entry, i) => (
            <LeaderboardMobileCard
              key={entry.deck_id || i}
              entry={entry}
              index={i}
              flagFor={flagFor}
              countries={countries}
              navigate={navigate}
              onClick={() => handleRowClick(entry)}
            />
          ))}
          {sorted.length === 0 && (
            <div className="t-md muted" style={{ textAlign: 'center', padding: 36 }}>
              {elo?.length === 0
                ? <>&gt; {t('lb.awaiting_elo')}<span className="blink">_</span></>
                : <>&gt; {t('cmp.no_matches')}</>
              }
            </div>
          )}
        </div>

        {wallOfShame.length > 0 && (
          <div style={{ marginTop: 8, display: 'flex', flexDirection: 'column', gap: 8 }}>
            <Tape
              left={t('lb.shame_tape_left', { bracket: bracketLabel, count: wallOfShame.length })}
              mid={t('lb.shame_tape_mid')}
              right="DOC HX-666"
            />
            <div className="panel" style={{ padding: 0 }}>
              {wallOfShame.map((entry, i) => (
                <div
                  key={`shame-${entry.deck_id || i}`}
                  className="lb-row"
                  onClick={() => handleRowClick(entry)}
                  style={{
                    display: 'flex',
                    justifyContent: 'space-between',
                    alignItems: 'center',
                    padding: '6px 10px',
                    gap: 10,
                    borderTop: i === 0 ? 'none' : '1px solid var(--rule-2)',
                    cursor: 'pointer',
                    flexWrap: 'wrap',
                  }}
                >
                  <span style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap', minWidth: 0 }}>
                    <span className="muted-2" style={{ minWidth: 22, fontWeight: 700 }}>#{i + 1}</span>
                    <span style={{ fontWeight: 700, color: 'var(--ink)' }}>
                      {entry.commander || '--'}
                    </span>
                    <ShameBadge rating={entry.rating} hexRating={entry.hex_rating} />
                    {entry.owner ? (
                      <a
                        onClick={(e) => { e.stopPropagation(); navigate(`/profile/${entry.owner}`) }}
                        className="t-xs muted"
                        style={{ cursor: 'pointer', textDecoration: 'none', borderBottom: '1px dotted var(--ink-3)', display: 'inline-flex', alignItems: 'center', gap: 4 }}
                      >
                        {flagFor(entry.owner) && (
                          <span aria-label={t('op.country_aria', { code: countries[entry.owner] })} style={{ fontSize: 12, lineHeight: 1 }}>
                            {flagFor(entry.owner)}
                          </span>
                        )}
                        {entry.owner.toUpperCase()}
                      </a>
                    ) : <span className="t-xs muted">--</span>}
                  </span>
                  <span style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                    <RecordDisplay wins={entry.wins || 0} losses={entry.losses || 0} />
                    <span style={{ color: 'var(--danger)', fontWeight: 700, minWidth: 48, textAlign: 'right' }}>
                      {Math.round(entry.rating || 0)}
                    </span>
                  </span>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </>
  )
}

// LeaderboardMobileCard — extracted so useArtContrast can run per-row.
// data-art-contrast adapts overlays to bright vs dark commander art.
function LeaderboardMobileCard({ entry, index, flagFor, countries, navigate, onClick }) {
  const t = useT()
  const artUrl = cardArtUrl(entry.commander_card || entry.commander)
  const artContrast = useArtContrast(artUrl)
  return (
    <div
      className="panel lb-card"
      data-art-contrast={artContrast || undefined}
      style={{ padding: 0, cursor: 'pointer', ...(artContrast ? { '--art-contrast': artContrast } : null) }}
      onClick={onClick}
    >
      <div className="lb-card-row">
        <div className="lb-card-art">
          {artUrl && (
            <img
              src={artUrl}
              alt={entry.commander || ''}
              onError={e => { e.target.style.display = 'none'; e.target.parentElement.classList.add('hatch') }}
            />
          )}
          <span className={`lb-card-rank${index < 3 ? ' lb-medal' : ''}`}>#{index + 1}</span>
        </div>
        <div className="lb-card-body">
          <div className="panel-hd">
            <span style={{ fontWeight: 700, color: 'var(--ink)', minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
              {entry.commander || '--'}
              <BandTag band={entry.band} bracket={entry.bracket} />
              <ShameBadge rating={entry.rating} hexRating={entry.hex_rating} />
            </span>
            <span style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-end', lineHeight: 1.1 }}>
              <span className="t-xs" style={{ fontWeight: 700 }}>
                HexELO {Math.round(entry.hex_rating || 0)}
              </span>
              <span className="t-xs muted-2">
                TS μ {Math.round(entry.mu || 0)}
              </span>
            </span>
          </div>
          <div style={{ padding: '8px 10px', display: 'flex', flexDirection: 'column', gap: 6 }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              {entry.owner ? (
                <a
                  onClick={(e) => { e.stopPropagation(); navigate(`/profile/${entry.owner}`) }}
                  className="t-xs muted"
                  style={{ cursor: 'pointer', textDecoration: 'none', borderBottom: '1px dotted var(--ink-3)', display: 'inline-flex', alignItems: 'center', gap: 4 }}
                >
                  {flagFor(entry.owner) && (
                    <span aria-label={t('op.country_aria', { code: countries[entry.owner] })} style={{ fontSize: 12, lineHeight: 1 }}>
                      {flagFor(entry.owner)}
                    </span>
                  )}
                  {entry.owner.toUpperCase()}
                </a>
              ) : <span className="t-xs muted">--</span>}
              <ConfidenceDots games={entry.games} showLabel />
            </div>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <RecordDisplay wins={entry.wins || 0} losses={entry.losses || 0} />
              <span className="t-xs">
                {entry.win_rate != null ? `${entry.win_rate.toFixed(1)}%` : '--'}
              </span>
            </div>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <span className="t-xs muted">
                {t((entry.games || 0) === 1 ? 'shelf.games_one' : 'shelf.games', { count: (entry.games || 0).toLocaleString() })}
              </span>
              <DeltaDisplay delta={entry.delta} />
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
