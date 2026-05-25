// Unit tests for the Freya coaching index — the bridge between
// `analysis.cuttable_card_rationale` and per-card inline UI markers.
//
// Run: node --test tests/unit/freyaCoaching.test.js   (from hexdek/)

import test from 'node:test'
import assert from 'node:assert/strict'
import {
  canonicalCardKey,
  buildCoachingIndex,
  coachingForCard,
  isCardFlaggedForCut,
  coachingEntriesByKind,
} from '../../src/lib/freyaCoaching.js'

// --- canonicalCardKey ------------------------------------------------------

test('canonicalCardKey: lowercases and trims', () => {
  assert.equal(canonicalCardKey('  Sol Ring  '), 'sol ring')
})

test('canonicalCardKey: strips COMMANDER: prefix', () => {
  // Decklist text format pins commanders with a "COMMANDER: " prefix;
  // the cuttable list never carries that prefix, so we have to strip
  // on the card-side to align the two.
  assert.equal(canonicalCardKey('COMMANDER: Atraxa, Praetors\' Voice'), 'atraxa, praetors\' voice')
})

test('canonicalCardKey: DFC cards key by the front face', () => {
  assert.equal(
    canonicalCardKey("Jace, Vryn's Prodigy // Jace, Telepath Unbound"),
    "jace, vryn's prodigy",
  )
})

test('canonicalCardKey: empty / null → ""', () => {
  assert.equal(canonicalCardKey(''), '')
  assert.equal(canonicalCardKey(null), '')
  assert.equal(canonicalCardKey(undefined), '')
})

// --- buildCoachingIndex ----------------------------------------------------

test('buildCoachingIndex: empty / missing analysis → empty map', () => {
  assert.equal(buildCoachingIndex(null).size, 0)
  assert.equal(buildCoachingIndex(undefined).size, 0)
  assert.equal(buildCoachingIndex({}).size, 0)
  assert.equal(buildCoachingIndex({ cuttable_card_rationale: [] }).size, 0)
})

test('buildCoachingIndex: structured rationale fields land as expected', () => {
  const idx = buildCoachingIndex({
    cuttable_card_rationale: [
      {
        name: 'Howling Mine',
        reason: 'Symmetric draw',
        detected: 'CMC 2 artifact, +1 card to all players',
        why_cut: 'Hands opponents resources you can\'t leverage',
        effect: 'Frees a slot for an asymmetric draw engine',
        suggested: ['Mystic Remora', 'Rhystic Study'],
      },
    ],
  })
  assert.equal(idx.size, 1)
  const entry = idx.get('howling mine')
  assert.ok(entry?.cut)
  assert.equal(entry.cut.kind, 'cut')
  assert.equal(entry.cut.name, 'Howling Mine')
  assert.equal(entry.cut.reason, 'Symmetric draw')
  assert.equal(entry.cut.detected, 'CMC 2 artifact, +1 card to all players')
  assert.equal(entry.cut.whyCut, 'Hands opponents resources you can\'t leverage')
  assert.equal(entry.cut.effect, 'Frees a slot for an asymmetric draw engine')
  assert.deepEqual(entry.cut.suggested, ['Mystic Remora', 'Rhystic Study'])
})

test('buildCoachingIndex: legacy string-only `cuttable_cards` still produces entries', () => {
  // Older strategy.json files on disk: a flat name list, no rationale.
  // Marker should still appear but the detail fields are blank — caller
  // can decide whether to render an empty popover.
  const idx = buildCoachingIndex({
    cuttable_cards: ['Mind Stone', 'Worn Powerstone'],
  })
  assert.equal(idx.size, 2)
  const ms = idx.get('mind stone')
  assert.equal(ms?.cut?.name, 'Mind Stone')
  assert.equal(ms?.cut?.reason, '')
  assert.deepEqual(ms?.cut?.suggested, [])
})

test('buildCoachingIndex: structured list wins over legacy list when both present', () => {
  const idx = buildCoachingIndex({
    cuttable_card_rationale: [{ name: 'Sol Ring', reason: 'structured' }],
    cuttable_cards: ['Sol Ring', 'Arcane Signet'],
  })
  // Should NOT process the legacy list when structured is present —
  // otherwise Arcane Signet would slip in even though Freya's newer
  // version didn't flag it.
  assert.equal(idx.size, 1)
  assert.equal(idx.get('sol ring')?.cut?.reason, 'structured')
  assert.equal(idx.has('arcane signet'), false)
})

test('buildCoachingIndex: skips malformed entries', () => {
  const idx = buildCoachingIndex({
    cuttable_card_rationale: [
      null,
      { name: '' },               // empty name — skip
      'Mind Stone',               // ok (legacy string mixed in)
      { name: 'Howling Mine' },   // ok
      42,                         // non-string non-object — skip
    ],
  })
  assert.equal(idx.size, 2)
  assert.ok(idx.has('mind stone'))
  assert.ok(idx.has('howling mine'))
})

test('buildCoachingIndex: indexes by canonical key so DFC cards match', () => {
  const idx = buildCoachingIndex({
    cuttable_card_rationale: [{ name: "Jace, Vryn's Prodigy" }],
  })
  // The decklist row would carry the full DFC name. Lookup against
  // the full name still resolves via the canonical front-face key.
  assert.ok(coachingForCard(idx, "Jace, Vryn's Prodigy // Jace, Telepath Unbound"))
})

// --- coachingForCard / isCardFlaggedForCut --------------------------------

test('coachingForCard: returns entry on match, null otherwise', () => {
  const idx = buildCoachingIndex({
    cuttable_card_rationale: [{ name: 'Howling Mine', reason: 'symmetric' }],
  })
  assert.equal(coachingForCard(idx, 'Howling Mine')?.cut?.reason, 'symmetric')
  assert.equal(coachingForCard(idx, 'Sol Ring'), null)
})

test('coachingForCard: null inputs → null', () => {
  const idx = buildCoachingIndex({ cuttable_card_rationale: [{ name: 'X' }] })
  assert.equal(coachingForCard(null, 'X'), null)
  assert.equal(coachingForCard(idx, ''), null)
  assert.equal(coachingForCard(idx, null), null)
})

test('isCardFlaggedForCut: true only when a cut entry exists for the card', () => {
  const idx = buildCoachingIndex({
    cuttable_card_rationale: [{ name: 'Howling Mine' }],
  })
  assert.equal(isCardFlaggedForCut(idx, 'Howling Mine'), true)
  assert.equal(isCardFlaggedForCut(idx, 'Sol Ring'), false)
  assert.equal(isCardFlaggedForCut(null, 'Howling Mine'), false)
})

// --- coachingEntriesByKind -------------------------------------------------

test('coachingEntriesByKind("cut"): every flagged card in insertion order', () => {
  const idx = buildCoachingIndex({
    cuttable_card_rationale: [
      { name: 'Howling Mine' },
      { name: 'Mind Stone' },
      { name: 'Worn Powerstone' },
    ],
  })
  const entries = coachingEntriesByKind(idx, 'cut')
  assert.equal(entries.length, 3)
  assert.deepEqual(entries.map(e => e.name), ['Howling Mine', 'Mind Stone', 'Worn Powerstone'])
})

test('coachingEntriesByKind: unknown kind → []', () => {
  const idx = buildCoachingIndex({
    cuttable_card_rationale: [{ name: 'Howling Mine' }],
  })
  assert.deepEqual(coachingEntriesByKind(idx, 'upgrade'), [])
})
