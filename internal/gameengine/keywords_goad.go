package gameengine

import (
	"strconv"
	"strings"
)

// keywords_goad.go — Goad (CR §701.39, Conspiracy: Take the Crown 2016).
//
// CR §701.39a: To goad a creature means to apply the following effect
//               to it: "Until your next turn, this creature attacks
//               each combat if able and attacks a player other than
//               you if able."
// CR §701.39b: If a creature is goaded by more than one player, on
//               each combat its controller chooses one of the goading
//               players. It must attack a player other than that
//               player if able. (We model the most-recent-goader case
//               via goaded_by_seat; a future commit can extend to a
//               goader set.)
//
// Engine model
// ------------
// Goad is a turn-windowed flag pair stamped on the targeted
// permanent:
//
//   perm.Flags["goaded_by_seat"]    = sourceSeat  // who goaded
//   perm.Flags["goaded_until_turn"] = expiryTurn  // see formula
//
// where expiryTurn = gs.Turn + seatsBefore(sourceSeat). The
// seatsBefore helper returns the number of player-turns that must
// pass before sourceSeat's NEXT turn begins; combined with the
// "IsGoaded → goaded_until_turn > currentTurn" predicate this gives
// the canonical "until your next turn" window: the goad is active on
// every intervening player's turn AND on the goaded creature's own
// next turn (which is the one that matters for "attacks each combat
// if able"), and expires exactly when the goader's next turn begins.
//
// Worked example (4-seat game, goader = seat 0, gs.Turn = 10,
// gs.Active = 0 at goad time):
//
//   seatsBefore(0) = 4     // need a full lap before seat 0 plays again
//   goaded_until_turn = 14
//
//   Turn 10 (seat 0):  14 > 10 → IsGoaded=true
//   Turn 11 (seat 1):  14 > 11 → IsGoaded=true
//   Turn 12 (seat 2):  14 > 12 → IsGoaded=true   ← if goaded creature is seat 2's, MUST attack
//   Turn 13 (seat 3):  14 > 13 → IsGoaded=true
//   Turn 14 (seat 0 — goader's next turn): 14 > 14 → IsGoaded=false  ← expires
//
// The predicates here are standalone — the existing
// DeclareAttackers pipeline in combat.go doesn't have a
// "required-attackers" hook today, so the AI/Hat layer is expected
// to consult MustAttackIfAble + CannotAttackGoader directly when
// choosing attackers and defenders. ExpireGoadAtCleanup is the
// explicit flag-cleanup pass that callers (typically the goader's
// untap step or a combat-AI prelude) can run to keep the flag map
// tidy; lazy callers can ignore it because IsGoaded already does
// the turn-comparison check.

// ---------------------------------------------------------------------------
// seatsBefore
// ---------------------------------------------------------------------------

// seatsBefore returns the number of player-turns that must pass
// before `sourceSeat`'s NEXT turn begins, counted from gs.Active.
// If sourceSeat is currently the active player, the answer is the
// total seat count N — they need a full lap before playing again.
//
// Examples (4-seat game, gs.Active = 1):
//
//   seatsBefore(2) = 1   // seat 2 next turn after this one
//   seatsBefore(3) = 2
//   seatsBefore(0) = 3
//   seatsBefore(1) = 4   // current active — full lap until next turn
//
// Returns 0 on degenerate inputs (nil game, no seats, out-of-range
// sourceSeat) so callers don't trip on edge cases.
func seatsBefore(gs *GameState, sourceSeat int) int {
	if gs == nil {
		return 0
	}
	n := len(gs.Seats)
	if n == 0 || sourceSeat < 0 || sourceSeat >= n {
		return 0
	}
	d := (sourceSeat - gs.Active + n) % n
	if d == 0 {
		return n
	}
	return d
}

// ---------------------------------------------------------------------------
// GoadCreature — stamp the turn-windowed flag pair
// ---------------------------------------------------------------------------

