// Pure helpers for the "MY COLLECTION" power-tier dashboard summary
// on /decks. Reuses deckBracketInt from screens/deckFilters.js so the
// summary bucketing matches the BRACKET chip filter (a deck filtered
// in via the "B4" chip must also count toward "B4" in this rollup).

// Use the explicit `.js` extension here so plain `node --test` resolves
// the import — Vite tolerates the extensionless form but the Node ESM
// resolver does not, and the unit suite runs under Node, not Vite.
import { deckBracketInt } from '../screens/deckFilters.js'

// Tier slots we expose. UNKNOWN catches decks where bracket can't be
// resolved (no Freya analysis yet, malformed filename). Kept ordered
// so the histogram render walks left-to-right cleanly.
export const POWER_TIER_KEYS = ['b1', 'b2', 'b3', 'b4', 'b5', 'unknown']

const TIER_FROM_INT = {
  1: 'b1', 2: 'b2', 3: 'b3', 4: 'b4', 5: 'b5',
}

export const POWER_TIER_LABELS = {
  b1: 'B1 · CASUAL',
  b2: 'B2 · FOCUSED',
  b3: 'B3 · OPTIMIZED',
  b4: 'B4 · HIGH POWER',
  b5: 'B5 · cEDH',
  unknown: 'UNKNOWN',
}

// summarizePowerTiers buckets a list of decks (the same DeckSummary
// shape /api/decks returns) into the six tier slots. Returns:
//   {
//     total: 24,                          // total deck count
//     counts:      {b1:1, b2:5, b3:9, b4:8, b5:0, unknown:1},
//     percentages: {b1:4,  b2:21, b3:38, b4:33, b5:0, unknown:4},  // rounded
//     peak: 'b3',                         // tier with the highest count (ties -> higher bracket)
//     peakCount: 9,
//     pricedShare: 1.0,                   // share of decks with a known bracket
//   }
//
// Empty input returns a zeroed structure so the dashboard renders
// gracefully ("no decks yet") without the caller needing a null guard.
export function summarizePowerTiers(decks) {
  const counts = Object.fromEntries(POWER_TIER_KEYS.map(k => [k, 0]))
  if (!Array.isArray(decks) || decks.length === 0) {
    return zeroSummary(counts)
  }
  for (const d of decks) {
    const n = deckBracketInt(d)
    if (n != null && TIER_FROM_INT[n]) {
      counts[TIER_FROM_INT[n]]++
    } else {
      counts.unknown++
    }
  }
  const total = decks.length
  const percentages = {}
  for (const k of POWER_TIER_KEYS) {
    percentages[k] = total > 0 ? Math.round((counts[k] / total) * 100) : 0
  }
  // Pick the peak bracket among b1..b5 (exclude unknown — "your
  // collection peaks at UNKNOWN" is not useful flavor). Ties break in
  // favor of the HIGHER bracket — a 3/4 split reads as "you're a B4
  // collector with a B3 backup," which matches how players talk.
  let peak = null
  let peakCount = -1
  for (const k of ['b1', 'b2', 'b3', 'b4', 'b5']) {
    if (counts[k] >= peakCount) {
      if (counts[k] > peakCount || counts[k] > 0) {
        peak = k
        peakCount = counts[k]
      }
    }
  }
  if (peakCount === 0) peak = null
  const known = total - counts.unknown
  return {
    total,
    counts,
    percentages,
    peak,
    peakCount,
    pricedShare: total > 0 ? known / total : 0,
  }
}

function zeroSummary(counts) {
  const percentages = Object.fromEntries(POWER_TIER_KEYS.map(k => [k, 0]))
  return { total: 0, counts, percentages, peak: null, peakCount: 0, pricedShare: 0 }
}

// topArchetypes returns the top-N archetype labels across the deck
// list, ranked by deck count. Case-insensitive grouping; the label
// returned is the most common casing observed for that archetype
// (avoids one row reading "AGGRO" and another "Aggro" as different
// archetypes). Excludes decks with no archetype label.
export function topArchetypes(decks, n = 5) {
  if (!Array.isArray(decks) || decks.length === 0) return []
  const counts = new Map()
  const casing = new Map()
  for (const d of decks) {
    const raw = String(d?.archetype || d?.analysis?.archetype || '').trim()
    if (!raw) continue
    const key = raw.toLowerCase()
    counts.set(key, (counts.get(key) || 0) + 1)
    if (!casing.has(key)) casing.set(key, raw)
  }
  const rows = [...counts.entries()].map(([key, count]) => ({
    archetype: casing.get(key),
    count,
  }))
  rows.sort((a, b) => {
    if (b.count !== a.count) return b.count - a.count
    return a.archetype.localeCompare(b.archetype)
  })
  return rows.slice(0, n)
}

// peakFlavor returns the short human-readable line shown next to the
// big tier label in the dashboard summary. Empty for the no-decks
// case so the caller can hide the section entirely.
export function peakFlavor(summary) {
  if (!summary || summary.total === 0) return ''
  if (!summary.peak) return 'NO BRACKETS YET — RUN FREYA TO POPULATE'
  const label = POWER_TIER_LABELS[summary.peak] || summary.peak.toUpperCase()
  const pct = summary.percentages[summary.peak] || 0
  return `YOUR COLLECTION PEAKS AT ${label} (${summary.peakCount} DECKS · ${pct}%)`
}
