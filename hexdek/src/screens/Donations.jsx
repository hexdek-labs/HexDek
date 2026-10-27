import { useState, useEffect } from 'react'
import { Panel, KV, Bar, Btn, Tape, Tag } from '../components/chrome'
import { api } from '../services/api'
import { useT } from '../i18n'
import { fill } from '../components/fill'

// Line items are [catalogKey, dollars]. Holding the English label here and
// reading it at render would freeze the words at module load and ignore a
// language switch, so the key is what's stored.
const RECURRING = [
  ['don.line_claude', 200],
  ['don.line_domain', 2],
]

const SUNK = [
  ['don.sunk_hw', 2000],
  ['don.sunk_claude', 600],
  ['don.sunk_domain', 20],
]

const MONTHLY_GOAL = RECURRING.reduce((s, r) => s + r[1], 0)
const TOTAL_INVESTED = SUNK.reduce((s, r) => s + r[1], 0)

const LINK_STYLE = {
  color: 'var(--ok)', textDecoration: 'none', borderBottom: '1px dotted var(--ok)',
}

export default function Donations() {
  const t = useT()
  const [summary, setSummary] = useState({ month_total: 0, all_time_total: 0, month_goal: MONTHLY_GOAL, recent: [] })

  useEffect(() => {
    api.getDonationsSummary().then(setSummary).catch(() => {})
  }, [])

  const raised = summary.month_total
  const goal = summary.month_goal || MONTHLY_GOAL
  const pct = Math.min(100, Math.round((raised / goal) * 100))

  return (
    <div style={{ padding: '20px 30px', maxWidth: 800, margin: '0 auto' }}>
      <Panel code="DON.0" title={t('don.title')}>
        <div className="t-md" style={{ lineHeight: 1.7 }}>
          <p>{t('don.intro')}</p>
        </div>
      </Panel>

      <Panel code="DON.1" title={t('don.costs_title')} style={{ marginTop: 16 }}>
        <div style={{ padding: '4px 0 12px' }}>
          <Tape
            left={t('don.raised', { amount: raised.toFixed(0) })}
            mid={`${pct}%`}
            right={t('don.goal', { amount: goal })}
          />
          <div style={{ marginTop: 12 }}>
            <Bar value={raised} max={goal} lg />
          </div>
        </div>
        <KV rows={RECURRING.map(([k, c]) => [t(k), t('don.per_mo', { amount: c })])} />
        <div style={{ marginTop: 12, paddingTop: 10, borderTop: '1px dashed var(--rule-2)' }}>
          <KV rows={[[t('don.monthly_total'), t('don.per_mo', { amount: MONTHLY_GOAL })]]} />
        </div>
        <div className="t-xs muted" style={{ marginTop: 10 }}>
          {t('don.surplus')}
        </div>
      </Panel>

      {summary.recent.length > 0 && (
        <Panel code="DON.R" title={t('don.recent')} style={{ marginTop: 16 }}>
          {summary.recent.map((d, i) => (
            <div key={i} style={{ display: 'flex', justifyContent: 'space-between', padding: '6px 0', borderBottom: i < summary.recent.length - 1 ? '1px dashed var(--rule-2)' : 'none' }}>
              <span className="t-md" style={{ fontWeight: 700 }}>{d.from_name}</span>
              <span className="t-md" style={{ color: 'var(--ok)' }}>${d.amount}</span>
            </div>
          ))}
        </Panel>
      )}

      <Panel code="DON.2" title={t('don.invested')} style={{ marginTop: 16 }}>
        <KV rows={SUNK.map(([k, c]) => [t(k), `$${c.toLocaleString()}`])} />
        <div style={{ marginTop: 12, paddingTop: 10, borderTop: '1px dashed var(--rule-2)' }}>
          <KV rows={[[t('don.total_to_date'), `$${(TOTAL_INVESTED + summary.all_time_total).toLocaleString()}`]]} />
        </div>
        <div className="t-xs muted" style={{ marginTop: 10 }}>
          {t('don.hw_note')}
          {summary.all_time_total > 0 && ` ${t('don.community_note', { amount: summary.all_time_total.toFixed(0) })}`}
        </div>
      </Panel>

      <Panel code="DON.3" title={t('don.ways')} style={{ marginTop: 16 }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div className="t-md" style={{ lineHeight: 1.6 }}>
            {t('don.ways_intro')}
          </div>

          <div style={{ display: 'flex', gap: 14, flexWrap: 'wrap' }}>
            <div className="panel" style={{ flex: 1, minWidth: 180, padding: '14px 16px' }}>
              <Tag solid>{t('don.donate')}</Tag>
              <div className="t-md" style={{ marginTop: 8, lineHeight: 1.5 }}>
                {t('don.donate_body')}
              </div>
              <div style={{ display: 'flex', gap: 8, marginTop: 12, flexWrap: 'wrap' }}>
                <a href="https://ko-fi.com/hexdek" target="_blank" rel="noopener noreferrer" style={{ textDecoration: 'none' }}>
                  <Btn solid>KO-FI</Btn>
                </a>
              </div>
            </div>

            <div className="panel" style={{ flex: 1, minWidth: 220, padding: '14px 16px', borderColor: 'var(--ink)' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 8 }}>
                <Tag solid>{t('don.boinc')}</Tag>
                <Tag kind="warn" solid style={{ fontSize: 8, padding: '1px 5px' }}>{t('don.coming_soon')}</Tag>
              </div>
              <div className="t-md" style={{ marginTop: 10, lineHeight: 1.55 }}>
                {t('don.boinc_body')}
              </div>
              <ul style={{
                marginTop: 10, marginBottom: 0, paddingLeft: 0, listStyle: 'none',
                fontSize: 10, letterSpacing: '0.04em', lineHeight: 1.7, color: 'var(--ink-2)',
              }}>
                <li>&gt; {t('don.boinc_li1')}</li>
                <li>&gt; {t('don.boinc_li2')}</li>
                <li>&gt; {t('don.boinc_li3')}</li>
              </ul>
              <div style={{ marginTop: 12, paddingTop: 10, borderTop: '1px dashed var(--rule-2)' }}>
                <span className="t-xs muted">{t('don.boinc_note')}</span>
              </div>
            </div>

            <div className="panel" style={{ flex: 1, minWidth: 220, padding: '14px 16px', borderColor: 'var(--ink)' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 8 }}>
                <Tag solid kind="ok">{t('don.support_dev')}</Tag>
                <span className="t-xs" style={{ color: 'var(--ok)', letterSpacing: '0.1em' }}>● {t('don.live')}</span>
              </div>
              <div className="t-md" style={{ marginTop: 10, lineHeight: 1.55 }}>
                {t('don.support_body')}
              </div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 8, marginTop: 12 }}>
                <a href="https://ko-fi.com/hexdek" target="_blank" rel="noopener noreferrer" style={{ textDecoration: 'none' }}>
                  <Btn solid arrow="↗" style={{ width: '100%', justifyContent: 'space-between' }}>{t('don.btn_kofi_once')}</Btn>
                </a>
                <a href="https://github.com/sponsors/hexdek-labs" target="_blank" rel="noopener noreferrer" style={{ textDecoration: 'none' }}>
                  <Btn arrow="↗" style={{ width: '100%', justifyContent: 'space-between' }}>{t('don.btn_gh')}</Btn>
                </a>
              </div>
            </div>
          </div>

          <div className="t-xs muted-2">
            {t('don.tokens_note')}
          </div>
        </div>
      </Panel>

      <Panel code="DON.4" title={t('don.philosophy')} style={{ marginTop: 16 }}>
        <div className="t-md" style={{ lineHeight: 1.7 }}>
          <p><strong>{t('don.phil_strong')}</strong></p>
          <p style={{ marginTop: 8 }}>{t('don.phil_body')}</p>
        </div>
      </Panel>

      <Panel code="DON.5" title={t('don.faq')} style={{ marginTop: 16 }}>
        <FAQItem
          q={t('don.faq_money_q')}
          a={fill(t('don.faq_money_a'), {
            claude: <strong>{t('don.faq_money_claude')}</strong>,
            domain: <strong>{t('don.faq_money_domain')}</strong>,
          })}
        />
        <FAQItem
          q={t('don.faq_oss_q')}
          a={fill(t('don.faq_oss_a'), {
            mit: <strong>{t('don.faq_oss_mit')}</strong>,
            link: (
              <a
                href="https://github.com/hexdek-labs/HexDek"
                target="_blank"
                rel="noopener noreferrer"
                style={LINK_STYLE}
              >
                github.com/hexdek-labs/HexDek ↗
              </a>
            ),
          })}
        />
        <FAQItem q={t('don.faq_pay_q')} a={t('don.faq_pay_a')} />
        <FAQItem
          q={t('don.faq_refund_q')}
          a={fill(t('don.faq_refund_a'), {
            discord: (
              <a
                href="https://discord.gg/Mz2ueRFXds"
                target="_blank"
                rel="noopener noreferrer"
                style={LINK_STYLE}
              >
                {t('don.faq_refund_discord')} ↗
              </a>
            ),
          })}
          last
        />
      </Panel>
    </div>
  )
}

function FAQItem({ q, a, last }) {
  return (
    <div style={{
      padding: '12px 0',
      borderBottom: last ? 'none' : '1px dashed var(--rule-2)',
    }}>
      <div className="t-sm" style={{
        color: 'var(--ink)',
        fontWeight: 700,
        letterSpacing: '0.06em',
        marginBottom: 6,
      }}>
        &gt; {q}
      </div>
      <div className="t-md" style={{ lineHeight: 1.6, color: 'var(--ink-2)' }}>
        {a}
      </div>
    </div>
  )
}
