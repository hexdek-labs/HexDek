// errorTelemetry — GA4 `exception` events + first-party error reporting.
//
// Sinks every reportable frontend crash to two places:
//   1. GA4 via `gtag('event', 'exception', ...)` (standard GA4 shape), and
//   2. POST /api/telemetry/error (first-party, lands in SQLite next to
//      the pageview rows so errors correlate with anon_id sessions).
//
// Three producers feed this module:
//   - ErrorBoundary.componentDidCatch (LazyBoundary.jsx) — render crashes,
//     chunk-load failures, iOS storage errors. source: 'boundary'.
//   - window 'error' listener (installGlobalErrorListeners) — uncaught
//     synchronous throws that never reached a boundary. source: 'window'.
//   - window 'unhandledrejection' listener — async failures (the 36
//     silently-caught fetch sites are NOT here; only rejections nobody
//     handled). source: 'promise'.
//
// Scope: app errors only — login/auth flow and core-service (API/render)
// failures. Deliberately EXCLUDED as chatter:
//   - resource-load errors (img/script/css 404s) — window 'error' events
//     with no `error` object;
//   - cross-origin "Script error." with no detail, and errors whose
//     filename is a third-party origin (gtag, extensions);
//   - plain network-connectivity failures ("Failed to fetch" et al.) —
//     environment noise, not code defects; flaky mobile links would
//     drown the signal;
//   - console.error chatter — nothing here hooks the console.
//
// Telemetry must never hurt the app: every entry point is try/catch'd,
// reports are deduped per message and capped per session, and a failing
// telemetry POST is itself swallowed (never re-reported — no loops).

// Explicit .js extensions so this module (and its pure helpers) load
// under plain `node --test` for the unit tests, same trick as api.js.
import { getAnonId } from './anonId.js'
import { API_BASE } from '../services/api.js'

const MAX_REPORTS_PER_SESSION = 20
const MAX_MESSAGE_LEN = 500
const MAX_STACK_LEN = 4000

// Network-connectivity rejection messages across engines: Chromium
// ("Failed to fetch"), Firefox ("NetworkError when attempting..."),
// Safari ("Load failed"). Connectivity is environment, not a code bug.
const NETWORK_NOISE = /^(typeerror: )?(failed to fetch|networkerror|load failed)/i

// Pure helper — exported for unit tests. Decides whether a candidate
// error is app signal or excluded chatter. `filename` is the script URL
// from the ErrorEvent (empty for same-document / eval / React renders).
export function shouldReport({ message, hasErrorObject, filename, source }) {
  const msg = String(message || '').trim()
  if (!msg) return false
  // Resource-load chatter: an ErrorEvent without an Error object is a
  // failed <img>/<script>/<link>, not a thrown exception.
  if (source === 'window' && !hasErrorObject) return false
  // Cross-origin scripts surface as bare "Script error." with no stack —
  // nothing actionable, and it's third-party by definition.
  if (/^script error\.?$/i.test(msg)) return false
  // Third-party script origins (gtag, browser extensions). Same-origin
  // and bundle-relative filenames are kept; so are empty filenames
  // (React render throws carry no filename).
  if (filename && typeof window !== 'undefined' && window.location) {
    try {
      const u = new URL(filename, window.location.href)
      if (u.origin !== window.location.origin) return false
    } catch { /* unparseable filename — keep the report */ }
  }
  // Connectivity noise from the promise channel.
  if (source === 'promise' && NETWORK_NOISE.test(msg)) return false
  return true
}

// Pure helper — exported for unit tests. Normalizes an unhandledrejection
// reason (Error | string | anything) into { message, stack }.
export function describeRejection(reason) {
  if (reason instanceof Error) {
    return { message: `${reason.name}: ${reason.message}`, stack: reason.stack || '' }
  }
  if (typeof reason === 'string') return { message: reason, stack: '' }
  try {
    return { message: `non-error rejection: ${JSON.stringify(reason).slice(0, 200)}`, stack: '' }
  } catch {
    return { message: 'non-error rejection (unserializable)', stack: '' }
  }
}

let sentCount = 0
const seenMessages = new Set()

// Test seam: reset per-session dedupe/cap state.
export function _resetForTests() {
  sentCount = 0
  seenMessages.clear()
}

// reportError — the single sink. Fires the GA4 exception event and the
// first-party POST. Deduped per distinct message per session, hard-capped
// so a crash loop can't blast the backend. Never throws.
export function reportError({ message, stack = '', kind = 'render', source = 'window', fatal = false }) {
  try {
    const msg = String(message || '').slice(0, MAX_MESSAGE_LEN)
    if (!msg) return
    if (sentCount >= MAX_REPORTS_PER_SESSION) return
    if (seenMessages.has(msg)) return
    seenMessages.add(msg)
    sentCount++

    if (typeof window !== 'undefined' && typeof window.gtag === 'function') {
      window.gtag('event', 'exception', {
        description: `[${kind}/${source}] ${msg}`.slice(0, MAX_MESSAGE_LEN),
        fatal,
      })
    }

    const body = JSON.stringify({
      anon_id: getAnonId() || '',
      message: msg,
      stack: String(stack || '').slice(0, MAX_STACK_LEN),
      kind,
      source,
      path: typeof window !== 'undefined' ? window.location.pathname + window.location.search : '',
      timestamp: Date.now(),
    })
    // keepalive so a crash-then-navigate still delivers. The catch is
    // terminal — a failed error POST is never itself reported.
    fetch(`${API_BASE}/api/telemetry/error`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body,
      keepalive: true,
    }).catch(() => {})
  } catch { /* telemetry must never throw */ }
}

let installed = false

// installGlobalErrorListeners — call once from main.jsx before render.
// Catches what the React error boundary can't: uncaught synchronous
// throws outside render and unhandled promise rejections.
export function installGlobalErrorListeners() {
  if (installed || typeof window === 'undefined') return
  installed = true

  window.addEventListener('error', (event) => {
    try {
      const message = event.error?.message
        ? `${event.error.name || 'Error'}: ${event.error.message}`
        : String(event.message || '')
      if (!shouldReport({
        message,
        hasErrorObject: !!event.error,
        filename: event.filename || '',
        source: 'window',
      })) return
      reportError({
        message,
        stack: event.error?.stack || '',
        kind: 'uncaught',
        source: 'window',
        fatal: true,
      })
    } catch { /* never throw from a listener */ }
  })

  window.addEventListener('unhandledrejection', (event) => {
    try {
      const { message, stack } = describeRejection(event.reason)
      if (!shouldReport({
        message,
        hasErrorObject: true,
        filename: '',
        source: 'promise',
      })) return
      reportError({
        message,
        stack,
        kind: 'rejection',
        source: 'promise',
        fatal: false,
      })
    } catch { /* never throw from a listener */ }
  })
}
