package gameengine

// control_change_302_6_r102.go — CR §302.6 across a control change.
//
// THE DEFECT. §302.6 verbatim:
//
//	"A creature's activated ability with the tap symbol or the untap
//	symbol in its activation cost can't be activated unless the creature
//	has been under its controller's control continuously since their
//	most recent turn began. A creature can't attack unless it has been
//	under its controller's control continuously since their most recent
//	turn began. This rule is informally called the 'summoning sickness'
//	rule."
//
// The predicate is about CONTINUOUS CONTROL, not about entering the
// battlefield. The engine only ever applied the entering half: a
// permanent is stamped SummoningSick at ETB (cast_counts.go) and the
// untap step clears it, and nothing anywhere set it when a permanent
// changed hands. Measured at the production entry point: Mind Control
// resolves on a creature that has been in play for turns, and
// canAttackGS returns true for the thief in the same instant.
//
// So a creature taken by a PERMANENT control effect — Mind Control,
// Control Magic, Agent of Treachery, Gilded Drake, Treachery, the
// Reaper King, Tevesh Szat's -10 — attacked and used its {T} abilities
// on the turn it changed hands, with no haste. The until-end-of-turn
// steals (Act of Treason / Threaten class) were never affected: they
// print "it gains haste" and clear the flag deliberately.
//
// NO NEW FIELD IS NEEDED, and this is a correction to the issue as I
// filed it in r101. The filed row asked for a per-permanent
// ControlledSince stamp plus new reads at the three SummoningSick
// readers. Permanent.SummoningSick ALREADY IS the §302.6 predicate:
// phases.go clears it, at the untap step, for exactly the permanents
// the active seat CONTROLS (it walks seat.Battlefield). So "true" means
// precisely "this controller has not had it continuously since their
// most recent turn began", and the three readers already ask the right
// question. The only thing missing was the write.
//
// THE REVERT DIRECTION — the second correction to the filed row. I
// warned there that a revert "must not stamp, because the owner HAS
// controlled it continuously". Read against the rule that is wrong. If
// you steal my creature on your turn and it comes home at your cleanup
// step, my control of it was INTERRUPTED; it has not been mine
// continuously since my most recent turn began, and §302.6 bites until
// my next untap step. The reverts stamp like any other change of hands.
//
// THE SHAPE. Twenty-five scopes across twenty files hand-rolled the
// same reparent — splice out of one seat's Battlefield, assign
// Controller, append to another's — each one re-deriving which of the
// surrounding bookkeeping to do. One of them, Karona, already stamped
// §302.6 by hand with a correct comment explaining why; it was the only
// one. This file is the canonical operation they now all route through,
// and control_change_sickness_gate_r102_test.go keeps it that way.

// ApplyControlChange moves p under newController's control: the whole
// §110.2a reparent, as one operation.
//
//   - splices p out of its current controller's battlefield slice and
//     appends it to newController's (a Controller flipped WITHOUT the
//     slice move aliases the permanent onto two battlefields — the r63
//     Scarab God / Reaper King CardIdentity duplication),
//   - breaks any soulbond pairing (CR §702.95e), before the timestamp
//     flip, while the old timestamp the pairing is keyed on is intact,
//   - stamps §302.6 summoning sickness on the new controller,
//   - bumps the timestamp and invalidates the characteristics cache, so
//     the §613 layer-2 → layer-7 dependency (anthems and "creatures you
//     control" statics follow control) recomputes.
//
// It does NOT log: callers emit their own card-specific gain_control /
// lose_control events, and a second event here would double-count.
//
// Returns false when nothing moved — p was already under newController
// (idempotent), the seat index is out of range, or p has left the
// battlefield since the caller found it.
//
// A caller whose card PRINTS haste (Act of Treason, Insurrection,
// Dihada's -11, Zidane) clears SummoningSick after this returns. That
// ordering is deliberate: the stamp is the rule, the clear is the card.
func ApplyControlChange(gs *GameState, p *Permanent, newController int) bool {
	if gs == nil || p == nil {
		return false
	}
	if newController < 0 || newController >= len(gs.Seats) || gs.Seats[newController] == nil {
		return false
	}
	if p.Controller == newController {
		return false
	}
	if !gs.removePermanent(p) {
		return false
	}
	// CR §702.95e — a control change breaks a soulbond pairing. Before the
	// Controller/Timestamp flip, while p still carries the old timestamp
	// the pairing link is keyed on.
	UnpairOnLeave(gs, p)
	p.Controller = newController
	p.Timestamp = gs.NextTimestamp()
	// CR §302.6 — the new controller has NOT had this permanent since
	// their most recent turn began. Stamped unconditionally rather than
	// only for creatures: every reader of the field gates on IsCreature
	// already, and a stolen artifact that is animated later is subject to
	// §302.6 from the moment it becomes a creature.
	p.SummoningSick = true
	gs.Seats[newController].Battlefield = append(gs.Seats[newController].Battlefield, p)
	gs.InvalidateCharacteristicsCache()
	return true
}
