package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Karn, Silver Golem — {5} 4/4 Legendary Artifact Creature — Golem.
//
//   Whenever Karn blocks or becomes blocked, it gets -4/+4 until end
//   of turn.
//   {1}: Target noncreature artifact becomes an artifact creature with
//     power and toughness each equal to its mana value until end of turn.
//
// Implementation covers the activated animate-artifact ability — the
// stax-tron archetype payoff. Target picked from ctx["target_perm"];
// must be a noncreature artifact controlled by anyone (CR §608.2 — no
// "you control" restriction). Animation = Card.Types append "creature"
// (Clavileño type-add pattern at clavileno_first_blessed.go:64) + a
// Modification with Power/Toughness = cardCMC(target) and Duration =
// "until_end_of_turn" (cleared by EOT cleanup).
//
// The blocks/blocked self-pump trigger needs a creature_blocks/blocked
// per_card event (no canonical wiring) — left to AST handling. The
// activated animate is the primary archetype use.

func init() {
	registerKarnSilverGolem(Global())
	AddResetHook(registerKarnSilverGolem)
}

func registerKarnSilverGolem(r *Registry) {
	r.OnActivated("Karn, Silver Golem", karnActivate)
}

func karnActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "karn_animate_artifact"
	if gs == nil || src == nil || src.Card == nil || ctx == nil {
		return
	}
	target, _ := ctx["target_perm"].(*gameengine.Permanent)
	if target == nil || target.Card == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_target_in_ctx", nil)
		return
	}
	if !cardHasType(target.Card, "artifact") {
		emitFail(gs, slug, src.Card.DisplayName(), "target_not_artifact",
			map[string]interface{}{"card": target.Card.DisplayName()})
		return
	}
	if cardHasType(target.Card, "creature") {
		emitFail(gs, slug, src.Card.DisplayName(), "target_already_creature",
			map[string]interface{}{"card": target.Card.DisplayName()})
		return
	}
	mv := cardCMC(target.Card)
	target.Card.Types = append(target.Card.Types, "creature")
	target.Modifications = append(target.Modifications, gameengine.Modification{
		Power:     mv,
		Toughness: mv,
		Duration:  "until_end_of_turn",
		Timestamp: gs.NextTimestamp(),
	})
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":          src.Controller,
		"target_card":   target.Card.DisplayName(),
		"target_seat":   target.Controller,
		"power":         mv,
		"toughness":     mv,
		"duration":      "until_end_of_turn",
	})
}
