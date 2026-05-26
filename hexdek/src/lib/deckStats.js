// Pure helpers for the deck-detail "AT A GLANCE" stats panel.
// Kept side-effect-free so they're unit-testable without React/DOM.

// Recent gauntlet winrate from the last N elo-history runs, weighted by games.
// Returns { pct, runs, games } or null when no usable history exists.
// The all-time win rate on the elo record covers every game ever played
// against this deck; this helper instead captures "how is it doing lately"
// — the most recent five gauntlets are the signal that responds to deck
// tuning while the all-time number drifts slowly.
export function recentGauntletWinrate(eloHistory, n = 5) {
  if (!Array.isArray(eloHistory) || eloHistory.length === 0) return null
  // eloHistory is oldest-first (see PR #79). Take the last n.
  const tail = eloHistory.slice(-n)
  let wins = 0
  let games = 0
  for (const r of tail) {
    if (!r) continue
    const g = Number(r.games) || 0
    const wr = Number(r.win_rate)
    if (g <= 0 || !Number.isFinite(wr)) continue
    games += g
    wins += g * (wr / 100)
  }
  if (games <= 0) return null
  return {
    pct: Math.round((wins / games) * 1000) / 10, // one decimal, e.g. 27.4
    runs: tail.length,
    games,
  }
}

// Top N win-condition card names. Prefers `finisher_cards`, falls back to
// pulling `card` fields from structured `win_lines`. Deduplicates while
// preserving order so the most authoritative source wins on collisions.
export function topWinConditions(analysis, n = 3) {
  if (!analysis) return []
  const out = []
  const seen = new Set()
  const push = (name) => {
    if (!name) return
    const key = String(name).toLowerCase()
    if (seen.has(key)) return
    seen.add(key)
    out.push(String(name))
  }
  const finishers = Array.isArray(analysis.finisher_cards) ? analysis.finisher_cards : []
  for (const f of finishers) push(typeof f === 'string' ? f : f?.name)
  if (out.length < n) {
    const lines = Array.isArray(analysis.win_lines) ? analysis.win_lines : []
    for (const l of lines) {
      if (l && typeof l === 'object') push(l.card || l.name)
      else push(l)
    }
  }
  return out.slice(0, n)
}

// One-shot summary used by the AT A GLANCE panel. Returns a stable shape
// regardless of which inputs are present so the view layer doesn't need
// to repeat the null-coalescing dance.
export function deckGlanceStats({ deck, analysis, deckElo, eloHistory } = {}) {
  const wbs = analysis?.bracket || deck?.bracket || null
  const pls = analysis?.plays_like || null
  const bracketLabel = analysis?.bracket_label || ''
  // measured_bracket is Freya's signal-computed value (renamed from
  // the legacy `mechanical_bracket`). The declared `bracket` field is
  // the rubber-stamp identity (B2 for wizards/ precons; user-editable);
  // measured diverges when the deck plays hotter than its stamp.
  const measuredBracket = analysis?.measured_bracket || null
  const measuredBracketLabel = analysis?.measured_bracket_label || ''
  const bracketDiverges = measuredBracket != null && wbs != null && Number(measuredBracket) !== Number(wbs)
  const archetypeRaw = analysis?.archetype || ''
  const archetype = archetypeRaw ? archetypeRaw.toUpperCase() : ''
  const winConditions = topWinConditions(analysis, 3)
  const recent = recentGauntletWinrate(eloHistory, 5)
  const allTime = (deckElo && deckElo.win_rate != null) ? {
    pct: Number(deckElo.win_rate),
    games: Number(deckElo.games) || 0,
  } : null
  return {
    bracket: wbs,
    playsLike: pls && pls !== wbs ? pls : null,
    bracketLabel,
    measuredBracket,
    measuredBracketLabel,
    bracketDiverges,
    archetype,
    winConditions,
    recent,
    allTime,
  }
}
