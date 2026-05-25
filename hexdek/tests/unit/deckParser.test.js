// Unit tests for the shared plain-decklist parser
// (lib/deckParser.js). Both ImportModal's full multi-step flow and the
// inline QUICK PASTE box on /decks consume this module — the assertions
// pin the formats both surfaces are required to round-trip.
//
// Run: npm run test:unit              (from hexdek/)
// Or:  node --test tests/unit/deckParser.test.js

import test from 'node:test'
import assert from 'node:assert/strict'

import {
  BASIC_LANDS,
  inferCommander,
  parseCardLine,
  parseDeckLines,
  summarize,
} from '../../src/lib/deckParser.js'

// ── parseCardLine ─────────────────────────────────────────────────────────

test('parseCardLine — "N CardName" base case', () => {
  assert.deepEqual(parseCardLine('1 Sol Ring'), { name: 'Sol Ring', quantity: 1, set: '', cn: '' })
  assert.deepEqual(parseCardLine('30 Forest'),  { name: 'Forest',   quantity: 30, set: '', cn: '' })
})

test('parseCardLine — "Nx CardName" (Tappedout-style x suffix)', () => {
  assert.deepEqual(parseCardLine('1x Sol Ring'), { name: 'Sol Ring', quantity: 1, set: '', cn: '' })
  assert.deepEqual(parseCardLine('4X Llanowar Elves'), { name: 'Llanowar Elves', quantity: 4, set: '', cn: '' })
})

test('parseCardLine — strips "(SET) CN" set + collector suffix', () => {
  assert.deepEqual(
    parseCardLine('2 Sol Ring (CMM) 339'),
    { name: 'Sol Ring', quantity: 2, set: 'CMM', cn: '339' },
  )
  assert.deepEqual(
    parseCardLine('1 Sol Ring (CMM)'),
    { name: 'Sol Ring', quantity: 1, set: 'CMM', cn: '' },
  )
})

test('parseCardLine — strips trailing foil / etched markers', () => {
  assert.deepEqual(
    parseCardLine('1 Sol Ring (CMM) 339 *F*'),
    { name: 'Sol Ring', quantity: 1, set: 'CMM', cn: '339' },
  )
  // Bare *F* without set wrapper.
  assert.deepEqual(
    parseCardLine('1 Sol Ring *F*'),
    { name: 'Sol Ring', quantity: 1, set: '', cn: '' },
  )
})

test('parseCardLine — SB:/MB: prefix stripped (section gating handled upstream)', () => {
  assert.deepEqual(parseCardLine('SB: 1 Sol Ring'), { name: 'Sol Ring', quantity: 1, set: '', cn: '' })
  assert.deepEqual(parseCardLine('CMD: 1 Atraxa'), { name: 'Atraxa', quantity: 1, set: '', cn: '' })
})

test('parseCardLine — bare name (no qty) defaults to quantity 1', () => {
  assert.deepEqual(parseCardLine('Sol Ring'), { name: 'Sol Ring', quantity: 1, set: '', cn: '' })
})

test('parseCardLine — bare name with trailing collector number is cleaned', () => {
  assert.deepEqual(parseCardLine('Sol Ring 339'), { name: 'Sol Ring', quantity: 1, set: '', cn: '' })
})

test('parseCardLine — empty / null returns null, qty 0 rejected', () => {
  assert.equal(parseCardLine(''), null)
  assert.equal(parseCardLine('   '), null)
  assert.equal(parseCardLine(null), null)
  assert.equal(parseCardLine('0 Sol Ring'), null)
})

// ── inferCommander ────────────────────────────────────────────────────────

test('inferCommander — "Commander: X" prefix wins', () => {
  assert.equal(inferCommander("Commander: Atraxa, Praetors' Voice\n1 Sol Ring"), "Atraxa, Praetors' Voice")
})

test('inferCommander — case-insensitive prefix', () => {
  assert.equal(inferCommander('COMMANDER:Krenko, Mob Boss\n1 Mountain'), 'Krenko, Mob Boss')
  assert.equal(inferCommander('commander: krenko, mob boss\n'), 'krenko, mob boss')
})

test('inferCommander — bare "Commander" header + next card row (Moxfield section style)', () => {
  const text = `Deck\n1 Sol Ring\n\nCommander\n1 Atraxa, Praetors' Voice\n`
  assert.equal(inferCommander(text), "Atraxa, Praetors' Voice")
})

test('inferCommander — bare "Commander (1)" header variant', () => {
  const text = `Commander (1)\n1 Krenko, Mob Boss\n`
  assert.equal(inferCommander(text), 'Krenko, Mob Boss')
})

