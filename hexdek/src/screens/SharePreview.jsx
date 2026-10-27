import { useEffect, useMemo, useState } from 'react'
import { useParams, Link, useSearchParams } from 'react-router-dom'
import { isAnonymousShare } from '../lib/deckShare'
import { Panel, Tag, Tape } from '../components/chrome'
import CardLink from '../components/CardLink'
import { api, cardArtUrl, cardImageUrl } from '../services/api'
import { useArtContrast } from '../hooks/useArtContrast'
import { useLiveSocket } from '../hooks/useLiveSocket'
import { useT } from '../i18n'

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
    return { wash: 'linear-gradient(135deg, rgba(28,29,22,0.9), rgba(20,21,15,0.9))', accent: '#8a9682', labelKey: 'shr.ci_colorless' }
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
  // The 25 guild / shard / wedge names are proper names and stay as they
  // are. The seven descriptive labels are catalog PATHS instead: this
  // function is memoised at its call site, so a word returned from here
  // would freeze in whatever language was active when the colour identity
  // last changed. The render site does the wording.
  const COMBO_NAMES = {
    WU: 'AZORIUS', UB: 'DIMIR', BR: 'RAKDOS', RG: 'GRUUL', GW: 'SELESNYA',
    WB: 'ORZHOV', UR: 'IZZET', BG: 'GOLGARI', RW: 'BOROS', UG: 'SIMIC',
    WUB: 'ESPER', UBR: 'GRIXIS', BRG: 'JUND', RGW: 'NAYA', GWU: 'BANT',
    WBG: 'ABZAN', URW: 'JESKAI', BGU: 'SULTAI', RWB: 'MARDU', GUR: 'TEMUR',
    WUBR: 'YORE-TILLER', WUBG: 'WITCH-MAW', WURG: 'INK-TREADER', WBRG: 'DUNE-BROOD', UBRG: 'GLINT-EYE',
  }
  const COMBO_KEYS = {
    W: 'shr.ci_mono_w', U: 'shr.ci_mono_u', B: 'shr.ci_mono_b',
    R: 'shr.ci_mono_r', G: 'shr.ci_mono_g', WUBRG: 'shr.ci_five_color',
  }
  const joined = ids.join('')
  return {
    wash: `linear-gradient(135deg, ${stops})`,
    accent: accentColor,
    label: COMBO_NAMES[joined] || joined,
    labelKey: COMBO_KEYS[joined] || null,
  }
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
  const t = useT()
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
    if (!slug) return t('shr.deck_fallback')
    let s = String(slug)
    s = s.replace(/_[A-Za-z0-9]{8,}$/, '')
    if (ownerSlug) s = s.replace(new RegExp(`_${ownerSlug.toLowerCase()}$`, 'i'), '')
    s = s.replace(/_b[0-5]$/i, '')
    return s.replace(/_/g, ' ').toUpperCase() || t('shr.deck_fallback')
  }
  const deckName = deck?.custom_name || deck?.commander || slugToTitle(id, owner)
  const cards = deck?.cards || []
  const wbs = analysis?.bracket || deck?.bracket || null
  const wbsLabel = analysis?.bracket_label || ''
  const measuredBracket = analysis?.measured_bracket || null
  const bracketDiverges = measuredBracket != null && wbs != null && Number(measuredBracket) !== Number(wbs)
  // Freya's archetype arrives already worded, so it is echoed as it comes.
  // The 'UNKNOWN' sentinel stays English because the og:/twitter: block
  // below compares against it; only the on-screen tag is worded.
  const archetype = analysis?.archetype?.toUpperCase() || 'UNKNOWN'
  const archetypeTag = archetype === 'UNKNOWN' ? t('shr.unknown') : archetype

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
  // Worded out here, not inside the memo: the colour label is the one
  // string on this screen that a memo would hold across a switch.
  const colorLabel = pageTheme.labelKey ? t(pageTheme.labelKey) : pageTheme.label
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
  //
  // The strings built below stay ENGLISH on purpose. They are crawler-
  // facing and have to match what the Go handler emits for the same
  // public URL, which has no viewer locale to follow.
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

  // Hot-cards ranking — filter+map+sort over commanderCardStats. Memoize
  // so it doesn't re-run on every render (e.g. when the ELO ticker
  // pushes a fresh `elo` array but nothing in this section changed).
  //
  // MUST stay above the `if (loading)` early return: a hook after a
  // conditional return changes the hook count between the loading render
  // and the loaded render, and React throws "Rendered more hooks than
  // during the previous render" — unmounting the whole route to the
  // error boundary the moment data arrives (caught by
  // e2e/render-crash-regression.spec.ts).
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

  if (loading) {
    return (
      <div style={{ padding: 36, textAlign: 'center' }}>
        <div className="t-md muted">{t('shr.loading')}<span className="blink">_</span></div>
      </div>
    )
  }

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
          ? t('shr.tape_anon', { name: deckName })
          : t('shr.tape_owner', { owner: owner?.toUpperCase(), name: deckName })}
        mid={
          bracketDiverges
            ? t('shr.mid_diverges', { measured: measuredBracket, declared: wbs, color: colorLabel })
            : wbs
              ? t('shr.mid_bracket', { declared: wbs, color: colorLabel })
              : t('shr.mid_pending', { color: colorLabel })
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
        <div className="deck-hero__corner deck-hero__corner--tl">04.HERO / / {colorLabel}</div>
        <div className="deck-hero__corner deck-hero__corner--tr">{anonymous ? t('shr.anonymous') : owner?.toUpperCase()} / / {id}</div>
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
              <Tag solid>{wbs ? `B${wbs}` : t('shr.bracket_pending')}{wbs && wbsLabel ? ' · ' + wbsLabel : ''}</Tag>
              {bracketDiverges && <Tag solid kind="warn">{t('shr.est_bracket', { measured: measuredBracket })}</Tag>}
              <Tag>{archetypeTag}</Tag>
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
            <div className="deck-vital-signs__sub">{t('shr.n_games', { count: deckElo.games.toLocaleString() })}</div>
          ) : (
            <div className="deck-vital-signs__sub" style={{ opacity: 0.55 }}>{t('shr.not_ranked')}</div>
          )}
        </div>
        <div className="deck-vital-signs__cell">
          <div className="deck-vital-signs__num">
            {wbs && wbs !== '?' ? `B${wbs}${bracketDiverges ? ` → B${measuredBracket}` : ''}` : '—'}
          </div>
          <div className="deck-vital-signs__lbl">{t('shr.power_level')}</div>
          {wbsLabel ? (
            <div className="deck-vital-signs__sub">{wbsLabel.toUpperCase()}</div>
          ) : (
            <div className="deck-vital-signs__sub" style={{ opacity: 0.55 }}>{t('shr.pending_analysis')}</div>
          )}
        </div>
        <div className="deck-vital-signs__cell">
          <div className="deck-vital-signs__num">
            {deckElo?.win_rate != null ? `${deckElo.win_rate}%` : '—'}
          </div>
          <div className="deck-vital-signs__lbl">{t('shr.win_rate')}</div>
          {deckElo?.wins != null && deckElo?.losses != null ? (
            <div className="deck-vital-signs__sub">
              <span style={{ color: 'var(--ok)' }}>{t('shr.wins_suffix', { count: deckElo.wins.toLocaleString() })}</span>
              {' · '}
              <span style={{ color: 'var(--danger)' }}>{t('shr.losses_suffix', { count: deckElo.losses.toLocaleString() })}</span>
            </div>
          ) : (
            <div className="deck-vital-signs__sub" style={{ opacity: 0.55 }}>{t('shr.no_sample')}</div>
          )}
        </div>
      </div>

      <div style={{ padding: '12px 16px 0', display: 'flex', flexDirection: 'column', gap: 14 }}>
        {ranked.length > 0 && (
          <Panel code="04.HC" title={t('shr.hot_title', { count: ranked.length })}>
            <div className="t-xs muted" style={{ marginBottom: 8 }}>
              {t('shr.hot_note', { commander: deck?.commander_card || t('shr.hot_note_commander') })}
            </div>
            <div className="hot-cards-grid">
              {ranked.map((r, i) => (
                <div key={i} className="hot-cards-tile">
                  <HotCardThumb name={r.name} />
                  <span className="hot-cards-chip hot-cards-chip--wr">{r.wr.toFixed(0)}%</span>
                  <span className="hot-cards-chip hot-cards-chip--games">{t('shr.games_short', { count: r.games })} · +{(r.wr - baseline).toFixed(0)}</span>
                </div>
              ))}
            </div>
          </Panel>
        )}

        {/* Plural picked at the CALL SITE with two keys, rather than
            bolting an English 'ES' onto the count. */}
        {similarDecks && similarDecks.length > 0 && (
          <Panel code="04.SD" title={similarDecks.length === 1
            ? t('shr.similar_title_one', { count: similarDecks.length })
            : t('shr.similar_title_many', { count: similarDecks.length })}>
            <div className="t-xs muted" style={{ marginBottom: 8 }}>
              {t('shr.similar_note')}
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
                    title={t('shr.tile_title', { name: showName, owner: d.owner, count: d.shared_cards })}
                  >
                    <div
                      className={`similar-decks-tile__art ${cmdrArt ? '' : 'hatch'}`}
                      style={cmdrArt ? { backgroundImage: `url(${cmdrArt})` } : undefined}
                    >
                      {hexRating != null && (
                        <span className="similar-decks-tile__chip similar-decks-tile__chip--elo">{hexRating}</span>
                      )}
                      <span className="similar-decks-tile__chip similar-decks-tile__chip--shared">{t('shr.n_shared', { count: d.shared_cards })}</span>
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
              {t('shr.view_full')}
            </Link>
          </div>
        )}
      </div>
    </div>
  )
}
