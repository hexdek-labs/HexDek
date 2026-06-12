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
