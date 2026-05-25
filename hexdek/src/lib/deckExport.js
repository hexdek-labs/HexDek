// Pure deck-export format builders. Side-effect-free so they can be
// unit-tested without React/DOM/clipboard. The DeckExportModal component
// wraps these with the format picker UI, clipboard, and download blobs.
//
// All builders accept a `cards` array of { name, quantity?, set?,
// collector_number?, cn? } and an optional `commanderName` (or
// comma-separated list of commander names for partners). The commander
// name(s) are separated out of the mainboard and re-emitted in the
// section appropriate for each format.

// Match a card row against a commander name. Strips the `// flip-half`
// suffix on both sides so DFC commanders match by the front face name.
function isCommanderRow(card, commanderNames) {
  if (!card?.name || !Array.isArray(commanderNames) || commanderNames.length === 0) return false
  const front = (n) => String(n || '').split('//')[0].trim().toLowerCase()
  const cardFront = front(card.name)
  for (const cn of commanderNames) {
    const cnFront = front(cn)
    if (!cnFront) continue
    if (cardFront === cnFront) return true
  }
  return false
}

// Normalize the commander input — accepts either a string (possibly
// "Kraum, Ludevic's Opus, Tymna the Weaver" partner pair joined by a
// comma) or an array. Returns a trimmed, non-empty string array.
//
// Heuristic: partner pairs are typically joined with " / " or " // " or
// ", " by deck-list sources. We only split on " / " and " // " because
// commas appear in single commander names ("Atraxa, Praetors' Voice").
// Callers that have a real array should pass it directly to skip the
// ambiguity.
export function parseCommanderNames(input) {
  if (!input) return []
  if (Array.isArray(input)) return input.map(s => String(s).trim()).filter(Boolean)
  const s = String(input).trim()
  if (!s) return []
  // Try the unambiguous partner separators first.
  if (s.includes(' // ')) return s.split(' // ').map(t => t.trim()).filter(Boolean)
  if (s.includes(' / ')) return s.split(' / ').map(t => t.trim()).filter(Boolean)
  return [s]
}

const qty = (c) => Number(c?.quantity) > 0 ? Number(c.quantity) : 1
const fmtPlain = (c) => `${qty(c)} ${c.name}`
const fmtArena = (c) => {
  const set = (c.set || '').toUpperCase()
  const cn = c.collector_number || c.cn || ''
  if (set && cn) return `${qty(c)} ${c.name} (${set}) ${cn}`
  if (set)       return `${qty(c)} ${c.name} (${set})`
  return `${qty(c)} ${c.name}`
}

// MTGO / .dec — mainboard then `Sideboard` section for commander(s).
function buildMtgo(main, commanders) {
  const out = main.map(fmtPlain)
  if (commanders.length) {
    out.push('')
    out.push('Sideboard')
    out.push(...commanders.map(fmtPlain))
  }
  return out.join('\n')
}

// Arena — same as MTGO but with `Commander` section header and the
// (SET) CN suffix where data is available.
function buildArena(main, commanders) {
  const out = main.map(fmtArena)
  if (commanders.length) {
    out.push('')
    out.push('Commander')
    out.push(...commanders.map(fmtArena))
  }
  return out.join('\n')
}

// Moxfield native plain text — mainboard under a `Deck` header,
// commander(s) under `Commander` (singular) or `Commanders` (2+).
// Matches the format Moxfield's own bulk-paste import expects (see
// internal/deckparser/deckparser_test.go TestParseDeckReader_CommanderSectionHeader
// and TestParseDeckReader_MoxfieldCommandersPartner — that's the
// same shape on the read side).
//
// We always emit a `Deck` header so partner decks parse symmetrically.
function buildMoxfield(main, commanders) {
  const out = []
  if (commanders.length) {
    out.push(commanders.length > 1 ? 'Commanders' : 'Commander')
    out.push(...commanders.map(fmtPlain))
    out.push('')
  }
  out.push('Deck')
  out.push(...main.map(fmtPlain))
  return out.join('\n')
}

// Raw — names only, one per line. Commanders appear in their original
// position (no section break) because RAW is the "give me the bare
// list" escape hatch.
function buildRaw(cards) {
  return cards.map(c => c.name).join('\n')
}

// Top-level dispatch. Returns '' for an empty input so callers can
// distinguish "nothing to export" from a populated string.
export function buildDeckExport(format, cards, commanderInput) {
  if (!Array.isArray(cards) || cards.length === 0) return ''
  if (format === 'raw') return buildRaw(cards)

  const commanderNames = parseCommanderNames(commanderInput)
  const commanders = []
  const main = []
  for (const c of cards) {
    if (!c?.name) continue
    if (isCommanderRow(c, commanderNames)) commanders.push(c)
    else main.push(c)
  }

  switch (format) {
    case 'moxfield': return buildMoxfield(main, commanders)
    case 'arena':    return buildArena(main, commanders)
    case 'mtgo':     return buildMtgo(main, commanders)
    default:         return buildMtgo(main, commanders)
  }
}

// File extension for the chosen format. Centralized so the modal +
// download helper agree.
export function exportExtension(format) {
  switch (format) {
    case 'arena':    return '_arena.txt'
    case 'mtgo':     return '.dec'
    case 'moxfield': return '_moxfield.txt'
    case 'raw':      return '.txt'
    default:         return '.txt'
  }
}
