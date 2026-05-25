// Run with: node --test hexdek/src/utils/replayControls.test.js
//
// Pure-logic regressions for the replay scrubber helpers (speed steps,
// turn steps, tick interval, keyboard resolution).

import { test } from 'node:test'
import assert from 'node:assert/strict'

import {
  REPLAY_SPEEDS,
  REPLAY_DEFAULT_SPEED_IDX,
  REPLAY_KEYBINDINGS,
  tickIntervalMs,
  nextSpeedIdx,
  stepTurn,
  resolveReplayKeyAction,
} from './replayControls.js'

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
// Speed
// -----------------------------------------------------------------------------

test('REPLAY_SPEEDS includes 1× at the default index', () => {
  assert.equal(REPLAY_SPEEDS[REPLAY_DEFAULT_SPEED_IDX], 1)
})

test('tickIntervalMs at 1× keeps the historical 900ms cadence', () => {
  assert.equal(tickIntervalMs(1), 900)
})

test('tickIntervalMs scales inversely with speed', () => {
  assert.equal(tickIntervalMs(2), 450)
  assert.equal(tickIntervalMs(4), 225)
  assert.equal(tickIntervalMs(0.5), 1800)
})

test('tickIntervalMs falls back to baseline on bad input', () => {
  assert.equal(tickIntervalMs(0), 900)
  assert.equal(tickIntervalMs(-1), 900)
  assert.equal(tickIntervalMs(NaN), 900)
  assert.equal(tickIntervalMs(undefined), 900)
})

test('nextSpeedIdx steps and clamps', () => {
  assert.equal(nextSpeedIdx(1, +1), 2)
  assert.equal(nextSpeedIdx(1, -1), 0)
  assert.equal(nextSpeedIdx(0, -1), 0, 'cannot go below slowest')
  assert.equal(nextSpeedIdx(REPLAY_SPEEDS.length - 1, +1), REPLAY_SPEEDS.length - 1,
    'cannot go above fastest')
})

test('nextSpeedIdx resets to default on bad input', () => {
  assert.equal(nextSpeedIdx(undefined, +1), REPLAY_DEFAULT_SPEED_IDX)
  assert.equal(nextSpeedIdx(null, -1),     REPLAY_DEFAULT_SPEED_IDX)
  assert.equal(nextSpeedIdx(1.5, +1),      REPLAY_DEFAULT_SPEED_IDX)
})

// -----------------------------------------------------------------------------
// Turn stepping
// -----------------------------------------------------------------------------

test('stepTurn first/last/prev/next', () => {
  assert.equal(stepTurn(5, 10, 'first'), 0)
  assert.equal(stepTurn(5, 10, 'last'),  9)
  assert.equal(stepTurn(5, 10, 'prev'),  4)
  assert.equal(stepTurn(5, 10, 'next'),  6)
})

test('stepTurn clamps at the edges', () => {
  assert.equal(stepTurn(0, 10, 'prev'), 0)
  assert.equal(stepTurn(9, 10, 'next'), 9)
})

test('stepTurn handles empty timeline', () => {
  assert.equal(stepTurn(0, 0, 'next'),  0)
  assert.equal(stepTurn(0, 0, 'last'),  0)
  assert.equal(stepTurn(0, 0, 'first'), 0)
})

test('stepTurn clamps an out-of-range current to the last valid index', () => {
  // Defensive: scrubber clamps after timeline shrink on game switch.
  assert.equal(stepTurn(50, 10, 'unknown'), 9)
  assert.equal(stepTurn(-3, 10, 'unknown'), 0)
})

// -----------------------------------------------------------------------------
// Keyboard resolution
// -----------------------------------------------------------------------------

test('Space and K toggle play', () => {
  assert.equal(resolveReplayKeyAction(ev({ key: ' ' })), 'togglePlay')
  assert.equal(resolveReplayKeyAction(ev({ key: 'Spacebar' })), 'togglePlay')
  assert.equal(resolveReplayKeyAction(ev({ key: 'k' })), 'togglePlay')
  assert.equal(resolveReplayKeyAction(ev({ key: 'K' })), 'togglePlay')
})

test('Arrow and JL navigate turns', () => {
  assert.equal(resolveReplayKeyAction(ev({ key: 'ArrowLeft' })),  'prev')
  assert.equal(resolveReplayKeyAction(ev({ key: 'j' })),          'prev')
  assert.equal(resolveReplayKeyAction(ev({ key: 'ArrowRight' })), 'next')
  assert.equal(resolveReplayKeyAction(ev({ key: 'l' })),          'next')
})

test('Home and End jump to ends', () => {
  assert.equal(resolveReplayKeyAction(ev({ key: 'Home' })), 'first')
  assert.equal(resolveReplayKeyAction(ev({ key: 'End' })),  'last')
})

test('Up/Down + brackets adjust speed', () => {
  assert.equal(resolveReplayKeyAction(ev({ key: 'ArrowDown' })), 'speedDown')
  assert.equal(resolveReplayKeyAction(ev({ key: 'ArrowUp' })),   'speedUp')
  assert.equal(resolveReplayKeyAction(ev({ key: '[' })),         'speedDown')
  assert.equal(resolveReplayKeyAction(ev({ key: ']' })),         'speedUp')
})

test('System modifiers suppress replay shortcuts', () => {
  for (const mod of ['meta', 'ctrl', 'alt']) {
    assert.equal(resolveReplayKeyAction(ev({ key: ' ', [mod]: true })), null,
      `Space + ${mod} should pass through to browser/OS`)
    assert.equal(resolveReplayKeyAction(ev({ key: 'ArrowLeft', [mod]: true })), null)
    assert.equal(resolveReplayKeyAction(ev({ key: 'k', [mod]: true })), null)
  }
})

test('Editable targets pass through (typing in a deck-search box, etc.)', () => {
  for (const tag of ['INPUT', 'TEXTAREA', 'SELECT']) {
    assert.equal(resolveReplayKeyAction(ev({ key: ' ', target: { tagName: tag } })), null,
      `Space inside <${tag}> must not toggle play`)
    assert.equal(resolveReplayKeyAction(ev({ key: 'j', target: { tagName: tag } })), null,
      `J inside <${tag}> must type instead of stepping prev`)
  }
  assert.equal(
    resolveReplayKeyAction(ev({ key: 'ArrowRight', target: { tagName: 'DIV', isContentEditable: true } })),
    null,
  )
})

test('Unbound keys return null', () => {
  for (const k of ['a', 'Tab', 'Escape', 'Enter', 'PageDown', 'F5']) {
    assert.equal(resolveReplayKeyAction(ev({ key: k })), null,
      `${k} should be unbound`)
  }
})

// -----------------------------------------------------------------------------
// Binding-table sanity
// -----------------------------------------------------------------------------

test('REPLAY_KEYBINDINGS has no symbol conflicts within the table', () => {
  const seen = new Map()
  for (const entry of REPLAY_KEYBINDINGS) {
    for (const k of entry.keys) {
      const prior = seen.get(k)
      if (prior && prior !== entry.action) {
        assert.fail(`symbol ${k} maps to both ${prior} and ${entry.action}`)
      }
      seen.set(k, entry.action)
    }
  }
})
