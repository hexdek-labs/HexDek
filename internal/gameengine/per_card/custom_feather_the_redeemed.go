package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerFeatherTheRedeemedCustom replaces the auto-generated stub
// with a real implementation of Feather's exile-and-return trigger.
//
// Oracle text:
//
//	Flying
//	Whenever you cast an instant or sorcery spell that targets a
//	creature you control, exile that card instead of putting it into
//	your graveyard as it resolves. If you do, return it to your hand
//	at the beginning of the next end step.
//
// Implementation:
//   - OnTrigger("instant_or_sorcery_cast"): the engine fires this when
//     an instant/sorcery hits the stack with ctx["card"] = the spell
//     card, ctx["caster_seat"] = controller, and the StackItem
//     accessible by walking gs.Stack for the matching card.
//   - We check that the caster is Feather's controller and the spell
//     targets at least one creature that controller controls (via
//     StackItem.Targets). When both hold, we stamp
//     CostMeta["exile_on_resolve"] = true so the engine's existing
//     post-resolve replacement (zone_cast.go honoring the flag) sends
//     the card to exile rather than graveyard.
//   - We register a delayed "next_end_step" trigger that returns the
//     exiled card to its owner's hand. The card pointer is captured in
//     the closure so we don't depend on name-based lookup, which is
//     unreliable for tokens / copies.
func registerFeatherTheRedeemedCustom(r *Registry) {
	r.OnTrigger("Feather, the Redeemed", "instant_or_sorcery_cast", featherExileAndReturn)
}

func featherExileAndReturn(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "feather_exile_and_return"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	spellCard, _ := ctx["card"].(*gameengine.Card)
	if spellCard == nil {
		return
	}

	// Locate the StackItem so we can read its targets and stamp the
	// exile-on-resolve flag.
	var item *gameengine.StackItem
	if si, ok := ctx["stack_item"].(*gameengine.StackItem); ok && si != nil {
		item = si
	} else {
		for i := len(gs.Stack) - 1; i >= 0; i-- {
			s := gs.Stack[i]
			if s != nil && s.Card == spellCard {
				item = s
				break
			}
		}
	}
	if item == nil {
		emitPartial(gs, slug, perm.Card.DisplayName(),
			"stack_item_not_found_in_ctx_or_stack")
		return
	}

	// "Targets a creature you control" — scan resolved targets for at
	// least one creature owned by Feather's controller.
	hits := false
	for _, t := range item.Targets {
		if t.Kind != gameengine.TargetKindPermanent || t.Permanent == nil {
			continue
		}
		if !t.Permanent.IsCreature() {
			continue
		}
		if t.Permanent.Controller != perm.Controller {
			continue
		}
		hits = true
		break
	}
	if !hits {
		return
	}

	if item.CostMeta == nil {
		item.CostMeta = map[string]interface{}{}
	}
	item.CostMeta["exile_on_resolve"] = true

	captured := spellCard
	owner := perm.Controller
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "next_end_step",
		ControllerSeat: owner,
		SourceCardName: perm.Card.DisplayName(),
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			if captured == nil {
				return
			}
			seat := gs.Seats[owner]
			if seat == nil {
				return
			}
			// Pull from exile if the engine's exile-on-resolve fired.
			// Wave 2 multi-step migration: route through MoveCard so
			// §614 replacements + §903.9b commander redirect +
			// zone_change observers fire. Pre-r60 spliced + manual
			// append, bypassing all of the above.
			for _, c := range seat.Exile {
				if c == captured {
					gameengine.MoveCard(gs, captured, owner, "exile", "hand", "feather_return_to_hand")
					gs.LogEvent(gameengine.Event{
						Kind:   "feather_return_to_hand",
						Seat:   owner,
						Source: "Feather, the Redeemed",
						Details: map[string]interface{}{
							"card": captured.DisplayName(),
						},
					})
					return
				}
			}
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"spell": spellCard.DisplayName(),
	})
}
