package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerRootha wires Rootha, Mastering the Moment.
//
// Oracle text:
//
//	{1}{U}{R}: Copy target instant or sorcery you control. You may
//	choose new targets for the copy. Return Rootha, Mastering the
//	Moment to its owner's hand.
//
// Implementation:
//   - Single activated ability (abilityIdx 0). Mana cost is paid by the
//     engine activation pipeline.
//   - Target selection: prefer ctx["target_stack_item"] / ctx["target_card"]
//     when the activation pipeline supplied one; otherwise pick the
//     topmost instant/sorcery on the stack controlled by Rootha's
//     controller. If nothing eligible is on the stack, the ability fizzles
//     (per CR §608.2b — illegal targets cause the ability to be removed
//     from the stack on resolution; we still bounce Rootha because that
//     part of the effect is non-target).
//   - Copy: deep-copy the spell, mark IsCopy, push a fresh StackItem
//     above the original. Mirrors krark.go / ulalek.go.
//   - Bounce: route Rootha to owner's hand via BouncePermanent so §614
//     replacements and §903.9b commander redirect fire correctly.
func registerRootha(r *Registry) {
	r.OnActivated("Rootha, Mastering the Moment", roothaActivated)
}

func roothaActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "rootha_mastering_the_moment_copy_bounce"
	if gs == nil || src == nil {
		return
	}
	if abilityIdx != 0 {
		return
	}

	// Find the target spell on the stack.
	var stackItem *gameengine.StackItem
	if ctx != nil {
		if si, ok := ctx["target_stack_item"].(*gameengine.StackItem); ok && si != nil {
			stackItem = si
		} else if tc, ok := ctx["target_card"].(*gameengine.Card); ok && tc != nil {
			for i := len(gs.Stack) - 1; i >= 0; i-- {
				if gs.Stack[i] != nil && gs.Stack[i].Card == tc {
					stackItem = gs.Stack[i]
					break
				}
			}
		}
	}
	if stackItem == nil {
		// Fall back to the topmost legal target controlled by Rootha's
		// controller on the stack.
		for i := len(gs.Stack) - 1; i >= 0; i-- {
			si := gs.Stack[i]
			if si == nil || si.Card == nil {
				continue
			}
			if si.Controller != src.Controller {
				continue
			}
			if !cardHasType(si.Card, "instant") && !cardHasType(si.Card, "sorcery") {
				continue
			}
			stackItem = si
			break
		}
	}

	copied := false
	if stackItem != nil && stackItem.Card != nil {
		copyCard := stackItem.Card.DeepCopy()
		copyCard.IsCopy = true
		copyItem := &gameengine.StackItem{
			Controller: src.Controller,
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
			Seat:   src.Controller,
			Source: src.Card.DisplayName(),
			Details: map[string]interface{}{
				"slug":    slug,
				"copied":  stackItem.Card.DisplayName(),
				"is_copy": true,
				"rule":    "707.2",
			},
		})
		copied = true
	} else {
		emitFail(gs, slug, src.Card.DisplayName(), "no_legal_instant_or_sorcery_on_stack", map[string]interface{}{
			"seat": src.Controller,
		})
	}

	bounced := gameengine.BouncePermanent(gs, src, src, "hand")

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":    src.Controller,
		"copied":  copied,
		"bounced": bounced,
	})
}
