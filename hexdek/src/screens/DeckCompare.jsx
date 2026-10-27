import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { Btn, Panel, Tag, Tape } from '../components/chrome'
import { api, cardArtUrl } from '../services/api'
import { useLiveSocket } from '../hooks/useLiveSocket'
import { useModalKeyboard } from '../hooks/useModalKeyboard'
import ContextBox from '../components/ContextBox'
import { useT } from '../i18n'
import { fill } from '../components/fill'

function normalizeCardName(name) {
  if (!name) return ''
  return name.split('//')[0].trim().toLowerCase()
}

function loadDeckBundle(owner, id) {
  if (!owner || !id) {
    return Promise.resolve({ deck: null, analysis: null, gauntlet: null })
  }
  const key = `${owner}/${id}`
  return Promise.allSettled([
    api.getDeck(key),
    api.getDeckAnalysis(key),
    api.getGauntlet(key),
  ]).then(([d, a, g]) => ({
    deck: d.status === 'fulfilled' ? d.value : null,
    analysis: a.status === 'fulfilled' && a.value?.status !== 'analyzing' ? a.value : null,
    gauntlet: g.status === 'fulfilled' && g.value?.status && g.value.status !== 'none' ? g.value : null,
  }))
}

function StatRow({ label, left, right, leftWin, rightWin }) {
  const showLeft = left ?? '—'
  const showRight = right ?? '—'
  return (
    <>
      <span className="cmp-stat__l" data-win={leftWin ? '1' : undefined}>{showLeft}</span>
      <span className="cmp-stat__k">{label}</span>
      <span className="cmp-stat__r" data-win={rightWin ? '1' : undefined}>{showRight}</span>
    </>
  )
}

function ColorBars({ demand }) {
  if (!demand) return <span className="t-xs muted">—</span>
  const order = ['W', 'U', 'B', 'R', 'G']
  const total = order.reduce((s, c) => s + (demand[c] || 0), 0)
  if (total === 0) return <span className="t-xs muted">—</span>
  return (
    <span className="cmp-colors">
      {order.map(c => {
        const v = demand[c] || 0
        if (!v) return null
        const pct = Math.round((v / total) * 100)
        return <span key={c} className={`cmp-colors__seg cmp-colors__seg--${c}`} title={`${c}: ${v} (${pct}%)`} style={{ width: `${pct}%` }} />
      })}
    </span>
  )
}

function ManaCurveMini({ curve }) {
  if (!curve?.distribution) return <span className="t-xs muted">—</span>
  const dist = curve.distribution
  const max = Math.max(1, ...Object.values(dist))
  const buckets = ['0', '1', '2', '3', '4', '5', '6+']
  return (
    <span className="cmp-curve">
      {buckets.map(b => {
        const v = dist[b] || 0
        const h = Math.round((v / max) * 100)
        return (
          <span key={b} className="cmp-curve__col" title={`${b}: ${v}`}>
            <span className="cmp-curve__bar" style={{ height: `${h}%` }} />
            <span className="cmp-curve__lbl">{b}</span>
          </span>
        )
      })}
    </span>
  )
}

function CommanderHero({ owner, id, deck, analysis, side, onPickReplace }) {
  const t = useT()
  // Strip bracket marker, owner slug, and trailing moxfield hash from the
  // id-based fallback so unparsed decks don't show titles like
  // "HERIGAST ERUPTING NULLKITE B2 LAODI Z8BQG8TF".
  const cleanIdTitle = (slug, ownerSlug) => {
    let s = String(slug || '')
    s = s.replace(/_[A-Za-z0-9]{8,}$/, '')
    if (ownerSlug) s = s.replace(new RegExp(`_${ownerSlug.toLowerCase()}$`, 'i'), '')
    s = s.replace(/_b[0-5]$/i, '')
    return s.replace(/_/g, ' ').toUpperCase() || t('cmp.deck_fallback')
  }
  const cmdr = deck?.commander_card || deck?.commander || cleanIdTitle(id, owner)
  const art = cardArtUrl(cmdr)
  const archetype = analysis?.archetype?.toUpperCase() || '—'
  const bracket = analysis?.bracket || deck?.bracket
  return (
    <div className={`cmp-hero cmp-hero--${side}`}>
      <div className="cmp-hero__corner">
        {side === 'L' ? t('cmp.side_a') : t('cmp.side_b')} / / {owner?.toUpperCase()} / / {id}
      </div>
      {art && <div className="cmp-hero__art" style={{ backgroundImage: `url(${art})` }} />}
      <div className="cmp-hero__scrim" />
      <div className="cmp-hero__body">
        <div className="cmp-hero__title">{cmdr?.toUpperCase()}</div>
        <div className="cmp-hero__meta">
          {bracket && <Tag solid>B{bracket}</Tag>}
          <Tag>{archetype}</Tag>
        </div>
        {onPickReplace && (
          <button className="cmp-hero__swap" type="button" onClick={onPickReplace}>{t('cmp.swap_deck')}</button>
        )}
      </div>
    </div>
  )
}

