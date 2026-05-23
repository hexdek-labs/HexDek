import { lazy } from 'react'
import { Routes, Route, Navigate } from 'react-router-dom'
import AppShell from './components/AppShell'
import LazyBoundary from './components/LazyBoundary'
import { useAuth } from './context/AuthContext'
import { usePageTracking } from './hooks/useAnalytics'

// Eager critical-path screens (landing, browse, primary auth flow).
// These are likely to be on the first paint, so keep them in the main
// bundle for fast first-route render.
import Landing from './screens/Landing'
import Splash from './screens/Splash'
import Dashboard from './screens/Dashboard'
import DeckArchive from './screens/DeckArchive'
import DeckList from './screens/DeckList'
import Leaderboard from './screens/Leaderboard'
import Login from './screens/Login'
import AuthCallback from './screens/AuthCallback'
import Import from './screens/Import'

// Lazy-load the remaining screens — none are on the first-paint critical
// path. Each becomes its own chunk via Vite's dynamic-import code-split,
// which drops the main bundle from ~690KB into a much smaller initial
// payload + on-demand chunks fetched when the user navigates.
const CardPage = lazy(() => import('./screens/CardPage'))
const SharePreview = lazy(() => import('./screens/SharePreview'))
const GameBoard = lazy(() => import('./screens/GameBoard'))
const Spectator = lazy(() => import('./screens/Spectator'))
const SpectateRoom = lazy(() => import('./screens/SpectateRoom'))
const Report = lazy(() => import('./screens/Report'))
const Forge = lazy(() => import('./screens/Forge'))
const About = lazy(() => import('./screens/About'))
const BugReport = lazy(() => import('./screens/BugReport'))
const Donations = lazy(() => import('./screens/Donations'))
const Profile = lazy(() => import('./screens/Profile'))
const PublicProfile = lazy(() => import('./screens/PublicProfile'))
const OperatorProfile = lazy(() => import('./screens/OperatorProfile'))
const Friends = lazy(() => import('./screens/Friends'))
const AdminConviction = lazy(() => import('./screens/AdminConviction'))
// DeckCompare must stay eager — DeckArchive imports a named export
// (DeckPicker) from it, which pulls the whole module into the main
// bundle regardless. Making it lazy() here would generate a warning
// without actually splitting it out.
import DeckCompare from './screens/DeckCompare'
import StreamOverlay from './components/StreamOverlay'

function RequireAuth({ children }) {
  const { user, loading } = useAuth()
  if (loading) return null
  return user ? children : <Navigate to="/login" replace />
}

export default function App() {
  usePageTracking()
  return (
    <Routes>
      {/* Stream overlay — outside AppShell so the appbar/footer/frame
          don't render. The component forces html/body bg transparent
          on mount so OBS browser-source captures alpha cleanly.
          /obs/:gameId is an alias of /stream/:gameId for streamers
          who configured their browser source against the legacy
          /obs path; both render the same component. */}
      {/* /share/:owner/:id — link-preview-friendly deck page. Sits
          outside AppShell so the global nav/sidebar/footer don't render,
          giving social embeds (Discord, Slack, Twitter) a clean frame. */}
      <Route
        path="share/:owner/:id"
        element={
          <LazyBoundary>
            <SharePreview />
          </LazyBoundary>
        }
      />
      <Route path="stream/:gameId" element={<StreamOverlay />} />
      <Route path="stream" element={<StreamOverlay />} />
      <Route path="obs/:gameId" element={<StreamOverlay />} />
      <Route path="obs" element={<StreamOverlay />} />
      <Route element={<AppShell />}>
        <Route index element={<Landing />} />
        <Route path="splash" element={<Splash />} />
        <Route path="login" element={<Login />} />
        <Route path="auth/callback" element={<AuthCallback />} />
        <Route path="dash" element={<Dashboard />} />
        <Route path="decks" element={<DeckList />} />
        <Route path="decks/:owner/:id" element={<DeckArchive />} />
        <Route path="import" element={<Import />} />
        <Route path="compare/:owner1/:deck1/:owner2/:deck2" element={<DeckCompare />} />
        <Route path="cards/:cardName" element={<CardPage />} />
        <Route path="play" element={<GameBoard />} />
        <Route path="forge" element={<Forge />} />
        <Route path="leaderboard" element={<Leaderboard />} />
        <Route path="meta" element={<Navigate to="/leaderboard?view=meta" replace />} />
        <Route path="spectate" element={<Spectator />} />
        <Route path="spectate/:roomId" element={<SpectateRoom />} />
        <Route path="report" element={<Report />} />
        <Route path="report/:gameId" element={<Report />} />
        <Route path="about" element={<About />} />
        <Route path="feedback" element={<BugReport />} />
        <Route path="donations" element={<Donations />} />
        <Route path="profile" element={<Profile />} />
        <Route path="profile/:owner" element={<PublicProfile />} />
        <Route path="operator" element={<RequireAuth><OperatorProfile /></RequireAuth>} />
        <Route path="me" element={<Navigate to="/operator" replace />} />
        <Route path="friends" element={<RequireAuth><Friends /></RequireAuth>} />
        <Route path="admin/conviction" element={<AdminConviction />} />
      </Route>
    </Routes>
  )
}
