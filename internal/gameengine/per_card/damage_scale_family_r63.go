package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// damage_scale_family_r63.go — broad CR §614 DAMAGE-REPLACEMENT audit
// (replacement-damage-r63 category).
//
// A large family of "if [a filtered source] would deal damage [to a
// filtered target], it deals [double/triple that damage | that much
// damage plus N] instead" cards parsed to an inert `if_intervening_tail`
// / `replacement_static` scaffold (resolve_helpers.go logs those kinds
// but applies no game-state change). So every one of these multiplier /
// bonus damage replacements did NOTHING — Gratuitous Violence didn't
// double, Fiery Emancipation didn't triple, Torbran's siblings added no
// bonus.
//
// They are all the SAME SHAPE as the already-wired Furnace of Rath /
// Torbran / Gisela / Dictate handlers: at ETB register a
// DamageReplacement closure on gs.DamageReplacements that mutates
// ctx.Amount; at LTB unregister it. This file factors that shape into
// registerDamageScale and wires the inert permanent-source members of
// the family. (Conditional / chosen-player / one-shot / leveler members
// — Sawhorn Nemesis, Bitter Feud, Anthem of Rakdos, Aether Revolt,
// Artist's Talent, Isengard Unleashed, Impulsive Maneuvers, etc. — need
// per-card state and are tracked in the report backlog, not here.)
//
// §616 ordering: these run in the gs.DamageReplacements registry, which
// is consulted by combat (applyCombatDamageTo{Player,Creature}), spell /
// ability damage (applyDamage), and the generic DealDamage path, BEFORE
// the §615 prevention shields — matching the verified damage-615 audit.
// Multiple effects stack by registration order (×2 then ×2 = ×4; +1 then
// ×2 applies in registration order — an approximation of the §616
// "controller of the affected object orders the replacements" choice,
// consistent with the existing Furnace/Torbran handlers).

func init() {
	registerDamageScaleFamily(Global())
	AddResetHook(registerDamageScaleFamily)
}

// dmgScaleMatch decides whether a damage event is affected by the
// replacement. self is the permanent that owns the replacement.
type dmgScaleMatch func(self *gameengine.Permanent, ctx *gameengine.DamageContext) bool

// registerDamageScale wires one §614 multiplier/bonus damage replacement:
// ETB registers a closure that, when match passes, multiplies ctx.Amount
// by mult (when mult > 1) then adds bonus; permanent_ltb unregisters it.
func registerDamageScale(r *Registry, cardName, handlerID string, mult, bonus int, match dmgScaleMatch) {
	etb := func(gs *gameengine.GameState, perm *gameengine.Permanent) {
		if gs == nil || perm == nil || perm.Card == nil {
			return
		}
		self := perm
		gs.RegisterDamageReplacement(&gameengine.DamageReplacement{
			SourcePerm: perm,
			HandlerID:  handlerID,
			Fn: func(gs *gameengine.GameState, ctx *gameengine.DamageContext) {
				if ctx == nil || ctx.Amount <= 0 {
					return
				}
				if match != nil && !match(self, ctx) {
					return
				}
				if mult > 1 {
					ctx.Amount *= mult
				}
				ctx.Amount += bonus
			},
		})
		emit(gs, handlerID, perm.Card.DisplayName(), map[string]interface{}{
			"seat":  perm.Controller,
			"mult":  mult,
			"bonus": bonus,
		})
	}
	ltb := func(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
		if gs == nil || perm == nil || ctx == nil {
			return
		}
		if leaving, _ := ctx["perm"].(*gameengine.Permanent); leaving != perm {
			return
		}
		gs.UnregisterDamageReplacementsForPermanent(perm)
	}
	r.OnETB(cardName, etb)
	r.OnTrigger(cardName, "permanent_ltb", ltb)
}

// ---- composable match predicates -----------------------------------------

