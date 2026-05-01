package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerEtali wires Etali, Primal Conqueror // Etali, Primal Sickness.
//
// Front face — Etali, Primal Conqueror:
//
//	When Etali, Primal Conqueror enters the battlefield, each player
//	exiles cards from the top of their library until they exile a
//	nonland card. You may cast any number of spells from among the
//	nonland cards exiled this way without paying their mana costs.
//	{7}{G}{G}: Transform Etali. Activate only as a sorcery.
//
// Back face — Etali, Primal Sickness:
//
//	Trample
//	Whenever Etali, Primal Sickness deals combat damage to a player,
//	that player gets that many poison counters.
//
// ETB implementation:
//   - For each seat, exile cards from the top of the library one at a
//     time until a nonland is exiled (lands stay in exile).
//   - The controller "casts" each nonland for free. In simulation we
//     short-circuit:
//       - Permanents (creature/artifact/enchantment/planeswalker/battle):
//         pulled from exile and entered onto the battlefield with a full
//         ETB cascade.
//       - Instants/sorceries: emitPartial (resolution shortcut TODO —
//         spell effects without paying cost is non-trivial without a
//         dedicated free-cast resolver path).
//
// Transform activation — emitPartial. The actual face swap calls
// gameengine.TransformPermanent and switches Etali to Primal Sickness.
//
// Combat trigger — registered on "combat_damage_player". Fires only
// when the source is the transformed back face (Primal Sickness) and
// gives the damaged player poison counters equal to the damage dealt.
func registerEtali(r *Registry) {
	r.OnETB("Etali, Primal Conqueror", etaliETB)
	r.OnActivated("Etali, Primal Conqueror", etaliActivate)
	r.OnTrigger("Etali, Primal Conqueror", "combat_damage_player", etaliPoisonTrigger)
}

func etaliETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "etali_primal_conqueror_exile_cast"
	if gs == nil || perm == nil {
		return
	}
	controller := perm.Controller
	if controller < 0 || controller >= len(gs.Seats) {
		return
	}

	type exiled struct {
		seat int
		card *gameengine.Card
	}
	var nonland []exiled
	var landsExiled int

	for seatIdx, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		for len(s.Library) > 0 {
			top := s.Library[0]
			if top == nil {
				s.Library = s.Library[1:]
				continue
			}
			gameengine.MoveCard(gs, top, seatIdx, "library", "exile", "etali_etb")
			if cardHasType(top, "land") {
				landsExiled++
				continue
			}
			nonland = append(nonland, exiled{seat: seatIdx, card: top})
			break
		}
	}

	var castNames []string
	var partialNames []string
	for _, ex := range nonland {
		card := ex.card
		if card == nil {
			continue
		}
		if cardHasType(card, "creature") ||
			cardHasType(card, "artifact") ||
			cardHasType(card, "enchantment") ||
			cardHasType(card, "planeswalker") ||
			cardHasType(card, "battle") {
			gameengine.MoveCard(gs, card, ex.seat, "exile", "battlefield", "etali_free_cast")
			ent := enterBattlefieldWithETB(gs, controller, card, false)
			if ent != nil {
				castNames = append(castNames, card.DisplayName())
			}
			continue
		}
		if cardHasType(card, "instant") || cardHasType(card, "sorcery") {
			partialNames = append(partialNames, card.DisplayName())
			continue
		}
		// Unknown nonland (e.g. tribal) — leave in exile.
		partialNames = append(partialNames, card.DisplayName())
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":             controller,
		"lands_exiled":     landsExiled,
		"nonlands_exiled":  len(nonland),
		"cast_permanents":  castNames,
		"partial_spells":   partialNames,
	})
	if len(partialNames) > 0 {
		emitPartial(gs, slug, perm.Card.DisplayName(),
			"instant_or_sorcery_free_cast_resolution_shortcut_unimplemented")
	}
}

func etaliActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "etali_transform_to_primal_sickness"
	if gs == nil || src == nil {
		return
	}
	if !gameengine.TransformPermanent(gs, src, "etali_activated_transform") {
		emitPartial(gs, slug, src.Card.DisplayName(),
			"transform_failed_face_data_missing")
		return
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": src.Controller,
		"to":   "Etali, Primal Sickness",
	})
}

func etaliPoisonTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "etali_primal_sickness_combat_poison"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	if !perm.Transformed {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	sourceName, _ := ctx["source_card"].(string)
	defenderSeat, _ := ctx["defender_seat"].(int)
	amount, _ := ctx["amount"].(int)
	if sourceSeat != perm.Controller || amount <= 0 {
		return
	}
	if defenderSeat < 0 || defenderSeat >= len(gs.Seats) {
		return
	}
	if sourceName != "" && sourceName != perm.Card.DisplayName() {
		return
	}

	gs.Seats[defenderSeat].PoisonCounters += amount
	gs.LogEvent(gameengine.Event{
		Kind:   "poison",
		Seat:   perm.Controller,
		Target: defenderSeat,
		Source: perm.Card.DisplayName(),
		Amount: amount,
		Details: map[string]interface{}{
			"slug":        slug,
			"reason":      "etali_primal_sickness_combat",
			"source_card": perm.Card.DisplayName(),
			"rule":        "702.2",
			"target_kind": "player",
			"combat":      true,
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"defender_seat": defenderSeat,
		"poison":        amount,
	})
}
