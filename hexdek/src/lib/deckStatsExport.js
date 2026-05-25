// Pure builders for the "EXPORT DECK STATS" flow on the deck page.
// Side-effect-free so the JSON and CSV layouts can be unit-tested
// without React/DOM/clipboard. DeckExportModal supplies the input
// bundle (deck, Freya analysis, gauntlet result, ELO history, recent
// games) and renders the returned string in the preview pane.
//
// Stable field naming matters: third-party spreadsheets and the
// HexDek API consumers downstream of this export key into the exact
// strings emitted here. Don't rename without a migration.

const EXPORT_SCHEMA_VERSION = 1

// extractStats normalizes the heterogeneous bundle into a single
// flat shape so JSON + CSV builders agree on what's available and
// what's null. Null sections survive into the JSON output as `null`
// rather than being dropped — downstream consumers can distinguish
// "not run yet" (null) from "run but empty" ({}).
export function extractStats(bundle) {
  const { deck = {}, analysis = null, gauntlet = null, eloHistory = null, games = null } = bundle || {}

  const cardCount = Array.isArray(deck.cards)
    ? deck.cards.reduce((n, c) => n + (Number(c.quantity) > 0 ? Number(c.quantity) : 1), 0)
    : (deck.card_count ?? null)

  return {
    schema_version: EXPORT_SCHEMA_VERSION,
    exported_at: new Date().toISOString(),
    deck: {
      id: deck.id || '',
      owner: deck.owner || '',
      name: deck.name || deck.commander_card || deck.commander || '',
      commander: deck.commander_card || deck.commander || '',
      card_count: cardCount,
      color_identity: Array.isArray(deck.color_identity) ? deck.color_identity.join('') : (deck.color || ''),
      bracket: deck.wbs ?? deck.pls ?? deck.bracket ?? null,
      legal: typeof deck.legal === 'boolean' ? deck.legal : null,
    },
    analysis: analysis ? {
      archetype: analysis.archetype || '',
      bracket: analysis.bracket ?? null,
      plays_like: analysis.plays_like ?? null,
      game_changer_count: analysis.game_changer_count ?? null,
      power_percentile: analysis.power_percentile ?? null,
      keepable_hand_pct: analysis.keepable_hand_pct ?? null,
      mana_base_grade: analysis.mana_base_grade ?? null,
      commander_synergy: analysis.commander_synergy ?? null,
      eval_weights: analysis.eval_weights || null,
      win_lines: Array.isArray(analysis.win_lines?.win_lines) ? analysis.win_lines.win_lines : (analysis.win_lines || null),
    } : null,
    gauntlet: gauntlet ? {
      games: gauntlet.games ?? gauntlet.total_games ?? null,
      wins: gauntlet.wins ?? null,
      losses: gauntlet.losses ?? null,
      win_rate: gauntlet.win_rate ?? null,
      elo_start: gauntlet.elo_start ?? null,
      elo_end: gauntlet.elo_end ?? null,
      placements: gauntlet.placements || null,
      finished_at: gauntlet.finished_at ?? gauntlet.completed_at ?? null,
    } : null,
    elo_history: Array.isArray(eloHistory)
      ? eloHistory.map(h => ({
          finished_at: h.finished_at ?? h.completed_at ?? null,
          elo_start: h.elo_start ?? null,
          elo_end: h.elo_end ?? null,
          win_rate: h.win_rate ?? null,
          games: h.games ?? null,
        }))
      : null,
    recent_games: Array.isArray(games)
      ? games.map(g => ({
          id: g.id || g.game_id || '',
          played_at: g.played_at ?? g.started_at ?? g.finished_at ?? null,
          winner_seat: g.winner_seat ?? null,
          turns: g.turns ?? null,
          duration_seconds: g.duration_seconds ?? g.duration ?? null,
          placement: g.placement ?? null,
          opponents: Array.isArray(g.opponents)
            ? g.opponents.map(o => o?.commander || o?.deck || '').filter(Boolean)
            : (Array.isArray(g.commanders) ? g.commanders.filter(Boolean) : []),
        }))
      : null,
  }
}

// buildStatsJSON — pretty-printed JSON for human reading + tooling.
// Returns a string ready to write to disk / copy to clipboard.
export function buildStatsJSON(bundle) {
  return JSON.stringify(extractStats(bundle), null, 2)
}

