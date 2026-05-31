// useTurnAnnouncer — drives a visually-hidden aria-live region with
// the current turn / phase / active-commander state for screen
// readers. Returns the JSX for the live region; the consumer
// mounts it into their layout. The hook is responsible for the
// dedup + debounce so the SR doesn't fire on every WS payload
// (the same step can replay 5-10 times during a stack resolution).
//
// SR users get a polite, non-interrupting announcement on each
// meaningful (turn, phase, step, active_seat) tuple change. Sighted
// users see nothing — the region is sr-only via inline styles.
//
// Pure helpers (rt, phasePhrase, announcementKey, buildAnnouncement)
// live in turnAnnouncerHelpers.js for node --test access.

import { useEffect, useRef, useState } from 'react'
import { announcementKey, buildAnnouncement } from './turnAnnouncerHelpers.js'

// SR_ONLY_STYLE matches the canonical "screen-reader only" CSS
// pattern: zero-size, clipped, off-screen but still in the
// accessibility tree so the screen reader picks up content changes
// via aria-live.
const SR_ONLY_STYLE = {
  position: 'absolute',
  width: 1,
  height: 1,
  padding: 0,
  margin: -1,
  overflow: 'hidden',
  clip: 'rect(0, 0, 0, 0)',
  whiteSpace: 'nowrap',
  border: 0,
}

export default function useTurnAnnouncer(game, seats, nSeats = 4) {
  const [announcement, setAnnouncement] = useState('')
  const lastKeyRef = useRef('')

  useEffect(() => {
    const key = announcementKey(game)
    if (!key) return
    if (key === lastKeyRef.current) return
    const text = buildAnnouncement(game, seats, nSeats)
    if (!text) return
    lastKeyRef.current = key
    setAnnouncement(text)
  }, [game, seats, nSeats])

  // The element rendered into the layout. aria-live=polite so the
  // SR queues the announcement after its current speech instead of
  // interrupting. aria-atomic=true so the whole sentence is read,
  // not just the diff.
  const liveRegion = (
    <div
      role="status"
      aria-live="polite"
      aria-atomic="true"
      data-testid="turn-announcer"
      style={SR_ONLY_STYLE}
    >
      {announcement}
    </div>
  )

  return { announcement, liveRegion }
}
