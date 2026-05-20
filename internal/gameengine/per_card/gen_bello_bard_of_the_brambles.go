package per_card

import (
	"strconv"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerBelloBardOfTheBrambles wires Bello, Bard of the Brambles.
//
// Oracle text:
//
//	During your turn, each non-Equipment artifact and non-Aura
//	enchantment you control with mana value 4 or greater is a 4/4
//	Elemental creature in addition to its other types and has
//	indestructible, haste, and "Whenever this creature deals combat
//	damage to a player, draw a card."
//
// Implementation (R46 stub port):
//   - Static-grant clause via layered continuous effects. The shared
//     predicate `belloQualifies` matches: not Bello herself; controlled
//     by Bello's controller; printed CMC ≥ 4; type is artifact OR
//     enchantment; subtype is NOT Equipment and NOT Aura; and gs.Active
//     == controller (the "during your turn" gate).
//   - Layer 4: ADD creature type + "elemental" subtype.
//   - Layer 7b: SET base P/T to 4/4 (only on perms that the layer-4
//     pass turned into creatures — printed artifact-creatures keep
//     their printed P/T).
//   - Layer 6: grant indestructible + haste.
//   - The "combat damage to player → draw" rider is a triggered
//     ability on each qualifying creature. Since we can't register a
//     per-target trigger statically here, we hook
//     OnTrigger("combat_damage_player") on Bello herself and route
//     the draw when the damaging permanent matches the predicate.
//     This is functionally equivalent for the common case (damage
//     fires per attacker → predicate filters).
//
// SourcePerm = Bello, so LTB unregisters all four layer entries
// automatically.
func registerBelloBardOfTheBrambles(r *Registry) {
	r.OnETB("Bello, Bard of the Brambles", belloRegister)
	r.OnTrigger("Bello, Bard of the Brambles", "combat_damage_player", belloCombatDraw)
}

func belloQualifies(gs *gameengine.GameState, src *gameengine.Permanent, t *gameengine.Permanent) bool {
	if t == nil || t == src || t.Card == nil {
		return false
	}
	if t.Controller != src.Controller {
		return false
	}
	if gs == nil || gs.Active != src.Controller {
		return false
	}
	// Printed type predicate — must be artifact or enchantment, NOT
	// Equipment, NOT Aura. We read printed types from t.Card.Types so
	// the predicate isn't recursive on its own layer-4 add.
	isArt, isEnch := false, false
	isEquip, isAura := false, false
	for _, ty := range t.Card.Types {
		switch ty {
		case "artifact":
			isArt = true
		case "enchantment":
			isEnch = true
		case "equipment", "Equipment":
			isEquip = true
		case "aura", "Aura":
			isAura = true
		}
	}
	if !isArt && !isEnch {
		return false
	}
	if isArt && isEquip {
		return false
	}
	if isEnch && isAura {
		return false
	}
	mv := t.Card.CMC
	if mv <= 0 {
		// Fall back to the per_card cardCMC heuristic (reads "cmc:N"
		// tag or proxies from base P/T) when the canonical Card.CMC
		// isn't set — Goldilocks fixtures use the tag form.
		mv = cardCMC(t.Card)
	}
	return mv >= 4
}

func belloRegister(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "bello_bard_of_the_brambles_static"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	src := perm
	pred := func(g *gameengine.GameState, t *gameengine.Permanent) bool {
		return belloQualifies(g, src, t)
	}
	ts := perm.Timestamp
	suffix := strconv.Itoa(ts)

	// Layer 4: add creature type + elemental subtype.
	gs.RegisterContinuousEffect(&gameengine.ContinuousEffect{
		Layer:          gameengine.LayerType,
		Timestamp:      ts,
		SourcePerm:     src,
		SourceCardName: "Bello, Bard of the Brambles",
		ControllerSeat: perm.Controller,
		HandlerID:      "Bello, Bard of the Brambles:type_elemental:" + suffix,
		Predicate:      pred,
		ApplyFn: func(_ *gameengine.GameState, _ *gameengine.Permanent, chars *gameengine.Characteristics) {
			hasCreature := false
			for _, t := range chars.Types {
				if t == "creature" {
					hasCreature = true
					break
				}
			}
			if !hasCreature {
				chars.Types = append(chars.Types, "creature")
			}
			hasElem := false
			for _, t := range chars.Subtypes {
				if t == "elemental" || t == "Elemental" {
					hasElem = true
					break
				}
			}
			if !hasElem {
				chars.Subtypes = append(chars.Subtypes, "elemental")
			}
		},
	})

	// Layer 7b: set base P/T to 4/4 (only when our layer-4 add brought
	// the permanent into creature-hood; printed artifact creatures keep
	// their printed P/T).
	gs.RegisterContinuousEffect(&gameengine.ContinuousEffect{
		Layer:          gameengine.LayerPT,
		Sublayer:       "b",
		Timestamp:      ts,
		SourcePerm:     src,
		SourceCardName: "Bello, Bard of the Brambles",
		ControllerSeat: perm.Controller,
		HandlerID:      "Bello, Bard of the Brambles:base_pt_4_4:" + suffix,
		Predicate:      pred,
		ApplyFn: func(_ *gameengine.GameState, t *gameengine.Permanent, chars *gameengine.Characteristics) {
			// Skip permanents that are already printed creatures —
			// only "Becomes a creature" carries the 4/4 base.
			if t != nil && t.Card != nil {
				for _, ty := range t.Card.Types {
					if ty == "creature" {
						return
					}
				}
			}
			chars.Power = 4
			chars.Toughness = 4
			chars.BasePower = 4
			chars.BaseToughness = 4
		},
	})

	// Layer 6: grant indestructible.
	gs.RegisterContinuousEffect(&gameengine.ContinuousEffect{
		Layer:          gameengine.LayerAbility,
		Timestamp:      ts,
		SourcePerm:     src,
		SourceCardName: "Bello, Bard of the Brambles",
		ControllerSeat: perm.Controller,
		HandlerID:      "Bello, Bard of the Brambles:kw_indestructible:" + suffix,
		Predicate:      pred,
		ApplyFn: func(_ *gameengine.GameState, _ *gameengine.Permanent, chars *gameengine.Characteristics) {
			for _, kw := range chars.Keywords {
				if kw == "indestructible" {
					return
				}
			}
			chars.Keywords = append(chars.Keywords, "indestructible")
		},
	})

	// Layer 6: grant haste.
	gs.RegisterContinuousEffect(&gameengine.ContinuousEffect{
		Layer:          gameengine.LayerAbility,
		Timestamp:      ts,
		SourcePerm:     src,
		SourceCardName: "Bello, Bard of the Brambles",
		ControllerSeat: perm.Controller,
		HandlerID:      "Bello, Bard of the Brambles:kw_haste:" + suffix,
		Predicate:      pred,
		ApplyFn: func(_ *gameengine.GameState, _ *gameengine.Permanent, chars *gameengine.Characteristics) {
			for _, kw := range chars.Keywords {
				if kw == "haste" {
					return
				}
			}
			chars.Keywords = append(chars.Keywords, "haste")
		},
	})

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"layers": []string{"4", "7b", "6_indestructible", "6_haste"},
	})
}

func belloCombatDraw(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "bello_combat_damage_draw"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	srcPerm, _ := ctx["source_perm"].(*gameengine.Permanent)
	if srcPerm == nil {
		// Some emitters pass "attacker_perm" instead.
		srcPerm, _ = ctx["attacker_perm"].(*gameengine.Permanent)
	}
	if srcPerm == nil {
		return
	}
	// Bello herself draws too if she is the attacker; the predicate
	// excludes her, so we add an explicit allow.
	if srcPerm != perm && !belloQualifies(gs, perm, srcPerm) {
		return
	}
	if c := drawOne(gs, perm.Controller, perm.Card.DisplayName()); c != nil {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   perm.Controller,
			"source": srcPerm.Card.DisplayName(),
			"drawn":  c.DisplayName(),
		})
	}
}
