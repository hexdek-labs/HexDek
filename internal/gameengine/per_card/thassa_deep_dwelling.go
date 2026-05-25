package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerThassaDeepDwelling wires Thassa, Deep-Dwelling.
//
// Oracle text (Scryfall, verified 2026-05-04):
//
//	Indestructible
//	As long as your devotion to blue is less than five, Thassa isn't a
//	  creature.
//	At the beginning of your end step, exile up to one other target
//	  creature you control, then return that card to the battlefield
//	  under your control.
//	{3}{U}: Tap another target creature.
//
// Implementation:
//   - "end_step_controller": pick the highest-power non-token creature
//     we control (other than Thassa). Exile + return: this re-triggers
//     ETB effects, which is the bread-and-butter Thassa Blink line.
//     For tokens we skip (they cease to exist on exile).
//   - Activated tap-another-target uses the activation pipeline; the
//     UEOT effect is straightforward but we emitPartial since the
//     per-card layer doesn't ship a target-resolution scaffold.
//   - Devotion/indestructible static handled at layer 6 elsewhere.
func registerThassaDeepDwelling(r *Registry) {
	r.OnETB("Thassa, Deep-Dwelling", thassaDeepDwellingETB)
	r.OnTrigger("Thassa, Deep-Dwelling", "end_step", thassaDeepDwellingEndStep)
	r.OnActivated("Thassa, Deep-Dwelling", thassaDeepDwellingActivate)
}

func thassaDeepDwellingETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, "thassa_deep_dwelling_static", perm.Card.DisplayName(),
		"devotion_isnt_a_creature_clause_not_enforced")
}

func thassaDeepDwellingEndStep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "thassa_deep_dwelling_blink"
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	var pick *gameengine.Permanent
	bestPow := -1
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || !p.IsCreature() || p.Card == nil {
			continue
		}
		if p.IsToken() {
			continue
		}
		if pw := p.Power(); pw > bestPow {
			bestPow = pw
			pick = p
		}
	}
	if pick == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_blink_target", nil)
		return
	}
	card := pick.Card
	// Exile through the canonical battlefield-exit API so §614
	// would_be_exiled replacements + aura detach + replacement-effect
	// unregister + LTB triggers fire. ExilePermanent handles the
	// "battlefield → exile" leg cleanly; enterBattlefieldWithETB pulls
	// the same *Card pointer back from exile and re-ETBs it (the
	// canonical Thassa blink: ETB triggers fire on the return because
	// the new permanent is a fresh object per CR §400.7).
	if !gameengine.ExilePermanent(gs, pick, perm) {
		emitFail(gs, slug, perm.Card.DisplayName(), "exile_failed_or_replaced", nil)
		return
	}
	enterBattlefieldWithETB(gs, perm.Controller, card, false)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"creature": card.DisplayName(),
	})
}

func thassaDeepDwellingActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil {
		return
	}
	emitPartial(gs, "thassa_deep_dwelling_tap_target", src.Card.DisplayName(),
		"tap_another_target_creature_activation_not_modeled")
}
