import { Component, Suspense, useState, useEffect } from 'react'
import { reportError } from '../lib/errorTelemetry'

// LazyBoundary.jsx
// ---------------------------------------------------------------------
// Two related primitives for screen-level resilience.
//
//   <ErrorBoundary>
//     Catches ANY render-time error in its subtree — chunk-load
//     failures from React.lazy, but also synchronous throws from
//     normal (eager) components like DeckArchive. iOS Safari render
//     errors (storage access, getImageData taint, etc.) bubble here
//     instead of unmounting the route to a black screen.
//
//   <LazyBoundary>
//     ErrorBoundary + Suspense. Use this around any subtree that
//     contains React.lazy descendants — gets both the loading
//     fallback and the crash recovery in one wrapper.
//
// componentDidCatch only fires for errors thrown during render /
// during a lifecycle method / inside a hook. Async errors (a setState
// inside a fetch.then handler) are NOT caught — those need their own
// try/catch at the call site.
// ---------------------------------------------------------------------

function DefaultLoadingFallback() {
  const [show, setShow] = useState(false)
  useEffect(() => {
    const id = setTimeout(() => setShow(true), 250)
    return () => clearTimeout(id)
  }, [])
  if (!show) return <div style={{ minHeight: 240 }} aria-hidden="true" />
  return (
    <div
      role="status"
      aria-live="polite"
      style={{
        flex: 1,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: 24,
        minHeight: 240,
        fontFamily: 'inherit',
        fontSize: 11,
        letterSpacing: '0.1em',
        textTransform: 'uppercase',
        color: 'var(--ink-2, #888)',
      }}
    >
      <span>&gt; LOADING<span className="blink">_</span></span>
    </div>
  )
}

function classifyError(error) {
  const msg = String(error?.message || error || '')
  const name = String(error?.name || '')
  if (/chunk|loading.*dynamically|failed to fetch dynamically/i.test(msg)) {
    return 'chunk'
  }
  if (name === 'SecurityError' || /securityerror|quotaexceeded/i.test(msg)) {
    return 'storage'
  }
  return 'render'
}

function ErrorFallback({ error, onRetry }) {
  const kind = classifyError(error)
  const headline =
    kind === 'chunk' ? '> CONNECTION HICCUP'
    : kind === 'storage' ? '> BROWSER STORAGE BLOCKED'
    : '> SOMETHING BROKE'
  const detail =
    kind === 'chunk'
      ? 'Failed to fetch this page. Tap retry to try again, or full-reload to refresh the bundle.'
      : kind === 'storage'
      ? 'Private browsing mode is blocking storage access. Exit private mode and reload, or continue with full-reload.'
      : String(error?.message || error || 'Unknown render error').slice(0, 240)
  return (
    <div
      role="alert"
      style={{
        flex: 1,
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        padding: 24,
        gap: 14,
        minHeight: 240,
        fontFamily: 'inherit',
        textAlign: 'center',
      }}
    >
      <div style={{ fontSize: 13, letterSpacing: '0.08em', textTransform: 'uppercase', color: 'var(--danger, #c33)', fontWeight: 700 }}>
        {headline}
      </div>
      <div style={{ fontSize: 11, letterSpacing: '0.04em', color: 'var(--ink-2, #888)', maxWidth: 420, lineHeight: 1.5 }}>
        {detail}
      </div>
      <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', justifyContent: 'center' }}>
        <button
          onClick={onRetry}
          style={{
            padding: '10px 18px',
            background: 'var(--accent, #c93)',
            color: 'var(--bg, #111)',
            border: '1px solid var(--accent, #c93)',
            fontFamily: 'inherit',
            fontSize: 12,
            fontWeight: 800,
            letterSpacing: '0.1em',
            textTransform: 'uppercase',
            cursor: 'pointer',
          }}
        >
          RETRY
        </button>
        <button
          onClick={() => { try { window.location.reload() } catch {} }}
          style={{
            padding: '10px 18px',
            background: 'transparent',
            color: 'var(--ink, #ddd)',
            border: '1px solid var(--rule, #555)',
            fontFamily: 'inherit',
            fontSize: 12,
            fontWeight: 800,
            letterSpacing: '0.1em',
            textTransform: 'uppercase',
            cursor: 'pointer',
          }}
        >
          FULL RELOAD
        </button>
      </div>
    </div>
  )
}

export class ErrorBoundary extends Component {
  constructor(props) {
    super(props)
    this.state = { error: null, bumpKey: 0 }
  }
  static getDerivedStateFromError(error) {
    return { error }
  }
  componentDidCatch(error, info) {
    if (typeof console !== 'undefined') {
      console.error('[ErrorBoundary]', error, info?.componentStack)
    }
    // GA4 exception event + first-party POST. The classification the
    // fallback UI already computes (chunk / storage / render) rides
    // along so operators can split chunk-load flake from real crashes.
    // reportError never throws — a boundary that crashes while
    // reporting a crash would be strictly worse than silence.
    reportError({
      message: `${error?.name || 'Error'}: ${error?.message || String(error)}`,
      stack: `${error?.stack || ''}\n--- component stack ---${info?.componentStack || ''}`,
      kind: classifyError(error),
      source: 'boundary',
      fatal: true,
    })
  }
  reset = () => {
    // Bump the wrapper key on reset so React remounts the subtree
    // from scratch — important when the prior render left a child
    // hook in a corrupted state (e.g., a half-resolved lazy import).
    this.setState(s => ({ error: null, bumpKey: s.bumpKey + 1 }))
  }
  render() {
    if (this.state.error) {
      return <ErrorFallback error={this.state.error} onRetry={this.reset} />
    }
    return (
      <div key={this.state.bumpKey} style={{ display: 'contents' }}>
        {this.props.children}
      </div>
    )
  }
}

export default function LazyBoundary({ children, fallback }) {
  return (
    <ErrorBoundary>
      <Suspense fallback={fallback ?? <DefaultLoadingFallback />}>
        {children}
      </Suspense>
    </ErrorBoundary>
  )
}
