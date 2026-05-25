package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerYawgmothsAgenda wires Yawgmoth's Agenda (R60).
//
// Oracle text:
//
//	You can't cast more than one spell each turn.
//	You may play lands and cast spells from your graveyard.
//	If a card would be put into your graveyard from anywhere, exile
//	it instead.
//
// The static permanent analogue to Yawgmoth's Will. Persistent
// graveyard-recursion engine for slow control / lock decks at the
// cost of a one-spell-per-turn restriction.
//
// Implementation:
//   - OnETB: RegisterPlayFromGraveyard with Permanent=true,
//     SourcePerm=perm, so the bundle (ZoneCastGrants, ZoneCastPolicy,
//     §614 GY→exile replacement) stays alive while the enchantment
//     is on the battlefield. The seat flag for land play uses the
//     "perm" suffix and is cleaned up on LTB, not at EOT cleanup.
//   - OnTrigger permanent_ltb: UnregisterPlayFromGraveyardForPermanent
//     plus the engine's standard UnregisterReplacementsForPermanent /
//     UnregisterZoneCastPoliciesForPermanent (handled by the LTB
//     pathway in zone_change.go) drop the entire bundle.
//   - One-spell-per-turn restriction is NOT modeled here — the engine
//     doesn't yet expose a "cap spell-casts at N per turn" primitive.
//     emitPartial flags the gap.
func registerYawgmothsAgenda(r *Registry) {
	r.OnETB("Yawgmoth's Agenda", yawgmothsAgendaETB)
	r.OnTrigger("Yawgmoth's Agenda", "permanent_ltb", yawgmothsAgendaLTB)
}

func yawgmothsAgendaETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "yawgmoths_agenda"
	if gs == nil || perm == nil {
		return
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	granted := gameengine.RegisterPlayFromGraveyard(gs, gameengine.PlayFromGraveyardOptions{
		SeatIdx:    seatIdx,
		SourceName: perm.Card.DisplayName(),
		SourcePerm: perm,
		Permanent:  true,
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":            seatIdx,
		"per_card_grants": granted,
		"duration":        "while_source_on_bf",
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"one_spell_per_turn restriction not modeled (no spell-cap primitive)")
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"land_play_from_graveyard consumer not yet wired in tryPlayLand")
}

func yawgmothsAgendaLTB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	gameengine.UnregisterPlayFromGraveyardForPermanent(gs, perm)
	// The engine's permanent-LTB pathway already invokes
	// UnregisterReplacementsForPermanent and
	// UnregisterZoneCastPoliciesForPermanent for `perm`, so the §614
	// replacement and the R55 policy fall away with the source.
}
