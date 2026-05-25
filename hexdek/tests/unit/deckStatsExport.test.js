// Unit tests for the deck stats export builders (lib/deckStatsExport.js).
//
// Run: npm run test:unit                  (from hexdek/)
// Or:  node --test tests/unit/deckStatsExport.test.js
//
// The DeckExportModal wraps these builders with the format picker,
// fetch bundle, and download/copy UI; these cases pin the output shape
// so third-party consumers (and the user's spreadsheet) don't shift
// underneath them.

import test from 'node:test'
import assert from 'node:assert/strict'

import {
  buildStatsCSV,
  buildStatsJSON,
  extractStats,
  statsExportFilename,
} from '../../src/lib/deckStatsExport.js'

// Realistic-but-minimal fixture covering every section the builders
// know how to render. Tests assert on this shape, then narrow to
// missing-section / edge-case variants.
const FIXTURE = {
  deck: {
    id: 'voltron_uril',
    owner: 'josh',
    name: 'Voltron Uril',
    commander_card: 'Uril, the Miststalker',
    cards: [
      { name: 'Sol Ring', quantity: 1 },
      { name: 'Forest', quantity: 30 },
    ],
    color: 'RGW',
    wbs: 3,
    legal: true,
  },
  analysis: {
    archetype: 'Voltron',
    bracket: 3,
    plays_like: 4,
    game_changer_count: 2,
    power_percentile: 0.78,
    keepable_hand_pct: 0.63,
    mana_base_grade: 'B+',
    commander_synergy: 0.91,
    eval_weights: { board_presence: 0.8, commander_progress: 2.0 },
    win_lines: { win_lines: [{ name: 'Commander damage', type: 'commander_damage' }] },
  },
  gauntlet: {
    games: 500, wins: 220, losses: 280,
    win_rate: 44.0,
    elo_start: 1500, elo_end: 1538,
    finished_at: '2026-05-20T12:00:00Z',
  },
  eloHistory: [
    { finished_at: '2026-04-01T00:00:00Z', games: 100, elo_start: 1500, elo_end: 1510, win_rate: 50 },
    { finished_at: '2026-05-01T00:00:00Z', games: 200, elo_start: 1510, elo_end: 1530, win_rate: 55 },
  ],
  games: [
    { id: 'g1', played_at: '2026-05-19T10:00:00Z', winner_seat: 0, turns: 11, duration_seconds: 1320, placement: 1, opponents: [{ commander: 'Atraxa' }, { commander: 'Krenko' }] },
    { id: 'g2', played_at: '2026-05-18T10:00:00Z', winner_seat: 2, turns: 14, placement: 3, commanders: ['Edric', 'Tasha'] },
  ],
}

// ── extractStats ──────────────────────────────────────────────────────────

test('extractStats — full fixture round-trips all sections', () => {
  const s = extractStats(FIXTURE)
  assert.equal(s.schema_version, 1)
  assert.match(s.exported_at, /^\d{4}-\d{2}-\d{2}T/)
  assert.equal(s.deck.id, 'voltron_uril')
  assert.equal(s.deck.commander, 'Uril, the Miststalker')
  assert.equal(s.deck.card_count, 31) // 1 Sol Ring + 30 Forest
  assert.equal(s.deck.color_identity, 'RGW')
  assert.equal(s.deck.bracket, 3)
  assert.equal(s.deck.legal, true)
  assert.equal(s.analysis.archetype, 'Voltron')
  assert.equal(s.analysis.eval_weights.commander_progress, 2.0)
  assert.equal(s.analysis.win_lines[0].name, 'Commander damage')
  assert.equal(s.gauntlet.games, 500)
  assert.equal(s.gauntlet.win_rate, 44.0)
  assert.equal(s.elo_history.length, 2)
  assert.equal(s.recent_games.length, 2)
  assert.deepEqual(s.recent_games[0].opponents, ['Atraxa', 'Krenko'])
  assert.deepEqual(s.recent_games[1].opponents, ['Edric', 'Tasha']) // commanders[] fallback
})

test('extractStats — missing sections survive as null, never crash', () => {
  const s = extractStats({ deck: { id: 'x', owner: 'y', commander: 'Z' } })
  assert.equal(s.analysis, null)
  assert.equal(s.gauntlet, null)
  assert.equal(s.elo_history, null)
  assert.equal(s.recent_games, null)
  assert.equal(s.deck.commander, 'Z')
})

test('extractStats — empty bundle returns a usable null shape', () => {
  const s = extractStats(null)
  assert.equal(s.deck.id, '')
  assert.equal(s.analysis, null)
  assert.equal(s.gauntlet, null)
})

test('extractStats — color_identity array gets joined into a string', () => {
  const s = extractStats({ deck: { id: 'x', owner: 'y', color_identity: ['W', 'U', 'B'] } })
  assert.equal(s.deck.color_identity, 'WUB')
})

