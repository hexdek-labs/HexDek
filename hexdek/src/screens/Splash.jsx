import { lazy, Suspense, useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Panel, KV, Btn, Stripes, Tape } from '../components/chrome'
import { useAuth } from '../context/AuthContext'
import { useLiveSocket } from '../hooks/useLiveSocket'
import { AnimatedCounter } from '../hooks/useAnimatedCounter.jsx'
import { useUploadDeck } from '../hooks/useUploadDeck'
import { useT } from '../i18n'

// FishtankEmbed is the heaviest piece on the splash — full WS-driven
// re-render per snapshot. Code-split it so the hero paints first, and
// only mount once the section scrolls into view (poor-man's
// `client:visible`). useLiveSocket is a Context singleton, so deferring
// the mount also defers the WS handshake.
const FishtankEmbed = lazy(() => import('../components/FishtankEmbed'))

const RUNTIME_LABEL_KEYS = {
  disconnected: 'splash.runtime_disconnected',
  contacting: 'splash.runtime_contacting',
  initializing: 'splash.runtime_initializing',
}

export default function Splash() {
  const navigate = useNavigate()
  const { user } = useAuth()
  const { stats, elo, status } = useLiveSocket()
  const t = useT()
  const upload = useUploadDeck(() => navigate('/decks'))

  const gpm = stats?.games_per_min || 0
  const runtimeText = status === 'live'
    ? t('splash.runtime_live', { rate: gpm ? Math.round(gpm / 60).toLocaleString() : '?' })
    : (RUNTIME_LABEL_KEYS[status] ? t(RUNTIME_LABEL_KEYS[status]) : t('splash.runtime_offline'))

  const ledClass = status === 'live' ? 'led--on blink' :
    (status === 'contacting' || status === 'initializing') ? 'led--on' : ''

  return (
    <>
      <Tape left={t('splash.tape_left')} mid="REV C.25" right={t('splash.tape_right')} />

      <div className="splash-layout">
        {/* LEFT */}
        <div className="splash-left">
          <div>
            <div className="t-xs muted">{t('splash.doc_line')}</div>
            <div style={{ marginTop: 18, display: 'flex', alignItems: 'flex-start', gap: 18 }}>
              <div className="splash-hero">HEX</div>
              <Stripes height={148} w={140} />
            </div>
            <div className="splash-hero" style={{ marginTop: -12 }}>DEK/</div>

            <div className="t-md muted" style={{ marginTop: 24, maxWidth: 540, lineHeight: 1.6, textTransform: 'uppercase', letterSpacing: '0.04em', fontSize: 13 }}>
              &gt; {t('splash.tagline_1')}
              <br />
              &gt; {t('splash.tagline_2')}
              <br />
              &gt; {t('splash.tagline_3')}
            </div>
          </div>

          {/* Hero CTA — primary conversion: upload a deck */}
          <button
            onClick={upload.open}
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              gap: 18,
              width: '100%',
              maxWidth: 540,
              padding: '18px 22px',
              background: 'var(--accent)',
              color: 'var(--bg)',
              border: '1px solid var(--accent)',
              fontFamily: 'inherit',
              fontSize: 20,
              fontWeight: 800,
              letterSpacing: '0.08em',
              textTransform: 'uppercase',
              cursor: 'pointer',
              boxShadow: '4px 4px 0 var(--rule-2)',
              transition: 'transform 80ms ease, box-shadow 80ms ease',
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.transform = 'translate(-2px, -2px)'
              e.currentTarget.style.boxShadow = '6px 6px 0 var(--rule-2)'
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.transform = 'translate(0, 0)'
              e.currentTarget.style.boxShadow = '4px 4px 0 var(--rule-2)'
            }}
          >
            <span style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
              <span style={{ fontSize: 28, lineHeight: 1, fontWeight: 900 }}>+</span>
              <span>{t('splash.cta_upload')}</span>
            </span>
            <span style={{ fontSize: 22, lineHeight: 1 }}>▶</span>
          </button>

          <div style={{ display: 'flex', gap: 14, alignItems: 'center', flexWrap: 'wrap' }}>
            <Btn solid arrow="▶" onClick={() => navigate(user ? '/dash' : '/login')}>{t('splash.btn_enter')}</Btn>
            <a href="https://github.com/hexdek-labs/HexDek#readme" target="_blank" rel="noopener noreferrer" style={{ textDecoration: 'none' }}><Btn ghost arrow="↗">{t('splash.btn_docs')}</Btn></a>
            <a href="https://github.com/hexdek-labs/HexDek" target="_blank" rel="noopener noreferrer" style={{ textDecoration: 'none' }}><Btn ghost arrow="↗">{t('splash.btn_github')}</Btn></a>
            <a href="https://discord.gg/Mz2ueRFXds" target="_blank" rel="noopener noreferrer" style={{ textDecoration: 'none' }}><Btn ghost arrow="↗">{t('nav.discord')}</Btn></a>
          </div>
        </div>

        {/* RIGHT */}
        <div className="splash-right">
          <Panel code="II.A" title={t('splash.panel_stats')} right={<span className={`led ${ledClass}`} />}>
            <KV rows={[
              [t('splash.kv_games'), <AnimatedCounter target={stats?.games_played} rate={gpm} className="punch" style={{ fontSize: 24 }} />],
              [t('splash.kv_gpm'), gpm ? Math.round(gpm).toLocaleString() : '—'],
              [t('splash.kv_turns'), stats ? stats.avg_turns : '—'],
              [t('splash.kv_dominant'), stats?.dominant?.split(',')[0]?.toUpperCase() || '—'],
              [t('splash.kv_top_wr'), stats ? `${stats.dominant_win_rate}%` : '—'],
              [t('splash.kv_elo_pool'), t('splash.elo_pool_value', { count: elo.length })],
            ]} />
          </Panel>

          <div className="panel inv" style={{ padding: 0 }}>
            <div className="panel-hd" style={{ borderColor: 'rgba(0,0,0,0.2)' }}>
              <span>{t('splash.builds_hd')}</span>
              <span>SLOT.01</span>
            </div>
            <div style={{ padding: '14px 16px', textAlign: 'center' }}>
              <div style={{ fontSize: 11, letterSpacing: '0.06em', lineHeight: 1.6 }}>
                {t('splash.core_line')}<br />
                {t('splash.engine_line')}<br />
                {t('splash.format_line')}<br />
                {t('splash.runtime_label')} {runtimeText}
              </div>
              <div style={{ borderTop: '1px solid rgba(0,0,0,0.15)', marginTop: 12, paddingTop: 8, fontSize: 10, letterSpacing: '0.1em' }}>
                HEXDEK__©2026
              </div>
            </div>
          </div>

          <Panel code="II.B" title={t('splash.notice_hd')}>
            <div className="t-md muted" style={{ lineHeight: 1.6 }}>
              &gt; {t('splash.notice_1')}<br />
              &gt; {t('splash.notice_2')}<br />
              &gt; {t('splash.notice_3')}<br />
              &gt; {t('splash.notice_4')}
            </div>
          </Panel>
        </div>
      </div>

      {/* Full-width live fishtank section, below the hero. Lazy-mounted
          via WhenVisible so the WS handshake doesn't compete with the
          first paint. The status text re-uses the same useLiveSocket
          context the right column already drives, so a single shared
          WS powers both regions. */}
      <section className="splash-fishtank">
        <Tape
          left={t('splash.fishtank_tape_left')}
          mid={status === 'live'
            ? t('splash.fishtank_live', { rate: gpm ? Math.round(gpm).toLocaleString() : '?' })
            : (RUNTIME_LABEL_KEYS[status] ? t(RUNTIME_LABEL_KEYS[status]) : t('splash.runtime_offline'))}
          right={t('splash.fishtank_watch')}
        />
        <div className="splash-fishtank-body">
          <WhenVisible
            placeholder={
              <div className="fishtank-embed fishtank-embed--state" style={{ maxHeight: 240 }}>
                <div className="fishtank-embed-hd">
                  <span>{t('splash.fishtank_hd')}</span>
                  <span className="fishtank-embed-badge">
                    <span className="led" /> {t('splash.fishtank_standby')}
                  </span>
                </div>
                <div className="fishtank-embed-empty">
                  &gt; {t('splash.fishtank_scroll')}<span className="blink">_</span>
                </div>
              </div>
            }
          >
            <Suspense fallback={
              <div className="fishtank-embed fishtank-embed--state" style={{ maxHeight: 240 }}>
                <div className="fishtank-embed-hd">
                  <span>{t('splash.fishtank_hd')}</span>
                  <span className="fishtank-embed-badge">
                    <span className="led led--on blink" /> {t('common.loading')}
                  </span>
                </div>
                <div className="fishtank-embed-empty">
                  &gt; {t('splash.fishtank_loading')}<span className="blink">_</span>
                </div>
              </div>
            }>
              <FishtankEmbed />
            </Suspense>
          </WhenVisible>
        </div>
      </section>

      {upload.modal}
    </>
  )
}

// WhenVisible defers rendering its children until the placeholder is
// scrolled into (or near) the viewport. Useful for deferring expensive
// mounts — here, the FishtankEmbed's WS-driven snapshot loop. Once
// mounted the children persist (no unmount on scroll-out).
function WhenVisible({ children, placeholder = null, rootMargin = '200px' }) {
  const [visible, setVisible] = useState(false)
  const ref = useRef(null)
  useEffect(() => {
    if (visible) return
    if (typeof IntersectionObserver === 'undefined') {
      setVisible(true)
      return
    }
    const node = ref.current
    if (!node) return
    const obs = new IntersectionObserver((entries) => {
      for (const e of entries) {
        if (e.isIntersecting) {
          setVisible(true)
          obs.disconnect()
          return
        }
      }
    }, { rootMargin })
    obs.observe(node)
    return () => obs.disconnect()
  }, [visible, rootMargin])
  return <div ref={ref}>{visible ? children : placeholder}</div>
}