export default function DeckCompare() {
  const t = useT()
  const { owner1, deck1, owner2, deck2 } = useParams()
  const navigate = useNavigate()
  const { elo } = useLiveSocket()

  const [bundleA, setBundleA] = useState({ deck: null, analysis: null, gauntlet: null })
  const [bundleB, setBundleB] = useState({ deck: null, analysis: null, gauntlet: null })
  const [loadingA, setLoadingA] = useState(true)
  const [loadingB, setLoadingB] = useState(true)
  const [pickerSide, setPickerSide] = useState(null)

  useEffect(() => {
    setLoadingA(true)
    loadDeckBundle(owner1, deck1).then(b => { setBundleA(b); setLoadingA(false) })
  }, [owner1, deck1])

  useEffect(() => {
    setLoadingB(true)
    loadDeckBundle(owner2, deck2).then(b => { setBundleB(b); setLoadingB(false) })
  }, [owner2, deck2])

  const eloByKey = useMemo(() => {
    const m = {}
    for (const e of elo || []) {
      if (e.deck_id) {
        m[e.deck_id] = e
        if (e.owner) m[`${e.owner}/${e.deck_id}`] = e
      }
    }
    return m
  }, [elo])

  const eloA = eloByKey[`${owner1}/${deck1}`] || eloByKey[deck1]
  const eloB = eloByKey[`${owner2}/${deck2}`] || eloByKey[deck2]

  const cardSets = useMemo(() => {
    const a = new Map()
    const b = new Map()
    for (const c of bundleA.deck?.cards || []) a.set(normalizeCardName(c.name), c)
    for (const c of bundleB.deck?.cards || []) b.set(normalizeCardName(c.name), c)
    const shared = []
    const onlyA = []
    const onlyB = []
    for (const [k, v] of a) (b.has(k) ? shared.push({ a: v, b: b.get(k) }) : onlyA.push(v))
    for (const [k, v] of b) if (!a.has(k)) onlyB.push(v)
    const byName = (x, y) => (x.name || x.a?.name || '').localeCompare(y.name || y.a?.name || '')
    return {
      shared: shared.sort((x, y) => (x.a.name || '').localeCompare(y.a.name || '')),
      onlyA: onlyA.sort(byName),
      onlyB: onlyB.sort(byName),
    }
  }, [bundleA.deck, bundleB.deck])

  const winsA = bundleA.gauntlet?.wins ?? null
  const gamesA = bundleA.gauntlet?.games_played ?? null
  const winsB = bundleB.gauntlet?.wins ?? null
  const gamesB = bundleB.gauntlet?.games_played ?? null
  const winRateA = gamesA ? Math.round((winsA / gamesA) * 1000) / 10 : null
  const winRateB = gamesB ? Math.round((winsB / gamesB) * 1000) / 10 : null

  const stats = [
    {
      id: 'bracket',
      label: t('deck.bracket'),
      left: bundleA.analysis?.bracket || bundleA.deck?.bracket,
      right: bundleB.analysis?.bracket || bundleB.deck?.bracket,
      cmp: 'higher',
    },
    { id: 'archetype', label: t('deck.archetype'), left: bundleA.analysis?.archetype?.toUpperCase(), right: bundleB.analysis?.archetype?.toUpperCase(), cmp: null },
    { id: 'elo', label: 'ELO', left: eloA ? Math.round(eloA.rating) : null, right: eloB ? Math.round(eloB.rating) : null, cmp: 'higher' },
    { id: 'wr', label: t('deck.win_rate'), left: winRateA != null ? `${winRateA}%` : null, right: winRateB != null ? `${winRateB}%` : null, cmp: 'higher', leftN: winRateA, rightN: winRateB },
    { id: 'keep', label: t('dka.sp_keepable'), left: bundleA.analysis?.keepable_hand_pct != null ? `${Math.round(bundleA.analysis.keepable_hand_pct)}%` : null, right: bundleB.analysis?.keepable_hand_pct != null ? `${Math.round(bundleB.analysis.keepable_hand_pct)}%` : null, cmp: 'higher', leftN: bundleA.analysis?.keepable_hand_pct, rightN: bundleB.analysis?.keepable_hand_pct },
    { id: 'power', label: t('cmp.s_power'), left: bundleA.analysis?.power_percentile, right: bundleB.analysis?.power_percentile, cmp: 'higher' },
    { id: 'cmc', label: t('cmp.s_avg_cmc'), left: bundleA.analysis?.mana_curve?.avg_cmc?.toFixed(2), right: bundleB.analysis?.mana_curve?.avg_cmc?.toFixed(2), cmp: 'lower', leftN: bundleA.analysis?.mana_curve?.avg_cmc, rightN: bundleB.analysis?.mana_curve?.avg_cmc },
    { id: 'mana', label: t('dka.sp_mana_base'), left: bundleA.analysis?.mana_base_grade, right: bundleB.analysis?.mana_base_grade, cmp: null },
    { id: 'cheap', label: t('cmp.s_cheap_int'), left: bundleA.analysis?.cheap_interaction, right: bundleB.analysis?.cheap_interaction, cmp: 'higher' },
    { id: 'count', label: t('cmp.s_card_count'), left: bundleA.deck?.card_count, right: bundleB.deck?.card_count, cmp: null },
  ]

  const tapeMid = (loadingA || loadingB)
    ? t('common.loading')
    : t('cmp.tape_counts', {
      shared: cardSets.shared.length,
      a: cardSets.onlyA.length,
      b: cardSets.onlyB.length,
    })

  return (
    <>
      <Tape
        left={t('cmp.tape_left', { a: (owner1 || '?').toUpperCase(), b: (owner2 || '?').toUpperCase() })}
        mid={tapeMid}
        right="DOC HX-410"
      />
      <div className="cmp-layout">
        <div className="cmp-heroes">
          <CommanderHero owner={owner1} id={deck1} deck={bundleA.deck} analysis={bundleA.analysis} side="L"
            onPickReplace={() => setPickerSide('A')} />
          <div className="cmp-vs">
            <span>{t('cmp.vs')}</span>
            <button type="button" className="cmp-vs__swap" onClick={() => navigate(`/compare/${owner2}/${deck2}/${owner1}/${deck1}`)}
              title={t('cmp.swap_sides')}>↔</button>
          </div>
          <CommanderHero owner={owner2} id={deck2} deck={bundleB.deck} analysis={bundleB.analysis} side="R"
            onPickReplace={() => setPickerSide('B')} />
        </div>

        <div className="cmp-stats-panel">
          <Panel code="CMP.STATS" title={t('cmp.metrics')}>
            <div className="cmp-stats">
              {stats.map(row => {
                let leftWin = false, rightWin = false
                if (row.cmp && row.left != null && row.right != null) {
                  const ln = row.leftN != null ? row.leftN : Number(row.left)
                  const rn = row.rightN != null ? row.rightN : Number(row.right)
                  if (Number.isFinite(ln) && Number.isFinite(rn) && ln !== rn) {
                    if (row.cmp === 'higher') { leftWin = ln > rn; rightWin = rn > ln }
                    else { leftWin = ln < rn; rightWin = rn < ln }
                  }
                }
                return <StatRow key={row.id} label={row.label} left={row.left} right={row.right} leftWin={leftWin} rightWin={rightWin} />
              })}

              <span className="cmp-stat__l"><ColorBars demand={bundleA.analysis?.color_demand} /></span>
              <span className="cmp-stat__k">{t('cmp.s_mana_ratio')}</span>
              <span className="cmp-stat__r"><ColorBars demand={bundleB.analysis?.color_demand} /></span>

              <span className="cmp-stat__l"><ManaCurveMini curve={bundleA.analysis?.mana_curve} /></span>
              <span className="cmp-stat__k">{t('cmp.s_mana_curve')}</span>
              <span className="cmp-stat__r"><ManaCurveMini curve={bundleB.analysis?.mana_curve} /></span>
            </div>
          </Panel>
        </div>

        <div className="cmp-cards">
          <Panel code="CMP.A" title={t('cmp.only_a', { count: cardSets.onlyA.length })}>
            <div className="cmp-cardlist">
              {cardSets.onlyA.length === 0 ? (
                <span className="t-xs muted">{t('rpt.none')}</span>
              ) : cardSets.onlyA.map(c => (
                <div key={c.name} className="cmp-cardrow"
                  onClick={() => navigate(`/cards/${encodeURIComponent(c.name.split('//')[0].trim())}`)}>
                  <span className="cmp-cardrow__qty">{c.quantity || 1}</span>
                  <span className="cmp-cardrow__name">{c.name}</span>
                </div>
              ))}
            </div>
          </Panel>

          <Panel code="CMP.X" title={t('cmp.shared', { count: cardSets.shared.length })}>
            <div className="cmp-cardlist">
              {cardSets.shared.length === 0 ? (
                <span className="t-xs muted">{t('cmp.no_overlap')}</span>
              ) : cardSets.shared.map(({ a }) => (
                <div key={a.name} className="cmp-cardrow cmp-cardrow--shared"
                  onClick={() => navigate(`/cards/${encodeURIComponent(a.name.split('//')[0].trim())}`)}>
                  <span className="cmp-cardrow__qty">{a.quantity || 1}</span>
                  <span className="cmp-cardrow__name">{a.name}</span>
                </div>
              ))}
            </div>
          </Panel>

          <Panel code="CMP.B" title={t('cmp.only_b', { count: cardSets.onlyB.length })}>
            <div className="cmp-cardlist">
              {cardSets.onlyB.length === 0 ? (
                <span className="t-xs muted">{t('rpt.none')}</span>
              ) : cardSets.onlyB.map(c => (
                <div key={c.name} className="cmp-cardrow"
                  onClick={() => navigate(`/cards/${encodeURIComponent(c.name.split('//')[0].trim())}`)}>
                  <span className="cmp-cardrow__qty">{c.quantity || 1}</span>
                  <span className="cmp-cardrow__name">{c.name}</span>
                </div>
              ))}
            </div>
          </Panel>
        </div>

        <ContextBox id="deck.compare.footer" style={{ maxWidth: 720, margin: '0 auto' }}>
          {fill(t('cmp.ctx_footer'), {
            swap: <strong>{t('cmp.swap_deck')}</strong>,
          })}
        </ContextBox>
        <div className="cmp-footer">
          <Btn ghost arrow="←" onClick={() => navigate(`/decks/${owner1}/${deck1}`)}>{t('cmp.back_to', { deck: `${owner1?.toUpperCase()}/${deck1}` })}</Btn>
          <Btn ghost arrow="←" onClick={() => navigate(`/decks/${owner2}/${deck2}`)}>{t('cmp.back_to', { deck: `${owner2?.toUpperCase()}/${deck2}` })}</Btn>
        </div>
      </div>

      {pickerSide && (
        <DeckPicker
          excludeKey={pickerSide === 'A' ? `${owner2}/${deck2}` : `${owner1}/${deck1}`}
          onClose={() => setPickerSide(null)}
          onPick={(d) => {
            setPickerSide(null)
            const next = pickerSide === 'A'
              ? `/compare/${d.owner}/${d.id}/${owner2}/${deck2}`
              : `/compare/${owner1}/${deck1}/${d.owner}/${d.id}`
            navigate(next)
          }}
        />
      )}
    </>
  )
}

