package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerEddieBrockCustom adds Eddie Brock's "ETB return MV ≤ 1 creature
// from graveyard" trigger that the auto-generated stub omits.
//
// Oracle text:
//
//	When Eddie Brock enters, return target creature card with mana value
//	1 or less from your graveyard to the battlefield.
//	{3}{B}{R}{G}: Transform Eddie Brock. Activate only as a sorcery.
//	(transformed back face: Venom)
//	Menace, trample, haste
//	Whenever Venom attacks, you may sacrifice another creature. If you
//	do, draw X cards, then you may put a permanent card with mana value
//	X or less from your hand onto the battlefield, where X is the
//	sacrificed creature's mana value.
//
// We wire the front-face ETB reanimate. Pick the highest-power creature
// in graveyard that costs 1 or less so the AI doesn't squander the slot
// on a 0/4 wall when there's a Goblin Guide available. Transform + Venom
// attack triggers are noted as partials; transforming requires
// engine-side DFC support that's outside this handler's scope.
func registerEddieBrockCustom(r *Registry) {
	r.OnETB("Eddie Brock // Venom, Lethal Protector", eddieBrockETB)
	r.OnETB("Eddie Brock", eddieBrockETB)
}

func eddieBrockETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "eddie_brock_etb_reanimate"
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	var best *gameengine.Card
	bestPower := -1
	for _, c := range seat.Graveyard {
		if c == nil {
			continue
		}
		if !cardHasType(c, "creature") {
			continue
		}
		if cardCMC(c) > 1 {
			continue
		}
		if c.BasePower > bestPower {
			best = c
			bestPower = c.BasePower
		}
	}
	if best == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_legal_target", nil)
		emitPartial(gs, "eddie_brock_transform", perm.Card.DisplayName(),
			"transform to Venom + Venom attack trigger require engine DFC + sacrifice-on-attack support")
		return
	}
	// Place on battlefield — createPermanent (called via
	// enterBattlefieldWithETB) sweeps the source from every private
	// zone, so the manual graveyard splice that used to precede this
	// call is redundant (Wave 2 final-audit cleanup).
	enterBattlefieldWithETB(gs, perm.Controller, best, false)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"reanimated": best.DisplayName(),
		"power":      bestPower,
	})
	emitPartial(gs, "eddie_brock_transform", perm.Card.DisplayName(),
		"transform to Venom + Venom attack trigger require engine DFC + sacrifice-on-attack support")
}
