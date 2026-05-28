package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerHungeringHydra wires Hungering Hydra.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Hungering%20Hydra):
//
//	{X}{G}{G}
//	Creature — Hydra
//	0/0
//	Trample
//	Hungering Hydra enters with X +1/+1 counters on it.
//	Whenever Hungering Hydra is dealt damage, put that many +1/+1
//	counters on it.
//
// The damage-trigger growth is the high-impact piece: a 3/3 hydra
// blocked by a 5/5 ends up a (3+5)/(3+5) = 8/8 after combat damage
// resolves, then survives because the marked damage compares against
// the new toughness. Combined with trample, it presents a sticky
// recurring threat that punishes durdly removal.
//
// Implementation:
//   - OnETB: read perm.Flags["x_paid"] (the standard X-on-cast plumb)
//     and AddCounter("+1/+1", x). Mirrors ingenious_prodigy.go pattern.
//     If x_paid is missing/0, Hydra enters as a 0/0 and dies to SBAs —
//     that's the rules-correct outcome.
//   - OnTrigger("damage_taken") gated on target_perm == self. Read
//     amount from ctx and AddCounter("+1/+1", amount). InvalidateCharacteristicsCache
//     so the new P/T propagates before SBAs check toughness.
func registerHungeringHydra(r *Registry) {
	r.OnETB("Hungering Hydra", hungeringHydraETB)
	// Register only "damage_taken" — "creature_dealt_damage" canonicalizes
	// to "damage_taken" via event_aliases.go, so double-registration would
	// fire the handler twice for each damage event.
	r.OnTrigger("Hungering Hydra", "damage_taken", hungeringHydraDamageTaken)
}

func hungeringHydraETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "hungering_hydra_etb_x_counters"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	x := 0
	if perm.Flags != nil {
		x = perm.Flags["x_paid"]
	}
	if x > 0 {
		perm.AddCounter("+1/+1", x)
		gs.InvalidateCharacteristicsCache()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"x":        x,
		"counters": perm.Counters["+1/+1"],
	})
	if x <= 0 {
		emitPartial(gs, slug, perm.Card.DisplayName(),
			"x_paid_not_threaded_into_etb_hook_enters_at_zero_counters")
	}
}

func hungeringHydraDamageTaken(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "hungering_hydra_damage_growth"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	tgt, _ := ctx["target_perm"].(*gameengine.Permanent)
	if tgt != nil && tgt != perm {
		return
	}
	amount, _ := ctx["amount"].(int)
	if amount <= 0 {
		return
	}
	perm.AddCounter("+1/+1", amount)
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":            perm.Controller,
		"damage":          amount,
		"counters_added":  amount,
		"counters_total":  perm.Counters["+1/+1"],
	})
}
