package per_card

import (
	"sync"

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
// Implementation:
//   - ETB: walk every graveyard and collect each creature card's
//     Activated abilities. Register them on the Ooze permanent as a
//     dynamically-granted ability list (stored on perm.Flags + a side
//     map).
//   - Graveyard-change trigger: whenever a creature enters or leaves
//     any graveyard, refresh the Ooze's granted ability list. This
//     handles the continuous nature of the static ability (CR §604.3a).
//   - Activation: callers invoke InvokeActivatedHook(ooze, idx) with
//     ctx["granted_ability"] = *gameast.Activated pointing at the
//     borrowed ability; we just resolve its Effect.
//
// The "granted abilities" list is stored on perm.Flags as a sentinel
// count (it'll be retrievable by test via the Ooze handler below).
func registerNecroticOoze(r *Registry) {
	r.OnETB("Necrotic Ooze", necroticOozeETB)
	r.OnActivated("Necrotic Ooze", necroticOozeActivate)
	// Refresh granted abilities when any creature enters or leaves a
	// graveyard (dies, mill, reanimate, exile-from-GY, etc.).
	r.OnTrigger("Necrotic Ooze", "creature_dies", necroticOozeRefresh)
	r.OnTrigger("Necrotic Ooze", "zone_change", necroticOozeRefresh)
}

var necroticOozeGrants sync.Map

// necroticOozeCollectGrants walks all graveyards and returns every
// Activated ability from every creature card found. Shared between
// ETB and the graveyard-change refresh trigger.
func necroticOozeCollectGrants(gs *gameengine.GameState) []*gameast.Activated {
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
	return grants
}

func necroticOozeETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "necrotic_ooze_grant_activated"
	if gs == nil || perm == nil {
		return
	}
	grants := necroticOozeCollectGrants(gs)
	necroticOozeGrants.Store(perm, grants)
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["granted_activated_count"] = len(grants)

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"granted_count": len(grants),
	})
}

// necroticOozeRefresh fires whenever a creature enters or leaves a
// graveyard. It re-scans all graveyards and updates the Ooze's
// granted ability list. This is how the Ooze dynamically gains/loses
// abilities as the game progresses (e.g. a Triskelion is reanimated
// out of the graveyard — the Ooze immediately loses that ability).
func necroticOozeRefresh(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "necrotic_ooze_refresh"
	if gs == nil || perm == nil {
		return
	}
	// Confirm the Ooze is still on the battlefield.
	onField := false
	if perm.Controller >= 0 && perm.Controller < len(gs.Seats) {
		for _, p := range gs.Seats[perm.Controller].Battlefield {
			if p == perm {
				onField = true
				break
			}
		}
	}
	if !onField {
		necroticOozeGrants.Delete(perm)
		return
	}

	oldCount := 0
	if v, ok := necroticOozeGrants.Load(perm); ok {
		oldCount = len(v.([]*gameast.Activated))
	}
	grants := necroticOozeCollectGrants(gs)
	necroticOozeGrants.Store(perm, grants)
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["granted_activated_count"] = len(grants)

	// Only log when the grant list actually changed.
	if len(grants) != oldCount {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"granted_count": len(grants),
			"previous":      oldCount,
		})
	}
}

func necroticOozeActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "necrotic_ooze_use_granted"
	if gs == nil || src == nil {
		return
	}
	var grants []*gameast.Activated
	if v, ok := necroticOozeGrants.Load(src); ok {
		grants = v.([]*gameast.Activated)
	}
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
	if v, ok := necroticOozeGrants.Load(perm); ok {
		return v.([]*gameast.Activated)
	}
	return nil
}

// NecroticOozeGrantCount returns the number of activated abilities
// currently granted to the Ooze. Useful for AI decision-making.
func NecroticOozeGrantCount(perm *gameengine.Permanent) int {
	if v, ok := necroticOozeGrants.Load(perm); ok {
		return len(v.([]*gameast.Activated))
	}
	return 0
}

// CleanupNecroticOoze removes the Ooze's grants entry. Called when the
// Ooze leaves the battlefield. Prevents stale entries from accumulating
// across long games.
func CleanupNecroticOoze(perm *gameengine.Permanent) {
	necroticOozeGrants.Delete(perm)
}
