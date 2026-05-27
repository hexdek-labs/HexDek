package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// per_card_batch_m_r60.go — 5 more commander-tier handler ports, same
// shape as batch K (per_card_batch_k_r60.go): one handler per card,
// one regression per handler, AddResetHook so the registrations
// survive test-side Reset() rebuilds.
//
//   - Vega, the Watcher          — cast from non-hand → draw 1
//   - Toothy, Imaginary Friend   — you draw → +1/+1 counter on Toothy
//   - Alandra, Sky Dreamer       — 2nd draw each turn → mint 2/2 U Drake
//                                    w/ flying. 5th-draw +X/+X to Drakes
//                                    is a continuous mod requiring a
//                                    Layer-7 grant; emitPartial.
//   - Tuvasa the Sunlit          — first enchantment cast each turn →
//                                    draw 1. Self-buff (+1/+1 per
//                                    enchantment) is a CDA; emitPartial.
//   - Grolnok, the Omnivore      — Frog you control attacks → mill 3.
//                                    Croak-counter exile-from-mill is
//                                    a separate trigger family;
//                                    emitPartial.

func init() {
	registerPerCardBatchMR60(Global())
	AddResetHook(registerPerCardBatchMR60)
}

func registerPerCardBatchMR60(r *Registry) {
	r.OnTrigger("Vega, the Watcher", "spell_cast", vegaTheWatcherOnCast)
	r.OnTrigger("Toothy, Imaginary Friend", "card_drawn", toothyImaginaryFriendOnDraw)
	r.OnTrigger("Alandra, Sky Dreamer", "card_drawn", alandraSkyDreamerOnDraw)
	r.OnTrigger("Tuvasa the Sunlit", "spell_cast", tuvasaTheSunlitOnCast)
	r.OnTrigger("Grolnok, the Omnivore", "creature_attacks", grolnokTheOmnivoreOnFrogAttack)
}

// ---------------------------------------------------------------------------
// Vega, the Watcher
// ---------------------------------------------------------------------------
//
// Oracle: "Flying. Whenever you cast a spell from anywhere other than
// your hand, draw a card."
//
// Gate on caster == controller AND cast_zone != "hand". The cast hooks
// surface "cast_zone" on the ctx (see The Twelfth Doctor batch H for
// the same key). Flying is AST keyword pipeline.
func vegaTheWatcherOnCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "vega_the_watcher_non_hand_cast_draw"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	caster, _ := ctx["caster_seat"].(int)
	if caster != perm.Controller {
		return
	}
	castZone, _ := ctx["cast_zone"].(string)
	if castZone == "" || castZone == "hand" {
		return
	}
	drawOne(gs, perm.Controller, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"cast_zone": castZone,
	})
}

// ---------------------------------------------------------------------------
// Toothy, Imaginary Friend
// ---------------------------------------------------------------------------
//
// Oracle: "Partner with Pir, Imaginative Rascal. Whenever you draw a
// card, put a +1/+1 counter on Toothy."
//
// Gate on draw seat == controller. Partner-with summon is a deck-
// construction concern (not runtime), so the trigger is just the
// self-buff. Toothy's "When this leaves, draw X" half is a separate
// permanent_ltb trigger we leave for a follow-up batch.
func toothyImaginaryFriendOnDraw(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "toothy_imaginary_friend_on_draw"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	seat, _ := ctx["seat"].(int)
	if seat != perm.Controller {
		return
	}
	perm.AddCounter("+1/+1", 1)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         perm.Controller,
		"new_counters": perm.Counters["+1/+1"],
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"leave_battlefield_draw_x_unimplemented_needs_permanent_ltb_handler")
}

