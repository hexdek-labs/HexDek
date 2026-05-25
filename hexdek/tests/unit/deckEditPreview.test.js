// Unit tests for the Workshop live-curve preview helpers.
//
// Run: npm run test:unit                (from hexdek/)
// Or:  node --test tests/unit/deckEditPreview.test.js
//
// The React component (WorkshopCurvePreview) is exercised in the
// browser; these tests pin the pure parser + curve math so curve
// regressions can fail before a deploy reaches dev.hexdek.dev.

import test from 'node:test'
import assert from 'node:assert/strict'

import {
  buildCardMeta,
  computeCurve,
  curveDelta,
  isLandMeta,
  parseDeckTextToCounts,
} from '../../src/screens/deckEditPreview.js'

// ── parseDeckTextToCounts ─────────────────────────────────────────────────

test('parseDeckTextToCounts — basic "N name" lines', () => {
  const m = parseDeckTextToCounts('1 Sol Ring\n2 Llanowar Elves\n1 Forest')
  assert.equal(m.get('Sol Ring'), 1)
  assert.equal(m.get('Llanowar Elves'), 2)
  assert.equal(m.get('Forest'), 1)
})

test('parseDeckTextToCounts — duplicate names accumulate', () => {
  const m = parseDeckTextToCounts('1 Sol Ring\n2 Sol Ring')
  assert.equal(m.get('Sol Ring'), 3)
})

test('parseDeckTextToCounts — COMMANDER: prefix folds in as qty=1', () => {
  const m = parseDeckTextToCounts('COMMANDER: Atraxa, Praetors\' Voice\n1 Sol Ring')
  assert.equal(m.get("Atraxa, Praetors' Voice"), 1)
  assert.equal(m.get('Sol Ring'), 1)
})

test('parseDeckTextToCounts — blank lines / garbage skipped', () => {
  const m = parseDeckTextToCounts('\n\n   \n// comment\n1 Sol Ring\n')
  assert.equal(m.size, 1)
  assert.equal(m.get('Sol Ring'), 1)
})

test('parseDeckTextToCounts — null / empty input returns empty map', () => {
  assert.equal(parseDeckTextToCounts('').size, 0)
  assert.equal(parseDeckTextToCounts(null).size, 0)
  assert.equal(parseDeckTextToCounts(undefined).size, 0)
})

// ── buildCardMeta ─────────────────────────────────────────────────────────

test('buildCardMeta — extracts cmc + type_line by name', () => {
  const meta = buildCardMeta([
    { name: 'Sol Ring', cmc: 1, type_line: 'Artifact' },
    { name: 'Forest', cmc: 0, type_line: 'Basic Land — Forest' },
  ])
  assert.deepEqual(meta.get('Sol Ring'), { cmc: 1, type_line: 'Artifact' })
  assert.deepEqual(meta.get('Forest'), { cmc: 0, type_line: 'Basic Land — Forest' })
})

test('buildCardMeta — missing cmc defaults to 0, missing type_line to ""', () => {
  const meta = buildCardMeta([{ name: 'Mystery' }])
  assert.deepEqual(meta.get('Mystery'), { cmc: 0, type_line: '' })
})

test('buildCardMeta — null / empty input returns empty map', () => {
  assert.equal(buildCardMeta(null).size, 0)
  assert.equal(buildCardMeta([]).size, 0)
})

// ── isLandMeta ────────────────────────────────────────────────────────────

test('isLandMeta — recognizes Land in type_line', () => {
  assert.equal(isLandMeta({ type_line: 'Basic Land — Forest', cmc: 0 }), true)
  assert.equal(isLandMeta({ type_line: 'Land', cmc: 0 }), true)
  assert.equal(isLandMeta({ type_line: 'Artifact Land', cmc: 0 }), true)
})

test('isLandMeta — non-land returns false even at cmc 0', () => {
  assert.equal(isLandMeta({ type_line: 'Artifact', cmc: 0 }), false)
  assert.equal(isLandMeta({ type_line: 'Creature', cmc: 1 }), false)
})

test('isLandMeta — empty type_line + cmc 0 treated as land (basic fallback)', () => {
  // Backend basics sometimes come back with empty meta until enriched —
  // we'd rather drop them than mis-bucket them as 0-CMC spells.
  assert.equal(isLandMeta({ type_line: '', cmc: 0 }), true)
})

// ── computeCurve ──────────────────────────────────────────────────────────