test('inferCommander — no commander signal returns empty string', () => {
  assert.equal(inferCommander('1 Sol Ring\n1 Forest'), '')
  assert.equal(inferCommander(''), '')
})

// ── parseDeckLines ────────────────────────────────────────────────────────

test('parseDeckLines — basic multi-line parse', () => {
  const got = parseDeckLines('1 Sol Ring\n2 Llanowar Elves\n1 Forest')
  assert.equal(got.length, 3)
  assert.equal(got[0].name, 'Sol Ring')
  assert.equal(got[1].quantity, 2)
  assert.equal(got[2].name, 'Forest')
})

test('parseDeckLines — # and // comments skipped', () => {
  const got = parseDeckLines('# Lands\n1 Forest\n// Ramp\n1 Sol Ring')
  assert.equal(got.length, 2)
  assert.deepEqual(got.map(c => c.name), ['Forest', 'Sol Ring'])
})

test('parseDeckLines — explicit "Commander: X" line tags isCommander', () => {
  const got = parseDeckLines("Commander: Atraxa, Praetors' Voice\n1 Sol Ring")
  assert.equal(got[0].isCommander, true)
  assert.equal(got[0].name, "Atraxa, Praetors' Voice")
  assert.equal(got[1].isCommander, false)
})

test('parseDeckLines — sideboard SECTION (not just header) is fully skipped', () => {
  // Previously the parser only skipped the "Sideboard" header line and
  // silently imported every sideboard card into the mainboard.
  const text = `1 Sol Ring\n1 Forest\n\nSideboard\n1 Pithing Needle\n1 Tormod's Crypt\n`
  const got = parseDeckLines(text)
  assert.deepEqual(got.map(c => c.name), ['Sol Ring', 'Forest'])
})

test('parseDeckLines — Mainboard header re-enters after Sideboard', () => {
  const text = `Sideboard\n1 Pithing Needle\n\nMainboard\n1 Sol Ring\n`
  const got = parseDeckLines(text)
  assert.deepEqual(got.map(c => c.name), ['Sol Ring'])
})

test('parseDeckLines — bare "Commander" header tags FIRST card row, then re-enters main', () => {
  const text = `Deck\n1 Sol Ring\n\nCommander\n1 Atraxa, Praetors' Voice\n`
  const got = parseDeckLines(text)
  const commander = got.find(c => c.isCommander)
  assert.equal(commander?.name, "Atraxa, Praetors' Voice")
  // Sol Ring stays in the deck and is NOT marked commander.
  const sr = got.find(c => c.name === 'Sol Ring')
  assert.equal(sr?.isCommander, false)
})

test('parseDeckLines — quantities, set codes, and foil markers all round-trip', () => {
  const text = `2 Sol Ring (CMM) 339\n1 Llanowar Elves (DOM) 168 *F*\n1x Cyclonic Rift`
  const got = parseDeckLines(text)
  assert.equal(got.length, 3)
  assert.equal(got[0].quantity, 2)
  assert.equal(got[0].set, 'CMM')
  assert.equal(got[1].set, 'DOM')
  assert.equal(got[2].name, 'Cyclonic Rift')
})

test('parseDeckLines — empty / null returns empty array', () => {
  assert.deepEqual(parseDeckLines(''), [])
  assert.deepEqual(parseDeckLines(null), [])
})

// ── summarize ─────────────────────────────────────────────────────────────

test('summarize — counts respect quantity, commander surfaced', () => {
  const text = "Commander: Atraxa, Praetors' Voice\n1 Sol Ring\n2 Forest"
  const s = summarize(parseDeckLines(text))
  assert.equal(s.cardCount, 4)
  assert.equal(s.uniqueCount, 3)
  assert.equal(s.commander, "Atraxa, Praetors' Voice")
  assert.equal(s.hasIllegalMultiples, false) // Forest is a basic
})

test('summarize — flags qty>1 of a non-basic non-commander', () => {
  const s = summarize(parseDeckLines('2 Sol Ring\n1 Forest'))
  assert.equal(s.hasIllegalMultiples, true)
})

test('summarize — empty input returns zero structure', () => {
  const s = summarize([])
  assert.deepEqual(s, { cardCount: 0, uniqueCount: 0, commander: '', hasIllegalMultiples: false })
})

// ── BASIC_LANDS sanity ────────────────────────────────────────────────────

test('BASIC_LANDS — contains canonical basics + snow + wastes', () => {
  for (const b of ['plains', 'island', 'swamp', 'mountain', 'forest', 'wastes']) {
    assert.equal(BASIC_LANDS.has(b), true, b)
  }
  assert.equal(BASIC_LANDS.has('snow-covered forest'), true)
})
