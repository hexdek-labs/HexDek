package gameengine

// keywords_enlist.go — Enlist (CR §702.154, The Brothers' War 2022).
//
// CR §702.154a: Enlist is a keyword on attacking creatures. "As this
//                creature attacks, you may tap a nonattacking creature you
//                control without summoning sickness. When you do, add its
//                power to this creature's power until end of turn." The
//                tapped creature is NOT an attacking creature — tapping it
//                this way doesn't remove it from combat and doesn't trigger
//                "becomes tapped" / attack triggers.
//
// Engine model (mirrors the SaddleMount / CrewVehicle explicit-action
// primitives — a validated, atomic action the caller drives; the live
// DeclareAttackers/AI auto-invoke is a separate wiring step, as with crew
// and saddle):
//
//   - HasEnlist        — thin keyword reader.
//   - CanEnlist        — validates the enlistee is a legal tap target.
//   - ApplyEnlist      — taps the enlistee, adds its power to the attacker
//                        until end of turn (a fixed amount captured now,
//                        CR §702.154a — not a continuous power-link), and
//                        sets Flags["enlisted_this_combat"] on the attacker
//                        for enlist payoffs (Aradesh, the Founder). The
//                        +X/+0 modification expires in the §514.2 cleanup
//                        sweep; the flag is cleared in EndOfCombatStep.

// HasEnlist reports whether the card has the enlist keyword.
func HasEnlist(card *Card) bool {
	return cardHasKeywordByName(card, "enlist")
}

// CanEnlist reports whether `enlistee` is a legal enlist tap target for an
// attacking `attacker` controlled by `seatIdx` (CR §702.154a): a DIFFERENT,
// untapped, non-summoning-sick creature the same player controls that is NOT
// itself an attacking creature. (Power is unconstrained — a 0-power creature
// is a legal enlistee; "add its power" then adds 0.)
func CanEnlist(gs *GameState, seatIdx int, attacker, enlistee *Permanent) bool {
	if gs == nil || attacker == nil || enlistee == nil {
		return false
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	if enlistee == attacker {
		return false
	}
	if attacker.Controller != seatIdx || enlistee.Controller != seatIdx {
		return false
	}
	if !enlistee.IsCreature() {
		return false
	}
	if enlistee.Tapped {
		return false
	}
	if enlistee.SummoningSick {
		return false
	}
	if enlistee.IsAttacking() {
		// A creature already attacking can't be tapped to enlist
		// ("a nonattacking creature").
		return false
	}
	return true
}

// ApplyEnlist resolves one enlist: taps `enlistee` and adds its current power
// to `attacker`'s power until end of turn (CR §702.154a). Returns true on a
// successful enlist, false on any validation failure (atomic — no tap on
// failure).
//
// Properties:
//   - the enlistee is tapped but does NOT become an attacker (its attacking
//     flag is untouched, so no attack triggers fire for it) — property (c);
//   - the power added is the enlistee's power AT THIS MOMENT, recorded as a
//     fixed +power/+0 modification (not a continuous link) — property (e);
//   - a 0-power (or negative-power) enlistee is legal and adds exactly that —
//     property (d);
//   - Flags["enlisted_this_combat"] is set on the attacker for payoffs
//     (Aradesh, the Founder), cleared in EndOfCombatStep.
func ApplyEnlist(gs *GameState, seatIdx int, attacker, enlistee *Permanent) bool {
	if !CanEnlist(gs, seatIdx, attacker, enlistee) {
		return false
	}
	power := enlistee.Power()
	enlistee.Tapped = true
	if power != 0 {
		attacker.Modifications = append(attacker.Modifications, Modification{
			Power:     power,
			Duration:  "until_end_of_turn",
			Timestamp: gs.NextTimestamp(),
		})
		gs.InvalidateCharacteristicsCache()
	}
	if attacker.Flags == nil {
		attacker.Flags = map[string]int{}
	}
	attacker.Flags["enlisted_this_combat"] = 1

	enlisteeName, attackerName := "", ""
	if enlistee.Card != nil {
		enlisteeName = enlistee.Card.DisplayName()
	}
	if attacker.Card != nil {
		attackerName = attacker.Card.DisplayName()
	}
	gs.LogEvent(Event{
		Kind:   "enlist",
		Seat:   seatIdx,
		Source: attackerName,
		Amount: power,
		Details: map[string]interface{}{
			"enlisted":    enlisteeName,
			"power_added": power,
			"rule":        "702.154a",
		},
	})
	return true
}
