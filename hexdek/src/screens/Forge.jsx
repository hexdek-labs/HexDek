import { useState, useEffect } from 'react'
import { useSearchParams } from 'react-router-dom'
import { Panel, KV, Bar, Tag, Btn, Tape } from '../components/chrome'
import { api, cardArtUrl } from '../services/api'
import { useDecks } from '../hooks/useData'
import { trackEvent } from '../hooks/useAnalytics'
import ContextBox from '../components/ContextBox'
import AnalysisFreshnessBanner from '../components/AnalysisFreshnessBanner'
import { useT } from '../i18n'
import { fill } from '../components/fill'

// Freya's eight eval dimensions, keyed by the name it serialises
// (cmd/hexdek-freya/deckprofile.go:738). Catalog PATHS, not words: this
// table is read at render from module scope, so a word here would be fixed
// at load. An unknown ninth dimension falls through to its raw wire name
// rather than rendering blank.
const EVAL_WEIGHT_KEYS = {
  board_presence: 'frg.ew_board_presence',
  card_advantage: 'frg.ew_card_advantage',
  mana_advantage: 'frg.ew_mana_advantage',
  life_resource: 'frg.ew_life_resource',
  combo_proximity: 'frg.ew_combo_proximity',
  threat_exposure: 'frg.ew_threat_exposure',
  commander_progress: 'frg.ew_commander_progress',
  graveyard_value: 'frg.ew_graveyard_value',
}

// Sentinel for a start-gauntlet failure that carried no message of its own.
// A worded fallback parked in state would freeze in whatever language was
// active when the call failed, so state holds the catalog path.
const START_FAILED = 'frg.err_start'

