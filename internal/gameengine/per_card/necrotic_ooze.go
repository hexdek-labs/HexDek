package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/gameast"
)

// registerNecroticOoze wires up Necrotic Ooze.
//
// Oracle text:
//
//	As long as Necrotic Ooze is on the battlefield, it has all
//	activated abilities of all creature cards in all graveyards.
//
// Muldrotha wincon. Classic activations granted: Triskelion
// (sac/counter-remove deals damage), Walking Ballista (remove counter,
// damage), Devoted Druid (tap for G / +1/-1 counter untap).
//
// Batch #1 scope:
//   - ETB: walk every graveyard and collect each creature card's
//     Activated abilities. Register them on the Ooze permanent as a
//     dynamically-granted ability list (stored on perm.Flags + a side
//     map).
//   - Activation: callers invoke InvokeActivatedHook(ooze, idx) with
//     ctx["granted_ability"] = *gameast.Activated pointing at the
//     borrowed ability; we just resolve its Effect.
//
// The "granted abilities" list is stored on perm.Flags as a sentinel
// count (it'll be retrievable by test via the Ooze handler below).
//
// This does NOT implement ability activation timing/mana-cost checks —
// the engine's activated-ability path is still simplified. We log a
// partial so downstream work can wire it fully.
func registerNecroticOoze(r *Registry) {
	r.OnETB("Necrotic Ooze", necroticOozeETB)
	r.OnActivated("Necrotic Ooze", necroticOozeActivate)
}

// necroticOozeGrants is a side map from Ooze permanent pointer to the
// list of granted Activated abilities. Cleared lazily when the Ooze
// leaves play (engine doesn't have LTB hook yet).
var necroticOozeGrants = map[*gameengine.Permanent][]*gameast.Activated{}

func necroticOozeETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "necrotic_ooze_grant_activated"
	if gs == nil || perm == nil {
		return
	}
	// Walk every seat's graveyard, collect activated abilities from
	// creature cards.
	grants := []*gameast.Activated{}
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, c := range s.Graveyard {
			if c == nil || c.AST == nil {
				continue
			}
			if !cardHasType(c, "creature") {
				continue
			}
			for _, ab := range c.AST.Abilities {
				a, ok := ab.(*gameast.Activated)
				if !ok {
					continue
				}
				grants = append(grants, a)
			}
		}
	}
	necroticOozeGrants[perm] = grants
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["granted_activated_count"] = len(grants)

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"granted_count": len(grants),
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"activated_abilities_collected_but_mana_cost_and_activation_timing_are_caller_responsibility")
}

func necroticOozeActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "necrotic_ooze_use_granted"
	if gs == nil || src == nil {
		return
	}
	grants := necroticOozeGrants[src]
	if abilityIdx < 0 || abilityIdx >= len(grants) {
		emitFail(gs, slug, src.Card.DisplayName(), "ability_idx_out_of_range", map[string]interface{}{
			"idx":   abilityIdx,
			"count": len(grants),
		})
		return
	}
	granted := grants[abilityIdx]
	if granted == nil || granted.Effect == nil {
		return
	}
	// Resolve the granted ability's effect. Cost is assumed already
	// paid by the caller (mana-pool debit, tap, etc.).
	gameengine.ResolveEffect(gs, src, granted.Effect)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"ability_idx": abilityIdx,
	})
}

// GetNecroticOozeGrants exposes the granted-ability list for tests /
// decision-makers. Returns nil if Ooze isn't in play.
func GetNecroticOozeGrants(perm *gameengine.Permanent) []*gameast.Activated {
	return necroticOozeGrants[perm]
}
