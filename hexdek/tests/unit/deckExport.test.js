// Unit tests for deck-export format builders.
//
// Run: node --test tests/unit/deckExport.test.js   (from hexdek/)
// Asserts each format's section headers, commander placement, and
// quantity handling against fixed decklists. The Moxfield output
// shape is pinned to round-trip with the deckparser's reader-side
// fixtures (TestParseDeckReader_CommanderSectionHeader and
// TestParseDeckReader_MoxfieldCommandersPartner in
// internal/deckparser/deckparser_test.go).

import test from 'node:test'
import assert from 'node:assert/strict'
import { buildDeckExport, parseCommanderNames, exportExtension } from '../../src/lib/deckExport.js'

const CARDS = [
  { name: 'Atraxa, Praetors\' Voice', quantity: 1 },
  { name: 'Sol Ring', quantity: 1 },
  { name: 'Plains', quantity: 8 },
  { name: 'Lightning Bolt' },     // no quantity → 1
]

// --- parseCommanderNames ---------------------------------------------------

test('parseCommanderNames: single string returns one entry', () => {
  assert.deepEqual(parseCommanderNames('Atraxa, Praetors\' Voice'), ['Atraxa, Praetors\' Voice'])
})

test('parseCommanderNames: " // " separator splits partners', () => {
  assert.deepEqual(
    parseCommanderNames('Kraum, Ludevic\'s Opus // Tymna the Weaver'),
    ['Kraum, Ludevic\'s Opus', 'Tymna the Weaver'],
  )
})

test('parseCommanderNames: " / " separator splits partners', () => {
  assert.deepEqual(
    parseCommanderNames('Kraum / Tymna'),
    ['Kraum', 'Tymna'],
  )
})

test('parseCommanderNames: comma alone does NOT split (commas appear in names)', () => {
  // "Atraxa, Praetors' Voice" must NOT split into ['Atraxa', 'Praetors\' Voice'].
  assert.deepEqual(parseCommanderNames('Atraxa, Praetors\' Voice'), ['Atraxa, Praetors\' Voice'])
})

test('parseCommanderNames: array input passes through (trimmed, non-empty)', () => {
  assert.deepEqual(
    parseCommanderNames(['Kraum  ', '', 'Tymna']),
    ['Kraum', 'Tymna'],
  )
})

test('parseCommanderNames: null/empty → []', () => {
  assert.deepEqual(parseCommanderNames(null), [])
  assert.deepEqual(parseCommanderNames(''), [])
  assert.deepEqual(parseCommanderNames('   '), [])
})

// --- moxfield format -------------------------------------------------------

test('moxfield: single commander uses singular "Commander" header', () => {
  const txt = buildDeckExport('moxfield', CARDS, 'Atraxa, Praetors\' Voice')
  assert.equal(txt,
    [
      'Commander',
      '1 Atraxa, Praetors\' Voice',
      '',
      'Deck',
      '1 Sol Ring',
      '8 Plains',
      '1 Lightning Bolt',
    ].join('\n'),
  )
})

test('moxfield: partner pair uses plural "Commanders" header', () => {
  const cards = [
    { name: 'Kraum, Ludevic\'s Opus', quantity: 1 },
    { name: 'Tymna the Weaver', quantity: 1 },
    { name: 'Sol Ring', quantity: 1 },
    { name: 'Command Tower', quantity: 1 },
  ]
  const txt = buildDeckExport('moxfield', cards, ['Kraum, Ludevic\'s Opus', 'Tymna the Weaver'])
  assert.equal(txt,
    [
      'Commanders',
      '1 Kraum, Ludevic\'s Opus',
      '1 Tymna the Weaver',
      '',
      'Deck',
      '1 Sol Ring',
      '1 Command Tower',
    ].join('\n'),
  )
})

test('moxfield: no commander declared → just a Deck section', () => {
  const cards = [{ name: 'Sol Ring', quantity: 1 }, { name: 'Plains', quantity: 8 }]
  const txt = buildDeckExport('moxfield', cards, '')
  assert.equal(txt, ['Deck', '1 Sol Ring', '8 Plains'].join('\n'))
})

test('moxfield: DFC commander matches by front face', () => {
  const cards = [
    { name: 'Jace, Vryn\'s Prodigy // Jace, Telepath Unbound', quantity: 1 },
    { name: 'Sol Ring', quantity: 1 },
  ]
  const txt = buildDeckExport('moxfield', cards, 'Jace, Vryn\'s Prodigy')
  // The DFC card should land in the Commander section, not the mainboard.
  assert.match(txt, /^Commander\n1 Jace, Vryn's Prodigy \/\/ Jace, Telepath Unbound/)
  assert.match(txt, /Deck\n1 Sol Ring$/)
})

// --- mtgo + arena formats (regression — refactor must preserve behavior) ---

test('mtgo: commander lands in Sideboard section', () => {
  const txt = buildDeckExport('mtgo', CARDS, 'Atraxa, Praetors\' Voice')
  assert.equal(txt,
    [
      '1 Sol Ring',
      '8 Plains',
      '1 Lightning Bolt',
      '',
      'Sideboard',
      '1 Atraxa, Praetors\' Voice',
    ].join('\n'),
  )
})

test('arena: commander lands in Commander section, (SET) CN passes through', () => {
  const cards = [
    { name: 'Atraxa, Praetors\' Voice', quantity: 1, set: 'cmr', collector_number: '347' },
    { name: 'Sol Ring', quantity: 1, set: 'cmm' },
    { name: 'Plains', quantity: 8 },
  ]
  const txt = buildDeckExport('arena', cards, 'Atraxa, Praetors\' Voice')
  assert.equal(txt,
    [
      '1 Sol Ring (CMM)',
      '8 Plains',
      '',
      'Commander',
      '1 Atraxa, Praetors\' Voice (CMR) 347',
    ].join('\n'),
  )
})

// --- raw format ------------------------------------------------------------

test('raw: just names, no quantities, no headers, original order', () => {
  const txt = buildDeckExport('raw', CARDS, 'Atraxa, Praetors\' Voice')
  assert.equal(txt, 'Atraxa, Praetors\' Voice\nSol Ring\nPlains\nLightning Bolt')
})

// --- empties / dispatch ----------------------------------------------------

test('buildDeckExport: empty cards → empty string', () => {
  assert.equal(buildDeckExport('moxfield', [], 'Atraxa'), '')
  assert.equal(buildDeckExport('mtgo', null, 'Atraxa'), '')
})

test('buildDeckExport: unknown format falls back to MTGO shape', () => {
  const txt = buildDeckExport('made-up-format', CARDS, 'Atraxa, Praetors\' Voice')
  assert.match(txt, /\nSideboard\n1 Atraxa/)
})

// --- exportExtension -------------------------------------------------------

test('exportExtension: per-format file extensions', () => {
  assert.equal(exportExtension('moxfield'), '_moxfield.txt')
  assert.equal(exportExtension('mtgo'), '.dec')
  assert.equal(exportExtension('arena'), '_arena.txt')
  assert.equal(exportExtension('raw'), '.txt')
  assert.equal(exportExtension('???'), '.txt')
})
