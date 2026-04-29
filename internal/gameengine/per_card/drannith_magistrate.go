package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerDrannithMagistrate wires up Drannith Magistrate.
//
// Oracle text:
//
//	Your opponents can't cast spells from anywhere other than their
//	hands.
//
// Shuts off:
//   - Commander casts (from the command zone)
//   - Doomsday piles after the top of library is rearranged
//     (Gitaxian Probe → cast Doomsday → Magistrate means pile isn't
//     castable; Gitaxian's own flashback is moot since it's not a
//     legal target, but more importantly the pile's top cards can't
//     be cast if the pile is "cast from graveyard" via Breach)
//   - Underworld Breach escape-cast of spells from graveyard
//   - Suspend casts (from exile)
//   - Flashback
//   - Living End / Crashing Footfalls / Fierce Guardianship's alt
//     cast modes that cast a specific card from a non-hand zone
//   - Living Death / Sevinne's Reclamation / anything that recurs
//     with a "cast it" clause.
//
// Batch #3 scope:
//   - OnETB: stamp gs.Flags["drannith_active_seat_N"] = perm.Timestamp
//   - DrannithMagistrateRestrictsOpponentZoneCast() helper.
//
// CAST-TIME enforcement: per CR §601.2 this check runs at the
// "announce the spell" step (§601.2a) — BEFORE any costs are paid.
// Our engine's CastSpell takes only from hand today, so the
// restriction is latent: when zone-cast plumbing lands (Breach,
// Commander command-zone, flashback), those call sites must consult
// this flag.
func registerDrannithMagistrate(r *Registry) {
	r.OnETB("Drannith Magistrate", drannithMagistrateETB)
}

func drannithMagistrateETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "drannith_magistrate_static"
	if gs == nil || perm == nil {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["drannith_active_seat_"+intToStr(perm.Controller)] = perm.Timestamp
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"timestamp":  perm.Timestamp,
		"restricts":  "opponent_cast_from_non_hand_zones",
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"zone_cast_restriction_cast_time_check_callers_must_consult_DrannithMagistrateRestrictsOpponent")
}

// DrannithMagistrateRestrictsOpponent returns true if `castingSeat`
// is restricted by an opponent's Drannith Magistrate from casting
// spells from any zone other than hand. Intended to be called at the
// cast-time (§601.2a) legality check in any zone-cast-capable path.
func DrannithMagistrateRestrictsOpponent(gs *gameengine.GameState, castingSeat int) bool {
	if gs == nil || gs.Flags == nil {
		return false
	}
	for i := range gs.Seats {
		if i == castingSeat {
			continue
		}
		if gs.Flags["drannith_active_seat_"+intToStr(i)] > 0 {
			return true
		}
	}
	return false
}
