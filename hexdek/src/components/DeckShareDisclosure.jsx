import { useState } from 'react'
import { toast } from './Toast'
import { trackEvent } from '../hooks/useAnalytics'
import {
  buildAnonShareUrl,
  isShareOptIn,
  setShareOptIn,
} from '../lib/deckShare'

// Owner-only disclosure for opt-in anonymous sharing. Renders inside
// DECK SPECS on the owner's deck page. Two visible states:
//
//   off  — toggle button + an explanation of what enabling does
//   on   — toggle to disable + the share URL with copy / download
//
// The disclosure is intentionally up-front about what the opt-in means:
// flipping it on doesn't gate the URL on the server, it just tells the
// owner "here's the link to use when you want to share anonymously."
// A future PR can promote this to a backend token without changing the
// helper signatures used here (see deckShare.js).

function copy(text) {
  if (navigator?.clipboard?.writeText) {
    return navigator.clipboard.writeText(text).then(() => true).catch(() => false)
  }
  try {
    const ta = document.createElement('textarea')
    ta.value = text
    ta.style.position = 'fixed'
    ta.style.opacity = '0'
    document.body.appendChild(ta)
    ta.select()
    const ok = document.execCommand('copy')
    document.body.removeChild(ta)
    return Promise.resolve(ok)
  } catch {
    return Promise.resolve(false)
  }
}

export default function DeckShareDisclosure({ owner, id, origin }) {
  const deckKey = owner && id ? `${owner}/${id}` : null
  const effectiveOrigin = origin
    || (typeof window !== 'undefined' ? window.location.origin : '')
  const url = buildAnonShareUrl(effectiveOrigin, owner, id)

  // Render-time sync — same pattern as DeckRating so the disclosure
  // reflects the persisted opt-in immediately on deck switch instead
  // of after a follow-up commit.
  const [trackedKey, setTrackedKey] = useState(deckKey)
  const [enabled, setEnabled] = useState(() => isShareOptIn(deckKey))
  if (deckKey !== trackedKey) {
    setTrackedKey(deckKey)
    setEnabled(isShareOptIn(deckKey))
  }

  if (!deckKey || !url) return null

  const onToggle = () => {
    const next = !enabled
    const persisted = setShareOptIn(deckKey, next)
    setEnabled(persisted)
    trackEvent('share_anon_toggle', { deck: deckKey, enabled: persisted })
    if (persisted) toast.success('ANON SHARE ENABLED')
    else           toast.success('ANON SHARE DISABLED')
  }

  const onCopy = async () => {
    if (!url) return
    const ok = await copy(url)
    trackEvent('share_anon_copy', { deck: deckKey, copied: ok })
    if (ok) toast.success('ANON LINK COPIED')
    else    toast.error('COPY FAILED — ' + url, 5000)
  }

  return (
    <div data-testid="deck-share-disclosure" style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
      <div className="t-xs muted" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <span>ANONYMOUS SHARE</span>
        <span style={{ color: enabled ? 'var(--ok)' : 'var(--ink-3)', letterSpacing: '0.06em' }}>
          {enabled ? '● ENABLED' : '○ OFF'}
        </span>
      </div>
      {enabled ? (
        <>
          <div className="t-xs muted" style={{ opacity: 0.8 }}>
            Anyone with this link views the deck without your username.
            They cannot see your owner page or other decks.
          </div>
          <input
            type="text"
            readOnly
            value={url}
            aria-label="Anonymous share URL"
            onFocus={(e) => e.target.select()}
            style={{
              width: '100%',
              padding: '4px 6px',
              background: 'var(--bg-2, transparent)',
              border: '1px solid var(--rule-2)',
              color: 'var(--ink)',
              font: 'inherit',
              fontSize: 10,
              letterSpacing: '0.02em',
            }}
          />
          <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
            <button
              type="button"
              onClick={onCopy}
              style={{
                background: 'var(--accent, var(--ink))',
                color: 'var(--bg)',
                border: 'none',
                padding: '4px 10px',
                cursor: 'pointer',
                font: 'inherit',
                fontSize: 10,
                letterSpacing: '0.08em',
                fontWeight: 700,
              }}
            >COPY LINK ⎘</button>
            <button
              type="button"
              onClick={onToggle}
              style={{
                background: 'transparent',
                color: 'var(--ink-2)',
                border: '1px solid var(--rule-2)',
                padding: '4px 10px',
                cursor: 'pointer',
                font: 'inherit',
                fontSize: 10,
                letterSpacing: '0.08em',
              }}
            >DISABLE</button>
          </div>
        </>
      ) : (
        <>
          <div className="t-xs muted" style={{ opacity: 0.8 }}>
            Generates a public link viewers can open without seeing your
            username. The link works only while sharing is enabled here.
          </div>
          <button
            type="button"
            onClick={onToggle}
            style={{
              background: 'transparent',
              color: 'var(--ink)',
              border: '1px solid var(--rule-2)',
              padding: '5px 10px',
              cursor: 'pointer',
              font: 'inherit',
              fontSize: 10,
              letterSpacing: '0.08em',
              fontWeight: 700,
              alignSelf: 'flex-start',
            }}
          >ENABLE ANON SHARE ↗</button>
        </>
      )}
    </div>
  )
}
