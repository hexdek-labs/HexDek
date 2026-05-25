// Pure diffing primitives for the DECK HISTORY view.
//
// The Workshop SAVE UPDATE diff readout has always parsed two textareas
// into card-count maps and produced +N / -M. The history view needs a
// richer breakdown: a card going from qty 4 → 2 is neither a pure add
// nor a pure remove — it's a "changed qty" event the user usually
// wants to see explicitly. This module replaces the inline helpers
// (parseDeckText + computeDeckDiff in DeckArchive.jsx) with a tested,
// reusable surface used by both Workshop and history views.

// parseDeckTextCounts is the canonical "decklist text → Map<name,qty>"
// parser. Treats `COMMANDER: X` as qty-1 of X so a commander swap
// renders as -OLD / +NEW in the diff.
export function parseDeckTextCounts(text) {
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

// diffCounts categorises every name that appears in either side into:
//   - added:   not present in baseline, qty > 0 in current
//   - removed: present in baseline, absent (or 0) in current
//   - changed: present in both but qty differs
//
// All three lists are sorted by card name. Each entry carries the
// from/to qty so callers can render "Sol Ring: 1 → 0" or "Forest: 28
// → 32" without re-deriving. Returns counts of cards (not unique
// names) for the totals at the top.
export function diffCounts(baseline, current) {
  const a = baseline instanceof Map ? baseline : new Map(Object.entries(baseline || {}))
  const b = current instanceof Map ? current : new Map(Object.entries(current || {}))
  const names = new Set([...a.keys(), ...b.keys()])
  const added = []
  const removed = []
  const changed = []
  let addedCards = 0
  let removedCards = 0
  let netCards = 0
  for (const name of names) {
    const before = a.get(name) || 0
    const after = b.get(name) || 0
    if (before === after) continue
    netCards += after - before
    if (before === 0) {
      added.push({ name, from: 0, to: after, delta: after })
      addedCards += after
    } else if (after === 0) {
      removed.push({ name, from: before, to: 0, delta: before })
      removedCards += before
    } else {
      changed.push({ name, from: before, to: after, delta: after - before })
      if (after > before) addedCards += after - before
      else removedCards += before - after
    }
  }
  const byName = (x, y) => x.name.localeCompare(y.name)
  added.sort(byName)
  removed.sort(byName)
  changed.sort(byName)
  return {
    added,
    removed,
    changed,
    addedCards,
    removedCards,
    netCards,
    isClean: added.length === 0 && removed.length === 0 && changed.length === 0,
  }
}

// diffDeckText is the convenience wrapper Workshop already used — feed
// two raw textareas, get a structural diff. Equivalent to
// diffCounts(parseDeckTextCounts(a), parseDeckTextCounts(b)).
export function diffDeckText(baselineText, currentText) {
  return diffCounts(parseDeckTextCounts(baselineText), parseDeckTextCounts(currentText))
}

// Compact one-line summary, e.g. "+3 / -2 / Δ1 changed (net +1)".
// Used by the history timeline's row chrome where the full per-card
// list is collapsed by default.
export function diffSummary(diff) {
  if (!diff || diff.isClean) return 'NO CHANGES'
  const parts = []
  if (diff.added.length) parts.push(`+${diff.added.length}`)
  if (diff.removed.length) parts.push(`-${diff.removed.length}`)
  if (diff.changed.length) parts.push(`Δ${diff.changed.length}`)
  const net = diff.netCards
  if (net !== 0) parts.push(`(net ${net > 0 ? '+' : ''}${net})`)
  return parts.join(' ')
}
