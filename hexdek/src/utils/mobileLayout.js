// Mobile-responsive layout helpers.
//
// Audit findings (Spectator screen at iPhone SE / 320px viewport):
//
//   1. ActionExplainBadge tooltip overflowed the right edge of the
//      viewport when a notable log entry sat in the right half of
//      the screen. The popover hard-coded `left: 0; min-width: 240`
//      with no flip logic — fine on desktop, ~80px of horizontal
//      clipping on mobile.
//
//   2. Keyboard-shortcut help dialog declared `minWidth: 320` —
//      exactly the iPhone SE viewport width. With the 24px viewport
//      padding around the dialog the body horizontally scrolled.
//
//   3. The same help dialog was only reachable via the `?` key.
//      Mobile users can't easily type `?` (Shift+/) on the soft
//      keyboard while the spectator is the active screen — they
//      had no path to discover the keyboard shortcuts panel.
//
// This module owns the pure math + feature detection. Components
// import these helpers and pass measured DOMRects from layout
// effects.

// MIN_VIEWPORT_MARGIN — keep this many px between any popover and
// the viewport edges. 8px reads as "intentional" without crowding.
export const MIN_VIEWPORT_MARGIN = 8

// computeTooltipPlacement returns absolute-positioning offsets for
// a tooltip anchored to `anchorRect`, given the tooltip's preferred
// size and the viewport's dimensions. The result is a CSS-ready
// object suitable for an inline style on a `position: fixed`
// element. It flips horizontally when the preferred (left-aligned-
// to-anchor) position would overflow the right edge.
//
//   { anchorRect, tooltipSize, viewport }
//
// Returns `{ left, top, side: 'right' | 'left', vertical: 'below' | 'above' }`
// where `side` indicates which edge of the anchor the tooltip
// extends from, and `vertical` indicates whether the tooltip
// rendered below or above the anchor.
export function computeTooltipPlacement({
  anchorRect,
  tooltipSize,
  viewport,
  margin = MIN_VIEWPORT_MARGIN,
} = {}) {
  if (!anchorRect || !tooltipSize || !viewport) {
    return { left: 0, top: 0, side: 'right', vertical: 'below' }
  }
  const { left: ax, top: ay, right: ar, bottom: ab } = anchorRect
  const { width: tw, height: th } = tooltipSize
  const { width: vw, height: vh } = viewport

  // Horizontal: prefer flush-left to anchor (so the tooltip points
  // outward from the right edge of the badge). Flip to right-aligned
  // if that overflows the viewport right edge.
  let left = ax
  let side = 'right'
  if (left + tw > vw - margin) {
    // Flip: anchor the tooltip's right edge to the anchor's right
    // edge instead.
    left = ar - tw
    side = 'left'
  }
  // Final clamp into [margin, vw - margin - tw].
  left = Math.max(margin, Math.min(left, vw - margin - tw))

  // Vertical: prefer below the anchor (matches the original
  // top: 100% behavior). Flip above if it would overflow the
  // viewport bottom.
  let top = ab + 4
  let vertical = 'below'
  if (top + th > vh - margin) {
    top = ay - th - 4
    vertical = 'above'
  }
  top = Math.max(margin, Math.min(top, vh - margin - th))

  return { left, top, side, vertical }
}

// isTouchDevice — best-effort feature detection. Touch devices need
// click-to-open behavior on hover-style affordances (popovers,
// tooltips) since `mouseenter` doesn't fire reliably on touch.
//
// `globals` accepts a stub in tests; pass `window` at runtime.
// Returns false in node tests automatically.
export function isTouchDevice(globals) {
  if (!globals) return false
  if (typeof globals.matchMedia === 'function') {
    try {
      // Coarse pointers = touchscreens / pencils.
      if (globals.matchMedia('(pointer: coarse)').matches) return true
    } catch { /* MQ throws on some test harnesses */ }
  }
  if (typeof globals.ontouchstart !== 'undefined') return true
  if (typeof globals.navigator !== 'undefined' && (globals.navigator.maxTouchPoints | 0) > 0) return true
  return false
}

// clampDialogSize returns CSS-ready `{ width, maxHeight }` for a
// centered dialog that should never exceed the viewport. The dialog
// gets MIN_VIEWPORT_MARGIN of breathing room on every side.
//
// `target` is the desktop-ideal width / height; on small viewports
// the result clamps inward and the dialog stays usable.
export function clampDialogSize({ target = { width: 480, height: 480 }, viewport, margin = MIN_VIEWPORT_MARGIN } = {}) {
  if (!viewport) return { width: target.width, maxHeight: target.height }
  const w = Math.max(0, Math.min(target.width,  Math.max(0, viewport.width  - margin * 2)))
  const h = Math.max(0, Math.min(target.height, Math.max(0, viewport.height - margin * 2)))
  return { width: w, maxHeight: h }
}

// isSmallViewport — convenience predicate (≤ 480px wide). Mirrors
// the project's existing 480px media-query breakpoints; lets a
// component branch its render without parsing media queries
// imperatively.
export function isSmallViewport(viewport) {
  if (!viewport) return false
  return Number.isFinite(viewport.width) && viewport.width <= 480
}
