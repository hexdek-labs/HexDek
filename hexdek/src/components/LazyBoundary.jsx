import { Component, Suspense, useState, useEffect } from 'react'

function DefaultFallback() {
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

function ErrorFallback({ error, onRetry }) {
  const msg = String(error?.message || error || 'unknown error')
  const isChunk = /chunk|loading|dynamically imported|importing/i.test(msg)
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
        &gt; SCREEN FAILED TO LOAD
      </div>
      <div style={{ fontSize: 11, letterSpacing: '0.04em', color: 'var(--ink-2, #888)', maxWidth: 420, lineHeight: 1.5 }}>
        {isChunk
          ? 'Connection hiccup while fetching this page. Tap retry to try again.'
          : msg.slice(0, 200)}
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
          onClick={() => { window.location.reload() }}
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

class LazyErrorBoundary extends Component {
  constructor(props) {
    super(props)
    this.state = { error: null }
  }
  static getDerivedStateFromError(error) {
    return { error }
  }
  componentDidCatch(error, info) {
    if (typeof console !== 'undefined') {
      console.error('[LazyBoundary]', error, info?.componentStack)
    }
  }
  reset = () => {
    this.setState({ error: null })
    this.props.onReset?.()
  }
  render() {
    if (this.state.error) {
      return <ErrorFallback error={this.state.error} onRetry={this.reset} />
    }
    return this.props.children
  }
}

export default function LazyBoundary({ children }) {
  const [bumpKey, setBumpKey] = useState(0)
  return (
    <LazyErrorBoundary onReset={() => setBumpKey(k => k + 1)}>
      <Suspense fallback={<DefaultFallback />}>
        <div key={bumpKey} style={{ display: 'contents' }}>{children}</div>
      </Suspense>
    </LazyErrorBoundary>
  )
}
