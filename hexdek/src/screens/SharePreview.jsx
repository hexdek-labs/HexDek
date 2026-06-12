import { useEffect, useMemo, useState } from 'react'
import { useParams, Link, useSearchParams } from 'react-router-dom'
import { isAnonymousShare } from '../lib/deckShare'
import { Panel, Tag, Tape } from '../components/chrome'
import CardLink from '../components/CardLink'
import { api, cardArtUrl, cardImageUrl } from '../services/api'
import { useArtContrast } from '../hooks/useArtContrast'
import { useLiveSocket } from '../hooks/useLiveSocket'

// Compact thumbnail used in HOT CARDS — mirrors DeckArchive's CardThumb
// without the cmc/score chips since the share preview only renders the
// curated win-rate top 5.
const HotCardThumb = ({ name }) => {
  const imgUrl = cardArtUrl(name)
  return (
    <CardLink name={name} underline={false} style={{ display: 'block' }}>
      <div className="panel" style={{ padding: 0 }}>
        <div style={{ aspectRatio: '5/7', borderBottom: '1px solid var(--rule-2)', position: 'relative', overflow: 'hidden' }}>
          <img
            src={imgUrl}
            alt={name}
            style={{ width: '100%', height: '100%', objectFit: 'cover', filter: 'saturate(0.6) contrast(1.1)' }}
            onError={e => { e.target.style.display = 'none'; e.target.parentElement.classList.add('hatch') }}
          />
        </div>
        <div style={{ padding: '5px 7px' }}>
          <div style={{ fontSize: 9, fontWeight: 700, letterSpacing: '0.04em', textTransform: 'uppercase', lineHeight: 1.2, minHeight: 24 }}>{name}</div>
        </div>
      </div>
    </CardLink>
  )
}

// Color identity → page-wash gradient + accent. Lifted from DeckArchive's
// pageTheme derivation so the share preview matches the live deck page tone.
function deriveTheme(colorIdentity) {
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
}

