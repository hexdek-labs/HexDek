import { useNavigate } from 'react-router-dom'
import { Panel, KV, Btn, Stripes, MiniBars, Tape } from '../components/chrome'
import { useAuth } from '../context/AuthContext'
import { useLiveSocket } from '../hooks/useLiveSocket'
import { AnimatedCounter } from '../hooks/useAnimatedCounter.jsx'

const RUNTIME_LABELS = {
  disconnected: 'DISCONNECTED',
  contacting: 'CONTACTING FORGE...',
  initializing: 'INITIALIZING LINK...',
}

export default function Splash() {
  const navigate = useNavigate()
  const { user } = useAuth()
  const { stats, elo, status } = useLiveSocket()

  const gpm = stats?.games_per_min || 0
  const runtimeText = status === 'live'
    ? `LIVE // ${gpm ? Math.round(gpm / 60).toLocaleString() : '?'} GAMES/SEC`
    : RUNTIME_LABELS[status] || 'OFFLINE'

  const ledClass = status === 'live' ? 'led--on blink' :
    (status === 'contacting' || status === 'initializing') ? 'led--on' : ''

  return (
    <>
      <Tape left="LANDING / / DOC HX-001" mid="REV C.25" right="FORGE TERMINAL" />

      <div className="splash-layout">
        {/* LEFT */}
        <div className="splash-left">
          <div>
            <div className="t-xs muted">DOC. HX-001 / / FORGE TERMINAL / / REV. C.25</div>
            <div style={{ marginTop: 18, display: 'flex', alignItems: 'flex-start', gap: 18 }}>
              <div className="splash-hero">HEX</div>
              <Stripes height={148} w={140} />
            </div>
            <div className="splash-hero" style={{ marginTop: -12 }}>DEK/</div>

            <div className="t-md muted" style={{ marginTop: 24, maxWidth: 540, lineHeight: 1.6, textTransform: 'uppercase', letterSpacing: '0.04em', fontSize: 13 }}>
              &gt; THE FORGE WHERE DECKS BECOME WEAPONS.
              <br />
              &gt; OPEN-SOURCE COMMANDER ANALYSIS ENGINE.
              <br />
              &gt; WASM-NATIVE. RUNS ON YOUR MACHINE.
            </div>
          </div>

          <div style={{ display: 'flex', gap: 14, alignItems: 'center', flexWrap: 'wrap' }}>
            <Btn solid arrow="▶" onClick={() => navigate(user ? '/dash' : '/login')}>ENTER THE FORGE</Btn>
            <a href="https://github.com/hexdek-labs/HexDek#readme" target="_blank" rel="noopener noreferrer" style={{ textDecoration: 'none' }}><Btn ghost arrow="↗">DOCS / / README</Btn></a>
            <a href="https://github.com/hexdek-labs/HexDek" target="_blank" rel="noopener noreferrer" style={{ textDecoration: 'none' }}><Btn ghost arrow="↗">GITHUB / / SRC</Btn></a>
            <a href="https://discord.gg/Mz2ueRFXds" target="_blank" rel="noopener noreferrer" style={{ textDecoration: 'none' }}><Btn ghost arrow="↗">DISCORD</Btn></a>
          </div>
        </div>

        {/* RIGHT */}
        <div className="splash-right">
          <div className="panel inv" style={{ padding: 0 }}>
            <div className="panel-hd" style={{ borderColor: 'rgba(0,0,0,0.2)' }}>
              <span>CATALOGUED BUILDS C.25</span>
              <span>SLOT.01</span>
            </div>
            <div style={{ padding: '18px 16px', textAlign: 'center' }}>
              <div style={{ fontSize: 11, letterSpacing: '0.06em', lineHeight: 1.6 }}>
                HEXDEK COMBAT CORE<br />
                ENGINE: HEXDEK V0.10D<br />
                FORMAT: COMMANDER / 1V1 / ARCHENEMY<br />
                RUNTIME: {runtimeText}
              </div>
              <div style={{ borderTop: '1px solid rgba(0,0,0,0.15)', marginTop: 14, paddingTop: 10, fontSize: 10, letterSpacing: '0.1em' }}>
                HEXDEK__©2026
              </div>
            </div>
          </div>

          <Panel code="II.A" title="LIVE FORGE STATS" right={<span className={`led ${ledClass}`} />}>
            <KV rows={[
              ['GAMES SIM.', <AnimatedCounter target={stats?.games_played} rate={gpm} className="punch" style={{ fontSize: 24 }} />],
              ['GAMES/MIN', gpm ? Math.round(gpm).toLocaleString() : '—'],
              ['AVG TURNS', stats ? stats.avg_turns : '—'],
              ['DOMINANT', stats?.dominant?.split(',')[0]?.toUpperCase() || '—'],
              ['TOP WIN RATE', stats ? `${stats.dominant_win_rate}%` : '—'],
              ['ELO POOL', `${elo.length} DECKS`],
            ]} />
          </Panel>

          <Panel code="II.B" title="SYS NOTICE">
            <div className="t-md muted" style={{ lineHeight: 1.6 }}>
              &gt; OPEN SOURCE.<br />
              &gt; DONATIONS-POWERED.<br />
              &gt; NO ADS. NO PAYWALLS.<br />
              &gt; LOGIN OPTIONAL — GUEST = FULL ANALYSIS.
            </div>
          </Panel>
        </div>
      </div>
    </>
  )
}
