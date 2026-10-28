import { useEffect, useMemo, useState } from 'react'
import { Panel, Tag } from './chrome'
import { API_BASE, cardArtUrl } from '../services/api'
import { useT } from '../i18n'

// MatchupsPanel — head-to-head record per opposing commander for a
// single deck. Backed by GET /api/decks/{owner}/{id}/matchups (returns
// rows already sorted by games desc, wins desc).
//
// Renders four blocks:
//   1. BEST MATCHUPS  — top 3 by win rate, min 3 games (otherwise
//                       small samples dominate the leaderboard).
//   2. WORST MATCHUPS — bottom 3 by win rate, same min-games gate.
//   3. TOP 5 ledger    — top 5 by win rate (full row layout).
//   4. BOTTOM 5 ledger — bottom 5 by win rate, deduped against top 5.
//
// The minimum-games filter is the natural fix for noisy aggregates: a
// 1-0 record looks "best" without the gate. We surface the threshold
// in the section header so it's not a hidden bias.

const MIN_GAMES_FOR_RANKING = 3

// Narrow viewports (mobile sidebar column on Pixel-class devices) can't
// fit the full 6-column ledger without truncating commander names to
// "Szarekh, the Silen...". Below this breakpoint we collapse the full
// ledger to the same dense layout used by BEST/WORST above.
function useIsNarrow(breakpoint = 720) {
  const [narrow, setNarrow] = useState(() =>
    typeof window !== 'undefined' && window.matchMedia(`(max-width: ${breakpoint}px)`).matches
  )
  useEffect(() => {
    if (typeof window === 'undefined') return
    const mq = window.matchMedia(`(max-width: ${breakpoint}px)`)
    const onChange = (e) => setNarrow(e.matches)
    mq.addEventListener('change', onChange)
    return () => mq.removeEventListener('change', onChange)
  }, [breakpoint])
  return narrow
}

function winRateColor(wr) {
  if (wr >= 60) return 'var(--ok)'
  if (wr <= 40) return 'var(--danger)'
  return 'var(--warn)'
}

function WinRateBar({ winRate }) {
  // Bar fills 0–100 left-to-right; a thin marker at 50% gives the
  // viewer an instant "above/below break-even" reference.
  const pct = Math.max(0, Math.min(100, winRate))
  return (
    <div
      title={`${winRate.toFixed(1)}%`}
      style={{
        position: 'relative',
        height: 10,
        background: 'var(--bg-2)',
        border: '1px solid var(--rule-2)',
        overflow: 'hidden',
      }}
    >
      <div
        style={{
          position: 'absolute',
          inset: 0,
          width: `${pct}%`,
          background: winRateColor(winRate),
          opacity: 0.85,
          transition: 'width 200ms ease-out',
        }}
      />
      <div
        style={{
          position: 'absolute',
          left: '50%',
          top: 0,
          bottom: 0,
          width: 1,
          background: 'var(--ink-3)',
          opacity: 0.6,
        }}
      />
    </div>
  )
}

function OpponentArt({ commander }) {
  const url = commander ? cardArtUrl(commander) : null
  return (
    <div
      className={url ? '' : 'hatch'}
      style={{
        width: 28, height: 28,
        flexShrink: 0,
        overflow: 'hidden',
        border: '1px solid var(--rule-2)',
        background: 'var(--bg-2)',
      }}
    >
      {url && (
        <img
          src={url}
          alt=""
          loading="lazy"
          style={{ width: '100%', height: '100%', objectFit: 'cover', display: 'block' }}
          onError={(e) => { e.target.style.display = 'none'; e.target.parentElement.classList.add('hatch') }}
        />
      )}
    </div>
  )
}

