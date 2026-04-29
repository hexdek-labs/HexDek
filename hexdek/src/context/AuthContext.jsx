import { createContext, useContext, useState, useEffect } from 'react'
import { onAuthChange, signOutUser } from '../lib/firebase'

const AuthContext = createContext(null)

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

  useEffect(() => {
    if (isLocalhost) return
    const unsub = onAuthChange((u) => {
      setUser(u)
      setLoading(false)
    })
    return unsub
  }, [])

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
