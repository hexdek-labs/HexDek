package gameengine

// r107 — the TYPE axis: one predicate for "this permanent is not a creature
// right now", shared by the layer-blind and the layer-aware readers.
//
// THE DEFECT, measured rather than reasoned about.
//
// Three keywords temporarily remove the creature type:
//
//	CR §702.103a  bestow      — while attached as an Aura it is an
//	                            enchantment — Aura and is NOT a creature.
//	CR §702.151b  reconfigure — while attached it is just an Equipment and
//	                            stops being a creature.
//	CR §702.176   impending   — while time counters remain it is not a
//	                            creature.
//
// `(*Permanent).IsCreature` implemented all three inline. The layer-aware
// `(*GameState).IsCreatureOf` — which reads GetEffectiveCharacteristics —
// implemented NONE of them, because nothing in layers.go had ever heard of
// the flags. A probe over all four type cases measured:
//
//	(0) plain creature            blind=true   layered=true    agree
//	(1) impending   §702.176      blind=false  layered=true    DIVERGE
//	(2) bestowed    §702.103a     blind=false  layered=true    DIVERGE
//	(3) reconfigured §702.151b    blind=false  layered=true    DIVERGE
//	(4) layer-4 type-add (Ensoul) blind=false  layered=true    DIVERGE
//
// The open item asked whether IsCreatureOf is a strict superset of
// IsCreature. It IS one — and that is the trap. A superset is only a safety
// property if the extra members are CORRECT, and three of these four extra
// trues are rules-WRONG. Migrating the 687 `.IsCreature()` call sites to the
// "layer-aware" reader, which is what "it's a superset" invites, would have
// introduced three §702 violations wherever those flags matter while fixing
// the one type-add case.
//
// IT REACHES PRODUCTION. sba704_5f (§704.5f toughness), sba704_5g (§704.5g
// lethal damage) and sba704_5h (§704.5h deathtouch) all gate on
// IsCreatureOf. Driving the real StateBasedActions entry point, a bestowed,
// a reconfigured and an impending permanent at lethal damage were each
// DESTROYED — while `IsCreature`'s own doc comment two files away promises a
// reconfigured permanent "isn't seen by creature SBAs". The contract was
// written down in the predicate nobody's SBA calls.
//
// THE FIX is not to migrate call sites. It is to give both readers one
// source of truth, so they cannot drift again: this predicate, consulted by
// `IsCreature` and applied to the computed characteristics at the end of the
// §613.1d layer-4 pass.
//
// Applied AFTER layer 4 rather than in BaseCharacteristics on purpose: these
// are type-CHANGING effects (§613.1d), so they belong in that layer, and
// running them at the end of it means an Ensoul-style type-add cannot
// re-grant the creature type to a permanent the keyword has suppressed.
func permSuppressesCreatureType(p *Permanent) bool {
	if p == nil || p.Flags == nil {
		return false
	}
	// CR §702.103a — bestow, attached as an Aura. Cleared by BestowFalloff.
	if p.Flags["bestowed"] == 1 {
		return true
	}
	// CR §702.151b — reconfigure, attached. ActivateReconfigure sets the flag
	// on attach and clears it on detach.
	if p.Flags["reconfigured"] == 1 {
		return true
	}
	// CR §702.176 — impending, while time counters remain.
	if p.Flags["not_creature_while_impending"] == 1 {
		return true
	}
	return false
}

// applyCreatureTypeSuppression removes the creature type from chars when one
// of the §702 suppression keywords is active. Called at the end of the
// layer-4 pass in GetEffectiveCharacteristics.
func applyCreatureTypeSuppression(p *Permanent, chars *Characteristics) {
	if chars == nil || !permSuppressesCreatureType(p) {
		return
	}
	out := chars.Types[:0]
	for _, t := range chars.Types {
		if t == "creature" {
			continue
		}
		out = append(out, t)
	}
	chars.Types = out
}
