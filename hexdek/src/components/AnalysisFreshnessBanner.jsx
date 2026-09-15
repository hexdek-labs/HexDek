import { useEffect, useRef, useState } from 'react'
import { Btn, Tag } from './chrome'
import { api } from '../services/api'

// AnalysisFreshnessBanner — tells the user when the Freya analysis they
// are looking at was produced by an older engine build, and when cards
// in their deck were skipped because the card database doesn't know
// them yet.
//
// Both signals ride along on the GET /api/decks/{owner}/{id}/analysis
// payload as OPTIONAL fields:
//
//   freya_version          version that PRODUCED this analysis (may be absent)
//   generated_at           RFC3339 timestamp        (may be absent)
//   current_freya_version  the server's live Freya  (may be absent)
//   stale                  true when the two differ (may be absent)
//   unresolved_cards       [{name, qty}]            (may be absent/empty)
//   unsupported_cards      [{name, qty}]            (may be absent/empty)
//
// Every field is optional on purpose: strategy.json files written
// before version tracking existed have none of them. When nothing is
// stale and nothing is unresolved (including the "old file, no fields
// at all" case) this component renders null.
//
// The two card notices are deliberately NOT dismissible — they are
// standing caveats about the analysis below them, not toasts.
//
// unresolved_cards and unsupported_cards are different problems and
// are worded differently on purpose. Unresolved means the card
// database doesn't know the card at all, so the analysis below was
// computed without it. Unsupported means the analysis IS correct but
// the game engine has no parsed rules for the card, so it does
// nothing in a simulated game. Collapsing them into one "problem
// cards" list would tell the user something false about one of them.

const POLL_INTERVAL_MS = 2000
const POLL_TIMEOUT_MS = 30000

// formatGeneratedAt renders an RFC3339 stamp as a short human date
// ("12 Sep 2026") using the browser locale's day/month/year parts.
// Returns '' for a missing or unparseable value so callers can fall
// back to the "predates version tracking" wording rather than
// printing "Invalid Date".
function formatGeneratedAt(iso) {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  try {
    return d.toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' })
  } catch {
    return d.toDateString()
  }
}

// cardNames renders a [{name, qty}] list (tolerating bare strings and
// malformed entries) as "A, B ×2, C".
function cardNames(list) {
  return list.map((c, i) => {
    const name = typeof c === 'string' ? c : (c?.name || '')
    const qty = typeof c === 'object' && c ? Number(c.qty) : 1
    if (!name) return null
    return (
      <span key={`${name}-${i}`}>
        {i > 0 ? ', ' : ''}
        <strong>{name}</strong>{qty > 1 ? ` ×${qty}` : ''}
      </span>
    )
  })
}

const noticeBase = {
  border: '1px solid var(--rule-2)',
  borderLeftWidth: 3,
  background: 'var(--panel-2)',
  padding: '8px 10px',
  marginBottom: 8,
  display: 'flex',
  flexWrap: 'wrap',
  alignItems: 'center',
  gap: 8,
}

