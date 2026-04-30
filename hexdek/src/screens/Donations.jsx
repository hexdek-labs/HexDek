import { useState, useEffect } from 'react'
import { Panel, KV, Bar, Btn, Tape, Tag } from '../components/chrome'
import { api } from '../services/api'

const RECURRING = [
  ['CLAUDE AI', 'Engine development + analysis', 200],
  ['DOMAIN', 'hexdek.dev (annual, amortized)', 2],
]

const SUNK = [
  ['DARKSTAR', 'Ryzen 9 9950X engine box', 2000],
  ['CLAUDE (3 MO)', 'Development to date', 600],
  ['DOMAIN (PAID)', 'hexdek.dev registration', 20],
]

const MONTHLY_GOAL = RECURRING.reduce((s, r) => s + r[2], 0)
const TOTAL_INVESTED = SUNK.reduce((s, r) => s + r[2], 0)

export default function Donations() {
  const [summary, setSummary] = useState({ month_total: 0, all_time_total: 0, month_goal: MONTHLY_GOAL, recent: [] })

  useEffect(() => {
    api.getDonationsSummary().then(setSummary).catch(() => {})
  }, [])

  const raised = summary.month_total
  const goal = summary.month_goal || MONTHLY_GOAL
  const pct = Math.min(100, Math.round((raised / goal) * 100))

  return (
    <div style={{ padding: '20px 30px', maxWidth: 800, margin: '0 auto' }}>
      <Panel code="DON.0" title="DONATIONS // KEEP HEXDEK ALIVE">
        <div className="t-md" style={{ lineHeight: 1.7 }}>
          <p>
            HexDek runs on dedicated hardware and AI-powered development. There are no forced ads,
            no subscriptions, and no data sales. Every dollar goes directly to keeping the
            engine running and the simulation queue chewing through games.
          </p>
        </div>
      </Panel>

      <Panel code="DON.1" title="MONTHLY OPERATING COSTS" style={{ marginTop: 16 }}>
        <div style={{ padding: '4px 0 12px' }}>
          <Tape
            left={`$${raised.toFixed(0)} RAISED`}
            mid={`${pct}%`}
            right={`$${goal}/MO GOAL`}
          />
          <div style={{ marginTop: 12 }}>
            <Bar value={raised} max={goal} lg />
          </div>
        </div>
        <KV rows={RECURRING.map(([k, v, c]) => [k, `${v} — $${c}/mo`])} />
        <div style={{ marginTop: 12, paddingTop: 10, borderTop: '1px dashed var(--rule-2)' }}>
          <KV rows={[['MONTHLY TOTAL', `$${MONTHLY_GOAL}/mo`]]} />
        </div>
        <div className="t-xs muted" style={{ marginTop: 10 }}>
          Surplus rolls into infrastructure upgrades and tournament prize pools.
        </div>
      </Panel>

      {summary.recent.length > 0 && (
        <Panel code="DON.R" title="RECENT SUPPORTERS" style={{ marginTop: 16 }}>
          {summary.recent.map((d, i) => (
            <div key={i} style={{ display: 'flex', justifyContent: 'space-between', padding: '6px 0', borderBottom: i < summary.recent.length - 1 ? '1px dashed var(--rule-2)' : 'none' }}>
              <span className="t-md" style={{ fontWeight: 700 }}>{d.from_name}</span>
              <span className="t-md" style={{ color: 'var(--ok)' }}>${d.amount}</span>
            </div>
          ))}
        </Panel>
      )}

      <Panel code="DON.2" title="ALREADY INVESTED" style={{ marginTop: 16 }}>
        <KV rows={SUNK.map(([k, v, c]) => [k, `${v} — $${c}`])} />
        <div style={{ marginTop: 12, paddingTop: 10, borderTop: '1px dashed var(--rule-2)' }}>
          <KV rows={[['TOTAL TO DATE', `$${(TOTAL_INVESTED + summary.all_time_total).toLocaleString()}`]]} />
        </div>
        <div className="t-xs muted" style={{ marginTop: 10 }}>
          DARKSTAR is committed hardware — amortized, not a recurring cost.
          {summary.all_time_total > 0 && ` Community has contributed $${summary.all_time_total.toFixed(0)} total.`}
        </div>
      </Panel>

      <Panel code="DON.3" title="WAYS TO SUPPORT" style={{ marginTop: 16 }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div className="t-md" style={{ lineHeight: 1.6 }}>
            Three ways to keep HexDek running. Pick what works for you.
          </div>

          <div style={{ display: 'flex', gap: 14, flexWrap: 'wrap' }}>
            <div className="panel" style={{ flex: 1, minWidth: 180, padding: '14px 16px' }}>
              <Tag solid>DONATE</Tag>
              <div className="t-md" style={{ marginTop: 8, lineHeight: 1.5 }}>
                One-time tips or recurring sponsorships. Goes directly to infrastructure and AI costs.
              </div>
              <div style={{ display: 'flex', gap: 8, marginTop: 12, flexWrap: 'wrap' }}>
                <a href="https://ko-fi.com/hexdek" target="_blank" rel="noopener noreferrer" style={{ textDecoration: 'none' }}>
                  <Btn solid>KO-FI</Btn>
                </a>
              </div>
            </div>

            <div className="panel" style={{ flex: 1, minWidth: 180, padding: '14px 16px' }}>
              <Tag>COMPUTE</Tag>
              <div className="t-md" style={{ marginTop: 8, lineHeight: 1.5 }}>
                Contribute CPU cycles to run simulations. Earn research tokens for your own deck analysis.
              </div>
              <div style={{ marginTop: 12 }}>
                <Btn ghost>COMING SOON</Btn>
              </div>
            </div>

            <div className="panel" style={{ flex: 1, minWidth: 180, padding: '14px 16px' }}>
              <Tag>WATCH ADS</Tag>
              <div className="t-md" style={{ marginTop: 8, lineHeight: 1.5 }}>
                Voluntary only. Watch a short ad to earn research tokens. Never forced, never intrusive.
              </div>
              <div style={{ marginTop: 12 }}>
                <Btn ghost>COMING SOON</Btn>
              </div>
            </div>
          </div>

          <div className="t-xs muted-2">
            Research tokens unlock extended deck analysis, priority simulation queue, and custom tournament runs.
          </div>
        </div>
      </Panel>

      <Panel code="DON.4" title="PHILOSOPHY" style={{ marginTop: 16 }}>
        <div className="t-md" style={{ lineHeight: 1.7 }}>
          <p><strong>No forced ads. No paywalls. No data sales.</strong></p>
          <p style={{ marginTop: 8 }}>
            HexDek is community-powered. Engine code is MIT-licensed and the data is yours.
            Power users who contribute — whether through compute, donations, or community engagement —
            get rewarded with research tokens and priority access. But the core experience is always free.
          </p>
        </div>
      </Panel>
    </div>
  )
}
