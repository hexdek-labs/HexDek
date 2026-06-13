package gameengine

import "github.com/hexdek/hexdek/internal/gameast"

// regeneration.go — the regeneration replacement effect (CR §701.15).
//
// BACKGROUND / WHY THIS EXISTS
//
// Regeneration was previously NON-FUNCTIONAL engine-wide: the
// `regenerate` resolve case (and the combat-damage path) set
// `Flags["regeneration_shield"]`, but NOTHING ever consumed it —
// DestroyPermanent and the SBA lethal-damage path destroyed the
// creature anyway. So "{B}: Regenerate this creature" and every
// `regenerate` / `regenerate_typed` ability was cosmetic.
//
// CR §701.15a: "If the effect of a resolving spell or ability
// regenerates a permanent, it creates a shield for that permanent. …
// The next time that permanent would be destroyed this turn, instead
// its controller … removes it from combat, taps it, and removes all
// damage marked on it." (The shield is also a replacement for the
// state-based 'destroyed by lethal/deathtouch damage' actions.)
//
// This file implements the shield as a per-permanent count
// (`Flags["regeneration_shield"]`) and the replacement itself
// (TryRegenerate). The DestroyPermanent guard (zone_change.go) and the
// SBA lethal/deathtouch guard (sba.go) call TryRegenerate; the
// `regenerate` / `regenerate_typed` resolve cases grant shields via
// GrantRegenerationShield; cleanup clears stale shields at end of turn
// (phases.go). This single mechanic powers ~200 `regenerate_typed`
// cards plus the existing `regenerate` effects.

// GrantRegenerationShield adds one regeneration shield to perm (CR
// §701.15a). Shields stack; each replaces one would-be destruction this
// turn. Safe on nil.
func GrantRegenerationShield(gs *GameState, perm *Permanent) {
	if perm == nil {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["regeneration_shield"]++
	if gs != nil {
		gs.LogEvent(Event{
			Kind:   "regenerate",
			Seat:   perm.Controller,
			Source: sourceName(perm),
			Details: map[string]interface{}{
				"shields": perm.Flags["regeneration_shield"],
				"rule":    "701.15a",
			},
		})
	}
}

// regenerateTypedSubjects resolves the `regenerate_typed` Modification's
// Filter arg (loaded as a map[string]interface{}) into the permanents
// that receive a shield. Corpus reality: 172/200 are base="self"
// ("Regenerate this creature"), almost all inside activated abilities
// whose source IS the creature; the rest are "regenerate target
// creature". Policy:
//   - base "self"/empty/missing → the source permanent.
//   - base "creature" → the source if it's a creature (covers the
//     activated-on-a-creature and self-target shapes); otherwise the
//     controller's highest-power creature (a reasonable AI "protect my
//     best creature" pick for the rare instant-speed "regenerate target
//     creature").
func regenerateTypedSubjects(gs *GameState, src *Permanent, e *gameast.ModificationEffect) []*Permanent {
	base := "self"
	if e != nil && len(e.Args) > 0 {
		if m, ok := e.Args[0].(map[string]interface{}); ok {
			if b, ok := m["base"].(string); ok && b != "" {
				base = b
			}
		}
	}
	if base == "self" || base == "" {
		if src != nil {
			return []*Permanent{src}
		}
		return nil
	}
	// base == "creature" (or other) — prefer the source when it's itself
	// a creature.
	if src != nil && src.IsCreature() {
		return []*Permanent{src}
	}
	// Otherwise protect the controller's best creature.
	if src == nil || gs == nil || src.Controller < 0 || src.Controller >= len(gs.Seats) {
		return nil
	}
	var best *Permanent
	for _, p := range gs.Seats[src.Controller].Battlefield {
		if p != nil && p.IsCreature() && (best == nil || p.Power() > best.Power()) {
			best = p
		}
	}
	if best != nil {
		return []*Permanent{best}
	}
	return nil
}

// TryRegenerate applies the regeneration replacement (CR §701.15a) if
// perm holds a shield: consume one shield, tap the permanent, remove it
// from combat, and clear all marked damage. Returns true when the
// destruction was replaced (caller must NOT destroy), false otherwise.
func TryRegenerate(gs *GameState, perm *Permanent) bool {
	if perm == nil || perm.Flags == nil || perm.Flags["regeneration_shield"] <= 0 {
		return false
	}
	perm.Flags["regeneration_shield"]--
	if perm.Flags["regeneration_shield"] <= 0 {
		delete(perm.Flags, "regeneration_shield")
	}

	// §701.15a: tap it, remove it from combat, remove all marked damage.
	perm.Tapped = true
	perm.MarkedDamage = 0
	if perm.IsAttacking() || perm.IsBlocking() {
		setPermFlag(perm, flagAttacking, false)
		setPermFlag(perm, flagBlocking, false)
	}

	if gs != nil {
		gs.LogEvent(Event{
			Kind:   "regenerated",
			Seat:   perm.Controller,
			Source: sourceName(perm),
			Details: map[string]interface{}{
				"card":              perm.Card.DisplayName(),
				"shields_remaining": perm.Flags["regeneration_shield"],
				"rule":              "701.15a",
			},
		})
	}
	return true
}