export default function AnalysisFreshnessBanner({ analysis, deckId, onRefreshed }) {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  // Run token — bumped on unmount and whenever the banner is pointed at
  // a different deck, so an in-flight poll from the previous deck can't
  // land its result on the new one.
  const run = useRef(0)
  const alive = (token) => run.current === token

  useEffect(() => {
    run.current += 1
    setBusy(false)
    setError('')
    return () => { run.current += 1 }
  }, [deckId])

  if (!analysis) return null

  const stale = analysis.stale === true
  const unresolved = Array.isArray(analysis.unresolved_cards) ? analysis.unresolved_cards : []
  const unsupported = Array.isArray(analysis.unsupported_cards) ? analysis.unsupported_cards : []
  if (!stale && unresolved.length === 0 && unsupported.length === 0) return null

  const generatedOn = formatGeneratedAt(analysis.generated_at)
  const producedBy = analysis.freya_version || ''
  const currentVersion = analysis.current_freya_version || ''

  // Wait for the server to write a new strategy.json. runAnalysis is
  // fire-and-forget ({status: "analyzing"}), so freshness is detected
  // by the stale flag clearing or the generated_at stamp moving.
  const pollForFresh = async (token) => {
    const startedAt = Date.now()
    const previousStamp = analysis.generated_at || ''
    while (Date.now() - startedAt < POLL_TIMEOUT_MS) {
      await new Promise(r => setTimeout(r, POLL_INTERVAL_MS))
      if (!alive(token)) return null
      let data
      try {
        data = await api.getDeckAnalysis(deckId)
      } catch {
        continue // transient read failure mid-regeneration — keep waiting
      }
      if (!alive(token)) return null
      if (!data || data.status === 'analyzing') continue
      const movedOn = data.generated_at && data.generated_at !== previousStamp
      if (data.stale === false || movedOn) return data
    }
    return null
  }

  const refresh = async () => {
    if (!deckId) return
    const token = run.current
    setBusy(true)
    setError('')
    try {
      await api.runAnalysis(deckId)
    } catch (err) {
      if (alive(token)) {
        setError(err?.status === 429
          ? 'Too many refreshes, try again in a minute'
          : 'Refresh failed — try again')
        setBusy(false)
      }
      return
    }
    const fresh = await pollForFresh(token)
    if (!alive(token)) return
    if (fresh) {
      onRefreshed?.(fresh)
    } else {
      setError('Still working — reload the page in a moment')
    }
    setBusy(false)
  }

  return (
    <div data-testid="analysis-freshness">
      {stale && (
        <div style={{ ...noticeBase, borderLeftColor: 'var(--warn)' }} role="status">
          <Tag kind="warn" solid>OUT OF DATE</Tag>
          <span className="t-xs" style={{ flex: '1 1 260px', lineHeight: 1.5 }}>
            {generatedOn
              ? <>This analysis was generated on <strong>{generatedOn}</strong>{producedBy ? <> by engine <strong>{producedBy}</strong></> : null}.{' '}</>
              : <>This analysis predates engine version tracking, so we can't tell which build produced it.{' '}</>}
            {currentVersion
              ? <>The current engine is <strong>{currentVersion}</strong> — some conclusions may be out of date.</>
              : <>A newer engine is running now — some conclusions may be out of date.</>}
          </span>
        </div>
      )}
      {unresolved.length > 0 && (
        <div style={{ ...noticeBase, borderLeftColor: 'var(--danger)' }} role="note">
          <Tag kind="bad" solid>CARDS SKIPPED</Tag>
          <span className="t-xs" style={{ flex: '1 1 260px', lineHeight: 1.5 }}>
            {unresolved.length} card{unresolved.length === 1 ? '' : 's'} in this deck aren't in
            our card database yet, so the analysis below ignores them:{' '}
            {cardNames(unresolved)}
          </span>
        </div>
      )}
      {unsupported.length > 0 && (
        <div style={{ ...noticeBase, borderLeftColor: 'var(--warn)' }} role="note">
          <Tag kind="warn" solid>NOT PLAYABLE YET</Tag>
          <span className="t-xs" style={{ flex: '1 1 260px', lineHeight: 1.5 }}>
            The analysis below covers {unsupported.length === 1 ? 'this card' : 'these cards'},
            but our game engine has no rules for {unsupported.length === 1 ? 'it' : 'them'} yet —
            so {unsupported.length === 1 ? 'it does' : 'they do'} nothing in simulated games and
            test results will understate the deck:{' '}
            {cardNames(unsupported)}
          </span>
        </div>
      )}
      {/* One refresh control for both notices. It is offered on the
          unresolved-cards path too, not just the stale one: a card the
          database didn't know is exactly what a corpus update fixes,
          and re-running is how the user finds out it has been. */}
      {deckId && (
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
          <Btn
            sm
            arrow={null}
            disabled={busy}
            onClick={refresh}
            title="Re-run Freya on this deck with the current engine"
          >
            {busy ? 'REFRESHING…' : 'REFRESH ANALYSIS'}
          </Btn>
          {error && (
            <span className="t-xs" style={{ color: 'var(--danger)' }}>{error}</span>
          )}
        </div>
      )}
    </div>
  )
}