function MatchupRow({ row, dense }) {
  const t = useT()
  return (
    <div style={{
      display: 'grid',
      gridTemplateColumns: dense
        ? '28px 1fr auto'
        : '28px 1fr 36px 36px 36px 70px',
      gap: 8,
      alignItems: 'center',
      padding: '5px 0',
      borderBottom: '1px dashed var(--rule)',
      fontSize: 11,
    }}>
      <OpponentArt commander={row.opponent_commander} />
      <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontWeight: 600 }}>
        {row.opponent_commander}
      </span>
      {dense ? (
        <span style={{ display: 'flex', alignItems: 'center', gap: 6, minWidth: 100 }}>
          <span className="t-xs muted" style={{ minWidth: 38, textAlign: 'right' }}>
            {row.wins}-{row.losses}
            {row.draws > 0 && (
              <span className="muted-2"> ({t('op.draws_suffix', { count: row.draws })})</span>
            )}
          </span>
          <span style={{ flex: 1, minWidth: 60 }}>
            <WinRateBar winRate={row.win_rate} />
          </span>
          <span style={{
            color: winRateColor(row.win_rate),
            fontWeight: 700,
            minWidth: 38,
            textAlign: 'right',
          }}>
            {row.win_rate.toFixed(0)}%
          </span>
        </span>
      ) : (
        <>
          <span className="t-xs" style={{ textAlign: 'right' }}>{row.games}</span>
          <span className="t-xs" style={{ textAlign: 'right', color: 'var(--ok)' }}>{row.wins}</span>
          <span className="t-xs" style={{ textAlign: 'right', color: 'var(--danger)' }}>{row.losses}</span>
          <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
            <span style={{ flex: 1, minWidth: 30 }}>
              <WinRateBar winRate={row.win_rate} />
            </span>
            <span style={{
              color: winRateColor(row.win_rate),
              fontWeight: 700,
              minWidth: 30,
              textAlign: 'right',
              fontSize: 10,
            }}>
              {row.win_rate.toFixed(0)}%
            </span>
          </span>
        </>
      )}
    </div>
  )
}

