import { NavLink, Outlet, useNavigate } from 'react-router-dom'
import { Crops } from './chrome'
import { useAuth } from '../context/AuthContext'

const PUBLIC_NAV = [
  { to: '/', label: 'SPLASH', end: true },
  { to: '/decks', label: 'DECKS' },
  { to: '/spectate', label: 'SPECTATE' },
  { to: '/about', label: 'ABOUT' },
]

const AUTH_NAV = [
  { to: '/dash', label: 'DASH' },
  { to: '/decks', label: 'DECKS' },
  { to: '/play', label: 'PLAY' },
  { to: '/forge', label: 'FORGE' },
  { to: '/spectate', label: 'SPECTATE' },
  { to: '/report', label: 'REPORT' },
  { to: '/about', label: 'ABOUT' },
]

export default function AppShell() {
  const { user, loading, logout } = useAuth()
  const navigate = useNavigate()
  const nav = user ? AUTH_NAV : PUBLIC_NAV

  const handleLogout = async () => {
    await logout()
    navigate('/')
  }

  return (
    <div style={{ height: '100vh', background: 'var(--bg)', position: 'relative', overflow: 'hidden', display: 'flex', flexDirection: 'column' }}>
      <span className="grain" />
      <div className="frame" style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        <Crops />

        <div className="appbar">
          <div className="flex items-center gap-4">
            <NavLink to={user ? '/dash' : '/'} className="brand" style={{ textDecoration: 'none' }}>HEXDEK//</NavLink>
            <nav>
              {nav.map(n => (
                <NavLink
                  key={n.to}
                  to={n.to}
                  end={n.end}
                  className={({ isActive }) => isActive ? 'on' : ''}
                >
                  {n.label}
                </NavLink>
              ))}
            </nav>
          </div>
          <span>SYS.BUILD 25.04.28 · HEXDEK V0.10D</span>
          {!loading && (
            <span style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
              {user ? (
                <>
                  <NavLink to="/profile" className="t-xs" style={{ color: 'var(--ok)', textDecoration: 'none' }}>● {user.email?.split('@')[0]?.toUpperCase()}</NavLink>
                  <a onClick={handleLogout} style={{ cursor: 'pointer', fontSize: 9, letterSpacing: '0.1em', color: 'var(--ink-2)' }}>LOGOUT</a>
                </>
              ) : (
                <NavLink to="/login" style={{ fontSize: 9, letterSpacing: '0.1em', color: 'var(--ink-2)', textDecoration: 'none' }}>LOGIN ↗</NavLink>
              )}
            </span>
          )}
        </div>

        <div style={{ flex: 1, overflow: 'auto', display: 'flex', flexDirection: 'column' }}>
          <Outlet />
        </div>

        <div className="statusbar">
          <span>+ + +  HEXDEK CORE READY  + + +</span>
          <NavLink to="/feedback" style={{ color: 'var(--danger)', textDecoration: 'none', fontSize: 9, letterSpacing: '0.08em', fontWeight: 700 }}>BUG / SUGGESTION</NavLink>
          <NavLink to="/donations" style={{ color: 'var(--ok)', textDecoration: 'none', fontSize: 9, letterSpacing: '0.08em', fontWeight: 700 }}>DONATE ♥</NavLink>
          <a href="https://discord.gg/Mz2ueRFXds" target="_blank" rel="noopener noreferrer" style={{ color: 'var(--ink-2)', textDecoration: 'none', fontSize: 9, letterSpacing: '0.08em', fontWeight: 700 }}>DISCORD</a>
          <span>OPEN SOURCE / / DONATIONS-POWERED / / NO ADS</span>
          <span>{user ? `USR.${user.email?.split('@')[0]?.toUpperCase()}` : 'GUEST'}</span>
        </div>
      </div>
    </div>
  )
}
