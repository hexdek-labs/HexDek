// Pure helpers powering PhaseRibbon. Lives in a .js file (vs the
// .jsx component file) so node --test can import it directly
// without a JSX transform.

// Phase bucket → display label mapping. Index = bucket index (0..4).
export const PHASE_LABELS = ['BEGIN', 'MAIN 1', 'COMBAT', 'MAIN 2', 'END']

// phaseBucket maps the engine's (phase, step) tuple to one of the
// five visual buckets. Returns -1 for an unknown / between-turn
// state so the ribbon can render in neutral mode.
//
// Engine vocabulary (canonical, from internal/gameengine/):
//   beginning:  untap, upkeep, draw
//   main:       precombat_main, postcombat_main
//   combat:     begin_of_combat (begin/beginning/start variants),
//               declare_attackers, declare_blockers,
//               first_strike_damage, combat_damage,
//               end_of_combat (end_of_combat / combat_end)
//   ending:     end (end/end_step/end_of_turn), cleanup
export function phaseBucket(phase, step) {
  const p = (phase || '').toLowerCase()
  const s = (step || '').toLowerCase()
  if (p === 'beginning') return 0
  if (p === 'main') {
    if (s === 'postcombat_main' || s === 'main2' || s === 'post_combat_main') return 3
    return 1
  }
  if (p === 'combat') return 2
  if (p === 'ending') return 4
  return -1
}

// stepLabel returns the human-readable label for a given step.
// Folds engine variant spellings (e.g. begin_of_combat /
// beginning_of_combat / combat_start) to one canonical label so the
// UI doesn't surface inconsistencies. Falls back to upper-cased raw
// step when the value is unrecognized.
export function stepLabel(step) {
  if (!step) return ''
  const s = step.toLowerCase()
  switch (s) {
    case 'untap': return 'UNTAP'
    case 'upkeep': return 'UPKEEP'
    case 'draw': return 'DRAW'
    case 'precombat_main': return 'PRECOMBAT'
    case 'postcombat_main':
    case 'main2':
    case 'post_combat_main':
      return 'POSTCOMBAT'
    case 'begin_of_combat':
    case 'beginning_of_combat':
    case 'combat_start':
      return 'BEGIN COMBAT'
    case 'declare_attackers': return 'DECLARE ATKS'
    case 'declare_blockers': return 'DECLARE BLKS'
    case 'first_strike_damage': return 'FIRST STRIKE'
    case 'combat_damage': return 'COMBAT DAMAGE'
    case 'end_of_combat':
    case 'combat_end':
      return 'END COMBAT'
    case 'end':
    case 'end_step':
    case 'end_of_turn':
      return 'END STEP'
    case 'cleanup': return 'CLEANUP'
    default: return step.toUpperCase()
  }
}

// nextActiveSeat returns the index of the seat whose turn comes
// after `activeSeat`, skipping any seats marked `lost`. Returns -1
// when no other living seats exist (game effectively over).
// `seats` is the SeatSnapshot array; only the `.lost` field is read.
export function nextActiveSeat(seats, activeSeat) {
  if (!seats || seats.length === 0) return -1
  const n = seats.length
  // Walk forward through the n-1 other seats. We DON'T fall through to
  // activeSeat itself — the caller wants "whose turn comes next", and
  // if every other seat is lost the answer is -1 (game effectively
  // over), not the same seat.
  const base = ((activeSeat % n) + n) % n
  for (let i = 1; i < n; i++) {
    const idx = (base + i) % n
    if (!seats[idx]?.lost) return idx
  }
  return -1
}
