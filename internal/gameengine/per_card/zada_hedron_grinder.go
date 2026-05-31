package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerZadaHedronGrinder wires Zada, Hedron Grinder.
//
// Oracle text:
//
//	Whenever you cast an instant or sorcery spell that targets only Zada,
//	copy that spell for each other creature you control that the spell
//	could target. Each copy targets a different one of those creatures.
//
// Implementation:
//   - "spell_cast" gated on caster_seat == perm.Controller, instant or
//     sorcery type, single target == Zada. We then deep-copy the
//     StackItem once per other creature we control (best-effort) and
//     push above the original. Targeting validity is approximated as
//     "any creature we control" — proper "could target" requires AST
//     target predicate evaluation; emitPartial flags this.
func registerZadaHedronGrinder(r *Registry) {
	r.OnTrigger("Zada, Hedron Grinder", "spell_cast", zadaSpellCast)
}

func zadaSpellCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "zada_hedron_grinder_copy"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil || card == perm.Card {
		return
	}
	if !cardHasType(card, "instant") && !cardHasType(card, "sorcery") {
		return
	}

	// Locate the original StackItem; verify it has exactly one target = Zada.
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
		return
	}
	if len(stackItem.Targets) != 1 {
		return
	}
	if stackItem.Targets[0].Permanent != perm {
		return
	}

	// Find other creatures we control to retarget the copies onto.
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	var others []*gameengine.Permanent
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || !p.IsCreature() {
			continue
		}
		others = append(others, p)
	}
	if len(others) == 0 {
		return
	}

	emitPartial(gs, slug, perm.Card.DisplayName(), "could_target_predicate_approximated_as_any_creature_we_control")

	for _, t := range others {
		copyCard := gameengine.MintSpellCopy(gs, card)
		copyItem := &gameengine.StackItem{
			Controller: perm.Controller,
			Card:       copyCard,
			Effect:     stackItem.Effect,
			Kind:       stackItem.Kind,
			IsCopy:     true,
			Targets:    []gameengine.Target{{Kind: gameengine.TargetKindPermanent, Permanent: t}},
		}
		gameengine.PushStackItem(gs, copyItem)
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"copies": len(others),
		"spell":  card.DisplayName(),
	})
}
