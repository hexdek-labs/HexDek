package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAnimarSoulOfElements wires Animar, Soul of Elements.
//
// Oracle text:
//
//	Protection from white and from black
//	Whenever you cast a creature spell, put a +1/+1 counter on
//	Animar, Soul of Elements.
//	Creature spells you cast cost {1} less to cast for each +1/+1
//	counter on Animar, Soul of Elements.
//
// Implementation:
//   - OnETB: stamp prot:W and prot:B on Animar's Permanent.Flags so
//     combat.go's protectionColors() recognizes them. The AST keyword
//     reader also covers this if Card.AST is loaded; setting the flags
//     guarantees coverage in test fixtures and synthetic decks.
//   - OnTrigger("creature_spell_cast"): add a +1/+1 counter when the
//     spell is cast by Animar's controller. Uses the engine's CountersAdd
//     surface so observers (proliferate triggers, etc.) see the event.
//   - Cost reduction: handled in cost_modifiers.go via a battlefield
//     scan keyed on "Animar, Soul of Elements" — counters on Animar
//     produce a CostModReduction equal to the +1/+1 counter total for
//     each creature spell cast by Animar's controller.
func registerAnimarSoulOfElements(r *Registry) {
	r.OnETB("Animar, Soul of Elements", animarSoulOfElementsETB)
	r.OnTrigger("Animar, Soul of Elements", "creature_spell_cast", animarSoulOfElementsCast)
}

func animarSoulOfElementsETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "animar_soul_of_elements_protection"
	if gs == nil || perm == nil {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["prot:W"] = 1
	perm.Flags["prot:B"] = 1
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"protection": []string{"W", "B"},
	})
}

func animarSoulOfElementsCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "animar_soul_of_elements_growth"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil || !cardHasType(card, "creature") {
		return
	}
	// "Whenever you cast a creature spell" — Animar herself is a creature
	// spell, but she doesn't trigger for her own cast because the trigger
	// reads the battlefield and Animar isn't there until she resolves.
	// fireTrigger already filters this for us.
	perm.AddCounter("+1/+1", 1)
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"cast_spell": card.DisplayName(),
		"counters":   perm.Counters["+1/+1"],
	})
}