// GoadCreature applies the §701.39 goad effect to `perm` for
// `sourceSeat`. Stamps:
//
//   perm.Flags["goaded_by_seat"]    = sourceSeat
//   perm.Flags["goaded_until_turn"] = gs.Turn + seatsBefore(sourceSeat)
//
// Re-goading the same permanent (same or different sourceSeat)
// REPLACES both flags — the most recent goad's controller is the
// one CannotAttackGoader keys on. A future commit can extend to a
// multi-goader set for §701.39b's "each combat its controller
// chooses one of the goading players" wording.
//
// No-op for nil game, nil perm, out-of-range sourceSeat, or a perm
// whose controller has lost the game.
func GoadCreature(gs *GameState, sourceSeat int, perm *Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if sourceSeat < 0 || sourceSeat >= len(gs.Seats) {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	expiry := gs.Turn + seatsBefore(gs, sourceSeat)
	// CR §701.39b — a creature can be goaded by more than one player, each
	// goad with its own "until your next turn" expiry. Store one key per
	// goader (goad_from_<seat> = that goader's expiry) so the must-attack and
	// "attack a player other than ANY goader" restrictions stack correctly and
	// each goad wears off independently.
	perm.Flags["goad_from_"+itoa(sourceSeat)] = expiry
	// Legacy single-goader fields: most-recent goader + the MAX expiry across
	// all active goads (so IsGoaded stays true while any goad is live). Kept
	// for readers that predate the multi-goader set.
	perm.Flags["goaded_by_seat"] = sourceSeat
	if cur, ok := perm.Flags["goaded_until_turn"]; !ok || expiry > cur {
		perm.Flags["goaded_until_turn"] = expiry
	}
	// Legacy boolean kept in lockstep with the turn-windowed version so
	// existing readers keep working without migration.
	perm.Flags["goaded"] = 1

	cardName := ""
	if perm.Card != nil {
		cardName = perm.Card.DisplayName()
	}
	gs.LogEvent(Event{
		Kind:   "goad_apply",
		Seat:   sourceSeat,
		Source: cardName,
		Details: map[string]interface{}{
			"rule":              "701.39a",
			"goaded_seat":       perm.Controller,
			"goaded_until_turn": expiry,
		},
	})
}

// ---------------------------------------------------------------------------
// IsGoaded / IsGoadedBy / GoadedBySeat
// ---------------------------------------------------------------------------

// IsGoaded reports whether `perm` is currently under a goad effect,
// based on the spec'd "goaded_until_turn > currentTurn" check.
// Lazy-cleanup safe: a stale goaded_until_turn flag (one that hasn't
// been swept by ExpireGoadAtCleanup) reads false here automatically.
func IsGoaded(perm *Permanent, currentTurn int) bool {
	if perm == nil || perm.Flags == nil {
		return false
	}
	expiry, ok := perm.Flags["goaded_until_turn"]
	if !ok {
		// Fall back on the legacy boolean for permanents stamped by
		// older code paths that never set goaded_until_turn.
		return perm.Flags["goaded"] > 0
	}
	return expiry > currentTurn
}

// IsGoadedBy reports whether `perm` is currently goaded BY a
// specific seat. Returns false if perm isn't goaded or if the
// goader is a different seat (multi-goader semantics not yet
// modelled — the most recent goader wins).
func IsGoadedBy(perm *Permanent, sourceSeat, currentTurn int) bool {
	if !IsGoaded(perm, currentTurn) {
		return false
	}
	if perm.Flags == nil {
		return false
	}
	goader, ok := perm.Flags["goaded_by_seat"]
	if !ok {
		return false
	}
	return goader == sourceSeat
}

// GoadedBySeat returns the seat that goaded `perm` and whether
// the perm is currently under an active goad. Useful for AI/Hat
// code that needs to derive the forbidden defender for an
// attack-target decision.
func GoadedBySeat(perm *Permanent, currentTurn int) (int, bool) {
	if !IsGoaded(perm, currentTurn) {
		return -1, false
	}
	if perm.Flags == nil {
		return -1, false
	}
	goader, ok := perm.Flags["goaded_by_seat"]
	if !ok {
		return -1, false
	}
	return goader, true
}

// ---------------------------------------------------------------------------
// MustAttackIfAble / CannotAttackGoader
// ---------------------------------------------------------------------------

// MustAttackIfAble reports whether `perm` must attack this combat
// per the goad rider. True iff the perm is goaded AND it's its
// controller's turn — the "attacks each combat if able" clause
// only applies during the goaded creature's controller's turn
// (it can't attack on someone else's turn anyway).
//
// "If able" is the caller's responsibility — tapped, summoning-
// sick, "can't attack" effects all override this. The predicate
// only answers "is this creature under a 'must attack' obligation
// right now?".
func MustAttackIfAble(gs *GameState, perm *Permanent) bool {
	if gs == nil || perm == nil {
		return false
	}
	if !IsGoaded(perm, gs.Turn) {
		return false
	}
	return perm.Controller == gs.Active
}

// activeGoaders returns every seat that currently has an active goad on
// `perm` (its per-goader expiry is still in the future). Multi-goader set
// per CR §701.39b. Order is not significant (the caller treats it as a set).
func activeGoaders(perm *Permanent, currentTurn int) []int {
	if perm == nil || perm.Flags == nil {
		return nil
	}
	var out []int
	for k, expiry := range perm.Flags {
		if !strings.HasPrefix(k, "goad_from_") {
			continue
		}
		if expiry <= currentTurn {
			continue
		}
		if seat, err := strconv.Atoi(strings.TrimPrefix(k, "goad_from_")); err == nil {
			out = append(out, seat)
		}
	}
	return out
}

// CannotAttackGoader reports whether `perm` is forbidden from
// attacking `defenderSeat` due to goad's "attacks a player other
// than [any goader] if able" clause (CR §701.39b). True iff perm is
// currently goaded by defenderSeat. Used by the attack-target picker
// to filter the defender pool.
//
// "If able" applies here too: when EVERY other defender is dead or
// otherwise unreachable, the rule allows attacking a goader. The
// caller is responsible for the "if able" escape hatch; this
// predicate only answers the structural "is this defender a goader?"
// question.
func CannotAttackGoader(gs *GameState, perm *Permanent, defenderSeat int) bool {
	if gs == nil || perm == nil {
		return false
	}
	for _, g := range activeGoaders(perm, gs.Turn) {
		if g == defenderSeat {
			return true
		}
	}
	// Fall back to the legacy single-goader field for goads stamped by older
	// paths that never wrote a goad_from_<seat> key.
	goader, ok := GoadedBySeat(perm, gs.Turn)
	return ok && goader == defenderSeat
}

// ---------------------------------------------------------------------------
// ExpireGoadAtCleanup
// ---------------------------------------------------------------------------

// ExpireGoadAtCleanup clears the goad flags on `perm` if the
// goader's next turn has begun (gs.Turn >= goaded_until_turn).
// Returns true iff it cleared an active stamp.
//
// Lazy callers can skip this and rely on IsGoaded's
// turn-comparison short-circuit. Eager callers (e.g. the goader's
// untap step, or a combat-AI prelude that wants a clean flag map
// before iterating) should run it via ExpireAllGoadsAtCleanup.
func ExpireGoadAtCleanup(gs *GameState, perm *Permanent) bool {
	if gs == nil || perm == nil || perm.Flags == nil {
		return false
	}
	expiry, ok := perm.Flags["goaded_until_turn"]
	if !ok {
		return false
	}
	if gs.Turn < expiry {
		return false
	}
	// goaded_until_turn is the MAX expiry across all goaders, so reaching it
	// means every goad has worn off — clear the full set.
	for k := range perm.Flags {
		if strings.HasPrefix(k, "goad_from_") {
			delete(perm.Flags, k)
		}
	}
	delete(perm.Flags, "goaded_by_seat")
	delete(perm.Flags, "goaded_until_turn")
	delete(perm.Flags, "goaded")

	cardName := ""
	if perm.Card != nil {
		cardName = perm.Card.DisplayName()
	}
	gs.LogEvent(Event{
		Kind:   "goad_expire",
		Seat:   perm.Controller,
		Source: cardName,
		Details: map[string]interface{}{
			"rule": "701.39a",
		},
	})
	return true
}

// ExpireAllGoadsAtCleanup sweeps every seat's battlefield and
// clears expired goads. Returns the count cleared. Intended to run
// at the start of each turn (after gs.Turn has advanced) so any
// goad whose expiry has passed releases the flag map.
func ExpireAllGoadsAtCleanup(gs *GameState) int {
	if gs == nil {
		return 0
	}
	cleared := 0
	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		for _, p := range seat.Battlefield {
			if ExpireGoadAtCleanup(gs, p) {
				cleared++
			}
		}
	}
	return cleared
}
