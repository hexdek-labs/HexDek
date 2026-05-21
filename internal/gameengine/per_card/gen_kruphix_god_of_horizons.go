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
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           perm.Controller,
		"unspent_to_col": 1,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"unspent-mana-becomes-colorless replacement needs ManaEmpty hook; flag set + end-step convert-to-colorless implemented as approximation")
}

func kruphixEndStepConvertMana(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	// Kruphix's mana conversion is "if you would lose" so it fires at the
	// per-phase mana empty step. As an approximation we run at end_step.
	// The engine clears mana pool to 0 at phase end; we don't need to
	// "convert" since there's no color-tracking pool — this hook is here
	// for parity with the partial breadcrumb. Future engine support will
	// replace it with a proper ManaEmpty hook.
	_ = perm
	_ = ctx
}
