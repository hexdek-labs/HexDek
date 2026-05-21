package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerCecilyHauntedMage wires Cecily, Haunted Mage.
//
// Oracle text (Scryfall, verified):
//
//	Your maximum hand size is eleven.
//	Whenever Cecily, Haunted Mage attacks, you draw a card and you
//	lose 1 life. Then if you have eleven or more cards in your hand,
//	you may cast an instant or sorcery spell from your hand without
//	paying its mana cost.
//	Partner—Friends forever
//
// Implementation (R44 stub port):
//   - "Max hand size eleven": stamp seat.Flags["max_hand_size"] = 11
//     at ETB. The cleanup-step max-hand-size scanner reads this flag
//     (same convention as The Second Doctor's no_max_hand_size flag).
//   - Attack trigger: OnTrigger("creature_attacks") gated on
//     attacker_perm == perm. Always draw + lose 1 life. After the
//     draw, if hand size ≥ 11, emit a breadcrumb noting the free-cast
//     option is available; the actual free I/S cast wiring is engine-
//     side and gets emitPartial'd (the per_card layer can't drive an
//     alt-cost cast from hand without changing the cast pipeline).
//   - Partner—Friends forever is a deck-construction tag; no runtime
//     observable behavior.
func registerCecilyHauntedMage(r *Registry) {
	r.OnETB("Cecily, Haunted Mage", cecilyHauntedMageETB)
	r.OnTrigger("Cecily, Haunted Mage", "creature_attacks", cecilyHauntedMageAttack)
	// R51 batch I: LTB clears the max_hand_size = 11 seat flag (only
	// when no OTHER Cecily remains on the controller's battlefield)
	// so the cleanup-step max-hand-size scanner reverts to the 7-card
	// default once Cecily leaves play.
	r.OnTrigger("Cecily, Haunted Mage", "permanent_ltb", cecilyLTBClearFlag)
}

func cecilyLTBClearFlag(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	// Don't clear if another Cecily still in play.
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || p.Card == nil {
			continue
		}
		if normalizeName(p.Card.DisplayName()) == normalizeName("Cecily, Haunted Mage") {
			return
		}
	}
	if seat.Flags != nil {
		delete(seat.Flags, "max_hand_size")
	}
}

func cecilyHauntedMageETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "cecily_haunted_mage_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	s := gs.Seats[perm.Controller]
	if s == nil {
		return
	}
	if s.Flags == nil {
		s.Flags = map[string]int{}
	}
	s.Flags["max_hand_size"] = 11
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"max_hand_size": 11,
	})
}

func cecilyHauntedMageAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "cecily_haunted_mage_attack"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	drew := drawOne(gs, perm.Controller, perm.Card.DisplayName())
	drewName := ""
	if drew != nil {
		drewName = drew.DisplayName()
	}
	gameengine.LoseLife(gs, perm.Controller, 1, perm.Card.DisplayName())
	_ = gs.CheckEnd()

	handCount := len(seat.Hand)
	freeCastEligible := handCount >= 11

	// R52 batch K: stamp/clear a per-seat "cecily_free_is_cast_pending"
	// flag the cast-legality scanner can read to offer the free I/S
	// cast option from hand. The flag is set to 1 when the controller
	// has 11+ cards in hand at this trigger, cleared otherwise. A
	// delayed end-of-turn trigger drops the flag so an unused offer
	// doesn't bleed past Cecily's combat.
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	if freeCastEligible {
		seat.Flags["cecily_free_is_cast_pending"] = 1
		gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
			TriggerAt:      "end_of_turn",
			ControllerSeat: perm.Controller,
			SourceCardName: perm.Card.DisplayName(),
			OneShot:        true,
			EffectFn: func(gs *gameengine.GameState) {
				s := gs.Seats[perm.Controller]
				if s != nil && s.Flags != nil {
					delete(s.Flags, "cecily_free_is_cast_pending")
				}
			},
		})
	} else {
		delete(seat.Flags, "cecily_free_is_cast_pending")
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":               perm.Controller,
		"drawn":              drewName,
		"life_lost":          1,
		"hand_size":          handCount,
		"free_cast_eligible": freeCastEligible,
	})
	if freeCastEligible {
		// Kept-alive breadcrumb: the per_card layer can't drive the
		// actual alt-cost cast from hand; emitPartial documents the
		// cast-pipeline boundary alongside the seat flag set above.
		emitPartial(gs, slug, perm.Card.DisplayName(),
			"free_is_cast_from_hand_alt_cost_pipeline_not_wired_at_per_card_layer")
	}
}
