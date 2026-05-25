// Spectator keyboard shortcuts.
//
// Pure helpers extracted from Spectator.jsx so the resolution logic
// (which key → which action, when to suppress) can be unit-tested
// without spinning up a browser. The Spectator component imports
// `resolveSpectatorKeyAction`, `nextSpeedMark`, and `computePauseToggle`
// and wires them into a single window-keydown listener.
//
// Conflict surface:
//   - DeckArchive: `k` / `K` (with Cmd/Ctrl override) — different route
//   - ImportModal / SearchBar / CardPopup / GlossaryTerm: Escape only,
//     and only while their modals are mounted
//   - useModalKeyboard: Escape + Tab trap — only while a modal is open
// No spectator shortcut here uses Escape, Tab, Cmd-K, or any letter
// key, so the surfaces above never collide. Browser-default arrows
// (page scroll) and Space (page scroll) ARE caught — the keydown
// listener calls preventDefault when it resolves an action.

export const SPECTATOR_KEYBINDINGS = [
  { keys: ['Space'],      action: 'togglePause', label: 'Pause / resume playback' },
  { keys: ['←'],          action: 'prevTurn',    label: 'Scroll log to previous turn' },
  { keys: ['→'],          action: 'nextTurn',    label: 'Scroll log to next turn' },
  { keys: ['↓', '['],     action: 'speedDown',   label: 'Decrease playback speed' },
  { keys: ['↑', ']'],     action: 'speedUp',     label: 'Increase playback speed' },
  { keys: ['?'],          action: 'toggleHelp',  label: 'Show keyboard shortcut help' },
]

export function isEditableTarget(el) {
  if (!el) return false
  const tag = el.tagName
  if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return true
  return el.isContentEditable === true
}

// resolveSpectatorKeyAction maps a KeyboardEvent-shaped object to one
// of the action names above (or null if the event should pass through
// to the browser / underlying inputs).
//
// Suppressed when:
//   - any system modifier is held (Cmd/Ctrl/Alt) — reserved for OS+browser
//   - the event target is an editable element (input/textarea/select/CE)
//   - the key isn't bound
//
// Shift IS allowed because '?' on US layouts arrives as Shift+/.
export function resolveSpectatorKeyAction(event) {
  if (!event) return null
  if (event.metaKey || event.ctrlKey || event.altKey) return null
  if (isEditableTarget(event.target)) return null
  switch (event.key) {
    case ' ':
    case 'Spacebar': // legacy Edge
      return 'togglePause'
    case 'ArrowLeft':
      return 'prevTurn'
    case 'ArrowRight':
      return 'nextTurn'
    case 'ArrowDown':
    case '[':
      return 'speedDown'
    case 'ArrowUp':
    case ']':
      return 'speedUp'
    case '?':
      return 'toggleHelp'
    default:
      return null
  }
}

// nextSpeedMark steps through a discrete list of speed multipliers.
// `direction` is +1 (faster) or -1 (slower). Clamps at both ends.
// If `currentSpeed` isn't an exact match, snaps to the nearest mark
// before stepping — keeps the keyboard control coherent with the slider
// even after the server clamps the value.
export function nextSpeedMark(currentSpeed, speedMarks, direction) {
  if (!Array.isArray(speedMarks) || speedMarks.length === 0) return currentSpeed
  let idx = speedMarks.indexOf(currentSpeed)
  if (idx < 0) {
    idx = 0
    let best = Math.abs(speedMarks[0] - currentSpeed)
    for (let i = 1; i < speedMarks.length; i++) {
      const d = Math.abs(speedMarks[i] - currentSpeed)
      if (d < best) { best = d; idx = i }
    }
  }
  const newIdx = Math.min(speedMarks.length - 1, Math.max(0, idx + direction))
  return speedMarks[newIdx]
}

// computePauseToggle returns the new playback state when the user
// presses Space. There's no server-side pause endpoint (the speed
// handler clamps to [0.2, 200.0]); "pause" means slamming speed down
// to the slowest mark and remembering where we were so the next Space
// restores it.
//
// `resumeFrom` carries the pre-pause speed across renders (null when
// not paused).
export function computePauseToggle(currentSpeed, resumeFrom, speedMarks) {
  const pauseSpeed = (Array.isArray(speedMarks) && speedMarks.length > 0) ? speedMarks[0] : 0.1
  const isPaused = Math.abs(currentSpeed - pauseSpeed) < 1e-6
  if (isPaused) {
    const restore = (typeof resumeFrom === 'number' && resumeFrom > pauseSpeed) ? resumeFrom : 1
    return { speed: restore, paused: false, resumeFrom: null }
  }
  return { speed: pauseSpeed, paused: true, resumeFrom: currentSpeed }
}

// pickNeighboringTurnHeader scans an ordered list of turn-header
// positions (top offsets in the scroll container) and returns the
// neighbor that the user should be scrolled to.
//
// Headers are passed newest-first (matching the log render order:
// `[...log].reverse()`), so header[0].top is the smallest (closest to
// the top of the container) and represents the NEWEST turn.
//
// `direction === 'next'` → newer turn → smaller top → previous index
// `direction === 'prev'` → older turn → larger top → next index
//
// Returns the chosen header object or null if no movement is possible
// (already at the edge, or no headers visible).
export function pickNeighboringTurnHeader(headers, scrollTop, direction) {
  if (!Array.isArray(headers) || headers.length === 0) return null
  // Find the header at or just below scrollTop — that's "where we
  // currently are". Tolerance of 1px to absorb fractional scrollTop.
  let currentIdx = 0
  for (let i = 0; i < headers.length; i++) {
    if (headers[i].top <= scrollTop + 1) currentIdx = i
    else break
  }
  if (direction === 'next') {
    // Newer turn = smaller index. If we're already on idx 0 (newest),
    // bail.
    if (currentIdx === 0) return null
    return headers[currentIdx - 1]
  }
  if (direction === 'prev') {
    if (currentIdx >= headers.length - 1) return null
    return headers[currentIdx + 1]
  }
  return null
}
