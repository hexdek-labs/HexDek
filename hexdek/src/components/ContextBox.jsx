// ContextBox — small muted helper text rendered above CTAs.
// Designed for neurodivergent UX clarity: every action button should be
// preceded by a 1-2 sentence TLDR explaining what will happen on click.
//
// Usage:
//   <ContextBox>Runs a 500-game gauntlet against the meta. Takes a few minutes.</ContextBox>
//   <Btn solid>RUN GAUNTLET</Btn>
//
// Dismissible variant — pass an `id` to enable per-user persistent dismissal.
// The dismissed state is stored in localStorage under `ctxbox:dismiss:<id>`,
// so once a user has read and dismissed a particular context box it stays
// hidden across sessions.
//
//   <ContextBox id="deck.gauntlet">Runs a 500-game gauntlet...</ContextBox>
//
// Without `id` the component renders as static (non-dismissible) — useful
// for inline help that's always relevant (e.g. confirmation flows).

import { useState, useCallback } from 'react'
import { useT } from '../i18n'

const STORAGE_PREFIX = 'ctxbox:dismiss:'

function readDismissed(id) {
  if (!id || typeof window === 'undefined') return false
  try {
    return window.localStorage.getItem(STORAGE_PREFIX + id) === '1'
  } catch {
    return false
  }
}

function writeDismissed(id) {
  if (!id || typeof window === 'undefined') return
  try {
    window.localStorage.setItem(STORAGE_PREFIX + id, '1')
  } catch {
    // localStorage may be disabled (Safari private mode, etc.) — silent fail
    // is fine, the box will simply re-appear next session.
  }
}

export default function ContextBox({
  id,
  children,
  tone = 'info',
  style,
  compact = false,
  dismissible,
}) {
  // Default: dismissible iff an id was supplied. Callers can force on/off
  // explicitly by passing dismissible={true|false}.
  const t = useT()
  const canDismiss = dismissible === undefined ? Boolean(id) : Boolean(dismissible)

  // Lazy initializer reads the persisted dismissal state once on mount.
  // Callers that reuse the same component instance for different `id`s
  // should remount with a `key` prop so this initializer fires again.
  const [dismissed, setDismissed] = useState(() => readDismissed(id))

  const handleDismiss = useCallback(() => {
    setDismissed(true)
    writeDismissed(id)
  }, [id])

  if (!children) return null
  if (canDismiss && dismissed) return null

  const accent =
    tone === 'warn' ? 'var(--warn, #c9a227)' :
    tone === 'danger' ? 'var(--danger, #b3433d)' :
    'var(--ink-3)'

  return (
    <div
      className={`ctx-box${compact ? ' ctx-box--compact' : ''}${canDismiss ? ' ctx-box--dismissible' : ''}`}
      role="note"
      style={{
        margin: compact ? '0 0 4px 0' : '0 0 6px 0',
        padding: compact ? '4px 8px' : '6px 10px',
        borderLeft: `2px solid ${accent}`,
        background: 'var(--bg-2, rgba(0,0,0,0.18))',
        color: 'var(--ink-2)',
        fontSize: 10,
        lineHeight: 1.45,
        letterSpacing: '0.03em',
        fontStyle: 'normal',
        position: 'relative',
        paddingRight: canDismiss ? (compact ? 22 : 26) : undefined,
        ...style,
      }}
    >
      {children}
      {canDismiss && (
        <button
          type="button"
          className="ctx-box__dismiss"
          aria-label={t('ctx.dismiss_aria')}
          title={t('ctx.dismiss_title')}
          onClick={handleDismiss}
          style={{
            position: 'absolute',
            top: compact ? 2 : 4,
            right: compact ? 4 : 6,
            width: 16,
            height: 16,
            padding: 0,
            background: 'transparent',
            border: 'none',
            color: 'var(--ink-3)',
            cursor: 'pointer',
            fontSize: 12,
            lineHeight: 1,
            opacity: 0.6,
          }}
          onMouseEnter={(e) => { e.currentTarget.style.opacity = '1' }}
          onMouseLeave={(e) => { e.currentTarget.style.opacity = '0.6' }}
        >
          ✕
        </button>
      )}
    </div>
  )
}
