package per_card

import (
	"strconv"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSamLoyalAttendant wires Sam, Loyal Attendant (Muninn parser-gap
// #159, single-game hit on 2026-05-14 — Frodo, Adventurous Hobbit's
// printed partner).
//
// Oracle text (Scryfall, verified 2026-05-17 via hexdek.dev oracle):
//
//	{1}{G}{W}
//	Legendary Creature — Halfling Peasant
//	Partner with Frodo, Adventurous Hobbit (When this creature enters,
//	target player may put Frodo into their hand from their library, then
//	shuffle.)
//	At the beginning of combat on your turn, create a Food token.
//	Activated abilities of Foods you control cost {1} less to activate.
//
// Implementation:
//   - Partner with Frodo: declarative deck-building mechanic. Same shape
//     as registerFrodoAdventurousHobbit — the partner ETB tutor lives in
//     party/lobby code, not in per-card runtime. R60 batch 9: dropped
//     the stale "partner_with_etb_search_for_frodo_declarative_only"
//     emitPartial; the partner-with semantics are handled in the party
//     layer, not by a missing per_card hook.
//   - "combat_begin" trigger gated on active_seat == controller: drop a
//     Food token via gameengine.CreateFoodToken. De-duped per turn via a
//     per-permanent flag so any extra-combat phases that ever land won't
//     stack tokens beyond the intent of the printed text.
//   - Food activation cost reduction: the engine has no first-class
//     static-cost-reduction effect for activated abilities of a card
//     subtype. emitPartial documents the gap. In practice Food's only
//     activated ability is "{2}, {T}, Sacrifice: gain 3 life", and Hat
//     evaluates Food sacs heuristically rather than via precise mana
//     budgeting, so the gap is observationally invisible at policy time.
func registerSamLoyalAttendant(r *Registry) {
	r.OnETB("Sam, Loyal Attendant", samLoyalAttendantETB)
	r.OnTrigger("Sam, Loyal Attendant", "combat_begin", samLoyalAttendantCombatBegin)
}

func samLoyalAttendantETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	// R60 batch 9: partner-with is handled at the party/lobby layer; no
	// per_card ETB hook needed. Body kept for future per_card expansion
	// (e.g., a fast-path mood emit for analytics if desired).
	_ = gs
	_ = perm
}

func samLoyalAttendantCombatBegin(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "sam_loyal_attendant_combat_begin_food"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	key := "sam_food_turn_" + strconv.Itoa(gs.Turn)
	if perm.Flags[key] > 0 {
		return
	}
	perm.Flags[key] = 1

	gameengine.CreateFoodToken(gs, perm.Controller)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"token": "Food",
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"food_activated_abilities_cost_one_less_static_effect_unimplemented")
}
