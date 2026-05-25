// Unit tests for the DENSE card-list view helpers
// (lib/cardListDense.js).
//
// Run: npm run test:unit              (from hexdek/)
// Or:  node --test tests/unit/cardListDense.test.js
//
// The React component (CardListDense) is exercised in the browser;
// these cases pin the type bucketing, color-identity parsing,
// comparator semantics (stable, tie-broken by name) and the
// toggleSort interaction so a future refactor can't silently shift
// the column ordering users have learned to rely on.

import test from 'node:test'
import assert from 'node:assert/strict'

import {
  SORT_KEYS,
  cardCMCForSort,
  cardColorIdentity,
  cardColorIdentityString,
  cardRole,
  cardTypeBucket,
  compareCards,
  sortCards,
  toggleSort,
} from '../../src/lib/cardListDense.js'

const SOL_RING = { name: 'Sol Ring', cmc: 1, type_line: 'Artifact', mana_cost: '{1}' }
const LLANOWAR  = { name: 'Llanowar Elves', cmc: 1, type_line: 'Creature — Elf Druid', mana_cost: '{G}' }
const BOLT      = { name: 'Lightning Bolt', cmc: 1, type_line: 'Instant', mana_cost: '{R}' }
const WRATH     = { name: 'Wrath of God', cmc: 4, type_line: 'Sorcery', mana_cost: '{2}{W}{W}' }
const CRATERHOOF = { name: 'Craterhoof Behemoth', cmc: 8, type_line: 'Creature — Beast', mana_cost: '{5}{G}{G}{G}' }
const FOREST    = { name: 'Forest', cmc: 0, type_line: 'Basic Land — Forest', mana_cost: '' }
const ATRAXA    = { name: 'Atraxa, Praetors\' Voice', cmc: 4, type_line: 'Legendary Creature — Phyrexian Angel Horror', mana_cost: '{G}{W}{U}{B}' }
const ARTLAND   = { name: 'Inventors\' Fair', cmc: 0, type_line: 'Legendary Land', mana_cost: '' }
const HYBRID    = { name: 'Boros Charm', cmc: 2, type_line: 'Instant', mana_cost: '{R}{W}' }
const ROLES     = { 'Sol Ring': 'ramp', 'Lightning Bolt': 'removal', 'Wrath of God': 'wipe', 'Llanowar Elves': 'ramp' }

// ── cardTypeBucket ────────────────────────────────────────────────────────

test('cardTypeBucket — land beats artifact (artifact lands count as lands)', () => {
  assert.equal(cardTypeBucket(ARTLAND), 'land')
})

test('cardTypeBucket — creature beats artifact/enchantment (creature-types win)', () => {
  assert.equal(cardTypeBucket({ type_line: 'Artifact Creature — Construct' }), 'creature')
  assert.equal(cardTypeBucket({ type_line: 'Enchantment Creature — Plant' }), 'creature')
})

test('cardTypeBucket — base types route correctly', () => {
  assert.equal(cardTypeBucket(BOLT),    'instant')
  assert.equal(cardTypeBucket(WRATH),   'sorcery')
  assert.equal(cardTypeBucket(SOL_RING),'artifact')
  assert.equal(cardTypeBucket({ type_line: 'Planeswalker — Atraxa' }), 'planeswalker')
})

test('cardTypeBucket — falls back to "other" when type_line missing', () => {
  assert.equal(cardTypeBucket({}), 'other')
  assert.equal(cardTypeBucket(null), 'other')
})

test('cardTypeBucket — accepts the types[] array shape from analysis output', () => {
  assert.equal(cardTypeBucket({ types: ['Creature', 'Elf'] }), 'creature')
})

// ── cardCMCForSort ────────────────────────────────────────────────────────

test('cardCMCForSort — lands sink with sort key -1 so they bucket below 0-CMC spells', () => {
  assert.equal(cardCMCForSort(FOREST), -1)
  assert.equal(cardCMCForSort(ARTLAND), -1)
  // 0-CMC spell (Lotus Petal-style) stays at 0.
  assert.equal(cardCMCForSort({ name: 'Lotus Petal', cmc: 0, type_line: 'Artifact' }), 0)
})

test('cardCMCForSort — non-land defaults to 0 when cmc missing', () => {
  assert.equal(cardCMCForSort({ name: 'Mystery', type_line: 'Creature' }), 0)
})

