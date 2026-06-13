package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Leyline Tyrant — {2}{R}{R}, Creature — Dragon, 4/4
//
// Oracle (Scryfall, verified via hexdek.dev/api/oracle/card/Leyline Tyrant):
//
//	Flying
//	You don't lose unspent red mana as steps and phases end.
//	When this creature dies, you may pay any amount of {R}. When you do,
//	it deals that much damage to any target.
//
// Two mana-pool surfaces, both previously unimplemented (no per_card
// handler existed and no generic AST path registers a colored "you don't
// lose unspent X mana" exemption):
//
//  1. ETB registers a controller-scoped RED ManaPoolExemption so floated
//     red survives the §106.4 step/phase-end drain — mirrors Ashling,
//     Flame Dancer's red retention and Omnath's green retention.
//  2. The self-dies trigger reads the (now-persisted) red pool and dumps
//     it as damage. "Pay any amount of {R}" is modeled as "pay all
//     available red" — the AI-rational maximum, since the creature is
//     already dead and the mana would otherwise empty at the next
//     boundary. Damage goes to an opponent (the standard any-target
//     heuristic). The red is consumed via ConsumeColoredMana.
//
// The flying keyword is supplied by the printed card data / keyword
// pipeline and is not this handler's concern.

func registerLeylineTyrant(r *Registry) {
	r.OnETB("Leyline Tyrant", leylineTyrantETB)
	r.OnTrigger("Leyline Tyrant", "permanent_ltb", leylineTyrantLTB)
	r.OnTrigger("Leyline Tyrant", "creature_dies", leylineTyrantDies)
}

func leylineTyrantETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "leyline_tyrant_red_retention"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	gameengine.RegisterManaPoolExemption(gs, perm, perm.Controller, []string{"R"})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         perm.Controller,
		"exempt_color": "R",
	})
}

func leylineTyrantLTB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	gameengine.UnregisterManaPoolExemptionForPerm(gs, perm)
}

func leylineTyrantDies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "leyline_tyrant_death_red_damage"
	if gs == nil || ctx == nil {
		return
	}
	// Self-dies gate: only fire for Leyline Tyrant's OWN death, not for
	// any other creature dying while a Leyline Tyrant is around.
	dying, _ := ctx["perm"].(*gameengine.Permanent)
	if dying == nil || dying.Card == nil || dying.Card.DisplayName() != "Leyline Tyrant" {
		return
	}
	controller := dying.Controller
	// At death time the red is still in the pool (the LTB exemption
	// unregister only affects FUTURE drains, and the dies trigger
	// resolves before the next step/phase boundary).
	amount := gameengine.UnspentRedMana(gs, controller)
	if amount <= 0 {
		// "you MAY pay any amount of {R}" — paying zero is legal and
		// produces no damage.
		emitPartial(gs, slug, "Leyline Tyrant", "no_red_mana_to_pay")
		return
	}
	target := leylineTyrantPickOpponent(gs, controller)
	if target < 0 {
		emitPartial(gs, slug, "Leyline Tyrant", "no_opponent_target")
		return
	}
	paid := gameengine.ConsumeColoredMana(gs, controller, "R", amount)
	if paid <= 0 {
		return
	}
	gameengine.DealDamage(gs, target, paid, "Leyline Tyrant")
	emit(gs, slug, "Leyline Tyrant", map[string]interface{}{
		"seat":        controller,
		"target_seat": target,
		"damage":      paid,
	})
}

// leylineTyrantPickOpponent returns the seat index of a living opponent
// to point the death damage at, or -1 if none. Picks the lowest-life
// living opponent (closest to a kill) for a sensible default heuristic.
func leylineTyrantPickOpponent(gs *gameengine.GameState, controller int) int {
	if gs == nil {
		return -1
	}
	best := -1
	bestLife := 1 << 30
	for i, s := range gs.Seats {
		if s == nil || i == controller {
			continue
		}
		if s.Lost || s.LeftGame {
			continue
		}
		if s.Life < bestLife {
			bestLife = s.Life
			best = i
		}
	}
	return best
}
