package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTovolarDireOverlordCustom wires the "transform any number
// of Human Werewolves you control" rider on Tovolar's night-flip
// upkeep trigger. The hand-written tovolar_dire_overlord.go covers
// the wolf-werewolf combat-damage draw, the upkeep wolf/werewolf
// count → is_night flip, and emits the partial breadcrumb for the
// transformation.
//
// Tovolar's tribal-payload completion: when his upkeep trigger has
// just flipped is_night (count ≥ 3 Wolves/Werewolves), iterate the
// controller's battlefield and transform every Human Werewolf (DFC
// front face). The TransformPermanent call mirrors the cecil_paladin
// pattern.
func registerTovolarDireOverlordCustom(r *Registry) {
	r.OnTrigger("Tovolar, Dire Overlord", "upkeep_controller", tovolarTransformHumanWerewolves)
	r.OnTrigger("Tovolar, Dire Overlord // Tovolar, the Midnight Scourge", "upkeep_controller", tovolarTransformHumanWerewolves)
}

func tovolarTransformHumanWerewolves(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "tovolar_transform_human_werewolves"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	// Only fire when the night flip happened. Hand-written upkeep
	// handler sets is_night=1 only after counting ≥3 wolves/werewolves.
	if gs.Flags == nil || gs.Flags["is_night"] != 1 {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	transformed := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		if p.Transformed {
			continue
		}
		// Human Werewolves only. "Human Werewolf" creature type-line means
		// it has both "human" and "werewolf" subtypes (DFC front face).
		if !cardHasTypeAny(p.Card, "human") {
			continue
		}
		if !cardHasTypeAny(p.Card, "werewolf") {
			continue
		}
		gameengine.TransformPermanent(gs, p, "tovolar_night_flip")
		transformed++
	}
	if transformed > 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":        perm.Controller,
			"transformed": transformed,
		})
	}
}
