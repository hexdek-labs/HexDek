// Unit tests for the deck-detail RECENT GAMES panel helpers.
//
// Run: node --test tests/unit/recentGames.test.js   (from hexdek/)
// Exercises the pure helpers only — no React, no DOM — so outcome
// classification and time-bucket formatting stay verified independent
// of the Playwright e2e suite.

import test from 'node:test'
import assert from 'node:assert/strict'
import {
  outcomeForDeck,
  summarizeRecentGames,
  opponentCommanders,
  formatRelativeFinished,
} from '../../src/lib/recentGames.js'

const KEY = 'josh/krenko'

// Builds a SummaryArchiveRow-shaped object. Seat indices line up between
// deck_keys and commanders, matching what the backend returns.
function row({ winner, decks, commanders, gameId = 1, finished = 1700000000 }) {
  return {
    game_id: gameId,
    finished_at: finished,
    turns: 8,
    winner,
    winner_name: winner >= 0 ? commanders[winner] : '',
    end_reason: 'combat',
    deck_keys: decks,
    commanders,
  }
}

// --- outcomeForDeck --------------------------------------------------------

test('outcomeForDeck: win when our seat is the winning seat', () => {
  const r = row({
    winner: 2,
    decks: ['josh/atraxa', 'sam/tymna', KEY, 'pat/edgar'],
    commanders: ['Atraxa', 'Tymna', 'Krenko', 'Edgar'],
  })
  assert.equal(outcomeForDeck(r, KEY), 'win')
})

test('outcomeForDeck: loss when our seat is not the winning seat', () => {
  const r = row({
    winner: 1,                                  // sam/tymna won
    decks: [KEY, 'sam/tymna', 'pat/edgar', 'alex/yuriko'],
    commanders: ['Krenko', 'Tymna', 'Edgar', 'Yuriko'],
  })
  assert.equal(outcomeForDeck(r, KEY), 'loss')
})

test('outcomeForDeck: draw when winner < 0 even if deck was seated', () => {
  const r = row({
    winner: -1,
    decks: [KEY, 'sam/tymna'],
    commanders: ['Krenko', 'Tymna'],
  })
  assert.equal(outcomeForDeck(r, KEY), 'draw')
})

test('outcomeForDeck: unknown when this deck was not in the game', () => {
  const r = row({
    winner: 0,
    decks: ['sam/tymna', 'pat/edgar'],
    commanders: ['Tymna', 'Edgar'],
  })
  assert.equal(outcomeForDeck(r, KEY), 'unknown')
})

test('outcomeForDeck: unknown on malformed/missing input', () => {
  assert.equal(outcomeForDeck(null, KEY), 'unknown')
  assert.equal(outcomeForDeck({}, KEY), 'unknown')
  assert.equal(outcomeForDeck({ deck_keys: [KEY] }, ''), 'unknown')
})

// --- summarizeRecentGames --------------------------------------------------

test('summarizeRecentGames: counts wins/losses/draws and skips unknowns', () => {
  const rows = [
    row({ winner: 0, decks: [KEY, 'a/x'], commanders: ['Krenko', 'X'], gameId: 1 }),  // win
    row({ winner: 1, decks: [KEY, 'a/x'], commanders: ['Krenko', 'X'], gameId: 2 }),  // loss
    row({ winner: -1, decks: [KEY, 'a/x'], commanders: ['Krenko', 'X'], gameId: 3 }), // draw
    row({ winner: 0, decks: ['a/x', 'b/y'], commanders: ['X', 'Y'], gameId: 4 }),     // unknown — not seated
  ]
  assert.deepEqual(summarizeRecentGames(rows, KEY), {
    wins: 1, losses: 1, draws: 1, total: 3,
  })
})

test('summarizeRecentGames: empty input → all zeros', () => {
  assert.deepEqual(summarizeRecentGames([], KEY), { wins: 0, losses: 0, draws: 0, total: 0 })
  assert.deepEqual(summarizeRecentGames(null, KEY), { wins: 0, losses: 0, draws: 0, total: 0 })
})

// --- opponentCommanders ----------------------------------------------------

test('opponentCommanders: filters our seat by deck_key, preserves seat order', () => {
  const r = row({
    winner: 1,
    decks: ['a/x', KEY, 'b/y', KEY],   // our deck is in two seats (mirror match)
    commanders: ['Atraxa', 'Krenko', 'Edgar', 'Krenko'],
  })
  assert.deepEqual(opponentCommanders(r, KEY), ['Atraxa', 'Edgar'])
})

test('opponentCommanders: tolerates missing arrays', () => {
  assert.deepEqual(opponentCommanders({}, KEY), [])
  assert.deepEqual(opponentCommanders(null, KEY), [])
})

// --- formatRelativeFinished -----------------------------------------------

test('formatRelativeFinished: each bucket', () => {
  const now = 1_000_000_000
  assert.equal(formatRelativeFinished(now - 30, now), 'just now')
  assert.equal(formatRelativeFinished(now - 90, now), '1m ago')
  assert.equal(formatRelativeFinished(now - 3600 * 2, now), '2h ago')
  assert.equal(formatRelativeFinished(now - 86400 * 3, now), '3d ago')
  assert.equal(formatRelativeFinished(now - 86400 * 14, now), '2w ago')
  assert.equal(formatRelativeFinished(now - 86400 * 60, now), '2mo ago')
  assert.equal(formatRelativeFinished(now - 86400 * 800, now), '2y ago')
})

test('formatRelativeFinished: future timestamps render as "just now"', () => {
  // Small clock skew shouldn't render as "-3s ago"
  assert.equal(formatRelativeFinished(1_000_000_100, 1_000_000_000), 'just now')
})

test('formatRelativeFinished: bad input returns em-dash', () => {
  assert.equal(formatRelativeFinished(null), '—')
  assert.equal(formatRelativeFinished(0), '—')
  assert.equal(formatRelativeFinished('not-a-number'), '—')
})
