package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Olivia Voldaren — {2}{B}{R} 3/3 Legendary Creature — Vampire.
//
//   Flying
//   {1}{R}: Olivia Voldaren deals 1 damage to another target creature.
//   That creature becomes a Vampire in addition to its other types.
//   Put a +1/+1 counter on Olivia Voldaren.
//   {3}{B}{B}: Gain control of target Vampire for as long as you control
//   Olivia Voldaren.
//
// Implementation:
//   - Ability 0 ({1}{R}): mark 1 damage on target creature (per the
//     Walking Ballista pattern at walking_ballista.go:103), append
//     "vampire" to target's Types (per the Clavileño pattern at
//     clavileno_first_blessed.go:64 — approximates "in addition to"),
//     put a +1/+1 counter on Olivia. Self-target ruled out per "another
//     target creature".
//   - Ability 1 ({3}{B}{B}): control-grab needs the conditional-duration
//     ("as long as you control Olivia") continuous-effect machinery.
//     Stamped as emitPartial; full implementation would track Olivia's
//     battlefield-presence and revert control on LTB.

func init() {
	registerOliviaVoldaren(Global())
	AddResetHook(registerOliviaVoldaren)
}

func registerOliviaVoldaren(r *Registry) {
	r.OnActivated("Olivia Voldaren", oliviaActivate)
}

func oliviaActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	switch abilityIdx {
	case 0:
		oliviaPingAndVampire(gs, src, ctx)
	case 1:
		oliviaControlGrab(gs, src, ctx)
	}
}

func oliviaPingAndVampire(gs *gameengine.GameState, src *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "olivia_ping_to_vampire"
	target, _ := ctx["target_perm"].(*gameengine.Permanent)
	if target == nil || target == src || target.Card == nil || !target.IsCreature() {
		emitFail(gs, slug, src.Card.DisplayName(), "no_eligible_target", nil)
		return
	}
	target.MarkedDamage += 1
	gs.LogEvent(gameengine.Event{
		Kind:   "damage",
		Seat:   src.Controller,
		Target: target.Controller,
		Source: src.Card.DisplayName(),
		Amount: 1,
		Details: map[string]interface{}{
			"target_card": target.Card.DisplayName(),
		},
	})
	if !cardHasType(target.Card, "vampire") {
		target.Card.Types = append(target.Card.Types, "vampire")
		gs.InvalidateCharacteristicsCache()
	}
	src.AddCounter("+1/+1", 1)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"target_card":      target.Card.DisplayName(),
		"target_seat":      target.Controller,
		"damage":           1,
		"became_vampire":   true,
		"olivia_p_after":   src.Power(),
		"olivia_t_after":   src.Toughness(),
	})
}

func oliviaControlGrab(gs *gameengine.GameState, src *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "olivia_grab_vampire"
	emitPartial(gs, slug, src.Card.DisplayName(),
		"control_grab_as_long_as_you_control_olivia_needs_conditional_duration_continuous_effect")
}