function DeckPicker({ excludeKey, onClose, onPick }) {
  const t = useT()
  const panelRef = useModalKeyboard({ onClose })
  const [decks, setDecks] = useState([])
  const [loading, setLoading] = useState(true)
  const [filter, setFilter] = useState('')

  useEffect(() => {
    api.getDecks().then(d => { setDecks(Array.isArray(d) ? d : []); setLoading(false) }).catch(() => setLoading(false))
  }, [])

  const filtered = useMemo(() => {
    const q = filter.trim().toLowerCase()
    return decks.filter(d => `${d.owner}/${d.id}` !== excludeKey && (
      !q || `${d.name} ${d.commander_card || d.commander || ''} ${d.owner}`.toLowerCase().includes(q)
    )).slice(0, 80)
  }, [decks, filter, excludeKey])

  return (
    <div className="cmp-picker" onMouseDown={onClose}>
      <div
        ref={panelRef}
        className="cmp-picker__panel"
        onMouseDown={e => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-label={t('cmp.pick_aria')}
      >
        <div className="cmp-picker__hd">
          <span>{t('cmp.pick_title')}</span>
          <button
            type="button"
            className="cmp-picker__close"
            onClick={onClose}
            aria-label={t('cmp.close_aria')}
            style={{ background: 'transparent', border: 'none', color: 'inherit', font: 'inherit', cursor: 'pointer', padding: 0 }}
          >{t('dka.esc')}</button>
        </div>
        <input
          autoFocus
          className="cmp-picker__input"
          placeholder={t('cmp.filter_ph')}
          value={filter}
          onChange={e => setFilter(e.target.value)}
        />
        <div className="cmp-picker__list">
          {loading ? <div className="cmp-picker__note">{t('dka.loading_ell')}</div>
            : filtered.length === 0 ? <div className="cmp-picker__note">{t('cmp.no_matches')}</div>
            : filtered.map(d => (
              <button key={`${d.owner}/${d.id}`} type="button" className="cmp-picker__row" onClick={() => onPick(d)}>
                <span className="cmp-picker__owner">{d.owner?.toUpperCase()}</span>
                <span className="cmp-picker__name">{(d.commander_card || d.commander || d.id || '').toString()}</span>
                {d.bracket && <span className="cmp-picker__b">B{d.bracket}</span>}
              </button>
            ))}
        </div>
      </div>
    </div>
  )
}

export { DeckPicker }
