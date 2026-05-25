// Run with: node --test hexdek/src/utils/spectatorKeybindings.test.js
//
// Pure-logic regressions for the spectator keyboard shortcut helpers.
// No DOM, no React — just the helper module so the tests stay <1s and
// don't need Vite / Playwright spinup. Covers:
//   - resolution: which key → which action
//   - suppression: editable target, modifier keys, unbound keys
//   - speed mark stepping + clamping + snap-to-nearest
//   - pause toggle round-trip + restore semantics
//   - turn-header navigation against the newest-first ordering
//   - conflict check: each non-alias key maps to exactly one action

import { test } from 'node:test'
import assert from 'node:assert/strict'

import {
  SPECTATOR_KEYBINDINGS,
  isEditableTarget,
  resolveSpectatorKeyAction,
  nextSpeedMark,
  computePauseToggle,
  pickNeighboringTurnHeader,
} from './spectatorKeybindings.js'

const SPEED_MARKS = [0.1, 0.2, 0.3, 0.5, 0.75, 1, 1.5, 2]

function ev(opts) {
  return {
    key: opts.key,
    metaKey: !!opts.meta,
    ctrlKey: !!opts.ctrl,
    altKey: !!opts.alt,
    shiftKey: !!opts.shift,
    target: opts.target ?? { tagName: 'BODY', isContentEditable: false },
  }
}

// -----------------------------------------------------------------------------
// Resolution
// -----------------------------------------------------------------------------

test('Space resolves to togglePause', () => {
  assert.equal(resolveSpectatorKeyAction(ev({ key: ' ' })), 'togglePause')
  assert.equal(resolveSpectatorKeyAction(ev({ key: 'Spacebar' })), 'togglePause')
})

test('Arrow keys resolve to the expected actions', () => {
  assert.equal(resolveSpectatorKeyAction(ev({ key: 'ArrowLeft' })),  'prevTurn')
  assert.equal(resolveSpectatorKeyAction(ev({ key: 'ArrowRight' })), 'nextTurn')
  assert.equal(resolveSpectatorKeyAction(ev({ key: 'ArrowDown' })),  'speedDown')
  assert.equal(resolveSpectatorKeyAction(ev({ key: 'ArrowUp' })),    'speedUp')
})

test('Bracket aliases mirror Up/Down', () => {
  assert.equal(resolveSpectatorKeyAction(ev({ key: '[' })), 'speedDown')
  assert.equal(resolveSpectatorKeyAction(ev({ key: ']' })), 'speedUp')
})

test('Question mark resolves to toggleHelp (Shift allowed)', () => {
  assert.equal(resolveSpectatorKeyAction(ev({ key: '?', shift: true })), 'toggleHelp')
})

// -----------------------------------------------------------------------------
// Suppression
// -----------------------------------------------------------------------------

test('Cmd/Ctrl/Alt modifiers suppress all spectator shortcuts', () => {
  for (const mod of ['meta', 'ctrl', 'alt']) {
    assert.equal(resolveSpectatorKeyAction(ev({ key: ' ', [mod]: true })), null,
      `Space + ${mod} should pass through`)
    assert.equal(resolveSpectatorKeyAction(ev({ key: 'ArrowLeft', [mod]: true })), null,
      `ArrowLeft + ${mod} should pass through`)
  }
})

test('Typing in an input/textarea/select/contenteditable passes through', () => {
  for (const tag of ['INPUT', 'TEXTAREA', 'SELECT']) {
    assert.equal(
      resolveSpectatorKeyAction(ev({ key: ' ', target: { tagName: tag } })),
      null,
      `Space in <${tag}> must not trigger pause`,
    )
  }
  assert.equal(
    resolveSpectatorKeyAction(ev({ key: 'ArrowLeft', target: { tagName: 'DIV', isContentEditable: true } })),
    null,
  )
})

test('isEditableTarget covers the right element shapes', () => {
  assert.equal(isEditableTarget(null), false)
  assert.equal(isEditableTarget({ tagName: 'INPUT' }), true)
  assert.equal(isEditableTarget({ tagName: 'TEXTAREA' }), true)
  assert.equal(isEditableTarget({ tagName: 'SELECT' }), true)
  assert.equal(isEditableTarget({ tagName: 'DIV', isContentEditable: true }), true)
  assert.equal(isEditableTarget({ tagName: 'DIV', isContentEditable: false }), false)
  assert.equal(isEditableTarget({ tagName: 'BUTTON' }), false)
})

test('Unbound keys return null', () => {
  for (const k of ['a', 'k', 'Escape', 'Tab', 'Enter', 'F5', 'PageDown']) {
    assert.equal(resolveSpectatorKeyAction(ev({ key: k })), null,
      `${k} should be unbound at the spectator level`)
  }
})

// -----------------------------------------------------------------------------
// Speed marks
// -----------------------------------------------------------------------------

test('nextSpeedMark steps through marks', () => {
  assert.equal(nextSpeedMark(1, SPEED_MARKS, +1), 1.5)
  assert.equal(nextSpeedMark(1, SPEED_MARKS, -1), 0.75)
  assert.equal(nextSpeedMark(0.5, SPEED_MARKS, +1), 0.75)
})

