// Pure helpers for the deck-detail RECENT GAMES panel. Kept side-effect-free
// so they can be unit-tested without DOM/network.
//
// SummaryArchiveRow shape (see internal/db/summary_archive.go):
//   {
//     game_id, finished_at (unix seconds), turns, winner (seat or -1 for draw),
//     winner_name, end_reason,
//     commanders: ordered-by-seat, deck_keys: ordered-by-seat
//   }
//
// "This deck" is identified by deck_key = `${owner}/${id}` — same string
// the server stores in showmatch_game_seat.deck_key (internal/hexapi/handler.go).

// Returns one of:
//   'win'     — this deck's seat won
//   'loss'    — this deck was in the game but a different seat won
//   'draw'    — game ended with no winner (winner < 0)
//   'unknown' — row malformed or this deck not seated
export function outcomeForDeck(row, deckKey) {
  if (!row || !deckKey) return 'unknown'
  const seats = Array.isArray(row.deck_keys) ? row.deck_keys : []
  const seatedAt = seats.indexOf(deckKey)
  if (seatedAt < 0) return 'unknown'
  const winner = Number(row.winner)
  if (!Number.isFinite(winner) || winner < 0) return 'draw'
  if (winner === seatedAt) return 'win'
  return 'loss'
}

// Aggregate wins / losses / draws across a row set.
export function summarizeRecentGames(rows, deckKey) {
  const out = { wins: 0, losses: 0, draws: 0, total: 0 }
  if (!Array.isArray(rows)) return out
  for (const r of rows) {
    const o = outcomeForDeck(r, deckKey)
    if (o === 'unknown') continue
    out.total++
    if (o === 'win') out.wins++
    else if (o === 'loss') out.losses++
    else if (o === 'draw') out.draws++
  }
  return out
}

// Returns the other commanders in the row, in seat order, with this deck's
// own commander filtered out. Used to render "vs Krenko · Tymna · Atraxa".
export function opponentCommanders(row, deckKey) {
  if (!row || !deckKey) return []
  const seats = Array.isArray(row.deck_keys) ? row.deck_keys : []
  const cmdrs = Array.isArray(row.commanders) ? row.commanders : []
  const out = []
  for (let i = 0; i < cmdrs.length; i++) {
    if (seats[i] === deckKey) continue
    if (cmdrs[i]) out.push(cmdrs[i])
  }
  return out
}

// "2h ago", "3d ago", "5w ago", "just now". nowSec is injected for testability.
export function formatRelativeFinished(epochSec, nowSec) {
  const epoch = Number(epochSec)
  if (!Number.isFinite(epoch) || epoch <= 0) return '—'
  const now = Number.isFinite(nowSec) ? nowSec : Math.floor(Date.now() / 1000)
  const diff = now - epoch
  if (diff < 0) return 'just now'      // small clock skew — treat as fresh
  if (diff < 60) return 'just now'
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
  if (diff < 86400 * 7) return `${Math.floor(diff / 86400)}d ago`
  if (diff < 86400 * 30) return `${Math.floor(diff / (86400 * 7))}w ago`
  if (diff < 86400 * 365) return `${Math.floor(diff / (86400 * 30))}mo ago`
  return `${Math.floor(diff / (86400 * 365))}y ago`
}
