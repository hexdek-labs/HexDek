package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Atogatog — {W}{U}{B}{R}{G} 5/5 Legendary Creature — Atog.
//
//   Sacrifice an Atog creature: Atogatog gets +X/+X until end of turn,
//   where X is the sacrificed creature's power.
//
// Implementation: OnActivated. Target picked from ctx["target_perm"];
// must be an Atog (subtype check) controlled by Atogatog's controller.
// Read X = target.Power() BEFORE sacrificing (CR §608.2b — last known
// information), then SacrificePermanent, then stamp Modification with
// Power=X / Toughness=X / Duration="until_end_of_turn" on Atogatog.
// Activating with Atogatog as the target is legal (self-sacrifice for
// no benefit since Atogatog leaves play before the buff lands — emitFail).

func init() {
	registerAtogatog(Global())
	AddResetHook(registerAtogatog)
}

func registerAtogatog(r *Registry) {
	r.OnActivated("Atogatog", atogatogActivate)
}

func atogatogActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "atogatog_eat_atog"
	if gs == nil || src == nil || src.Card == nil || ctx == nil {
		return
	}
	target, _ := ctx["target_perm"].(*gameengine.Permanent)
	if target == nil || target.Card == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_target_in_ctx", nil)
		return
	}
	if target == src {
		emitFail(gs, slug, src.Card.DisplayName(), "self_sacrifice_pointless", nil)
		return
	}
	if target.Controller != src.Controller {
		emitFail(gs, slug, src.Card.DisplayName(), "target_not_yours", nil)
		return
	}
	if !cardHasSubtype(target.Card, "atog") {
		emitFail(gs, slug, src.Card.DisplayName(), "target_not_atog", nil)
		return
	}
	x := target.Power()
	targetName := target.Card.DisplayName()
	gameengine.SacrificePermanent(gs, target, "atogatog_sac_atog")
	if x > 0 {
		src.Modifications = append(src.Modifications, gameengine.Modification{
			Power:     x,
			Toughness: x,
			Duration:  "until_end_of_turn",
			Timestamp: gs.NextTimestamp(),
		})
		gs.InvalidateCharacteristicsCache()
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":             src.Controller,
		"sacrificed":       targetName,
		"x":                x,
		"atogatog_p_after": src.Power(),
	})
}
