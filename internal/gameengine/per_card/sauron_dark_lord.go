package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSauronDarkLord wires Sauron, the Dark Lord.
//
// Oracle text (Scryfall, LTR — verified 2026-04-30):
//
//	Ward—Sacrifice a legendary artifact or legendary creature.
//	Whenever an opponent casts a spell, amass Orcs 1.
//	Whenever an Army you control deals combat damage to a player,
//	the Ring tempts you.
//	Whenever the Ring tempts you, you may discard your hand. If you
//	do, draw four cards.
//
// Note: the user-facing task description had Sauron's amass on ETB and
// the Ring trigger on Sauron attacks; printed oracle puts amass on
// opponent spell-cast and Ring tempt on Army combat damage. We follow
// the printed oracle (Scryfall is authoritative).
//
// Implementation:
//   - ETB: emitPartial for the non-mana ward payment (engine ward grants
//     only model mana costs cleanly, matching auntie_ool's blight ward).
//   - "spell_cast_by_opponent" trigger: amass Orcs 1 for Sauron's
//     controller.
//   - "combat_damage_player" trigger: when an Army Sauron's controller
//     controls deals combat damage to a player, the Ring tempts Sauron's
//     controller AND we apply Sauron's own "may discard hand, draw 4"
//     reaction inline (the discard-draw clause is a separate trigger
//     that the engine doesn't fire as a per-card event for ring tempts;
//     emitPartial flags the gap for ring tempts from other sources).
func registerSauronDarkLord(r *Registry) {
	r.OnETB("Sauron, the Dark Lord", sauronETB)
	r.OnTrigger("Sauron, the Dark Lord", "spell_cast_by_opponent", sauronOpponentSpell)
	r.OnTrigger("Sauron, the Dark Lord", "combat_damage_player", sauronArmyCombatDamage)
}

func sauronETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "sauron_ward_legendary_sacrifice"
	if gs == nil || perm == nil {
		return
	}
	// R60 closure of half-finished-features-r48 #4: stamp the alt-
	// payment ward kind so CheckWardOnTargeting routes through the
	// canonical sacrifice-a-legendary handler (ward_alt_payment.go).
	// Replaces the previous "ward_sacrifice_legendary_alt_payment_
	// unimplemented" emitPartial.
	// r60 — migrated 2026-05-27 to unified WardCost primitive. Filter
	// "legendary artifact or creature" matches Sauron's literal printed
	// wording; payWardBySacrifice routes accordingly.
	gameengine.SetWardCost(perm, gameengine.WardCost{
		Type:   gameengine.WardCostSacrifice,
		Amount: 1,
		Filter: "legendary artifact or creature",
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"ward_alt_kind": "sacrifice_legendary",
	})
	// Out-of-scope gap (NOT half-finished #4): Ring tempts from sources
	// other than Sauron's own combat-damage trigger don't dispatch the
	// "discard hand → draw 4" reaction. The Ring infrastructure fires
	// TheRingTemptsYou but doesn't fan a per-card ring_tempt trigger,
	// so this card has no way to observe tempts from other Army-bearers.
	// Kept as a partial so a future ring_tempt-trigger PR can find it.
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"ring_tempts_from_other_sources_does_not_dispatch_discard_draw_four")
}

func sauronOpponentSpell(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "sauron_opponent_spell_amass"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat == perm.Controller {
		return
	}
	amassOrcs(gs, perm.Controller)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        perm.Controller,
		"caster_seat": casterSeat,
	})
}

func sauronArmyCombatDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "sauron_army_combat_damage_ring_tempt"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	srcSeat, _ := ctx["source_seat"].(int)
	if srcSeat != perm.Controller {
		return
	}
	srcName, _ := ctx["source_card"].(string)
	if !controllerOwnsArmyNamed(gs, perm.Controller, srcName) {
		return
	}

	gameengine.TheRingTemptsYou(gs, perm.Controller)
	sauronDiscardHandDrawFour(gs, perm)

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        perm.Controller,
		"source_card": srcName,
	})
}

// sauronDiscardHandDrawFour applies the "Whenever the Ring tempts you,
// you may discard your hand. If you do, draw four cards." clause for
// Sauron's controller. The hat always opts in when discarding nets cards
// (i.e. we have fewer than 4 in hand) — heuristic mirrors the standard
// "discard for value" decision.
func sauronDiscardHandDrawFour(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "sauron_ring_tempt_discard_draw"
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}
	if len(s.Hand) >= 4 {
		emitFail(gs, slug, perm.Card.DisplayName(), "hand_too_full_to_net_value", map[string]interface{}{
			"seat":      seat,
			"hand_size": len(s.Hand),
		})
		return
	}
	discarded := len(s.Hand)
	hand := append([]*gameengine.Card(nil), s.Hand...)
	for _, c := range hand {
		if c == nil {
			continue
		}
		gameengine.DiscardCard(gs, c, seat)
	}
	drawn := 0
	for i := 0; i < 4; i++ {
		if c := drawOne(gs, seat, perm.Card.DisplayName()); c != nil {
			drawn++
		}
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      seat,
		"discarded": discarded,
		"drawn":     drawn,
	})
}

// controllerOwnsArmyNamed returns true if seat controls a permanent of
// type Army whose card display name matches sourceName.
func controllerOwnsArmyNamed(gs *gameengine.GameState, seat int, sourceName string) bool {
	if gs == nil || seat < 0 || seat >= len(gs.Seats) || sourceName == "" {
		return false
	}
	s := gs.Seats[seat]
	if s == nil {
		return false
	}
	want := strings.ToLower(sourceName)
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if strings.ToLower(p.Card.DisplayName()) != want {
			continue
		}
		for _, t := range p.Card.Types {
			if strings.EqualFold(t, "army") {
				return true
			}
		}
	}
	return false
}
