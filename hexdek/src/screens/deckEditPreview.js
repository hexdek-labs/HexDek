// Pure helpers for the Workshop live mana-curve preview. Kept outside
// DeckArchive.jsx so the parsing + curve math can be unit-tested without
// React. DeckArchive.jsx already has parseDeckText for the diff readout;
// we deliberately duplicate the parser here (rather than import it) so
// the preview module stays self-contained and doesn't pull a 2000-line
// screen into the test surface.

// parseDeckTextToCounts parses one card per line into a Map<name, qty>.
// Lines look like "1 Sol Ring", "2 Llanowar Elves", "COMMANDER: Atraxa".
// Mirrors DeckArchive.jsx's parseDeckText — same semantics, separately
// owned so the live-preview shape changes don't bleed into the diff.
export function parseDeckTextToCounts(text) {
  const counts = new Map()
  for (const raw of (text || '').split('\n')) {
    const line = raw.trim()
    if (!line) continue
    const cmdrMatch = line.match(/^COMMANDER:\s*(.+)$/i)
    if (cmdrMatch) {
      const name = cmdrMatch[1].trim()
      counts.set(name, (counts.get(name) || 0) + 1)
      continue
    }
    const m = line.match(/^(\d+)\s+(.+)$/)
    if (m) {
      const name = m[2].trim()
      counts.set(name, (counts.get(name) || 0) + parseInt(m[1], 10))
    }
  }
  return counts
}

// buildCardMeta produces a Map<name, {cmc, type_line}> from a saved
// deck's `cards` array. Names are matched verbatim (the textarea uses
// the same canonical name the backend stores), so a workshop-typed name
// hits this cache iff it matches an existing entry.
export function buildCardMeta(cards) {
  const m = new Map()
  for (const c of cards || []) {
    if (!c || !c.name) continue
    m.set(c.name, {
      cmc: typeof c.cmc === 'number' ? c.cmc : 0,
      type_line: c.type_line || '',
    })
  }
  return m
}

// isLand replicates DeckStatsSummary's land detection so curve preview
// matches what users will see after save. Land beats every other type
// keyword; a zero-CMC card with no mana cost AND no type_line is also
// treated as a land (covers basics whose meta hasn't been fetched yet).
export function isLandMeta(meta) {
  if (!meta) return false
  const t = (meta.type_line || '').toLowerCase()
  if (/\bland\b/.test(t)) return true
  if (!t && (meta.cmc ?? 0) === 0) return true
  return false
}

// computeCurve buckets non-land cards into the 0..6,7+ histogram used by
// DeckStatsSummary. Returns {curve, total, unknown}:
//   - curve: length-8 array, index 7 is "7 or more"
//   - total: total non-land cards counted (excludes unknown names)
//   - unknown: array of names present in counts but missing from
//     cardMeta. The caller uses this to drive lookups; cards on the
//     unknown list are NOT folded into the histogram (would otherwise
//     mis-bucket as 0-CMC lands).
export function computeCurve(counts, cardMeta) {
  const curve = [0, 0, 0, 0, 0, 0, 0, 0]
  let total = 0
  const unknown = []
  for (const [name, qty] of counts) {
    const meta = cardMeta.get(name)
    if (!meta) {
      unknown.push(name)
      continue
    }
    if (isLandMeta(meta)) continue
    const cmc = Math.max(0, Math.min(7, meta.cmc ?? 0))
    curve[cmc] += qty
    total += qty
  }
  return { curve, total, unknown }
}

// curveDelta returns currentCurve[i] - baselineCurve[i] for each bucket.
// Used by the preview header to show "+2 at 3-CMC, -1 at 5-CMC" style
// indicators next to the live histogram.
export function curveDelta(baseline, current) {
  const out = new Array(8).fill(0)
  for (let i = 0; i < 8; i++) {
    out[i] = (current[i] || 0) - (baseline[i] || 0)
  }
  return out
}
