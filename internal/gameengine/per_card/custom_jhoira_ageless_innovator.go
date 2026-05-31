package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerJhoiraAgelessInnovatorCustom implements Jhoira's tap-to-cheat
// activation. The auto-generated stub is a no-op.
//
// Oracle text:
//
//	{T}: Put two ingenuity counters on Jhoira, then you may put an
//	artifact card with mana value X or less from your hand onto the
//	battlefield, where X is the number of ingenuity counters on Jhoira.
//
// Implementation notes:
//   - Cost is just {T}; engine activation gates handle that. We
//     defensively check src.Tapped.
//   - Add 2 ingenuity counters first (the counter-add precedes the
//     cheat per the "then" wording).
//   - X = current ingenuity counter count *after* the +2. Pick the
//     highest-CMC artifact in hand with cmc ≤ X.
//   - enterBattlefieldWithETB fires the proper ETB cascade (ETB
//     replacements, per-card hooks, observer triggers).
func registerJhoiraAgelessInnovatorCustom(r *Registry) {
	r.OnActivated("Jhoira, Ageless Innovator", jhoiraIngenuityActivate)
}

func jhoiraIngenuityActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "jhoira_ingenuity_cheat"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}

	src.Tapped = true
	src.AddCounter("ingenuity", 2)
	x := src.Counters["ingenuity"]

	// Pick the best artifact in hand with CMC ≤ X. Prefer highest CMC
	// (we get the biggest free play); tiebreak by hand index for
	// determinism.
	var pick *gameengine.Card
	bestCMC := -1
	for _, c := range seat.Hand {
		if c == nil {
			continue
		}
		if !cardHasType(c, "artifact") {
			continue
		}
		cmc := cardCMC(c)
		if cmc > x {
			continue
		}
		if cmc > bestCMC {
			pick = c
			bestCMC = cmc
		}
	}

	if pick == nil {
		// Counters still added; the cheat is a "may" so this is a clean
		// pass, not a failure.
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":              src.Controller,
			"ingenuity_counters": x,
			"cheated":           "",
			"note":              "no_eligible_artifact",
		})
		return
	}

	// createPermanent (called via enterBattlefieldWithETB) sweeps the
	// source from every private zone, so the manual hand splice is
	// redundant (Wave 2 final-audit cleanup).
	enterBattlefieldWithETB(gs, src.Controller, pick, false)

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":              src.Controller,
		"ingenuity_counters": x,
		"cheated":           pick.DisplayName(),
		"cheated_cmc":       bestCMC,
	})
}
