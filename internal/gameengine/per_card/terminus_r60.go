package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// terminus_r60.go — per_card handler for Terminus.
//
// Oracle text (Scryfall / ast_dataset):
//
//	Put all creatures on the bottom of their owners' libraries.
//	Miracle {W}
//
// {4}{W}{W} Sorcery. A premier white board wipe — the library-tuck
// answer that beats indestructible (Avacyn, Darksteel) and regeneration
// because it's neither destruction nor damage. Parses to a
// `parsed_effect_residual` raw-text node ("put all creatures on the
// bottom of their owners' libraries") with no structured mass-bounce
// node, so the generic dispatch logged it inert — the board wipe did
// NOTHING. (The miracle cost is a casting modifier handled elsewhere;
// this handler is the resolution body.)
//
// Implementation:
//   - OnResolve. Snapshots every creature permanent across all seats'
//     battlefields FIRST (BouncePermanent mutates the battlefield
//     slices, so iterating live would skip entries), then routes each
//     through BouncePermanent(..., "library_bottom"). BouncePermanent
//     runs §903.9b commander redirect, aura/equipment detach, token
//     cease, and fires LTB triggers — exactly the canonical
//     battlefield-exit machinery the CLAUDE.md zone-leak post-mortems
//     mandate (vs. a manual removePermanent that leaks stale refs).
func init() {
	registerTerminusR60(Global())
	AddResetHook(registerTerminusR60)
}

func registerTerminusR60(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Terminus", terminusResolve)
}

func terminusResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "terminus"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	// Snapshot all creatures before mutating any battlefield.
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

	emit(gs, slug, "Terminus", map[string]interface{}{
		"seat":   seat,
		"tucked": tucked,
	})
}
