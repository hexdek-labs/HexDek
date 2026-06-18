import { useEffect, useState } from 'react'
import { useLocation } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import {
  stagingAllows, stagingRouteExempt,
  passphraseEnabled, passphraseMatches, previewParamMatches, STAGING_PASS_STORAGE_KEY,
} from '../lib/stagingGate'

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

// The shared reviewer passphrase, baked in at build time. Empty/unset
// (including every prod build) disables the bypass entirely.
const EXPECTED_PASS = import.meta.env.VITE_STAGING_PASSPHRASE || ''

function hasStoredPassGrant() {
  try {
    // The stored value must equal the CURRENT build's passphrase, so a
    // passphrase rotation at the next deploy invalidates old grants.
    return passphraseMatches(localStorage.getItem(STAGING_PASS_STORAGE_KEY), EXPECTED_PASS)
  } catch {
    return false // storage blocked (private mode) — fall back to prompting
  }
}

// A correct ?preview=<passphrase> in the current URL grants access
// directly — read synchronously at mount so a shared link never flashes
// the lost page before the effect below persists the grant.
function hasPreviewParamGrant() {
  try {
    return previewParamMatches(window.location.search, EXPECTED_PASS)
  } catch {
    return false
  }
}

function StagingGateInner({ children }) {
  const { user, loading } = useAuth()
  const location = useLocation()
  const [passGranted, setPassGranted] = useState(() => hasStoredPassGrant() || hasPreviewParamGrant())

  // A correct ?preview=<passphrase> link persists the grant so it
  // survives reloads and in-app navigation that drop the query string.
  useEffect(() => {
    if (previewParamMatches(location.search, EXPECTED_PASS)) {
      try { localStorage.setItem(STAGING_PASS_STORAGE_KEY, EXPECTED_PASS) } catch { /* private mode: grant lasts this load only */ }
      setPassGranted(true)
    }
  }, [location.search])

  if (stagingRouteExempt(location.pathname)) {
    return children
  }
  if (passGranted) {
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
  return (
    <LostPage
      onPassGranted={() => {
        try { localStorage.setItem(STAGING_PASS_STORAGE_KEY, EXPECTED_PASS) } catch { /* private mode: grant lasts this load only */ }
        setPassGranted(true)
      }}
    />
  )
}

function LostPage({ onPassGranted }) {
  const [secondsLeft, setSecondsLeft] = useState(REDIRECT_AFTER_MS / 1000)
  // Once the reviewer touches the passphrase field, the auto-redirect
  // is cancelled — nothing worse than being yanked to prod mid-typing.
  const [redirectPaused, setRedirectPaused] = useState(false)
  const [passInput, setPassInput] = useState('')
  const [passError, setPassError] = useState(false)

  useEffect(() => {
    if (redirectPaused) return undefined
    const tick = setInterval(() => setSecondsLeft((s) => Math.max(0, s - 1)), 1000)
    const go = setTimeout(() => { window.location.href = PROD_URL }, REDIRECT_AFTER_MS)
    return () => { clearInterval(tick); clearTimeout(go) }
  }, [redirectPaused])

  const submitPass = (e) => {
    e.preventDefault()
    if (passphraseMatches(passInput, EXPECTED_PASS)) {
      onPassGranted()
    } else {
      setPassError(true)
    }
  }

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
        {!redirectPaused && (
          <div className="t-xs muted" style={{ marginTop: 18 }}>
            redirecting automatically in {secondsLeft}s…
          </div>
        )}
        {passphraseEnabled(EXPECTED_PASS) && (
          <form onSubmit={submitPass} style={{ marginTop: 30 }}>
            <div className="t-xs muted" style={{ marginBottom: 8 }}>
              reviewer? enter the staging passphrase:
            </div>
            <input
              type="password"
              value={passInput}
              autoComplete="off"
              onFocus={() => setRedirectPaused(true)}
              onChange={(e) => { setPassInput(e.target.value); setPassError(false); setRedirectPaused(true) }}
              style={{
                padding: '10px 12px', width: 220,
                border: passError ? '1px solid #c0392b' : '1px solid var(--ink)',
                background: 'transparent', color: 'var(--ink)',
              }}
            />
            <button
              type="submit"
              style={{
                marginLeft: 8, padding: '10px 16px',
                border: '1px solid var(--ink)', background: 'transparent',
                color: 'var(--ink)', fontWeight: 700, cursor: 'pointer',
              }}
            >
              ENTER
            </button>
            {passError && (
              <div className="t-xs" style={{ color: '#c0392b', marginTop: 8 }}>
                that's not it — check with Josh or 7174n1c
              </div>
            )}
          </form>
        )}
      </div>
    </div>
  )
}

const wrapStyle = {
  minHeight: '100vh', display: 'flex',
  alignItems: 'center', justifyContent: 'center', padding: 36,
}
