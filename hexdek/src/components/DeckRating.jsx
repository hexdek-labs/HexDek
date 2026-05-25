import { useState } from 'react'
import { getDeckRating, setDeckRating } from '../lib/deckRating'

// Owner-private 1–5 star rating for "subjective feel" tracking. Persists
// to localStorage under a user-namespaced key (see deckRating.js) — never
// uploaded to the server, never shown to other users. Clicking the same
// star again clears the rating; hover previews the prospective value.
//
// Render this only when the viewer is the deck's owner. The parent owns
// that gate (isOwner check) so this component stays presentation-only.

const LABELS = ['', 'CUT PILE', 'STALE', 'MID', 'FUN', 'POWERHOUSE']

export default function DeckRating({ userSlug, deckKey }) {
  // Track (userSlug, deckKey) as a single derived "identity" string and
  // sync state during render when it changes. This is the React-recommended
  // pattern for "reset local state when a prop changes" — preferable to
  // useEffect because the new value is applied in the same render, not on
  // a follow-up commit (no flash of stale UI on deck switch).
  const identity = userSlug && deckKey ? `${userSlug}::${deckKey}` : null
  const [trackedIdentity, setTrackedIdentity] = useState(identity)
  const [rating, setRatingState] = useState(() => getDeckRating(userSlug, deckKey))
  const [hover, setHover] = useState(0)

  if (identity !== trackedIdentity) {
    setTrackedIdentity(identity)
    setRatingState(getDeckRating(userSlug, deckKey))
    setHover(0)
  }

  if (!userSlug || !deckKey) return null

  const onPick = (n) => {
    // Click same star → clear (toggle off). This is the only way to
    // "unrate" since there's no separate clear button.
    const next = rating === n ? 0 : n
    setRatingState(setDeckRating(userSlug, deckKey, next))
  }

  const shown = hover || rating
  const labelText = shown > 0 ? LABELS[shown] : (rating > 0 ? LABELS[rating] : 'UNRATED')

  return (
    <div
      data-testid="deck-rating"
      style={{ display: 'flex', flexDirection: 'column', gap: 4 }}
    >
      <div className="t-xs muted" style={{ display: 'flex', justifyContent: 'space-between' }}>
        <span>YOUR RATING</span>
        <span style={{ letterSpacing: '0.06em' }}>{labelText}</span>
      </div>
      <div
        role="radiogroup"
        aria-label="Rate this deck"
        onMouseLeave={() => setHover(0)}
        style={{ display: 'inline-flex', gap: 2, fontSize: 22, lineHeight: 1, userSelect: 'none' }}
      >
        {[1, 2, 3, 4, 5].map(n => {
          const filled = n <= shown
          return (
            <button
              key={n}
              type="button"
              role="radio"
              aria-checked={rating === n}
              aria-label={`${n} star${n === 1 ? '' : 's'} — ${LABELS[n].toLowerCase()}`}
              onMouseEnter={() => setHover(n)}
              onFocus={() => setHover(n)}
              onBlur={() => setHover(0)}
              onClick={() => onPick(n)}
              style={{
                background: 'transparent',
                border: 'none',
                padding: '2px 2px',
                cursor: 'pointer',
                color: filled ? 'var(--accent, var(--ink))' : 'var(--rule-2)',
                font: 'inherit',
                fontSize: 'inherit',
                lineHeight: 1,
                transition: 'color 60ms ease',
              }}
            >
              {filled ? '★' : '☆'}
            </button>
          )
        })}
      </div>
      <div className="t-xs muted" style={{ opacity: 0.7 }}>
        Private — saved on this browser, never shared.
      </div>
    </div>
  )
}
