// Pure helpers powering useTurnAnnouncer. Lives in a .js file so
// node --test can exercise them without a JSX transform.

// rt — render turn label as "R{round}T{turn}" (matches the
// Spectator's rt() inline helper but exported so the announcer
// stays consistent with the visual log).
export function rt(turn, nSeats = 4) {
  const safeSeats = nSeats > 0 ? nSeats : 4
  return `R${Math.ceil(turn / safeSeats)}T${turn}`
}

// Long-form phase name spoken by the announcer. Maps the engine's
// internal (phase, step) tuple into a readable phrase. Falls back
// to the upper-cased step / phase when unknown so screen-reader
// users still hear *something* identifiable.
//
// Choice rationale: combat substeps are the spectator-relevant
// signal; phase alone is too coarse during combat. Beginning /
// main / end phases collapse to the phase name since the substep
// rarely matters to a non-rules-savvy listener.
export function phasePhrase(phase, step) {
  const p = (phase || '').toLowerCase()
  const s = (step || '').toLowerCase()
  if (p === 'beginning') {
    if (s === 'untap') return 'untap step'
    if (s === 'upkeep') return 'upkeep'
    if (s === 'draw') return 'draw step'
    return 'beginning phase'
  }
  if (p === 'main') {
    if (s === 'postcombat_main' || s === 'main2' || s === 'post_combat_main') {
      return 'postcombat main'
    }
    return 'precombat main'
  }
  if (p === 'combat') {
    if (s === 'begin_of_combat' || s === 'beginning_of_combat' || s === 'combat_start') {
      return 'beginning of combat'
    }
    if (s === 'declare_attackers') return 'declare attackers'
    if (s === 'declare_blockers') return 'declare blockers'
    if (s === 'first_strike_damage') return 'first-strike damage'
    if (s === 'combat_damage') return 'combat damage'
    if (s === 'end_of_combat' || s === 'combat_end') return 'end of combat'
    return 'combat'
  }
  if (p === 'ending') {
    if (s === 'end' || s === 'end_step' || s === 'end_of_turn') return 'end step'
    if (s === 'cleanup') return 'cleanup'
    return 'ending phase'
  }
  // Fall back to whatever we have so SR users still hear a token.
  if (s) return s.replace(/_/g, ' ')
  if (p) return p
  return ''
}

// announcementKey returns a stable string for the (turn, phase,
// step, active_seat) tuple. Used by the hook to detect "this is the
// same announcement as last time" — repeated WS payloads at the
// same step should not re-announce.
export function announcementKey(game) {
  if (!game) return ''
  const t = game.turn ?? 0
  const p = game.phase ?? ''
  const s = game.step ?? ''
  const a = game.active_seat ?? -1
  return `${t}|${p}|${s}|${a}`
}

// buildAnnouncement formats the screen-reader sentence for the
// current turn state. Shape: "{rt}, {commander}'s {phase}.".
// Emits "Game ended" when finished — the post-game state is
// stable and the spectator UI surfaces the winner separately so
// the announcer doesn't need to repeat it.
//
// Returns empty string for invalid input (no announcement → no
// aria-live mutation → no SR speech).
export function buildAnnouncement(game, seats, nSeats = 4) {
  if (!game) return ''
  if (game.finished) {
    return 'Game ended.'
  }
  const phrase = phasePhrase(game.phase, game.step)
  const seat = game.active_seat
  const commander = (Array.isArray(seats) && seat >= 0 && seat < seats.length)
    ? seats[seat]?.commander
    : null
  const turnTag = rt(game.turn ?? 0, nSeats)
  if (commander && phrase) {
    return `${turnTag}, ${commander}'s ${phrase}.`
  }
  if (commander) {
    return `${turnTag}, ${commander}'s turn.`
  }
  if (phrase) {
    return `${turnTag}, ${phrase}.`
  }
  return `${turnTag}.`
}
