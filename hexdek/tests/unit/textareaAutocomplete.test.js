// Unit tests for the Workshop textarea card-name autocomplete helpers.
//
// Run: npm run test:unit                          (from hexdek/)
// Or:  node --test tests/unit/textareaAutocomplete.test.js
//
// The React wrapper (WorkshopTextarea) is exercised in the browser;
// these cases pin the pure parser + suggestion-application math so
// caret-handling regressions can fail before a deploy reaches staging.

import test from 'node:test'
import assert from 'node:assert/strict'

import {
  applySuggestion,
  extractFragment,
  MIN_FRAGMENT_CHARS,
  nextSuggestionIndex,
} from '../../src/screens/textareaAutocomplete.js'

// Convenience: build a {text, caret} pair from a marker string. Use "|"
// in the source string to denote the caret position; the helper strips
// it and returns the offset. Lets each test stay readable.
function withCaret(marker) {
  const caret = marker.indexOf('|')
  if (caret < 0) throw new Error('expected | caret marker in: ' + marker)
  return { text: marker.replace('|', ''), caret }
}

// ── extractFragment ───────────────────────────────────────────────────────

test('extractFragment — typical "1 Sol Ri|" mid-name case', () => {
  const { text, caret } = withCaret('1 Sol Ri|')
  const info = extractFragment(text, caret)
  assert.equal(info.fragment, 'Sol Ri')
  assert.equal(info.qtyPrefix, '1 ')
  assert.equal(info.fragmentStart, 2)
  assert.equal(info.fragmentEnd, caret)
})

test('extractFragment — bare name with no qty prefix still fires', () => {
  const { text, caret } = withCaret('Sol Ri|')
  const info = extractFragment(text, caret)
  assert.equal(info.fragment, 'Sol Ri')
  assert.equal(info.qtyPrefix, '')
  assert.equal(info.fragmentStart, 0)
})

test('extractFragment — COMMANDER: prefix preserved', () => {
  const { text, caret } = withCaret('COMMANDER: Atra|')
  const info = extractFragment(text, caret)
  assert.equal(info.fragment, 'Atra')
  assert.equal(info.qtyPrefix, 'COMMANDER: ')
})

test('extractFragment — fragment shorter than MIN_FRAGMENT_CHARS returns null', () => {
  assert.equal(MIN_FRAGMENT_CHARS, 2)
  const { text, caret } = withCaret('1 S|')
  assert.equal(extractFragment(text, caret), null)
})

test('extractFragment — empty line / caret at start of line returns null', () => {
  assert.equal(extractFragment('|', 0), null)
  const { text, caret } = withCaret('1 Sol Ring\n|')
  assert.equal(extractFragment(text, caret), null)
})

test('extractFragment — caret in the middle of an existing name does NOT fire', () => {
  // Caret between "Sol" and " Ring" — user is editing a finished line.
  const { text, caret } = withCaret('1 Sol| Ring')
  assert.equal(extractFragment(text, caret), null)
})

test('extractFragment — multi-line: only the current line is considered', () => {
  const { text, caret } = withCaret('1 Lightning Bolt\n2 Sol Ri|\n3 Forest')
  const info = extractFragment(text, caret)
  assert.equal(info.fragment, 'Sol Ri')
  assert.equal(info.qtyPrefix, '2 ')
  // fragmentStart should land in the second line, not bleed from the first.
  assert.ok(text.slice(info.fragmentStart, info.fragmentEnd) === 'Sol Ri')
})

test('extractFragment — trailing spaces inside the fragment are trimmed', () => {
  // "1 Sol  |" (two spaces before caret) → fragment "Sol", not "Sol "
  const { text, caret } = withCaret('1 Sol |')
  const info = extractFragment(text, caret)
  assert.equal(info.fragment, 'Sol')
})

test('extractFragment — multi-digit qty prefix matches', () => {
  const { text, caret } = withCaret('12 Sol Ri|')
  const info = extractFragment(text, caret)
  assert.equal(info.qtyPrefix, '12 ')
  assert.equal(info.fragment, 'Sol Ri')
})

test('extractFragment — invalid args return null', () => {
  assert.equal(extractFragment(null, 0), null)
  assert.equal(extractFragment('abc', -1), null)
  assert.equal(extractFragment('abc', 99), null)
  assert.equal(extractFragment('abc', 'nope'), null)
})

// ── applySuggestion ───────────────────────────────────────────────────────

test('applySuggestion — replaces fragment with full card name, preserves qty', () => {
  const text = '1 Sol Ri'
  const info = extractFragment(text, text.length)
  const { text: next, caret } = applySuggestion(text, info, 'Sol Ring')
  assert.equal(next, '1 Sol Ring')
  assert.equal(caret, next.length)
})

test('applySuggestion — bare-name line gets a default "1 " prefix prepended', () => {
  const text = 'Sol Ri'
  const info = extractFragment(text, text.length)
  const { text: next, caret } = applySuggestion(text, info, 'Sol Ring')
  assert.equal(next, '1 Sol Ring')
  assert.equal(caret, next.length)
})

test('applySuggestion — preserves trailing newlines and other lines', () => {
  const text = '1 Lightning Bolt\n2 Sol Ri\n3 Forest'
  const caret = text.indexOf('\n3 Forest') // end of "2 Sol Ri"
  const info = extractFragment(text, caret)
  const { text: next } = applySuggestion(text, info, 'Sol Ring')
  assert.equal(next, '1 Lightning Bolt\n2 Sol Ring\n3 Forest')
})

test('applySuggestion — COMMANDER: line keeps its prefix', () => {
  const text = 'COMMANDER: Atra'
  const info = extractFragment(text, text.length)
  const { text: next } = applySuggestion(text, info, "Atraxa, Praetors' Voice")
  assert.equal(next, "COMMANDER: Atraxa, Praetors' Voice")
})

test('applySuggestion — null fragment returns text unchanged', () => {
  const text = '1 Sol Ring'
  const { text: next, caret } = applySuggestion(text, null, 'Anything')
  assert.equal(next, text)
  assert.equal(caret, text.length)
})

test('applySuggestion — empty suggestion is a no-op', () => {
  const text = '1 Sol Ri'
  const info = extractFragment(text, text.length)
  const { text: next } = applySuggestion(text, info, '')
  assert.equal(next, text)
})

// ── nextSuggestionIndex ───────────────────────────────────────────────────

test('nextSuggestionIndex — wraps forward and backward', () => {
  assert.equal(nextSuggestionIndex(0, 1, 4), 1)
  assert.equal(nextSuggestionIndex(3, 1, 4), 0) // wraps to start
  assert.equal(nextSuggestionIndex(0, -1, 4), 3) // wraps to end
  assert.equal(nextSuggestionIndex(2, -1, 4), 1)
})

test('nextSuggestionIndex — empty list returns 0', () => {
  assert.equal(nextSuggestionIndex(5, 1, 0), 0)
  assert.equal(nextSuggestionIndex(0, -1, 0), 0)
})

// ── end-to-end: extract → apply round-trip ────────────────────────────────

test('round-trip — caret position after acceptance lets typing continue cleanly', () => {
  const text = '1 Lightning Bolt\n1 Sol Ri'
  const info = extractFragment(text, text.length)
  const { text: next, caret } = applySuggestion(text, info, 'Sol Ring')
  // Simulate the user typing another character — should land at the
  // end of "Sol Ring", NOT in the middle of the previous fragment.
  const typed = next.slice(0, caret) + '\n' + next.slice(caret)
  assert.equal(typed, '1 Lightning Bolt\n1 Sol Ring\n')
})
