import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Panel, Tag, Tape, Btn } from '../components/chrome'
import { api } from '../services/api'

// SummaryArchive lists every game with a persisted observation
// snapshot (the read-side of #180's persistence pipeline). Four
// filters above a paginated table; clicking a row navigates to
// /games/:id/summary (#189). Empty state when no games match.

// Date inputs surface YYYY-MM-DD; the backend takes unix-epoch
// seconds. These helpers do the conversion in both directions
// with timezone-naive UTC midnight so the URL/state is stable.
const isoToEpoch = (iso) => {
  if (!iso) return 0
  const d = new Date(iso + 'T00:00:00Z')
  const t = d.getTime()
  return Number.isFinite(t) ? Math.floor(t / 1000) : 0
}

const formatFinished = (epoch) => {
  if (!epoch) return '—'
  try {
    return new Date(epoch * 1000).toLocaleString()
  } catch {
    return String(epoch)
  }
}

const formatSeat = (seat) => (seat < 0 ? 'DRAW' : `SEAT ${seat}`)

const PAGE_SIZE = 25

export default function SummaryArchive() {
  const navigate = useNavigate()
  const [filters, setFilters] = useState({
    sinceIso: '',
    untilIso: '',
    commander: '',
    deck: '',
    winner: '',
  })
  const [rows, setRows] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [offset, setOffset] = useState(0)
  const [submitNonce, setSubmitNonce] = useState(0)

  useEffect(() => {
    let cancelled = false
    api.searchGameSummaries({
      since: isoToEpoch(filters.sinceIso),
      until: isoToEpoch(filters.untilIso),
      commander: filters.commander.trim(),
      deck: filters.deck.trim(),
      winner: filters.winner.trim(),
      limit: PAGE_SIZE,
      offset,
    })
      .then((res) => {
        if (cancelled) return
        setRows(res?.rows || [])
        setError(null)
        setLoading(false)
      })
      .catch((e) => {
        if (cancelled) return
        setError(e?.message || String(e))
        setLoading(false)
      })
    return () => { cancelled = true }
  }, [submitNonce, offset, filters.sinceIso, filters.untilIso, filters.commander, filters.deck, filters.winner])

  const applyFilters = (e) => {
    e?.preventDefault?.()
    setOffset(0)
    setSubmitNonce((n) => n + 1)
  }

  const clearFilters = () => {
    setFilters({ sinceIso: '', untilIso: '', commander: '', deck: '', winner: '' })
    setOffset(0)
    setSubmitNonce((n) => n + 1)
  }

  const inputStyle = {
    background: 'var(--ink-bg-2, #0b0c10)',
    border: '1px solid var(--rule-2)',
    color: 'var(--ink-0)',
    padding: '4px 8px',
    fontSize: 11,
    minWidth: 120,
  }

  return (
    <div className="container summary-archive" style={{ padding: 24, display: 'grid', gap: 16 }}>
      <Tape
        left="PAST SUMMARIES"
        mid={loading ? 'LOADING' : `${rows.length} ROWS`}
        right={<Tag kind="good" solid>RICH GAMES ONLY</Tag>}
      />

      <Panel title="FILTERS" code="SA.F" data-testid="summary-archive-filters">
        <form onSubmit={applyFilters} style={{ display: 'flex', flexWrap: 'wrap', gap: 12, alignItems: 'end' }}>
          <label style={{ display: 'grid', gap: 2, fontSize: 9, textTransform: 'uppercase', letterSpacing: '0.1em' }}>
            FROM
            <input
              type="date"
              value={filters.sinceIso}
              onChange={(e) => setFilters((f) => ({ ...f, sinceIso: e.target.value }))}
              style={inputStyle}
              data-testid="filter-since"
            />
          </label>
          <label style={{ display: 'grid', gap: 2, fontSize: 9, textTransform: 'uppercase', letterSpacing: '0.1em' }}>
            UNTIL
            <input
              type="date"
              value={filters.untilIso}
              onChange={(e) => setFilters((f) => ({ ...f, untilIso: e.target.value }))}
              style={inputStyle}
              data-testid="filter-until"
            />
          </label>
          <label style={{ display: 'grid', gap: 2, fontSize: 9, textTransform: 'uppercase', letterSpacing: '0.1em' }}>
            COMMANDER
            <input
              type="text"
              placeholder="Atraxa, Krenko, ..."
              value={filters.commander}
              onChange={(e) => setFilters((f) => ({ ...f, commander: e.target.value }))}
              style={inputStyle}
              data-testid="filter-commander"
            />
          </label>
          <label style={{ display: 'grid', gap: 2, fontSize: 9, textTransform: 'uppercase', letterSpacing: '0.1em' }}>
            DECK
            <input
              type="text"
              placeholder="owner/slug"
              value={filters.deck}
              onChange={(e) => setFilters((f) => ({ ...f, deck: e.target.value }))}
              style={inputStyle}
              data-testid="filter-deck"
            />
          </label>
          <label style={{ display: 'grid', gap: 2, fontSize: 9, textTransform: 'uppercase', letterSpacing: '0.1em' }}>
            WINNER
            <input
              type="text"
              placeholder="winner name substring"
              value={filters.winner}
              onChange={(e) => setFilters((f) => ({ ...f, winner: e.target.value }))}
              style={inputStyle}
              data-testid="filter-winner"
            />
          </label>
          <div style={{ display: 'inline-flex', gap: 6 }}>
            <Btn sm solid arrow={null} onClick={applyFilters} data-testid="filter-apply">APPLY</Btn>
            <Btn sm ghost arrow={null} onClick={clearFilters} data-testid="filter-clear">CLEAR</Btn>
          </div>
        </form>
      </Panel>

      <Panel title="GAMES" code="SA.L" right={
        <span className="muted-2 t-xs">
          {offset > 0 && <Btn sm ghost arrow={null} onClick={() => setOffset((o) => Math.max(0, o - PAGE_SIZE))} data-testid="prev-page">◀ PREV</Btn>}
          {' '}
          OFFSET {offset}
          {' '}
          {rows.length === PAGE_SIZE && <Btn sm ghost arrow={null} onClick={() => setOffset((o) => o + PAGE_SIZE)} data-testid="next-page">NEXT ▶</Btn>}
        </span>
      }>
        {error && <p style={{ color: 'var(--err)' }}>{error}</p>}
        {!error && !loading && rows.length === 0 && (
          <p className="muted" data-testid="summary-archive-empty">
            No persisted summaries match the current filters. Once games run with the
            live snapshot writer enabled (PR #186), they'll appear here.
          </p>
        )}
        {rows.length > 0 && (
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 11 }} data-testid="summary-archive-table">
            <thead>
              <tr style={{ borderBottom: '1px solid var(--rule-2)', textAlign: 'left' }}>
                <th style={{ padding: '4px 6px' }}>GAME</th>
                <th style={{ padding: '4px 6px' }}>FINISHED</th>
                <th style={{ padding: '4px 6px' }}>T</th>
                <th style={{ padding: '4px 6px' }}>WINNER</th>
                <th style={{ padding: '4px 6px' }}>COMMANDERS</th>
                <th style={{ padding: '4px 6px' }}>END</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((r) => (
                <tr
                  key={r.game_id}
                  onClick={() => navigate(`/games/${r.game_id}/summary`)}
                  style={{ borderBottom: '1px solid var(--rule-3)', cursor: 'pointer' }}
                  data-testid={`summary-row-${r.game_id}`}
                >
                  <td style={{ padding: '4px 6px' }}><strong>#{r.game_id}</strong></td>
                  <td style={{ padding: '4px 6px' }}>{formatFinished(r.finished_at)}</td>
                  <td style={{ padding: '4px 6px' }}>{r.turns}</td>
                  <td style={{ padding: '4px 6px' }}>
                    {r.winner_name} <span className="muted-2">({formatSeat(r.winner)})</span>
                  </td>
                  <td style={{ padding: '4px 6px' }}>
                    {(r.commanders || []).join(' · ')}
                  </td>
                  <td style={{ padding: '4px 6px' }}>
                    <span className="muted-2">{(r.end_reason || '').toUpperCase()}</span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Panel>
    </div>
  )
}
