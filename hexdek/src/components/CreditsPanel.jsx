import { useEffect, useState, useCallback } from 'react'
import { Panel, KV, Tag } from './chrome'
import { api } from '../services/api'
import { useT } from '../i18n'

// CreditsPanel — displays the operator's current balance, free-tier
// quota state, and recent transaction history. Hosts the
// "insufficient credits" UX messaging. Designed to drop into the
// operator profile but reusable wherever a credit summary makes
// sense (gauntlet pre-flight modal, future spending pages).
//
// Props:
//   compact   — render the body without the history list (sidebar use)
//   refreshKey — bump this to force a re-fetch (e.g. after the user
//                runs a gauntlet; the parent screen flips a counter)
//
// Failure modes:
//   - Unauthenticated → renders "Sign in to view credits"
//   - API error → renders the error message inline; balance reads as —
//
// The component does no spending of its own — the spend flow lives
// alongside the gauntlet button in DeckArchive. This panel is read-
// mostly so the user can audit their balance and recent activity.

// The four transaction reasons as they arrive on the wire, mapped to
// catalog PATHS rather than words: this table is read at render from
// module scope, so a word here would be fixed at load and stay in the
// language the page started in.
const REASON_KEYS = {
  compute_contribution: 'cr.rs_compute',
  gauntlet_run:         'cr.rs_gauntlet',
  extended_analysis:    'cr.rs_extended',
  admin_adjustment:     'cr.rs_admin',
}

// Takes the translator as a PARAMETER for the same reason: a module-level
// formatter that reached for a translator of its own would word the label
// once, at import. An unrecognised reason is a server string we do not
// know — prettify and show it rather than hide it.
function formatReason(reason, t) {
  const key = REASON_KEYS[reason]
  if (key) return t(key)
  if (!reason) return t('cr.rs_other')
  return reason.toUpperCase().replace(/_/g, ' ')
}

function formatTimestamp(ts) {
  if (!ts) return '—'
  const d = new Date(ts * 1000)
  if (isNaN(d.getTime())) return '—'
  return d.toISOString().slice(0, 16).replace('T', ' ')
}

export default function CreditsPanel({ compact = false, refreshKey = 0 }) {
  const t = useT()
  const [balance, setBalance] = useState(null)
  const [quota, setQuota] = useState(null)
  const [history, setHistory] = useState([])
  const [error, setError] = useState(null)
  const [loading, setLoading] = useState(true)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const [b, q] = await Promise.all([
        api.getCreditBalance(),
        api.getCreditQuota(),
      ])
      setBalance(b?.credits ?? 0)
      setQuota(q)
      if (!compact) {
        const h = await api.getCreditHistory(20)
        setHistory(h?.transactions || [])
      }
    } catch (err) {
      // State holds a catalog PATH, or the server's own message to be
      // echoed — never a worded sentence. load() is async, so a string
      // worded here would keep the language it was fetched in.
      if (err?.status === 401) {
        setError({ key: 'cr.err_signin' })
      } else if (err?.message) {
        setError({ text: err.message })
      } else {
        setError({ key: 'cr.err_load' })
      }
    } finally {
      setLoading(false)
    }
  }, [compact])

  useEffect(() => { load() }, [load, refreshKey])

  return (
    <Panel
      code="CR.0"
      title={t('cr.title')}
      right={
        balance != null && quota
          ? <Tag solid kind={quota.can_run_paid || quota.can_run_free ? 'ok' : 'warn'}>
              {t('cr.cr_suffix', { count: balance })}
            </Tag>
          : null
      }
    >
      {loading && balance == null ? (
        <div className="t-md muted" style={{ textAlign: 'center', padding: 18 }}>
          {t('cr.loading_line')}<span className="blink">_</span>
        </div>
      ) : error ? (
        <div className="t-md muted" style={{ textAlign: 'center', padding: 14 }}>
          {/* Worded here, at the render site, from whatever load() parked
              in state. The catalog values are already upper case, so the
              uppercasing is only for an echoed server message. */}
          &gt; {(error.key ? t(error.key) : error.text).toUpperCase()}
        </div>
      ) : (
        <>
          <KV rows={[
            [t('cr.kv_balance'), t('cr.cr_suffix', { count: balance }), 'credit_balance'],
            [t('cr.kv_free_today'), quota
              ? `${Math.max(0, quota.free_limit - quota.free_remaining)} / ${quota.free_limit}`
              : '—',
              'free_tier'],
            [t('cr.kv_cost_per_run'), quota ? t('cr.cr_suffix', { count: quota.cost_per_run }) : '—'],
            [t('cr.kv_status'),
              quota?.can_run_free
                ? <span style={{ color: 'var(--ok)' }}>{t('cr.status_free')}</span>
                : quota?.can_run_paid
                  ? <span style={{ color: 'var(--accent)' }}>{t('cr.status_paid')}</span>
                  : <span style={{ color: 'var(--danger)' }}>{t('cr.status_exhausted')}</span>
            ],
          ]} />

          {!compact && (
            <>
              <div className="hr" style={{ margin: '12px 0' }} />
              <div className="t-xs muted" style={{ marginBottom: 6, letterSpacing: '0.08em' }}>
                {t('cr.recent_tx')}
              </div>
              {history.length === 0 ? (
                <div className="t-md muted" style={{ textAlign: 'center', padding: 12 }}>
                  {t('cr.no_tx')}
                </div>
              ) : (
                <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                  {/* The row parameter is `tx`, not `t` — it used to be `t`,
                      which would now shadow the translator. */}
                  {history.map((tx) => {
                    const positive = tx.amount > 0
                    return (
                      <div
                        key={tx.id}
                        style={{
                          display: 'grid',
                          gridTemplateColumns: '1fr auto auto',
                          alignItems: 'center',
                          gap: 8,
                          padding: '4px 6px',
                          borderTop: '1px solid var(--rule-2)',
                          fontSize: 10,
                        }}
                      >
                        <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                          <span style={{ color: 'var(--ink)', fontWeight: 600 }}>
                            {formatReason(tx.reason, t)}
                          </span>
                          {tx.reference && (
                            <span className="muted" style={{ marginLeft: 6 }}>
                              {tx.reference}
                            </span>
                          )}
                        </span>
                        <span style={{
                          color: positive ? 'var(--ok)' : 'var(--danger)',
                          fontWeight: 700,
                          minWidth: 50,
                          textAlign: 'right',
                        }}>
                          {positive ? '+' : ''}{tx.amount}
                        </span>
                        <span className="muted-2" style={{ minWidth: 110, textAlign: 'right' }}>
                          {formatTimestamp(tx.created_at)}
                        </span>
                      </div>
                    )
                  })}
                </div>
              )}
            </>
          )}
        </>
      )}
    </Panel>
  )
}