// dsYouControl: the damage source is a permanent/spell controlled by the
// replacement's controller. ctx.Source is the synthetic *Permanent the
// engine builds for spell/ability damage too, so this covers combat,
// activated abilities, and instant/sorcery damage. nil Source (rare
// untied damage) never matches — correct conservative behavior.
func dsYouControl(self *gameengine.Permanent, ctx *gameengine.DamageContext) bool {
	return ctx.Source != nil && ctx.Source.Controller == self.Controller
}

// dsOpponentTarget: "an opponent or a permanent an opponent controls" —
// TargetSeat is the damaged player or the target permanent's controller.
func dsOpponentTarget(self *gameengine.Permanent, ctx *gameengine.DamageContext) bool {
	return ctx.TargetSeat != self.Controller
}

// dsRed: source is red (Colors carries "R" or a pip:R type tag — reuses
// torbranIsRedSource for the same test-fixture-friendly convention).
func dsRed(ctx *gameengine.DamageContext) bool {
	return ctx.Source != nil && torbranIsRedSource(ctx.Source.Card)
}

func dsSourceHasType(ctx *gameengine.DamageContext, t string) bool {
	return ctx.Source != nil && cardHasType(ctx.Source.Card, t)
}

func dsSourceHasSubtype(ctx *gameengine.DamageContext, sub string) bool {
	return ctx.Source != nil && cardHasSubtype(ctx.Source.Card, sub)
}

func dsInstantOrSorcery(ctx *gameengine.DamageContext) bool {
	return dsSourceHasType(ctx, "instant") || dsSourceHasType(ctx, "sorcery")
}

// ---- the family -----------------------------------------------------------

