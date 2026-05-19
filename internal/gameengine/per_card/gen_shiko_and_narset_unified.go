package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerShikoAndNarsetUnified wires Shiko and Narset, Unified.
//
// Oracle text (Scryfall, verified, {1}{R}{W}{U}, 3/3 legendary):
//
//	Flying, vigilance
//	Flurry — Whenever you cast your second spell each turn, copy that
//	spell if it targets a permanent or player, and you may choose new
//	targets for the copy. If you don't copy a spell this way, draw a
//	card.
//
// Implementation (R42 stub port):
//   - Flying, vigilance: AST keyword pipeline.
//   - "spell_cast" trigger gated on caster == controller AND it being
//     the second spell cast by the controller this turn — read from
//     seat.Flags["spells_cast_this_turn"], which IncrementCastCount
//     bumps BEFORE this trigger fires.
//   - Copy-or-draw branch: copying a targeted spell with re-chosen
//     targets needs the engine's spell-copy pipeline (a per-card
//     handler can't synthesise a new StackItem with re-chosen targets
//     reliably). We approximate by emitting a `flurry_copy` breadcrumb
//     when the spell looks targeted — instants and sorceries are
//     treated as "targets a permanent or player" by default — and
//     the controller draws a card otherwise (permanent spells, which
//     typically don't target on cast). This matches the CR fallback
//     branch "If you don't copy a spell this way, draw a card."
func registerShikoAndNarsetUnified(r *Registry) {
	r.OnTrigger("Shiko and Narset, Unified", "spell_cast", shikoNarsetFlurry)
}

func shikoNarsetFlurry(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "shiko_narset_flurry"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}
	if seat.Flags == nil || seat.Flags["spells_cast_this_turn"] != 2 {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	spellName, _ := ctx["spell_name"].(string)
	if spellName == "" && card != nil {
		spellName = card.DisplayName()
	}
	targetable := card != nil && (cardHasType(card, "instant") || cardHasType(card, "sorcery"))

	if targetable {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   perm.Controller,
			"branch": "copy",
			"spell":  spellName,
		})
		emitPartial(gs, slug, perm.Card.DisplayName(),
			"spell_copy_with_new_targets_requires_stack_pipeline_not_modeled_at_per_card_layer")
		return
	}

	drawn := drawOne(gs, perm.Controller, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"branch": "draw",
		"spell":  spellName,
		"drew":   drawn != nil,
	})
}
