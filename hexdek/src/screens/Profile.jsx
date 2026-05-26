import { useState, useEffect } from 'react'
import { Panel, Btn, KV } from '../components/chrome'
import { useAuth } from '../context/AuthContext'

const inputStyle = {
  width: '100%', padding: '8px 10px', background: 'var(--bg-2)', border: '1px solid var(--rule-2)',
  color: 'var(--ink)', fontFamily: 'inherit', fontSize: 12, letterSpacing: '0.02em',
}

// Server-side preferences sync — half-finished-features-r48 #8 closure.
// API is GET/PATCH /api/me/preferences (auth-gated via internal/preferences).
// When the call 401s or fails (network down, auth model not wired, dev-
// mode localhost without a session) we fall back to localStorage so the
// screen keeps working — same shape as the pre-r60 localStorage-only flow.
const API_BASE = (import.meta.env && import.meta.env.VITE_API_URL) ?? ''
const PREFS_URL = `${API_BASE}/api/me/preferences`

async function fetchRemotePrefs() {
  try {
    const res = await fetch(PREFS_URL, { credentials: 'include' })
    if (!res.ok) return null
    return await res.json()
  } catch {
    return null
  }
}

async function patchRemotePrefs(body) {
  try {
    const res = await fetch(PREFS_URL, {
      method: 'PATCH',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    })
    return res.ok
  } catch {
    return false
  }
}

export default function Profile() {
  const { user } = useAuth()
  const [displayName, setDisplayName] = useState('')
  const [owner, setOwner] = useState('')
  const [saved, setSaved] = useState(false)
  // syncStatus: "" (unknown), "synced" (last load came from server),
  // "local" (last load fell back to localStorage). Drives the footer
  // copy so users know whether their prefs persist cross-device.
  const [syncStatus, setSyncStatus] = useState('')

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      const remote = await fetchRemotePrefs()
      if (cancelled) return
      if (remote) {
        // Server is authoritative on load. Cache the values in
        // localStorage so an offline reload still populates the
        // inputs sensibly.
        setDisplayName(remote.display_name || '')
        setOwner(remote.owner_name || '')
        try {
          localStorage.setItem('hexdek_display_name', remote.display_name || '')
          localStorage.setItem('hexdek_owner', remote.owner_name || '')
        } catch { /* private-mode browsers throw — fall through */ }
        setSyncStatus('synced')
        return
      }
      // Fallback: localStorage. Either the user isn't signed in, the
      // auth model isn't fully wired in this environment, or the
      // server is unreachable.
      setDisplayName(localStorage.getItem('hexdek_display_name') || '')
      setOwner(localStorage.getItem('hexdek_owner') || '')
      setSyncStatus('local')
    })()
    return () => { cancelled = true }
  }, [])

  const handleSave = async () => {
    const dn = displayName.trim()
    const on = owner.trim()
    // Try server first; mirror to localStorage either way so the
    // offline-reload path has the freshest values.
    const ok = await patchRemotePrefs({ display_name: dn, owner_name: on })
    try {
      localStorage.setItem('hexdek_display_name', dn)
      localStorage.setItem('hexdek_owner', on)
    } catch { /* ignore private-mode write failure */ }
    setSyncStatus(ok ? 'synced' : 'local')
    setSaved(true)
    setTimeout(() => setSaved(false), 2000)
  }

  return (
    <div style={{ padding: '20px 30px', maxWidth: 600, margin: '0 auto' }}>
      <Panel code="USR.0" title="USER PROFILE">
        <KV rows={[
          ['AUTH EMAIL', user?.email || '— not signed in —'],
          ['UID', user?.uid || '—'],
          ['STATUS', user ? 'AUTHENTICATED' : 'GUEST'],
        ]} />
      </Panel>

      <Panel code="USR.1" title="DISPLAY PREFERENCES" style={{ marginTop: 16 }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>

          <div>
            <div className="t-xs muted" style={{ marginBottom: 4 }}>DISPLAY NAME</div>
            <input
              type="text"
              value={displayName}
              onChange={e => setDisplayName(e.target.value)}
              placeholder="How you want to appear in lobbies and reports"
              style={inputStyle}
            />
            <div className="t-xs muted-2" style={{ marginTop: 2 }}>
              Shown in spectator chats, party lobbies, and game reports.
            </div>
          </div>

          <div>
            <div className="t-xs muted" style={{ marginBottom: 4 }}>OWNER NAME</div>
            <input
              type="text"
              value={owner}
              onChange={e => setOwner(e.target.value)}
              placeholder="e.g. josh, kylie, blake..."
              style={inputStyle}
            />
            <div className="t-xs muted-2" style={{ marginTop: 2 }}>
              Controls which decks appear under "My Decks". Match the folder name in <code>data/decks/</code>.
            </div>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <Btn onClick={handleSave}>SAVE PREFERENCES</Btn>
            {saved && <span className="t-xs" style={{ color: 'var(--ok)' }}>● SAVED</span>}
          </div>

          <div className="t-xs muted-2">
            {syncStatus === 'synced'
              ? 'Preferences sync to your account — available on every device you sign in from.'
              : 'Preferences cached locally (sign in to sync across devices).'}
          </div>
        </div>
      </Panel>
    </div>
  )
}
