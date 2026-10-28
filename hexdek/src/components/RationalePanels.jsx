import { useState } from 'react'
import { Panel, Tag } from './chrome'
import CardLink from './CardLink'
import { useT } from '../i18n'

// Shared brutalist row styling used by all three rationale panels.
const rowBase = {
  borderTop: '1px dashed var(--rule-2)',
  padding: '8px 0',
}

const headerBase = {
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'center',
  gap: 8,
  cursor: 'pointer',
  userSelect: 'none',
}

const detailGrid = {
  display: 'grid',
  gridTemplateColumns: '90px 1fr',
  gap: '4px 12px',
  marginTop: 8,
  paddingLeft: 16,
  borderLeft: '2px solid var(--rule-2)',
}

const detailLabel = {
  fontSize: 9,
  fontWeight: 700,
  letterSpacing: '0.08em',
  textTransform: 'uppercase',
  color: 'var(--ink-2)',
  paddingTop: 2,
}

function DetailRow({ label, children }) {
  if (children == null) return null
  if (Array.isArray(children) && children.length === 0) return null
  return (
    <>
      <span style={detailLabel}>{label}</span>
      <span className="t-xs" style={{ lineHeight: 1.5 }}>{children}</span>
    </>
  )
}

function Caret({ open }) {
  return (
    <span style={{
      fontSize: 10,
      color: 'var(--ink-2)',
      width: 14,
      textAlign: 'center',
      transition: 'transform 0.15s',
      transform: open ? 'rotate(90deg)' : 'rotate(0deg)',
    }}>▶</span>
  )
}

// ---------------------------------------------------------------------------
// CONSIDER CUTTING — explains why each cuttable card was flagged.
// ---------------------------------------------------------------------------

// `onCut` (optional) — invoked with the card name when a row's CUT
// action is clicked. When set, the CUT tag becomes a proper button
// that opens the workshop with the card removed. When unset (e.g.
// public viewer who can't edit this deck), CUT renders as a passive
// flag and the click is a no-op.
export function ConsiderCuttingRationale({ cuts, onCut }) {
  const t = useT()
  const [openIdx, setOpenIdx] = useState(null)
  const items = (cuts || []).filter(c => c && (c.name || typeof c === 'string'))
  if (items.length === 0) return null

  return (
    <Panel
      code="04.QR"
      title={t('rat.title_cuts', { count: items.length })}
      right={<Tag kind="warn">{t('rat.tag_rationale')}</Tag>}
    >
      <div className="t-xs muted" style={{ marginBottom: 6 }}>
        {t('rat.intro_cuts')}
      </div>
      {items.map((c, i) => {
        // Tolerate both legacy string entries and structured objects so the
        // UI keeps rendering even if an older strategy.json is on disk.
        const name = typeof c === 'string' ? c : c.name
        const reason = typeof c === 'string' ? '' : (c.reason || '')
        const detected = typeof c === 'string' ? '' : (c.detected || '')
        const whyCut = typeof c === 'string' ? '' : (c.why_cut || '')
        const effect = typeof c === 'string' ? '' : (c.effect || '')
        const suggested = typeof c === 'string' ? [] : (c.suggested || [])
        const open = openIdx === i
        const hasDetail = detected || whyCut || effect || (suggested && suggested.length > 0)
        return (
          <div key={i} style={rowBase}>
            <div
              style={headerBase}
              onClick={() => hasDetail && setOpenIdx(open ? null : i)}
            >
              <span style={{ display: 'flex', alignItems: 'center', gap: 8, minWidth: 0 }}>
                {hasDetail && <Caret open={open} />}
                <CardLink name={name} className="t-md" style={{ fontWeight: 700, borderBottom: 'none' }}>
                  {name}
                </CardLink>
                {reason && (
                  <span className="t-xs muted" style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    — {reason}
                  </span>
                )}
              </span>
              {onCut ? (
                <button
                  type="button"
                  className="cut-action"
                  onClick={(e) => { e.stopPropagation(); onCut(name) }}
                  title={t('rat.cut_title', { name })}
                  style={{
                    cursor: 'pointer',
                    background: 'transparent',
                    border: '1px solid var(--warn)',
                    color: 'var(--warn)',
                    padding: '2px 8px',
                    fontSize: 9,
                    fontWeight: 700,
                    letterSpacing: '0.08em',
                    fontFamily: 'inherit',
                  }}
                >
                  {t('rat.cut_action')}
                </button>
              ) : (
                <Tag kind="warn">{t('rat.cut_flag')}</Tag>
              )}
            </div>
            {open && hasDetail && (
              <div style={detailGrid}>
                <DetailRow label={t('rat.lbl_detected')}>{detected}</DetailRow>
                <DetailRow label={t('rat.lbl_why')}>{whyCut}</DetailRow>
                <DetailRow label={t('rat.lbl_if_cut')}>{effect}</DetailRow>
                {suggested && suggested.length > 0 && (
                  <DetailRow label={t('rat.lbl_swap_for')}>
                    <ul style={{ margin: 0, paddingLeft: 16 }}>
                      {suggested.map((s, si) => <li key={si}>{s}</li>)}
                    </ul>
                  </DetailRow>
                )}
              </div>
            )}
          </div>
        )
      })}
    </Panel>
  )
}