test('nextSpeedMark clamps at both ends', () => {
  assert.equal(nextSpeedMark(0.1, SPEED_MARKS, -1), 0.1, 'cannot go below slowest mark')
  assert.equal(nextSpeedMark(2, SPEED_MARKS, +1), 2,     'cannot go above fastest mark')
})

test('nextSpeedMark snaps to nearest when current is off-grid', () => {
  // Server clamps below 0.2 — if the client speed lands at 0.4 (between
  // 0.3 and 0.5), step-up should land at 0.5, step-down at 0.3.
  assert.equal(nextSpeedMark(0.4, SPEED_MARKS, +1), 0.75)
  assert.equal(nextSpeedMark(0.4, SPEED_MARKS, -1), 0.3)
})

test('nextSpeedMark with empty marks list returns current', () => {
  assert.equal(nextSpeedMark(1, [], +1), 1)
  assert.equal(nextSpeedMark(1, null, +1), 1)
})

// -----------------------------------------------------------------------------
// Pause toggle
// -----------------------------------------------------------------------------

test('computePauseToggle pauses from normal speed and remembers it', () => {
  const r = computePauseToggle(1, null, SPEED_MARKS)
  assert.equal(r.paused, true)
  assert.equal(r.speed, 0.1)
  assert.equal(r.resumeFrom, 1)
})

test('computePauseToggle resumes to the prior speed', () => {
  const r = computePauseToggle(0.1, 1.5, SPEED_MARKS)
  assert.equal(r.paused, false)
  assert.equal(r.speed, 1.5)
  assert.equal(r.resumeFrom, null)
})

test('computePauseToggle resume falls back to 1× when no memory', () => {
  // E.g. user joined while the game was already paused — no resumeFrom
  // recorded. Resume should land at the natural 1× default rather than
  // stay stuck at 0.1×.
  const r = computePauseToggle(0.1, null, SPEED_MARKS)
  assert.equal(r.paused, false)
  assert.equal(r.speed, 1)
})

test('computePauseToggle round-trips', () => {
  const a = computePauseToggle(1.5, null, SPEED_MARKS)
  const b = computePauseToggle(a.speed, a.resumeFrom, SPEED_MARKS)
  assert.equal(b.speed, 1.5)
  assert.equal(b.paused, false)
})

// -----------------------------------------------------------------------------
// Turn header navigation (newest-first ordering)
// -----------------------------------------------------------------------------

test('pickNeighboringTurnHeader moves to older turn (prev) by going further down', () => {
  // Headers ordered newest-first: top values increase as we go older.
  const headers = [
    { turn: 10, top: 0 },
    { turn: 9,  top: 200 },
    { turn: 8,  top: 400 },
  ]
  const r = pickNeighboringTurnHeader(headers, 0, 'prev')
  assert.equal(r.turn, 9)
})

test('pickNeighboringTurnHeader moves to newer turn (next) by going up', () => {
  const headers = [
    { turn: 10, top: 0 },
    { turn: 9,  top: 200 },
    { turn: 8,  top: 400 },
  ]
  const r = pickNeighboringTurnHeader(headers, 250, 'next')
  assert.equal(r.turn, 10)
})

test('pickNeighboringTurnHeader clamps at edges', () => {
  const headers = [
    { turn: 10, top: 0 },
    { turn: 9,  top: 200 },
  ]
  assert.equal(pickNeighboringTurnHeader(headers, 0, 'next'), null,
    'already on newest — no further next')
  assert.equal(pickNeighboringTurnHeader(headers, 250, 'prev'), null,
    'already on oldest — no further prev')
})

test('pickNeighboringTurnHeader handles empty/invalid input', () => {
  assert.equal(pickNeighboringTurnHeader([], 0, 'prev'), null)
  assert.equal(pickNeighboringTurnHeader(null, 0, 'prev'), null)
  assert.equal(pickNeighboringTurnHeader([{ turn: 1, top: 0 }], 0, 'sideways'), null)
})

// -----------------------------------------------------------------------------
// Conflict check
// -----------------------------------------------------------------------------

test('SPECTATOR_KEYBINDINGS has no symbol conflicts (only intentional aliases)', () => {
  // Build a map of symbol → action. If the same symbol maps to two
  // different actions, that's a real conflict and the table is wrong.
  const seen = new Map()
  for (const entry of SPECTATOR_KEYBINDINGS) {
    for (const k of entry.keys) {
      const prior = seen.get(k)
      if (prior && prior !== entry.action) {
        assert.fail(`symbol ${k} maps to both ${prior} and ${entry.action}`)
      }
      seen.set(k, entry.action)
    }
  }
})

test('No spectator binding collides with reserved app keys', () => {
  // Cmd-K (DeckArchive), Escape (modals), Tab (modal focus trap),
  // single letters used by DeckArchive (`k`). Each should still
  // resolve to null at the spectator level so DeckArchive's handler
  // wins on its own routes.
  const reserved = ['k', 'K', 'Escape', 'Tab', 'Enter']
  for (const key of reserved) {
    assert.equal(resolveSpectatorKeyAction(ev({ key })), null,
      `reserved key ${key} must not bind a spectator action`)
  }
  // Cmd-K specifically — modifier-held variant.
  assert.equal(resolveSpectatorKeyAction(ev({ key: 'k', meta: true })), null)
  assert.equal(resolveSpectatorKeyAction(ev({ key: 'k', ctrl: true })), null)
})
