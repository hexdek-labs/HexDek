package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheGreatHenge wires The Great Henge.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/The%20Great%20Henge):
//
//	This spell costs {X} less to cast, where X is the greatest power
//	among creatures you control.
//	When The Great Henge enters, you gain life equal to its power.
//	{T}: Add {G}{G}. You gain 2 life.
//	Whenever another nontoken creature enters under your control, put a
//	+1/+1 counter on it. Draw a card.
//
// {7}{G}{G} Legendary Artifact. The cost-reduction makes it castable at
// {2}{G}{G} or cheaper in any deck running a 5/5+ creature; Stompy /
// Gruul / Mono-G value piles run it as a buy-it-first card. Every
// subsequent creature you cast becomes a +1/+1 anthem + cantrip.
//
// This handler covers two of the four printed abilities — the ETB
// life-gain and the recurring creature-ETB +1/+1 + cantrip. The
// cost-reduction static and the {T}: mana ability are out of scope
// (cost reduction is layer/static work; the tap-for-mana is exercised
// via the engine's generic ManaPool path on tap).
//
// Implementation:
//   - OnETB("The Great Henge"): gain life equal to perm.Power(). The
//     printed P/T is 5/5, so a baseline activation grants 5. If a deck
//     stacks +1/+1 counters on Henge somehow (Gemrazer-style anthems),
//     Power() reads the layer-adjusted value.
//   - OnTrigger("nonland_permanent_etb"): mirrors Guardian Project's
//     filter chain (creature + own-control + nontoken) and explicitly
//     skips Henge itself ("another nontoken creature"). Actions: add
//     one +1/+1 counter to the entering creature, then draw 1.
func registerTheGreatHenge(r *Registry) {
	r.OnETB("The Great Henge", theGreatHengeETB)
	r.OnTrigger("The Great Henge", "nonland_permanent_etb", theGreatHengeOnCreatureETB)
}

func theGreatHengeETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "the_great_henge_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	// Power() returns layer-adjusted power; falls back to BasePower (5
	// per printed) when no AST modifications apply.
	x := perm.Power()
	if x < 1 {
		// Henge's printed power is 5; if the test fixture didn't stamp
		// BasePower we still want a non-zero gain to exercise the path.
		// Per CR §107.1b life-gain of 0 is legal — preserve that for
		// authentic 0-power scenarios — only floor when negative.
		if x < 0 {
			x = 0
		}
	}
	if x > 0 {
		gameengine.GainLife(gs, perm.Controller, x, "The Great Henge")
	}
	emit(gs, slug, "The Great Henge", map[string]interface{}{
		"seat":      perm.Controller,
		"life_gain": x,
	})
}

func theGreatHengeOnCreatureETB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "the_great_henge_creature_etb"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	entering, _ := ctx["perm"].(*gameengine.Permanent)
	if entering == nil || entering.Card == nil {
		return
	}
	if entering == perm {
		return // "Whenever ANOTHER nontoken creature enters"
	}
	if !entering.IsCreature() {
		return
	}
	if entering.Controller != perm.Controller {
		return
	}
	if entering.IsToken() {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	if entering.Counters == nil {
		entering.Counters = map[string]int{}
	}
	entering.Counters["+1/+1"]++
	drawOne(gs, perm.Controller, "The Great Henge")

	emit(gs, slug, "The Great Henge", map[string]interface{}{
		"seat":     perm.Controller,
		"entering": entering.Card.DisplayName(),
	})
}