// SharePreview — link-preview-friendly version of the deck page. No nav,
// no sidebar, no edit/share/compare affordances. Just the hero, vital
// signs, hot cards, and similar decks. Rendered outside <AppShell /> so
// the surrounding chrome doesn't clip the preview frame on social embeds.
export default function SharePreview() {
  const { owner, id } = useParams()
  // ?anon=1 — strips owner-revealing UI (tape label, corner badge, doc
  // title, full-deck-page link). The deck data itself still fetches by
  // owner/id, but the rendered surface and the meta tags omit the owner.
  const [searchParams] = useSearchParams()
  const anonymous = isAnonymousShare(searchParams)
  const [deck, setDeck] = useState(null)
  const [analysis, setAnalysis] = useState(null)
  const [commanderCardStats, setCommanderCardStats] = useState(null)
  const [similarDecks, setSimilarDecks] = useState(null)
  const [loading, setLoading] = useState(true)
  const { elo } = useLiveSocket()

  useEffect(() => {
    if (!owner || !id) { setLoading(false); return }
    Promise.allSettled([
      api.getDeck(`${owner}/${id}`),
      api.getDeckAnalysis(`${owner}/${id}`),
    ]).then(([deckRes, analysisRes]) => {
      if (deckRes.status === 'fulfilled') setDeck(deckRes.value)
      if (analysisRes.status === 'fulfilled' && analysisRes.value?.status !== 'analyzing') {
        setAnalysis(analysisRes.value)
      }
      setLoading(false)
    })
  }, [owner, id])

  useEffect(() => {
    if (!owner || !id) { setSimilarDecks([]); return }
    let cancelled = false
    api.getSimilarDecks(`${owner}/${id}`, 5)
      .then(rows => { if (!cancelled) setSimilarDecks(Array.isArray(rows) ? rows : []) })
      .catch(() => { if (!cancelled) setSimilarDecks([]) })
    return () => { cancelled = true }
  }, [owner, id])

  useEffect(() => {
    const cmdr = deck?.commander_card
    if (!cmdr) { setCommanderCardStats(null); return }
    let cancelled = false
    api.getCardStatsByCommander(cmdr)
      .then(res => {
        if (cancelled) return
        const rows = Array.isArray(res) ? res : (res?.cards || [])
        setCommanderCardStats(rows)
      })
      .catch(() => { if (!cancelled) setCommanderCardStats([]) })
    return () => { cancelled = true }
  }, [deck?.commander_card])

  // ELO index + this deck's row. Recomputing on every render burned
  // measurable CPU during the 4-stage hydration (deck → analysis →
  // similar decks → commander card stats each fire a setState). Cache
  // the index against the live ELO array reference.
  const eloByDeckId = useMemo(() => {
    const idx = {}
    for (const e of elo) {
      if (e.deck_id) idx[e.deck_id] = e
    }
    return idx
  }, [elo])
  const deckKey = owner && id ? `${owner}/${id}` : null
  const deckElo = useMemo(
    () => eloByDeckId[deckKey] || eloByDeckId[id] || null,
    [eloByDeckId, deckKey, id]
  )

  const slugToTitle = (slug, ownerSlug) => {
    if (!slug) return 'DECK'
    let s = String(slug)
    s = s.replace(/_[A-Za-z0-9]{8,}$/, '')
    if (ownerSlug) s = s.replace(new RegExp(`_${ownerSlug.toLowerCase()}$`, 'i'), '')
    s = s.replace(/_b[0-5]$/i, '')
    return s.replace(/_/g, ' ').toUpperCase() || 'DECK'
  }
  const deckName = deck?.custom_name || deck?.commander || slugToTitle(id, owner)
  const cards = deck?.cards || []
  const wbs = analysis?.bracket || deck?.bracket || null
  const wbsLabel = analysis?.bracket_label || ''
  const measuredBracket = analysis?.measured_bracket || null
  const bracketDiverges = measuredBracket != null && wbs != null && Number(measuredBracket) !== Number(wbs)
  const archetype = analysis?.archetype?.toUpperCase() || 'UNKNOWN'

  // colorIdentity walks every card scanning mana_cost regexes when the
  // analysis payload doesn't carry color_identity — on a 100-card deck
  // that's ~100 regex executions per render. Memoize so it runs once per
  // deck/analysis change instead of once per render.
  const colorIdentity = useMemo(() => {
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
  }, [analysis?.color_identity, deck?.commander_card, cards])

  const pageTheme = useMemo(() => deriveTheme(colorIdentity), [colorIdentity])
  const cmdrCardName = deck?.commander_card || cards.find(c => c.name?.startsWith('COMMANDER:'))?.name?.replace('COMMANDER:', '').trim()
  const cmdrImageUrl = cmdrCardName ? cardArtUrl(cmdrCardName) : null
  const cmdrFullUrl = cmdrCardName ? cardImageUrl(cmdrCardName) : null
  const cmdrContrast = useArtContrast(cmdrImageUrl)

  useEffect(() => {
    if (!deckName) return
    const ownerLabel = (anonymous || !owner) ? '' : ` · ${owner.toUpperCase()}`
    document.title = `${deckName}${ownerLabel} — HEXDEK`
  }, [deckName, owner, anonymous])

  // Mirror the backend OG injection client-side. Crawlers parse static
  // HTML so the Go /share/{owner}/{id} handler does the real work; this
  // keeps the meta accurate during SPA navigation and for the small set
  // of crawlers (LinkedIn, some Slackbot fallbacks) that do execute JS.
  useEffect(() => {
    if (!deck) return
    const title = cmdrCardName && cmdrCardName.toUpperCase() !== deckName
      ? `${deckName} · ${cmdrCardName}`
      : deckName
    const archetypeLabel = archetype && archetype !== 'UNKNOWN'
      ? archetype.charAt(0) + archetype.slice(1).toLowerCase()
      : ''
    const summaryParts = []
    if (archetypeLabel) summaryParts.push(archetypeLabel)
    if (wbs) summaryParts.push(`Bracket B${wbs}`)
    if (deckElo?.games > 0 && deckElo?.win_rate != null) {
      summaryParts.push(`${Math.round(deckElo.win_rate)}% WR · ${deckElo.games} games`)
    }
    const summary = summaryParts.length
      ? summaryParts.join(' · ')
      : `${title} — Commander deck on HEXDEK.`
    const pageURL = `https://hexdek.dev/share/${owner}/${id}${anonymous ? '?anon=1' : ''}`
    const imageURL = cmdrCardName
      ? `https://hexdek.dev/api/card-art/${encodeURIComponent(cmdrCardName.split('//')[0].trim())}`
      : 'https://hexdek.dev/og-default.png'

    const setMeta = (selector, attr, value) => {
      let el = document.head.querySelector(selector)
      if (!el) {
        el = document.createElement('meta')
        const [k, v] = selector.replace(/[\[\]"]/g, '').split('=')
        el.setAttribute(k, v)
        document.head.appendChild(el)
      }
      el.setAttribute(attr, value)
    }
    setMeta('meta[property="og:title"]', 'content', title)
    setMeta('meta[property="og:description"]', 'content', summary)
    setMeta('meta[property="og:url"]', 'content', pageURL)
    setMeta('meta[property="og:image"]', 'content', imageURL)
    setMeta('meta[property="og:type"]', 'content', 'article')
    setMeta('meta[name="twitter:title"]', 'content', title)
    setMeta('meta[name="twitter:description"]', 'content', summary)
    setMeta('meta[name="twitter:image"]', 'content', imageURL)
    setMeta('meta[name="twitter:card"]', 'content', 'summary_large_image')
  }, [deck, deckName, cmdrCardName, archetype, wbs, deckElo, owner, id, anonymous])

  if (loading) {
    return (
      <div style={{ padding: 36, textAlign: 'center' }}>
        <div className="t-md muted">&gt; LOADING DECK PREVIEW<span className="blink">_</span></div>
      </div>
    )
  }

  // Hot-cards ranking — filter+map+sort over commanderCardStats. Memoize
  // so it doesn't re-run on every render (e.g. when the ELO ticker
  // pushes a fresh `elo` array but nothing in this section changed).
  const ranked = useMemo(() => {
    const baseline = 25
    const deckCardNames = new Set(cards.map(c => c.name))
    return (commanderCardStats || [])
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
  }, [commanderCardStats, cards])
  const baseline = 25

  return (
    <div
      className="deck-archive-page"
      style={{
        '--page-wash': pageTheme.wash,
        '--accent': pageTheme.accent,
        minHeight: '100vh',
        padding: '0 0 24px',
      }}
    >
      {cmdrImageUrl && (
        <img className="art-ambience" src={cmdrImageUrl} alt="" aria-hidden="true" />
      )}

      <Tape
        left={anonymous
          ? `SHARE / / ANONYMOUS / / ${deckName}`
          : `SHARE / / ${owner?.toUpperCase()} / / ${deckName}`}
        mid={
          bracketDiverges
            ? `Estimated Bracket B${measuredBracket} (Bracket B${wbs}) · ${pageTheme.label}`
            : wbs
              ? `Bracket B${wbs} · ${pageTheme.label}`
              : `Bracket pending · ${pageTheme.label}`
        }
        right="HEXDEK ↗"
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
        <div className="deck-hero__corner deck-hero__corner--tr">{anonymous ? 'ANONYMOUS' : owner?.toUpperCase()} / / {id}</div>
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
              {bracketDiverges && <Tag solid kind="warn">EST. B{measuredBracket}</Tag>}
              <Tag>{archetype}</Tag>
              {colorIdentity.length > 0 && <Tag>{colorIdentity.join('')}</Tag>}
            </div>
            <div className="deck-hero__title-row">
              <h1 className="deck-hero__title">{deckName}</h1>
            </div>
            {cmdrCardName && cmdrCardName.toUpperCase() !== deckName && (
              <div className="deck-hero__sub">{cmdrCardName}</div>
            )}
          </div>
        </div>
      </div>

      <div className="deck-vital-signs">
        <div className="deck-vital-signs__cell">
          <div className="deck-vital-signs__num">
            {deckElo?.hex_rating != null ? Math.round(deckElo.hex_rating) : '—'}
          </div>
          <div className="deck-vital-signs__lbl">HexELO</div>
          {deckElo?.games > 0 ? (
            <div className="deck-vital-signs__sub">{deckElo.games.toLocaleString()} GAMES</div>
          ) : (
            <div className="deck-vital-signs__sub" style={{ opacity: 0.55 }}>NOT YET RANKED</div>
          )}
        </div>
        <div className="deck-vital-signs__cell">
          <div className="deck-vital-signs__num">
            {wbs && wbs !== '?' ? `B${wbs}${bracketDiverges ? ` → B${measuredBracket}` : ''}` : '—'}
          </div>
          <div className="deck-vital-signs__lbl">POWER LEVEL</div>
          {wbsLabel ? (
            <div className="deck-vital-signs__sub">{wbsLabel.toUpperCase()}</div>
          ) : (
            <div className="deck-vital-signs__sub" style={{ opacity: 0.55 }}>PENDING ANALYSIS</div>
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

      <div style={{ padding: '12px 16px 0', display: 'flex', flexDirection: 'column', gap: 14 }}>
        {ranked.length > 0 && (
          <Panel code="04.HC" title={`HOT CARDS / / TOP ${ranked.length} BY WR CONTRIBUTION`}>
            <div className="t-xs muted" style={{ marginBottom: 8 }}>
              Cards in this deck pulling the most weight in {deck?.commander_card || 'commander'} games — sorted by win-rate lift over the 25% 4-player baseline, sample-size weighted (√games).
            </div>
            <div className="hot-cards-grid">
              {ranked.map((r, i) => (
                <div key={i} className="hot-cards-tile">
                  <HotCardThumb name={r.name} />
                  <span className="hot-cards-chip hot-cards-chip--wr">{r.wr.toFixed(0)}%</span>
                  <span className="hot-cards-chip hot-cards-chip--games">{r.games}g · +{(r.wr - baseline).toFixed(0)}</span>
                </div>
              ))}
            </div>
          </Panel>
        )}

        {similarDecks && similarDecks.length > 0 && (
          <Panel code="04.SD" title={`SIMILAR DECKS / / ${similarDecks.length} MATCH${similarDecks.length === 1 ? '' : 'ES'}`}>
            <div className="t-xs muted" style={{ marginBottom: 8 }}>
              Decks ranked by shared-card overlap with bonuses for matching commander, archetype, and bracket.
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

        {/* Footer link to the full deck page — keeps the share preview
            self-contained as a teaser but lets readers click through for
            the analysis tab, decklist, gauntlet history, etc. Hidden in
            anonymous mode since the target URL contains the owner slug. */}
        {!anonymous && (
          <div style={{ padding: '8px 12px', textAlign: 'center' }}>
            <Link to={`/decks/${owner}/${id}`} className="t-xs muted" style={{ letterSpacing: '0.06em' }}>
              VIEW FULL DECK PAGE ON HEXDEK ↗
            </Link>
          </div>
        )}
      </div>
    </div>
  )
}
