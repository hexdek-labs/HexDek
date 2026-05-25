// Replay scrubber controls.
//
// Pure helpers extracted from Report.jsx's ReplayScrubber so the
// speed-stepping, turn-stepping, tick-interval computation, and keyboard
// resolution can be unit-tested without spinning up a browser. The
// scrubber owns React state (turnIdx, playing, speedIdx) and calls
// these helpers from its handlers.
//
// Audit of the pre-change scrubber (cmd/screens/Report.jsx:227):
//   ✓ scrub      — <input type="range"> over turnIdx
//   ✓ seek      — ⏮ first / ◀ prev / ▶ next / ⏭ last buttons
//   ✗ speed     — playback locked at setInterval(..., 900) — no way to
//                 watch a long game at 2× or study a single combat at 0.5×
//   ✗ keyboard  — no key handler; users have to mouse to every button
//
// "speed" and "keyboard" are the two most-missing controls; both live
// here.

export const REPLAY_SPEEDS = [0.5, 1, 2, 4]
export const REPLAY_DEFAULT_SPEED_IDX = 1 // 1× — index into REPLAY_SPEEDS
const REPLAY_BASE_TICK_MS = 900

// tickIntervalMs returns the setInterval delay for the autoplay loop
// at the given speed multiplier. Speed 1 keeps the historical 900ms
// cadence so this is backwards-compatible with the pre-speed scrubber.
export function tickIntervalMs(speedMultiplier) {
  if (!Number.isFinite(speedMultiplier) || speedMultiplier <= 0) return REPLAY_BASE_TICK_MS
  return Math.round(REPLAY_BASE_TICK_MS / speedMultiplier)
}

// nextSpeedIdx steps through REPLAY_SPEEDS. direction is +1 (faster)
// or -1 (slower). Clamps at both ends.
export function nextSpeedIdx(currentIdx, direction) {
  const max = REPLAY_SPEEDS.length - 1
  if (!Number.isInteger(currentIdx)) return REPLAY_DEFAULT_SPEED_IDX
  return Math.min(max, Math.max(0, currentIdx + direction))
}

// stepTurn computes the new turnIdx after a transport action. Returns
// the clamped index. `totalTurns` is the timeline length (NOT the last
// index — we accept the same value the scrubber renders for max).
export function stepTurn(currentIdx, totalTurns, action) {
  if (totalTurns <= 0) return 0
  const last = totalTurns - 1
  switch (action) {
    case 'first':
      return 0
    case 'last':
      return last
    case 'prev':
      return Math.max(0, currentIdx - 1)
    case 'next':
      return Math.min(last, currentIdx + 1)
    default:
      return Math.min(last, Math.max(0, currentIdx))
  }
}

function isEditableTarget(el) {
  if (!el) return false
  const tag = el.tagName
  if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return true
  return el.isContentEditable === true
}

// resolveReplayKeyAction maps a KeyboardEvent-shaped object to a
// transport action name (or null to pass through). The scrubber owns
// the dispatch.
//
//   Space             togglePlay
//   ← / J             prev turn
//   → / L             next turn
//   Home              first turn
//   End               last turn
//   ↓ / [             speed down
//   ↑ / ]             speed up
//
// J/K/L are the YouTube/video-player convention (rewind/pause/forward)
// — natural muscle memory for a replay viewer. K is the same as Space.
//
// Suppressed when:
//   - any system modifier is held (Cmd/Ctrl/Alt) — reserved for browser
//   - the event target is an editable element (input/textarea/select/CE)
//   - the key isn't bound
export function resolveReplayKeyAction(event) {
  if (!event) return null
  if (event.metaKey || event.ctrlKey || event.altKey) return null
  if (isEditableTarget(event.target)) return null
  switch (event.key) {
    case ' ':
    case 'Spacebar':
    case 'k':
    case 'K':
      return 'togglePlay'
    case 'ArrowLeft':
    case 'j':
    case 'J':
      return 'prev'
    case 'ArrowRight':
    case 'l':
    case 'L':
      return 'next'
    case 'Home':
      return 'first'
    case 'End':
      return 'last'
    case 'ArrowDown':
    case '[':
      return 'speedDown'
    case 'ArrowUp':
    case ']':
      return 'speedUp'
    default:
      return null
  }
}

export const REPLAY_KEYBINDINGS = [
  { keys: ['Space', 'K'],     action: 'togglePlay', label: 'Play / pause' },
  { keys: ['←', 'J'],         action: 'prev',       label: 'Previous turn' },
  { keys: ['→', 'L'],         action: 'next',       label: 'Next turn' },
  { keys: ['Home'],           action: 'first',      label: 'Jump to first turn' },
  { keys: ['End'],            action: 'last',       label: 'Jump to last turn' },
  { keys: ['↓', '['],         action: 'speedDown',  label: 'Slower playback' },
  { keys: ['↑', ']'],         action: 'speedUp',    label: 'Faster playback' },
]
