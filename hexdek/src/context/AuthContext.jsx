import { createContext, useContext, useState, useEffect, useRef } from 'react'
import { onAuthChange, signOutUser } from '../lib/firebase'
import { stitchSession } from '../hooks/useAnalytics'
import { reportError } from '../lib/errorTelemetry'

const AuthContext = createContext(null)

function ownerSlug(u) {
  if (!u) return ''
  if (typeof window !== 'undefined') {
    try {
      const stored = window.localStorage.getItem('hexdek_owner')
      if (stored) return stored.toLowerCase()
    } catch { /* ignore */ }
  }
  const slug = u.displayName?.toLowerCase()
    || u.email?.split('@')[0]?.split('.')[0]
    || ''
  return slug.toLowerCase()
}

async function resolveAndStoreOwner(email) {
  try {
    const res = await fetch(`/api/resolve-owner?email=${encodeURIComponent(email)}`)
    if (!res.ok) return
    const { owner } = await res.json()
    if (owner) localStorage.setItem('hexdek_owner', owner)
  } catch { /* best effort */ }
}

const DEV_USER = {
  uid: 'dev-local',
  email: 'dev@localhost',
  displayName: 'DEV OPERATOR',
}

const isLocalhost = typeof window !== 'undefined' &&
  (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1')

export function AuthProvider({ children }) {
  const [user, setUser] = useState(isLocalhost ? DEV_USER : null)
  const [loading, setLoading] = useState(!isLocalhost)
  const stitchedFor = useRef(null)

  useEffect(() => {
    if (isLocalhost) return
    // Watchdog (r63 works-only-in-private incident): a wedged Firebase
    // persistence layer (corrupt/locked IndexedDB from a prior session)
    // can keep onAuthStateChanged from EVER firing — loading stays true
    // and RequireAuth renders a permanent blank page that reads as
    // "login is broken". After 8s, stop blocking the UI: the user is
    // treated as signed out (functional app, can re-auth) instead of
    // staring at nothing. Telemetry records the wedge.
    let authResolved = false
    const watchdog = setTimeout(() => {
      if (authResolved) return
      reportError({
        message: 'auth init watchdog: onAuthStateChanged never fired within 8s (wedged persistence?)',
        kind: 'auth',
        source: 'window',
      })
      setLoading(false)
    }, 8000)
    const unsub = onAuthChange((u) => {
      authResolved = true
      clearTimeout(watchdog)
      if (u?.email) resolveAndStoreOwner(u.email)
      setUser(u)
      setLoading(false)
    })
    return () => {
      clearTimeout(watchdog)
      unsub()
    }
  }, [])

  // Temporal Pincer — stitch the anonymous browser id to the owner the
  // first time auth resolves (and again if the owner changes). Idempotent
  // on the server side via INSERT OR REPLACE on (anon_id, owner).
  useEffect(() => {
    const slug = ownerSlug(user)
    if (!slug) {
      stitchedFor.current = null
      return
    }
    if (stitchedFor.current === slug) return
    stitchedFor.current = slug
    stitchSession(slug)
  }, [user])

  const logout = async () => {
    await signOutUser()
    setUser(null)
  }

  return (
    <AuthContext.Provider value={{ user, loading, logout }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be inside AuthProvider')
  return ctx
}
