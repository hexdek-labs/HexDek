package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMendicantCoreGuidelightCustom wires Mendicant Core's max-speed
// artifact-spell copy payload. The auto-generated handler in
// gen_mendicant_core_guidelight.go covers the CDA power, speed counter,
// and detects the max-speed cast event; the partial it emits is exactly
// the "actual spell-copy via the engine's spell-copy pipeline" gap that
// this file closes.
//
// "Max speed — Whenever you cast an artifact spell, you may pay {1}.
// If you do, copy it." We always pay the {1} when mana is available
// (the AI heuristic — paying for a copy on the resolving spell is
// almost always a value win). The copy is pushed onto the stack via
// PushStackItem as a fresh StackItem with IsCopy=true, mirroring the
// shape of gameengine/resolve.go resolveCopySpell.
func registerMendicantCoreGuidelightCustom(r *Registry) {
	r.OnTrigger("Mendicant Core, Guidelight", "spell_cast", mendicantMaxSpeedCopy)
}

func mendicantMaxSpeedCopy(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "mendicant_max_speed_copy_artifact_apply"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	if perm.Flags == nil || perm.Flags["speed"] < 4 {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil || !cardHasType(card, "artifact") {
		return
	}
	// Pay the {1}: drain one from the controller's mana pool. If the
	// controller has zero mana available, we skip — the printed text is
	// optional ("you MAY pay {1}").
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}
	if seat.ManaPool < 1 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"copied":   false,
			"declined": "insufficient_mana",
		})
		return
	}
	seat.ManaPool -= 1

	// Locate the cast spell on the stack (top item should match by
	// pointer or by card identity). Mirrors resolveCopySpell's scan.
	var target *gameengine.StackItem
	for i := len(gs.Stack) - 1; i >= 0; i-- {
		item := gs.Stack[i]
		if item == nil || item.Countered || item.Card == nil {
			continue
		}
		if item.Card == card || item.Card.DisplayName() == card.DisplayName() {
			target = item
			break
		}
	}
	if target == nil {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"copied":   false,
			"declined": "spell_not_on_stack",
		})
		return
	}

	copyCard := gameengine.MintSpellCopy(gs, target.Card)
	copyItem := &gameengine.StackItem{
		Controller: target.Controller,
		Card:       copyCard,
		Effect:     target.Effect,
		IsCopy:     true,
		Kind:       target.Kind,
	}
	if len(target.Targets) > 0 {
		copyItem.Targets = append([]gameengine.Target(nil), target.Targets...)
	}
	gameengine.PushStackItem(gs, copyItem)

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"copied":     true,
		"source":     target.Card.DisplayName(),
		"controller": target.Controller,
	})
}

