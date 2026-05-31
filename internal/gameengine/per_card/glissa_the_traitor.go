package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Glissa, the Traitor — {B}{R}{G} 3/3 Legendary Creature — Zombie Elf.
//
//   First strike, deathtouch
//   Whenever a creature an opponent controls dies, you may return target
//   artifact card from your graveyard to your hand.
//
// Implementation: OnTrigger creature_dies, filter:
//   - dying creature's controller != Glissa's controller
//   - dying is a creature (defensive)
// Greedy yes on the may-return — picks highest-CMC artifact from
// controller's graveyard (most valuable to rebuy). First strike +
// deathtouch are keyword-pipeline.
//
// Note: the original printed text was "dealt damage by Glissa this turn",
// but the Oracle text in the AST dataset is the modern wording shown
// above ("a creature an opponent controls dies"). Implemented per Oracle.

func init() {
	registerGlissaTheTraitor(Global())
	AddResetHook(registerGlissaTheTraitor)
}

func registerGlissaTheTraitor(r *Registry) {
	r.OnTrigger("Glissa, the Traitor", "creature_dies", glissaCreatureDies)
}

func glissaCreatureDies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "glissa_traitor_artifact_rebuy"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	dying, _ := ctx["perm"].(*gameengine.Permanent)
	if dying == nil || dying.Card == nil {
		return
	}
	if !dying.IsCreature() {
		return
	}
	if dying.Controller == perm.Controller {
		return // same-side death is not "an opponent controls"
	}
	seat := perm.Controller
	s := gs.Seats[seat]
	if s == nil || s.Lost || len(s.Graveyard) == 0 {
		return
	}
	pick := pickHighestCMCArtifactFromGraveyard(s.Graveyard)
	if pick == nil {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":          seat,
			"trigger_cause": dying.Card.DisplayName(),
			"reason":        "no_artifact_in_graveyard",
		})
		return
	}
	moveCardBetweenZones(gs, seat, pick, "graveyard", "hand", "glissa_rebuy_artifact")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          seat,
		"trigger_cause": dying.Card.DisplayName(),
		"returned":      pick.DisplayName(),
	})
}
