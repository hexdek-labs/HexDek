package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKruphixGodOfHorizons wires Kruphix, God of Horizons.
//
// Oracle text:
//
//	Indestructible
//	As long as your devotion to green and blue is less than seven,
//	Kruphix isn't a creature.
//	You have no maximum hand size.
//	If you would lose unspent mana, that mana becomes colorless instead.
//
// Implementation:
//   - Indestructible + god-creature toggle: AST-keyword pipeline owns
//     these; we just set the runtime flag for fast-path consumers.
//   - "No maximum hand size": set the same gs.Flags["no_max_hand_size_seat_N"]
//     marker that Reliquary Tower / Thought Vessel use, so the cleanup-step
//     check finds it on the same path.
//   - "Unspent mana becomes colorless instead of lost": the engine's
//     mana-pool empty path doesn't yet expose a replacement hook.
//     Set a per-seat flag and emit a partial.
func registerKruphixGodOfHorizons(r *Registry) {
	r.OnETB("Kruphix, God of Horizons", kruphixETBSetSeatFlags)
	r.OnTrigger("Kruphix, God of Horizons", "end_step", kruphixEndStepConvertMana)
	// R50 batch G: LTB hook clears the seat + game flags so a
	// stolen / exiled Kruphix doesn't leave permanent no-max-hand-size
	// + unspent-mana-to-colorless effects on the controller.
	r.OnTrigger("Kruphix, God of Horizons", "permanent_ltb", kruphixLTBClearFlags)
}

func kruphixLTBClearFlags(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	if gs.Flags != nil {
		delete(gs.Flags, "no_max_hand_size_seat_"+intToStr(perm.Controller))
	}
	seat := gs.Seats[perm.Controller]
	if seat != nil && seat.Flags != nil {
		delete(seat.Flags, "kruphix_unspent_mana_to_colorless")
	}
	// R63: drop the colorless-conversion effect registered at ETB.
	gameengine.UnregisterManaColorlessConverterForPerm(gs, perm)
}

func kruphixETBSetSeatFlags(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "kruphix_god_of_horizons_etb_flags"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["no_max_hand_size_seat_"+intToStr(perm.Controller)] = 1
	seat.Flags["kruphix_unspent_mana_to_colorless"] = 1
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["kw:indestructible"] = 1
	// R63: register the "becomes colorless instead of being lost"
	// converter (replaces the R55/R57 all-color exemption stand-in).
	// Unlike an exemption — which retained each color in its own bucket —
	// the converter recolors all would-be-lost unspent mana to {C} at the
	// §106.4 boundary and keeps it. So a floated {R}{G}{U} reads as three
	// colorless after a step/phase end, matching Kruphix's printed text.
	gameengine.RegisterManaColorlessConverter(gs, perm, perm.Controller)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           perm.Controller,
		"unspent_to_col": 1,
		"convert":        "colorless",
	})
}

func kruphixEndStepConvertMana(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	// R63: the "becomes colorless instead of lost" conversion is now done
	// by the engine at EVERY step/phase boundary inside DrainAllPools (via
	// the ManaPoolColorlessConverter registered at ETB), not just at end
	// step — matching CR §500.4. This end_step hook is retained as a
	// harmless no-op for trigger-dispatch parity.
	_ = perm
	_ = ctx
}