// ---------------------------------------------------------------------------
// Alandra, Sky Dreamer
// ---------------------------------------------------------------------------
//
// Oracle: "Whenever you draw your second card each turn, create a 2/2
// blue Drake creature token with flying. Whenever you draw your fifth
// card each turn, Alandra and Drakes you control get +X/+X until end
// of turn, where X is the number of cards in your hand."
//
// First clause: gate on draw seat == controller AND seat.Turn.CardsDrawn
// == 2 at the moment the draw fired. The engine bumps CardsDrawn BEFORE
// firing the card_drawn trigger (see state.go:1794), so reading == 2
// here picks up exactly the second draw of the turn. Second clause
// (5th-draw mass +X/+X) is a Layer-7 continuous mod across the Drake
// tribe + self; emitPartial.
func alandraSkyDreamerOnDraw(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "alandra_sky_dreamer_second_draw_drake"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	seatIdx, _ := ctx["seat"].(int)
	if seatIdx != perm.Controller {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}
	if seat.Turn.CardsDrawn != 2 {
		return
	}
	tok := gameengine.CreateCreatureToken(gs, seatIdx, "Drake", []string{"creature", "drake"}, 2, 2)
	if tok == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "token_creation_failed", nil)
		return
	}
	if tok.Card != nil {
		tok.Card.Colors = []string{"U"}
	}
	if tok.Flags == nil {
		tok.Flags = map[string]int{}
	}
	tok.Flags["kw:flying"] = 1
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": seatIdx,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"fifth_draw_mass_plus_x_x_to_drakes_and_self_unimplemented_layer_7_continuous_mod")
}

// ---------------------------------------------------------------------------
// Tuvasa the Sunlit
// ---------------------------------------------------------------------------
//
// Oracle: "Tuvasa gets +1/+1 for each enchantment you control. Whenever
// you cast your first enchantment spell each turn, draw a card."
//
// Self-buff (+1/+1 per enchantment) is a CDA — Layer 7b dynamic-power
// set, like Adeline's creature-count CDA in gen_adeline_resplendent_cathar.go.
// Out of scope for this trigger-shape handler; emitPartial.
//
// First-enchantment-each-turn draw: stamp perm.Flags["tuvasa_drew_turn"] =
// gs.Turn+1 on first fire (+1 avoids 0-vs-unset collision on turn 0,
// the same pattern used by The Reaper, King No More's once-per-turn
// gate). Subsequent enchantment casts the same turn no-op.
func tuvasaTheSunlitOnCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "tuvasa_the_sunlit_first_enchantment_draw"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	caster, _ := ctx["caster_seat"].(int)
	if caster != perm.Controller {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil {
		return
	}
	isEnchant := false
	for _, t := range card.Types {
		if t == "enchantment" {
			isEnchant = true
			break
		}
	}
	if !isEnchant {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if perm.Flags["tuvasa_drew_turn"] == gs.Turn+1 {
		return
	}
	perm.Flags["tuvasa_drew_turn"] = gs.Turn + 1
	drawOne(gs, perm.Controller, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"spell": card.DisplayName(),
		"turn":  gs.Turn,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"plus_one_plus_one_per_enchantment_self_buff_unimplemented_layer_7b_cda")
}

// ---------------------------------------------------------------------------
// Grolnok, the Omnivore
// ---------------------------------------------------------------------------
//
// Oracle: "Whenever a Frog you control attacks, mill three cards.
// Whenever a permanent card is put into your graveyard from your
// library, exile it with a croak counter on it. You may play permanent
// spells with croak counters on them from exile."
//
// First clause: filter to creature attacker controlled by Grolnok's
// controller with the Frog subtype. Mill 3 via MoveCard library →
// graveyard (matches phenax_god_of_deception's mill primitive).
// Second + third clauses (croak-counter exile + zone-cast permission)
// need card_milled OR permanent-to-graveyard-from-library handler
// plus a ZoneCastPolicy/ZoneCastGrant — a separate batch's worth of
// engine plumbing; emitPartial breadcrumb here.
func grolnokTheOmnivoreOnFrogAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "grolnok_the_omnivore_frog_attack_mill"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk == nil || atk.Card == nil || atk.Controller != perm.Controller {
		return
	}
	if !atk.IsCreature() || !cardHasSubtype(atk.Card, "frog") {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	milled := 0
	for i := 0; i < 3; i++ {
		if len(seat.Library) == 0 {
			break
		}
		card := seat.Library[0]
		gameengine.MoveCard(gs, card, perm.Controller, "library", "graveyard", "grolnok_frog_attack_mill")
		milled++
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"attacker": atk.Card.DisplayName(),
		"milled":   milled,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"croak_counter_exile_on_perm_to_gy_from_library_plus_zone_cast_grant_unimplemented")
}
