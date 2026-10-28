import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Panel, Tag, Tape, Btn } from '../components/chrome'
import { api } from '../services/api'
import { useT } from '../i18n'

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

const formatSeat = (seat, t) => (seat < 0 ? t('gs.draw') : t('gs.seat', { n: seat }))

const PAGE_SIZE = 25

export default function SummaryArchive() {
  const t = useT()
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
        left={t('gs.sa_tape_left')}
        mid={loading ? t('common.loading') : t('gs.sa_rows', { count: rows.length })}
        right={<Tag kind="good" solid>{t('gs.sa_tag_rich')}</Tag>}
      />

      <Panel title={t('gs.sa_filters')} code="SA.F" data-testid="summary-archive-filters">
        <form onSubmit={applyFilters} style={{ display: 'flex', flexWrap: 'wrap', gap: 12, alignItems: 'end' }}>
          <label style={{ display: 'grid', gap: 2, fontSize: 9, textTransform: 'uppercase', letterSpacing: '0.1em' }}>
            {t('gs.sa_from')}
            <input
              type="date"
              value={filters.sinceIso}
              onChange={(e) => setFilters((f) => ({ ...f, sinceIso: e.target.value }))}
              style={inputStyle}
              data-testid="filter-since"
            />
          </label>
          <label style={{ display: 'grid', gap: 2, fontSize: 9, textTransform: 'uppercase', letterSpacing: '0.1em' }}>
            {t('gs.sa_until')}
            <input
              type="date"
              value={filters.untilIso}
              onChange={(e) => setFilters((f) => ({ ...f, untilIso: e.target.value }))}
              style={inputStyle}
              data-testid="filter-until"
            />
          </label>
          <label style={{ display: 'grid', gap: 2, fontSize: 9, textTransform: 'uppercase', letterSpacing: '0.1em' }}>
            {t('common.commander')}
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
            {t('crd.col_deck')}
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
            {t('common.winner')}
            <input
              type="text"
              placeholder={t('gs.sa_ph_winner')}
              value={filters.winner}
              onChange={(e) => setFilters((f) => ({ ...f, winner: e.target.value }))}
              style={inputStyle}
              data-testid="filter-winner"
            />
          </label>
          <div style={{ display: 'inline-flex', gap: 6 }}>
            <Btn sm solid arrow={null} onClick={applyFilters} data-testid="filter-apply">{t('gs.sa_apply')}</Btn>
            <Btn sm ghost arrow={null} onClick={clearFilters} data-testid="filter-clear">{t('dka.clear')}</Btn>
          </div>
        </form>
      </Panel>

      <Panel title={t('rpt.games')} code="SA.L" right={
        <span className="muted-2 t-xs">
          {offset > 0 && <Btn sm ghost arrow={null} onClick={() => setOffset((o) => Math.max(0, o - PAGE_SIZE))} data-testid="prev-page">◀ {t('gs.sa_prev')}</Btn>}
          {' '}
          {t('gs.sa_offset', { count: offset })}
          {' '}
          {rows.length === PAGE_SIZE && <Btn sm ghost arrow={null} onClick={() => setOffset((o) => o + PAGE_SIZE)} data-testid="next-page">{t('gs.sa_next')} ▶</Btn>}
        </span>
      }>
        {error && <p style={{ color: 'var(--err)' }}>{error}</p>}
        {!error && !loading && rows.length === 0 && (
          /* The live snapshot writer landed in PR #186; the read side is
             #180's pipeline and the row target is #189. Those references
             belong here, not in a string shown to the public. */
          <p className="muted" data-testid="summary-archive-empty">
            {t('gs.sa_empty')}
          </p>
        )}
        {rows.length > 0 && (
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 11 }} data-testid="summary-archive-table">
            <thead>
              <tr style={{ borderBottom: '1px solid var(--rule-2)', textAlign: 'left' }}>
                <th style={{ padding: '4px 6px' }}>{t('dka.col_game')}</th>
                <th style={{ padding: '4px 6px' }}>{t('gs.sa_col_finished')}</th>
                <th style={{ padding: '4px 6px' }}>T</th>
                <th style={{ padding: '4px 6px' }}>{t('common.winner')}</th>
                <th style={{ padding: '4px 6px' }}>{t('gs.sa_col_commanders')}</th>
                <th style={{ padding: '4px 6px' }}>{t('op.col_end')}</th>
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
                    {r.winner_name} <span className="muted-2">({formatSeat(r.winner, t)})</span>
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
