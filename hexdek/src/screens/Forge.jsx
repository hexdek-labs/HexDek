import { useState, useEffect } from 'react'
import { Panel, KV, Bar, Tag, Btn, Tape } from '../components/chrome'
import { api, cardArtUrl } from '../services/api'
import { useDecks } from '../hooks/useData'

export default function Forge() {
  const { data: decks, loading: decksLoading } = useDecks()
  const [selectedDeck, setSelectedDeck] = useState(null)
  const [deck, setDeck] = useState(null)
  const [analysis, setAnalysis] = useState(null)
  const [loading, setLoading] = useState(false)

  // Auto-select first deck when list loads
  useEffect(() => {
    if (!selectedDeck && decks.length > 0 && decks[0].owner && decks[0].id) {
      setSelectedDeck(decks[0])
    }
  }, [decks, selectedDeck])

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

  const deckName = deck?.commander || selectedDeck?.name || 'SELECT A DECK'
  const cardCount = deck?.card_count || deck?.cards?.length || 0
  const bracket = deck?.bracket || analysis?.bracket || '?'
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
        left="VARIANT FORGE / / DOC HX-1101"
        mid={selectedDeck ? `B${bracket}` : 'SELECT DECK'}
        right={selectedDeck ? `${selectedDeck.owner?.toUpperCase()} / / ${deckName}` : ''}
      />

      <div style={{ padding: 18, flex: 1, display: 'flex', flexDirection: 'column', gap: 14, overflow: 'auto' }}>
        {/* Deck selector */}
        <Panel code="11.SEL" title="SELECT DECK">
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', alignItems: 'center' }}>
            {decksLoading ? (
              <span className="t-xs muted">LOADING DECKS<span className="blink">_</span></span>
            ) : decks.length === 0 ? (
              <span className="t-xs muted">NO DECKS FOUND</span>
            ) : (
              decks.map((d) => (
                <Tag
                  key={`${d.owner}/${d.id}`}
                  solid={selectedDeck?.id === d.id && selectedDeck?.owner === d.owner}
                  onClick={() => setSelectedDeck(d)}
                  style={{ cursor: 'pointer' }}
                >
                  {d.owner?.toUpperCase()} / {d.name}
                </Tag>
              ))
            )}
          </div>
        </Panel>

        {loading && (
          <div className="t-md muted" style={{ textAlign: 'center', padding: 36 }}>
            &gt; LOADING DECK DATA<span className="blink">_</span>
          </div>
        )}

        {!loading && selectedDeck && (
          <>
            {/* Deck overview */}
            <Panel code="11.A" title={`${deckName} / / FORGE SUBJECT`}>
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
                  <KV rows={[
                    ['COMMANDER', deckName],
                    ['OWNER', selectedDeck.owner?.toUpperCase()],
                    ['CARDS', `${cardCount}`],
                    ['BRACKET', `${bracket}`],
                    ['ARCHETYPE', archetype],
                  ]} />
                  {summary && (
                    <div className="t-xs muted" style={{ marginTop: 10, lineHeight: 1.6, textTransform: 'uppercase', letterSpacing: '0.04em' }}>
                      &gt; {summary}
                    </div>
                  )}
                </div>
              </div>
            </Panel>

            {/* Analysis section — from Freya */}
            {analysis ? (
              <>
                {/* Eval weights */}
                {Object.keys(evalWeights).length > 0 && (
                  <Panel code="11.B" title="FREYA / / EVAL WEIGHTS" right={<Tag solid>BRACKET {bracket}</Tag>}>
                    {Object.entries(evalWeights).slice(0, 8).map(([k, v], i) => (
                      <div key={i} style={{ display: 'grid', gridTemplateColumns: '140px 1fr 36px', alignItems: 'center', gap: 6, marginTop: 6 }}>
                        <span className="t-xs">{k.replace(/_/g, ' ').toUpperCase()}</span>
                        <Bar value={v * 100} />
                        <span className="t-xs muted text-right">{v}</span>
                      </div>
                    ))}
                  </Panel>
                )}

                {/* Win lines */}
                {winLines.length > 0 && (
                  <Panel code="11.C" title={`WIN LINES / / ${winLines.length} DETECTED`}>
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
                                TUTORS: {wl.tutor_paths.map(t => t.tutor).join(', ')}
                              </div>
                            )}
                          </div>
                        </div>
                      )
                    })}
                  </Panel>
                )}

                {/* Value engine */}
                {valueKeys.length > 0 && (
                  <Panel code="11.D" title={`VALUE ENGINE / / ${valueKeys.length} KEY CARDS`}>
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
              <Panel code="11.B" title="FREYA / / ENGINE ANALYSIS">
                <div style={{ padding: '20px 0', textAlign: 'center' }}>
                  <div className="t-md muted" style={{ lineHeight: 1.8, textTransform: 'uppercase', letterSpacing: '0.04em' }}>
                    &gt; NO FREYA ANALYSIS ON FILE<br />
                    &gt; RUN <span style={{ color: 'var(--ink)' }}>MTGSQUAD-FREYA</span> TO GENERATE STRATEGY REPORT<br />
                    &gt; BRACKET, ARCHETYPE, WIN LINES, EVAL WEIGHTS<span className="blink">_</span>
                  </div>
                </div>
              </Panel>
            )}

            {/* Card list */}
            {cards.length > 0 && (
              <Panel code="11.E" title={`CARD LIST / / ${cards.length} ENTRIES`}>
                <div style={{ maxHeight: 300, overflow: 'auto' }}>
                  {cards.map((c, i) => (
                    <div key={i} style={{ display: 'flex', justifyContent: 'space-between', padding: '3px 0', borderBottom: i < cards.length - 1 ? '1px dotted var(--rule)' : 'none' }}>
                      <span className="t-xs">{c.name}</span>
                      <span className="t-xs muted">{c.quantity > 1 ? `x${c.quantity}` : ''}</span>
                    </div>
                  ))}
                </div>
              </Panel>
            )}

            {/* Action buttons — simulation features are placeholders */}
            <Panel code="11.F" title="FORGE ACTIONS">
              <div className="t-xs muted" style={{ marginBottom: 10, lineHeight: 1.6 }}>
                &gt; VARIANT SIMULATION ENGINE NOT YET CONNECTED.<br />
                &gt; USE THE DECK ARCHIVE TO VIEW FULL ANALYSIS.<span className="blink">_</span>
              </div>
              <div className="flex gap-2">
                <Btn solid arrow="▶" onClick={() => {}}>RUN GAUNTLET (SOON)</Btn>
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
                }}>EXPORT .TXT</Btn>
              </div>
            </Panel>
          </>
        )}

        {!loading && !selectedDeck && (
          <div style={{ padding: 36, textAlign: 'center' }}>
            <div className="t-md muted" style={{ lineHeight: 1.8, textTransform: 'uppercase', letterSpacing: '0.04em' }}>
              &gt; SELECT A DECK ABOVE TO BEGIN FORGE SESSION<br />
              &gt; ANALYSIS + CARD LIST + VARIANT TESTING<span className="blink">_</span>
            </div>
          </div>
        )}
      </div>
    </>
  )
}