export default function Forge() {
  const t = useT()
  const { data: decks, loading: decksLoading } = useDecks()
  const [searchParams] = useSearchParams()
  const [selectedDeck, setSelectedDeck] = useState(null)
  const [deckFilter, setDeckFilter] = useState('')
  const [deck, setDeck] = useState(null)
  const [analysis, setAnalysis] = useState(null)
  const [loading, setLoading] = useState(false)
  const [gauntlet, setGauntlet] = useState(null)
  const [gauntletGames, setGauntletGames] = useState(500)
  const [gauntletError, setGauntletError] = useState(null)

  // Auto-select: prefer ?deck=owner/id query param, otherwise first deck.
  useEffect(() => {
    if (!decks.length) return
    const qDeck = searchParams.get('deck')
    if (qDeck) {
      const [qOwner, qId] = qDeck.split('/')
      const match = decks.find(d => d.owner === qOwner && d.id === qId)
      if (match && (!selectedDeck || selectedDeck.owner !== qOwner || selectedDeck.id !== qId)) {
        setSelectedDeck(match)
        return
      }
    }
    if (!selectedDeck && decks[0].owner && decks[0].id) {
      setSelectedDeck(decks[0])
    }
  }, [decks, selectedDeck, searchParams])

  // Fetch deck + analysis when selection changes
  useEffect(() => {
    if (!selectedDeck || !selectedDeck.owner || !selectedDeck.id) return
    setLoading(true)
    const deckId = `${selectedDeck.owner}/${selectedDeck.id}`
    Promise.allSettled([
      api.getDeck(deckId),
      api.getDeckAnalysis(deckId),
    ]).then(([deckRes, analysisRes]) => {
      if (deckRes.status === 'fulfilled') setDeck(deckRes.value)
      else setDeck(null)
      if (analysisRes.status === 'fulfilled') setAnalysis(analysisRes.value)
      else setAnalysis(null)
      setLoading(false)
    })
  }, [selectedDeck])

  // Clear gauntlet state when deck changes
  useEffect(() => {
    if (!selectedDeck) return
    setGauntlet(null)
    setGauntletError(null)
    // Check if there's an existing gauntlet for this deck
    const deckId = `${selectedDeck.owner}/${selectedDeck.id}`
    api.getGauntlet(deckId).then(r => {
      if (r && r.status && r.status !== 'none') setGauntlet(r)
    }).catch(() => {})
  }, [selectedDeck])

  // Stream gauntlet progress via SSE while running.
  useEffect(() => {
    if (!gauntlet || gauntlet.status !== 'running' || !selectedDeck) return
    const deckId = `${selectedDeck.owner}/${selectedDeck.id}`
    const es = new EventSource(api.tournamentEventsUrl(deckId))
    const onSnapshot = (ev) => {
      try {
        const r = JSON.parse(ev.data)
        if (r && r.status && r.status !== 'none') setGauntlet(r)
        if (r && r.status !== 'running' && r.status !== 'none') es.close()
      } catch {}
    }
    es.addEventListener('snapshot', onSnapshot)
    es.onerror = () => { es.close() }
    return () => {
      es.removeEventListener('snapshot', onSnapshot)
      es.close()
    }
  }, [gauntlet?.status, selectedDeck])

  const handleStartGauntlet = async () => {
    if (!selectedDeck) return
    setGauntletError(null)
    const deckId = `${selectedDeck.owner}/${selectedDeck.id}`
    try {
      trackEvent('start_gauntlet', { deck: deckId, games: gauntletGames })
      await api.startGauntlet(deckId, gauntletGames)
      setGauntlet({ status: 'running', deck_key: deckId, games: 0, target: gauntletGames, wins: 0, losses: 0, win_rate: 0, commander: deck?.commander || selectedDeck?.name || '' })
    } catch (e) {
      setGauntletError(e.message || START_FAILED)
    }
  }

  const deckName = deck?.commander || selectedDeck?.name || t('frg.no_deck_selected')
  const cardCount = deck?.card_count || deck?.cards?.length || 0
  const wbs = analysis?.bracket || deck?.bracket || '?'
  const wbsLabel = analysis?.bracket_label || ''
  // measured_bracket = Freya's signal-computed value (renamed from
  // mechanical_bracket as part of the bracket-vs-measured refactor).
  // Distinct from bracket which is the declared / rubber-stamp identity
  // (B2 for wizards/ precons, user-editable otherwise).
  const measuredBracket = analysis?.measured_bracket || null
  const measuredBracketLabel = analysis?.measured_bracket_label || ''
  const bracketDiverges = measuredBracket != null && wbs !== '?' && measuredBracket !== Number(wbs)
  const gameChangers = analysis?.game_changer_count ?? null
  // Freya words the archetype server-side - echoed, never re-worded here.
  const archetype = analysis?.archetype?.toUpperCase() || '—'
  const summary = analysis?.gameplan_summary || ''
  const winLines = analysis?.win_lines || []
  const valueKeys = analysis?.value_engine_keys || []
  const evalWeights = analysis?.eval_weights || {}
  const cards = deck?.cards || []
  const cmdrCardName = deck?.commander_card || ''

  return (
    <>
      <Tape
        left={t('frg.tape_left')}
        mid={selectedDeck
          ? (bracketDiverges
              ? t('frg.tape_est_bracket', { measured: measuredBracket, declared: wbs })
              : t('frg.tape_bracket', { declared: wbs }))
          : t('frg.select_deck')}
        right={selectedDeck ? `${selectedDeck.owner?.toUpperCase()} / / ${deckName}` : ''}
      />

      <div style={{ padding: 18, flex: 1, display: 'flex', flexDirection: 'column', gap: 14, overflow: 'auto' }}>
        {/* Deck selector */}
        <Panel code="11.SEL" title={t('frg.select_deck')}>
          {/* Filter input — with 1300+ decks the pill wall was unscannable.
              Live filter against owner OR deck name. Empty filter shows all. */}
          {decks.length > 12 && (
            <input
              type="text"
              value={deckFilter}
              onChange={(e) => setDeckFilter(e.target.value)}
              placeholder={t('frg.filter_ph', { count: decks.length })}
              style={{
                width: '100%', padding: '8px 10px', marginBottom: 10,
                background: 'var(--bg-2, rgba(0,0,0,0.3))',
                border: '1px solid var(--rule-2)',
                color: 'var(--ink)', fontFamily: 'inherit', fontSize: 11,
                letterSpacing: '0.04em',
              }}
            />
          )}
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', alignItems: 'center', maxHeight: 320, overflowY: 'auto' }}>
            {decksLoading ? (
              <span className="t-xs muted">{t('frg.loading_decks')}<span className="blink">_</span></span>
            ) : decks.length === 0 ? (
              <span className="t-xs muted">{t('frg.no_decks')}</span>
            ) : (() => {
              const q = deckFilter.trim().toLowerCase()
              const filtered = q
                ? decks.filter(d =>
                    (d.owner || '').toLowerCase().includes(q) ||
                    (d.name || '').toLowerCase().includes(q) ||
                    (d.commander_card || d.commander || '').toLowerCase().includes(q)
                  )
                : decks
              if (filtered.length === 0) {
                return <span className="t-xs muted">{t('frg.no_matches', { query: deckFilter })}</span>
              }
              return filtered.slice(0, 200).map((d) => (
                <Tag
                  key={`${d.owner}/${d.id}`}
                  solid={selectedDeck?.id === d.id && selectedDeck?.owner === d.owner}
                  onClick={() => setSelectedDeck(d)}
                  style={{ cursor: 'pointer' }}
                >
                  {d.owner?.toUpperCase()} / {d.name}
                </Tag>
              )).concat(
                filtered.length > 200
                  ? [<span key="more" className="t-xs muted" style={{ alignSelf: 'center' }}>
                      {t('frg.more_decks', { count: filtered.length - 200 })}
                    </span>]
                  : []
              )
            })()}
          </div>
        </Panel>

        {loading && (
          <div className="t-md muted" style={{ textAlign: 'center', padding: 36 }}>
            &gt; {t('frg.loading_deck_data')}<span className="blink">_</span>
          </div>
        )}

        {!loading && selectedDeck && (
          <>
            {/* Deck overview */}
            <Panel code="11.A" title={t('frg.p_subject', { name: deckName })}>
              <div style={{ display: 'grid', gridTemplateColumns: '120px 1fr', gap: 18, alignItems: 'start' }}>
                <div style={{ aspectRatio: '5/7', border: '1px solid var(--rule-2)', overflow: 'hidden', position: 'relative' }} className={cmdrCardName ? '' : 'hatch'}>
                  {cmdrCardName ? (
                    <img
                      src={cardArtUrl(cmdrCardName)}
                      alt={cmdrCardName}
                      style={{ width: '100%', height: '100%', objectFit: 'cover', filter: 'saturate(0.6) contrast(1.1)' }}
                      onError={e => { e.target.style.display = 'none'; e.target.parentElement.classList.add('hatch') }}
                    />
                  ) : (
                    <div style={{ position: 'absolute', inset: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 36, color: 'var(--ink-3)' }}>?</div>
                  )}
                </div>
                <div>
                  {/* wbsLabel / measuredBracketLabel are Freya's own
                      bracket wording - echoed as they arrive. GAME CHANGERS
                      is the format's term and stays English. */}
                  <KV rows={[
                    [t('frg.kv_commander'), deckName],
                    [t('frg.kv_owner'), selectedDeck.owner?.toUpperCase()],
                    [t('frg.kv_cards'), `${cardCount}`],
                    [t('frg.kv_bracket'), `B${wbs}${wbsLabel ? ' ' + wbsLabel : ''}`],
                    [t('frg.kv_est_bracket'), measuredBracket != null
                      ? `B${measuredBracket}${measuredBracketLabel ? ' ' + measuredBracketLabel : ''}${bracketDiverges ? ' ' + t('frg.diverges') : ''}`
                      : '—'],
                    ['GAME CHANGERS', gameChangers != null ? `${gameChangers}` : '—'],
                    [t('frg.kv_archetype'), archetype],
                  ]} />
                  {/* gameplan_summary hidden — Freya win-line detection needs accuracy pass */}
                </div>
              </div>
            </Panel>

            {/* Stale-analysis / unresolved-card caveats for the Freya
                sections below. Renders nothing for a fresh analysis. */}
            <AnalysisFreshnessBanner
              analysis={analysis}
              deckId={`${selectedDeck.owner}/${selectedDeck.id}`}
              onRefreshed={setAnalysis}
            />

            {/* Analysis section — from Freya */}
            {analysis ? (
              <>
                {/* Eval weights */}
                {Object.keys(evalWeights).length > 0 && (
                  <Panel code="11.B" title={t('frg.p_eval')} right={<Tag solid title={bracketDiverges ? t('frg.bracket_hint') : undefined}>{bracketDiverges ? t('frg.tag_bracket_est', { declared: wbs, measured: measuredBracket }) : t('frg.tape_bracket', { declared: wbs })}</Tag>}>
                    {Object.entries(evalWeights).slice(0, 8).map(([k, v], i) => (
                      <div key={i} style={{ display: 'grid', gridTemplateColumns: '140px 1fr 36px', alignItems: 'center', gap: 6, marginTop: 6 }}>
                        <span className="t-xs">{EVAL_WEIGHT_KEYS[k] ? t(EVAL_WEIGHT_KEYS[k]) : k.replace(/_/g, ' ').toUpperCase()}</span>
                        <Bar value={v * 100} />
                        <span className="t-xs muted text-right">{v}</span>
                      </div>
                    ))}
                  </Panel>
                )}

                {/* Win lines */}
                {winLines.length > 0 && (
                  <Panel code="11.C" title={t('frg.p_winlines', { count: winLines.length })}>
                    {winLines.map((wl, i) => {
                      const kindMap = { finisher: 'bad', combat: 'warn', commander_damage: 'ok', combo: 'bad', synergy: null }
                      const symbols = ['α', 'β', 'γ', 'δ', 'ε', 'ζ']
                      return (
                        <div key={i} className="winline-row" style={{ padding: '10px 0', borderBottom: i < winLines.length - 1 ? '1px dashed var(--rule-2)' : 'none' }}>
                          <div style={{ fontSize: 24, fontWeight: 700, color: kindMap[wl.type] === 'bad' ? 'var(--danger)' : kindMap[wl.type] === 'warn' ? 'var(--warn)' : kindMap[wl.type] === 'ok' ? 'var(--ok)' : 'var(--ink)' }}>
                            {symbols[i] || '.'}
                          </div>
                          <Tag kind={kindMap[wl.type]} solid>{wl.type?.toUpperCase()}</Tag>
                          <div>
                            <div className="t-md" style={{ fontWeight: 700 }}>{wl.pieces?.join(' + ')}</div>
                            {wl.tutor_paths && (
                              <div className="t-xs muted" style={{ marginTop: 2 }}>
                                {t('frg.tutors', { list: wl.tutor_paths.map(p => p.tutor).join(', ') })}
                              </div>
                            )}
                          </div>
                        </div>
                      )
                    })}
                  </Panel>
                )}

                {/* Value engine. Reads the deck archive's key deliberately:
                    both screens render this same panel off the same
                    analysis.value_engine_keys field, and holding one sentence
                    in two keys is what let one of them keep the English
                    'VALUE ENGINE' in 18 locales. One key cannot drift from
                    itself. */}
                {valueKeys.length > 0 && (
                  <Panel code="11.D" title={t('dka.value_engine', { count: valueKeys.length })}>
                    <div className="grid col-5 gap-2">
                      {valueKeys.slice(0, 10).map((name, i) => (
                        <div key={i} className="panel" style={{ padding: 0 }}>
                          <div style={{ aspectRatio: '5/7', borderBottom: '1px solid var(--rule-2)', position: 'relative', overflow: 'hidden' }}>
                            <img src={cardArtUrl(name)} alt={name} style={{ width: '100%', height: '100%', objectFit: 'cover', filter: 'saturate(0.6) contrast(1.1)' }} onError={e => { e.target.style.display = 'none'; e.target.parentElement.classList.add('hatch') }} />
                          </div>
                          <div style={{ padding: '5px 7px' }}>
                            <div style={{ fontSize: 9, fontWeight: 700, letterSpacing: '0.04em', textTransform: 'uppercase', lineHeight: 1.2, minHeight: 24 }}>{name}</div>
                          </div>
                        </div>
                      ))}
                    </div>
                  </Panel>
                )}
              </>
            ) : (
              <Panel code="11.B" title={t('frg.p_no_analysis')}>
                <div style={{ padding: '20px 0', textAlign: 'center' }}>
                  <div className="t-md muted" style={{ lineHeight: 1.8, textTransform: 'uppercase', letterSpacing: '0.04em' }}>
                    {/* fill() so the tool name can sit wherever the target
                        language puts it, not where English does. */}
                    &gt; {t('frg.na_1')}<br />
                    &gt; {fill(t('frg.na_2'), { tool: <span style={{ color: 'var(--ink)' }}>HEXDEK-FREYA</span> })}<br />
                    &gt; {t('frg.na_3')}<span className="blink">_</span>
                  </div>
                </div>
              </Panel>
            )}

            {/* Card list */}
            {cards.length > 0 && (
              <Panel code="11.E" title={t('frg.p_cards', { count: cards.length })}>
                <div>
                  {cards.map((c, i) => (
                    <div key={i} style={{ display: 'flex', justifyContent: 'space-between', padding: '3px 0', borderBottom: i < cards.length - 1 ? '1px dotted var(--rule)' : 'none' }}>
                      <span className="t-xs">{c.name}</span>
                      <span className="t-xs muted">{c.quantity > 1 ? `x${c.quantity}` : ''}</span>
                    </div>
                  ))}
                </div>
              </Panel>
            )}

            {/* Gauntlet controls + results */}
            <Panel code="11.F" title={t('frg.p_actions')}>
              {gauntletError && (
                <div className="t-xs" style={{ color: 'var(--danger)', marginBottom: 8 }}>
                  &gt; {t('frg.err_prefix', { error: gauntletError === START_FAILED ? t(START_FAILED) : gauntletError })}
                </div>
              )}

              {(!gauntlet || gauntlet.status === 'none' || gauntlet.status === 'complete' || gauntlet.status === 'error') && (
                <>
                  <div className="t-xs muted" style={{ marginBottom: 10, lineHeight: 1.6 }}>
                    &gt; {t('frg.help_1')}<br />
                    &gt; {t('frg.help_2')}<span className="blink">_</span>
                  </div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 10 }}>
                    <span className="t-xs">{t('frg.games_label')}</span>
                    {[500, 1000, 5000, 10000].map(n => (
                      <Tag key={n} solid={gauntletGames === n} onClick={() => setGauntletGames(n)} style={{ cursor: 'pointer' }}>
                        {n >= 1000 ? `${n / 1000}K` : n}
                      </Tag>
                    ))}
                  </div>
                </>
              )}

              <ContextBox id="forge.run-actions">
                {fill(t('frg.ctx_run'), { button: <strong>{t('frg.btn_run')}</strong> })}
                {' '}{fill(t('frg.ctx_export'), { button: <strong>{t('frg.btn_export')}</strong> })}
              </ContextBox>
              <div className="flex gap-2">
                {(!gauntlet || gauntlet.status === 'none' || gauntlet.status === 'complete' || gauntlet.status === 'error') && (
                  <Btn solid arrow="▶" onClick={handleStartGauntlet}>{t('frg.btn_run')}</Btn>
                )}
                {gauntlet?.status === 'running' && (
                  <Btn solid arrow="◼" disabled>{t('frg.btn_running', { done: gauntlet.games || 0, total: gauntlet.target || gauntletGames })}</Btn>
                )}
                <Btn ghost arrow="↗" onClick={() => {
                  if (!cards.length) return
                  const lines = cards.map(c => c.quantity > 1 ? `${c.quantity} ${c.name}` : `1 ${c.name}`)
                  const blob = new Blob([lines.join('\n')], { type: 'text/plain' })
                  const url = URL.createObjectURL(blob)
                  const a = document.createElement('a')
                  a.href = url
                  a.download = `${selectedDeck.id || 'deck'}.txt`
                  a.click()
                  URL.revokeObjectURL(url)
                }}>{t('frg.btn_export')}</Btn>
              </div>
            </Panel>

            {/* Gauntlet progress / results */}
            {gauntlet && gauntlet.status === 'running' && (
              <Panel code="11.G" title={t('frg.p_gauntlet', { name: gauntlet.commander || deckName })} right={<Tag>{t('frg.tag_running')}</Tag>}>
                <div style={{ marginBottom: 10 }}>
                  <Bar value={gauntlet.games && gauntlet.target ? (gauntlet.games / gauntlet.target) * 100 : 0} />
                </div>
                <KV rows={[
                  [t('frg.kv_progress'), t('frg.progress_val', { done: gauntlet.games || 0, total: gauntlet.target || gauntletGames })],
                  [t('frg.kv_wins'), `${gauntlet.wins || 0}`],
                  [t('frg.kv_losses'), `${gauntlet.losses || 0}`],
                  [t('frg.kv_winrate'), `${gauntlet.win_rate || 0}%`],
                ]} />
                <div className="t-xs muted" style={{ marginTop: 10 }}>
                  &gt; {t('frg.streaming')}<span className="blink">_</span>
                </div>
              </Panel>
            )}

            {gauntlet && gauntlet.status === 'complete' && (
              <Panel code="11.G" title={t('frg.p_results', { name: gauntlet.commander || deckName })} right={<Tag solid>{t('frg.tag_complete')}</Tag>}>
                <KV rows={[
                  [t('frg.kv_games'), `${gauntlet.games}`],
                  [t('frg.kv_wins_losses'), `${gauntlet.wins} / ${gauntlet.losses}`],
                  [t('frg.kv_winrate'), `${gauntlet.win_rate}%`],
                  [t('frg.kv_elo_delta'), `${gauntlet.elo_delta >= 0 ? '+' : ''}${gauntlet.elo_delta}`],
                  ['ELO', `${gauntlet.elo_start} → ${gauntlet.elo_end}`],
                  [t('frg.kv_avg_turns'), `${gauntlet.avg_turns}`],
                ]} />
                {(gauntlet.top_beaten?.length > 0 || gauntlet.top_lost_to?.length > 0) && (
                  <div style={{ marginTop: 12, display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
                    {gauntlet.top_beaten?.length > 0 && (
                      <div>
                        <div className="t-xs" style={{ fontWeight: 700, marginBottom: 4 }}>{t('frg.top_beaten')}</div>
                        {gauntlet.top_beaten.map((b, i) => (
                          <div key={i} className="t-xs muted" style={{ lineHeight: 1.6 }}>{b}</div>
                        ))}
                      </div>
                    )}
                    {gauntlet.top_lost_to?.length > 0 && (
                      <div>
                        <div className="t-xs" style={{ fontWeight: 700, marginBottom: 4 }}>{t('frg.top_lost_to')}</div>
                        {gauntlet.top_lost_to.map((l, i) => (
                          <div key={i} className="t-xs muted" style={{ lineHeight: 1.6 }}>{l}</div>
                        ))}
                      </div>
                    )}
                  </div>
                )}
              </Panel>
            )}

            {gauntlet && gauntlet.status === 'error' && (
              <Panel code="11.G" title={t('frg.p_gauntlet_error')}>
                <div className="t-xs" style={{ color: 'var(--danger)', lineHeight: 1.6 }}>
                  &gt; {t('frg.ge_1')}<br />
                  &gt; {t('frg.ge_2')}<span className="blink">_</span>
                </div>
              </Panel>
            )}
          </>
        )}

        {!loading && !selectedDeck && (
          <div style={{ padding: 36, textAlign: 'center' }}>
            <div className="t-md muted" style={{ lineHeight: 1.8, textTransform: 'uppercase', letterSpacing: '0.04em' }}>
              &gt; {t('frg.empty_1')}<br />
              &gt; {t('frg.empty_2')}<span className="blink">_</span>
            </div>
          </div>
        )}
      </div>
    </>
  )
}
