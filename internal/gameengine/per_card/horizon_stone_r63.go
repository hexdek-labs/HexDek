package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Horizon Stone — {3}, Artifact
//
// Oracle (Scryfall, verified via hexdek.dev/api/oracle/card/Horizon Stone):
//
//	If you would lose unspent mana, that mana becomes colorless instead.
//
// This is the same replacement effect as Kruphix, God of Horizons'
// fourth line. It was previously unmodeled (no per_card handler; no
// generic AST path emits a "becomes colorless instead of lost"
// converter). Wired in r63 alongside the engine
// ManaPoolColorlessConverter primitive.
//
// Semantics: at every step/phase boundary (CR §500.4 / §106.4), the
// controller's would-be-lost unspent mana is RECOLORED to {C} and kept
// in the pool, rather than emptied. So a floated {R}{G}{U} reads as
// three colorless after the boundary. Distinct from the Omnath/Leyline
// Tyrant retention exemptions, which keep mana in its original color.
//
// Horizon Stone affects only its controller's pool (CR: "If you would
// lose unspent mana" refers to the permanent's controller), so the
// converter is controller-scoped.

func registerHorizonStone(r *Registry) {
	r.OnETB("Horizon Stone", horizonStoneETB)
	r.OnTrigger("Horizon Stone", "permanent_ltb", horizonStoneLTB)
}

func horizonStoneETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "horizon_stone_unspent_to_colorless"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	gameengine.RegisterManaColorlessConverter(gs, perm, perm.Controller)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":    perm.Controller,
		"convert": "colorless",
	})
}

func horizonStoneLTB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	gameengine.UnregisterManaColorlessConverterForPerm(gs, perm)
}
