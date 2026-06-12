import { useEffect, useState } from 'react'
import { useLocation } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { stagingAllows, stagingRouteExempt } from '../lib/stagingGate'

// StagingGate — the staging-review whitelist (r63, owner direction:
// sweeping UX ships to a whitelisted staging build for human review
// before production).
//
// Build-time switch: the gate is active ONLY when the bundle was built
// with VITE_STAGING set (VITE_STAGING=1 npx vite build — the
// `frontend-dev` deploy target). In production builds
// import.meta.env.VITE_STAGING is undefined, the early-return below is
// statically false, and the bundler drops the entire gate UI — prod is
// byte-for-byte unaffected by this component beyond the passthrough.
//
// Staging behavior:
//   - /login and /auth/* stay reachable signed-out (reviewers must be
//     able to complete the magic-link flow — without this exemption
//     nobody could ever satisfy the whitelist).
//   - a whitelisted authenticated session sees the full app, deck
//     editor included;
//   - anyone else gets the lost page with a button + auto-redirect to
//     the production site.

const REDIRECT_AFTER_MS = 10_000
const PROD_URL = 'https://hexdek.dev'

export default function StagingGate({ children }) {
  if (!import.meta.env.VITE_STAGING) {
    return children
  }
  return <StagingGateInner>{children}</StagingGateInner>
}

function StagingGateInner({ children }) {
  const { user, loading } = useAuth()
  const location = useLocation()

  if (stagingRouteExempt(location.pathname)) {
    return children
  }
  if (loading) {
    return (
      <div style={wrapStyle}>
        <div className="t-md muted">CHECKING CREDENTIALS<span className="blink">_</span></div>
      </div>
    )
  }
  if (user && stagingAllows(user.email)) {
    return children
  }
  return <LostPage />
}

function LostPage() {
  const [secondsLeft, setSecondsLeft] = useState(REDIRECT_AFTER_MS / 1000)

  useEffect(() => {
    const tick = setInterval(() => setSecondsLeft((s) => Math.max(0, s - 1)), 1000)
    const go = setTimeout(() => { window.location.href = PROD_URL }, REDIRECT_AFTER_MS)
    return () => { clearInterval(tick); clearTimeout(go) }
  }, [])

  return (
    <div style={wrapStyle}>
      <div style={{ maxWidth: 440, textAlign: 'center' }}>
        <div className="t-2xl" style={{ fontWeight: 700, marginBottom: 14 }}>
          hmm, you seem to have gotten lost
        </div>
        <div className="t-md muted" style={{ lineHeight: 1.6, marginBottom: 22 }}>
          this is a private preview build. the real HexDek lives over here:
        </div>
        <a
          href={PROD_URL}
          style={{
            display: 'inline-block', padding: '12px 22px',
            border: '1px solid var(--ink)', color: 'var(--ink)',
            textDecoration: 'none', fontWeight: 700, letterSpacing: '0.06em',
          }}
        >
          TAKE ME TO HEXDEK.DEV ▶
        </a>
        <div className="t-xs muted" style={{ marginTop: 18 }}>
          redirecting automatically in {secondsLeft}s…
        </div>
      </div>
    </div>
  )
}

const wrapStyle = {
  minHeight: '100vh', display: 'flex',
  alignItems: 'center', justifyContent: 'center', padding: 36,
}