// ---------------------------------------------------------------------------
// VALUE ENGINE — explains why each engine was identified for this deck.
// ---------------------------------------------------------------------------

const REDUNDANCY_KIND = { HIGH: 'ok', MEDIUM: null, LOW: 'warn' }
const RECURSION_KIND = { infinite: 'ok', deep: 'ok', shallow: null, none: null }

// Parallel catalog-PATH maps beside the colour maps above -- the shape
// PublicProfile uses for RARITY_KEY beside RARITY_COLOR. These hold WHOLE
// phrases rather than an adjective to be glued onto a noun: a magnitude
// adjective has to agree with its noun's gender and number in most of the 25
// (de HOHE REDUNDANZ), and the word order differs too.
//
// `crd.role_recursion` reads 'RECURSION' in English and is NOT reused here:
// its translations are the GRAVEYARD-recursion card role (ja リアニメイト,
// sv ÅTERVINNING, zh 回收), not this file's computer-science sense of a chain
// that loops back on itself.
const REDUNDANCY_KEY = { HIGH: 'rat.red_high', MEDIUM: 'rat.red_medium', LOW: 'rat.red_low' }
const RECURSION_KEY = { infinite: 'rat.rec_infinite', deep: 'rat.rec_deep', shallow: 'rat.rec_shallow' }

export function ValueEngineRationale({ chains }) {
  const t = useT()
  const [openIdx, setOpenIdx] = useState(0) // first chain expanded by default
  const items = (chains || []).filter(Boolean)
  if (items.length === 0) return null

  return (
    <Panel
      code="04.ER"
      title={t('rat.title_engines', { count: items.length })}
      right={<Tag kind="ok">{t('rat.tag_rationale')}</Tag>}
    >
      <div className="t-xs muted" style={{ marginBottom: 6 }}>
        {t('rat.intro_engines')}
      </div>
      {items.map((chain, i) => {
        const open = openIdx === i
        const rationale = chain.rationale || {}
        const totalPieces = (chain.steps || []).reduce((s, st) => s + (st.cards?.length || 0), 0)
        return (
          <div key={i} style={rowBase}>
            <div style={headerBase} onClick={() => setOpenIdx(open ? -1 : i)}>
              <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <Caret open={open} />
                <span className="t-md" style={{ fontWeight: 700 }}>{chain.name}</span>
                <span className="t-xs muted">{t('rat.steps_pieces', { steps: chain.depth, pieces: totalPieces })}</span>
              </span>
              <span style={{ display: 'flex', gap: 4 }}>
                {chain.redundancy && (
                  <Tag kind={REDUNDANCY_KIND[chain.redundancy] ?? null}>
                    {REDUNDANCY_KEY[chain.redundancy]
                      ? t(REDUNDANCY_KEY[chain.redundancy])
                      : `${chain.redundancy} REDUNDANCY`}
                  </Tag>
                )}
                {chain.recursion_depth && chain.recursion_depth !== 'none' && (
                  <Tag kind={RECURSION_KIND[chain.recursion_depth] ?? null}>
                    {RECURSION_KEY[chain.recursion_depth]
                      ? t(RECURSION_KEY[chain.recursion_depth])
                      : `${chain.recursion_depth.toUpperCase()} RECURSION`}
                  </Tag>
                )}
              </span>
            </div>
            {open && (
              <div style={detailGrid}>
                <DetailRow label={t('rat.lbl_trigger')}>{rationale.trigger}</DetailRow>
                <DetailRow label={t('rat.lbl_how')}>{rationale.how_it_works}</DetailRow>
                {rationale.key_pieces && rationale.key_pieces.length > 0 && (
                  <DetailRow label={t('rat.lbl_key_pieces')}>
                    <span style={{ display: 'inline-flex', flexWrap: 'wrap', gap: 4 }}>
                      {rationale.key_pieces.map((kp, ki) => (
                        <CardLink key={ki} name={kp} className="t-xs" style={{ fontWeight: 700 }}>
                          {kp}
                        </CardLink>
                      ))}
                    </span>
                  </DetailRow>
                )}
                {(chain.steps || []).map((step, si) => (
                  <ChainStepRow key={si} step={step} weakest={si === chain.weakest_link} />
                ))}
              </div>
            )}
          </div>
        )
      })}
    </Panel>
  )
}

function ChainStepRow({ step, weakest }) {
  const t = useT()
  const cards = step.cards || []
  return (
    <>
      <span style={{ ...detailLabel, color: weakest ? 'var(--warn)' : 'var(--ink-2)' }}>
        [{step.label}]
      </span>
      <span className="t-xs" style={{ lineHeight: 1.5 }}>
        <span className="muted-2" style={{ fontSize: 9 }}>
          {step.from}→{step.to}{step.resource ? ` (${step.resource})` : ''}
        </span>
        <span style={{ marginLeft: 6 }}>
          {cards.length === 0 ? <span className="muted">{t('rat.none')}</span> :
            cards.slice(0, 8).map((c, ci) => (
              <span key={ci}>
                {ci > 0 && <span className="muted-2">, </span>}
                <CardLink name={c} className="t-xs" style={{ borderBottom: 'none' }}>{c}</CardLink>
              </span>
            ))}
          {cards.length > 8 && <span className="muted-2"> {t('rat.more', { count: cards.length - 8 })}</span>}
        </span>
        {weakest && <Tag kind="warn" style={{ marginLeft: 8 }}>{t('rat.weakest_link')}</Tag>}
      </span>
    </>
  )
}

