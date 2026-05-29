package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAtemsisAllSeeing wires Atemsis, All-Seeing.
//
// Oracle text (Scryfall, verified 2026-05-04):
//
//	Flying
//	{2}{U}, {T}: Draw two cards, then discard a card.
//	Whenever Atemsis deals damage to an opponent, you may reveal your
//	hand. If cards with at least six different mana values are revealed
//	this way, that player loses the game.
//
// Implementation:
//   - "combat_damage_player": gate on damage_seat == perm.Controller and
//     source_perm == perm. Count distinct mana values in controller's
//     hand; if >= 6, mark the damaged opponent as Lost.
//   - {2}{U},{T} draw-two-discard-one is documented via emitPartial; the
//     activation pipeline doesn't yet route through here for activated
//     loot effects.
//   - Flying handled by AST keyword pipeline.
func registerAtemsisAllSeeing(r *Registry) {
	r.OnTrigger("Atemsis, All-Seeing", "combat_damage_player", atemsisCombatDamage)
	r.OnActivated("Atemsis, All-Seeing", atemsisActivate)
}

func atemsisCombatDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "atemsis_six_mana_values_loss"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	src, _ := ctx["source_perm"].(*gameengine.Permanent)
	if src != perm {
		return
	}
	defenderSeat, ok := ctx["target_seat"].(int)
	if !ok {
		return
	}
	if defenderSeat == perm.Controller || defenderSeat < 0 || defenderSeat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[perm.Controller]
	if s == nil {
		return
	}

	seen := map[int]bool{}
	for _, c := range s.Hand {
		if c == nil {
			continue
		}
		seen[gameengine.ManaCostOf(c)] = true
	}
	distinct := len(seen)

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":            perm.Controller,
		"target_seat":     defenderSeat,
		"distinct_values": distinct,
		"hand_size":       len(s.Hand),
	})

	if distinct >= 6 {
		emitWin(gs, perm.Controller, slug, perm.Card.DisplayName(),
			"opponent_loses_atemsis_six_distinct_mana_values")
		// CR §104.3e: route through canonical helper so §614
		// would_lose_game (Platinum Angel / Angel's Grace) can cancel.
		gameengine.MarkSeatLostByEffect(gs, defenderSeat, perm.Card.DisplayName())
		gs.LogEvent(gameengine.Event{
			Kind:   "player_loses",
			Seat:   defenderSeat,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"slug":   slug,
				"reason": "atemsis_six_mana_values",
			},
		})
		_ = gs.CheckEnd()
	}
}

func atemsisActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil {
		return
	}
	if src.Tapped {
		return
	}
	src.Tapped = true
	emitPartial(gs, "atemsis_loot_ability", src.Card.DisplayName(),
		"draw_two_discard_one_activation_not_routed_through_per_card")
}
