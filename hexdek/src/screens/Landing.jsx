import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { Panel, Tag, Btn, Tape } from '../components/chrome'
import { useGames } from '../hooks/useData'
import { useT } from '../i18n'
import { relativeAge, formatAge } from '../lib/relativeAge'

// Landing — / route. Visitor-facing entry point: hero, three feature
// pillars, recent-activity feed, import CTA.
//
// Style note: the codebase's brutalist tokens are var(--bg), var(--ink),
// var(--panel), var(--accent), --rule, --rule-2, --ok, --warn, --danger.
// (The spec mentioned --fg; the corresponding token here is --ink. Used
// throughout the chrome components.)

const FEATURES = [
  {
    icon: '◈',
    title: 'DECK ANALYSIS',
    desc: 'Freya scores archetype, win lines, mana base, combos, and roles per card.',
    to: '/decks',
    cta: 'BROWSE DECKS',
  },
  {
    icon: '⌬',
    title: 'GAME ENGINE',
    desc: 'Open-source MTG Commander rules engine running 4-player AI matches live.',
    to: '/spectate',
    cta: 'WATCH LIVE',
  },
  {
    icon: '☷',
    title: 'LEADERBOARD',
    desc: 'HexELO ratings + win-rate stats over thousands of showmatch games.',
    to: '/leaderboard',
    cta: 'SEE RANKINGS',
  },
]

