package gameengine

// keywords_speed_counter.go — Aetherdrift Speed counter system
// implementing CR §702.178 (Max Speed) and the §702.179 (Start Your
// Engines!) advancement rules.
//
// Model:
//
//   - Every seat carries a Speed int in [0..MaxSpeedCap]. Persistent
//     across turns. Stored on Seat.Speed (typed, not the legacy
//     Flags["speed"] entry — see §704.5z initializer in sba.go which
//     also writes Seat.Speed).
//
//   - Speed advances by exactly 1 when a damage event satisfies:
//
//       * source is controlled by a player whose Speed < MaxSpeedCap
//       * the damage lands on a player (not a creature/planeswalker)
//       * the source-controller seat has not already advanced its
//         speed this turn (Turn.SpeedAdvancedThisTurn==false)
//
//     Per the printed rules, both combat damage and spell/ability
//     damage qualify. The once-per-turn gate applies *per controller*,
//     so two seats can each tick +1 on the same turn from a single
//     symmetric damage source.
//
//   - MaxSpeedActive(gs, seat) returns true when Seat.Speed ==
//     MaxSpeedCap. Cards with the "max speed" rider gate their riders
//     on this predicate at resolve time. HasMaxSpeed (the keyword
//     check on a Permanent) and MaxSpeedActive are *orthogonal*: the
//     keyword says "I have a max-speed-gated effect"; the player-
//     speed predicate says "the gate is open."
//
// Once-per-turn reset: Turn.SpeedAdvancedThisTurn is part of the
// TurnCounters block and is zeroed by Turn.Reset() in the untap step
// (phases.go:UntapAll), so no extra cleanup hook is needed.

// MaxSpeedCap is the §702.178 ceiling. Speed cannot exceed this value.
const MaxSpeedCap = 4

// SpeedOf returns the current speed of `seatIdx`, or 0 if the seat is
// out of range / nil.
func SpeedOf(gs *GameState, seatIdx int) int {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	s := gs.Seats[seatIdx]
	if s == nil {
		return 0
	}
	return s.Speed
}

// SetSpeed sets a seat's speed directly, clamping to [0..MaxSpeedCap].
// Used by tests and by §704.5z's Start Your Engines! initializer.
// Logs a speed_set event when the value actually changes.
func SetSpeed(gs *GameState, seatIdx, speed int) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seatIdx]
	if s == nil {
		return
	}
	if speed < 0 {
		speed = 0
	}
	if speed > MaxSpeedCap {
		speed = MaxSpeedCap
	}
	if s.Speed == speed {
		return
	}
	old := s.Speed
	s.Speed = speed
	gs.LogEvent(Event{
		Kind:   "speed_set",
		Seat:   seatIdx,
		Amount: speed,
		Details: map[string]interface{}{
			"from": old,
			"to":   speed,
			"rule": "702.178",
		},
	})
}

// MaxSpeedActive returns true when seat.Speed == MaxSpeedCap. This is
// the player-side predicate that gates "max speed" rider effects per
// §702.178. Use IsMaxSpeed(perm) on the rider's controller to bind
// the two together: e.g. MaxSpeedActive(gs, perm.Controller).
func MaxSpeedActive(gs *GameState, seatIdx int) bool {
	return SpeedOf(gs, seatIdx) == MaxSpeedCap
}

// AdvanceSpeed implements the §702.179 advancement rule for one damage
// event:
//
//   - If dealerSeat already advanced this turn → no-op.
//   - If dealerSeat is already at MaxSpeedCap → no-op (logs nothing).
//   - Otherwise: speed +=1, mark SpeedAdvancedThisTurn=true, log
//     a speed_advance event with from/to values.
//
// Returns true if the speed actually changed. Called from
// applyCombatDamageToPlayer (combat.go) and applyDamage (resolve.go)
// at the point a damage event is confirmed to have landed on a player.
//
// dealerSeat may be -1 (e.g. burn from an effect with no controller,
// like a copy with no owner) — those events do not advance any
// player's speed.
func AdvanceSpeed(gs *GameState, dealerSeat int) bool {
	if gs == nil || dealerSeat < 0 || dealerSeat >= len(gs.Seats) {
		return false
	}
	s := gs.Seats[dealerSeat]
	if s == nil {
		return false
	}
	if s.Turn.SpeedAdvancedThisTurn {
		return false
	}
	if s.Speed >= MaxSpeedCap {
		// Mark advanced-this-turn anyway so we don't keep checking on
		// every damage event after the player is already at max. This
		// is a cheap idempotency optimization and not load-bearing for
		// correctness.
		s.Turn.SpeedAdvancedThisTurn = true
		return false
	}
	old := s.Speed
	s.Speed++
	s.Turn.SpeedAdvancedThisTurn = true
	gs.LogEvent(Event{
		Kind:   "speed_advance",
		Seat:   dealerSeat,
		Amount: s.Speed,
		Details: map[string]interface{}{
			"from": old,
			"to":   s.Speed,
			"rule": "702.179",
		},
	})
	return true
}

// ResetSpeedAdvancedFlag is an explicit reset hook for callers (mostly
// tests) that want to simulate a new turn without spinning the full
// untap step. Production code does NOT need to call this — UntapAll's
// Turn.Reset() already zeroes the flag.
func ResetSpeedAdvancedFlag(gs *GameState, seatIdx int) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seatIdx]
	if s == nil {
		return
	}
	s.Turn.SpeedAdvancedThisTurn = false
}