func registerDamageScaleFamily(r *Registry) {
	// ----- doublers (×2), source you control, ANY target -----
	// Gratuitous Violence: "If a creature you control would deal damage…"
	registerDamageScale(r, "Gratuitous Violence", "gratuitous_violence_double", 2, 0,
		func(self *gameengine.Permanent, ctx *gameengine.DamageContext) bool {
			return dsYouControl(self, ctx) && dsSourceHasType(ctx, "creature")
		})
	// Angrath's Marauders: "If a source you control would deal damage…"
	registerDamageScale(r, "Angrath's Marauders", "angraths_marauders_double", 2, 0, dsYouControl)
	// Calamity Bearer: "If a Giant source you control would deal damage…"
	registerDamageScale(r, "Calamity Bearer", "calamity_bearer_double", 2, 0,
		func(self *gameengine.Permanent, ctx *gameengine.DamageContext) bool {
			return dsYouControl(self, ctx) && dsSourceHasSubtype(ctx, "giant")
		})
	// Obosh, the Preypiercer: "…source you control with an odd mana value…"
	registerDamageScale(r, "Obosh, the Preypiercer", "obosh_double_odd_mv", 2, 0,
		func(self *gameengine.Permanent, ctx *gameengine.DamageContext) bool {
			return dsYouControl(self, ctx) && ctx.Source.Card != nil && ctx.Source.Card.CMC%2 == 1
		})
	// Uncivil Unrest: "…creature you control with a +1/+1 counter on it…"
	registerDamageScale(r, "Uncivil Unrest", "uncivil_unrest_double", 2, 0,
		func(self *gameengine.Permanent, ctx *gameengine.DamageContext) bool {
			return dsYouControl(self, ctx) && dsSourceHasType(ctx, "creature") &&
				ctx.Source.Counters != nil && ctx.Source.Counters["+1/+1"] > 0
		})

	// ----- doublers (×2), source you control, OPPONENT target -----
	// Twinflame Tyrant.
	registerDamageScale(r, "Twinflame Tyrant", "twinflame_tyrant_double", 2, 0,
		func(self *gameengine.Permanent, ctx *gameengine.DamageContext) bool {
			return dsYouControl(self, ctx) && dsOpponentTarget(self, ctx)
		})

	// ----- doublers (×2), ANY source, OPPONENT (player only) target -----
	// Fiendish Duo: "If a source would deal damage to an opponent…".
	registerDamageScale(r, "Fiendish Duo", "fiendish_duo_double", 2, 0,
		func(self *gameengine.Permanent, ctx *gameengine.DamageContext) bool {
			return ctx.TargetPerm == nil && dsOpponentTarget(self, ctx)
		})

	// ----- triplers (×3), source you control, ANY target -----
	registerDamageScale(r, "Fiery Emancipation", "fiery_emancipation_triple", 3, 0, dsYouControl)
	registerDamageScale(r, "City on Fire", "city_on_fire_triple", 3, 0, dsYouControl)

	// ----- +1 bonus, source you control -----
	// Embermaw Hellion / Jaya, Venerated Firemage:
	// "another red source you control" → +1 (exclude self).
	embermaw := func(self *gameengine.Permanent, ctx *gameengine.DamageContext) bool {
		return dsYouControl(self, ctx) && ctx.Source != self && dsRed(ctx)
	}
	registerDamageScale(r, "Embermaw Hellion", "embermaw_hellion_plus1", 1, 1, embermaw)
	registerDamageScale(r, "Jaya, Venerated Firemage", "jaya_venerated_plus1", 1, 1, embermaw)
	// Mechanized Warfare: "a red or artifact source you control" → opponent → +1.
	registerDamageScale(r, "Mechanized Warfare", "mechanized_warfare_plus1", 1, 1,
		func(self *gameengine.Permanent, ctx *gameengine.DamageContext) bool {
			return dsYouControl(self, ctx) && dsOpponentTarget(self, ctx) &&
				(dsRed(ctx) || dsSourceHasType(ctx, "artifact"))
		})
	// Valley Flamecaller: "a Lizard, Mouse, Otter, or Raccoon you control" → +1.
	registerDamageScale(r, "Valley Flamecaller", "valley_flamecaller_plus1", 1, 1,
		func(self *gameengine.Permanent, ctx *gameengine.DamageContext) bool {
			if !dsYouControl(self, ctx) {
				return false
			}
			return dsSourceHasSubtype(ctx, "lizard") || dsSourceHasSubtype(ctx, "mouse") ||
				dsSourceHasSubtype(ctx, "otter") || dsSourceHasSubtype(ctx, "raccoon")
		})

	// ----- spell-source families (instant/sorcery; noncombat by nature) -----
	// Fire Servant: "a red instant or sorcery spell you control" → ×2.
	registerDamageScale(r, "Fire Servant", "fire_servant_double", 2, 0,
		func(self *gameengine.Permanent, ctx *gameengine.DamageContext) bool {
			return dsYouControl(self, ctx) && dsRed(ctx) && dsInstantOrSorcery(ctx)
		})
	// Sulfuric Vapors: "a red spell" (ANY controller) → +1.
	registerDamageScale(r, "Sulfuric Vapors", "sulfuric_vapors_plus1", 1, 1,
		func(self *gameengine.Permanent, ctx *gameengine.DamageContext) bool {
			return dsRed(ctx) && dsInstantOrSorcery(ctx)
		})
	// Pyromancer's Swath: "an instant or sorcery source you control" → +2.
	registerDamageScale(r, "Pyromancer's Swath", "pyromancers_swath_plus2", 1, 2,
		func(self *gameengine.Permanent, ctx *gameengine.DamageContext) bool {
			return dsYouControl(self, ctx) && dsInstantOrSorcery(ctx)
		})
	// Pyromancer's Gauntlet: "a red instant or sorcery spell you control OR a
	// red planeswalker you control" → +2.
	registerDamageScale(r, "Pyromancer's Gauntlet", "pyromancers_gauntlet_plus2", 1, 2,
		func(self *gameengine.Permanent, ctx *gameengine.DamageContext) bool {
			return dsYouControl(self, ctx) && dsRed(ctx) &&
				(dsInstantOrSorcery(ctx) || dsSourceHasType(ctx, "planeswalker"))
		})
}
