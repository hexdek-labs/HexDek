package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Sharuum the Hegemon — {3}{W}{U}{B} 5/5 Legendary Artifact Creature — Sphinx.
//
//   Flying
//   When Sharuum enters, you may return target artifact card from your
//   graveyard to the battlefield.
//
// Implementation: OnETB picks the highest-CMC artifact in controller's
// graveyard (greedy "best target" pick — typically the most valuable
// artifact to reanimate, e.g. Wurmcoil Engine over a Sol Ring). Routes
// through enterBattlefieldWithETB so the reanimated artifact fires its
// own ETB cascade. Flying is keyword-pipeline. The infinite combo with
// Phyrexian Metamorph / Sculpting Steel (Sharuum bouncing itself via
// legend-rule) is enabled by this single hook.

func init() {
	registerSharuumTheHegemon(Global())
	AddResetHook(registerSharuumTheHegemon)
}

func registerSharuumTheHegemon(r *Registry) {
	r.OnETB("Sharuum the Hegemon", sharuumETB)
}

func sharuumETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "sharuum_etb_artifact_reanimate"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := perm.Controller
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}
	pick := pickHighestCMCArtifactFromGraveyard(s.Graveyard)
	if pick == nil {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":    seat,
			"reason":  "no_artifact_in_graveyard",
		})
		return
	}
	if !gameengine.CardCanEnterBattlefield(pick) {
		emitFail(gs, slug, perm.Card.DisplayName(), "target_cannot_enter_battlefield",
			map[string]interface{}{"card": pick.DisplayName()})
		return
	}
	moveCardBetweenZones(gs, seat, pick, "graveyard", "battlefield", "sharuum_reanimate")
	enterBattlefieldWithETB(gs, seat, pick, false)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         seat,
		"reanimated":   pick.DisplayName(),
	})
}

func pickHighestCMCArtifactFromGraveyard(gy []*gameengine.Card) *gameengine.Card {
	var best *gameengine.Card
	bestCMC := -1
	for _, c := range gy {
		if c == nil || !cardHasType(c, "artifact") {
			continue
		}
		cmc := cardCMC(c)
		if cmc > bestCMC {
			bestCMC = cmc
			best = c
		}
	}
	return best
}
