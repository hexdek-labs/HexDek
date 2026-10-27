import { useState, useEffect, useMemo, useCallback } from 'react'
import { Panel, Tape, Tag, Btn } from '../components/chrome'
import { api } from '../services/api'
import { useT } from '../i18n'
import { fill } from '../components/fill'

// AdminConviction — live view of the conviction diagnostic ring buffer
// exposed by /api/admin/conviction-events. The endpoint is gated by the
// HEXDEK_ADMIN_OWNER env var matched against the X-HexDek-Owner header,
// so a 403 here is the expected response for non-owners.
//
// The on-the-wire ConvictionEvent (internal/hat/conviction_telemetry.go)
// carries: seq, captured_at, game_seed, seat, turn, relative_position,
// window_samples, score_triggered, winline_extinct, winline_detail,
// any_triggered. There is no commander name or per-seat life value in
// the payload (Round 16 stripped conviction down to a non-acting
// diagnostic), so the filter set below maps onto what's actually there:
//   - "Game" filters by game_seed (the closest deck/game identity signal)
//   - "Min relative position" filters by the relative_position score
//     (analog to a life threshold: the lower the score, the worse the
//     hat thinks it's doing — same input that feeds the score-window
//     trigger conviction would have used)
//   - "Min turn" filters by turn
//   - "Severity" filters by which trigger fired (any/score/winline/none)

const POLL_INTERVAL_MS = 3000

// labelKey, not label: this array is read at render from module scope, so
// a word here would be fixed at load and stay in whatever language the
// page started in. The values are the wire/filter identities and stay.
const SEVERITY_OPTIONS = [
  { value: 'all',     labelKey: 'adm.sv_all' },
  { value: 'any',     labelKey: 'adm.sv_any' },
  { value: 'score',   labelKey: 'adm.sv_score' },
  { value: 'winline', labelKey: 'adm.sv_winline' },
  { value: 'both',    labelKey: 'adm.sv_both' },
  { value: 'none',    labelKey: 'adm.sv_none' },
]

// Takes the translator as a PARAMETER: a module-level formatter that
// reached for one of its own would word these pills once, at import.
// They are separate keys from the matching filter options — a pill is
// seven characters in a table cell, an option is a phrase in a dropdown.
function severityTag(t, ev) {
  if (ev.score_triggered && ev.winline_extinct) {
    return <Tag kind="danger" solid>{t('adm.tag_both')}</Tag>
  }
  if (ev.score_triggered) {
    return <Tag kind="warn" solid>{t('adm.tag_score')}</Tag>
  }
  if (ev.winline_extinct) {
    return <Tag kind="warn" solid>{t('adm.tag_winline')}</Tag>
  }
  return <Tag kind="info">—</Tag>
}

function formatTime(iso) {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toISOString().replace('T', ' ').replace(/\.\d+Z$/, 'Z')
}

function formatSeed(seed) {
  if (seed == null || seed === 0) return '—'
  // Show a short prefix so the column stays narrow but still
  // disambiguates concurrent games.
  const s = String(seed)
  return s.length > 10 ? `${s.slice(0, 10)}…` : s
}