// ── cardColorIdentity / cardColorIdentityString ───────────────────────────

test('cardColorIdentity — single-color mana cost', () => {
  assert.deepEqual([...cardColorIdentity(LLANOWAR)], ['G'])
})

test('cardColorIdentity — multi-color partner-style cost', () => {
  const ids = cardColorIdentity(ATRAXA)
  assert.equal(ids.size, 4)
  for (const c of ['W', 'U', 'B', 'G']) assert.ok(ids.has(c))
  assert.equal(ids.has('R'), false)
})

test('cardColorIdentity — hybrid pip counts both colors', () => {
  const ids = cardColorIdentity({ mana_cost: '{W/U}{W/U}' })
  assert.equal(ids.size, 2)
  assert.ok(ids.has('W') && ids.has('U'))
})

test('cardColorIdentity — colorless / missing returns empty set', () => {
  assert.equal(cardColorIdentity(FOREST).size, 0)
  assert.equal(cardColorIdentity({}).size, 0)
})

test('cardColorIdentityString — canonical WUBRG order (sort stability)', () => {
  // Construct two equivalent costs in different orders — both must
  // render identically in canonical W-U-B-R-G order regardless of
  // pip insertion order.
  assert.equal(cardColorIdentityString(HYBRID), 'WR')
  assert.equal(cardColorIdentityString({ mana_cost: '{W}{R}' }), 'WR')
  // ATRAXA: WUBG (no R) — note canonical order is W, U, B, R, G so
  // green sits LAST even though pip {G} appears first in the cost.
  assert.equal(cardColorIdentityString(ATRAXA), 'WUBG')
})

// ── cardRole ──────────────────────────────────────────────────────────────

test('cardRole — pulls per-name role from the Freya roles map', () => {
  assert.equal(cardRole(SOL_RING, ROLES), 'ramp')
  assert.equal(cardRole(BOLT, ROLES), 'removal')
})

test('cardRole — returns "" when card or map missing', () => {
  assert.equal(cardRole(SOL_RING, null), '')
  assert.equal(cardRole({}, ROLES), '')
  assert.equal(cardRole(null, ROLES), '')
})

// ── compareCards ──────────────────────────────────────────────────────────

test('compareCards — name asc/desc', () => {
  const cards = [BOLT, ATRAXA, SOL_RING]
  cards.sort((a, b) => compareCards(a, b, 'name', 'asc'))
  assert.deepEqual(cards.map(c => c.name), ["Atraxa, Praetors' Voice", 'Lightning Bolt', 'Sol Ring'])
  cards.sort((a, b) => compareCards(a, b, 'name', 'desc'))
  assert.deepEqual(cards.map(c => c.name), ['Sol Ring', 'Lightning Bolt', "Atraxa, Praetors' Voice"])
})

test('compareCards — cmc ascending puts lands first (sort key -1)', () => {
  const cards = [WRATH, SOL_RING, FOREST, CRATERHOOF]
  cards.sort((a, b) => compareCards(a, b, 'cmc', 'asc'))
  assert.equal(cards[0].name, 'Forest')
  assert.equal(cards[cards.length - 1].name, 'Craterhoof Behemoth')
})

test('compareCards — equal CMC ties break by name ascending (stable)', () => {
  const cards = [LLANOWAR, BOLT, SOL_RING] // all CMC 1
  cards.sort((a, b) => compareCards(a, b, 'cmc', 'desc'))
  // Direction is desc on the cmc value, but ties → name ascending.
  assert.deepEqual(cards.map(c => c.name), ['Lightning Bolt', 'Llanowar Elves', 'Sol Ring'])
})

test('compareCards — type sorts in EDHREC-friendly order (creatures first, lands last)', () => {
  const mix = [FOREST, SOL_RING, LLANOWAR, BOLT, WRATH, ATRAXA]
  mix.sort((a, b) => compareCards(a, b, 'type', 'asc'))
  // creature, planeswalker, instant, sorcery, artifact, enchantment, land
  assert.equal(cardTypeBucket(mix[0]), 'creature')
  assert.equal(cardTypeBucket(mix[mix.length - 1]), 'land')
})

