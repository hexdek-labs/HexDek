import { test } from 'node:test'
import assert from 'node:assert/strict'
import { recentGauntletWinrate, topWinConditions, deckGlanceStats } from './deckStats.js'

// --- recentGauntletWinrate -------------------------------------------------

test('recentGauntletWinrate: null when no history', () => {
  assert.equal(recentGauntletWinrate(null), null)
  assert.equal(recentGauntletWinrate([]), null)
  assert.equal(recentGauntletWinrate(undefined), null)
})

test('recentGauntletWinrate: takes the last N runs (oldest-first input)', () => {
  // Oldest two are 0% over a large sample; would dominate an unweighted mean.
  // n=5 keeps every run, so all six entries do feed in, but...
  const hist = [
    { games: 1000, win_rate: 0 },
    { games: 1000, win_rate: 0 },
    { games: 10, win_rate: 50 },
    { games: 10, win_rate: 50 },
    { games: 10, win_rate: 50 },
    { games: 10, win_rate: 50 },
  ]
  const r = recentGauntletWinrate(hist, 5)
  // Last five only — 1 zero-run (1000g, 0w) + 4 fifty-runs (40g, 20w).
  // wins=20, games=1040, pct = 20/1040 = ~1.923%, rounded to 1.9.
  assert.equal(r.runs, 5)
  assert.equal(r.games, 1040)
  assert.equal(r.pct, 1.9)
})

test('recentGauntletWinrate: weights by games not run count', () => {
  // Single 100-game run at 30%, plus a single 10-game spike at 100%.
  // Unweighted mean would be 65; weighted is (30*100 + 100*10) / 110 = 36.4.
  const hist = [
    { games: 100, win_rate: 30 },
    { games: 10, win_rate: 100 },
  ]
  const r = recentGauntletWinrate(hist)
  assert.equal(r.runs, 2)
  assert.equal(r.games, 110)
  assert.equal(r.pct, 36.4)
})

test('recentGauntletWinrate: skips malformed entries', () => {
  const hist = [
    null,
    { games: 0, win_rate: 100 },          // zero games — skip
    { games: 50, win_rate: 'abc' },        // bad win_rate — skip
    { games: 50, win_rate: 40 },           // counts
  ]
  const r = recentGauntletWinrate(hist)
  assert.equal(r.runs, 4)
  assert.equal(r.games, 50)
  assert.equal(r.pct, 40)
})

test('recentGauntletWinrate: returns null when every entry is unusable', () => {
  assert.equal(recentGauntletWinrate([{ games: 0, win_rate: 50 }, null]), null)
})

// --- topWinConditions ------------------------------------------------------

test('topWinConditions: returns up to N from finisher_cards', () => {
  const a = { finisher_cards: ['Craterhoof', 'Aetherflux', 'Thoracle', 'Dockside'] }
  assert.deepEqual(topWinConditions(a, 3), ['Craterhoof', 'Aetherflux', 'Thoracle'])
})

test('topWinConditions: falls back to win_lines when finisher_cards short', () => {
  const a = {
    finisher_cards: ['Craterhoof'],
    win_lines: [{ card: 'Aetherflux' }, 'Thoracle', { name: 'Dockside' }],
  }
  assert.deepEqual(topWinConditions(a, 3), ['Craterhoof', 'Aetherflux', 'Thoracle'])
})

test('topWinConditions: dedupes case-insensitively', () => {
  const a = {
    finisher_cards: ['Craterhoof Behemoth'],
    win_lines: [{ card: 'craterhoof behemoth' }, { card: 'Aetherflux Reservoir' }],
  }
  assert.deepEqual(topWinConditions(a, 3), ['Craterhoof Behemoth', 'Aetherflux Reservoir'])
})

test('topWinConditions: returns [] when analysis missing', () => {
  assert.deepEqual(topWinConditions(null), [])
  assert.deepEqual(topWinConditions(undefined), [])
  assert.deepEqual(topWinConditions({}), [])
})

// --- deckGlanceStats -------------------------------------------------------

test('deckGlanceStats: synthesizes the AT A GLANCE shape', () => {
  const out = deckGlanceStats({
    deck: { bracket: '3' },
    analysis: {
      bracket: '4',
      bracket_label: 'Optimized',
      archetype: 'combo',
      finisher_cards: ['Thoracle', 'Demonic Consultation'],
      win_lines: [{ card: 'Tainted Pact' }],
    },
    deckElo: { win_rate: 27, games: 600 },
    eloHistory: [{ games: 100, win_rate: 31 }, { games: 100, win_rate: 25 }],
  })
  assert.equal(out.bracket, '4')              // analysis wins over deck.bracket
  assert.equal(out.bracketLabel, 'Optimized')
  assert.equal(out.archetype, 'COMBO')
  assert.deepEqual(out.winConditions, ['Thoracle', 'Demonic Consultation', 'Tainted Pact'])
  assert.deepEqual(out.recent, { pct: 28, runs: 2, games: 200 })
  assert.deepEqual(out.allTime, { pct: 27, games: 600 })
})

test('deckGlanceStats: tolerates a totally empty input', () => {
  const out = deckGlanceStats({})
  assert.equal(out.bracket, null)
  assert.equal(out.archetype, '')
  assert.deepEqual(out.winConditions, [])
  assert.equal(out.recent, null)
  assert.equal(out.allTime, null)
})

test('deckGlanceStats: surfaces measuredBracket when it differs from declared bracket', () => {
  const out = deckGlanceStats({
    analysis: { bracket: '3', measured_bracket: '4', measured_bracket_label: 'Optimized', archetype: 'midrange' },
  })
  assert.equal(out.bracket, '3')
  assert.equal(out.measuredBracket, '4')
  assert.equal(out.bracketDiverges, true)
})
