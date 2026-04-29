package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerNullRod wires up Null Rod.
//
// Oracle text:
//
//	Activated abilities of artifacts can't be activated unless they
//	are mana abilities.
//
// The pre-eminent anti-artifact-combo stax piece. A single Null Rod
// shuts off Sol Ring's mana ability? NO — mana abilities (CR §605) are
// exempted. What Null Rod DOES shut off: Aetherflux Reservoir's 50-
// damage activation, Isochron Scepter's copy activation, Walking
// Ballista's ping, Sensei's Divining Top's look and draw, Mana Vault's
// {4}: Untap (not a mana ability — it's the cost of UNTAPPING, not a
// mana production), etc.
//
// Batch #3 scope:
//   - OnETB: stamp gs.Flags["null_rod_active"] to the controller's
//     seat (as bitfield of seats that have any non-mana-ability
//     suppressor — a future pass will keep multi-Null-Rod counts).
//   - Provide NullRodActive(gs) helper so the activation-dispatch path
//     (InvokeActivatedHook or a future ActivateAbility engine path)
//     can short-circuit non-mana activations of artifacts.
//
// Important: the suppression is GLOBAL (affects all players including
// the controller). This is oracle-correct — Null Rod is symmetric.
//
// The enforcement is "activation-time" per CR §602.1b — checked when
// a player announces activation. Our current engine routes per-card
// activated abilities through InvokeActivatedHook directly, so the
// check MUST happen at that seam. We add a check in
// NullRodSuppresses(perm, abilityKind) that per_card activation
// handlers (and downstream engine code) can call before invoking
// their body.
func registerNullRod(r *Registry) {
	r.OnETB("Null Rod", nullRodETB)
}

func nullRodETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "null_rod_static"
	if gs == nil || perm == nil {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	// Increment a ref count — multiple Null Rods / Collector Ouphes
	// stack additively (symmetric; one is enough).
	gs.Flags["null_rod_count"]++
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"rod_count":  gs.Flags["null_rod_count"],
		"suppresses": "artifact_activated_non_mana",
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"activation_dispatch_must_consult_NullRodSuppresses_at_activation_time")
}

// NullRodActive returns true if at least one Null Rod / Collector
// Ouphe is on the battlefield. Callers (activation dispatch) consult
// this before invoking an artifact's non-mana activated ability.
//
// Oracle note: Null Rod's "can't be activated unless they are mana
// abilities" is CR §605 scoped. A mana ability is (§605.1a) any
// activated ability that (1) doesn't target, (2) could add mana, and
// (3) is not a loyalty ability. Basalt Monolith's {T}: Add {C}{C}{C}
// IS a mana ability (exempted). Basalt Monolith's {3}: Untap is NOT
// (no mana produced) — Null Rod suppresses it.
func NullRodActive(gs *gameengine.GameState) bool {
	if gs == nil || gs.Flags == nil {
		return false
	}
	return gs.Flags["null_rod_count"] > 0
}

// NullRodSuppresses reports whether a given permanent's activated
// ability at index `abilityIdx` is SUPPRESSED by an active Null Rod /
// Collector Ouphe. Returns false if the permanent is not an artifact,
// or if no Null Rod is active, or if the ability is a mana ability.
//
// Classification of "mana ability" per CR §605: if the per-card
// handler's slug STARTS WITH a known mana-ability marker (e.g. contains
// "_tap" or "add_mana") we exempt it. MVP: we key by (card_name,
// ability_idx) against a hand-rolled whitelist of known mana abilities
// on per_card-registered artifacts.
func NullRodSuppresses(gs *gameengine.GameState, perm *gameengine.Permanent, abilityIdx int) bool {
	if !NullRodActive(gs) || perm == nil || perm.Card == nil {
		return false
	}
	if !perm.IsArtifact() {
		return false
	}
	// Mana-ability whitelist. Each entry is (card_name, ability_idx).
	// These are the artifacts whose per_card handlers implement an
	// ability that is a "mana ability" per CR §605.
	//
	// If you add a new artifact whose ability idx N is a mana ability,
	// add (name, N) here so Null Rod correctly exempts it.
	manaAbilities := map[string]map[int]bool{
		"Mana Crypt":      {0: true},
		"Basalt Monolith": {0: true},            // idx 0 is Add {C}{C}{C}
		"Grim Monolith":   {0: true},
		"Urza, Lord High Artificer": {0: true}, // Urza's {T}: artifact → {U}
		"Sol Ring":        {0: true},
		// Aetherflux Reservoir: NO mana abilities — 50-damage is NOT a mana ability.
		// Isochron Scepter: NO mana abilities — copy is NOT a mana ability.
		// Sensei's Divining Top: NO mana abilities.
		// Walking Ballista: activated on a creature, not an artifact-only ability; still suppressed.
	}
	name := perm.Card.DisplayName()
	if abilities, ok := manaAbilities[name]; ok {
		if abilities[abilityIdx] {
			return false // It IS a mana ability; not suppressed.
		}
	}
	return true // Suppressed.
}