export default function AdminConviction() {
  const t = useT()
  const [events, setEvents] = useState([])
  const [totalSeen, setTotalSeen] = useState(0)
  const [bufferCapacity, setBufferCapacity] = useState(1024)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [autoRefresh, setAutoRefresh] = useState(true)

  // Filters
  const [seedFilter, setSeedFilter] = useState('')
  const [minRelPos, setMinRelPos] = useState('')
  const [maxRelPos, setMaxRelPos] = useState('')
  const [minTurn, setMinTurn] = useState('')
  const [severity, setSeverity] = useState('all')

  const fetchEvents = useCallback(async () => {
    try {
      // Always pull the full window (limit=0 returns everything in the
      // ring) so client-side filtering operates on the same dataset
      // regardless of how aggressive the user's filters are. The buffer
      // is capped at 1024 entries — small enough to ship in one request.
      const data = await api.getConvictionEvents({ limit: 0 })
      setEvents(data.events || [])
      setTotalSeen(data.total_seen || 0)
      setBufferCapacity(data.buffer_capacity || 1024)
      setError(null)
    } catch (e) {
      setError(e.message || String(e))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchEvents()
  }, [fetchEvents])

  useEffect(() => {
    if (!autoRefresh) return undefined
    const id = setInterval(fetchEvents, POLL_INTERVAL_MS)
    return () => clearInterval(id)
  }, [autoRefresh, fetchEvents])

  const filtered = useMemo(() => {
    const seedQ = seedFilter.trim()
    const minRP = minRelPos === '' ? null : Number(minRelPos)
    const maxRP = maxRelPos === '' ? null : Number(maxRelPos)
    const minT = minTurn === '' ? null : Number(minTurn)
    const rows = []
    for (const ev of events) {
      if (seedQ && !String(ev.game_seed ?? '').includes(seedQ)) continue
      if (minRP != null && !Number.isNaN(minRP) && (ev.relative_position ?? 0) < minRP) continue
      if (maxRP != null && !Number.isNaN(maxRP) && (ev.relative_position ?? 0) > maxRP) continue
      if (minT != null && !Number.isNaN(minT) && (ev.turn ?? 0) < minT) continue
      switch (severity) {
        case 'any':     if (!ev.any_triggered) continue; break
        case 'score':   if (!ev.score_triggered) continue; break
        case 'winline': if (!ev.winline_extinct) continue; break
        case 'both':    if (!(ev.score_triggered && ev.winline_extinct)) continue; break
        case 'none':    if (ev.any_triggered) continue; break
        default: break
      }
      rows.push(ev)
    }
    // Newest first — the ring buffer hands us oldest-first.
    return rows.slice().reverse()
  }, [events, seedFilter, minRelPos, maxRelPos, minTurn, severity])

  const counts = useMemo(() => {
    let triggered = 0
    let score = 0
    let winline = 0
    for (const ev of events) {
      if (ev.any_triggered) triggered++
      if (ev.score_triggered) score++
      if (ev.winline_extinct) winline++
    }
    return { triggered, score, winline }
  }, [events])

  const evictionWarning = totalSeen > bufferCapacity
    ? t('adm.eviction', {
        total: totalSeen.toLocaleString(),
        evicted: (totalSeen - bufferCapacity).toLocaleString(),
      })
    : null

  return (
    <div className="admin-conviction" style={{ maxWidth: 1400, margin: '0 auto', padding: '20px 30px' }}>
      <style>{adminConvictionCSS}</style>
      <Panel
        code="ADM.CV"
        title={t('adm.title')}
        right={
          <span style={{ display: 'inline-flex', gap: 8, alignItems: 'center', flexWrap: 'wrap', justifyContent: 'flex-end' }}>
            <Tag kind={autoRefresh ? 'ok' : undefined}>
              {/* The interval rides in as a value, so the label cannot
                  drift from the constant that drives it. */}
              {autoRefresh ? t('adm.auto', { secs: POLL_INTERVAL_MS / 1000 }) : t('adm.paused')}
            </Tag>
            <Btn sm arrow={null} onClick={() => setAutoRefresh((v) => !v)}>
              {autoRefresh ? t('adm.pause') : t('adm.resume')}
            </Btn>
            <Btn sm arrow={null} onClick={fetchEvents}>{t('adm.refresh')}</Btn>
          </span>
        }
      >
        <Tape
          left={t('adm.tape_buffer', { used: events.length, cap: bufferCapacity })}
          mid={t('adm.tape_total', { count: totalSeen.toLocaleString() })}
          right={t('adm.tape_trig', {
            trig: counts.triggered, score: counts.score, winline: counts.winline,
          })}
        />
        {evictionWarning && (
          <div
            className="t-sm"
            role="status"
            style={{
              color: 'var(--warn)',
              padding: '8px 10px',
              borderBottom: '1px solid var(--rule-2)',
              background: 'color-mix(in srgb, var(--warn) 8%, transparent)',
            }}
          >
            {evictionWarning}
          </div>
        )}
        <div style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(160px, 1fr))',
          gap: 10,
          padding: '12px 4px',
        }}>
          <Field label={t('adm.f_seed')}>
            <input
              type="text"
              value={seedFilter}
              onChange={(e) => setSeedFilter(e.target.value)}
              placeholder={t('adm.f_seed_ph')}
              style={inputStyle}
            />
          </Field>
          <Field label={t('adm.f_minrp')}>
            <input
              type="number"
              step="0.01"
              value={minRelPos}
              onChange={(e) => setMinRelPos(e.target.value)}
              placeholder={t('adm.f_minrp_ph')}
              style={inputStyle}
            />
          </Field>
          <Field label={t('adm.f_maxrp')}>
            <input
              type="number"
              step="0.01"
              value={maxRelPos}
              onChange={(e) => setMaxRelPos(e.target.value)}
              placeholder={t('adm.f_maxrp_ph')}
              style={inputStyle}
            />
          </Field>
          <Field label={t('adm.f_minturn')}>
            <input
              type="number"
              min="0"
              value={minTurn}
              onChange={(e) => setMinTurn(e.target.value)}
              placeholder={t('adm.f_minturn_ph')}
              style={inputStyle}
            />
          </Field>
          <Field label={t('adm.f_sev')}>
            <select
              value={severity}
              onChange={(e) => setSeverity(e.target.value)}
              style={inputStyle}
            >
              {SEVERITY_OPTIONS.map((o) => (
                <option key={o.value} value={o.value}>{t(o.labelKey)}</option>
              ))}
            </select>
          </Field>
        </div>
      </Panel>

      <Panel
        code="ADM.CV.1"
        title={filtered.length === 1
          ? t('adm.events_one', { count: filtered.length })
          : t('adm.events', { count: filtered.length })}
        style={{ marginTop: 16 }}
      >
        {error && (
          <div
            className="t-md"
            role="alert"
            style={{
              color: 'var(--danger)',
              padding: '12px',
              border: '1px solid var(--danger)',
              background: 'color-mix(in srgb, var(--danger) 8%, transparent)',
              margin: '8px 0',
            }}
          >
            {/* The server's own message is echoed, never worded here. */}
            <strong>{t('adm.err_label')}</strong> {error}
            <div className="t-sm muted" style={{ marginTop: 6, color: 'var(--ink-2)' }}>
              {/* Both identifiers ride in rather than being written 25
                  times, and the storage key stays a <code> node, so
                  fill() splits the sentence around it instead of pinning
                  it to the position English puts it in. */}
              {fill(t('adm.err_help'), {
                env: 'HEXDEK_ADMIN_OWNER',
                key: <code>hexdek_owner</code>,
              })}
            </div>
          </div>
        )}
        {loading && !error && (
          <div className="t-md" style={{ padding: '16px 4px', color: 'var(--ink-2)' }}>{t('adm.loading')}</div>
        )}
        {!loading && !error && filtered.length === 0 && (
          <div
            className="t-md"
            style={{
              padding: '24px 16px',
              textAlign: 'center',
              color: 'var(--ink)',
              border: '1px dashed var(--rule-2)',
              margin: '8px 0',
            }}
          >
            <div style={{ marginBottom: 6 }}>{t('adm.no_match')}</div>
            <div className="t-sm" style={{ color: 'var(--ink-2)' }}>
              {events.length === 0
                ? t('adm.buf_empty')
                : t('adm.loosen', { count: events.length })}
            </div>
          </div>
        )}
        {!loading && !error && filtered.length > 0 && (
          <div style={{ overflowX: 'auto' }}>
            <table style={tableStyle}>
              <thead>
                <tr style={{ background: 'var(--bg-2)' }}>
                  <Th>{t('adm.th_seq')}</Th>
                  <Th>{t('adm.th_captured')}</Th>
                  <Th>{t('adm.th_seed')}</Th>
                  <Th>{t('adm.th_seat')}</Th>
                  <Th>{t('adm.th_turn')}</Th>
                  <Th>{t('adm.th_relpos')}</Th>
                  <Th>{t('adm.th_window')}</Th>
                  {/* Same word as the filter above it, in the same
                      register on the same screen, so it is one key. */}
                  <Th>{t('adm.f_sev')}</Th>
                  <Th>{t('adm.th_winline')}</Th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((ev, i) => (
                  <tr
                    key={ev.seq}
                    className="cv-row"
                    style={{
                      borderTop: '1px solid var(--rule-2)',
                      background: i % 2 === 1 ? 'color-mix(in srgb, var(--ink) 3%, transparent)' : 'transparent',
                    }}
                  >
                    <Td>{ev.seq}</Td>
                    <Td>{formatTime(ev.captured_at)}</Td>
                    <Td title={String(ev.game_seed ?? '')}>{formatSeed(ev.game_seed)}</Td>
                    <Td>{ev.seat}</Td>
                    <Td>{ev.turn}</Td>
                    <Td style={{ color: (ev.relative_position ?? 0) < 0 ? 'var(--warn)' : undefined }}>
                      {Number(ev.relative_position ?? 0).toFixed(3)}
                    </Td>
                    <Td>{ev.window_samples}</Td>
                    <Td>{severityTag(t, ev)}</Td>
                    <Td
                      title={ev.winline_detail || ''}
                      style={{
                        color: 'var(--ink-2)',
                        maxWidth: 360,
                        whiteSpace: 'nowrap',
                        overflow: 'hidden',
                        textOverflow: 'ellipsis',
                      }}
                    >
                      {ev.winline_detail || '—'}
                    </Td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Panel>
    </div>
  )
}

const inputStyle = {
  width: '100%',
  padding: '6px 8px',
  fontSize: 11,
  fontFamily: 'inherit',
  background: 'var(--bg-1)',
  color: 'var(--ink)',
  border: '1px solid var(--rule-2)',
  borderRadius: 0,
  minHeight: 28,
}

const adminConvictionCSS = `
  .admin-conviction .cv-row { transition: background 120ms ease; }
  .admin-conviction .cv-row:hover { background: color-mix(in srgb, var(--ink) 8%, transparent) !important; }
  .admin-conviction input:focus,
  .admin-conviction select:focus {
    outline: 1px solid var(--hi);
    outline-offset: -1px;
    border-color: var(--hi);
  }
  @media (max-width: 720px) {
    .admin-conviction { padding: 12px 10px !important; }
  }
`

const tableStyle = {
  width: '100%',
  borderCollapse: 'collapse',
  fontSize: 11,
  fontVariantNumeric: 'tabular-nums',
}

function Field({ label, children }) {
  return (
    <label style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
      <span className="muted-2" style={{ fontSize: 8, letterSpacing: '0.1em' }}>{label}</span>
      {children}
    </label>
  )
}

function Th({ children }) {
  return (
    <th style={{
      textAlign: 'left',
      padding: '6px 8px',
      fontSize: 9,
      letterSpacing: '0.1em',
      color: 'var(--ink-2)',
      fontWeight: 600,
      borderBottom: '1px solid var(--rule-2)',
    }}>
      {children}
    </th>
  )
}

function Td({ children, className, title, style }) {
  return (
    <td className={className} title={title} style={{ padding: '6px 8px', verticalAlign: 'top', ...style }}>
      {children}
    </td>
  )
}