export default function MatchupsPanel({ owner, id, code = '04.MU' }) {
  const t = useT()
  const [rows, setRows] = useState(null)
  const [error, setError] = useState(false)
  const narrow = useIsNarrow()

  useEffect(() => {
    if (!owner || !id) return
    let alive = true
    setRows(null)
    setError(false)
    fetch(`${API_BASE}/api/decks/${encodeURIComponent(owner)}/${encodeURIComponent(id)}/matchups`)
      .then(r => r.ok ? r.json() : Promise.reject(new Error(`${r.status}`)))
      .then(d => { if (alive) setRows(Array.isArray(d?.matchups) ? d.matchups : []) })
      .catch(() => { if (alive) { setError(true); setRows([]) } })
    return () => { alive = false }
  }, [owner, id])

  // Best / worst: rank only opponents with enough games to be meaningful.
  // Backend sorts by games desc; we re-sort by win_rate within the
  // qualified subset for the leaderboards. The full ledger keeps top 5
  // and bottom 5 — middle entries are dropped to keep the panel scannable.
  const { best, worst, topFive, bottomFive } = useMemo(() => {
    if (!rows || rows.length === 0) return { best: [], worst: [], topFive: [], bottomFive: [] }
    const qualified = rows.filter(r => r.games >= MIN_GAMES_FOR_RANKING)
    const byWinRate = [...qualified].sort((a, b) => b.win_rate - a.win_rate)
    const top = byWinRate.slice(0, 5)
    // Dedupe bottom against top so a small qualified pool (<10) doesn't
    // render the same row in both groups.
    const topKeys = new Set(top.map(r => r.opponent_commander))
    const bottom = byWinRate
      .filter(r => !topKeys.has(r.opponent_commander))
      .slice(-5)
      .reverse()
    return {
      best: byWinRate.slice(0, 3),
      worst: byWinRate.slice(-3).reverse(),
      topFive: top,
      bottomFive: bottom,
    }
  }, [rows])

  if (rows === null) {
    return (
      <Panel code={code} title={t('dka.mu_title_loading')}>
        <div className="t-md muted" style={{ textAlign: 'center', padding: 12 }}>
          {t('dka.mu_fetching')}<span className="blink">_</span>
        </div>
      </Panel>
    )
  }

  if (error) {
    return (
      <Panel code={code} title={t('dka.mu_title_offline')}>
        <div className="t-xs muted" style={{ textAlign: 'center', padding: 12 }}>
          {t('dka.mu_unavailable')}
        </div>
      </Panel>
    )
  }

  if (rows.length === 0) {
    return (
      <Panel code={code} title={t('dka.mu_title_empty')}>
        <div className="t-xs muted" style={{ textAlign: 'center', padding: 12 }}>
          {t('dka.mu_no_games')}
        </div>
      </Panel>
    )
  }

  return (
    <Panel
      code={code}
      title={t(rows.length === 1 ? 'dka.mu_title_one' : 'dka.mu_title', { count: rows.length })}
      right={<Tag solid>{(() => {
        const g = rows.reduce((s, r) => s + r.games, 0)
        return t(g === 1 ? 'dka.mu_games_one' : 'dka.mu_games', { count: g })
      })()}</Tag>}
    >
      {/* Best */}
      {best.length > 0 && (
        <div style={{ marginBottom: 12 }}>
          <div style={{
            fontSize: 9, fontWeight: 700, letterSpacing: '0.14em',
            color: 'var(--ok)', textTransform: 'uppercase',
            marginBottom: 4, paddingBottom: 3,
            borderBottom: '1px dashed color-mix(in srgb, var(--ok) 35%, var(--rule-2))',
          }}>
            {t('dka.mu_best')}
            <span className="muted-2" style={{ fontWeight: 400, marginLeft: 6 }}>
              {t(MIN_GAMES_FOR_RANKING === 1 ? 'dka.mu_min_games_one' : 'dka.mu_min_games', { count: MIN_GAMES_FOR_RANKING })}
            </span>
          </div>
          {best.map((r) => <MatchupRow key={`b-${r.opponent_commander}`} row={r} dense />)}
        </div>
      )}

      {/* Worst */}
      {worst.length > 0 && (
        <div style={{ marginBottom: 12 }}>
          <div style={{
            fontSize: 9, fontWeight: 700, letterSpacing: '0.14em',
            color: 'var(--danger)', textTransform: 'uppercase',
            marginBottom: 4, paddingBottom: 3,
            borderBottom: '1px dashed color-mix(in srgb, var(--danger) 35%, var(--rule-2))',
          }}>
            {t('dka.mu_worst')}
            <span className="muted-2" style={{ fontWeight: 400, marginLeft: 6 }}>
              {t(MIN_GAMES_FOR_RANKING === 1 ? 'dka.mu_min_games_one' : 'dka.mu_min_games', { count: MIN_GAMES_FOR_RANKING })}
            </span>
          </div>
          {worst.map((r) => <MatchupRow key={`w-${r.opponent_commander}`} row={r} dense />)}
        </div>
      )}

      {/* Top / bottom 5 ledger — middle entries dropped to keep the panel
          scannable. Same min-games gate as the leaderboards above. Narrow
          viewports drop the G/W/L columns and use the dense layout to
          avoid truncating commander names. */}
      {(topFive.length > 0 || bottomFive.length > 0) && (
        <div>
          {!narrow && (
            <div style={{
              display: 'grid',
              gridTemplateColumns: '28px 1fr 36px 36px 36px 70px',
              gap: 8,
              padding: '4px 0',
              borderBottom: '1px solid var(--rule-2)',
              fontSize: 9,
              letterSpacing: '0.1em',
              color: 'var(--ink-3)',
              fontWeight: 700,
            }}>
              <span></span>
              <span>{t('op.col_opponent')}</span>
              <span style={{ textAlign: 'right' }}>{t('dka.mu_col_g')}</span>
              <span style={{ textAlign: 'right' }}>{t('dka.mu_col_w')}</span>
              <span style={{ textAlign: 'right' }}>{t('dka.mu_col_l')}</span>
              <span style={{ textAlign: 'right' }}>{t('tlm.th_winpct')}</span>
            </div>
          )}
          {topFive.length > 0 && (
            <>
              <div style={{
                fontSize: 9, fontWeight: 700, letterSpacing: '0.14em',
                color: 'var(--ok)', textTransform: 'uppercase',
                padding: '6px 0 3px',
              }}>
                {t('dka.mu_top', { count: topFive.length })}
                <span className="muted-2" style={{ fontWeight: 400, marginLeft: 6 }}>
                  {t(MIN_GAMES_FOR_RANKING === 1 ? 'dka.mu_min_games_one' : 'dka.mu_min_games', { count: MIN_GAMES_FOR_RANKING })}
                </span>
              </div>
              {topFive.map((r) => <MatchupRow key={`t-${r.opponent_commander}`} row={r} dense={narrow} />)}
            </>
          )}
          {bottomFive.length > 0 && (
            <>
              <div style={{
                marginTop: 8,
                borderTop: '1px solid var(--rule-2)',
                fontSize: 9, fontWeight: 700, letterSpacing: '0.14em',
                color: 'var(--danger)', textTransform: 'uppercase',
                padding: '8px 0 3px',
              }}>
                {t('dka.mu_bottom', { count: bottomFive.length })}
                <span className="muted-2" style={{ fontWeight: 400, marginLeft: 6 }}>
                  {t(MIN_GAMES_FOR_RANKING === 1 ? 'dka.mu_min_games_one' : 'dka.mu_min_games', { count: MIN_GAMES_FOR_RANKING })}
                </span>
              </div>
              {bottomFive.map((r) => <MatchupRow key={`bot-${r.opponent_commander}`} row={r} dense={narrow} />)}
            </>
          )}
        </div>
      )}
    </Panel>
  )
}
