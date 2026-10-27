import { useEffect, useMemo, useState } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { Panel, KV, Tag, Tape, Btn, Bar } from '../components/chrome'
import ManaCost from '../components/ManaCost'
import { API_BASE, cardArtUrl, cardImageUrl } from '../services/api'
import { useArtContrast } from '../hooks/useArtContrast'
import { useLiveSocket } from '../hooks/useLiveSocket'
import { useT } from '../i18n'

// CardPage — /cards/:cardName
//
// Two-tier data flow:
//
//   1. Try /api/cards/{name} first. The local oracle corpus knows the
//      card's mana cost, type line, oracle text, CMC, set, and — via a
//      DecksDir scan — every indexed deck that contains it (with its
//      Freya role tag when strategy.json exists). One round trip, no
//      per-deck fan-out, no Scryfall dependency for the page to render.
//
//   2. If the local API returns 404 (card not in the loaded oracle
//      bulk), fall back to Scryfall /cards/named?exact=... for type/
//      oracle/cost. The "USED IN" panel then comes back empty since
//      the card isn't indexed locally.
//
// Card art always uses the existing /api/card-art/{name} proxy
// (cardArtUrl()), which redirects to Scryfall's image_uris.art_crop
// server-side. That keeps a single Scryfall dependency and avoids
// CORS surprises.

const ROLE_KIND = {
  // Map Freya role identifiers to Tag kinds for color.
  removal: 'bad',
  counterspell: 'bad',
  ramp: 'ok',
  draw: 'info',
  tutor: 'warn',
  finisher: 'warn',
  combo_piece: 'warn',
  protection: 'ok',
  recursion: 'info',
  threat: 'bad',
  utility: null,
  value: 'ok',
}

// Freya's role identifiers as they arrive on the wire, mapped to catalog
// PATHS rather than words: this table is read at render from module scope,
// so a word here would be fixed at load. An unknown role falls through to
// the old prettifier rather than rendering blank.
const ROLE_KEYS = {
  removal: 'crd.role_removal',
  counterspell: 'crd.role_counterspell',
  ramp: 'crd.role_ramp',
  draw: 'crd.role_draw',
  tutor: 'crd.role_tutor',
  finisher: 'crd.role_finisher',
  combo_piece: 'crd.role_combo_piece',
  protection: 'crd.role_protection',
  recursion: 'crd.role_recursion',
  threat: 'crd.role_threat',
  utility: 'crd.role_utility',
  value: 'crd.role_value',
}

// Scryfall's four commander-legality values, same shape as ROLE_KEYS.
const LEGALITY_KEYS = {
  legal: 'crd.leg_legal',
  not_legal: 'crd.leg_not_legal',
  banned: 'crd.leg_banned',
  restricted: 'crd.leg_restricted',
}

function readableRole(r, t) {
  const key = ROLE_KEYS[r]
  return key ? t(key) : (r || '').replace(/_/g, ' ').toUpperCase()
}

// Module-level formatter: takes the translator as a PARAMETER so the (FOIL)
// suffix is worded by the caller's live locale rather than frozen at load.
function priceFromScryfall(s, t) {
  const usd = s?.prices?.usd
  if (usd != null && usd !== '') return `$${usd}`
  const usdFoil = s?.prices?.usd_foil
  if (usdFoil != null && usdFoil !== '') return t('crd.foil_price', { price: `$${usdFoil}` })
  return '—'
}

