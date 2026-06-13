package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// hallowed_burial_r60.go — per_card handler for Hallowed Burial.
//
// Oracle text (Scryfall / ast_dataset):
//
//	Put all creatures on the bottom of their owners' libraries.
//
// {4}{W}{W} Sorcery. The white "tuck-everything" board wipe — beats
// indestructible and regeneration (it's neither destruction nor
// damage), the twin of Terminus's resolution body. Parses to a single
// inert `parsed_effect_residual` node with no mass-bounce structure, so
// the board wipe did NOTHING (the text fallback has no "put all
// creatures on the bottom" shape — verified inert alongside Terminus).
//
// Implementation:
//   - OnResolve. Snapshots every creature across all battlefields FIRST
//     (BouncePermanent mutates the slices), then routes each through
//     BouncePermanent(..., "library_bottom") — the canonical
//     battlefield-exit (commander redirect, aura/equipment detach, token
//     cease, LTB triggers).
func init() {
	registerHallowedBurialR60(Global())
	AddResetHook(registerHallowedBurialR60)
}

func registerHallowedBurialR60(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Hallowed Burial", hallowedBurialResolve)
}

func hallowedBurialResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "hallowed_burial"
	if gs == nil || item == nil {
		return
	}
	var creatures []*gameengine.Permanent
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p != nil && p.IsCreature() {
				creatures = append(creatures, p)
			}
		}
	}
	tucked := 0
	for _, p := range creatures {
		if gameengine.BouncePermanent(gs, p, nil, "library_bottom") {
			tucked++
		}
	}
	gameengine.StateBasedActions(gs)
	emit(gs, slug, "Hallowed Burial", map[string]interface{}{
		"seat":   item.Controller,
		"tucked": tucked,
	})
}
