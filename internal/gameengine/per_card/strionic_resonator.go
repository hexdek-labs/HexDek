package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerStrionicResonator wires up Strionic Resonator.
//
// Oracle text:
//
//	{2}, {T}: Copy target triggered ability you control. You may
//	choose new targets for the copy.
//
// This is a "copy triggered ability" primitive. The engine's triggered
// abilities are pushed onto the stack via PushTriggeredAbility. To
// copy one, we push a second copy of the same ability onto the stack.
//
// Implementation:
//   - OnActivated: find the most recent triggered ability on the stack
//     that the controller owns, and push a copy of it.
func registerStrionicResonator(r *Registry) {
	r.OnActivated("Strionic Resonator", strionicResonatorActivated)
}

func strionicResonatorActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "strionic_resonator_copy_trigger"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller

	// Pay {2} + tap.
	s := gs.Seats[seat]
	if s.ManaPool < 2 {
		emitFail(gs, slug, "Strionic Resonator", "insufficient_mana", nil)
		return
	}
	if src.Tapped {
		emitFail(gs, slug, "Strionic Resonator", "already_tapped", nil)
		return
	}
	s.ManaPool -= 2
	gameengine.SyncManaAfterSpend(s)
	src.Tapped = true

	// Find the most recent triggered ability on the stack that we control.
	var targetItem *gameengine.StackItem
	for i := len(gs.Stack) - 1; i >= 0; i-- {
		item := gs.Stack[i]
		if item == nil || item.Controller != seat {
			continue
		}
		if item.Kind == "triggered" || (item.Kind == "" && item.Source != nil && item.Card == nil) {
			targetItem = item
			break
		}
	}
	if targetItem == nil {
		emitFail(gs, slug, "Strionic Resonator", "no_triggered_ability_on_stack", nil)
		return
	}

	// Push a copy of the triggered ability onto the stack.
	copy := &gameengine.StackItem{
		Controller: targetItem.Controller,
		Card:       targetItem.Card,
		Source:     targetItem.Source,
		Effect:     targetItem.Effect,
		Targets:    append([]gameengine.Target(nil), targetItem.Targets...),
		Kind:       "triggered",
		IsCopy:     true,
	}
	gameengine.PushStackItem(gs, copy)

	sourceName := ""
	if targetItem.Source != nil && targetItem.Source.Card != nil {
		sourceName = targetItem.Source.Card.DisplayName()
	}

	emit(gs, slug, "Strionic Resonator", map[string]interface{}{
		"seat":             seat,
		"copied_trigger":   sourceName,
		"stack_item_id":    targetItem.ID,
	})
}
