package per_card

import (
	"fmt"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerPantlaza wires Pantlaza, Sun-Favored.
//
// Oracle text:
//
//	Whenever Pantlaza, Sun-Favored or another Dinosaur you control
//	enters, you may discover X, where X is that creature's toughness.
//	Do this only once each turn.
//
// Implementation:
//   - "permanent_etb" trigger: filter to creatures of type Dinosaur whose
//     entering controller equals Pantlaza's controller. Discover the
//     entering creature's current toughness.
//   - Once-per-turn gate via perm.Flags["pantlaza_discover_turn"]; AI
//     greedily takes the first eligible ETB per turn (toughness-X discover
//     value monotonically rises with toughness, but the only-once-per-turn
//     restriction means later/bigger ETBs in the same turn are skipped).
func registerPantlaza(r *Registry) {
	r.OnTrigger("Pantlaza, Sun-Favored", "permanent_etb", pantlazaETBTrigger)
}

func pantlazaETBTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "pantlaza_discover_on_dino_etb"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	entering, _ := ctx["perm"].(*gameengine.Permanent)
	if entering == nil || entering.Card == nil {
		return
	}
	enteringSeat, _ := ctx["controller_seat"].(int)
	if enteringSeat != perm.Controller {
		return
	}
	if !entering.IsCreature() {
		return
	}
	if !cardHasType(entering.Card, "dinosaur") {
		return
	}

	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	// Sentinel = gs.Turn+1 so a 0 default means "never fired" even on turn 0.
	if perm.Flags["pantlaza_discover_turn"] == gs.Turn+1 {
		return
	}
	perm.Flags["pantlaza_discover_turn"] = gs.Turn + 1

	x := entering.Toughness()
	if x < 0 {
		x = 0
	}

	found := gameengine.PerformDiscover(gs, perm.Controller, x)
	discovered := ""
	if found != nil {
		discovered = found.DisplayName()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":              perm.Controller,
		"trigger_source":    entering.Card.DisplayName(),
		"discover_x":        x,
		"discovered":        discovered,
		"once_per_turn_key": fmt.Sprintf("turn_%d", gs.Turn),
	})
}
