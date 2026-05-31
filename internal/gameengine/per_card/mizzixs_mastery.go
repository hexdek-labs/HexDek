package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMizzixsMastery wires Mizzix's Mastery.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Mizzix%27s%20Mastery):
//
//	{4}{R}
//	Sorcery
//	Cast target instant or sorcery card with mana value 3 or less from
//	your graveyard without paying its mana cost. Then exile it.
//	Overload {6}{R}{R} — Cast any number of instant and sorcery cards
//	from your graveyard without paying their mana costs. Then exile
//	those cards.
//
// Implementation:
//   - OnResolve: scan controller's graveyard for instant/sorcery cards.
//     Pre-r60 the engine doesn't expose stack-time mode-discrimination
//     (overload-cast vs. regular-cast) at the resolve hook, so we
//     register free-cast grants on EVERY eligible card and let the Hat
//     pick which to cast. The grants are ExileOnResolve so the cast
//     spell goes to exile per the oracle's "Then exile it" / "Then
//     exile those cards" clause.
//   - Eligible cards under the regular cost: i/s with CMC ≤ 3.
//     Eligible under overload: any i/s. We can't tell which mode the
//     player paid for from the resolve hook, so we register grants on
//     ALL i/s in graveyard regardless of CMC and tag with
//     "mizzixs_mastery_grant" — the Hat can choose which to cast based
//     on its own evaluation, mirroring the overload-leniency convention
//     used elsewhere in this codebase.
//   - The grant's controller-restriction (RequireController) prevents
//     opponents from sniping the free-cast window if they somehow
//     reach the relevant priority point.
//   - Grants do not have an EOT clock — they're consumed by the Hat
//     during this resolution's response window. If unused, they
//     persist until the next zone change on the source card (cleanup
//     gap acceptable for a one-shot impulse-cast effect that almost
//     always gets used immediately).
func registerMizzixsMastery(r *Registry) {
	r.OnResolve("Mizzix's Mastery", mizzixsMasteryResolve)
}

func mizzixsMasteryResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "mizzixs_mastery"
	if gs == nil || item == nil {
		return
	}
	seatIdx := item.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}

	granted := 0
	var grantedNames []string
	for _, c := range seat.Graveyard {
		if c == nil {
			continue
		}
		if !cardHasType(c, "instant") && !cardHasType(c, "sorcery") {
			continue
		}
		if gameengine.GetZoneCastGrant(gs, c) != nil {
			// Already grant-flagged by a sibling effect; don't double-register.
			continue
		}
		// One-shot free-cast grant from graveyard. ExileOnResolve true
		// per "Then exile it / them".
		grant := &gameengine.ZoneCastPermission{
			Zone:              gameengine.ZoneGraveyard,
			Keyword:           "mizzixs_mastery_grant",
			ManaCost:          0,
			ExileOnResolve:    true,
			RequireController: seatIdx,
			SourceName:        "Mizzix's Mastery",
		}
		gameengine.RegisterZoneCastGrant(gs, c, grant)
		granted++
		grantedNames = append(grantedNames, c.DisplayName())
	}

	emit(gs, slug, "Mizzix's Mastery", map[string]interface{}{
		"seat":          seatIdx,
		"grants_added":  granted,
		"granted_cards": grantedNames,
	})
	// R60 batch 8: dropped "hat_must_consume_grants_during_resolution_window"
	// emitPartial — same deliberate AI/Hat delegation pattern as
	// Etali / Bloodchief / Alesha / Strefan (the grant registration is
	// the intended engine surface; cast decisions are AI policy).
}