// ---------------------------------------------------------------------------
// WIN CONDITION — explains the detection logic for each win line.
// ---------------------------------------------------------------------------

const WINLINE_KIND = {
  infinite: 'bad',
  determined: 'bad',
  finisher: 'warn',
  combat: 'warn',
  commander_damage: 'ok',
  alt_wincon: 'bad',
}

// Catalog paths for the same six wire values. `finisher` and
// `commander_damage` reuse existing standalone nouns rather than minting a
// second word for each.
const WINLINE_KEY = {
  infinite: 'rat.wl_infinite',
  determined: 'rat.wl_determined',
  finisher: 'crd.role_finisher',
  combat: 'rat.wl_combat',
  commander_damage: 'rpt.wc_cmdr',
  alt_wincon: 'rat.wl_alt',
}

export function WinConditionRationale({ winLines }) {
  const t = useT()
  const [openIdx, setOpenIdx] = useState(0)
  const items = (winLines || []).filter(Boolean)
  if (items.length === 0) return null

  return (
    <Panel
      code="04.WR"
      title={t('rat.title_winlines', { count: items.length })}
      right={<Tag kind="ok">{t('rat.tag_rationale')}</Tag>}
    >
      <div className="t-xs muted" style={{ marginBottom: 6 }}>
        {t('rat.intro_winlines')}
      </div>
      {items.map((wl, i) => {
        const open = openIdx === i
        const rationale = wl.rationale || {}
        const tutorCount = (wl.tutor_paths || []).length
        return (
          <div key={i} style={rowBase}>
            <div style={headerBase} onClick={() => setOpenIdx(open ? -1 : i)}>
              <span style={{ display: 'flex', alignItems: 'center', gap: 8, minWidth: 0, flex: 1 }}>
                <Caret open={open} />
                <Tag kind={WINLINE_KIND[wl.type] ?? null} solid>
                  {WINLINE_KEY[wl.type] ? t(WINLINE_KEY[wl.type]) : (wl.type || '').toUpperCase()}
                </Tag>
                {/* min-width: 0 + flex: 1 lets the title actually shrink
                    inside the flex parent so text-overflow:ellipsis kicks
                    in instead of overflowing past the panel edge. */}
                <span className="t-md" style={{ fontWeight: 700, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', minWidth: 0, flex: 1 }}>
                  {(wl.pieces || []).join(' + ')}
                </span>
              </span>
              {tutorCount > 0 && (
                <Tag>{t(tutorCount === 1 ? 'rat.tutor_paths_one' : 'rat.tutor_paths', { count: tutorCount })}</Tag>
              )}
            </div>
            {open && (
              <div style={detailGrid}>
                {rationale.forms && rationale.forms.length > 0 && (
                  <DetailRow label={t('rat.lbl_forms')}>
                    <span style={{ display: 'inline-flex', flexWrap: 'wrap', gap: 4 }}>
                      {rationale.forms.map((f, fi) => (
                        // Forms can be card names OR descriptive phrases (e.g.
                        // "10 creatures CMC 3+ in deck"). Only linkify when
                        // the string looks like a single card name.
                        /^[A-Z][^,]+$/.test(f) && f.length < 50
                          ? <CardLink key={fi} name={f} className="t-xs" style={{ fontWeight: 700 }}>{f}</CardLink>
                          : <span key={fi} className="t-xs">{f}</span>
                      )).reduce((acc, el, fi) => fi === 0 ? [el] : [...acc, <span key={`s${fi}`} className="muted-2">, </span>, el], [])}
                    </span>
                  </DetailRow>
                )}
                {rationale.conditions && rationale.conditions.length > 0 && (
                  <DetailRow label={t('rat.lbl_conditions')}>
                    <ul style={{ margin: 0, paddingLeft: 16 }}>
                      {rationale.conditions.map((c, ci) => <li key={ci}>{c}</li>)}
                    </ul>
                  </DetailRow>
                )}
                {rationale.resolves && rationale.resolves.length > 0 && (
                  <DetailRow label={t('rat.lbl_resolves')}>
                    <ul style={{ margin: 0, paddingLeft: 16 }}>
                      {rationale.resolves.map((r, ri) => <li key={ri}>{r}</li>)}
                    </ul>
                  </DetailRow>
                )}
                {!rationale.resolves && wl.description && (
                  <DetailRow label={t('rat.lbl_description')}>{wl.description}</DetailRow>
                )}
              </div>
            )}
          </div>
        )
      })}
    </Panel>
  )
}
