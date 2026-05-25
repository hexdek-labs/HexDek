// Plain-decklist parser. Used by:
//   - components/ImportModal.jsx (full multi-step import flow)
//   - screens/DeckList.jsx       (inline QUICK PASTE box on /decks)
//
// Lives in lib/ so the pasted-text formats we accept (Moxfield, Archidekt,
// Tappedout, MTGGoldfish, raw `N CardName`) can be exercised by node:test
// without spinning up React. Keep this file pure — no React, no fetch.

export const BASIC_LANDS = new Set([
  'plains', 'island', 'swamp', 'mountain', 'forest',
  'snow-covered plains', 'snow-covered island', 'snow-covered swamp',
  'snow-covered mountain', 'snow-covered forest',
  'wastes',
])

// Section headers that mean "everything after this line is NOT in the
// 99 (or 100 with commander)". Once we hit any of these, parseDeckLines
// stops consuming card rows until it encounters a re-enter header
// (Deck/Mainboard/Commander) — covers Moxfield exports that pack
// sideboard / maybeboard / companion rows after the main list.
const STOP_HEADERS = /^(sideboard|maybeboard|companion|considering|tokens?|extra)\b/i
const RESUME_HEADERS = /^(deck|mainboard|commander|main)\b\s*[:]?\s*$/i

// inferCommander pulls the first "Commander: <name>" line out of a
// pasted decklist. Moxfield + Archidekt both emit this prefix; we also
// match a bare "COMMANDER" / "Commander (1)" header line — in that
// case the commander is the next non-empty card row.
export function inferCommander(text) {
  if (!text) return ''
  const lines = text.split('\n')
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i].trim()
    if (/^commander\s*:/i.test(line)) {
      return line.replace(/^commander\s*:/i, '').trim()
    }
    if (/^commander\s*(\(\d+\))?\s*$/i.test(line)) {
      // Header line — find the next card row.
      for (let j = i + 1; j < lines.length; j++) {
        const next = lines[j].trim()
        if (!next) continue
        const parsed = parseCardLine(next)
        if (parsed) return parsed.name
        return next
      }
    }
  }
  return ''
}

// parseCardLine — single-line parser. Returns {name, quantity, set, cn}
// or null when the line is unrecognized. Exposed for tests + UI hints
// ("we parsed N cards"). Accepted forms (whitespace flexible):
//   1 Sol Ring
//   1x Sol Ring
//   2 Sol Ring (CMM) 339
//   1 Sol Ring (CMM) 339 *F*
//   Sol Ring            (bare name, qty defaults to 1)
//   SB: 1 Sol Ring      (sideboard prefix stripped here too, caller decides)
export function parseCardLine(raw) {
  if (!raw) return null
  let line = raw.trim()
  if (!line) return null

  // Strip leading "SB: " / "MB: " markers; the section gating happens
  // upstream in parseDeckLines.
  line = line.replace(/^(SB|MB|CMD|CMDR|MAIN):\s*/i, '')

  // Strip trailing foil/etched/promo annotations.
  line = line.replace(/\s+\*[\w/]+\*\s*$/i, '')

  // "N CardName ..." or "Nx CardName ..."
  const qtyMatch = line.match(/^(\d+)x?\s+(.+)$/i)
  if (qtyMatch) {
    const qty = parseInt(qtyMatch[1], 10)
    const rest = qtyMatch[2].trim()
    if (qty <= 0) return null
    const { name, set, cn } = stripSetCollector(rest)
    if (!name) return null
    return { name, quantity: qty, set, cn }
  }

  // Bare name — default quantity 1.
  const { name, set, cn } = stripSetCollector(line)
  if (!name) return null
  return { name, quantity: 1, set, cn }
}

// stripSetCollector pulls trailing "(SET) 123" or "(SET)" off a card
// name and returns the canonical name + the optional set/collector
// metadata. Tolerant of either part being absent.
function stripSetCollector(s) {
  const m = s.match(/^(.+?)\s*\(([^)]+)\)(?:\s+(\S+))?\s*$/)
  if (m) return { name: m[1].trim(), set: m[2].trim(), cn: m[3]?.trim() || '' }
  // Trailing bare collector number with no set wrapper — uncommon but
  // some exports do this; strip it so the name matches the oracle.
  const trailing = s.match(/^(.+?)\s+\d{1,4}\s*$/)
  if (trailing) return { name: trailing[1].trim(), set: '', cn: '' }
  return { name: s.trim(), set: '', cn: '' }
}

// parseDeckLines — multi-line parser. Returns [{name, quantity,
// isCommander, set, cn}, ...]. Skips:
//   - blank lines, comment lines (#, //)
//   - any line inside a sideboard/maybeboard/companion section until a
//     re-enter header (Deck/Mainboard/Commander) is seen
//   - bare section headers that don't carry a card payload
//
// Commander detection: explicit "Commander: <name>" lines mark
// isCommander=true. Bare "Commander" header lines tag the FIRST card
// row inside that section as commander, then re-enter mainboard.
export function parseDeckLines(text) {
  if (!text) return []
  const out = []
  let inSideboard = false
  let pendingCommanderTag = false

  for (const raw of text.split('\n')) {
    const line = raw.trim()
    if (!line) {
      // Blank line in a Moxfield export usually ends the current
      // section block — drop the pending commander tag so a blank after
      // the commander header doesn't tag a random later card.
      pendingCommanderTag = false
      continue
    }
    if (line.startsWith('#') || line.startsWith('//')) continue

    if (STOP_HEADERS.test(line)) {
      inSideboard = true
      pendingCommanderTag = false
      continue
    }
    if (RESUME_HEADERS.test(line)) {
      inSideboard = false
      if (/^commander\b/i.test(line) && !/:/.test(line)) {
        pendingCommanderTag = true
      }
      continue
    }
    if (inSideboard) continue

    if (/^commander\s*:/i.test(line)) {
      const name = line.replace(/^commander\s*:/i, '').trim()
      if (name) out.push({ name, quantity: 1, isCommander: true, set: '', cn: '' })
      continue
    }

    const parsed = parseCardLine(line)
    if (!parsed) continue

    out.push({
      ...parsed,
      isCommander: pendingCommanderTag,
    })
    pendingCommanderTag = false
  }
  return out
}

// summarize collapses a parsed list into {cardCount, uniqueCount,
// commander, hasIllegalMultiples}. Useful for the QUICK PASTE preview
// strip on /decks and the import modal's pre-submit summary.
export function summarize(parsedCards) {
  if (!parsedCards || parsedCards.length === 0) {
    return { cardCount: 0, uniqueCount: 0, commander: '', hasIllegalMultiples: false }
  }
  let cardCount = 0
  let commander = ''
  let hasIllegalMultiples = false
  for (const c of parsedCards) {
    cardCount += c.quantity
    if (c.isCommander && !commander) commander = c.name
    if (!c.isCommander && c.quantity > 1 && !BASIC_LANDS.has(c.name.toLowerCase())) {
      hasIllegalMultiples = true
    }
  }
  return {
    cardCount,
    uniqueCount: parsedCards.length,
    commander,
    hasIllegalMultiples,
  }
}
