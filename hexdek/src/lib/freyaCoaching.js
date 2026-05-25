// Pure helpers for surfacing Freya's coaching output inline against
// individual card rows. Today this covers cut suggestions; the index
// shape is extensible so future coaching kinds (suggested swaps,
// upgrade picks, role tags) can drop in without touching call sites.
//
// The existing CONSIDER CUTTING panel (RationalePanels.jsx) renders
// the same data as a standalone block — this helper makes the same
// rationale callable from any per-card UI: list rows, grid tiles,
// hover popovers. The structured fields below mirror what Freya
// emits in `analysis.cuttable_card_rationale`:
//
//   { name, reason, detected, why_cut, effect, suggested:[] }
//
// Legacy `analysis.cuttable_cards` is a flat array of names (no
// rationale) — we accept it too so older strategy.json files on disk
// still surface a marker (just with the reason/detail fields blank).

// Normalize a card name to its match key. Strip the optional "COMMANDER:"
// prefix that some deck-list shapes carry, and lowercase the front face
// only (so DFC commanders match by either face). Internal use only — the
// canonical display name flows untouched through the rest of the UI.
export function canonicalCardKey(name) {
  if (!name) return ''
  let s = String(name).replace(/^COMMANDER:\s*/i, '').trim()
  // DFC cards arrive as "Front // Back"; index by front face since that's
  // what the cuttable list and the deck-list rows agree on.
  const slash = s.indexOf(' // ')
  if (slash > 0) s = s.slice(0, slash)
  return s.toLowerCase()
}

// CoachingEntry: { kind, name, reason, detected, whyCut, effect, suggested }
// Today `kind` is always 'cut'; the field exists so a future PR can mix
// 'cut' / 'upgrade' / 'role' entries against the same lookup without a
// shape change.
//
// `suggested` is always an array (possibly empty). All string fields
// default to '' rather than undefined so callers don't have to guard.

function normalizeCutEntry(raw) {
  if (typeof raw === 'string') {
    return {
      kind: 'cut',
      name: raw,
      reason: '',
      detected: '',
      whyCut: '',
      effect: '',
      suggested: [],
    }
  }
  if (raw && typeof raw === 'object' && raw.name) {
    return {
      kind: 'cut',
      name: String(raw.name),
      reason: raw.reason ? String(raw.reason) : '',
      detected: raw.detected ? String(raw.detected) : '',
      whyCut: raw.why_cut ? String(raw.why_cut) : '',
      effect: raw.effect ? String(raw.effect) : '',
      suggested: Array.isArray(raw.suggested) ? raw.suggested.map(String) : [],
    }
  }
  return null
}

// Returns a Map keyed by canonical card name → { cut?: CoachingEntry }.
// Future kinds attach as siblings (e.g. `entry.upgrade`, `entry.role`).
// Map is empty when analysis is missing or has no coaching data.
export function buildCoachingIndex(analysis) {
  const out = new Map()
  if (!analysis || typeof analysis !== 'object') return out

  // Prefer the structured rationale list; fall back to the flat names
  // list. Both feed into the same `kind: 'cut'` slot — the legacy list
  // just produces sparser entries.
  const cuts = Array.isArray(analysis.cuttable_card_rationale)
    ? analysis.cuttable_card_rationale
    : (Array.isArray(analysis.cuttable_cards) ? analysis.cuttable_cards : [])
  for (const raw of cuts) {
    const entry = normalizeCutEntry(raw)
    if (!entry) continue
    const key = canonicalCardKey(entry.name)
    if (!key) continue
    const existing = out.get(key) || {}
    existing.cut = entry
    out.set(key, existing)
  }
  return out
}

// Look up coaching for a specific card by name. Returns null when there's
// no entry — callers can short-circuit rendering.
export function coachingForCard(index, cardName) {
  if (!index || !cardName) return null
  const key = canonicalCardKey(cardName)
  if (!key) return null
  return index.get(key) || null
}

// Convenience: did Freya flag this card for cutting? Returns boolean so
// the inline marker can decide whether to render at all.
export function isCardFlaggedForCut(index, cardName) {
  const entry = coachingForCard(index, cardName)
  return !!(entry && entry.cut)
}

// Returns a flat array of CoachingEntry for the given kind, in the order
// they appeared in the source analysis. Used by summary panels that want
// to show "N cards flagged" totals without re-walking the analysis.
export function coachingEntriesByKind(index, kind) {
  if (!index || !kind) return []
  const out = []
  for (const v of index.values()) {
    if (v[kind]) out.push(v[kind])
  }
  return out
}