const META_FIXTURE = new Map([
  ['Sol Ring', { cmc: 1, type_line: 'Artifact' }],
  ['Llanowar Elves', { cmc: 1, type_line: 'Creature — Elf Druid' }],
  ['Cyclonic Rift', { cmc: 2, type_line: 'Instant' }],
  ['Wrath of God', { cmc: 4, type_line: 'Sorcery' }],
  ['Craterhoof Behemoth', { cmc: 8, type_line: 'Creature — Beast' }],
  ['Forest', { cmc: 0, type_line: 'Basic Land — Forest' }],
])

test('computeCurve — buckets non-land CMCs', () => {
  const counts = new Map([
    ['Sol Ring', 1],
    ['Llanowar Elves', 1],
    ['Cyclonic Rift', 1],
    ['Wrath of God', 1],
    ['Forest', 30],
  ])
  const { curve, total, unknown } = computeCurve(counts, META_FIXTURE)
  // Two 1-CMC, one 2-CMC, one 4-CMC, 30 lands excluded.
  assert.equal(curve[1], 2)
  assert.equal(curve[2], 1)
  assert.equal(curve[4], 1)
  assert.equal(curve[0], 0)
  assert.equal(total, 4)
  assert.deepEqual(unknown, [])
})

test('computeCurve — CMC 7+ collapses into bucket 7', () => {
  const counts = new Map([['Craterhoof Behemoth', 2]])
  const { curve, total } = computeCurve(counts, META_FIXTURE)
  assert.equal(curve[7], 2)
  assert.equal(total, 2)
})

test('computeCurve — unknown names returned and excluded from histogram', () => {
  const counts = new Map([
    ['Sol Ring', 1],
    ['Some New Card', 1],
    ['Another Mystery', 2],
  ])
  const { curve, total, unknown } = computeCurve(counts, META_FIXTURE)
  assert.equal(curve[1], 1)
  assert.equal(total, 1)
  assert.deepEqual(unknown.sort(), ['Another Mystery', 'Some New Card'])
})

test('computeCurve — quantity multiplies the bucket count', () => {
  const counts = new Map([['Wrath of God', 3]])
  const { curve, total } = computeCurve(counts, META_FIXTURE)
  assert.equal(curve[4], 3)
  assert.equal(total, 3)
})

test('computeCurve — empty input returns zero curve', () => {
  const { curve, total, unknown } = computeCurve(new Map(), META_FIXTURE)
  assert.deepEqual(curve, [0, 0, 0, 0, 0, 0, 0, 0])
  assert.equal(total, 0)
  assert.deepEqual(unknown, [])
})

// ── curveDelta ────────────────────────────────────────────────────────────

test('curveDelta — per-bucket difference, current minus baseline', () => {
  const base = [10, 5, 8, 7, 4, 2, 1, 0]
  const curr = [10, 6, 8, 5, 4, 2, 1, 1]
  assert.deepEqual(curveDelta(base, curr), [0, 1, 0, -2, 0, 0, 0, 1])
})

test('curveDelta — missing buckets treated as zero', () => {
  assert.deepEqual(curveDelta([], [1, 2]), [1, 2, 0, 0, 0, 0, 0, 0])
  assert.deepEqual(curveDelta([3, 3], []), [-3, -3, 0, 0, 0, 0, 0, 0])
})

// ── end-to-end: parse → compute → delta ───────────────────────────────────

test('end-to-end — adding a card surfaces a delta in the right bucket', () => {
  const baseText = '1 Sol Ring\n1 Wrath of God\n30 Forest'
  const editText = '1 Sol Ring\n2 Wrath of God\n30 Forest' // +1 4-CMC
  const baseCurve = computeCurve(parseDeckTextToCounts(baseText), META_FIXTURE).curve
  const currCurve = computeCurve(parseDeckTextToCounts(editText), META_FIXTURE).curve
  const d = curveDelta(baseCurve, currCurve)
  assert.equal(d[4], 1)
  assert.equal(d.reduce((a, b) => a + b, 0), 1)
})

test('end-to-end — removing a card surfaces a negative delta', () => {
  const baseText = '1 Sol Ring\n1 Llanowar Elves'
  const editText = '1 Sol Ring' // -1 at 1-CMC
  const baseCurve = computeCurve(parseDeckTextToCounts(baseText), META_FIXTURE).curve
  const currCurve = computeCurve(parseDeckTextToCounts(editText), META_FIXTURE).curve
  const d = curveDelta(baseCurve, currCurve)
  assert.equal(d[1], -1)
})