test('extractStats — card_count falls back to deck.card_count when no cards array', () => {
  const s = extractStats({ deck: { id: 'x', owner: 'y', card_count: 99 } })
  assert.equal(s.deck.card_count, 99)
})

// ── buildStatsJSON ────────────────────────────────────────────────────────

test('buildStatsJSON — valid JSON, pretty-printed', () => {
  const out = buildStatsJSON(FIXTURE)
  assert.ok(out.includes('\n  '), 'expected indentation')
  const parsed = JSON.parse(out)
  assert.equal(parsed.schema_version, 1)
  assert.equal(parsed.deck.commander, 'Uril, the Miststalker')
  assert.equal(parsed.gauntlet.win_rate, 44.0)
})

test('buildStatsJSON — null bundle still emits valid JSON', () => {
  const out = buildStatsJSON(null)
  const parsed = JSON.parse(out)
  assert.equal(parsed.schema_version, 1)
  assert.equal(parsed.deck.id, '')
})

// ── buildStatsCSV ─────────────────────────────────────────────────────────

test('buildStatsCSV — emits every section with a leading "# section" comment', () => {
  const out = buildStatsCSV(FIXTURE)
  for (const h of ['# overview', '# eval_weights', '# gauntlet', '# elo_history', '# recent_games']) {
    assert.ok(out.includes(h), `expected section header ${h}`)
  }
})

test('buildStatsCSV — overview keys land in stable column order', () => {
  const out = buildStatsCSV(FIXTURE)
  const idx = (s) => out.indexOf(s)
  assert.ok(idx('overview,deck_id') < idx('overview,owner'))
  assert.ok(idx('overview,owner') < idx('overview,name'))
  assert.ok(idx('overview,name') < idx('overview,commander'))
})

test('buildStatsCSV — eval_weights renders one row per dimension', () => {
  const out = buildStatsCSV(FIXTURE)
  assert.ok(out.includes('eval_weights,board_presence,0.8'))
  assert.ok(out.includes('eval_weights,commander_progress,2'))
})

test('buildStatsCSV — quotes cells that contain commas or quotes (RFC 4180)', () => {
  const bundle = {
    deck: { id: 'x', owner: 'y', commander_card: "Atraxa, Praetors' Voice" },
  }
  const out = buildStatsCSV(bundle)
  // The commander cell has a comma — must be wrapped in double quotes.
  assert.ok(out.includes(`overview,commander,"Atraxa, Praetors' Voice"`))
})

test('buildStatsCSV — embedded double-quote is doubled per RFC 4180', () => {
  const bundle = {
    deck: { id: 'x', owner: 'y', name: 'My "Best" Deck' },
  }
  const out = buildStatsCSV(bundle)
  assert.ok(out.includes(`overview,name,"My ""Best"" Deck"`))
})

test('buildStatsCSV — recent_games joins opponents with " | "', () => {
  const out = buildStatsCSV(FIXTURE)
  assert.ok(out.includes('Atraxa | Krenko'))
})

test('buildStatsCSV — sections absent from bundle are not rendered', () => {
  const bundle = { deck: { id: 'x', owner: 'y' } }
  const out = buildStatsCSV(bundle)
  assert.ok(out.includes('# overview'))
  assert.equal(out.includes('# gauntlet'), false)
  assert.equal(out.includes('# eval_weights'), false)
  assert.equal(out.includes('# elo_history'), false)
  assert.equal(out.includes('# recent_games'), false)
})

test('buildStatsCSV — null / empty cells render as empty string, never "null"/"undefined"', () => {
  const bundle = {
    deck: { id: 'x', owner: 'y' },
    gauntlet: { games: 10, wins: null, losses: undefined, win_rate: 50, elo_start: null, elo_end: null, finished_at: null },
  }
  const out = buildStatsCSV(bundle)
  assert.ok(out.includes('gauntlet,wins,\n'))
  assert.ok(out.includes('gauntlet,losses,\n'))
  assert.equal(out.includes('null'), false)
  assert.equal(out.includes('undefined'), false)
})

// ── statsExportFilename ───────────────────────────────────────────────────

test('statsExportFilename — strips non-safe chars, switches by format', () => {
  assert.equal(statsExportFilename('voltron uril!', 'json'), 'voltron_uril__stats.json')
  assert.equal(statsExportFilename('voltron_uril', 'csv'),  'voltron_uril_stats.csv')
})

test('statsExportFilename — missing id falls back to "deck"', () => {
  assert.equal(statsExportFilename('', 'json'), 'deck_stats.json')
  assert.equal(statsExportFilename(null, 'csv'), 'deck_stats.csv')
})

test('statsExportFilename — unknown format defaults to .json', () => {
  assert.equal(statsExportFilename('x', 'whatever'), 'x_stats.json')
})
