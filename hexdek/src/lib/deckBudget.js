// Pure helpers for the DECK BUDGET panel on the deck page.
// Side-effect-free so the currency formatter + summary derivation can
// be unit-tested without React. The backend endpoint
// (GET /api/decks/{owner}/{id}/budget) already ships its own
// `top_expensive` and totals — these helpers normalise the response,
// format prices for display, and re-derive top-N if a downstream
// caller wants to slice differently.

// formatUSD — locale-independent display formatter so server-side
// pretty-print (`$12.34`) matches client-side regardless of the
// user's browser locale. Negative numbers stay signed; sub-cent
// amounts round to 2 decimals (matches the backend's round2).
export function formatUSD(amount, { withCents = true, currency = 'USD' } = {}) {
  if (amount == null || !Number.isFinite(Number(amount))) return '—'
  const n = Number(amount)
  const symbol = currency === 'USD' ? '$' : ''
  const abs = Math.abs(n)
  const body = withCents ? abs.toFixed(2) : Math.round(abs).toString()
  const withCommas = body.replace(/\B(?=(\d{3})+(?!\d))/g, ',')
  return (n < 0 ? '-' : '') + symbol + withCommas
}

// formatUSDShort — compact rendering for tight chrome. $1289.99 →
// "$1.29K", $25.00 → "$25" (no cents). Falls back to formatUSD for
// values under $1000.
export function formatUSDShort(amount) {
  if (amount == null || !Number.isFinite(Number(amount))) return '—'
  const n = Number(amount)
  if (Math.abs(n) >= 1000) {
    const k = n / 1000
    return `$${k.toFixed(k >= 10 ? 1 : 2)}K`
  }
  return formatUSD(n, { withCents: n < 10 })
}

// budgetTier — human-readable bracket label for the headline number.
// The cutoffs roughly mirror community budget tiers (e.g. /r/EDH's
// "budget" / "moderate" / "high-power" rule of thumb). Used in the
// BUDGET panel header strip; tests pin the boundaries so a future
// retune is intentional.
export function budgetTier(total) {
  const n = Number(total) || 0
  if (n <= 0)    return 'UNPRICED'
  if (n <  50)   return 'ULTRA BUDGET'
  if (n <  150)  return 'BUDGET'
  if (n <  400)  return 'MODERATE'
  if (n <  1000) return 'PREMIUM'
  return 'CHASE'
}

// summarize folds the backend response into the panel's display
// shape: title metric + secondary metrics + a curated top-N list.
// Defends against missing fields so a partial response (e.g.
// pre-prices-migration cache where total=0 / missing_count>0) still
// renders cleanly.
export function summarize(payload, { topN = 5 } = {}) {
  const safe = payload || {}
  const cards = Array.isArray(safe.cards) ? safe.cards : []
  const supplied = Array.isArray(safe.top_expensive) ? safe.top_expensive : null
  // Prefer the backend's pre-sorted top_expensive (already filters out
  // basics + unpriced rows). Re-derive only when missing or topN is
  // larger than what the server shipped.
  let top = supplied && supplied.length >= topN ? supplied.slice(0, topN) : null
  if (!top) {
    const priced = cards
      .filter(c => c?.priced && Number(c.line) > 0)
      .sort((a, b) => (Number(b.line) || 0) - (Number(a.line) || 0))
    top = priced.slice(0, topN)
  }
  const total = Number(safe.total) || 0
  const cardCount = Number(safe.card_count) || 0
  const uniqueCount = Number(safe.unique_count) || 0
  const pricedCount = Number(safe.priced_count) || 0
  const missingCount = Number(safe.missing_count) || 0
  return {
    total,
    cardCount,
    uniqueCount,
    pricedCount,
    missingCount,
    coverage: uniqueCount > 0 ? pricedCount / uniqueCount : 0,
    avgPerCard: cardCount > 0 ? total / cardCount : 0,
    topExpensive: top,
    tier: budgetTier(total),
    currency: safe.currency || 'USD',
  }
}
