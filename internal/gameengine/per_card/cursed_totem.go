package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerCursedTotem wires up Cursed Totem.
//
// Oracle text:
//
//	Activated abilities of creatures can't be activated.
//
// The creature-side counterpart to Null Rod: shuts off mana dorks
// (Birds of Paradise, Noble Hierarch, Llanowar Elves), Walking
// Ballista's ping, Razaketh's tutor, Thrasios/Rograkh activations,
// and most planeswalker-esque creature loyalty clauses. Like Null
// Rod, this DOES NOT exempt mana abilities — Cursed Totem is stricter
// (shuts off Birds' {T}: add G, which IS a mana ability).
//
// Oracle-correct subtlety: "activated abilities of creatures" includes
// planeswalkers' loyalty abilities that READ as activated on creatures
// (e.g. Grist, the Hunger Tide in the graveyard mode). It does NOT
// shut off static or triggered abilities.
//
// Batch #3 scope:
//   - OnETB: stamp gs.Flags["cursed_totem_count"]++
//   - CursedTotemSuppresses helper for activation-dispatch callers.
func registerCursedTotem(r *Registry) {
	r.OnETB("Cursed Totem", cursedTotemETB)
}

func cursedTotemETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "cursed_totem_static"
	if gs == nil || perm == nil {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["cursed_totem_count"]++
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"count":      gs.Flags["cursed_totem_count"],
		"suppresses": "creature_activated_including_mana_abilities",
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"activation_dispatch_must_consult_CursedTotemSuppresses_at_activation_time")
}

// CursedTotemActive returns true if at least one Cursed Totem is on
// the battlefield.
func CursedTotemActive(gs *gameengine.GameState) bool {
	if gs == nil || gs.Flags == nil {
		return false
	}
	return gs.Flags["cursed_totem_count"] > 0
}

// CursedTotemSuppresses reports whether a given permanent's activated
// ability is suppressed by Cursed Totem. Suppresses ALL creature
// activated abilities — including mana abilities. Returns true only
// when a Totem is active AND the permanent is a creature.
func CursedTotemSuppresses(gs *gameengine.GameState, perm *gameengine.Permanent) bool {
	if !CursedTotemActive(gs) || perm == nil {
		return false
	}
	return perm.IsCreature()
}
