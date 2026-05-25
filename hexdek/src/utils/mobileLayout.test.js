// Run with: node --test hexdek/src/utils/mobileLayout.test.js

import { test } from 'node:test'
import assert from 'node:assert/strict'

import {
  MIN_VIEWPORT_MARGIN,
  computeTooltipPlacement,
  isTouchDevice,
  clampDialogSize,
  isSmallViewport,
} from './mobileLayout.js'

// -----------------------------------------------------------------------------
// computeTooltipPlacement
// -----------------------------------------------------------------------------

const TIP = { width: 240, height: 120 }
const VW_DESKTOP = { width: 1440, height: 900 }
const VW_MOBILE  = { width: 320,  height: 568 }

function rect(x, y, w, h) {
  return { left: x, top: y, right: x + w, bottom: y + h, width: w, height: h }
}

test('computeTooltipPlacement places below + left-flush when there is room', () => {
  const r = rect(100, 200, 50, 24) // anchor at (100, 200), 50×24
  const p = computeTooltipPlacement({ anchorRect: r, tooltipSize: TIP, viewport: VW_DESKTOP })
  assert.equal(p.side, 'right',  'tooltip extends right from anchor\'s left edge')
  assert.equal(p.vertical, 'below')
  assert.equal(p.left, 100, 'flush to anchor.left')
  assert.equal(p.top, 200 + 24 + 4, 'below anchor with a 4px gap')
})

test('computeTooltipPlacement flips horizontally when overflowing the right edge', () => {
  // Anchor near the right edge on a mobile viewport.
  const r = rect(280, 100, 30, 20) // anchor.right = 310, tooltip would extend to 280+240=520 > 320
  const p = computeTooltipPlacement({ anchorRect: r, tooltipSize: TIP, viewport: VW_MOBILE })
  assert.equal(p.side, 'left', 'flipped to right-aligned')
  // Anchor's right edge - tooltip width
  // 310 - 240 = 70, clamped into [margin, vw-margin-tw] = [8, 320-8-240=72] ⇒ 70
  assert.equal(p.left, 70)
})

test('computeTooltipPlacement clamps when even the flipped position overflows', () => {
  // Anchor pinned all the way to the right of a narrow viewport.
  const r = rect(310, 100, 8, 20)
  const p = computeTooltipPlacement({ anchorRect: r, tooltipSize: TIP, viewport: VW_MOBILE })
  // Both placements overflow; the clamp lands the tooltip at
  // vw - margin - tooltipWidth = 320 - 8 - 240 = 72.
  assert.equal(p.left, 72)
})

test('computeTooltipPlacement clamps the left edge to MIN_VIEWPORT_MARGIN', () => {
  // Anchor at the very left edge.
  const r = rect(0, 100, 16, 20)
  const p = computeTooltipPlacement({ anchorRect: r, tooltipSize: TIP, viewport: VW_DESKTOP })
  assert.equal(p.left, MIN_VIEWPORT_MARGIN, 'never starts at -1px / past the left edge')
})

test('computeTooltipPlacement flips vertically when below would overflow the bottom', () => {
  // Anchor at the bottom of a short viewport: tooltip would extend
  // past the bottom; should flip above.
  const r = rect(50, 540, 30, 20) // anchor.bottom = 560; vh = 568
  const p = computeTooltipPlacement({ anchorRect: r, tooltipSize: TIP, viewport: VW_MOBILE })
  assert.equal(p.vertical, 'above')
  assert.equal(p.top, 540 - 120 - 4, 'tooltip above anchor with 4px gap')
})

test('computeTooltipPlacement returns origin for bad input', () => {
  const fallback = computeTooltipPlacement({})
  assert.deepEqual(fallback, { left: 0, top: 0, side: 'right', vertical: 'below' })
})

// -----------------------------------------------------------------------------
// isTouchDevice
// -----------------------------------------------------------------------------

test('isTouchDevice returns false for null / desktop-shaped globals', () => {
  assert.equal(isTouchDevice(null), false)
  assert.equal(isTouchDevice({}), false)
  assert.equal(isTouchDevice({
    matchMedia: () => ({ matches: false }),
    navigator: { maxTouchPoints: 0 },
  }), false)
})

test('isTouchDevice catches the matchMedia pointer:coarse signal', () => {
  const globals = {
    matchMedia: (q) => ({ matches: q === '(pointer: coarse)' }),
  }
  assert.equal(isTouchDevice(globals), true)
})

test('isTouchDevice catches ontouchstart on older iOS Safari', () => {
  assert.equal(isTouchDevice({
    matchMedia: () => ({ matches: false }),
    ontouchstart: null,
  }), true)
})

test('isTouchDevice catches maxTouchPoints > 0 on Android', () => {
  assert.equal(isTouchDevice({
    matchMedia: () => ({ matches: false }),
    navigator: { maxTouchPoints: 5 },
  }), true)
})

test('isTouchDevice tolerates a matchMedia that throws', () => {
  const globals = {
    matchMedia: () => { throw new Error('jsdom') },
    navigator: { maxTouchPoints: 0 },
  }
  // Doesn't throw; falls back to ontouchstart + maxTouchPoints checks.
  assert.equal(isTouchDevice(globals), false)
})

// -----------------------------------------------------------------------------
// clampDialogSize
// -----------------------------------------------------------------------------

test('clampDialogSize returns the target on a roomy desktop viewport', () => {
  const r = clampDialogSize({ target: { width: 480, height: 480 }, viewport: VW_DESKTOP })
  assert.deepEqual(r, { width: 480, maxHeight: 480 })
})

test('clampDialogSize shrinks to fit a narrow viewport with margin breathing room', () => {
  const r = clampDialogSize({ target: { width: 480, height: 480 }, viewport: VW_MOBILE })
  // 320 - 8*2 = 304
  assert.equal(r.width, 304)
  // 568 - 8*2 = 552, but target is 480 so target wins.
  assert.equal(r.maxHeight, 480)
})

test('clampDialogSize clamps both axes when both target dims exceed viewport', () => {
  const r = clampDialogSize({
    target: { width: 9999, height: 9999 },
    viewport: { width: 360, height: 640 },
  })
  assert.equal(r.width, 360 - 16)
  assert.equal(r.maxHeight, 640 - 16)
})

test('clampDialogSize falls back to target dims when viewport is missing', () => {
  const r = clampDialogSize({ target: { width: 480, height: 480 } })
  assert.deepEqual(r, { width: 480, maxHeight: 480 })
})

// -----------------------------------------------------------------------------
// isSmallViewport
// -----------------------------------------------------------------------------

test('isSmallViewport is true at and below 480px', () => {
  assert.equal(isSmallViewport({ width: 320 }), true)
  assert.equal(isSmallViewport({ width: 480 }), true)
  assert.equal(isSmallViewport({ width: 481 }), false)
  assert.equal(isSmallViewport({ width: 1440 }), false)
})

test('isSmallViewport handles missing input', () => {
  assert.equal(isSmallViewport(null), false)
  assert.equal(isSmallViewport({}), false)
})
