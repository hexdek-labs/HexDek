import { useState, useEffect } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { Panel, KV, Bar, Tag, Btn, Tape, ConfidenceDots, ManaCurveChart, ColorPie, computeColorByCmc } from '../components/chrome'
import { api, cardArtUrl } from '../services/api'
import { useLiveSocket } from '../hooks/useLiveSocket'
import { useAuth } from '../context/AuthContext'
import { trackEvent } from '../hooks/useAnalytics'
import { MOCK_DECK_ANALYSIS } from '../services/mock'

const CardThumb = ({ name, cmc, score, compact }) => {
  const imgUrl = cardArtUrl(name)
  if (compact) {
    return (
      <div className="panel" style={{ padding: 0 }}>
        <div style={{ aspectRatio: '5/4', position: 'relative', overflow: 'hidden' }}>
          <img src={imgUrl} alt={name} style={{ width: '100%', height: '100%', objectFit: 'cover', filter: 'saturate(0.6) contrast(1.1)' }} onError={e => { e.target.style.display = 'none'; e.target.parentElement.classList.add('hatch') }} />
        </div>
        <div style={{ padding: '3px 5px' }}>
          <div style={{ fontSize: 7, fontWeight: 700, letterSpacing: '0.04em', textTransform: 'uppercase', lineHeight: 1.1, minHeight: 14, overflow: 'hidden', textOverflow: 'ellipsis' }}>{name}</div>
        </div>
      </div>
    )
  }
  return (
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
  const [saving, setSaving] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)
  const [versions, setVersions] = useState([])
  const [gauntlet, setGauntlet] = useState(null)
  const { elo } = useLiveSocket()
  const { user } = useAuth()

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
        setTimeout(() => fetchAnalysis(ownerId, deckId), 3000)
      } else {
        setAnalysis(data)
        setAnalyzing(false)
      }
    }).catch(() => setAnalyzing(false))
  }

  useEffect(() => {
    if (!owner || !id) {
      setAnalysis(MOCK_DECK_ANALYSIS.tinybones)
      setLoading(false)
      return
    }
    Promise.allSettled([
      api.getDeck(`${owner}/${id}`),
      api.getDeckAnalysis(`${owner}/${id}`),
      api.getGauntlet(`${owner}/${id}`),
    ]).then(([deckRes, analysisRes, gauntletRes]) => {
      if (deckRes.status === 'fulfilled') setDeck(deckRes.value)
      if (analysisRes.status === 'fulfilled') {
        const data = analysisRes.value
        if (data.status === 'analyzing') {
          setAnalyzing(true)
          setTimeout(() => fetchAnalysis(owner, id), 3000)
        } else {
          setAnalysis(data)
        }
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

  const deckName = deck?.commander || id?.replace(/_/g, ' ').toUpperCase() || 'DECK'
  const cardCount = deck?.card_count || deck?.cards?.length || 99
  const userBracket = deck?.bracket || '?'
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
    <>
      <Tape left={`DECK ARCHIVE / / ${owner?.toUpperCase()} / / ${deckName}`} mid={pls ? `Plays Like B${pls} (Bracket B${wbs})` : `Bracket B${wbs}`} right="EXPORT ↗ ANALYZE ↗" />

      <div className="archive-layout">
        <div className="archive-sidebar">
          <Panel code="04.A" title="COMMANDER SPECIMEN" solid>
            <div style={{ aspectRatio: '5/7', position: 'relative', border: '1px solid var(--rule-2)', overflow: 'hidden' }} className={cmdrImageUrl ? '' : 'hatch'}>
              {cmdrImageUrl ? (
                <img src={cmdrImageUrl} alt={cmdrCardName} style={{ width: '100%', height: '100%', objectFit: 'cover', filter: 'saturate(0.7) contrast(1.1)' }} />
              ) : (
                <div style={{ position: 'absolute', inset: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 64, color: 'var(--ink-3)', fontWeight: 800 }}>◇</div>
              )}
              <span style={{ position: 'absolute', top: 6, left: 6, background: 'rgba(12,13,10,0.7)', padding: '1px 4px' }} className="t-xs muted-2">CMDR.PORTRAIT</span>
            </div>
            <div style={{ marginTop: 10 }}>
              <div className="t-xl" style={{ fontWeight: 700, lineHeight: 1.1 }}>{deckName}</div>
              {cmdrCardName && cmdrCardName.toUpperCase() !== deckName && (
                <div className="t-xs" style={{ marginTop: 4, color: 'var(--ink-2)' }}>{cmdrCardName}</div>
              )}
              <div className="t-xs muted" style={{ marginTop: 4 }}>{owner?.toUpperCase()} / / {id}</div>
            </div>
            <div className="hr" style={{ margin: '10px 0' }} />
            <KV rows={[
              ['OWNER', <Link to={`/decks?q=${owner}`} style={{ color: 'var(--ink)', textDecoration: 'none', borderBottom: '1px dotted var(--ink-3)' }}>{owner?.toUpperCase()}</Link>],
              ['CARDS', `${cardCount}`],
              ['BRACKET', `B${wbs}${wbsLabel ? ' ' + wbsLabel : ''}`],
              ['PLAYS LIKE', pls ? `B${pls}${plsLabel ? ' ' + plsLabel : ''}${pls != wbs ? ' ⬆' : ''}` : '—'],
              ['GAME CHANGERS', gameChangers != null ? `${gameChangers}` : '—'],
              ['ARCHETYPE', archetype],
            ]} />
            {deckElo && (
              <>
                <div className="hr" style={{ margin: '10px 0' }} />
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
                  <span className="t-xs muted">CONFIDENCE</span>
                  <ConfidenceDots games={deckElo.games} showLabel size="lg" />
                </div>
                <KV rows={[
                  ['HexELO', <span className="punch">{Math.round(deckElo.rating)}</span>],
                  ['RECORD', <span><span style={{ color: 'var(--ok)' }}>{deckElo.wins}W</span> — <span style={{ color: 'var(--danger)' }}>{deckElo.losses}L</span></span>],
                  ['WIN RATE', `${deckElo.win_rate}%`],
                  ['GAMES', `${deckElo.games?.toLocaleString()}`],
                  ['DELTA', <span style={{ color: deckElo.delta >= 0 ? 'var(--ok)' : 'var(--danger)' }}>{deckElo.delta >= 0 ? '+' : ''}{Math.round(deckElo.delta)}</span>],
                ]} />
              </>
            )}
            <div className="hr" style={{ margin: '10px 0' }} />
            <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
              {owner && id && (
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
                }}>EDIT DECK</Btn>
              )}
              <Btn ghost arrow="↗" onClick={() => {
                if (!cards.length) return
                const lines = cards.map(c => c.quantity > 1 ? `${c.quantity} ${c.name}` : `1 ${c.name}`)
                const blob = new Blob([lines.join('\n')], { type: 'text/plain' })
                const url = URL.createObjectURL(blob)
                const a = document.createElement('a')
                a.href = url
                a.download = `${id || 'deck'}.txt`
                a.click()
                URL.revokeObjectURL(url)
              }}>EXPORT .TXT</Btn>
              <Btn ghost arrow="↗" onClick={() => {
                if (!owner || !id) return
                setAnalyzing(true)
                trackEvent('run_freya', { deck: `${owner}/${id}` })
                api.runAnalysis(`${owner}/${id}`).then(() => {
                  setTimeout(() => fetchAnalysis(owner, id), 3000)
                }).catch(() => setAnalyzing(false))
              }}>{analyzing ? 'ANALYZING...' : 'RUN FREYA'}</Btn>
              {owner && id && (
                <>
                  <div className="hr" style={{ margin: '4px 0' }} />
                  {!confirmDelete ? (
                    <Btn ghost onClick={() => setConfirmDelete(true)} style={{ color: 'var(--danger)', borderColor: 'var(--danger)' }}>DELETE DECK</Btn>
                  ) : (
                    <div style={{ display: 'flex', gap: 6 }}>
                      <Btn solid onClick={() => {
                        api.deleteDeck(`${owner}/${id}`).then(() => navigate('/decks')).catch(() => setConfirmDelete(false))
                      }} style={{ flex: 1, background: 'var(--danger)', borderColor: 'var(--danger)' }}>CONFIRM</Btn>
                      <Btn ghost onClick={() => setConfirmDelete(false)} style={{ flex: 1 }}>CANCEL</Btn>
                    </div>
                  )}
                </>
              )}
            </div>
          </Panel>

          {cards.length > 0 && (
            <Panel code="04.B" title={`CARD LIST / / ${cards.length} ENTRIES`}>
              <div style={{ maxHeight: 300, overflow: 'auto' }}>
                {cards.map((c, i) => (
                  <div key={i} style={{ display: 'flex', justifyContent: 'space-between', padding: '3px 0', borderBottom: i < cards.length - 1 ? '1px dotted var(--rule)' : 'none' }}>
                    <span className="t-xs">{c.name}</span>
                    <span className="t-xs muted">{c.quantity > 1 ? `×${c.quantity}` : ''}</span>
                  </div>
                ))}
              </div>
            </Panel>
          )}
        </div>

        <div className="archive-main">
          {/* Edit mode */}
          {editing && (
            <Panel code="04.X" title="EDIT DECK LIST" right={
              <span className="t-xs" style={{ color: 'var(--warn)' }}>EDITING</span>
            }>
              <textarea
                value={editText}
                onChange={e => setEditText(e.target.value)}
                style={{
                  width: '100%', minHeight: 300, padding: 10,
                  background: 'var(--bg-2, rgba(0,0,0,0.3))', border: '1px solid var(--rule-2)',
                  color: 'var(--ink)', fontFamily: 'inherit', fontSize: 11,
                  letterSpacing: '0.04em', lineHeight: 1.6, resize: 'vertical',
                }}
                spellCheck={false}
              />
              <div style={{ display: 'flex', gap: 8, marginTop: 10 }}>
                <Btn solid onClick={() => {
                  if (!editText.trim() || saving) return
                  setSaving(true)
                  api.updateDeck(`${owner}/${id}`, editText).then(updated => {
                    setEditing(false)
                    setSaving(false)
                    api.getDeck(`${owner}/${id}`).then(setDeck)
                    api.getDeckVersions(`${owner}/${id}`).then(setVersions).catch(() => {})
                  }).catch(() => setSaving(false))
                }}>{saving ? 'SAVING...' : 'SAVE UPDATE'}</Btn>
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
          )}

          {/* Strategy summary */}
          <Panel code="04.C" title="FREYA / / ENGINE ANALYSIS" right={<Tag solid>Bracket B{wbs}{pls && pls !== wbs ? ` → Plays Like B${pls}` : ''}</Tag>}>
            {!analysis ? (
              <div style={{ padding: '20px 0', textAlign: 'center' }}>
                <div className="t-md muted" style={{ lineHeight: 1.8, textTransform: 'uppercase', letterSpacing: '0.04em' }}>
                  {analyzing ? (
                    <>&gt; FREYA ENGINE ANALYZING DECK<span className="blink">_</span><br />&gt; DETECTING COMBOS, SYNERGIES, WIN LINES...<br />&gt; THIS MAY TAKE A FEW SECONDS</>
                  ) : (
                    <>&gt; NO FREYA ANALYSIS ON FILE<br />&gt; RUN <span style={{ color: 'var(--ink)' }}>HEXDEK-FREYA</span> TO GENERATE STRATEGY REPORT<br />&gt; BRACKET, ARCHETYPE, WIN LINES, EVAL WEIGHTS<span className="blink">_</span></>
                  )}
                </div>
              </div>
            ) : (
              <div className="analysis-grid">
                <div>
                  <div className="t-xs muted">ARCHETYPE</div>
                  <div className="t-2xl" style={{ fontWeight: 700, marginTop: 2 }}>{archetype}</div>
                  {summary && (
                    <div className="t-md muted" style={{ marginTop: 10, lineHeight: 1.6, textTransform: 'uppercase', letterSpacing: '0.04em' }}>
                      &gt; {summary}
                    </div>
                  )}
                </div>
                <div className="analysis-weights">
                  <div className="t-xs muted">EVAL WEIGHTS</div>
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
          {winLines.length > 0 && (
            <Panel code="04.D" title={`WIN LINES / / ${winLines.length} DETECTED`}>
              {winLines.map((wl, i) => {
                const kindMap = { finisher: 'bad', combat: 'warn', commander_damage: 'ok', combo: 'bad', synergy: null }
                const symbols = ['α', 'β', 'γ', 'δ', 'ε', 'ζ']
                return (
                  <div key={i} className="winline-row" style={{ padding: '10px 0', borderBottom: i < winLines.length - 1 ? '1px dashed var(--rule-2)' : 'none' }}>
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
            </Panel>
          )}

          {/* Value engine keys */}
          {valueKeys.length > 0 && (
            <Panel code="04.E" title={`VALUE ENGINE / / ${valueKeys.length} KEY CARDS`}>
              <div className="grid col-5 gap-2">
                {valueKeys.slice(0, 10).map((name, i) => (
                  <CardThumb key={i} name={name} />
                ))}
              </div>
            </Panel>
          )}

          {/* Tutor targets */}
          {analysis?.tutor_targets && (
            <Panel code="04.F" title="TUTOR TARGETS">
              <KV rows={analysis.tutor_targets.map((t, i) => [`TARGET.${i + 1}`, t])} />
            </Panel>
          )}

          {/* Gauntlet */}
          {gauntlet && gauntlet.status !== 'none' && (
            <Panel code="04.G" title="GAUNTLET REPORT" right={
              <Tag solid kind={gauntlet.status === 'complete' ? 'ok' : null}>
                {gauntlet.status === 'running' ? `${gauntlet.games}/${gauntlet.target}` : gauntlet.status?.toUpperCase()}
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
                  <div className="grid col-3" style={{ gap: 14, marginBottom: 14 }}>
                    <div>
                      <div className="t-xs muted">WIN RATE</div>
                      <div className="t-2xl" style={{ fontWeight: 700, color: gauntlet.win_rate >= 25 ? 'var(--ok)' : 'var(--danger)' }}>{gauntlet.win_rate}%</div>
                    </div>
                    <div>
                      <div className="t-xs muted">RECORD</div>
                      <div className="t-2xl" style={{ fontWeight: 700 }}><span style={{ color: 'var(--ok)' }}>{gauntlet.wins}W</span> — <span style={{ color: 'var(--danger)' }}>{gauntlet.losses}L</span></div>
                    </div>
                    <div>
                      <div className="t-xs muted">ELO DELTA</div>
                      <div className="t-2xl" style={{ fontWeight: 700, color: gauntlet.elo_delta >= 0 ? 'var(--ok)' : 'var(--danger)' }}>
                        {gauntlet.elo_delta >= 0 ? '+' : ''}{Math.round(gauntlet.elo_delta)}
                      </div>
                    </div>
                  </div>
                  <KV rows={[
                    ['GAMES', `${gauntlet.games?.toLocaleString()}`],
                    ['AVG TURNS', `${gauntlet.avg_turns}`],
                    ['ELO', `${gauntlet.elo_start} → ${gauntlet.elo_end}`],
                  ]} />
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
              ) : null}
            </Panel>
          )}

          {/* Actions */}
          {owner && id && (
            <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap' }}>
              <Btn solid arrow="▶" onClick={() => {
                if (gauntlet?.status === 'running') return
                trackEvent('start_gauntlet', { deck: `${owner}/${id}`, games: 10000 })
                api.startGauntlet(`${owner}/${id}`, 10000).then(() => {
                  const poll = () => {
                    api.getGauntlet(`${owner}/${id}`).then(r => {
                      setGauntlet(r)
                      if (r.status === 'running') setTimeout(poll, 3000)
                    })
                  }
                  setTimeout(poll, 2000)
                })
                setGauntlet({ status: 'running', games: 0, target: 10000, win_rate: 0 })
              }}>{gauntlet?.status === 'running' ? 'GAUNTLET RUNNING...' : 'RUN GAUNTLET (10K)'}</Btn>
              <Btn arrow="▶">TEST VARIANT</Btn>
              <Btn ghost arrow="↗">DIFF BUILDS</Btn>
            </div>
          )}
        </div>
      </div>
    </>
  )
}
