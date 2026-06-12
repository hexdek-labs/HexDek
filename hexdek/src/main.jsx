import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { AuthProvider } from './context/AuthContext'
import { LiveProvider } from './hooks/useLiveSocket'
import { installGlobalErrorListeners } from './lib/errorTelemetry'
import './index.css'
import App from './App.jsx'

// Error telemetry for everything the React error boundary can't see:
// uncaught synchronous throws and unhandled promise rejections. Installed
// before render so boot-time crashes are reported too. Resource-load
// chatter, cross-origin script noise, and plain connectivity failures
// are filtered out in the module (see errorTelemetry.js).
installGlobalErrorListeners()

// HexDek intentionally ships NO service worker. Unregister any zombie
// SW (and drop its caches) that an earlier deployment or a previous app
// on this origin may have left controlling normal browser profiles — a
// stale SW serving cached bundles is invisible in private windows and
// produces exactly the "works only in incognito" failure class (r63
// login incident, defense-in-depth layer). Best-effort and async; never
// blocks boot. If a SW ever becomes intentional, remove this sweep.
if (typeof navigator !== 'undefined' && 'serviceWorker' in navigator) {
  navigator.serviceWorker.getRegistrations()
    .then((regs) => regs.forEach((r) => { r.unregister().catch(() => {}) }))
    .catch(() => {})
  if (typeof caches !== 'undefined' && caches?.keys) {
    caches.keys()
      .then((keys) => keys.forEach((k) => { caches.delete(k).catch(() => {}) }))
      .catch(() => {})
  }
}

createRoot(document.getElementById('root')).render(
  <StrictMode>
    <BrowserRouter>
      <AuthProvider>
        <LiveProvider>
          <App />
        </LiveProvider>
      </AuthProvider>
    </BrowserRouter>
  </StrictMode>,
)