export default function CardPage() {
  const { cardName: rawName } = useParams()
  const navigate = useNavigate()
  const t = useT()
  const cardName = useMemo(() => decodeURIComponent(rawName || ''), [rawName])

  // Local API response (CardDetail). null = no data yet or 404.
  const [detail, setDetail] = useState(null)
  // Scryfall response — only fetched when the local API 404s.
  const [scry, setScry] = useState(null)
  // /api/cards/:name/stats — null until resolved; {games_played:0,...} on cold cards.
  const [stats, setStats] = useState(null)
  // /api/card-stats/card/:name — extended analytics (inclusion rate, commanders, brackets).
  const [perfStats, setPerfStats] = useState(null)
  // Loading / error state covers whichever path is in flight.
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)
  const [source, setSource] = useState(null) // 'local' | 'scryfall' | null
  const { elo: liveElo } = useLiveSocket()

  useEffect(() => {
    if (!cardName) return
    let cancelled = false
    setLoading(true)
    setError(false)
    setDetail(null)
    setScry(null)
    setStats(null)
    setPerfStats(null)
    setSource(null)

    // Engine win-rate stats — independent of card-data resolution. Fired
    // in parallel; failures are non-fatal (cold endpoint or new server).
    fetch(`${API_BASE}/api/cards/${encodeURIComponent(cardName)}/stats`)
      .then(r => r.ok ? r.json() : null)
      .then(data => { if (!cancelled && data) setStats(data) })
      .catch(() => {})

    // Extended card performance stats — inclusion rate, commander breakdown,
    // bracket distribution. Non-fatal on failure.
    fetch(`${API_BASE}/api/card-stats/card/${encodeURIComponent(cardName)}`)
      .then(r => r.ok ? r.json() : null)
      .then(data => { if (!cancelled && data) setPerfStats(data) })
      .catch(() => {})

    const tryLocal = fetch(`${API_BASE}/api/cards/${encodeURIComponent(cardName)}`)
      .then(async r => {
        if (r.status === 404) return { kind: '404' }
        if (!r.ok) throw new Error(`local ${r.status}`)
        return { kind: 'ok', data: await r.json() }
      })

    const tryScryfall = () =>
      fetch(`https://api.scryfall.com/cards/named?exact=${encodeURIComponent(cardName)}`)
        .then(r => r.ok ? r.json() : Promise.reject(new Error(`scryfall ${r.status}`)))

    tryLocal
      .then(res => {
        if (cancelled) return
        if (res.kind === 'ok') {
          setDetail(res.data)
          setSource('local')
          setLoading(false)
          return
        }
        // 404 from local API → fall back to Scryfall entirely.
        return tryScryfall().then(data => {
          if (cancelled) return
          setScry(data)
          setSource('scryfall')
          setLoading(false)
        })
      })
      .catch(() => {
        if (!cancelled) {
          setError(true)
          setLoading(false)
        }
      })

    return () => { cancelled = true }
  }, [cardName])

  useEffect(() => {
    if (!cardName) return
    document.title = `${cardName} — HEXDEK`
  }, [cardName])

  if (!cardName) {
    return (
      <>
        <Tape left={t('crd.tape_card')} mid={t('crd.tape_missing')} right="DOC HX-700" />
        <div style={{ padding: 36, textAlign: 'center' }}>
          <div className="t-md muted">{t('crd.no_name')}</div>
        </div>
      </>
    )
  }

  // Unified view model. The local CardDetail is preferred; Scryfall's
  // payload is the fallback shape (mana_cost / type_line / oracle_text /
  // set / cmc all live at the top level too, with card_faces for DFCs).
  const upperName = cardName.toUpperCase()

  const localDecks = detail?.decks_using || []
  // Freya roles: union of every non-empty role across the decks_using
  // list. The backend resolves card_roles from each deck's strategy.json,
  // so this needs no per-deck fan-out from the frontend.
  const roles = useMemo(() => {
    const set = new Set()
    for (const d of localDecks) {
      if (d?.role) set.add(d.role)
    }
    return [...set]
  }, [localDecks])

  const manaCost = detail?.mana_cost
    || scry?.mana_cost
    || scry?.card_faces?.[0]?.mana_cost
    || ''
  const typeLine = detail?.type_line
    || scry?.type_line
    || scry?.card_faces?.[0]?.type_line
    || '—'
  const oracle = detail?.oracle_text
    || scry?.oracle_text
    || scry?.card_faces?.[0]?.oracle_text
    || ''
  const setCode = (detail?.set || scry?.set || '').toUpperCase()
  const setName = scry?.set_name || setCode || '—'
  const cmc = detail?.cmc != null ? detail.cmc : (scry?.cmc != null ? scry.cmc : null)
  const price = priceFromScryfall(scry, t) // local API doesn't carry price; only present on Scryfall fallback.

  // Cross-reference local decks (containing the card) with live ELO data
  // to derive per-deck win rates and the WITH/WITHOUT comparison. Done as
  // a memoized join on deck_id (or owner/id) since liveElo is a flat list.
  const eloByKey = useMemo(() => {
    const m = {}
    for (const e of (liveElo || [])) {
      const dk = e.deck_id || (e.owner && e.id ? `${e.owner}/${e.id}` : null)
      if (dk) m[dk] = e
      // Also key by bare id as a fallback (older payloads).
      if (e.id) m[e.id] = m[e.id] || e
    }
    return m
  }, [liveElo])

  const decksWithElo = useMemo(() => {
    if (!localDecks.length) return []
    const out = []
    for (const d of localDecks) {
      const k = `${d.owner}/${d.id}`
      const e = eloByKey[k] || eloByKey[d.id]
      if (e && (e.games || 0) > 0) {
        out.push({ ...d, rating: e.rating, wins: e.wins || 0, losses: e.losses || 0, games: e.games || 0, win_rate: e.win_rate ?? (e.wins / e.games) * 100 })
      }
    }
    out.sort((a, b) => (b.win_rate || 0) - (a.win_rate || 0))
    return out
  }, [localDecks, eloByKey])

  // Aggregate WR across decks WITH this card vs the rest of the elo pool.
  const withVsWithout = useMemo(() => {
    if (!liveElo || liveElo.length === 0) return null
    const hasCard = new Set(localDecks.map(d => `${d.owner}/${d.id}`))
    let withW = 0, withL = 0, woW = 0, woL = 0
    for (const e of liveElo) {
      const dk = e.deck_id || (e.owner && e.id ? `${e.owner}/${e.id}` : '')
      const inSet = hasCard.has(dk) || hasCard.has(e.id)
      if (inSet) { withW += e.wins || 0; withL += e.losses || 0 }
      else       { woW   += e.wins || 0; woL   += e.losses || 0 }
    }
    const withGames = withW + withL
    const woGames = woW + woL
    if (withGames === 0 && woGames === 0) return null
    return {
      withWR: withGames > 0 ? (withW / withGames) * 100 : null,
      withGames,
      woWR:   woGames   > 0 ? (woW   / woGames)   * 100 : null,
      woGames,
    }
  }, [liveElo, localDecks])

  const heroArt = cardArtUrl(cardName)
  const heroImage = cardImageUrl(cardName)
  const heroContrast = useArtContrast(heroArt)
  // Worded here in the render body rather than parked in state by the
  // fetch: a label stored at fetch time keeps the language it was fetched
  // in. SCRYFALL is the site's own name and stays as it is.
  const sourceLabel = source === 'local'
    ? t('crd.src_local')
    : source === 'scryfall'
      ? 'SCRYFALL'
      : error ? t('crd.src_unavailable') : t('crd.src_loading')

  return (
    <div className="card-page">
      {heroArt && (
        <img
          className="art-ambience"
          src={heroArt}
          alt=""
          aria-hidden="true"
        />
      )}

      <Tape
        left={t('crd.tape_left', { name: upperName })}
        mid={loading ? t('crd.src_loading') : (error ? t('crd.src_unavailable') : sourceLabel)}
        right="DOC HX-700"
      />

      {/* Card hero — full card render + info side by side */}
      <div
        className="card-page-hero"
        data-art-contrast={heroContrast || undefined}
        style={{
          backgroundImage: heroArt ? `url(${heroArt})` : undefined,
          ...(heroContrast ? { '--art-contrast': heroContrast } : null),
        }}
      >
        <div className="card-page-hero-scrim" />
        <div className="card-page-hero-corner card-page-hero-corner--tl">04.HERO / / {setCode || t('crd.unknown')}</div>
        <div className="card-page-hero-corner card-page-hero-corner--tr">{upperName}</div>
        <div className="card-page-hero-body">
          {heroImage && (
            <div className="card-page-hero-card">
              <img
                src={heroImage}
                alt={cardName}
                className="card-page-hero-card-img"
                onError={(e) => { e.target.style.display = 'none' }}
              />
            </div>
          )}
          <div style={{ flex: 1, minWidth: 0 }}>
            <div className="card-page-hero-meta">
              {typeLine !== '—' && <Tag solid>{typeLine.toUpperCase()}</Tag>}
              {manaCost && (
                <Tag>
                  <ManaCost cost={manaCost} size={14} />
                </Tag>
              )}
              {price !== '—' && <Tag kind="ok">{price}</Tag>}
            </div>
            <h1 className="card-page-hero-title">{upperName}</h1>
            {setName !== '—' && (
              <div className="card-page-hero-sub">{setName.toUpperCase()}{setCode ? ` · ${setCode}` : ''}</div>
            )}
          </div>
        </div>
      </div>

      <div className="card-page-layout">
        <div className="card-page-sidebar">
          <Panel code="07.A" title={t('crd.specs')} solid>
            <KV rows={[
              [t('crd.kv_name'), upperName],
              [t('crd.kv_type'), typeLine],
              [t('crd.kv_mana'), manaCost ? <ManaCost cost={manaCost} size={16} /> : '—'],
              ['CMC', cmc != null ? `${cmc}` : '—'],
              [t('crd.kv_set'), setCode || '—'],
              ...(source === 'scryfall' ? [[t('crd.kv_price'), price]] : []),
              ...(scry?.legalities?.commander ? [[
                t('crd.kv_legal_commander'),
                <span style={{ color: scry.legalities.commander === 'legal' ? 'var(--ok)' : 'var(--danger)', fontWeight: 700 }}>
                  {LEGALITY_KEYS[scry.legalities.commander]
                    ? t(LEGALITY_KEYS[scry.legalities.commander])
                    : scry.legalities.commander.toUpperCase()}
                </span>,
              ]] : []),
              [t('crd.kv_source'), sourceLabel],
            ]} />
            {scry?.scryfall_uri && (
              <>
                <div className="hr" style={{ margin: '10px 0' }} />
                <a href={scry.scryfall_uri} target="_blank" rel="noopener noreferrer" style={{ textDecoration: 'none' }}>
                  <Btn ghost arrow="↗">{t('crd.view_scryfall')}</Btn>
                </a>
              </>
            )}
          </Panel>

          {roles.length > 0 && (
            <Panel code="07.R" title={t('crd.roles_title', { count: roles.length })}>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4 }}>
                {roles.map(r => (
                  <Tag key={r} kind={ROLE_KIND[r]} solid>{readableRole(r, t)}</Tag>
                ))}
              </div>
              <div className="t-xs muted" style={{ marginTop: 6 }}>
                {/* The filename rides in as a value so it is written once
                    here rather than 25 times across the catalogs. */}
                {t('crd.roles_note', { file: 'STRATEGY.JSON' })}
              </div>
            </Panel>
          )}
        </div>

        <div className="card-page-main">
          {/* Oracle text */}
          <Panel code="07.B" title={t('crd.oracle_title')} right={
            <span className="t-xs muted">{loading ? t('crd.src_loading') : (error ? t('crd.offline') : sourceLabel)}</span>
          }>
            {loading ? (
              <div className="t-md muted" style={{ padding: '14px 0', textAlign: 'center' }}>
                {t('crd.fetching_card')}<span className="blink">_</span>
              </div>
            ) : error ? (
              <div className="t-md muted" style={{ padding: '14px 0', textAlign: 'center' }}>
                {t('crd.card_unavailable')}
              </div>
            ) : (
              <div style={{ whiteSpace: 'pre-wrap', lineHeight: 1.55, fontSize: 12 }}>
                {oracle || <span className="muted">{t('crd.no_oracle')}</span>}
              </div>
            )}
          </Panel>

          {/* Engine win-rate stats — sourced from /api/cards/:name/stats.
              The endpoint returns games_played=0 for cards with no
              recorded plays; we render an honest empty state in that
              case instead of a misleading 0% bar. */}
          <Panel
            code="07.W"
            title={t('crd.engine_title')}
            right={stats?.games_played > 0 ? <Tag solid kind={stats.win_rate >= 0.5 ? 'ok' : null}>{(stats.win_rate * 100).toFixed(1)}% WR</Tag> : null}
          >
            {stats == null ? (
              <div className="t-md muted" style={{ padding: '14px 0', textAlign: 'center' }}>
                {t('crd.fetching_stats')}<span className="blink">_</span>
              </div>
            ) : (stats.games_played || 0) === 0 ? (
              <div className="t-md muted" style={{ padding: '14px 0', textAlign: 'center' }}>
                {t('crd.no_game_data')}
              </div>
            ) : (
              <>
                <div style={{ display: 'grid', gridTemplateColumns: '120px 1fr 56px', alignItems: 'center', gap: 8 }}>
                  <span className="t-xs muted">{t('crd.win_rate')}</span>
                  <Bar value={stats.win_rate * 100} lg />
                  <span className="t-md text-right" style={{ fontWeight: 700, fontVariantNumeric: 'tabular-nums' }}>
                    {(stats.win_rate * 100).toFixed(1)}%
                  </span>
                </div>
                <div className="hr" style={{ margin: '12px 0' }} />
                <KV rows={[
                  [t('crd.kv_games_played'), `${stats.games_played.toLocaleString()}`],
                  [t('crd.kv_wins'), <span style={{ color: 'var(--ok)' }}>{(stats.wins || 0).toLocaleString()}</span>],
                  [t('crd.kv_losses'), <span style={{ color: 'var(--danger)' }}>{(stats.losses || 0).toLocaleString()}</span>],
                  ...(stats.avg_turn_played > 0 ? [[t('crd.kv_avg_turn'), stats.avg_turn_played.toFixed(1)]] : []),
                ]} />

                {withVsWithout && (withVsWithout.withWR != null || withVsWithout.woWR != null) && (
                  <>
                    <div className="hr" style={{ margin: '12px 0' }} />
                    <div className="t-xs muted" style={{ marginBottom: 6 }}>{t('crd.wr_comparison')}</div>
                    {withVsWithout.withWR != null && (
                      <div style={{ display: 'grid', gridTemplateColumns: '160px 1fr 90px', alignItems: 'center', gap: 8, marginBottom: 4 }}>
                        <span className="t-xs">{t('crd.decks_with')}</span>
                        <Bar value={withVsWithout.withWR} />
                        <span className="t-xs text-right" style={{ fontVariantNumeric: 'tabular-nums', color: 'var(--ok)' }}>
                          {withVsWithout.withWR.toFixed(1)}% · {t('crd.games_suffix', { count: withVsWithout.withGames.toLocaleString() })}
                        </span>
                      </div>
                    )}
                    {withVsWithout.woWR != null && (
                      <div style={{ display: 'grid', gridTemplateColumns: '160px 1fr 90px', alignItems: 'center', gap: 8 }}>
                        <span className="t-xs">{t('crd.decks_without')}</span>
                        <Bar value={withVsWithout.woWR} />
                        <span className="t-xs text-right" style={{ fontVariantNumeric: 'tabular-nums' }}>
                          {withVsWithout.woWR.toFixed(1)}% · {t('crd.games_suffix', { count: withVsWithout.woGames.toLocaleString() })}
                        </span>
                      </div>
                    )}
                    {withVsWithout.withWR != null && withVsWithout.woWR != null && (
                      <div className="t-xs muted" style={{ marginTop: 6 }}>
                        {t('crd.delta')}{' '}
                        <span style={{ color: withVsWithout.withWR > withVsWithout.woWR ? 'var(--ok)' : 'var(--danger)', fontWeight: 700 }}>
                          {t('crd.pp_suffix', { value: `${withVsWithout.withWR > withVsWithout.woWR ? '+' : ''}${(withVsWithout.withWR - withVsWithout.woWR).toFixed(1)}` })}
                        </span>
                      </div>
                    )}
                  </>
                )}
              </>
            )}
          </Panel>

          {/* Performance panel — inclusion rate, top commanders, bracket
              distribution. Sourced from /api/card-stats/card/:name. */}
          {perfStats && (perfStats.games_played > 0 || perfStats.decks_using > 0) && (
            <Panel
              code="07.P"
              title={t('crd.perf_title')}
              right={perfStats.decks_using > 0 ? <Tag solid>{t('crd.n_decks', { count: perfStats.decks_using })}</Tag> : null}
            >
              {/* Inclusion rate */}
              {perfStats.total_decks > 0 && (
                <>
                  <div style={{ display: 'grid', gridTemplateColumns: '120px 1fr 80px', alignItems: 'center', gap: 8 }}>
                    <span className="t-xs muted">{t('crd.inclusion')}</span>
                    <Bar value={perfStats.inclusion_rate * 100} />
                    <span className="t-xs text-right" style={{ fontVariantNumeric: 'tabular-nums' }}>
                      {perfStats.decks_using} / {perfStats.total_decks}
                    </span>
                  </div>
                  <div className="t-xs muted" style={{ marginTop: 4 }}>
                    {t('crd.appears_in', { pct: (perfStats.inclusion_rate * 100).toFixed(1) })}
                  </div>
                  <div className="hr" style={{ margin: '12px 0' }} />
                </>
              )}

              {/* Top commanders using this card */}
              {perfStats.top_commanders && perfStats.top_commanders.length > 0 && (
                <>
                  <div className="t-xs muted" style={{ marginBottom: 6 }}>{t('crd.top_commanders')}</div>
                  <div style={{
                    display: 'grid', gridTemplateColumns: '24px 1fr 60px 60px',
                    gap: 4, padding: '4px 0', borderBottom: '1px solid var(--rule-2)', marginBottom: 4,
                  }}>
                    <span className="t-xs muted">#</span>
                    <span className="t-xs muted">{t('crd.col_commander')}</span>
                    <span className="t-xs muted text-right">{t('crd.col_games')}</span>
                    <span className="t-xs muted text-right">WR%</span>
                  </div>
                  {perfStats.top_commanders.slice(0, 5).map((c, i) => (
                    <div
                      key={c.commander}
                      style={{
                        display: 'grid', gridTemplateColumns: '24px 1fr 60px 60px',
                        gap: 4, padding: '4px 0',
                        borderBottom: i < Math.min(perfStats.top_commanders.length, 5) - 1 ? '1px dashed var(--rule-2)' : 'none',
                        alignItems: 'center',
                      }}
                    >
                      <span className="t-xs muted-2">{i + 1}</span>
                      <span className="t-xs" style={{ fontWeight: i < 3 ? 700 : 400, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        {(c.commander || '').toUpperCase()}
                      </span>
                      <span className="t-xs text-right" style={{ fontVariantNumeric: 'tabular-nums' }}>{c.games}</span>
                      <span className="t-xs text-right" style={{ fontVariantNumeric: 'tabular-nums', fontWeight: 700, color: c.win_rate >= 0.5 ? 'var(--ok)' : 'var(--ink-2)' }}>
                        {(c.win_rate * 100).toFixed(1)}%
                      </span>
                    </div>
                  ))}
                  <div className="hr" style={{ margin: '12px 0' }} />
                </>
              )}

              {/* Bracket distribution */}
              {perfStats.bracket_distribution && perfStats.bracket_distribution.length > 0 && (
                <>
                  <div className="t-xs muted" style={{ marginBottom: 6 }}>{t('crd.bracket_dist')}</div>
                  <div style={{ display: 'flex', gap: 8, alignItems: 'flex-end', height: 60, padding: '0 4px' }}>
                    {perfStats.bracket_distribution.map(b => {
                      const maxCount = Math.max(...perfStats.bracket_distribution.map(x => x.count))
                      const pct = maxCount > 0 ? (b.count / maxCount) * 100 : 0
                      return (
                        <div key={b.bracket} style={{ flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 2 }}>
                          <div style={{
                            width: '100%', minHeight: 4, height: `${pct}%`,
                            background: 'var(--accent)', borderRadius: 2, transition: 'height 0.3s',
                          }} />
                          <span className="t-xs" style={{ fontWeight: 700 }}>B{b.bracket}</span>
                          <span className="t-xs muted-2">{b.count}</span>
                        </div>
                      )
                    })}
                  </div>
                </>
              )}
            </Panel>
          )}

          {/* Top decks running this card — joins detail.decks_using with
              live ELO data and sorts by win rate. Empty when no overlap
              between the deck index and the showmatch ELO pool. */}
          {decksWithElo.length > 0 && (
            <Panel
              code="07.T"
              title={t('crd.top_decks_title', { count: decksWithElo.length })}
              right={<Tag solid>{decksWithElo.length}</Tag>}
            >
              <div style={{
                display: 'grid', gridTemplateColumns: '24px 1fr 70px 70px 60px',
                gap: 6, padding: '4px 0', borderBottom: '1px solid var(--rule-2)', marginBottom: 4,
              }}>
                <span className="t-xs muted">#</span>
                <span className="t-xs muted">{t('crd.col_deck')}</span>
                <span className="t-xs muted text-right">ELO</span>
                <span className="t-xs muted text-right">{t('crd.col_rec')}</span>
                <span className="t-xs muted text-right">WR%</span>
              </div>
              {decksWithElo.slice(0, 5).map((d, i) => (
                <Link
                  key={`${d.owner}/${d.id}`}
                  to={`/decks/${d.owner}/${d.id}`}
                  style={{
                    display: 'grid', gridTemplateColumns: '24px 1fr 70px 70px 60px',
                    gap: 6, padding: '6px 0',
                    borderBottom: i < Math.min(decksWithElo.length, 5) - 1 ? '1px dashed var(--rule-2)' : 'none',
                    alignItems: 'center', textDecoration: 'none', color: 'var(--ink)',
                  }}
                >
                  <span className="t-xs muted-2">{i + 1}</span>
                  <span className="t-xs" style={{ fontWeight: i < 3 ? 700 : 400, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {(d.commander || d.name || d.id || '').toUpperCase()}
                    <span className="muted-2" style={{ marginLeft: 4 }}>· {(d.owner || '').toUpperCase()}</span>
                  </span>
                  <span className="t-xs text-right" style={{ fontVariantNumeric: 'tabular-nums' }}>{Math.round(d.rating)}</span>
                  <span className="t-xs text-right" style={{ fontVariantNumeric: 'tabular-nums' }}>
                    <span style={{ color: 'var(--ok)' }}>{d.wins}</span>
                    <span className="muted-2">-</span>
                    <span style={{ color: 'var(--danger)' }}>{d.losses}</span>
                  </span>
                  <span className="t-xs text-right" style={{ fontVariantNumeric: 'tabular-nums', fontWeight: 700, color: d.win_rate >= 30 ? 'var(--ok)' : 'var(--ink-2)' }}>
                    {(d.win_rate || 0).toFixed(1)}%
                  </span>
                </Link>
              ))}
              {decksWithElo.length > 5 && (
                <div className="t-xs muted" style={{ textAlign: 'center', marginTop: 8 }}>
                  {t('crd.more_see_used_in', { count: decksWithElo.length - 5 })}
                </div>
              )}
            </Panel>
          )}

          {/* Used in — decks containing this card. Sourced from
              detail.decks_using when the local API responded; the
              Scryfall fallback path leaves this panel empty since the
              card isn't in our oracle corpus. */}
          <Panel
            code="07.C"
            title={t('crd.used_in_title', { count: loading ? '…' : localDecks.length })}
            right={<Tag solid kind={localDecks.length > 0 ? 'ok' : null}>{localDecks.length}</Tag>}
          >
            {loading ? (
              <div className="t-md muted" style={{ padding: '14px 0', textAlign: 'center' }}>
                {t('crd.searching_index')}<span className="blink">_</span>
              </div>
            ) : localDecks.length === 0 ? (
              <div className="t-md muted" style={{ padding: '14px 0', textAlign: 'center' }}>
                {t('crd.no_indexed_decks')}
              </div>
            ) : (
              <div style={{
                display: 'grid',
                gridTemplateColumns: 'repeat(auto-fill, minmax(150px, 1fr))',
                gap: 8,
              }}>
                {localDecks.slice(0, 60).map(d => {
                  const cmdrName = d.commander || ''
                  const art = cardArtUrl(cmdrName)
                  return (
                    <Link
                      key={`${d.owner}/${d.id}`}
                      to={`/decks/${d.owner}/${d.id}`}
                      className="card-page-deck-tile"
                      title={`${d.commander || d.name || d.id} (${d.owner})${d.role ? ' · ' + readableRole(d.role, t) : ''}`}
                    >
                      <div
                        className="card-page-deck-art"
                        style={art ? { backgroundImage: `url(${art})` } : undefined}
                      />
                      <div className="card-page-deck-meta">
                        <div className="card-page-deck-name">
                          {(d.commander || d.name || d.id || '').toUpperCase()}
                        </div>
                        <div className="card-page-deck-owner">
                          {(d.owner || '').toUpperCase()}
                          {d.role && (
                            <span style={{ marginLeft: 4, color: 'var(--ink-3)' }}>· {readableRole(d.role, t)}</span>
                          )}
                        </div>
                      </div>
                    </Link>
                  )
                })}
              </div>
            )}
            {localDecks.length > 60 && (
              <div className="t-xs muted" style={{ textAlign: 'center', marginTop: 8 }}>
                {t('crd.showing_n', { shown: 60, total: localDecks.length })}
              </div>
            )}
          </Panel>
        </div>
      </div>
    </div>
  )
}
