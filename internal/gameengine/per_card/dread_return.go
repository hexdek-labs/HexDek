package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// dread_return.go — per_card handler for Dread Return.
//
// Oracle text (Sorcery, {2}{B}{B}):
//
//	Return target creature card from your graveyard to the battlefield.
//	Flashback—Sacrifice three creatures.
//
// Why this needs a per_card handler: the spell body parsed to inert
// scaffold nodes, so on resolution Dread Return did NOTHING — a premier
// reanimation/combo enabler running dead. Verified inert: no registered
// handler for "Dread Return".
//
// Implemented here (OnResolve — authoritative spell body): reanimate the
// strongest creature card in the CASTER's own graveyard onto their
// battlefield under their control. Targeting is AI-resolved to the
// highest-impact creature (power, CMC tiebreak), matching how the engine
// resolves other "target creature card from your graveyard" effects.
//
// The Flashback cast mode (sacrifice three creatures to cast from the
// graveyard) is a casting-cost alternative handled by the cast pipeline,
// not the resolve body; it is orthogonal to this handler.

func init() {
	registerDreadReturn(Global())
	AddResetHook(registerDreadReturn)
}

func registerDreadReturn(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Dread Return", dreadReturnResolve)
}

func dreadReturnResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "dread_return"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return
	}

	// Pick the strongest creature card in the caster's own graveyard.
	var best *gameengine.Card
	bestScore := -1
	for _, c := range gs.Seats[seat].Graveyard {
		if c == nil || !cardHasType(c, "creature") {
			continue
		}
		score := c.BasePower*100 + cardCMC(c)
		if score > bestScore {
			bestScore = score
			best = c
		}
	}
	if best == nil {
		emitFail(gs, slug, "Dread Return", "no_creature_in_own_graveyard", map[string]interface{}{"seat": seat})
		return
	}

	enterFromGraveyardOrFallback(gs, best, seat, slug)
	emit(gs, slug, "Dread Return", map[string]interface{}{
		"seat":       seat,
		"reanimated": best.DisplayName(),
	})
	_ = gs.CheckEnd()
}
