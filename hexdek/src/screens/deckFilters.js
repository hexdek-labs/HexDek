// Pure helpers for the Decks-screen filter chip rows. Keeping these out
// of DeckList.jsx so the chip predicates can be reused (and exercised
// without a DOM) by tests, Storybook, or the public-profile deck strip.
//
// All `matches*` predicates take the chip id (string) and the deck
// record, and return true when the deck should be visible under that
// chip's selection. The "all" chip id always matches.

// ── Bracket ───────────────────────────────────────────────────────────────
// Decks expose bracket via `pls` (Freya plays-like) > `wbs` (whole-bracket
// score) > `bracket` (filename-parsed digit). pls/wbs are integers 1–5;
// `bracket` is the single-digit string from the filename ("3") or "?".
// We floor to nearest integer so a 3.5 plays-like still buckets to B3.
export function deckBracketInt(d) {
  const raw = d?.pls ?? d?.wbs ?? d?.bracket
  const n = parseFloat(raw)
  return Number.isFinite(n) ? Math.floor(n) : null
}

export const BRACKET_CHIPS = [
  { id: 'all', label: 'ALL' },
  { id: 'b1', label: 'B1', bracket: 1 },
  { id: 'b2', label: 'B2', bracket: 2 },
  { id: 'b3', label: 'B3', bracket: 3 },
  { id: 'b4', label: 'B4', bracket: 4 },
  { id: 'b5', label: 'B5', bracket: 5 },
]

export function matchesBracketChip(d, chipId) {
  if (!chipId || chipId === 'all') return true
  const chip = BRACKET_CHIPS.find(c => c.id === chipId)
  if (!chip || chip.bracket == null) return true
  const n = deckBracketInt(d)
  return n === chip.bracket
}

// ── Color identity ────────────────────────────────────────────────────────
// `d.color` lands as "MONO-B", "WUB", "WUBRG", or "?" from
// parseDeckFilename / freya analysis. `d.analysis.color_identity` is an
// array of letters when present. We collapse both into a Set of canonical
// WUBRG letters; "?" / empty → empty set (treated as colorless for the
// COLORLESS chip but matches nothing for individual-color chips).
const WUBRG = new Set(['W', 'U', 'B', 'R', 'G'])

export function deckColorIdentity(d) {
  if (!d) return new Set()
  // Prefer structured analysis.color_identity when the backend ships it.
  const ci = d.analysis?.color_identity
  if (Array.isArray(ci) && ci.length) {
    const out = new Set()
    for (const c of ci) {
      const u = String(c).toUpperCase()
      if (WUBRG.has(u)) out.add(u)
    }
    return out
  }
  // Fall back to the filename-derived `color` string. "MONO-X" → {X};
  // any WUBRG letters anywhere else in the string → set.
  const raw = String(d.color || '').toUpperCase()
  if (!raw || raw === '?') return new Set()
  const monoMatch = raw.match(/MONO-?([WUBRG])/)
  if (monoMatch) return new Set([monoMatch[1]])
  const out = new Set()
  for (const ch of raw) if (WUBRG.has(ch)) out.add(ch)
  return out
}

export const COLOR_CHIPS = [
  { id: 'all', label: 'ALL' },
  { id: 'w', label: 'W', color: 'W' },
  { id: 'u', label: 'U', color: 'U' },
  { id: 'b', label: 'B', color: 'B' },
  { id: 'r', label: 'R', color: 'R' },
  { id: 'g', label: 'G', color: 'G' },
  { id: 'colorless', label: 'C' },
  { id: 'multi', label: 'MULTI' },
]

export function matchesColorChip(d, chipId) {
  if (!chipId || chipId === 'all') return true
  const chip = COLOR_CHIPS.find(c => c.id === chipId)
  if (!chip) return true
  const ids = deckColorIdentity(d)
  if (chip.id === 'colorless') return ids.size === 0
  if (chip.id === 'multi') return ids.size >= 2
  if (chip.color) return ids.has(chip.color)
  return true
}

// ── Archetype ─────────────────────────────────────────────────────────────
// Mirrors the chip list DeckList.jsx already used (kept here so the three
// filter axes live in one module). `match` runs against a lowercased
// archetype string; "all" always matches.
export const ARCHETYPE_CHIPS = [
  { id: 'all', label: 'ALL', match: () => true },
  { id: 'aggro', label: 'AGGRO', match: (a) => a.includes('aggro') },
  { id: 'combo', label: 'COMBO', match: (a) => a.includes('combo') || a.includes('storm') },
  { id: 'control', label: 'CONTROL', match: (a) => a.includes('control') },
  { id: 'tokens', label: 'TOKENS', match: (a) => a.includes('token') || a.includes('go wide') || a.includes('aristocrat') },
  { id: 'voltron', label: 'VOLTRON', match: (a) => a.includes('voltron') },
  { id: 'midrange', label: 'MIDRANGE', match: (a) => a.includes('midrange') },
  { id: 'lands', label: 'LANDS', match: (a) => a.includes('lands') },
  { id: 'tribal', label: 'TRIBAL', match: (a) => a.includes('tribal') },
  { id: 'stax', label: 'STAX', match: (a) => a.includes('stax') },
  { id: 'turns', label: 'TURNS', match: (a) => a.includes('turns') },
]

export function matchesArchetypeChip(d, chipId) {
  if (!chipId || chipId === 'all') return true
  const chip = ARCHETYPE_CHIPS.find(c => c.id === chipId)
  if (!chip) return true
  const a = String(d?.archetype || d?.analysis?.archetype || '').toLowerCase()
  if (!a) return false
  return chip.match(a)
}
