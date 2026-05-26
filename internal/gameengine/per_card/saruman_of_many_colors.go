package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSarumanOfManyColors wires Saruman of Many Colors.
//
// Oracle text (Scryfall, verified 2026-05-04):
//
//	Ward—Discard an enchantment, instant, or sorcery card.
//	Whenever you cast your second spell each turn, each opponent
//	mills two cards. When one or more cards are milled this way,
//	exile target enchantment, instant, or sorcery card with equal or
//	lesser mana value than that spell from an opponent's graveyard.
//	Copy the exiled card. You may cast the copy without paying its
//	mana cost.
//
// Implementation:
//   - ETB stamps a non-mana ward marker (alt-payment ward isn't fully
//     modeled — emitPartial flags the gap).
//   - "spell_cast" trigger: when Saruman's controller casts their second
//     spell of the turn, each opponent mills 2; we then pick the
//     highest-CMC eligible card across opponent graveyards (CMC <= just-
//     cast spell CMC), exile it, and emit a "saruman_copy_cast" event
//     for the cast pipeline. The actual free-cast hand-off is partial.
func registerSarumanOfManyColors(r *Registry) {
	r.OnETB("Saruman of Many Colors", sarumanOfManyColorsETB)
	r.OnTrigger("Saruman of Many Colors", "spell_cast", sarumanOfManyColorsSecondSpell)
}

func sarumanOfManyColorsETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "saruman_of_many_colors_ward"
	if gs == nil || perm == nil {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["kw:ward"] = 1
	// R60 closure of half-finished-features-r48 #4: route the discard
	// alt-payment ward through ward_alt_payment.go's canonical handler.
	// Replaces the prior
	// "ward_discard_enchantment_instant_sorcery_alt_payment_unimplemented"
	// emitPartial. The engine's caster-side handler discards the highest-
	// CMC matching card (instant/sorcery/enchantment) from the opponent's
	// hand; if none, the targeting spell is countered per CR §702.21c.
	perm.Flags["ward_alt_kind"] = gameengine.WardAltKindDiscardInstSorcEnch
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"ward_alt_kind": "discard_inst_sorc_ench",
	})
}

func sarumanOfManyColorsSecondSpell(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "saruman_of_many_colors_second_spell_mill_copy"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	if casterSeat < 0 || casterSeat >= len(gs.Seats) || gs.Seats[casterSeat] == nil {
		return
	}
	if gs.Seats[casterSeat].SpellsCastThisTurn != 2 {
		return
	}
	castCard, _ := ctx["card"].(*gameengine.Card)
	castCMC := 0
	if castCard != nil {
		castCMC = cardCMC(castCard)
	}

	totalMilled := 0
	for _, oppIdx := range gs.Opponents(perm.Controller) {
		s := gs.Seats[oppIdx]
		if s == nil || s.Lost {
			continue
		}
		for i := 0; i < 2 && len(s.Library) > 0; i++ {
			top := s.Library[0]
			gameengine.MoveCard(gs, top, oppIdx, "library", "graveyard", "saruman_mill")
			totalMilled++
		}
	}

	// Find best eligible exile target across opponent graveyards.
	var best *gameengine.Card
	var bestSeat int = -1
	bestCMC := -1
	for _, oppIdx := range gs.Opponents(perm.Controller) {
		s := gs.Seats[oppIdx]
		if s == nil {
			continue
		}
		for _, c := range s.Graveyard {
			if c == nil {
				continue
			}
			if !cardHasType(c, "enchantment") && !cardHasType(c, "instant") && !cardHasType(c, "sorcery") {
				continue
			}
			cm := cardCMC(c)
			if cm > castCMC {
				continue
			}
			if cm > bestCMC {
				bestCMC = cm
				best = c
				bestSeat = oppIdx
			}
		}
	}
	if best != nil {
		gameengine.MoveCard(gs, best, bestSeat, "graveyard", "exile", "saruman_exile_target")
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"milled_total":  totalMilled,
		"exiled":        cardDisp(best),
		"exiled_seat":   bestSeat,
		"cast_cmc":      castCMC,
	})
	if best != nil {
		emitPartial(gs, slug, perm.Card.DisplayName(),
			"copy_and_free_cast_of_exiled_target_not_executed_in_handler")
	}
}
