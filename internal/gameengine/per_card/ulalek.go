package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerUlalek wires Ulalek, Fused Atrocity.
//
// Oracle text:
//
//	Devoid
//	Whenever you cast an Eldrazi spell, you may pay {C}{C}. If you do,
//	copy that spell. You may choose new targets for the copy.
//
// Implementation:
//   - Listens on "spell_cast"; gates on caster_seat == controller and
//     the cast card having the "eldrazi" type.
//   - Optional cost {C}{C} ≈ 2 generic mana; we gate on ManaPool ≥ 2 so
//     the AI only fires when it has the floats. Auto-pay when affordable.
//   - Copy: deep-copy the cast card, mark IsCopy, and push a fresh
//     StackItem on top of the existing one — mirrors Krark's coin-flip
//     "win" branch and resolveCopySpell's CR §707.2 path.
//
// Devoid (the source-side colorless static) is a characteristic-defining
// ability handled at card load; no per-card hook needed.
func registerUlalek(r *Registry) {
	r.OnTrigger("Ulalek, Fused Atrocity", "spell_cast", ulalekCastTrigger)
}

func ulalekCastTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "ulalek_eldrazi_copy"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil {
		return
	}
	if !cardHasType(card, "eldrazi") {
		return
	}

	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}
	const cost = 2 // {C}{C}
	if seat.ManaPool < cost {
		emitFail(gs, slug, perm.Card.DisplayName(), "insufficient_colorless_mana", map[string]interface{}{
			"seat":      perm.Controller,
			"spell":     card.DisplayName(),
			"required":  cost,
			"available": seat.ManaPool,
		})
		return
	}

	// Find the spell's StackItem.
	var stackItem *gameengine.StackItem
	for i := len(gs.Stack) - 1; i >= 0; i-- {
		si := gs.Stack[i]
		if si == nil || si.Card != card {
			continue
		}
		stackItem = si
		break
	}
	if stackItem == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "spell_not_on_stack", map[string]interface{}{
			"spell": card.DisplayName(),
		})
		return
	}

	seat.ManaPool -= cost
	gs.LogEvent(gameengine.Event{
		Kind:   "pay_mana",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Amount: cost,
		Details: map[string]interface{}{
			"reason": "ulalek_copy_eldrazi",
			"colorless": cost,
		},
	})

	copyCard := card.DeepCopy()
	copyCard.IsCopy = true
	copyItem := &gameengine.StackItem{
		Controller: perm.Controller,
		Card:       copyCard,
		Effect:     stackItem.Effect,
		Kind:       stackItem.Kind,
		IsCopy:     true,
	}
	if len(stackItem.Targets) > 0 {
		copyItem.Targets = append([]gameengine.Target(nil), stackItem.Targets...)
	}
	gameengine.PushStackItem(gs, copyItem)
	gs.LogEvent(gameengine.Event{
		Kind:   "copy_spell",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"slug":    slug,
			"copied":  card.DisplayName(),
			"is_copy": true,
			"rule":    "707.2",
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"spell":     card.DisplayName(),
		"cost_paid": cost,
		"copied":    true,
	})
}
