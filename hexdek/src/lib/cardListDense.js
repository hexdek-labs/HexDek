// Pure helpers for the DENSE card-list view on the deck page.
//
// The CardRolesGrid (tile view) groups by Freya role and shows large
// images — beautiful but slow to scan when you want "what's my curve
// against role X" or "all my 2-CMC removal." The DENSE view trades
// images for a sortable spreadsheet. These helpers do the parsing /
// classification / sorting so the React shell stays a thin renderer.

// Card type buckets, highest priority first. Same precedence as
// DeckStatsSummary's TYPE_BUCKETS — land beats everything, creature
// beats artifact/enchantment so enchantment-creatures count as
// creatures (matches EDHREC convention).
const TYPE_PRIORITY = [
  ['land',         /\bland\b/i],
  ['planeswalker', /\bplaneswalker\b/i],
  ['creature',     /\bcreature\b/i],
  ['enchantment',  /\benchantment\b/i],
  ['artifact',     /\bartifact\b/i],
  ['sorcery',      /\bsorcery\b/i],
  ['instant',      /\binstant\b/i],
]

// Sort order for the TYPE column — lands sink last so the deck reads
// "spells first, lands at the bottom" like every paper-deck list.
const TYPE_SORT_ORDER = {
  creature: 1, planeswalker: 2, instant: 3, sorcery: 4,
  artifact: 5, enchantment: 6, land: 7, other: 8,
}

export function cardTypeBucket(card) {
  const t = String(card?.type_line || (Array.isArray(card?.types) ? card.types.join(' ') : '') || '').toLowerCase()
  if (!t) return 'other'
  for (const [bucket, re] of TYPE_PRIORITY) {
    if (re.test(t)) return bucket
  }
  return 'other'
}

// Effective CMC for sort. Land buckets at -1 so they sort separately
// from 0-CMC spells (Mox / Sol-Ring-by-cost). Falls back to 0 when
// the card meta is missing CMC entirely.
export function cardCMCForSort(card) {
  if (!card) return 0
  if (cardTypeBucket(card) === 'land') return -1
  return Number.isFinite(card.cmc) ? card.cmc : 0
}

// Role string per Freya: { "Sol Ring": "ramp", ... }. Returns '' when
// the role map is missing or the card has no role assignment.
export function cardRole(card, cardRoles) {
  if (!cardRoles || !card?.name) return ''
  return cardRoles[card.name] || ''
}

// Color identity from the mana cost. Returns a Set<W|U|B|R|G> by
// scanning {…} pips. Hybrid halves ({W/U}) count for both colors.
// Empty when the card is colorless or its mana_cost is unknown.
export function cardColorIdentity(card) {
  const out = new Set()
  const cost = String(card?.mana_cost || '')
  if (!cost) return out
  for (const m of cost.matchAll(/\{([^}]+)\}/g)) {
    for (const ch of m[1]) {
      const u = ch.toUpperCase()
      if (u === 'W' || u === 'U' || u === 'B' || u === 'R' || u === 'G') out.add(u)
    }
  }
  return out
}

// Color identity rendered in canonical WUBRG order. Used by the COLOR
// column so multi-color cards sort deterministically (WU vs UW would
// otherwise depend on insertion order).
export function cardColorIdentityString(card) {
  const ids = cardColorIdentity(card)
  return ['W', 'U', 'B', 'R', 'G'].filter(c => ids.has(c)).join('')
}

// SORT_KEYS — the column ids the dense view accepts. Mirrors the
// header buttons; helps tests stay aligned with the rendered UI.
export const SORT_KEYS = ['name', 'cmc', 'type', 'role', 'color', 'qty']

// Stable comparator. Primary key + direction; ties broken by name
// ascending so identical-CMC cards group alphabetically inside their
// bucket regardless of source-array ordering.
export function compareCards(a, b, key, dir, cardRoles) {
  const mult = dir === 'desc' ? -1 : 1
  const tieByName = (a?.name || '').localeCompare(b?.name || '')
  const num = (x, y) => {
    if (x === y) return tieByName
    return (x - y) * mult
  }
  const str = (x, y) => {
    const c = (x || '').localeCompare(y || '') * mult
    return c !== 0 ? c : tieByName
  }
  switch (key) {
    case 'name': return str(a?.name, b?.name)
    case 'cmc':  return num(cardCMCForSort(a), cardCMCForSort(b))
    case 'type': {
      const ax = TYPE_SORT_ORDER[cardTypeBucket(a)] ?? 99
      const bx = TYPE_SORT_ORDER[cardTypeBucket(b)] ?? 99
      const c = (ax - bx) * mult
      return c !== 0 ? c : tieByName
    }
    case 'role': {
      // Unassigned ('') sinks in asc, surfaces in desc — same
      // empty-bucketing rule as color so spreadsheets read
      // consistently ("filled values first" by default).
      const ar = cardRole(a, cardRoles)
      const br = cardRole(b, cardRoles)
      if (ar === '' && br !== '') return mult > 0 ? 1 : -1
      if (br === '' && ar !== '') return mult > 0 ? -1 : 1
      return str(ar, br)
    }
    case 'color': {
      const ac = cardColorIdentityString(a)
      const bc = cardColorIdentityString(b)
      // Colorless ('') sinks in ascending, surfaces in descending —
      // mirrors how spreadsheets bucket empty strings against the
      // current sort direction.
      if (ac === '' && bc !== '') return mult > 0 ? 1 : -1
      if (bc === '' && ac !== '') return mult > 0 ? -1 : 1
      return str(ac, bc)
    }
    case 'qty':  return num(Number(a?.quantity) || 1, Number(b?.quantity) || 1)
    default:     return tieByName
  }
}

// sortCards is a non-mutating wrapper. Always returns a fresh array
// so React's referential-equality checks notice the new order.
export function sortCards(cards, key, dir, cardRoles) {
  if (!Array.isArray(cards)) return []
  if (!SORT_KEYS.includes(key)) return [...cards]
  return [...cards].sort((a, b) => compareCards(a, b, key, dir, cardRoles))
}

// toggleSort: clicking the same column flips direction; clicking a
// new column resets to the column's natural default (ascending for
// strings, descending for numbers — so qty/cmc click goes biggest-
// first the way Tappedout / Moxfield trained the user to expect).
export function toggleSort(current, nextKey) {
  if (current?.key === nextKey) {
    return { key: nextKey, dir: current.dir === 'asc' ? 'desc' : 'asc' }
  }
  const numericKeys = new Set(['cmc', 'qty'])
  return { key: nextKey, dir: numericKeys.has(nextKey) ? 'desc' : 'asc' }
}