export default function Landing() {
  const t = useT()
  const navigate = useNavigate()
  const { data: games, loading: gamesLoading } = useGames(5)
  // Subtle hero shimmer — pure CSS would be cleaner but the brutalist
  // chrome doesn't have a keyframes lib loaded; one-time mount opacity
  // bump is enough.
  const [mounted, setMounted] = useState(false)
  useEffect(() => { setMounted(true) }, [])

  return (
    <>
      <Tape
        left="LANDING / / DOC HX-001"
        mid="HEXDEK · COMMANDER INTELLIGENCE"
        right="REV C.25"
      />

      <div style={{ padding: '40px 30px 30px', maxWidth: 1100, margin: '0 auto' }}>

        {/* HERO */}
        <div style={{
          display: 'grid',
          gridTemplateColumns: 'minmax(0, 1fr) auto',
          alignItems: 'flex-end',
          gap: 24,
          paddingBottom: 28,
          borderBottom: '1px solid var(--rule-2)',
          marginBottom: 28,
          opacity: mounted ? 1 : 0,
          transition: 'opacity 280ms ease',
        }}>
          <div>
            <div className="t-xs muted" style={{ letterSpacing: '0.16em', marginBottom: 10 }}>
              HX-001 / / OPEN-SOURCE / / RUNS ON YOUR MACHINE
            </div>
            <h1 style={{
              margin: 0,
              fontSize: 'clamp(56px, 12vw, 128px)',
              fontWeight: 800,
              letterSpacing: '-0.06em',
              lineHeight: 0.85,
              color: 'var(--ink)',
            }}>
              HEXDEK<span style={{ color: 'var(--accent)' }}>/</span>
            </h1>
            <div className="t-md" style={{
              marginTop: 14,
              maxWidth: 640,
              lineHeight: 1.55,
              textTransform: 'uppercase',
              letterSpacing: '0.06em',
              fontSize: 14,
              color: 'var(--ink-2)',
            }}>
              MTG COMMANDER INTELLIGENCE PLATFORM. AI-DRIVEN MATCHES, FREYA-GRADE
              DECK ANALYSIS, AND A LEADERBOARD POWERED BY A RULES-AUTHORITATIVE
              GAME ENGINE.
            </div>
          </div>

          {/* Vertical brutalist crop block — pure decoration */}
          <div style={{
            display: 'flex', flexDirection: 'column', gap: 4,
            color: 'var(--rule-2)', alignSelf: 'stretch', justifyContent: 'flex-end',
          }} aria-hidden="true">
            <div style={{ width: 8, height: 80, background: 'var(--rule-2)' }} />
            <div style={{ width: 8, height: 22, background: 'var(--accent)' }} />
            <div style={{ width: 8, height: 40, background: 'var(--rule-2)' }} />
          </div>
        </div>

        {/* PRIMARY CTA — "Import Your Deck" */}
        <button
          onClick={() => navigate('/import')}
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: 18,
            width: '100%',
            padding: '20px 24px',
            background: 'var(--accent)',
            color: 'var(--inv-ink)',
            border: '1px solid var(--accent)',
            fontFamily: 'inherit',
            fontSize: 18,
            fontWeight: 800,
            letterSpacing: '0.10em',
            textTransform: 'uppercase',
            cursor: 'pointer',
            boxShadow: '4px 4px 0 var(--rule-2)',
            marginBottom: 32,
          }}
        >
          <span>IMPORT YOUR DECK</span>
          <span style={{ fontSize: 22 }}>↗</span>
        </button>

        {/* FEATURE CARDS */}
        <div className="grid col-3 gap-3" style={{ marginBottom: 32 }}>
          {FEATURES.map(f => (
            <Link
              key={f.title}
              to={f.to}
              className="panel"
              style={{
                padding: 18,
                textDecoration: 'none',
                color: 'var(--ink)',
                display: 'flex', flexDirection: 'column', gap: 10,
                minHeight: 180,
              }}
            >
              <div style={{
                fontSize: 36, lineHeight: 1, color: 'var(--accent)',
                fontWeight: 400,
              }} aria-hidden="true">{f.icon}</div>
              <div className="t-xl" style={{ fontWeight: 800, letterSpacing: '-0.01em' }}>{f.title}</div>
              <div className="t-md muted" style={{ flex: 1, lineHeight: 1.5 }}>{f.desc}</div>
              <div style={{
                display: 'flex', justifyContent: 'space-between', alignItems: 'center',
                paddingTop: 6, borderTop: '1px dashed var(--rule-2)', marginTop: 4,
              }}>
                <span className="t-xs" style={{ fontWeight: 800, color: 'var(--accent)', letterSpacing: '0.10em' }}>
                  {f.cta}
                </span>
                <span style={{ color: 'var(--accent)' }}>↗</span>
              </div>
            </Link>
          ))}
        </div>

        {/* RECENT ACTIVITY — last 5 games from /api/games */}
        <Panel
          code="LND.A"
          title="RECENT ACTIVITY / / LIVE FORGE"
          right={<Tag solid kind={games && games.length > 0 ? 'ok' : null}>{games?.length ?? 0}</Tag>}
        >
          {gamesLoading ? (
            <div className="t-md muted" style={{ padding: '14px 0', textAlign: 'center' }}>
              &gt; FETCHING RECENT GAMES<span className="blink">_</span>
            </div>
          ) : !games || games.length === 0 ? (
            <div className="t-md muted" style={{ padding: '14px 0', textAlign: 'center' }}>
              &gt; NO GAMES RECORDED YET — SHOWMATCH ENGINE OFFLINE.
            </div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column' }}>
              {games.slice(0, 5).map((g, i) => (
                <div
                  key={g.id || i}
                  style={{
                    display: 'grid',
                    gridTemplateColumns: '60px 1fr 80px 80px',
                    gap: 12,
                    padding: '10px 0',
                    borderBottom: i < Math.min(games.length, 5) - 1 ? '1px dashed var(--rule-2)' : 'none',
                    alignItems: 'center',
                  }}
                >
                  <span className="t-xs muted-2" style={{ fontVariantNumeric: 'tabular-nums' }}>{g.id}</span>
                  <div style={{ minWidth: 0 }}>
                    <div className="t-md" style={{
                      fontWeight: 700, lineHeight: 1.2,
                      overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                    }}>
                      {g.deck || t('dash.unknown')} <span className="muted-2">·</span> <span className="muted">{t('dash.vs_ai', { count: g.opponentCount })}</span>
                    </div>
                    <div className="t-xs muted" style={{ marginTop: 2 }}>{t('dash.game_detail', { turn: g.turns, reason: g.endReason })}</div>
                  </div>
                  <Tag kind={g.kind} solid>{t(g.won ? 'dash.result_win' : 'dash.result_draw')}</Tag>
                  <span className="t-xs muted text-right">{formatAge(t, relativeAge(g.finishedAt))}</span>
                </div>
              ))}
            </div>
          )}
          <div style={{ marginTop: 12, textAlign: 'right' }}>
            <Link to="/spectate" style={{ textDecoration: 'none' }}>
              <Btn ghost arrow="↗" sm>WATCH LIVE</Btn>
            </Link>
          </div>
        </Panel>

        {/* Footer-of-section line */}
        <div className="t-xs muted-2" style={{
          marginTop: 28, textAlign: 'center', letterSpacing: '0.12em',
        }}>
          + + + HEXDEK CORE READY · MIT-LICENSED · OPEN SOURCE + + +
        </div>
      </div>
    </>
  )
}