// CSV cell escaper. RFC 4180 — quote when the value contains a
// comma, newline, or double-quote; double-up embedded quotes.
function csvCell(value) {
  if (value === null || value === undefined) return ''
  let s = String(value)
  if (s === '') return ''
  if (/[",\n\r]/.test(s)) {
    s = '"' + s.replace(/"/g, '""') + '"'
  }
  return s
}

function csvRow(values) {
  return values.map(csvCell).join(',')
}

// buildStatsCSV — multi-section CSV. Each section emits a blank-line
// separator + section header + its own column row so spreadsheets
// can split on the headers. The OVERVIEW section is intentionally
// always present (with empty values when data is missing) so a
// minimal export still parses cleanly.
export function buildStatsCSV(bundle) {
  const stats = extractStats(bundle)
  const rows = []

  rows.push('# section,key,value')

  // ── Overview (deck + analysis summary) ─────────────────────────
  rows.push('')
  rows.push('# overview')
  rows.push(csvRow(['overview', 'deck_id', stats.deck.id]))
  rows.push(csvRow(['overview', 'owner', stats.deck.owner]))
  rows.push(csvRow(['overview', 'name', stats.deck.name]))
  rows.push(csvRow(['overview', 'commander', stats.deck.commander]))
  rows.push(csvRow(['overview', 'color_identity', stats.deck.color_identity]))
  rows.push(csvRow(['overview', 'card_count', stats.deck.card_count]))
  rows.push(csvRow(['overview', 'bracket', stats.deck.bracket]))
  rows.push(csvRow(['overview', 'legal', stats.deck.legal]))
  if (stats.analysis) {
    rows.push(csvRow(['overview', 'archetype', stats.analysis.archetype]))
    rows.push(csvRow(['overview', 'plays_like', stats.analysis.plays_like]))
    rows.push(csvRow(['overview', 'game_changer_count', stats.analysis.game_changer_count]))
    rows.push(csvRow(['overview', 'power_percentile', stats.analysis.power_percentile]))
    rows.push(csvRow(['overview', 'keepable_hand_pct', stats.analysis.keepable_hand_pct]))
    rows.push(csvRow(['overview', 'mana_base_grade', stats.analysis.mana_base_grade]))
    rows.push(csvRow(['overview', 'commander_synergy', stats.analysis.commander_synergy]))
  }

  // ── Eval weights (one row per dimension) ───────────────────────
  if (stats.analysis?.eval_weights && typeof stats.analysis.eval_weights === 'object') {
    rows.push('')
    rows.push('# eval_weights')
    rows.push(csvRow(['eval_weights', 'dimension', 'weight']))
    for (const [k, v] of Object.entries(stats.analysis.eval_weights)) {
      rows.push(csvRow(['eval_weights', k, v]))
    }
  }

  // ── Gauntlet (most recent run) ─────────────────────────────────
  if (stats.gauntlet) {
    rows.push('')
    rows.push('# gauntlet')
    rows.push(csvRow(['gauntlet', 'games', stats.gauntlet.games]))
    rows.push(csvRow(['gauntlet', 'wins', stats.gauntlet.wins]))
    rows.push(csvRow(['gauntlet', 'losses', stats.gauntlet.losses]))
    rows.push(csvRow(['gauntlet', 'win_rate', stats.gauntlet.win_rate]))
    rows.push(csvRow(['gauntlet', 'elo_start', stats.gauntlet.elo_start]))
    rows.push(csvRow(['gauntlet', 'elo_end', stats.gauntlet.elo_end]))
    rows.push(csvRow(['gauntlet', 'finished_at', stats.gauntlet.finished_at]))
  }

  // ── ELO history (one row per completed gauntlet run) ───────────
  if (Array.isArray(stats.elo_history) && stats.elo_history.length > 0) {
    rows.push('')
    rows.push('# elo_history')
    rows.push(csvRow(['elo_history', 'finished_at', 'games', 'elo_start', 'elo_end', 'win_rate']))
    for (const h of stats.elo_history) {
      rows.push(csvRow(['elo_history', h.finished_at, h.games, h.elo_start, h.elo_end, h.win_rate]))
    }
  }

  // ── Recent games (one row per game) ────────────────────────────
  if (Array.isArray(stats.recent_games) && stats.recent_games.length > 0) {
    rows.push('')
    rows.push('# recent_games')
    rows.push(csvRow(['recent_games', 'id', 'played_at', 'placement', 'winner_seat', 'turns', 'duration_seconds', 'opponents']))
    for (const g of stats.recent_games) {
      rows.push(csvRow([
        'recent_games', g.id, g.played_at, g.placement, g.winner_seat, g.turns, g.duration_seconds,
        // Opponents joined with " | " — survives a CSV cell without
        // needing quoting unless a commander name has a comma (then
        // csvCell quotes the whole cell).
        (g.opponents || []).join(' | '),
      ]))
    }
  }

  return rows.join('\n')
}

// Filename suggestion. Deck id sanitized to a-z0-9_- (DeckExportModal
// uses the same convention for decklist exports).
export function statsExportFilename(deckId, format) {
  const base = (deckId || 'deck').replace(/[^a-z0-9_-]/gi, '_')
  return `${base}_stats.${format === 'csv' ? 'csv' : 'json'}`
}