test('compareCards — role sort puts assigned roles before unassigned', () => {
  const cards = [WRATH, { name: 'Mystery', cmc: 3, type_line: 'Creature' }, SOL_RING]
  cards.sort((a, b) => compareCards(a, b, 'role', 'asc', ROLES))
  // ramp (Sol Ring), wipe (Wrath), then unassigned Mystery.
  assert.equal(cards[0].name, 'Sol Ring')
  assert.equal(cards[1].name, 'Wrath of God')
  assert.equal(cards[2].name, 'Mystery')
})

test('compareCards — color sort buckets colorless last in asc, first in desc', () => {
  const cards = [SOL_RING, BOLT, ATRAXA]
  cards.sort((a, b) => compareCards(a, b, 'color', 'asc'))
  // Colored cards first, colorless (Sol Ring) sinks.
  assert.equal(cards[cards.length - 1].name, 'Sol Ring')
  cards.sort((a, b) => compareCards(a, b, 'color', 'desc'))
  assert.equal(cards[0].name, 'Sol Ring')
})

test('compareCards — qty desc surfaces basic-land stacks first', () => {
  const cards = [
    { name: 'Forest', cmc: 0, type_line: 'Basic Land — Forest', quantity: 30 },
    { name: 'Sol Ring', cmc: 1, type_line: 'Artifact', quantity: 1 },
  ]
  cards.sort((a, b) => compareCards(a, b, 'qty', 'desc'))
  assert.equal(cards[0].name, 'Forest')
})

// ── sortCards ─────────────────────────────────────────────────────────────

test('sortCards — non-mutating: returns a new array', () => {
  const cards = [WRATH, SOL_RING]
  const sorted = sortCards(cards, 'name', 'asc')
  assert.notEqual(sorted, cards)
  // original untouched
  assert.deepEqual(cards.map(c => c.name), ['Wrath of God', 'Sol Ring'])
})

test('sortCards — unknown key returns input order in a fresh array', () => {
  const cards = [SOL_RING, BOLT]
  const sorted = sortCards(cards, 'nope', 'asc')
  assert.deepEqual(sorted.map(c => c.name), ['Sol Ring', 'Lightning Bolt'])
  assert.notEqual(sorted, cards)
})

test('sortCards — null / non-array returns []', () => {
  assert.deepEqual(sortCards(null, 'name', 'asc'), [])
  assert.deepEqual(sortCards(undefined, 'name', 'asc'), [])
})

// ── toggleSort ────────────────────────────────────────────────────────────

test('toggleSort — clicking same column flips direction', () => {
  assert.deepEqual(toggleSort({ key: 'name', dir: 'asc' },  'name'), { key: 'name', dir: 'desc' })
  assert.deepEqual(toggleSort({ key: 'name', dir: 'desc' }, 'name'), { key: 'name', dir: 'asc' })
})

test('toggleSort — string column defaults to asc when new', () => {
  assert.deepEqual(toggleSort({ key: 'cmc', dir: 'asc' }, 'name'),  { key: 'name', dir: 'asc' })
  assert.deepEqual(toggleSort({ key: 'cmc', dir: 'asc' }, 'role'),  { key: 'role', dir: 'asc' })
  assert.deepEqual(toggleSort({ key: 'cmc', dir: 'asc' }, 'color'), { key: 'color', dir: 'asc' })
})

test('toggleSort — numeric column (cmc / qty) defaults to desc when new', () => {
  // Tappedout / Moxfield trained users to expect "click qty → biggest first."
  assert.deepEqual(toggleSort({ key: 'name', dir: 'asc' }, 'cmc'), { key: 'cmc', dir: 'desc' })
  assert.deepEqual(toggleSort({ key: 'name', dir: 'asc' }, 'qty'), { key: 'qty', dir: 'desc' })
})

test('toggleSort — null current state still resolves to a valid sort', () => {
  assert.deepEqual(toggleSort(null, 'name'), { key: 'name', dir: 'asc' })
  assert.deepEqual(toggleSort(undefined, 'cmc'), { key: 'cmc', dir: 'desc' })
})

// ── SORT_KEYS contract ────────────────────────────────────────────────────

test('SORT_KEYS — every key the UI button row uses is recognized', () => {
  for (const k of ['name', 'cmc', 'type', 'role', 'color', 'qty']) {
    assert.ok(SORT_KEYS.includes(k), `missing ${k}`)
  }
})
